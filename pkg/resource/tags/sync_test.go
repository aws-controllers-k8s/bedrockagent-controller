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
	"errors"
	"reflect"
	"testing"

	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagent"
)

// mockTagsClient is a mock implementation of the tagsClient interface
type mockTagsClient struct {
	tagResourceFunc         func(context.Context, *svcsdk.TagResourceInput, ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error)
	listTagsForResourceFunc func(context.Context, *svcsdk.ListTagsForResourceInput, ...func(*svcsdk.Options)) (*svcsdk.ListTagsForResourceOutput, error)
	untagResourceFunc       func(context.Context, *svcsdk.UntagResourceInput, ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error)
}

func (m *mockTagsClient) TagResource(ctx context.Context, input *svcsdk.TagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error) {
	return m.tagResourceFunc(ctx, input, opts...)
}

func (m *mockTagsClient) ListTagsForResource(ctx context.Context, input *svcsdk.ListTagsForResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.ListTagsForResourceOutput, error) {
	return m.listTagsForResourceFunc(ctx, input, opts...)
}

func (m *mockTagsClient) UntagResource(ctx context.Context, input *svcsdk.UntagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error) {
	return m.untagResourceFunc(ctx, input, opts...)
}

// mockMetricsRecorder is a mock implementation of the metricsRecorder interface
type mockMetricsRecorder struct {
	recordAPICallFunc func(opType string, opID string, err error)
}

func (m *mockMetricsRecorder) RecordAPICall(opType string, opID string, err error) {
	m.recordAPICallFunc(opType, opID, err)
}

func TestGetResourceTags(t *testing.T) {
	tests := []struct {
		name        string
		resourceARN string
		mockClient  func() tagsClient
		mockMetrics func() metricsRecorder
		want        map[string]*string
		wantErr     bool
	}{
		{
			name:        "Success case",
			resourceARN: "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent",
			mockClient: func() tagsClient {
				return &mockTagsClient{
					listTagsForResourceFunc: func(ctx context.Context, input *svcsdk.ListTagsForResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.ListTagsForResourceOutput, error) {
						if *input.ResourceArn != "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent" {
							t.Errorf("Expected ResourceArn %s, got %s", "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent", *input.ResourceArn)
						}
						tags := map[string]string{
							"key1": "value1",
							"key2": "value2",
						}
						return &svcsdk.ListTagsForResourceOutput{
							Tags: tags,
						}, nil
					},
				}
			},
			mockMetrics: func() metricsRecorder {
				return &mockMetricsRecorder{
					recordAPICallFunc: func(opType string, opID string, err error) {
						if opType != "GET" || opID != "ListTagsForResource" || err != nil {
							t.Errorf("Expected RecordAPICall(GET, ListTagsForResource, nil), got RecordAPICall(%s, %s, %v)", opType, opID, err)
						}
					},
				}
			},
			want: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantErr: false,
		},
		{
			name:        "Error case",
			resourceARN: "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent",
			mockClient: func() tagsClient {
				return &mockTagsClient{
					listTagsForResourceFunc: func(ctx context.Context, input *svcsdk.ListTagsForResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.ListTagsForResourceOutput, error) {
						return nil, errors.New("API error")
					},
				}
			},
			mockMetrics: func() metricsRecorder {
				return &mockMetricsRecorder{
					recordAPICallFunc: func(opType string, opID string, err error) {
						if opType != "GET" || opID != "ListTagsForResource" || err == nil {
							t.Errorf("Expected RecordAPICall(GET, ListTagsForResource, error), got RecordAPICall(%s, %s, %v)", opType, opID, err)
						}
					},
				}
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.mockClient()
			mr := tt.mockMetrics()
			got, err := GetResourceTags(context.Background(), client, mr, tt.resourceARN)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetResourceTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResourceTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSyncResourceTags(t *testing.T) {
	const resourceARN = "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent"

	tests := []struct {
		name        string
		desiredTags map[string]*string
		latestTags  map[string]*string
		// wantTagged is the Tags expected in TagResource; nil means not called.
		wantTagged map[string]string
		// wantUntagged is the TagKeys expected in UntagResource; nil means not called.
		wantUntagged []string
		// wantCalls is the ordered list of API operations expected to be invoked.
		wantCalls []string
		// failOn names the operation that should return an error, if any.
		failOn  string
		wantErr bool
	}{
		{
			name: "No changes needed",
			desiredTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantCalls: nil,
		},
		{
			name: "Add new tag only",
			desiredTags: map[string]*string{
				"new": stringPtr("v"),
			},
			latestTags: map[string]*string{},
			wantTagged: map[string]string{
				"new": "v",
			},
			wantCalls: []string{"TagResource"},
		},
		{
			name: "Value change only calls TagResource not UntagResource",
			desiredTags: map[string]*string{
				"key1": stringPtr("new-value1"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("old-value1"),
			},
			wantTagged: map[string]string{
				"key1": "new-value1",
			},
			wantCalls: []string{"TagResource"},
		},
		{
			name: "Add and update tags",
			desiredTags: map[string]*string{
				"key1": stringPtr("new-value1"),
				"key3": stringPtr("value3"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantTagged: map[string]string{
				"key1": "new-value1",
				"key3": "value3",
			},
			wantUntagged: []string{"key2"},
			wantCalls:    []string{"UntagResource", "TagResource"},
		},
		{
			name: "Remove multiple tags with sorted keys",
			desiredTags: map[string]*string{
				"keep": stringPtr("value"),
			},
			latestTags: map[string]*string{
				"keep": stringPtr("value"),
				"c":    stringPtr("v"),
				"a":    stringPtr("v"),
				"b":    stringPtr("v"),
			},
			wantUntagged: []string{"a", "b", "c"},
			wantCalls:    []string{"UntagResource"},
		},
		{
			name: "Nil desired value is sent as empty string without panicking",
			desiredTags: map[string]*string{
				"key1": nil,
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
			},
			wantTagged: map[string]string{
				"key1": "",
			},
			wantCalls: []string{"TagResource"},
		},
		{
			name: "Nil values on both sides produce no API calls",
			desiredTags: map[string]*string{
				"key1": nil,
			},
			latestTags: map[string]*string{
				"key1": nil,
			},
			wantCalls: nil,
		},
		{
			name:        "Both empty produce no API calls",
			desiredTags: map[string]*string{},
			latestTags:  map[string]*string{},
			wantCalls:   nil,
		},
		{
			name: "Error in UntagResource aborts pending TagResource",
			desiredTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key3": stringPtr("value3"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantUntagged: []string{"key2"},
			wantCalls:    []string{"UntagResource"},
			failOn:       "UntagResource",
			wantErr:      true,
		},
		{
			name: "Error in TagResource",
			desiredTags: map[string]*string{
				"key1": stringPtr("new-value1"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
			},
			wantTagged: map[string]string{
				"key1": "new-value1",
			},
			wantCalls: []string{"TagResource"},
			failOn:    "TagResource",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotTagInput *svcsdk.TagResourceInput
			var gotUntagInput *svcsdk.UntagResourceInput
			var gotCalls []string
			var gotMetricOps []string

			client := &mockTagsClient{
				tagResourceFunc: func(ctx context.Context, input *svcsdk.TagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error) {
					gotCalls = append(gotCalls, "TagResource")
					gotTagInput = input
					if tt.failOn == "TagResource" {
						return nil, errors.New("API error")
					}
					return &svcsdk.TagResourceOutput{}, nil
				},
				untagResourceFunc: func(ctx context.Context, input *svcsdk.UntagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error) {
					gotCalls = append(gotCalls, "UntagResource")
					gotUntagInput = input
					if tt.failOn == "UntagResource" {
						return nil, errors.New("API error")
					}
					return &svcsdk.UntagResourceOutput{}, nil
				},
			}
			mr := &mockMetricsRecorder{
				recordAPICallFunc: func(opType string, opID string, err error) {
					gotMetricOps = append(gotMetricOps, opID)
					if opType != "UPDATE" {
						t.Errorf("Expected opType UPDATE, got %s", opType)
					}
					if (err != nil) != (opID == tt.failOn) {
						t.Errorf("RecordAPICall(%s) error = %v, failOn = %q", opID, err, tt.failOn)
					}
				},
			}

			err := SyncResourceTags(context.Background(), client, mr, resourceARN, tt.desiredTags, tt.latestTags)
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncResourceTags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(gotCalls, tt.wantCalls) {
				t.Errorf("API calls = %v, want %v", gotCalls, tt.wantCalls)
			}
			if !reflect.DeepEqual(gotMetricOps, tt.wantCalls) {
				t.Errorf("RecordAPICall ops = %v, want %v", gotMetricOps, tt.wantCalls)
			}

			if tt.wantTagged == nil {
				if gotTagInput != nil {
					t.Errorf("TagResource should not be called, got %v", gotTagInput.Tags)
				}
			} else if gotTagInput == nil {
				t.Errorf("TagResource was not called, want tags %v", tt.wantTagged)
			} else {
				if *gotTagInput.ResourceArn != resourceARN {
					t.Errorf("Expected ResourceArn %s, got %s", resourceARN, *gotTagInput.ResourceArn)
				}
				if !reflect.DeepEqual(gotTagInput.Tags, tt.wantTagged) {
					t.Errorf("TagResource tags = %v, want %v", gotTagInput.Tags, tt.wantTagged)
				}
			}

			if tt.wantUntagged == nil {
				if gotUntagInput != nil {
					t.Errorf("UntagResource should not be called, got %v", gotUntagInput.TagKeys)
				}
			} else if gotUntagInput == nil {
				t.Errorf("UntagResource was not called, want keys %v", tt.wantUntagged)
			} else {
				if *gotUntagInput.ResourceArn != resourceARN {
					t.Errorf("Expected ResourceArn %s, got %s", resourceARN, *gotUntagInput.ResourceArn)
				}
				if !reflect.DeepEqual(gotUntagInput.TagKeys, tt.wantUntagged) {
					t.Errorf("UntagResource keys = %v, want %v", gotUntagInput.TagKeys, tt.wantUntagged)
				}
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
