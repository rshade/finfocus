// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestProjectedPlanReferencesIgnoreAppendedState(t *testing.T) {
	t.Parallel()
	for _, op := range []string{DiffOperationCreate, DiffOperationUpdate, DiffOperationDelete, DiffOperationSame} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			pool := referencedNodePool(op, "Standard_D4s_v3", "Standard_D2s_v3")
			target := referencedCluster(DiffOperationSame)
			target.Origin = OriginState
			run := func(resources []ResourceDescriptor) ([]map[string]string, []map[string]any) {
				client := &recordingCostClient{}
				_, err := New(
					[]*pluginhost.Client{{Name: "azure", API: client}},
					nil,
				).GetProjectedCostDiff(context.Background(), resources)
				require.NoError(t, err)
				client.mu.Lock()
				defer client.mu.Unlock()
				var properties []map[string]string
				var attributes []map[string]any
				for _, request := range client.projected {
					for _, resource := range request.Resources {
						if resource.ID == nodePoolURN {
							properties = append(properties, resource.Properties)
							attributes = append(attributes, resource.Attributes.AsMap())
						}
					}
				}
				require.NotEmpty(t, properties)
				return properties, attributes
			}
			baselineProps, baselineAttrs := run([]ResourceDescriptor{pool})
			liveProps, liveAttrs := run([]ResourceDescriptor{pool, target})
			require.GreaterOrEqual(t, len(liveProps), len(baselineProps))
			planRequests := len(baselineProps)
			assert.Equal(t, baselineProps, liveProps[:planRequests],
				"state inventory must not change plugin-bound plan properties")
			assert.Equal(t, baselineAttrs, liveAttrs[:planRequests],
				"state inventory must not change plugin-bound plan attributes")
			stackRequests := liveProps[planRequests:]
			if op == DiffOperationDelete {
				assert.Empty(t, stackRequests, "a delete has no after cost in the stack total")
				return
			}
			require.Len(t, stackRequests, 1, "only the stack total reprices with the full inventory")
			assert.Equal(t, "westeurope", stackRequests[0]["ref.kubernetesClusterId.region"])
		})
	}
}

func TestProjectedStateReferencesUsePlanTargets(t *testing.T) {
	t.Parallel()
	pool := referencedNodePool(DiffOperationSame, "Standard_D2s_v3", "")
	pool.Origin = OriginState
	client := newRegionPlugin(map[string]float64{"Standard_D2s_v3": 10})
	_, err := New(
		[]*pluginhost.Client{{Name: "azure", API: client}},
		nil,
	).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{referencedCluster(DiffOperationSame), pool})
	require.NoError(t, err)
	assert.Equal(t, []string{"westeurope"}, client.regionsFor(pool.ID))
}

// regionPricedPlugin prices a node pool by the region its cluster reference
// resolves to, so a dropped reference changes the price.
type regionPricedPlugin struct{ mockCostSourceClient }

func (*regionPricedPlugin) GetProjectedCost(
	_ context.Context, in *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	results := make([]*proto.CostResult, 0, len(in.Resources))
	for _, resource := range in.Resources {
		monthly := 1.0
		if resource.Properties["vmSize"] != "" {
			monthly = 4
			if resource.Properties["ref.kubernetesClusterId.region"] == "westeurope" {
				monthly = 10
			}
		}
		results = append(results, &proto.CostResult{Currency: "USD", MonthlyCost: monthly})
	}
	return &proto.GetProjectedCostResponse{Results: results}, nil
}

func TestProjectedStackTotalUsesFullInventoryReferences(t *testing.T) {
	t.Parallel()
	diff := func(clusterOrigin DescriptorOrigin) *DiffResult {
		pool := referencedNodePool(DiffOperationCreate, "Standard_D4s_v3", "")
		pool.Origin = OriginPlan
		cluster := referencedCluster(DiffOperationSame)
		cluster.Origin = clusterOrigin
		result, err := New(
			[]*pluginhost.Client{{Name: "azure", API: &regionPricedPlugin{}}},
			nil,
		).GetProjectedCostDiff(context.Background(), []ResourceDescriptor{pool, cluster})
		require.NoError(t, err)
		return result
	}
	defaultPreview := diff(OriginState)
	showSames := diff(OriginPlan)
	assert.InDelta(
		t,
		4,
		defaultPreview.Summary.TotalAfter,
		1e-9,
		"the plan total keeps the preview's own reference scope",
	)
	assert.InDelta(t, 11, showSames.Summary.TotalAfter, 1e-9)
	assert.InDelta(t, showSames.Summary.StackTotal, defaultPreview.Summary.StackTotal, 1e-9,
		"the stack total must not depend on whether the preview lists unchanged resources")
}
