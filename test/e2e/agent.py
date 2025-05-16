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

"""Utilities for working with Agent resources"""

import datetime
import time

import boto3
import pytest

DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS = 60 * 10
DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS = 60 * 10
DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS = 15


def wait_until_exists(
    agent_id: str,
    timeout_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS,
    interval_seconds: int = DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS,
) -> None:
    """Waits until a Agent with a supplied ID is returned from
    Bedrock GetAgent API.

    Usage:
        from e2e.agent import wait_until_exists

        wait_until_exists(agent_id)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for Agent to exist "
                "in Bedrock GetAgent API"
            )
        time.sleep(interval_seconds)

        latest = get(agent_id)
        if latest is not None:
            break


def wait_until_deleted(
    agent_id: str,
    timeout_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS,
    interval_seconds: int = DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS,
) -> None:
    """Waits until a Agent with a supplied ID is no longer returned from
    the Bedrock GetAgent API.

    Usage:
        from e2e.agent import wait_until_deleted

        wait_until_deleted(agent_id)

    Raises:
        pytest.fail upon timeout
    """
    now = datetime.datetime.now()
    timeout = now + datetime.timedelta(seconds=timeout_seconds)

    while True:
        if datetime.datetime.now() >= timeout:
            pytest.fail(
                "Timed out waiting for Agent to be "
                "deleted in Bedrock GetAgent API"
            )
        time.sleep(interval_seconds)

        latest = get(agent_id)
        if latest is None:
            break


def get(agent_id: str):
    """Returns a dict containing the Agent record from the Bedrock GetAgent
    API.

    If no such Agent exists, returns None.
    """
    c = boto3.client("bedrock-agent")
    try:
        resp = c.get_vpc_origin(Id=agent_id)
        return resp["agent"]
    except c.exceptions.ResourceNotFoundException:
        return None