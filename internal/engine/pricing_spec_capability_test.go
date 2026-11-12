// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// specOnlyClient advertises pricing_spec and not projected_costs. It is asked
// for GetPricingSpec by the projected fallback and by --explain, never for
// GetProjectedCost.
func specOnlyClient(api *pricingSpecPlugin) *pluginhost.Client {
	return &pluginhost.Client{
		Name:     "spec-only",
		API:      api,
		Metadata: &proto.PluginMetadata{Capabilities: []string{"pricing_spec"}},
	}
}

func projectedResult(
	t *testing.T, eng *Engine, withErrors bool,
) CostResult {
	t.Helper()
	ctx := context.Background()
	if withErrors {
		got, err := eng.GetProjectedCostWithErrors(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, got.Results, 1)
		return got.Results[0]
	}
	results, err := eng.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
	require.NoError(t, err)
	require.Len(t, results, 1)
	return results[0]
}

func TestPricingSpecFallbackAsksSpecOnlyPlugin(t *testing.T) {
	t.Parallel()

	for _, withErrors := range []bool{false, true} {
		name := "GetProjectedCost"
		if withErrors {
			name = "GetProjectedCostWithErrors"
		}

		t.Run(name+" uses the spec-only plugin price", func(t *testing.T) {
			t.Parallel()
			spec := &pricingSpecPlugin{spec: hourlyPricingSpec()}
			eng := New([]*pluginhost.Client{specOnlyClient(spec)}, pricingYAMLLoader()).
				WithPricingSpecFallback(true)

			result := projectedResult(t, eng, withErrors)
			assert.Equal(t, adapterPluginSpec, result.Adapter)
			assert.InDelta(t, 73.0, result.Monthly, 1e-9)
			assert.Equal(t, 1, spec.specCalls)
		})

		t.Run(name+" disabled does not ask the spec-only plugin", func(t *testing.T) {
			t.Parallel()
			spec := &pricingSpecPlugin{spec: hourlyPricingSpec()}
			eng := New([]*pluginhost.Client{specOnlyClient(spec)}, pricingYAMLLoader())

			result := projectedResult(t, eng, withErrors)
			assert.Equal(t, "local-spec", result.Adapter)
			assert.Equal(t, 0, spec.specCalls)
		})

		t.Run(name+" keeps the router order after a failed projected plugin", func(t *testing.T) {
			t.Parallel()
			first := &pricingSpecPlugin{projectedErr: errors.New("plugin down")}
			spec := &pricingSpecPlugin{spec: hourlyPricingSpec()}
			firstClient := &pluginhost.Client{
				Name:     "first",
				API:      first,
				Metadata: &proto.PluginMetadata{Capabilities: []string{"projected_costs", "pricing_spec"}},
			}
			specClient := specOnlyClient(spec)
			router := &mockRouter{
				selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
					return []PluginMatch{
						{Client: firstClient, Priority: 20, Fallback: true},
						{Client: specClient, Priority: 10, Fallback: true},
					}
				},
			}
			eng := New([]*pluginhost.Client{firstClient, specClient}, nil).
				WithRouter(router).
				WithPricingSpecFallback(true)

			result := projectedResult(t, eng, withErrors)
			assert.Equal(t, adapterPluginSpec, result.Adapter)
			assert.Equal(t, 1, first.specCalls)
			assert.Equal(t, 1, spec.specCalls)
		})

		t.Run(name+" stopped chain does not ask the spec-only plugin", func(t *testing.T) {
			t.Parallel()
			first := &pricingSpecPlugin{projectedErr: errors.New("plugin down"), spec: hourlyPricingSpec()}
			spec := &pricingSpecPlugin{spec: hourlyPricingSpec()}
			firstClient := &pluginhost.Client{
				Name:     "first",
				API:      first,
				Metadata: &proto.PluginMetadata{Capabilities: []string{"projected_costs", "pricing_spec"}},
			}
			specClient := specOnlyClient(spec)
			router := &mockRouter{
				selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
					return []PluginMatch{
						{Client: firstClient, Priority: 20, Fallback: false},
						{Client: specClient, Priority: 10, Fallback: true},
					}
				},
			}
			eng := New([]*pluginhost.Client{firstClient, specClient}, nil).
				WithRouter(router).
				WithPricingSpecFallback(true)

			result := projectedResult(t, eng, withErrors)
			assert.NotEqual(t, adapterPluginSpec, result.Adapter)
			assert.Equal(t, 0, first.specCalls)
			assert.Equal(t, 0, spec.specCalls)
		})
	}
}

// A projected-only plugin is not asked for its pricing spec: GetPricingSpec
// needs pricing_spec, the same as any other capability.
func TestPricingSpecFallbackSkipsProjectedOnlyPlugin(t *testing.T) {
	t.Parallel()

	projectedOnly := &pricingSpecPlugin{spec: hourlyPricingSpec()}
	eng := New([]*pluginhost.Client{{
		Name:     "projected-only",
		API:      projectedOnly,
		Metadata: &proto.PluginMetadata{Capabilities: []string{"projected_costs"}},
	}}, pricingYAMLLoader()).WithPricingSpecFallback(true)

	result := projectedResult(t, eng, false)
	assert.Equal(t, "local-spec", result.Adapter)
	assert.Equal(t, 0, projectedOnly.specCalls)
}

func TestExplainPricingAsksSpecOnlyPlugin(t *testing.T) {
	t.Parallel()

	price := explainPlugin(nil, nil)
	spec := &pricingSpecPlugin{spec: explainSpec()}
	eng := New([]*pluginhost.Client{
		{
			Name:     "price",
			API:      price,
			Metadata: &proto.PluginMetadata{Capabilities: []string{"projected_costs"}},
		},
		specOnlyClient(spec),
	}, nil).WithExplainPricing(true)

	diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{pricingResource()})
	require.NoError(t, err)
	require.Len(t, diff.Entries, 1)
	assert.InDelta(t, 9.0, diff.Entries[0].After.Monthly, 1e-9)
	require.NotNil(t, diff.Entries[0].PricingSpec)
	assert.Equal(t, "on-demand", diff.Entries[0].PricingSpec.BillingMode)
	assert.Equal(t, 0, price.specCalls)
	assert.Equal(t, 1, spec.specCalls)
}
