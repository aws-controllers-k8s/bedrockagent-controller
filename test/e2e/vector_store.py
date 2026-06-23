# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""A bootstrappable OpenSearch Serverless vector store for KnowledgeBase tests.

A Bedrock KnowledgeBase backed by OpenSearch Serverless requires, before the
KnowledgeBase itself can be created:

  * an IAM role the Bedrock service assumes to invoke the embedding model and
    read/write the vector store;
  * an OpenSearch Serverless (AOSS) encryption, network and data-access policy;
  * a VECTORSEARCH collection; and
  * a vector index whose field mapping matches the KnowledgeBase spec.

This module provisions all of the above as a single unit so the ordering and
the data-access principals (the KB role *and* the bootstrap caller, which
creates the index) are handled in one place.
"""

import json
import logging
import time

from dataclasses import dataclass, field

import boto3
from opensearchpy import AWSV4SignerAuth, OpenSearch, RequestsHttpConnection

from acktest import resources
from acktest.bootstrapping import Bootstrappable
from acktest.bootstrapping.iam import Role, UserPolicies

# amazon.titan-embed-text-v2:0 emits 1024-dimension vectors.
EMBEDDING_MODEL_ID = "amazon.titan-embed-text-v2:0"
EMBEDDING_VECTOR_DIMENSION = 1024

# Field names referenced by resources/knowledge_base.yaml fieldMapping.
TEXT_FIELD = "text"
VECTOR_FIELD = "vector"
METADATA_FIELD = "metadata"

COLLECTION_ACTIVE_TIMEOUT_SECONDS = 60 * 10
COLLECTION_DELETE_TIMEOUT_SECONDS = 60 * 10
POLL_INTERVAL_SECONDS = 15
# AOSS data-access policy and index propagation are eventually consistent;
# give them time to settle before a KnowledgeBase tries to use the index.
PROPAGATION_WAIT_SECONDS = 60


def _role_arn_from_caller(caller_arn: str) -> str:
    """Normalises an STS caller ARN to an IAM principal ARN usable in an AOSS
    data-access policy.

    A test runner authenticated via an assumed role reports an ARN like
    ``arn:aws:sts::123456789012:assumed-role/MyRole/session``; AOSS data-access
    policies match on the underlying ``arn:aws:iam::123456789012:role/MyRole``.
    User and role ARNs are returned unchanged.
    """
    if ":assumed-role/" not in caller_arn:
        return caller_arn
    account_id = caller_arn.split(":")[4]
    role_name = caller_arn.split(":assumed-role/")[1].split("/")[0]
    return f"arn:aws:iam::{account_id}:role/{role_name}"


@dataclass
class VectorStore(Bootstrappable):
    # Inputs
    name_prefix: str

    # Subresources
    role: Role = field(default=None)

    # Outputs
    collection_name: str = field(init=False)
    index_name: str = field(default="ack-e2e-index", init=False)
    collection_id: str = field(default="", init=False)
    collection_arn: str = field(default="", init=False)
    collection_endpoint: str = field(default="", init=False)
    encryption_policy_name: str = field(default="", init=False)
    network_policy_name: str = field(default="", init=False)
    access_policy_name: str = field(default="", init=False)

    def __post_init__(self):
        # AOSS names must be 3-32 chars, lowercase, and start with a letter.
        self.collection_name = resources.random_suffix_name(self.name_prefix, 32)
        self.encryption_policy_name = resources.random_suffix_name("ack-enc", 32)
        self.network_policy_name = resources.random_suffix_name("ack-net", 32)
        self.access_policy_name = resources.random_suffix_name("ack-acc", 32)
        self.role = Role(
            "kb-vector-role",
            "bedrock.amazonaws.com",
            description="Role assumed by Bedrock to access the KnowledgeBase vector store",
            user_policies=self._role_policies(),
        )

    def _role_policies(self):
        policy = {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": ["bedrock:InvokeModel"],
                    "Resource": [self.embedding_model_arn],
                },
                {
                    "Effect": "Allow",
                    "Action": ["aoss:APIAccessAll"],
                    "Resource": ["*"],
                },
            ],
        }
        return UserPolicies("kb-vector-policy", [json.dumps(policy)])

    @property
    def aoss_client(self):
        return boto3.client("opensearchserverless", region_name=self.region)

    @property
    def embedding_model_arn(self) -> str:
        return f"arn:aws:bedrock:{self.region}::foundation-model/{EMBEDDING_MODEL_ID}"

    def bootstrap(self):
        """Creates the IAM role, AOSS policies, collection and vector index."""
        # Bootstraps the role subresource (and its inline policy) first.
        super().bootstrap()

        collection_resource = [f"collection/{self.collection_name}"]

        self.aoss_client.create_security_policy(
            name=self.encryption_policy_name,
            type="encryption",
            policy=json.dumps(
                {
                    "Rules": [
                        {"ResourceType": "collection", "Resource": collection_resource}
                    ],
                    "AWSOwnedKey": True,
                }
            ),
        )
        self.aoss_client.create_security_policy(
            name=self.network_policy_name,
            type="network",
            policy=json.dumps(
                [
                    {
                        "Rules": [
                            {"ResourceType": "collection", "Resource": collection_resource},
                            {"ResourceType": "dashboard", "Resource": collection_resource},
                        ],
                        "AllowFromPublic": True,
                    }
                ]
            ),
        )

        caller_arn = _role_arn_from_caller(
            boto3.client("sts", region_name=self.region).get_caller_identity()["Arn"]
        )
        self.aoss_client.create_access_policy(
            name=self.access_policy_name,
            type="data",
            policy=json.dumps(
                [
                    {
                        "Rules": [
                            {
                                "ResourceType": "index",
                                "Resource": [f"index/{self.collection_name}/*"],
                                "Permission": ["aoss:*"],
                            },
                            {
                                "ResourceType": "collection",
                                "Resource": collection_resource,
                                "Permission": ["aoss:*"],
                            },
                        ],
                        # The bootstrap caller creates the index; the KB role
                        # reads/writes it at runtime. Both need data access.
                        "Principal": [caller_arn, self.role.arn],
                    }
                ]
            ),
        )

        resp = self.aoss_client.create_collection(
            name=self.collection_name, type="VECTORSEARCH"
        )
        self.collection_id = resp["createCollectionDetail"]["id"]
        self._wait_until_collection_active()
        self._create_vector_index()
        # Let the data-access policy and the new index propagate before the
        # KnowledgeBase attempts to use them.
        time.sleep(PROPAGATION_WAIT_SECONDS)

    def _wait_until_collection_active(self):
        timeout = time.time() + COLLECTION_ACTIVE_TIMEOUT_SECONDS
        while True:
            details = self.aoss_client.batch_get_collection(ids=[self.collection_id])[
                "collectionDetails"
            ]
            if details and details[0]["status"] == "ACTIVE":
                self.collection_arn = details[0]["arn"]
                # collectionEndpoint is https://<id>.<region>.aoss.amazonaws.com
                self.collection_endpoint = details[0]["collectionEndpoint"]
                return
            if details and details[0]["status"] == "FAILED":
                raise RuntimeError(
                    f"AOSS collection {self.collection_name} entered FAILED state"
                )
            if time.time() >= timeout:
                raise TimeoutError(
                    f"Timed out waiting for AOSS collection {self.collection_name} to become ACTIVE"
                )
            time.sleep(POLL_INTERVAL_SECONDS)

    def _opensearch_client(self) -> OpenSearch:
        host = self.collection_endpoint.replace("https://", "")
        auth = AWSV4SignerAuth(
            boto3.Session().get_credentials(), self.region, "aoss"
        )
        return OpenSearch(
            hosts=[{"host": host, "port": 443}],
            http_auth=auth,
            use_ssl=True,
            verify_certs=True,
            connection_class=RequestsHttpConnection,
            pool_maxsize=20,
        )

    def _create_vector_index(self):
        body = {
            "settings": {"index": {"knn": True}},
            "mappings": {
                "properties": {
                    VECTOR_FIELD: {
                        "type": "knn_vector",
                        "dimension": EMBEDDING_VECTOR_DIMENSION,
                        "method": {
                            "name": "hnsw",
                            "engine": "faiss",
                            "space_type": "l2",
                        },
                    },
                    TEXT_FIELD: {"type": "text"},
                    METADATA_FIELD: {"type": "text"},
                }
            },
        }
        client = self._opensearch_client()
        # The data-access policy can take a moment to apply after creation;
        # retry the index creation rather than failing on a transient 403.
        last_exc = None
        for _ in range(10):
            try:
                client.indices.create(index=self.index_name, body=body)
                return
            except Exception as ex:  # noqa: BLE001 - surfaced after retries
                last_exc = ex
                logging.info("Waiting to create vector index: %s", ex)
                time.sleep(POLL_INTERVAL_SECONDS)
        raise RuntimeError(
            f"Failed to create vector index {self.index_name}: {last_exc}"
        )

    def cleanup(self):
        """Deletes the collection, AOSS policies and IAM role."""
        client = self.aoss_client
        if self.collection_id:
            try:
                client.delete_collection(id=self.collection_id)
                self._wait_until_collection_deleted()
            except client.exceptions.ResourceNotFoundException:
                pass

        for name, policy_type, deleter in (
            (self.access_policy_name, "data", client.delete_access_policy),
            (self.network_policy_name, "network", client.delete_security_policy),
            (self.encryption_policy_name, "encryption", client.delete_security_policy),
        ):
            try:
                deleter(name=name, type=policy_type)
            except Exception as ex:  # noqa: BLE001 - best-effort teardown
                logging.info("Could not delete AOSS policy %s: %s", name, ex)

        # Deletes the role subresource last.
        super().cleanup()

    def _wait_until_collection_deleted(self):
        timeout = time.time() + COLLECTION_DELETE_TIMEOUT_SECONDS
        while True:
            details = self.aoss_client.batch_get_collection(ids=[self.collection_id])[
                "collectionDetails"
            ]
            if not details:
                return
            if time.time() >= timeout:
                raise TimeoutError(
                    f"Timed out waiting for AOSS collection {self.collection_name} to delete"
                )
            time.sleep(POLL_INTERVAL_SECONDS)
