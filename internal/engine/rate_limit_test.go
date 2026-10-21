// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"sync"
	"sync/atomic"
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

func TestRetryHintFromErrorDetail(t *testing.T) {
	t.Parallel()

	seconds := int32(2)
	wrapped := pluginStatusError(rateLimitError(&seconds))
	got, ok := retryHint(wrapped)
	require.True(t, ok)
	assert.Equal(t, 2*time.Second, got)
	assert.True(t, isRateLimited(wrapped))

	_, ok = retryHint(status.Error(codes.ResourceExhausted, "no hint"))
	assert.False(t, ok)
	assert.False(t, isRateLimited(status.Error(codes.Unavailable, "down")))
	assert.False(t, isRateLimited(nil))
}

func TestBackoffDelayStaysBounded(t *testing.T) {
	t.Parallel()

	for attempt := range 8 {
		delay := backoffDelay(attempt)
		assert.GreaterOrEqual(t, delay, rateLimitBaseDelay/2)
		assert.LessOrEqual(t, delay, rateLimitMaxDelay)
	}
}

func TestWaitRateLimitStopsWhenContextEnds(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := waitRateLimit(ctx, 5*time.Second)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second)
}

func TestFallbackMessageDistinguishesRateLimit(t *testing.T) {
	t.Parallel()

	limited := status.Error(codes.ResourceExhausted, "slow down")
	other := status.Error(codes.Internal, "boom")
	assert.Equal(t, "plugin rate limited, falling back to next priority plugin", fallbackMessage(limited))
	assert.Equal(t, "plugin failed, falling back to next priority plugin", fallbackMessage(other))
	assert.Equal(t, "plugin rate limited and fallback disabled, stopping fallback chain", stoppedChainMessage(limited))
	assert.Equal(t, "plugin failed and fallback disabled, stopping fallback chain", stoppedChainMessage(other))
}

func TestRateLimitRetriesSamePluginBeforeFallback(t *testing.T) {
	t.Parallel()

	var primaryCalls, fallbackCalls atomic.Int32
	zero := int32(0)
	primaryAPI := &mockBatchCostSourceClient{
		getProjectedCostFunc: func(
			_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			if primaryCalls.Add(1) == 1 {
				return nil, rateLimitError(&zero)
			}
			return projectedPrice(12), nil
		},
	}
	fallbackAPI := &mockBatchCostSourceClient{
		getProjectedCostFunc: func(
			_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			fallbackCalls.Add(1)
			return projectedPrice(99), nil
		},
	}
	primary := makeNonBatchClient("primary", primaryAPI)
	fallback := makeNonBatchClient("fallback", fallbackAPI)
	eng := New([]*pluginhost.Client{primary, fallback}, nil).WithRouter(priorityRouter(primary, fallback))

	start := time.Now()
	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(1))
	require.NoError(t, err)

	assert.Less(t, time.Since(start), 250*time.Millisecond)
	assert.Equal(t, int32(2), primaryCalls.Load())
	assert.Equal(t, int32(0), fallbackCalls.Load())
	require.Len(t, results, 1)
	assert.Equal(t, "primary", results[0].Adapter)
	assert.InDelta(t, 12.0, results[0].Monthly, 0.001)
}

func TestRateLimitDeadlineDuringWaitDoesNotFallBack(t *testing.T) {
	t.Parallel()

	var primaryCalls, fallbackCalls atomic.Int32
	hint := int32(30)
	primaryAPI := &mockBatchCostSourceClient{
		getProjectedCostFunc: func(
			_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			primaryCalls.Add(1)
			return nil, rateLimitError(&hint)
		},
	}
	fallbackAPI := &mockBatchCostSourceClient{
		getProjectedCostFunc: func(
			_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			fallbackCalls.Add(1)
			return projectedPrice(99), nil
		},
	}
	primary := makeNonBatchClient("primary", primaryAPI)
	fallback := makeNonBatchClient("fallback", fallbackAPI)
	eng := New([]*pluginhost.Client{primary, fallback}, nil).WithRouter(priorityRouter(primary, fallback))

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	_, err := eng.GetProjectedCost(ctx, makeTestResources(1))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(1), primaryCalls.Load())
	assert.Equal(t, int32(0), fallbackCalls.Load())
}

func TestBatchRateLimitRetriesWithoutPerResourceCalls(t *testing.T) {
	t.Parallel()

	var batchCalls, primaryUnary, fallbackCalls atomic.Int32
	zero := int32(0)
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			if batchCalls.Add(1) == 1 {
				return nil, rateLimitError(&zero)
			}
			return projectedBatchResponse(in, 8), nil
		},
		getProjectedCostFunc: countProjected(&primaryUnary, 1),
	}
	fallbackAPI := &mockBatchCostSourceClient{
		getProjectedCostFunc: countProjected(&fallbackCalls, 99),
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	fallback := makeNonBatchClient("fallback", fallbackAPI)
	eng := New([]*pluginhost.Client{primary, fallback}, nil).WithRouter(priorityRouter(primary, fallback))

	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(2))
	require.NoError(t, err)

	assert.Equal(t, int32(2), batchCalls.Load())
	assert.Equal(t, int32(0), primaryUnary.Load())
	assert.Equal(t, int32(0), fallbackCalls.Load())
	require.Len(t, results, 2)
	assert.Equal(t, "primary", results[0].Adapter)
	assert.InDelta(t, 8.0, results[0].Monthly, 0.001)
}

func TestBatchRateLimitExhaustedSkipsUnaryOnThatPlugin(t *testing.T) {
	t.Parallel()

	var batchCalls, primaryUnary, fallbackCalls atomic.Int32
	zero := int32(0)
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			batchCalls.Add(1)
			return nil, rateLimitError(&zero)
		},
		getProjectedCostFunc: countProjected(&primaryUnary, 1),
	}
	fallbackAPI := &mockBatchCostSourceClient{
		getProjectedCostFunc: countProjected(&fallbackCalls, 4),
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	fallback := makeNonBatchClient("fallback", fallbackAPI)
	eng := New([]*pluginhost.Client{primary, fallback}, nil).WithRouter(priorityRouter(primary, fallback))

	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(2))
	require.NoError(t, err)

	assert.Equal(t, int32(rateLimitMaxAttempts), batchCalls.Load())
	assert.Equal(t, int32(0), primaryUnary.Load())
	assert.Equal(t, int32(2), fallbackCalls.Load())
	require.Len(t, results, 2)
	assert.Equal(t, "fallback", results[0].Adapter)
}

func TestBatchItemRateLimitIsRetriedTogether(t *testing.T) {
	t.Parallel()

	var batchCalls, unary atomic.Int32
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			if batchCalls.Add(1) == 1 {
				return rateLimitedBatchItems(in), nil
			}
			return projectedBatchResponse(in, 6), nil
		},
		getProjectedCostFunc: countProjected(&unary, 1),
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	eng := New([]*pluginhost.Client{primary}, nil)

	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(3))
	require.NoError(t, err)

	assert.Equal(t, int32(2), batchCalls.Load())
	assert.Equal(t, int32(0), unary.Load())
	require.Len(t, results, 3)
	assert.Equal(t, "primary", results[0].Adapter)
	assert.InDelta(t, 6.0, results[0].Monthly, 0.001)
}

func TestBatchRateLimitDeadlineDoesNotFanOut(t *testing.T) {
	t.Parallel()

	var batchCalls, unary atomic.Int32
	hint := int32(30)
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			batchCalls.Add(1)
			return nil, rateLimitError(&hint)
		},
		getProjectedCostFunc: countProjected(&unary, 1),
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	eng := New([]*pluginhost.Client{primary}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	_, err := eng.GetProjectedCost(ctx, makeTestResources(2))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(1), batchCalls.Load())
	assert.Equal(t, int32(0), unary.Load())
}

func TestActualBatchRateLimitRetriesWithoutPerResourceCalls(t *testing.T) {
	t.Parallel()

	var batchCalls, unary atomic.Int32
	zero := int32(0)
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			if batchCalls.Add(1) == 1 {
				return nil, rateLimitError(&zero)
			}
			return actualBatchResponse(in, 7), nil
		},
		getActualCostFunc: func(
			_ context.Context, _ *proto.GetActualCostRequest, _ ...grpc.CallOption,
		) (*proto.GetActualCostResponse, error) {
			unary.Add(1)
			return &proto.GetActualCostResponse{}, nil
		},
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	eng := New([]*pluginhost.Client{primary}, nil)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	results, err := eng.GetActualCost(context.Background(), makeTestResources(2), from, to)
	require.NoError(t, err)

	assert.Equal(t, int32(2), batchCalls.Load())
	assert.Equal(t, int32(0), unary.Load())
	require.Len(t, results, 2)
	assert.Equal(t, "primary", results[0].Adapter)
	assert.InDelta(t, 7.0, results[0].TotalCost, 0.001)
}

func TestBatchItemRetryFailureKeepsPricedRows(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	var unaryIDs []string
	var mu sync.Mutex
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			if batchCalls.Add(1) == 1 {
				return pricedAndRateLimited(in, 11), nil
			}
			return nil, status.Error(codes.Unavailable, "down")
		},
		getProjectedCostFunc: func(
			_ context.Context, in *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			mu.Lock()
			unaryIDs = append(unaryIDs, in.Resources[0].ID)
			mu.Unlock()
			return projectedPrice(3), nil
		},
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	eng := New([]*pluginhost.Client{primary}, nil)

	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(2))
	require.NoError(t, err)

	assert.Equal(t, int32(2), batchCalls.Load())
	assert.Equal(t, []string{"i-001"}, unaryIDs)
	require.Len(t, results, 2)
	assert.Equal(t, "i-000", results[0].ResourceID)
	assert.Equal(t, "primary", results[0].Adapter)
	assert.InDelta(t, 11.0, results[0].Monthly, 0.001)
	assert.Equal(t, "i-001", results[1].ResourceID)
	assert.InDelta(t, 3.0, results[1].Monthly, 0.001)
}

func TestBatchOrdinaryErrorSurvivesLaterRateLimit(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	var unaryIDs []string
	var mu sync.Mutex
	zero := int32(0)
	primaryAPI := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			batchCalls.Add(1)
			if len(in.GetResources()) > 1 {
				return firstChunkWithOrdinaryError(in), nil
			}
			return nil, rateLimitError(&zero)
		},
		getProjectedCostFunc: func(
			_ context.Context, in *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			mu.Lock()
			unaryIDs = append(unaryIDs, in.Resources[0].ID)
			mu.Unlock()
			return projectedPrice(4), nil
		},
	}
	primary := makeBatchCapableClient("primary", primaryAPI)
	eng := New([]*pluginhost.Client{primary}, nil)

	resources := makeTestResources(batchProcessingThreshold + 1)
	results, err := eng.GetProjectedCost(context.Background(), resources)
	require.NoError(t, err)

	assert.Equal(t, int32(1+rateLimitMaxAttempts), batchCalls.Load())
	assert.Equal(t, []string{"i-000"}, unaryIDs)
	require.Len(t, results, len(resources))
	assert.Equal(t, "i-000", results[0].ResourceID)
	assert.InDelta(t, 4.0, results[0].Monthly, 0.001)
	assert.Equal(t, "primary", results[1].Adapter)
	assert.InDelta(t, 5.0, results[1].Monthly, 0.001)
	last := results[len(results)-1]
	assert.Equal(t, "i-100", last.ResourceID)
	assert.Equal(t, adapterNone, last.Adapter)
	assert.Contains(t, last.Notes, errBatchRateLimited.Error())
}

func TestActualBatchRateLimitIsNotCached(t *testing.T) {
	t.Parallel()

	var batchCalls, unary atomic.Int32
	zero := int32(0)
	api := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			batchCalls.Add(1)
			return nil, rateLimitError(&zero)
		},
		getActualCostFunc: func(
			_ context.Context, _ *proto.GetActualCostRequest, _ ...grpc.CallOption,
		) (*proto.GetActualCostResponse, error) {
			unary.Add(1)
			return &proto.GetActualCostResponse{}, nil
		},
	}
	store := newMockCache(true)
	eng := New([]*pluginhost.Client{makeBatchCapableClient("primary", api)}, nil).WithCache(store)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	resources := makeTestResources(1)

	first, err := eng.GetActualCost(context.Background(), resources, from, to)
	require.NoError(t, err)
	second, err := eng.GetActualCost(context.Background(), resources, from, to)
	require.NoError(t, err)

	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.Equal(t, adapterNone, first[0].Adapter)
	assert.Equal(t, int32(0), unary.Load())
	assert.Equal(t, int32(rateLimitMaxAttempts*2), batchCalls.Load())
	assert.Equal(t, 0, store.setCalls)
	assert.Equal(t, 0, store.setWithTTL)
}

func TestBatchRateLimitExhaustedNamesThePlugin(t *testing.T) {
	t.Parallel()

	zero := int32(0)
	api := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			return nil, rateLimitError(&zero)
		},
		getProjectedCostFunc: countProjected(new(atomic.Int32), 1),
	}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("primary", api)}, nil)

	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(1))
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, adapterNone, results[0].Adapter)
	assert.Contains(t, results[0].Notes, errBatchRateLimited.Error())
}

func rateLimitError(seconds *int32) error {
	detail := &pbc.ErrorDetail{Code: pbc.ErrorCode_ERROR_CODE_RATE_LIMITED}
	if seconds != nil {
		detail.RetryAfterSeconds = seconds
	}
	st, err := status.New(codes.ResourceExhausted, "slow down").WithDetails(detail)
	if err != nil {
		return status.Error(codes.ResourceExhausted, "slow down")
	}
	return st.Err()
}

func projectedPrice(monthly float64) *proto.GetProjectedCostResponse {
	return &proto.GetProjectedCostResponse{
		Results: []*proto.CostResult{{Currency: "USD", MonthlyCost: monthly}},
	}
}

func countProjected(
	counter *atomic.Int32, monthly float64,
) func(context.Context, *proto.GetProjectedCostRequest, ...grpc.CallOption) (*proto.GetProjectedCostResponse, error) {
	return func(
		_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
	) (*proto.GetProjectedCostResponse, error) {
		counter.Add(1)
		return projectedPrice(monthly), nil
	}
}

func projectedBatchResponse(in *pbc.BatchCostRequest, monthly float64) *pbc.BatchCostResponse {
	results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
	for i, res := range in.GetResources() {
		results[i] = projectedBatchResult(res, monthly, 0.01)
	}
	return &pbc.BatchCostResponse{Results: results}
}

func pricedAndRateLimited(in *pbc.BatchCostRequest, monthly float64) *pbc.BatchCostResponse {
	results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
	for i, res := range in.GetResources() {
		if i == len(in.GetResources())-1 {
			results[i] = &pbc.ResourceCostResult{
				Resource: res,
				Result: &pbc.ResourceCostResult_Error{
					Error: &pbc.ResourceError{
						Code:    int32(codes.ResourceExhausted),
						Message: "slow down",
					},
				},
			}
			continue
		}
		results[i] = projectedBatchResult(res, monthly, 0.01)
	}
	return &pbc.BatchCostResponse{Results: results}
}

func firstChunkWithOrdinaryError(in *pbc.BatchCostRequest) *pbc.BatchCostResponse {
	results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
	for i, res := range in.GetResources() {
		if i == 0 {
			results[i] = &pbc.ResourceCostResult{
				Resource: res,
				Result: &pbc.ResourceCostResult_Error{
					Error: &pbc.ResourceError{
						Code:    int32(codes.Unavailable),
						Message: "down",
					},
				},
			}
			continue
		}
		results[i] = projectedBatchResult(res, 5, 0.01)
	}
	return &pbc.BatchCostResponse{Results: results}
}

func rateLimitedBatchItems(in *pbc.BatchCostRequest) *pbc.BatchCostResponse {
	results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
	for i, res := range in.GetResources() {
		results[i] = &pbc.ResourceCostResult{
			Resource: res,
			Result: &pbc.ResourceCostResult_Error{
				Error: &pbc.ResourceError{
					Code:    int32(codes.ResourceExhausted),
					Message: "slow down",
				},
			},
		}
	}
	return &pbc.BatchCostResponse{Results: results}
}

func actualBatchResponse(in *pbc.BatchCostRequest, total float64) *pbc.BatchCostResponse {
	results := make([]*pbc.ResourceCostResult, len(in.GetResources()))
	for i, res := range in.GetResources() {
		results[i] = &pbc.ResourceCostResult{
			Resource: res,
			Result: &pbc.ResourceCostResult_CostData{
				CostData: &pbc.CostData{
					Data: &pbc.CostData_ActualCost{
						ActualCost: &pbc.ActualCostData{
							Results: []*pbc.ActualCostResult{{Cost: total, Source: "aws-ce"}},
						},
					},
				},
			},
		}
	}
	return &pbc.BatchCostResponse{Results: results}
}

func priorityRouter(plugins ...*pluginhost.Client) *mockRouter {
	matches := make([]PluginMatch, len(plugins))
	for i, client := range plugins {
		matches[i] = PluginMatch{Client: client, Priority: 100 - i, Fallback: true}
	}
	return &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return matches
		},
	}
}

func TestBatchRetryHonorsLearnedSizeLimit(t *testing.T) {
	t.Parallel()

	const total, hint = 40, 10
	var mu sync.Mutex
	var sizes []int
	var unary atomic.Int32
	api := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			mu.Lock()
			first := len(sizes) == 0
			sizes = append(sizes, len(in.GetResources()))
			mu.Unlock()
			if !first {
				return projectedBatchResponse(in, 6), nil
			}
			resp := pricedThenRateLimited(in, hint, 6)
			resp.MaxBatchSize = hint
			return resp, nil
		},
		getProjectedCostFunc: countProjected(&unary, 1),
	}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("primary", api)}, nil)

	results, err := eng.GetProjectedCost(context.Background(), makeTestResources(total))
	require.NoError(t, err)

	require.NotEmpty(t, sizes)
	assert.Equal(t, total, sizes[0])
	retried := 0
	for _, n := range sizes[1:] {
		assert.LessOrEqual(t, n, hint, "a retry must respect the advertised batch size")
		retried += n
	}
	assert.Equal(t, total-hint, retried)
	assert.Equal(t, int32(0), unary.Load())
	require.Len(t, results, total)
	for _, r := range results {
		assert.Equal(t, "primary", r.Adapter)
		assert.InDelta(t, 6.0, r.Monthly, 0.001)
	}
}

func TestBatchLaterChunkFailureKeepsRateLimitMarkers(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	var mu sync.Mutex
	var unaryIDs []string
	api := &mockBatchCostSourceClient{
		batchCostFunc: func(
			_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
		) (*pbc.BatchCostResponse, error) {
			batchCalls.Add(1)
			if len(in.GetResources()) > 1 {
				return rateLimitedBatchItems(in), nil
			}
			return nil, status.Error(codes.Unavailable, "down")
		},
		getProjectedCostFunc: func(
			_ context.Context, in *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*proto.GetProjectedCostResponse, error) {
			mu.Lock()
			unaryIDs = append(unaryIDs, in.Resources[0].ID)
			mu.Unlock()
			return projectedPrice(4), nil
		},
	}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("primary", api)}, nil)

	resources := makeTestResources(batchProcessingThreshold + 1)
	results, err := eng.GetProjectedCost(context.Background(), resources)
	require.NoError(t, err)

	assert.Equal(t, int32(rateLimitMaxAttempts+1), batchCalls.Load())
	assert.Equal(t, []string{"i-100"}, unaryIDs, "only the chunk that failed may be priced one by one")
	require.Len(t, results, len(resources))
	assert.Equal(t, adapterNone, results[0].Adapter)
	assert.Contains(t, results[0].Notes, errBatchRateLimited.Error())
	last := results[len(results)-1]
	assert.Equal(t, "i-100", last.ResourceID)
	assert.InDelta(t, 4.0, last.Monthly, 0.001)
}

func TestBatchRateLimitKindsShareOneAttemptBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		script     []string
		wantCalls  int32
		wantPriced bool
	}{
		{"two item limits then success", []string{"items", "items", "ok"}, 3, true},
		{"rpc limit then item limit then success", []string{"rpc", "items", "ok"}, 3, true},
		{"item limits exhaust the budget", []string{"items", "items", "items", "ok"}, 3, false},
		{"mixed limits exhaust the budget", []string{"rpc", "items", "rpc", "ok"}, 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var batchCalls, unary atomic.Int32
			zero := int32(0)
			api := &mockBatchCostSourceClient{
				batchCostFunc: func(
					_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption,
				) (*pbc.BatchCostResponse, error) {
					call := int(batchCalls.Add(1)) - 1
					switch tt.script[call] {
					case "rpc":
						return nil, rateLimitError(&zero)
					case "items":
						return rateLimitedBatchItems(in), nil
					default:
						return projectedBatchResponse(in, 7), nil
					}
				},
				getProjectedCostFunc: countProjected(&unary, 1),
			}
			eng := New([]*pluginhost.Client{makeBatchCapableClient("primary", api)}, nil)

			results, err := eng.GetProjectedCost(context.Background(), makeTestResources(2))
			require.NoError(t, err)

			assert.Equal(t, tt.wantCalls, batchCalls.Load())
			assert.Equal(t, int32(0), unary.Load())
			require.Len(t, results, 2)
			if tt.wantPriced {
				assert.Equal(t, "primary", results[0].Adapter)
				assert.InDelta(t, 7.0, results[0].Monthly, 0.001)
				return
			}
			assert.Equal(t, adapterNone, results[0].Adapter)
			assert.Contains(t, results[0].Notes, errBatchRateLimited.Error())
		})
	}
}

func pricedThenRateLimited(in *pbc.BatchCostRequest, priced int, monthly float64) *pbc.BatchCostResponse {
	limited := rateLimitedBatchItems(in)
	for i, res := range in.GetResources() {
		if i < priced {
			limited.Results[i] = projectedBatchResult(res, monthly, 0.01)
		}
	}
	return limited
}
