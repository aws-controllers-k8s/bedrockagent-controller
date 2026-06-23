package knowledge_base

import (
	"context"

	"github.com/aws-controllers-k8s/bedrockagent-controller/pkg/resource/tags"
)

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
