// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type countingBudgetAPI struct {
	mockCostSourceClient

	calls atomic.Int32
}

func (c *countingBudgetAPI) GetBudgets(
	ctx context.Context,
	req *pbc.GetBudgetsRequest,
	opts ...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	c.calls.Add(1)
	return c.mockCostSourceClient.GetBudgets(ctx, req, opts...)
}

func budgetPlugin(name string, api *countingBudgetAPI) *pluginhost.Client {
	return &pluginhost.Client{
		Name:     name,
		API:      api,
		Metadata: &proto.PluginMetadata{Capabilities: []string{pluginhost.CapabilityBudgets}},
	}
}

func TestGetBudgetsSkipsPluginsThatDoNotAdvertiseBudgets(t *testing.T) {
	t.Parallel()

	silent := &countingBudgetAPI{}
	talking := &countingBudgetAPI{mockCostSourceClient: mockCostSourceClient{budgets: []*pbc.Budget{{
		Id:     "b1",
		Source: "aws-budgets",
		Amount: &pbc.BudgetAmount{Limit: 10},
		Status: &pbc.BudgetStatus{CurrentSpend: 1},
	}}}}
	eng := New([]*pluginhost.Client{
		nil,
		{Name: "aws-public", API: silent},
		budgetPlugin("budgets", talking),
	}, nil)

	result, err := eng.GetBudgets(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, result.Budgets, 1)
	assert.Equal(t, "b1", result.Budgets[0].GetId())
	assert.Empty(t, result.Errors)
	assert.Zero(t, silent.calls.Load())
	assert.Equal(t, int32(1), talking.calls.Load())
}

func TestGetBudgetsUnimplementedIsNotAnError(t *testing.T) {
	t.Parallel()

	api := &countingBudgetAPI{mockCostSourceClient: mockCostSourceClient{
		err: status.Error(codes.Unimplemented, "GetBudgets not implemented"),
	}}
	eng := New([]*pluginhost.Client{budgetPlugin("aws-public", api)}, nil)

	result, err := eng.GetBudgets(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Budgets)
	assert.Empty(t, result.Errors)
	assert.Equal(t, int32(1), api.calls.Load())
}

func TestGetBudgetsKeepsOtherPluginErrors(t *testing.T) {
	t.Parallel()

	api := &countingBudgetAPI{mockCostSourceClient: mockCostSourceClient{
		err: status.Error(codes.Unavailable, "down"),
	}}
	eng := New([]*pluginhost.Client{budgetPlugin("aws-public", api)}, nil)

	result, err := eng.GetBudgets(context.Background(), nil)
	require.Error(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "Unavailable")
	assert.Equal(t, int32(1), api.calls.Load())
}
