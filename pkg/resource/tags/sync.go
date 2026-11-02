// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package tags

import (
	"context"
	"maps"
	"slices"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	acktags "github.com/aws-controllers-k8s/runtime/pkg/tags"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagent"
)

type metricsRecorder interface {
	RecordAPICall(opType string, opID string, err error)
}

type tagsClient interface {
	TagResource(context.Context, *svcsdk.TagResourceInput, ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error)
	ListTagsForResource(context.Context, *svcsdk.ListTagsForResourceInput, ...func(*svcsdk.Options)) (*svcsdk.ListTagsForResourceOutput, error)
	UntagResource(context.Context, *svcsdk.UntagResourceInput, ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error)
}

// GetResourceTags retrieves a resource list of tags.
func GetResourceTags(
	ctx context.Context,
	client tagsClient,
	mr metricsRecorder,
	resourceARN string,
) (map[string]*string, error) {
	listTagsForResourceResponse, err := client.ListTagsForResource(
		ctx,
		&svcsdk.ListTagsForResourceInput{
			ResourceArn: &resourceARN,
		},
	)
	mr.RecordAPICall("GET", "ListTagsForResource", err)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]*string)
	for key, value := range listTagsForResourceResponse.Tags {
		tags[key] = &value
	}

	return tags, nil
}

// SyncResourceTags uses TagResource and UntagResource API Calls to add, remove
// and update resource tags.
func SyncResourceTags(
	ctx context.Context,
	client tagsClient,
	mr metricsRecorder,
	resourceARN string,
	desiredTags map[string]*string,
	latestTags map[string]*string,
) error {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("common.SyncResourceTags")
	defer func() {
		exit(err)
	}()

	from := toACKTags(latestTags)
	to := toACKTags(desiredTags)
	added, _, removed := ackcompare.GetTagsDifference(from, to)

	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	if len(removed) > 0 {
		_, err = client.UntagResource(
			ctx,
			&svcsdk.UntagResourceInput{
				ResourceArn: &resourceARN,
				TagKeys:     slices.Sorted(maps.Keys(removed)),
			},
		)
		mr.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(added) > 0 {
		_, err = client.TagResource(
			ctx,
			&svcsdk.TagResourceInput{
				ResourceArn: &resourceARN,
				Tags:        added,
			},
		)
		mr.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}
	return nil
}

// toACKTags converts map-based tags into acktags.Tags, treating nil values as
// empty strings.
func toACKTags(tags map[string]*string) acktags.Tags {
	m := make(acktags.Tags)
	for k, v := range tags {
		val := ""
		if v != nil {
			val = *v
		}
		m[k] = val
	}
	return m
}
