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
	tests := []struct {
		name        string
		resourceARN string
		desiredTags map[string]*string
		latestTags  map[string]*string
		mockClient  func() tagsClient
		mockMetrics func() metricsRecorder
		wantErr     bool
	}{
		{
			name:        "No changes needed",
			resourceARN: "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent",
			desiredTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			mockClient: func() tagsClient {
				return &mockTagsClient{
					tagResourceFunc: func(ctx context.Context, input *svcsdk.TagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error) {
						t.Error("TagResource should not be called")
						return nil, nil
					},
					untagResourceFunc: func(ctx context.Context, input *svcsdk.UntagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error) {
						t.Error("UntagResource should not be called")
						return nil, nil
					},
				}
			},
			mockMetrics: func() metricsRecorder {
				return &mockMetricsRecorder{
					recordAPICallFunc: func(opType string, opID string, err error) {
						// No API calls should be recorded
					},
				}
			},
			wantErr: false,
		},
		{
			name:        "Add and update tags",
			resourceARN: "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent",
			desiredTags: map[string]*string{
				"key1": stringPtr("new-value1"),
				"key3": stringPtr("value3"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			mockClient: func() tagsClient {
				return &mockTagsClient{
					tagResourceFunc: func(ctx context.Context, input *svcsdk.TagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error) {
						if *input.ResourceArn != "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent" {
							t.Errorf("Expected ResourceArn %s, got %s", "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent", *input.ResourceArn)
						}
						if len(input.Tags) != 2 {
							t.Errorf("Expected 2 tags, got %d", len(input.Tags))
						}
						if input.Tags["key1"] != "new-value1" {
							t.Errorf("Expected key1=new-value1, got key1=%s", input.Tags["key1"])
						}
						if input.Tags["key3"] != "value3" {
							t.Errorf("Expected key3=value3, got key3=%s", input.Tags["key3"])
						}
						return &svcsdk.TagResourceOutput{}, nil
					},
					untagResourceFunc: func(ctx context.Context, input *svcsdk.UntagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error) {
						if *input.ResourceArn != "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent" {
							t.Errorf("Expected ResourceArn %s, got %s", "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent", *input.ResourceArn)
						}
						if len(input.TagKeys) != 1 {
							t.Errorf("Expected 1 tag key, got %d", len(input.TagKeys))
						}
						if input.TagKeys[0] != "key2" {
							t.Errorf("Expected key2 to be removed, got %s", input.TagKeys[0])
						}
						return &svcsdk.UntagResourceOutput{}, nil
					},
				}
			},
			mockMetrics: func() metricsRecorder {
				callCount := 0
				return &mockMetricsRecorder{
					recordAPICallFunc: func(opType string, opID string, err error) {
						callCount++
						if callCount == 1 {
							if opType != "UPDATE" || opID != "UntagResource" || err != nil {
								t.Errorf("Expected RecordAPICall(UPDATE, UntagResource, nil), got RecordAPICall(%s, %s, %v)", opType, opID, err)
							}
						} else if callCount == 2 {
							if opType != "UPDATE" || opID != "TagResource" || err != nil {
								t.Errorf("Expected RecordAPICall(UPDATE, TagResource, nil), got RecordAPICall(%s, %s, %v)", opType, opID, err)
							}
						}
					},
				}
			},
			wantErr: false,
		},
		{
			name:        "Error in UntagResource",
			resourceARN: "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent",
			desiredTags: map[string]*string{
				"key1": stringPtr("value1"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			mockClient: func() tagsClient {
				return &mockTagsClient{
					untagResourceFunc: func(ctx context.Context, input *svcsdk.UntagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error) {
						return nil, errors.New("API error")
					},
				}
			},
			mockMetrics: func() metricsRecorder {
				return &mockMetricsRecorder{
					recordAPICallFunc: func(opType string, opID string, err error) {
						if opType != "UPDATE" || opID != "UntagResource" || err == nil {
							t.Errorf("Expected RecordAPICall(UPDATE, UntagResource, error), got RecordAPICall(%s, %s, %v)", opType, opID, err)
						}
					},
				}
			},
			wantErr: true,
		},
		{
			name:        "Error in TagResource",
			resourceARN: "arn:aws:bedrock:us-west-2:123456789012:agent/test-agent",
			desiredTags: map[string]*string{
				"key1": stringPtr("new-value1"),
			},
			latestTags: map[string]*string{
				"key1": stringPtr("value1"),
			},
			mockClient: func() tagsClient {
				return &mockTagsClient{
					tagResourceFunc: func(ctx context.Context, input *svcsdk.TagResourceInput, opts ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error) {
						return nil, errors.New("API error")
					},
				}
			},
			mockMetrics: func() metricsRecorder {
				return &mockMetricsRecorder{
					recordAPICallFunc: func(opType string, opID string, err error) {
						if opType != "UPDATE" || opID != "TagResource" || err == nil {
							t.Errorf("Expected RecordAPICall(UPDATE, TagResource, error), got RecordAPICall(%s, %s, %v)", opType, opID, err)
						}
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.mockClient()
			mr := tt.mockMetrics()
			err := SyncResourceTags(context.Background(), client, mr, tt.resourceARN, tt.desiredTags, tt.latestTags)
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncResourceTags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeTagsDelta(t *testing.T) {
	tests := []struct {
		name             string
		a                map[string]*string
		b                map[string]*string
		wantAddOrUpdate  map[string]string
		wantRemoved      []string
	}{
		{
			name: "No changes",
			a: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantAddOrUpdate: nil,
			wantRemoved:     nil,
		},
		{
			name: "Add new tags",
			a: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
				"key3": stringPtr("value3"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
			},
			wantAddOrUpdate: map[string]string{
				"key2": "value2",
				"key3": "value3",
			},
			wantRemoved: nil,
		},
		{
			name: "Remove tags",
			a: map[string]*string{
				"key1": stringPtr("value1"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
				"key3": stringPtr("value3"),
			},
			wantAddOrUpdate: nil,
			wantRemoved:     []string{"key2", "key3"},
		},
		{
			name: "Update existing tags",
			a: map[string]*string{
				"key1": stringPtr("new-value1"),
				"key2": stringPtr("value2"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantAddOrUpdate: map[string]string{
				"key1": "new-value1",
			},
			wantRemoved: nil,
		},
		{
			name: "Add, update, and remove tags",
			a: map[string]*string{
				"key1": stringPtr("new-value1"),
				"key3": stringPtr("value3"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			wantAddOrUpdate: map[string]string{
				"key1": "new-value1",
				"key3": "value3",
			},
			wantRemoved: []string{"key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddOrUpdate, gotRemoved := computeTagsDelta(tt.a, tt.b)
			
			// Check added or updated tags
			if !reflect.DeepEqual(gotAddOrUpdate, tt.wantAddOrUpdate) {
				t.Errorf("computeTagsDelta() addedOrUpdated = %v, want %v", gotAddOrUpdate, tt.wantAddOrUpdate)
			}
			
			// For removed tags, we need to check if all expected keys are present
			// regardless of order
			if len(gotRemoved) != len(tt.wantRemoved) {
				t.Errorf("computeTagsDelta() removed length = %d, want %d", len(gotRemoved), len(tt.wantRemoved))
			} else {
				removedMap := make(map[string]bool)
				for _, key := range gotRemoved {
					removedMap[key] = true
				}
				
				for _, key := range tt.wantRemoved {
					if !removedMap[key] {
						t.Errorf("computeTagsDelta() removed does not contain key %s", key)
					}
				}
			}
		})
	}
}

func TestEqualTags(t *testing.T) {
	tests := []struct {
		name string
		a    map[string]*string
		b    map[string]*string
		want bool
	}{
		{
			name: "Equal tags",
			a: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			want: true,
		},
		{
			name: "Different values",
			a: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("different"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			want: false,
		},
		{
			name: "Different keys",
			a: map[string]*string{
				"key1": stringPtr("value1"),
				"key3": stringPtr("value3"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			want: false,
		},
		{
			name: "Different number of tags",
			a: map[string]*string{
				"key1": stringPtr("value1"),
			},
			b: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
			},
			want: false,
		},
		{
			name: "Both empty",
			a:    map[string]*string{},
			b:    map[string]*string{},
			want: true,
		},
		{
			name: "One empty, one not",
			a:    map[string]*string{},
			b: map[string]*string{
				"key1": stringPtr("value1"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EqualTags(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("EqualTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}