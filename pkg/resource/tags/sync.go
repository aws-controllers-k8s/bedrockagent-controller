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

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
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

	addedOrUpdated, removed := computeTagsDelta(desiredTags, latestTags)

	if len(removed) > 0 {
		_, err = client.UntagResource(
			ctx,
			&svcsdk.UntagResourceInput{
				ResourceArn: &resourceARN,
				TagKeys:     removed,
			},
		)
		mr.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(addedOrUpdated) > 0 {
		_, err = client.TagResource(
			ctx,
			&svcsdk.TagResourceInput{
				ResourceArn: &resourceARN,
				Tags:        addedOrUpdated,
			},
		)
		mr.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}
	return nil
}

// computeTagsDelta compares two Tag arrays and return two different list
// containing the addedOrupdated and removed tags. The removed tags array
// only contains the tags Keys.
func computeTagsDelta(
	a map[string]*string,
	b map[string]*string,
) (addedOrUpdated map[string]string, removed []string) {

	// Find the keys in the Spec have either been added or updated.
	for key, value := range a {
		if bValue, exists := b[key]; !exists || *value != *bValue {
			if addedOrUpdated == nil {
				addedOrUpdated = make(map[string]string)
			}
			addedOrUpdated[key] = *value
		}
	}

	for key := range b {
		if _, exists := a[key]; !exists {
			removed = append(removed, key)
		}
	}

	return addedOrUpdated, removed
}

// equalTags returns true if two Tag arrays are equal regardless of the order
// of their elements.
func EqualTags(
	a map[string]*string,
	b map[string]*string,
) bool {
	addedOrUpdated, removed := computeTagsDelta(a, b)
	return len(addedOrUpdated) == 0 && len(removed) == 0
}
