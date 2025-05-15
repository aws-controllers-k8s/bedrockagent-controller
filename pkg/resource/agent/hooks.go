package agent

import (
	"context"

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

	return nil
}

func compareAgentStatus(
	latestStatus *string,
	delta *ackcompare.Delta,
) {
	if latestStatus != nil && *latestStatus != string(svcsdktypes.AgentAliasStatusPrepared) {
		delta.Add("Spec.AgentStatus", latestStatus, string(svcsdktypes.AgentAliasStatusPrepared))
	}
}
