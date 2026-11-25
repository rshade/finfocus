// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// actualCallRecorder fakes a plugin and records every BatchCost and unary
// GetActualCost call. A resource's cost is the number after "i-" in its ID.
type actualCallRecorder struct {
	mu      sync.Mutex
	batches [][]string
	starts  []time.Time
	arns    map[string]string
	unary   []string
	// unaryStart is the start each unary call sent, by resource ID.
	unaryStart map[string]time.Time
	// batchErr, when set, fails the nth BatchCost call (counting from 1).
	batchErr func(call int) error
	// itemResult overrides the batch row for a resource ID.
	itemResult map[string]func(*pbc.ResourceDescriptor) *pbc.ResourceCostResult
}

func costFromID(t *testing.T, id string) float64 {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimLeft(strings.TrimPrefix(id, "i-"), "0"))
	if err != nil {
		return 0
	}
	return float64(n)
}

func (r *actualCallRecorder) api(t *testing.T) *mockBatchCostSourceClient {
	t.Helper()
	return &mockBatchCostSourceClient{
		batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.batchErr != nil {
				if err := r.batchErr(len(r.batches) + 1); err != nil {
					r.batches = append(r.batches, nil)
					return nil, err
				}
			}
			ids := make([]string, 0, len(in.GetResources()))
			results := make([]*pbc.ResourceCostResult, 0, len(in.GetResources()))
			for _, res := range in.GetResources() {
				ids = append(ids, res.GetId())
				if override, ok := r.itemResult[res.GetId()]; ok {
					results = append(results, override(res))
					continue
				}
				results = append(results, &pbc.ResourceCostResult{
					Resource: res,
					Result: &pbc.ResourceCostResult_CostData{CostData: &pbc.CostData{
						Data: &pbc.CostData_ActualCost{ActualCost: &pbc.ActualCostData{
							Results: []*pbc.ActualCostResult{{Cost: costFromID(t, res.GetId())}},
						}},
					}},
				})
			}
			r.batches = append(r.batches, ids)
			r.starts = append(r.starts, in.GetStart().AsTime())
			for _, res := range in.GetResources() {
				if r.arns == nil {
					r.arns = map[string]string{}
				}
				r.arns[res.GetId()] = res.GetArn()
			}
			return &pbc.BatchCostResponse{Results: results}, nil
		},
		getActualCostFunc: func(
			_ context.Context, in *proto.GetActualCostRequest, _ ...grpc.CallOption,
		) (*proto.GetActualCostResponse, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			id := in.ResourceIDs[0]
			r.unary = append(r.unary, id)
			if r.unaryStart == nil {
				r.unaryStart = map[string]time.Time{}
			}
			r.unaryStart[id] = time.Unix(in.StartTime, 0).UTC()
			return &proto.GetActualCostResponse{Results: []*proto.ActualCostResult{
				{TotalCost: costFromID(t, id), Currency: "USD"},
			}}, nil
		},
	}
}

func resourceIDs(results []CostResult) []string {
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.ResourceID
	}
	return ids
}

func TestActualCostAndErrorsUsesBatchCost(t *testing.T) {
	t.Parallel()
	from, to, createdMidWindow := actualWindow()

	rec := &actualCallRecorder{}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	createdLater := createdMidWindow.Add(17 * time.Minute)
	resources := []ResourceDescriptor{
		instanceWithProps("i-001", map[string]any{"pulumi:arn": "arn:aws:ec2:us-east-1:123456789012:instance/i-001"}),
		instanceWithProps("i-002", nil),
		instanceWithProps("i-003", map[string]any{PropertyPulumiCreated: createdLater.Format(time.RFC3339)}),
		instanceWithProps("i-004", nil),
		instanceWithProps("i-005", map[string]any{PropertyPulumiCreated: createdMidWindow.Format(time.RFC3339)}),
	}
	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: resources, From: from, To: to,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Errors)

	assert.Equal(t, [][]string{{"i-001", "i-002", "i-004"}, {"i-003", "i-005"}}, rec.batches,
		"one BatchCost call per start; resources created in the same hour share one")
	require.Len(t, rec.starts, 2)
	assert.True(t, from.Equal(rec.starts[0]), "got %s", rec.starts[0])
	assert.True(t, createdMidWindow.Truncate(time.Hour).Equal(rec.starts[1]), "got %s", rec.starts[1])
	assert.Empty(t, rec.unary, "no resource is priced alone")
	assert.Equal(t, "arn:aws:ec2:us-east-1:123456789012:instance/i-001", rec.arns["i-001"],
		"the batch descriptor carries the ARN")

	assert.Equal(t, []string{"i-001", "i-002", "i-003", "i-004", "i-005"}, resourceIDs(result.Results),
		"input order is kept")
	for _, r := range result.Results {
		assert.Equal(t, "cloud-billing", r.Adapter)
		assert.InDelta(t, costFromID(t, r.ResourceID), r.TotalCost, 1e-9)
		assert.True(t, from.Equal(r.StartDate), "every row keeps the requested window")
		require.Len(t, r.DailyCosts, 29)
	}
	created := result.Results[4].DailyCosts
	assert.Zero(t, sum(created[:16]), "no spend before the creation day")
}

func TestActualCostAndErrorsBatchItemErrorsBecomeErrorDetails(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	rec := &actualCallRecorder{itemResult: map[string]func(*pbc.ResourceDescriptor) *pbc.ResourceCostResult{
		"i-002": func(res *pbc.ResourceDescriptor) *pbc.ResourceCostResult {
			return &pbc.ResourceCostResult{
				Resource: res,
				Result: &pbc.ResourceCostResult_Error{Error: &pbc.ResourceError{
					Code: int32(codes.NotFound), Message: "no such resource",
				}},
			}
		},
		"i-003": func(res *pbc.ResourceDescriptor) *pbc.ResourceCostResult {
			return &pbc.ResourceCostResult{
				Resource: res,
				Result: &pbc.ResourceCostResult_CostData{CostData: &pbc.CostData{
					Data: &pbc.CostData_ActualCost{ActualCost: &pbc.ActualCostData{}},
				}},
			}
		},
	}}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	resources := []ResourceDescriptor{
		instanceWithProps("i-001", nil),
		instanceWithProps("i-002", nil),
		instanceWithProps("i-003", nil),
	}
	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: resources, From: from, To: to, FallbackEstimate: true, SkipStateEstimate: true,
	})
	require.NoError(t, err)

	assert.Len(t, rec.batches, 1)
	assert.Empty(t, rec.unary, "a resource the batch already answered is not asked again")

	require.Len(t, result.Errors, 2)
	byID := map[string]ErrorDetail{}
	for _, detail := range result.Errors {
		byID[detail.ResourceID] = detail
	}
	assert.Equal(t, "cloud-billing", byID["i-002"].PluginName)
	require.Error(t, byID["i-002"].Error)
	assert.Contains(t, byID["i-002"].Error.Error(), "no such resource")
	require.ErrorIs(t, byID["i-003"].Error, ErrNoCostData)

	assert.Equal(t, []string{"i-001", "i-002", "i-003"}, resourceIDs(result.Results))
	assert.InDelta(t, 1.0, result.Results[0].TotalCost, 1e-9)
	assert.Equal(t, adapterNone, result.Results[1].Adapter, "a failed resource gets the usual placeholder")
	assert.Contains(t, result.Results[1].Notes, "no such resource")
}

func TestActualCostAndErrorsNonBatchPluginStaysUnary(t *testing.T) {
	t.Parallel()
	from, to, createdMidWindow := actualWindow()

	rec := &actualCallRecorder{}
	eng := New([]*pluginhost.Client{makeNonBatchClient("cloud-billing", rec.api(t))}, nil)

	created := createdMidWindow.Add(17*time.Minute + 23*time.Second)
	resources := []ResourceDescriptor{
		instanceWithProps("i-001", nil),
		instanceWithProps("i-002", map[string]any{PropertyPulumiCreated: created.Format(time.RFC3339)}),
		instanceWithProps("i-003", nil),
	}
	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: resources, From: from, To: to,
	})
	require.NoError(t, err)

	assert.Empty(t, rec.batches)
	assert.ElementsMatch(t, []string{"i-001", "i-002", "i-003"}, rec.unary)
	assert.True(t, created.Equal(rec.unaryStart["i-002"]),
		"a plugin without batch_cost gets the exact creation time, not the hour; got %s", rec.unaryStart["i-002"])
	assert.True(t, from.Equal(rec.unaryStart["i-001"]), "got %s", rec.unaryStart["i-001"])
	assert.Equal(t, []string{"i-001", "i-002", "i-003"}, resourceIDs(result.Results))
}

func TestActualCostAndErrorsBatchUnavailableItemIsRetriedAlone(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	rec := &actualCallRecorder{itemResult: map[string]func(*pbc.ResourceDescriptor) *pbc.ResourceCostResult{
		"i-002": func(res *pbc.ResourceDescriptor) *pbc.ResourceCostResult {
			return &pbc.ResourceCostResult{
				Resource: res,
				Result: &pbc.ResourceCostResult_Error{Error: &pbc.ResourceError{
					Code: int32(codes.Unavailable), Message: "upstream unreachable",
				}},
			}
		},
	}}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	resources := []ResourceDescriptor{
		instanceWithProps("i-001", nil),
		instanceWithProps("i-002", nil),
		instanceWithProps("i-003", nil),
	}
	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: resources, From: from, To: to,
	})
	require.NoError(t, err)

	assert.Len(t, rec.batches, 1)
	assert.Equal(t, []string{"i-002"}, rec.unary, "only the Unavailable resource is asked again, alone")
	assert.Empty(t, result.Errors)
	require.Len(t, result.Results, 3)
	assert.InDelta(t, 2.0, result.Results[1].TotalCost, 1e-9, "the unary retry's price is used")
}

func TestActualCostAndErrorsBatchResultsAreCached(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	rec := &actualCallRecorder{}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil).
		WithCache(newMockCache(true))

	request := ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil), instanceWithProps("i-002", nil)},
		From:      from,
		To:        to,
	}
	first, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)
	require.NoError(t, err)
	second, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)
	require.NoError(t, err)

	assert.Len(t, rec.batches, 1, "the second query is served from the cache")
	assert.Equal(t, resourceIDs(first.Results), resourceIDs(second.Results))
}

func TestActualCostAndErrorsBatchRateLimitKeepsPluginMessage(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	limited := func(res *pbc.ResourceDescriptor) *pbc.ResourceCostResult {
		return &pbc.ResourceCostResult{
			Resource: res,
			Result: &pbc.ResourceCostResult_Error{Error: &pbc.ResourceError{
				Code: int32(codes.ResourceExhausted), Message: "daily request limit reached; retry later",
			}},
		}
	}
	rec := &actualCallRecorder{itemResult: map[string]func(*pbc.ResourceDescriptor) *pbc.ResourceCostResult{
		"i-001": limited,
	}}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil)}, From: from, To: to,
	})
	require.NoError(t, err)

	assert.Empty(t, rec.unary, "a rate-limited plugin is not asked again one resource at a time")
	require.Len(t, result.Errors, 1)
	require.ErrorIs(t, result.Errors[0].Error, errBatchRateLimited)
	assert.Contains(t, result.Errors[0].Error.Error(), "retry later", "the plugin's message reaches the user")
}

func TestActualCostAndErrorsFailedBatchRetryPricesResourcesAlone(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	rec := &actualCallRecorder{
		itemResult: map[string]func(*pbc.ResourceDescriptor) *pbc.ResourceCostResult{
			"i-001": func(res *pbc.ResourceDescriptor) *pbc.ResourceCostResult {
				return &pbc.ResourceCostResult{
					Resource: res,
					Result: &pbc.ResourceCostResult_Error{Error: &pbc.ResourceError{
						Code: int32(codes.ResourceExhausted), Message: "slow down",
					}},
				}
			},
		},
		batchErr: func(call int) error {
			if call > 1 {
				return status.Error(codes.Internal, "boom")
			}
			return nil
		},
	}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil), instanceWithProps("i-002", nil)},
		From:      from, To: to,
	})
	require.NoError(t, err)

	assert.Len(t, rec.batches, 2, "the first call and its failed retry")
	assert.Equal(t, []string{"i-001"}, rec.unary, "a resource the failed retry never answered is priced alone")
	assert.Empty(t, result.Errors)
	require.Len(t, result.Results, 2)
	assert.InDelta(t, 1.0, result.Results[0].TotalCost, 1e-9)
	assert.InDelta(t, 2.0, result.Results[1].TotalCost, 1e-9)
}

func TestActualCostAndErrorsBatchesPluginsThatShareAReportedName(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	recA, recB := &actualCallRecorder{}, &actualCallRecorder{}
	twinA := makeBatchCapableClient("twin", recA.api(t))
	twinA.InstallName = "twin-a"
	twinB := makeBatchCapableClient("twin", recB.api(t))
	twinB.InstallName = "twin-b"
	router := &mockRouter{
		selectPluginsFunc: func(_ context.Context, resource ResourceDescriptor, _ string) []PluginMatch {
			if resource.ID == "i-001" {
				return []PluginMatch{{Client: twinA, Priority: 10, Fallback: true}}
			}
			return []PluginMatch{{Client: twinB, Priority: 10, Fallback: true}}
		},
	}
	eng := New([]*pluginhost.Client{twinA, twinB}, nil).WithRouter(router)

	_, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil), instanceWithProps("i-002", nil)},
		From:      from, To: to,
	})
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"i-001"}}, recA.batches)
	assert.Equal(t, [][]string{{"i-002"}}, recB.batches, "each resource goes to the plugin it matched")
	assert.Empty(t, recA.unary)
	assert.Empty(t, recB.unary)
}

func TestActualCostAndErrorsAdapterBatchesTheNamedPlugin(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	first, named := &actualCallRecorder{}, &actualCallRecorder{}
	eng := New([]*pluginhost.Client{
		makeBatchCapableClient("a-plugin", first.api(t)),
		makeBatchCapableClient("b-plugin", named.api(t)),
	}, nil)

	_, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil), instanceWithProps("i-002", nil)},
		From:      from, To: to, Adapter: "b-plugin",
	})
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"i-001", "i-002"}}, named.batches)
	assert.Empty(t, named.unary)
	assert.Empty(t, first.batches)
	assert.Empty(t, first.unary)
}

func TestActualCostAndErrorsBatchFollowsActualCostRoute(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	listPrice, billing := &actualCallRecorder{}, &actualCallRecorder{}
	listClient := makeNonBatchClient("list-price", listPrice.api(t))
	billingClient := makeBatchCapableClient("billing", billing.api(t))
	router := &mockRouter{
		selectPluginsFunc: func(_ context.Context, _ ResourceDescriptor, feature string) []PluginMatch {
			if feature != actualCostsFeature {
				return []PluginMatch{{Client: billingClient, Priority: 10, Fallback: true}}
			}
			return []PluginMatch{
				{Client: listClient, Priority: 30, Fallback: true},
				{Client: billingClient, Priority: 10, Fallback: true},
			}
		},
	}
	eng := New([]*pluginhost.Client{listClient, billingClient}, nil).WithRouter(router)

	_, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil), instanceWithProps("i-002", nil)},
		From:      from, To: to,
	})
	require.NoError(t, err)

	assert.Empty(t, billing.batches, "the batch plugin is not the first plugin the ActualCosts route asks")
	assert.ElementsMatch(t, []string{"i-001", "i-002"}, listPrice.unary)
}

func TestActualCostAndErrorsOversizedDescriptorIsPricedAlone(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	rec := &actualCallRecorder{}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{
			instanceWithProps("i-001", nil),
			instanceWithProps("i-002", map[string]any{"userData": strings.Repeat("x", 4096)}),
			instanceWithProps("i-003", nil),
		},
		From: from, To: to,
	})
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"i-001", "i-003"}}, rec.batches, "one oversized descriptor does not sink the batch")
	assert.Equal(t, []string{"i-002"}, rec.unary)
	assert.Empty(t, result.Errors)
}

func TestActualCostAndErrorsBatchKeepsCreationDayOffHourWindow(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 10, 1, 0, 37, 0, 0, time.UTC)
	to := time.Date(2026, 10, 30, 0, 37, 0, 0, time.UTC)
	created := time.Date(2026, 10, 17, 0, 45, 0, 0, time.UTC)

	rec := &actualCallRecorder{}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("cloud-billing", rec.api(t))}, nil)

	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{
			instanceWithProps("i-001", map[string]any{PropertyPulumiCreated: created.Format(time.RFC3339)}),
		},
		From: from, To: to,
	})
	require.NoError(t, err)
	require.Len(t, rec.batches, 1)
	require.Len(t, result.Results, 1)

	daily := result.Results[0].DailyCosts
	assert.Zero(t, sum(daily[:16]), "no spend before the creation day, counted from the window start")
	assert.Positive(t, daily[16])
}
