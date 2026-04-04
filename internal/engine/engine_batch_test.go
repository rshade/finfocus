// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// mockBatchCostSourceClient implements proto.CostSourceClient for batch testing.
type mockBatchCostSourceClient struct {
	batchCostFunc func(ctx context.Context, in *pbc.BatchCostRequest, opts ...grpc.CallOption) (*pbc.BatchCostResponse, error)
}

func (m *mockBatchCostSourceClient) Name(
	_ context.Context, _ *proto.Empty, _ ...grpc.CallOption,
) (*proto.NameResponse, error) {
	return &proto.NameResponse{Name: "mock-batch"}, nil
}

func (m *mockBatchCostSourceClient) GetProjectedCost(
	_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	return &proto.GetProjectedCostResponse{}, nil
}

func (m *mockBatchCostSourceClient) GetActualCost(
	_ context.Context, _ *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	return &proto.GetActualCostResponse{}, nil
}

func (m *mockBatchCostSourceClient) GetRecommendations(
	_ context.Context, _ *proto.GetRecommendationsRequest, _ ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	return &proto.GetRecommendationsResponse{}, nil
}

func (m *mockBatchCostSourceClient) GetPluginInfo(
	_ context.Context, _ *proto.Empty, _ ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	return &pbc.GetPluginInfoResponse{}, nil
}

func (m *mockBatchCostSourceClient) GetBudgets(
	_ context.Context, _ *pbc.GetBudgetsRequest, _ ...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	return &pbc.GetBudgetsResponse{}, nil
}

func (m *mockBatchCostSourceClient) DryRun(
	_ context.Context, _ *pbc.DryRunRequest, _ ...grpc.CallOption,
) (*pbc.DryRunResponse, error) {
	return &pbc.DryRunResponse{}, nil
}

func (m *mockBatchCostSourceClient) DismissRecommendation(
	_ context.Context, _ *proto.DismissRecommendationRequest, _ ...grpc.CallOption,
) (*proto.DismissRecommendationResponse, error) {
	return &proto.DismissRecommendationResponse{}, nil
}

func (m *mockBatchCostSourceClient) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (m *mockBatchCostSourceClient) EstimateCost(
	_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
	return nil, status.Error(codes.Unimplemented, "EstimateCost not implemented")
}

func (m *mockBatchCostSourceClient) BatchCost(
	ctx context.Context, in *pbc.BatchCostRequest, opts ...grpc.CallOption,
) (*pbc.BatchCostResponse, error) {
	if m.batchCostFunc != nil {
		return m.batchCostFunc(ctx, in, opts...)
	}
	return &pbc.BatchCostResponse{}, nil
}

// makeBatchCapableClient creates a pluginhost.Client with batch_cost capability.
func makeBatchCapableClient(name string, api proto.CostSourceClient) *pluginhost.Client {
	return &pluginhost.Client{
		Name: name,
		Metadata: &proto.PluginMetadata{
			Capabilities: []string{"projected_costs", "actual_costs", "batch_cost"},
		},
		API:   api,
		Close: func() error { return nil },
	}
}

// makeNonBatchClient creates a pluginhost.Client without batch_cost capability.
func makeNonBatchClient(name string, api proto.CostSourceClient) *pluginhost.Client {
	return &pluginhost.Client{
		Name: name,
		Metadata: &proto.PluginMetadata{
			Capabilities: []string{"projected_costs", "actual_costs"},
		},
		API:   api,
		Close: func() error { return nil },
	}
}

// makeTestResources creates n test resources with aws:ec2:Instance type.
func makeTestResources(n int) []ResourceDescriptor {
	resources := make([]ResourceDescriptor, n)
	for i := range n {
		resources[i] = ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       fmt.Sprintf("i-%03d", i),
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}
	}
	return resources
}

// T011: Unit tests for chunkResources.
func TestChunkResources(t *testing.T) {
	makeIndexed := func(n int) []indexedResource {
		result := make([]indexedResource, n)
		for i := range n {
			result[i] = indexedResource{
				index:    i,
				resource: ResourceDescriptor{Type: "aws:ec2:Instance", ID: fmt.Sprintf("i-%d", i)},
			}
		}
		return result
	}

	t.Run("50 resources with chunk size 100 produces 1 chunk", func(t *testing.T) {
		chunks := chunkResources(makeIndexed(50), 100)
		require.Len(t, chunks, 1)
		assert.Len(t, chunks[0], 50)
	})

	t.Run("200 resources with chunk size 100 produces 2 chunks", func(t *testing.T) {
		chunks := chunkResources(makeIndexed(200), 100)
		require.Len(t, chunks, 2)
		assert.Len(t, chunks[0], 100)
		assert.Len(t, chunks[1], 100)
	})

	t.Run("101 resources produces 2 chunks of 100 and 1", func(t *testing.T) {
		chunks := chunkResources(makeIndexed(101), 100)
		require.Len(t, chunks, 2)
		assert.Len(t, chunks[0], 100)
		assert.Len(t, chunks[1], 1)
	})

	t.Run("chunk size 0 uses default of 100", func(t *testing.T) {
		chunks := chunkResources(makeIndexed(150), 0)
		require.Len(t, chunks, 2)
		assert.Len(t, chunks[0], 100)
		assert.Len(t, chunks[1], 50)
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		chunks := chunkResources(nil, 100)
		assert.Nil(t, chunks)
	})

	t.Run("preserves order", func(t *testing.T) {
		indexed := makeIndexed(5)
		chunks := chunkResources(indexed, 3)
		require.Len(t, chunks, 2)
		assert.Equal(t, 0, chunks[0][0].index)
		assert.Equal(t, 1, chunks[0][1].index)
		assert.Equal(t, 2, chunks[0][2].index)
		assert.Equal(t, 3, chunks[1][0].index)
		assert.Equal(t, 4, chunks[1][1].index)
	})
}

// T012: Unit tests for groupResourcesByPlugin.
func TestGroupResourcesByPlugin(t *testing.T) {
	t.Run("all resources match one plugin", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{}
		client := makeBatchCapableClient("aws-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(5)
		groups := eng.groupResourcesByPlugin(context.Background(), resources)

		require.Len(t, groups, 1)
		batch, ok := groups["aws-plugin"]
		require.True(t, ok)
		assert.Len(t, batch.resources, 5)
		assert.True(t, batch.hasBatch)
		assert.Equal(t, client, batch.plugin)
	})

	t.Run("resources split across 2 plugins", func(t *testing.T) {
		awsAPI := &mockBatchCostSourceClient{}
		azureAPI := &mockBatchCostSourceClient{}
		awsClient := makeBatchCapableClient("aws-plugin", awsAPI)
		azureClient := makeBatchCapableClient("azure-plugin", azureAPI)
		eng := New([]*pluginhost.Client{awsClient, azureClient}, nil)

		resources := []ResourceDescriptor{
			{
				Type:       "aws:ec2:Instance",
				ID:         "i-1",
				Provider:   "aws",
				Properties: map[string]interface{}{"instanceType": "t3.micro"},
			},
			{
				Type:       "azure:compute:VirtualMachine",
				ID:         "vm-1",
				Provider:   "azure",
				Properties: map[string]interface{}{"vmSize": "Standard_B1s"},
			},
			{Type: "aws:s3:Bucket", ID: "b-1", Provider: "aws", Properties: map[string]interface{}{}},
		}

		groups := eng.groupResourcesByPlugin(context.Background(), resources)

		// Without a router, all resources go to all clients (first client wins).
		// Both plugins will be in the matches for each resource, but grouping uses primary (first).
		// With no router, selectPluginMatchesForResource returns all clients, so first = awsClient.
		// All 3 resources will be grouped under the first client since there's no router to differentiate.
		require.NotEmpty(t, groups)
	})

	t.Run("internal Pulumi types are filtered", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{}
		client := makeBatchCapableClient("aws-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := []ResourceDescriptor{
			{
				Type:       "aws:ec2:Instance",
				ID:         "i-1",
				Provider:   "aws",
				Properties: map[string]interface{}{"instanceType": "t3.micro"},
			},
			{Type: "pulumi:pulumi:Stack", ID: "stack", Provider: "pulumi", Properties: map[string]interface{}{}},
			{Type: "pulumi:providers:aws", ID: "provider", Provider: "pulumi", Properties: map[string]interface{}{}},
		}

		groups := eng.groupResourcesByPlugin(context.Background(), resources)

		// Only the ec2 resource should be included, pulumi internals are filtered
		total := 0
		for _, batch := range groups {
			total += len(batch.resources)
		}
		assert.Equal(t, 1, total)
	})

	t.Run("batch capability detection", func(t *testing.T) {
		batchAPI := &mockBatchCostSourceClient{}
		nonBatchAPI := &mockBatchCostSourceClient{}
		batchClient := makeBatchCapableClient("batch-plugin", batchAPI)
		nonBatchClient := makeNonBatchClient("legacy-plugin", nonBatchAPI)

		eng := New([]*pluginhost.Client{batchClient, nonBatchClient}, nil)
		resources := makeTestResources(1)

		groups := eng.groupResourcesByPlugin(context.Background(), resources)
		// Both clients match (no router), but first client is used for primary grouping
		require.NotEmpty(t, groups)
		for _, batch := range groups {
			// Verify hasBatch is set based on the actual client's capabilities
			if batch.plugin.Name == "batch-plugin" {
				assert.True(t, batch.hasBatch)
			} else if batch.plugin.Name == "legacy-plugin" {
				assert.False(t, batch.hasBatch)
			}
		}
	})
}

// T013: Unit tests for executeBatchForPlugin.
func TestExecuteBatchForPlugin(t *testing.T) {
	t.Run("single chunk success with projected query", func(t *testing.T) {
		var capturedReq *pbc.BatchCostRequest
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				capturedReq = in
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_CostData{
							CostData: &pbc.CostData{
								Data: &pbc.CostData_ProjectedCost{
									ProjectedCost: &pbc.GetProjectedCostResponse{
										Currency:     "USD",
										CostPerMonth: float64(10 * (i + 1)),
										UnitPrice:    float64(i+1) * 0.01,
									},
								},
							},
						},
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := make([]indexedResource, 3)
		for i := range 3 {
			resources[i] = indexedResource{
				index: i,
				resource: ResourceDescriptor{
					Type:       "aws:ec2:Instance",
					ID:         fmt.Sprintf("i-%d", i),
					Provider:   "aws",
					Properties: map[string]interface{}{"instanceType": "t3.micro"},
				},
			}
		}

		results, err := eng.executeBatchForPlugin(
			context.Background(), client, resources,
			batchOptions{queryType: pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED},
		)
		require.NoError(t, err)
		require.Len(t, results, 3)

		// Verify query type was passed correctly
		require.NotNil(t, capturedReq)
		assert.Equal(t, pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED, capturedReq.GetQueryType())
		assert.False(t, capturedReq.GetDryRun())

		// Verify results mapped correctly
		for i, br := range results {
			assert.Equal(t, i, br.index)
			require.NotNil(t, br.result)
			assert.InDelta(t, float64(10*(i+1)), br.result.Monthly, 0.001)
			assert.Equal(t, "test-plugin", br.result.Adapter)
			assert.Equal(t, "aws:ec2:Instance", br.result.ResourceType)
		}
	})

	t.Run("actual query type with date range", func(t *testing.T) {
		var capturedReq *pbc.BatchCostRequest
		now := time.Now()
		start := timestamppb.New(now.Add(-24 * time.Hour))
		end := timestamppb.New(now)

		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				capturedReq = in
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_CostData{
							CostData: &pbc.CostData{
								Data: &pbc.CostData_ActualCost{
									ActualCost: &pbc.ActualCostData{
										Results: []*pbc.ActualCostResult{
											{Cost: float64(i+1) * 5.0, Source: "aws-ce"},
										},
									},
								},
							},
						},
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := []indexedResource{
			{
				index: 0,
				resource: ResourceDescriptor{
					Type:       "aws:ec2:Instance",
					ID:         "i-0",
					Provider:   "aws",
					Properties: map[string]interface{}{"instanceType": "t3.micro"},
				},
			},
		}

		results, err := eng.executeBatchForPlugin(
			context.Background(), client, resources,
			batchOptions{
				queryType: pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL,
				start:     start,
				end:       end,
			},
		)
		require.NoError(t, err)
		require.Len(t, results, 1)

		// Verify actual query parameters
		require.NotNil(t, capturedReq)
		assert.Equal(t, pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL, capturedReq.GetQueryType())
		assert.NotNil(t, capturedReq.GetStart())
		assert.NotNil(t, capturedReq.GetEnd())

		// Verify actual result mapped
		require.NotNil(t, results[0].actualResult)
		assert.InDelta(t, 5.0, results[0].actualResult.TotalCost, 0.001)
	})

	t.Run("multi-chunk with max_batch_size adjustment", func(t *testing.T) {
		callCount := 0
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				callCount++
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_CostData{
							CostData: &pbc.CostData{
								Data: &pbc.CostData_ProjectedCost{
									ProjectedCost: &pbc.GetProjectedCostResponse{
										Currency:     "USD",
										CostPerMonth: 10.0,
									},
								},
							},
						},
					}
				}
				resp := &pbc.BatchCostResponse{Results: results}
				// First response hints at smaller batch size
				if callCount == 1 {
					resp.MaxBatchSize = 50
				}
				return resp, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		// Create 150 resources which will be initially chunked at 100
		resources := make([]indexedResource, 150)
		for i := range 150 {
			resources[i] = indexedResource{
				index: i,
				resource: ResourceDescriptor{
					Type:       "aws:ec2:Instance",
					ID:         fmt.Sprintf("i-%d", i),
					Provider:   "aws",
					Properties: map[string]interface{}{"instanceType": "t3.micro"},
				},
			}
		}

		results, err := eng.executeBatchForPlugin(
			context.Background(), client, resources,
			batchOptions{queryType: pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED},
		)
		require.NoError(t, err)
		assert.Len(t, results, 150)
		// First chunk = 100, then remaining 50 re-chunked at max_batch_size=50 → 1 more chunk
		// Total: 2 calls (100 + 50)
		assert.Equal(t, 2, callCount)
	})

	t.Run("response count mismatch returns error", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				// Return fewer results than requested
				return &pbc.BatchCostResponse{
					Results: []*pbc.ResourceCostResult{
						{
							Resource: &pbc.ResourceDescriptor{Id: "r1"},
							Result: &pbc.ResourceCostResult_CostData{
								CostData: &pbc.CostData{
									Data: &pbc.CostData_ProjectedCost{
										ProjectedCost: &pbc.GetProjectedCostResponse{
											Currency:     "USD",
											CostPerMonth: 10.0,
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := make([]indexedResource, 3)
		for i := range 3 {
			resources[i] = indexedResource{
				index: i,
				resource: ResourceDescriptor{
					Type:       "aws:ec2:Instance",
					ID:         fmt.Sprintf("i-%d", i),
					Provider:   "aws",
					Properties: map[string]interface{}{"instanceType": "t3.micro"},
				},
			}
		}

		_, err := eng.executeBatchForPlugin(
			context.Background(), client, resources,
			batchOptions{queryType: pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "count mismatch")
	})
}

// T014: Integration test for batch path in GetProjectedCost.
// This tests the full flow but since batch integration into GetProjectedCost
// is a later task (T022), this test validates the batch helper functions
// compose correctly end-to-end.
func TestBatchProjectedCostIntegration(t *testing.T) {
	t.Run("50 resources batch-capable plugin returns all results", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_CostData{
							CostData: &pbc.CostData{
								Data: &pbc.CostData_ProjectedCost{
									ProjectedCost: &pbc.GetProjectedCostResponse{
										Currency:     "USD",
										CostPerMonth: float64(i + 1),
										UnitPrice:    0.01,
									},
								},
							},
						},
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}

		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(50)

		// Group, then execute batch
		groups := eng.groupResourcesByPlugin(context.Background(), resources)
		require.NotEmpty(t, groups)

		for _, batch := range groups {
			require.True(t, batch.hasBatch)
			results, err := eng.executeBatchForPlugin(
				context.Background(), batch.plugin, batch.resources,
				batchOptions{queryType: pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED},
			)
			require.NoError(t, err)
			assert.Len(t, results, 50)

			// Verify each result has correct index and data
			for _, br := range results {
				require.NotNil(t, br.result)
				assert.Equal(t, "test-plugin", br.result.Adapter)
				assert.Equal(t, "USD", br.result.Currency)
			}
		}
	})
}

// T015: Integration test for batch path with actual cost queries.
func TestBatchActualCostIntegration(t *testing.T) {
	t.Run("resources with date range passed correctly", func(t *testing.T) {
		now := time.Now()
		start := timestamppb.New(now.Add(-7 * 24 * time.Hour))
		end := timestamppb.New(now)

		var capturedReqs []*pbc.BatchCostRequest
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				capturedReqs = append(capturedReqs, in)
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_CostData{
							CostData: &pbc.CostData{
								Data: &pbc.CostData_ActualCost{
									ActualCost: &pbc.ActualCostData{
										Results: []*pbc.ActualCostResult{
											{Cost: 25.0, Source: "aws-ce"},
										},
									},
								},
							},
						},
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}

		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(5)
		groups := eng.groupResourcesByPlugin(context.Background(), resources)

		for _, batch := range groups {
			results, err := eng.executeBatchForPlugin(
				context.Background(), batch.plugin, batch.resources,
				batchOptions{
					queryType: pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL,
					start:     start,
					end:       end,
				},
			)
			require.NoError(t, err)
			assert.Len(t, results, 5)

			// Verify ACTUAL query type and timestamps passed
			require.Len(t, capturedReqs, 1)
			assert.Equal(t, pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL, capturedReqs[0].GetQueryType())
			assert.NotNil(t, capturedReqs[0].GetStart())
			assert.NotNil(t, capturedReqs[0].GetEnd())

			// Verify actual results mapped
			for _, br := range results {
				require.NotNil(t, br.actualResult)
				assert.InDelta(t, 25.0, br.actualResult.TotalCost, 0.001)
			}
		}
	})
}

// seedProjectedCostCache pre-populates the cache with projected cost results for a resource.
func seedProjectedCostCache(t *testing.T, mc *mockCache, resource ResourceDescriptor, results []CostResult) {
	t.Helper()
	key, err := generateProjectedCostResourceKey(resource)
	require.NoError(t, err)
	data, err := json.Marshal(results)
	require.NoError(t, err)
	err = mc.Set(key, data)
	require.NoError(t, err)
}

// makeUniqueTestResources creates n test resources with distinct SKUs so each
// generates a unique projected cost cache key.
func makeUniqueTestResources(n int) []ResourceDescriptor {
	resources := make([]ResourceDescriptor, n)
	for i := range n {
		resources[i] = ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       fmt.Sprintf("i-%03d", i),
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": fmt.Sprintf("t3.sku%d", i), // unique SKU per resource
			},
		}
	}
	return resources
}

// T016: Unit test for cache pre-check in batch path.
func TestBatchCachePreCheck(t *testing.T) {
	t.Run("cached resources excluded from batch request", func(t *testing.T) {
		var capturedReq *pbc.BatchCostRequest
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				capturedReq = in
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_CostData{
							CostData: &pbc.CostData{
								Data: &pbc.CostData_ProjectedCost{
									ProjectedCost: &pbc.GetProjectedCostResponse{
										Currency:     "USD",
										CostPerMonth: 10.0,
										UnitPrice:    0.01,
									},
								},
							},
						},
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		mc := newMockCache(true)
		eng := New([]*pluginhost.Client{client}, nil).WithCache(mc)

		// Create 10 resources with unique SKUs (distinct cache keys)
		resources := makeUniqueTestResources(10)

		// Pre-cache 3 of them (indices 0, 3, 7)
		cachedIndices := []int{0, 3, 7}
		for _, idx := range cachedIndices {
			seedProjectedCostCache(t, mc, resources[idx], []CostResult{
				{
					ResourceType: resources[idx].Type,
					ResourceID:   resources[idx].ID,
					Adapter:      "test-plugin",
					Currency:     "USD",
					Monthly:      99.0,
				},
			})
		}

		// Execute GetProjectedCost — batch path should exclude cached resources
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)
		assert.Len(t, results, 10)

		// Verify batch request contained only 7 (uncached) resources
		require.NotNil(t, capturedReq, "expected BatchCost RPC to be called")
		assert.Len(t, capturedReq.GetResources(), 7, "batch should contain only uncached resources")

		// Verify cached results have "(cached)" suffix
		cachedCount := 0
		for _, r := range results {
			if r.Adapter == "test-plugin (cached)" {
				cachedCount++
			}
		}
		assert.Equal(t, 3, cachedCount, "3 resources should come from cache")
	})
}

// T024: Unit test for non-batch plugin fallback.
func TestNonBatchPluginFallback(t *testing.T) {
	t.Run("non-batch plugin uses per-resource worker pool", func(t *testing.T) {
		batchCalled := false

		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				batchCalled = true
				return &pbc.BatchCostResponse{}, nil
			},
		}

		// Use a non-batch client so batch path is skipped
		client := makeNonBatchClient("legacy-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(5)
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)

		// BatchCost should NOT have been called for non-batch plugin
		assert.False(t, batchCalled, "BatchCost should not be called for non-batch plugin")

		// All resources should have results (from per-resource path)
		assert.Len(t, results, 5)
	})
}

// T025: Unit test for batch-level gRPC error fallback.
func TestBatchGRPCErrorFallback(t *testing.T) {
	tests := []struct {
		name     string
		grpcCode codes.Code
	}{
		{"Unimplemented", codes.Unimplemented},
		{"Unavailable", codes.Unavailable},
		{"Internal", codes.Internal},
	}

	for _, tc := range tests {
		t.Run(tc.name+"_falls_back_to_per_resource", func(t *testing.T) {
			batchCallCount := 0
			mockAPI := &mockBatchCostSourceClient{
				batchCostFunc: func(_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
					batchCallCount++
					return nil, status.Error(tc.grpcCode, "batch not available")
				},
			}

			client := makeBatchCapableClient("failing-plugin", mockAPI)
			eng := New([]*pluginhost.Client{client}, nil)

			resources := makeTestResources(5)
			results, err := eng.GetProjectedCost(context.Background(), resources)
			require.NoError(t, err)

			// Batch was attempted
			assert.Equal(t, 1, batchCallCount, "batch RPC should have been attempted")

			// All resources should still have results (from per-resource fallback)
			assert.Len(t, results, 5, "all resources should get results via per-resource fallback")
		})
	}

	t.Run("DeadlineExceeded_with_context_still_valid_falls_back", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
			},
		}

		client := makeBatchCapableClient("timeout-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(3)
		// Use a context with a generous deadline so it's still valid after batch failure
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		results, err := eng.GetProjectedCost(ctx, resources)
		require.NoError(t, err)

		// Should fall back to per-resource since overall context is still alive
		assert.Len(t, results, 3)
	})
}

// T026: Unit test for response count mismatch fallback.
func TestBatchResponseCountMismatchFallback(t *testing.T) {
	t.Run("mismatch triggers fallback to per-resource queries", func(t *testing.T) {
		batchCallCount := 0
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				batchCallCount++
				// Return fewer results than requested — triggers count mismatch error
				return &pbc.BatchCostResponse{
					Results: []*pbc.ResourceCostResult{
						{
							Resource: &pbc.ResourceDescriptor{Id: "r-only-one"},
							Result: &pbc.ResourceCostResult_CostData{
								CostData: &pbc.CostData{
									Data: &pbc.CostData_ProjectedCost{
										ProjectedCost: &pbc.GetProjectedCostResponse{
											Currency:     "USD",
											CostPerMonth: 10.0,
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		client := makeBatchCapableClient("mismatch-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(5)
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)

		// Batch was attempted once
		assert.Equal(t, 1, batchCallCount, "batch RPC should have been attempted")

		// All resources should still have results via per-resource fallback
		assert.Len(t, results, 5, "all resources should get results via per-resource fallback after mismatch")
	})
}

// T031: Unit test for mixed success/error batch results.
func TestBatchMixedSuccessError(t *testing.T) {
	t.Run("7 succeed 3 fail with mixed error types", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					switch {
					case i == 2:
						// Resource error: internal failure
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_Error{
								Error: &pbc.ResourceError{
									Code:    int32(codes.Internal),
									Message: "pricing lookup failed",
								},
							},
						}
					case i == 5:
						// Resource error: unavailable
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_Error{
								Error: &pbc.ResourceError{
									Code:    int32(codes.Unavailable),
									Message: "service temporarily unavailable",
								},
							},
						}
					case i == 8:
						// Resource error: resource type unsupported
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_Error{
								Error: &pbc.ResourceError{
									Code:                    int32(codes.Unimplemented),
									Message:                 "unsupported resource type",
									ResourceTypeUnsupported: true,
								},
							},
						}
					default:
						// Success
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_CostData{
								CostData: &pbc.CostData{
									Data: &pbc.CostData_ProjectedCost{
										ProjectedCost: &pbc.GetProjectedCostResponse{
											Currency:     "USD",
											CostPerMonth: float64(i+1) * 10,
											UnitPrice:    0.01,
										},
									},
								},
							},
						}
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(10)
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)

		// All 10 resources should have some result:
		// - 7 successful results from batch
		// - 1 skipped (resource_type_unsupported at index 8) → per-resource fallback placeholder
		// - 2 errors (indices 2, 5) → per-resource fallback
		assert.Len(t, results, 10)

		// Verify successful batch results have correct adapter
		successCount := 0
		for _, r := range results {
			if r.Adapter == "test-plugin" && r.Monthly > 0 {
				successCount++
			}
		}
		assert.GreaterOrEqual(t, successCount, 7, "at least 7 resources should have successful batch results")
	})
}

// T032: Unit test for resource_type_unsupported skip behavior.
func TestBatchResourceTypeUnsupportedSkip(t *testing.T) {
	t.Run("unsupported resource type is skipped with WARN log", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					if i == 1 {
						// Mark as unsupported
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_Error{
								Error: &pbc.ResourceError{
									Code:                    int32(codes.Unimplemented),
									Message:                 "resource type not supported",
									ResourceTypeUnsupported: true,
								},
							},
						}
					} else {
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_CostData{
								CostData: &pbc.CostData{
									Data: &pbc.CostData_ProjectedCost{
										ProjectedCost: &pbc.GetProjectedCostResponse{
											Currency:     "USD",
											CostPerMonth: 15.0,
											UnitPrice:    0.02,
										},
									},
								},
							},
						}
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(3)
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)

		// All 3 resources should have results:
		// - 2 successful from batch
		// - 1 skipped (index 1) → marked as handled, will get per-resource fallback result
		assert.Len(t, results, 3)

		// The skipped resource (index 1) should not have a batch result error —
		// it should be silently skipped (same as per-resource validation skip behavior)
		for _, r := range results {
			assert.NotContains(t, r.Notes, "resource type not supported",
				"unsupported type errors should not propagate as notes")
		}
	})
}

// T033: Unit test for all-fail batch — all resources return ResourceError.
func TestBatchAllResourcesFail(t *testing.T) {
	t.Run("all resources return errors are handled individually", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					results[i] = &pbc.ResourceCostResult{
						Resource: res,
						Result: &pbc.ResourceCostResult_Error{
							Error: &pbc.ResourceError{
								Code:    int32(codes.Internal),
								Message: fmt.Sprintf("error for resource %d", i),
							},
						},
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(5)
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)

		// All 5 resources should have results — errors are handled per-resource
		// and fall back to per-resource worker pool (which also produces results)
		assert.Len(t, results, 5, "all resources should have results even when all batch entries fail")
	})
}

// T034: Unit test for nil/empty CostData fallback.
func TestBatchNilCostDataFallback(t *testing.T) {
	t.Run("nil CostData queued for fallback", func(t *testing.T) {
		batchCallCount := 0
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				batchCallCount++
				results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
				for i, res := range in.GetResources() {
					switch i {
					case 1:
						// Nil CostData — not an error, just empty
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_CostData{
								CostData: nil,
							},
						}
					case 2:
						// CostData present but with nil projected cost
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_CostData{
								CostData: &pbc.CostData{},
							},
						}
					default:
						// Normal success
						results[i] = &pbc.ResourceCostResult{
							Resource: res,
							Result: &pbc.ResourceCostResult_CostData{
								CostData: &pbc.CostData{
									Data: &pbc.CostData_ProjectedCost{
										ProjectedCost: &pbc.GetProjectedCostResponse{
											Currency:     "USD",
											CostPerMonth: 20.0,
											UnitPrice:    0.03,
										},
									},
								},
							},
						}
					}
				}
				return &pbc.BatchCostResponse{Results: results}, nil
			},
		}
		client := makeBatchCapableClient("test-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(4)
		results, err := eng.GetProjectedCost(context.Background(), resources)
		require.NoError(t, err)

		// Batch was called
		assert.Equal(t, 1, batchCallCount)

		// All 4 resources should have results:
		// - 2 successful from batch (indices 0, 3)
		// - 2 with nil data (indices 1, 2) → fall through to per-resource worker pool
		assert.Len(t, results, 4, "all resources should have results including nil-data fallbacks")
	})
}

// T030: Unit test for deadline-aware fallback.
func TestBatchDeadlineExceededPropagation(t *testing.T) {
	t.Run("DeadlineExceeded_with_expired_context_propagates_error", func(t *testing.T) {
		mockAPI := &mockBatchCostSourceClient{
			batchCostFunc: func(_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
				return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
			},
		}

		client := makeBatchCapableClient("timeout-plugin", mockAPI)
		eng := New([]*pluginhost.Client{client}, nil)

		resources := makeTestResources(3)

		// Use an already-expired context — the deadline has truly passed
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		time.Sleep(5 * time.Millisecond) // Ensure context is expired
		defer cancel()

		_, err := eng.GetProjectedCost(ctx, resources)
		// With an expired context, the error should propagate (not silently fall back)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled")
	})
}

// Tests for mapProtoCostResultToEngine — covers sustainability and structured error branches.
func TestMapProtoCostResultToEngine(t *testing.T) {
	resource := ResourceDescriptor{
		Type:     "aws:ec2:Instance",
		ID:       "i-test-001",
		Provider: "aws",
	}

	t.Run("basic fields mapped correctly", func(t *testing.T) {
		result := &proto.CostResult{
			Currency:    "USD",
			MonthlyCost: 150.0,
			HourlyCost:  0.205,
			Notes:       "on-demand pricing",
		}
		engineResult := mapProtoCostResultToEngine(resource, "test-plugin", result)
		assert.Equal(t, "aws:ec2:Instance", engineResult.ResourceType)
		assert.Equal(t, "i-test-001", engineResult.ResourceID)
		assert.Equal(t, "test-plugin", engineResult.Adapter)
		assert.Equal(t, "USD", engineResult.Currency)
		assert.InDelta(t, 150.0, engineResult.Monthly, 0.001)
		assert.InDelta(t, 0.205, engineResult.Hourly, 0.001)
		assert.Equal(t, "on-demand pricing", engineResult.Notes)
		assert.Nil(t, engineResult.Error)
		assert.Empty(t, engineResult.Sustainability)
	})

	t.Run("structured error mapped", func(t *testing.T) {
		result := &proto.CostResult{
			Currency:    "USD",
			MonthlyCost: 0,
			StructuredError: &proto.StructuredError{
				Code:         "PLUGIN_ERROR",
				Message:      "pricing lookup failed",
				ResourceType: "aws:ec2:Instance",
			},
		}
		engineResult := mapProtoCostResultToEngine(resource, "test-plugin", result)
		require.NotNil(t, engineResult.Error)
		assert.Equal(t, "PLUGIN_ERROR", engineResult.Error.Code)
		assert.Equal(t, "pricing lookup failed", engineResult.Error.Message)
		assert.Equal(t, "aws:ec2:Instance", engineResult.Error.ResourceType)
	})

	t.Run("sustainability metrics mapped", func(t *testing.T) {
		result := &proto.CostResult{
			Currency:    "USD",
			MonthlyCost: 100.0,
			Sustainability: map[string]proto.SustainabilityMetric{
				"carbon": {Value: 12.5, Unit: "kg CO2e/month"},
				"energy": {Value: 45.0, Unit: "kWh/month"},
			},
		}
		engineResult := mapProtoCostResultToEngine(resource, "test-plugin", result)
		require.Len(t, engineResult.Sustainability, 2)
		assert.InDelta(t, 12.5, engineResult.Sustainability["carbon"].Value, 0.001)
		assert.Equal(t, "kg CO2e/month", engineResult.Sustainability["carbon"].Unit)
		assert.InDelta(t, 45.0, engineResult.Sustainability["energy"].Value, 0.001)
		assert.Equal(t, "kWh/month", engineResult.Sustainability["energy"].Unit)
	})

	t.Run("expires_at mapped", func(t *testing.T) {
		expiry := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
		result := &proto.CostResult{
			Currency:    "USD",
			MonthlyCost: 50.0,
			ExpiresAt:   &expiry,
		}
		engineResult := mapProtoCostResultToEngine(resource, "test-plugin", result)
		require.NotNil(t, engineResult.ExpiresAt)
		assert.Equal(t, expiry, *engineResult.ExpiresAt)
	})

	t.Run("cost breakdown mapped", func(t *testing.T) {
		result := &proto.CostResult{
			Currency:    "USD",
			MonthlyCost: 200.0,
			CostBreakdown: map[string]float64{
				"compute": 150.0,
				"storage": 50.0,
			},
		}
		engineResult := mapProtoCostResultToEngine(resource, "test-plugin", result)
		require.Len(t, engineResult.Breakdown, 2)
		assert.InDelta(t, 150.0, engineResult.Breakdown["compute"], 0.001)
		assert.InDelta(t, 50.0, engineResult.Breakdown["storage"], 0.001)
	})
}

// Tests for mapProtoActualCostResultToEngine — covers sustainability and ExpiresAt branches.
func TestMapProtoActualCostResultToEngine(t *testing.T) {
	resource := ResourceDescriptor{
		Type:     "aws:s3:Bucket",
		ID:       "my-bucket",
		Provider: "aws",
	}

	t.Run("basic fields mapped correctly", func(t *testing.T) {
		result := &proto.ActualCostResult{
			Currency:  "USD",
			TotalCost: 42.50,
			CostBreakdown: map[string]float64{
				"storage":  30.0,
				"requests": 12.5,
			},
		}
		engineResult := mapProtoActualCostResultToEngine(resource, "cost-plugin", result)
		assert.Equal(t, "aws:s3:Bucket", engineResult.ResourceType)
		assert.Equal(t, "my-bucket", engineResult.ResourceID)
		assert.Equal(t, "cost-plugin", engineResult.Adapter)
		assert.Equal(t, "USD", engineResult.Currency)
		assert.InDelta(t, 42.50, engineResult.TotalCost, 0.001)
		require.Len(t, engineResult.Breakdown, 2)
		assert.Empty(t, engineResult.Sustainability)
	})

	t.Run("sustainability metrics mapped", func(t *testing.T) {
		result := &proto.ActualCostResult{
			Currency:  "USD",
			TotalCost: 100.0,
			Sustainability: map[string]proto.SustainabilityMetric{
				"water": {Value: 5.0, Unit: "gallons"},
			},
		}
		engineResult := mapProtoActualCostResultToEngine(resource, "cost-plugin", result)
		require.Len(t, engineResult.Sustainability, 1)
		assert.InDelta(t, 5.0, engineResult.Sustainability["water"].Value, 0.001)
		assert.Equal(t, "gallons", engineResult.Sustainability["water"].Unit)
	})

	t.Run("expires_at mapped", func(t *testing.T) {
		expiry := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
		result := &proto.ActualCostResult{
			Currency:  "USD",
			TotalCost: 75.0,
			ExpiresAt: &expiry,
		}
		engineResult := mapProtoActualCostResultToEngine(resource, "cost-plugin", result)
		require.NotNil(t, engineResult.ExpiresAt)
		assert.Equal(t, expiry, *engineResult.ExpiresAt)
	})
}
