package agent

import (
	"context"
	"reflect"

	"github.com/aws-controllers-k8s/bedrockagent-controller/apis/v1alpha1"
	"github.com/aws-controllers-k8s/bedrockagent-controller/pkg/resource/tags"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagent"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagent/types"
)

type metricsRecorder interface {
	RecordAPICall(opType string, opID string, err error)
}

type agentClient interface {
	PrepareAgent(context.Context, *svcsdk.PrepareAgentInput, ...func(*svcsdk.Options)) (*svcsdk.PrepareAgentOutput, error)
}

// prepareAgent makes a request to the PrepareAgent operation to
// move the Agent into the PREPARED state.
func prepareAgent(
	ctx context.Context,
	client agentClient,
	metrics metricsRecorder,
	agentId string,
) error {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("hooks.prepareAgent")
	defer func() {
		exit(err)
	}()

	_, err = client.PrepareAgent(ctx, &svcsdk.PrepareAgentInput{
		AgentId: &agentId,
	})
	metrics.RecordAPICall("UPDATE", "PREPARE_AGENT", err)

	return err
}

// compareAgentStatus checks if the latest AgentStatus is in the PREPARED state.
// If not a virtual spec field Spec.AgentStatus is added to the delta.
func compareAgentStatus(
	delta *ackcompare.Delta,
	latestStatus *string,
) {
	if latestStatus != nil && *latestStatus != string(svcsdktypes.AgentAliasStatusPrepared) {
		delta.Add("Spec.AgentStatus", latestStatus, string(svcsdktypes.AgentAliasStatusPrepared))
	}
}

// comparePropertyOverrideConfiguration compares delta of Spec.PromptOverrideConfiguration between two resources.
// If PromptOverrideConfiguration is not set for the desired resource no delta is set. This is to prevent errors when
// AWS has set defaults that are not considered valid by UpdateAgent.
func comparePropertyOverrideConfiguration(
	delta *ackcompare.Delta,
	desired *resource,
	latest *resource,
) {
	if ackcompare.HasNilDifference(desired.ko.Spec.PromptOverrideConfiguration, latest.ko.Spec.PromptOverrideConfiguration) {
		delta.Add("Spec.PromptOverrideConfiguration", desired.ko.Spec.PromptOverrideConfiguration, latest.ko.Spec.PromptOverrideConfiguration)
	} else if desired.ko.Spec.PromptOverrideConfiguration != nil && latest.ko.Spec.PromptOverrideConfiguration != nil {
		if ackcompare.HasNilDifference(desired.ko.Spec.PromptOverrideConfiguration.OverrideLambda, latest.ko.Spec.PromptOverrideConfiguration.OverrideLambda) {
			delta.Add("Spec.PromptOverrideConfiguration.OverrideLambda", desired.ko.Spec.PromptOverrideConfiguration.OverrideLambda, latest.ko.Spec.PromptOverrideConfiguration.OverrideLambda)
		} else if desired.ko.Spec.PromptOverrideConfiguration.OverrideLambda != nil && latest.ko.Spec.PromptOverrideConfiguration.OverrideLambda != nil {
			if *desired.ko.Spec.PromptOverrideConfiguration.OverrideLambda != *latest.ko.Spec.PromptOverrideConfiguration.OverrideLambda {
				delta.Add("Spec.PromptOverrideConfiguration.OverrideLambda", desired.ko.Spec.PromptOverrideConfiguration.OverrideLambda, latest.ko.Spec.PromptOverrideConfiguration.OverrideLambda)
			}
		}

		var desiredNonDefaultPromptConfigs []*v1alpha1.PromptConfiguration
		for _, promptConfig := range desired.ko.Spec.PromptOverrideConfiguration.PromptConfigurations {
			if promptConfig != nil && *promptConfig.PromptCreationMode != "DEFAULT" {
				desiredNonDefaultPromptConfigs = append(desiredNonDefaultPromptConfigs, promptConfig)
			}
		}

		var latestNonDefaultPromptConfigs []*v1alpha1.PromptConfiguration
		for _, promptConfig := range latest.ko.Spec.PromptOverrideConfiguration.PromptConfigurations {
			if promptConfig != nil && *promptConfig.PromptCreationMode != "DEFAULT" {
				latestNonDefaultPromptConfigs = append(latestNonDefaultPromptConfigs, promptConfig)
			}
		}

		if len(desiredNonDefaultPromptConfigs) != len(latestNonDefaultPromptConfigs) {
			delta.Add("Spec.PromptOverrideConfiguration.PromptConfigurations", desired.ko.Spec.PromptOverrideConfiguration.PromptConfigurations, latest.ko.Spec.PromptOverrideConfiguration.PromptConfigurations)
		} else if len(desiredNonDefaultPromptConfigs) > 0 {
			if !reflect.DeepEqual(desiredNonDefaultPromptConfigs, latestNonDefaultPromptConfigs) {
				delta.Add("Spec.PromptOverrideConfiguration.PromptConfigurations", desired.ko.Spec.PromptOverrideConfiguration.PromptConfigurations, latest.ko.Spec.PromptOverrideConfiguration.PromptConfigurations)
			}
		}
	}
}

// getTags retrieves the resource's associated tags.
func (rm *resourceManager) getTags(
	ctx context.Context,
	resourceARN string,
) (map[string]*string, error) {
	return tags.GetResourceTags(ctx, rm.sdkapi, rm.metrics, resourceARN)
}

// syncTags keeps the resource's tags in sync.
func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	return tags.SyncResourceTags(
		ctx,
		rm.sdkapi,
		rm.metrics,
		string(*latest.ko.Status.ACKResourceMetadata.ARN),
		desired.ko.Spec.Tags,
		latest.ko.Spec.Tags,
	)
}
