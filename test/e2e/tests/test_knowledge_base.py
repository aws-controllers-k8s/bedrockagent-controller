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

"""Integration tests for the Bedrock KnowledgeBase resource"""

import time
import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e import knowledge_base
from logging import getLogger

KNOWLEDGE_BASE_RESOURCE_PLURAL = "knowledgebases"
DELETE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_PERIODS = 3
CHECK_STATUS_WAIT_PERIODS = 5
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30

logger = getLogger(__name__)


@pytest.fixture(scope="module")
def simple_knowledge_base():
    knowledge_base_name = random_suffix_name("bedrock-test-kb", 32)
    knowledge_base_description = "Test knowledge base for e2e testing"
    knowledge_base_role_arn = get_bootstrap_resources().AgentRole.arn
    embedding_model_arn = "arn:aws:bedrock:us-west-2::foundation-model/amazon.titan-embed-text-v1"
    opensearch_collection_arn = "arn:aws:aoss:us-west-2:050752643586:collection/kqs7wb5kssgmnlomcb7e"
    vector_index_name = "e2e-index"
    knowledge_base_tag_key = "test1"
    knowledge_base_tag_value = "value1"

    replacements = REPLACEMENT_VALUES.copy()
    replacements["KNOWLEDGE_BASE_NAME"] = knowledge_base_name
    replacements["KNOWLEDGE_BASE_DESCRIPTION"] = knowledge_base_description
    replacements["KNOWLEDGE_BASE_ROLE_ARN"] = knowledge_base_role_arn
    replacements["EMBEDDING_MODEL_ARN"] = embedding_model_arn
    replacements["OPENSEARCH_COLLECTION_ARN"] = opensearch_collection_arn
    replacements["VECTOR_INDEX_NAME"] = vector_index_name
    replacements["TAG_KEY_1"] = knowledge_base_tag_key
    replacements["TAG_VALUE_1"] = knowledge_base_tag_value

    resource_data = load_resource(
        "knowledge_base",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        KNOWLEDGE_BASE_RESOURCE_PLURAL,
        knowledge_base_name,
        namespace="default",
    )

    logger.info("Creating KnowledgeBase %s", knowledge_base_name)
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert k8s.get_resource_exists(ref)
    assert cr is not None
    assert "status" in cr
    assert "knowledgeBaseID" in cr["status"]
    assert "ackResourceMetadata" in cr["status"]
    assert "arn" in cr["status"]["ackResourceMetadata"]
    knowledge_base_id = cr["status"]["knowledgeBaseID"]
    knowledge_base_arn = cr["status"]["ackResourceMetadata"]["arn"]

    knowledge_base.wait_until_exists(knowledge_base_id)
    logger.info("KnowledgeBase %s exists in cluster", knowledge_base_name)

    yield (ref, cr, knowledge_base_id, knowledge_base_arn)

    logger.info("Deleting KnowledgeBase %s", knowledge_base_name)
    _, deleted = k8s.delete_custom_resource(
        ref,
        wait_periods=DELETE_WAIT_PERIODS,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    knowledge_base.wait_until_deleted(knowledge_base_id)


@service_marker
@pytest.mark.canary
class TestKnowledgeBase:
    def test_crud(self, simple_knowledge_base):
        ref, res, knowledge_base_id, knowledge_base_arn = simple_knowledge_base

        assert k8s.wait_on_condition(
            ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=CHECK_STATUS_WAIT_PERIODS,
            period_length=CHECK_STATUS_WAIT_SECONDS
        )

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert "spec" in cr
        assert "description" in cr["spec"]
        assert cr["spec"]["description"] == "Test knowledge base for e2e testing"

        latest = knowledge_base.get(knowledge_base_id)
        assert latest is not None
        assert "description" in latest
        assert latest["description"] == "Test knowledge base for e2e testing"

        # Test update
        updates = {
            "spec": {"description": "Updated test knowledge base description"},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest = knowledge_base.get(knowledge_base_id)
        assert latest is not None
        assert "description" in latest
        assert latest["description"] == "Updated test knowledge base description"

    def test_tags(self, simple_knowledge_base):
        ref, res, knowledge_base_id, knowledge_base_arn = simple_knowledge_base

        assert k8s.wait_on_condition(
            ref,
            "ACK.ResourceSynced",
            "True",
            wait_periods=CHECK_STATUS_WAIT_PERIODS,
            period_length=CHECK_STATUS_WAIT_SECONDS
        )

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert "spec" in cr
        assert "tags" in cr["spec"]
        assert cr["spec"]["tags"] == {"test1": "value1"}

        latest = knowledge_base.getTags(knowledge_base_arn)
        assert latest is not None
        assert "test1" in latest
        assert latest["test1"] == "value1"

        # Test update
        updates = {
            "spec": {
                "tags": {
                    "test1": "newValue1",
                    "test2": "value2", 
                    "test3": "value3"
                }
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest = knowledge_base.getTags(knowledge_base_arn)
        assert latest is not None
        assert "test1" in latest
        assert latest["test1"] == "newValue1"
        assert "test2" in latest
        assert latest["test2"] == "value2"
        assert "test3" in latest
        assert latest["test3"] == "value3"