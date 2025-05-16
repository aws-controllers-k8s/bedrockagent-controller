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

"""Integration tests for the Bedrock Agent resource"""

import time
import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e import agent
from logging import getLogger

AGENT_RESOURCE_PLURAL = "agents"
DELETE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_PERIODS = 3
CHECK_STATUS_WAIT_PERIODS = 5
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30

logger = getLogger(__name__)


@pytest.fixture(scope="module")
def simple_agent():
    agent_name = random_suffix_name("bedrock-test-agent", 32)
    agent_description = "Test agent for e2e testing"
    agent_instruction = "You are a helpful assistant"
    agent_model = "arn:aws:bedrock:us-east-2:807147659905:inference-profile/us.amazon.nova-lite-v1:0"
    agent_role_arn = get_bootstrap_resources().IAMRole.arn
    agent_prompt_temp = "0.7"
    agent_top_p = "0.9"
    agent_max_length = "2048"
    agent_tag_key = "test1"
    agent_tag_value = "value1"

    replacements = REPLACEMENT_VALUES.copy()
    replacements["AGENT_NAME"] = agent_name
    replacements["AGENT_DESCRIPTION"] = agent_description
    replacements["AGENT_INSTRUCTION"] = agent_instruction
    replacements["AGENT_MODEL"] = agent_model
    replacements["AGENT_ROLE_ARN"] = agent_role_arn
    replacements["AGENT_PROMPT_TEMP"] = agent_prompt_temp
    replacements["AGENT_TOP_P"] = agent_top_p
    replacements["AGENT_MAX_LENGTH"] = agent_max_length
    replacements["TAG_KEY_1"] = agent_tag_key
    replacements["TAG_VALUE_1"] = agent_tag_value

    resource_data = load_resource(
        "agent",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP,
        CRD_VERSION,
        AGENT_RESOURCE_PLURAL,
        agent_name,
        namespace="default",
    )

    logger.info("Creating Agent %s", agent_name)
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert k8s.get_resource_exists(ref)
    assert cr is not None
    assert "status" in cr
    assert "agentID" in cr["status"]
    agent_id = cr["status"]["agentID"]

    agent.wait_until_exists(agent_id)
    logger.info("Agent %s exists in cluster", agent_name)

    yield (ref, cr, agent_id)

    logger.info("Deleting Agent %s", agent_name)
    _, deleted = k8s.delete_custom_resource(
        ref,
        wait_periods=DELETE_WAIT_PERIODS,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    agent.wait_until_deleted(agent_id)


@service_marker
@pytest.mark.canary
class TestAgent:
    def test_crud(self, simple_agent):
        ref, res, agent_id = simple_agent

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
        assert cr["spec"]["description"] == "Test agent for e2e testing"

        latest = agent.get(agent_id)
        assert latest is not None
        assert "description" in latest
        assert latest["description"] == "Test agent for e2e testing"

        # Test update
        updates = {
            "spec": {"description": "Updated test agent description"},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        latest = agent.get(agent_id)
        assert latest is not None
        assert "description" in latest
        assert latest["description"] == "Updated test agent description"