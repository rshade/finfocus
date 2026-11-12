// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const usageOnlyDeclineReason = "prometheus plugin reports usage only"

func capsMeta(caps ...string) *proto.PluginMetadata {
	return &proto.PluginMetadata{Capabilities: caps}
}

func kubernetesWorkload() ResourceDescriptor {
	return ResourceDescriptor{
		Type:     "kubernetes:core/v1:ConfigMap",
		ID:       "cm-001",
		Provider: "kubernetes",
	}
}

// A usage-source plugin advertises no provider (every provider) and only
// usage_stats. It must not be asked to price, so its decline never joins the
// note of a resource another plugin declined.
func TestFeatureCapability_UsageOnlyPluginOmittedFromProjectedNote(t *testing.T) {
	t.Parallel()

	promAPI := newCallClient(false, usageOnlyDeclineReason)
	kubeAPI := newCallClient(false, kubernetesDeclineReason)
	prom := clientWithMeta("prometheus", capsMeta(pluginhost.CapabilityUsageStats), promAPI)
	kube := clientWithMeta("kubernetes", &proto.PluginMetadata{
		Capabilities: []string{
			pluginhost.CapabilityUsageStats,
			pluginhost.CapabilityAllocation,
			"projected_costs",
		},
		SupportedProviders: []string{"kubernetes"},
	}, kubeAPI)
	e := New([]*pluginhost.Client{kube, prom}, nil)

	results, err := e.GetProjectedCost(context.Background(), []ResourceDescriptor{kubernetesWorkload()})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, noteNoPricingInfo+" (declined by kubernetes: "+kubernetesDeclineReason+")", results[0].Notes)
	assert.NotContains(t, results[0].Notes, "prometheus")
	assertNoCostRPCs(t, promAPI)
	kubeSupports, _, _, _ := kubeAPI.counts()
	assert.Positive(t, kubeSupports)
}

// Each routed feature offers a plugin only when it advertises the capability
// the Supports request names. A plugin that advertises nothing stays offered,
// as the router's capability check does.
func TestFeatureCapability_SelectionFollowsAdvertisedCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		feature string
		caps    []string
		offered bool
	}{
		{"usage only projected", "ProjectedCosts", []string{pluginhost.CapabilityUsageStats}, false},
		{"usage only actual", actualCostsFeature, []string{pluginhost.CapabilityUsageStats}, false},
		{"usage only batch", batchCostFeature, []string{pluginhost.CapabilityUsageStats}, false},
		{"usage only recommendations", recommendationsFeature, []string{pluginhost.CapabilityUsageStats}, false},
		{"usage only carbon", carbonFeature, []string{pluginhost.CapabilityUsageStats}, false},
		{"projected only asked projected", "ProjectedCosts", []string{"projected_costs"}, true},
		{"projected only asked batch projected", batchCostFeature, []string{"projected_costs"}, true},
		{"projected only not asked actual", actualCostsFeature, []string{"projected_costs"}, false},
		{"projected only not asked recommendations", recommendationsFeature, []string{"projected_costs"}, false},
		{"actual only asked actual", actualCostsFeature, []string{"actual_costs"}, true},
		{"actual only not asked projected", "ProjectedCosts", []string{"actual_costs"}, false},
		{"batch without projected not asked batch", batchCostFeature, []string{"batch_cost"}, false},
		{"recommendations asked recommendations", recommendationsFeature, []string{"recommendations"}, true},
		{"scorer and recommendations not asked projected", "ProjectedCosts", []string{
			pluginhost.CapabilityRecommendationScoring, capabilityRecommendations,
		}, false},
		{"carbon asked carbon", carbonFeature, []string{"carbon"}, true},
		{"carbon only not asked projected", "ProjectedCosts", []string{"carbon"}, false},
		{"no capabilities asked projected", "ProjectedCosts", nil, true},
		{"no capabilities asked actual", actualCostsFeature, nil, true},
		{"no capabilities asked recommendations", recommendationsFeature, nil, true},
		{"no capabilities not asked carbon", carbonFeature, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := newCallClient(true, "")
			e := New([]*pluginhost.Client{clientWithMeta("plugin", capsMeta(tt.caps...), api)}, nil)

			matches, declines := e.selectPluginMatchesForResource(context.Background(), pricedResource(), tt.feature)
			assert.Empty(t, declines)
			supports, _, _, _ := api.counts()
			if tt.offered {
				assert.Equal(t, []string{"plugin"}, matchNames(matches))
				assert.Positive(t, supports)
				return
			}
			require.NotNil(t, matches)
			assert.Empty(t, matches)
			assert.Zero(t, supports)
		})
	}
}

// An explicit route that names only a plugin that cannot serve the feature
// stays empty: it does not fall back to every other client.
func TestFeatureCapability_ExplicitRouteToUsageOnlyDoesNotFallBack(t *testing.T) {
	t.Parallel()

	promAPI := newCallClient(true, "")
	otherAPI := newCallClient(true, "")
	prom := clientWithMeta("prometheus", capsMeta(pluginhost.CapabilityUsageStats), promAPI)
	other := clientWithMeta("aws", nil, otherAPI)
	router := &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return []PluginMatch{{Client: prom, Priority: 10, Fallback: false}}
		},
	}
	e := New([]*pluginhost.Client{prom, other}, nil).WithRouter(router)

	matches, declines := e.selectPluginMatchesForResource(context.Background(), pricedResource(), "ProjectedCosts")
	require.NotNil(t, matches)
	assert.Empty(t, matches)
	assert.Empty(t, declines)
	assertNoCostRPCs(t, promAPI)
	assertNoCostRPCs(t, otherAPI)
}

// When the router matches nothing, the fallback to every client applies the
// same capability rule.
func TestFeatureCapability_RouterFallbackSkipsUsageOnly(t *testing.T) {
	t.Parallel()

	promAPI := newCallClient(false, usageOnlyDeclineReason)
	awsAPI := newCallClient(true, "")
	prom := clientWithMeta("prometheus", capsMeta(pluginhost.CapabilityUsageStats), promAPI)
	aws := clientWithMeta("aws", capsMeta("projected_costs", "actual_costs"), awsAPI)
	router := &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return nil
		},
	}
	e := New([]*pluginhost.Client{aws, prom}, nil).WithRouter(router)

	matches, declines := e.selectPluginMatchesForResource(context.Background(), pricedResource(), "ProjectedCosts")
	assert.Equal(t, []string{"aws"}, matchNames(matches))
	assert.Empty(t, declines)
	assertNoCostRPCs(t, promAPI)
}

// Batch grouping routes on BatchCost and checks the query it serves. An
// actual-cost batch must not pick a plugin that advertises only projected
// cost, even when it is batch-capable.
func TestFeatureCapability_BatchGroupingFollowsQueryCapability(t *testing.T) {
	t.Parallel()

	api := newCallClient(true, "")
	client := clientWithMeta("money", capsMeta("projected_costs", batchCostCapability), api)
	e := New([]*pluginhost.Client{client}, nil)
	resources := []ResourceDescriptor{pricedResource()}

	projected := e.groupResourcesByPlugin(context.Background(), resources, pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED)
	require.Contains(t, projected, "money")
	assert.True(t, projected["money"].hasBatch)

	actual := e.groupResourcesByPlugin(context.Background(), resources, pbc.CostQueryType_COST_QUERY_TYPE_ACTUAL)
	assert.Empty(t, actual)
}
