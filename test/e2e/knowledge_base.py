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

"""Utilities for working with KnowledgeBase resources"""

import datetime
import time

import boto3
import pytest

DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS = 60 * 10
DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS = 60 * 10
DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS = 15


def wait_until_exists(
    knowledge_base_id: str,
    timeout_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS,
    interval_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS,
) -> None:
    """Waits until a KnowledgeBase with a supplied ID is returned from
    Bedrock GetKnowledgeBase API.

    Usage:
        from e2e.knowledge_base import wait_until_exists

        wait_until_exists(knowledge_base_id)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for KnowledgeBase to exist "
                "in Bedrock GetKnowledgeBase API"
            )
        time.sleep(interval_seconds)

        latest = get(knowledge_base_id)
        if latest is not None:
            break


def wait_until_deleted(
    knowledge_base_id: str,
    timeout_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS,
    interval_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS,
) -> None:
    """Waits until a KnowledgeBase with a supplied ID is no longer returned from
    the Bedrock GetKnowledgeBase API.

    Usage:
        from e2e.knowledge_base import wait_until_deleted

        wait_until_deleted(knowledge_base_id)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for KnowledgeBase to be "
                "deleted in Bedrock GetKnowledgeBase API"
            )
        time.sleep(interval_seconds)

        latest = get(knowledge_base_id)
        if latest is None:
            break


def get(knowledge_base_id: str):
    """Returns a dict containing the KnowledgeBase record from the Bedrock GetKnowledgeBase
    API.

    If no such KnowledgeBase exists, returns None.
    """
    client = boto3.client("bedrock-agent")
    try:
        resp = client.get_knowledge_base(knowledgeBaseId=knowledge_base_id)
        return resp["knowledgeBase"]
    except client.exceptions.ResourceNotFoundException:
        return None
    
def getTags(knowledge_base_arn: str):
    """Returns a dict containing the Tags for Bedrock KnowledgeBase resource from the Bedrock ListTagsForResource
    API.

    If no such Resource exists, returns None.
    """
    client = boto3.client("bedrock-agent")
    try:
        resp = client.list_tags_for_resource(resourceArn=knowledge_base_arn)
        return resp["tags"]
    except client.exceptions.ResourceNotFoundException:
        return None