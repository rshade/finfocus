package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func hourlyPricingSpec() *pbc.PricingSpec {
	return &pbc.PricingSpec{BillingMode: billingPerHour, RatePerUnit: 0.1, Currency: "USD", Source: "aws-public"}
}

func TestPricingSpecFallbackSeparatesCacheEntries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a plugin-spec result is not served when the fallback is off", func(t *testing.T) {
		t.Parallel()
		store := newMockCache(true)
		plugin := &pricingSpecPlugin{spec: hourlyPricingSpec()}
		clients := []*pluginhost.Client{{Name: "aws-plugin", API: plugin}}

		on := New(clients, pricingYAMLLoader()).WithCache(store).WithPricingSpecFallback(true)
		results, err := on.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, adapterPluginSpec, results[0].Adapter)

		off := New(clients, pricingYAMLLoader()).WithCache(store)
		results, err = off.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "local-spec", results[0].Adapter)
	})

	t.Run("a yaml result does not hide the spec after enabling the fallback", func(t *testing.T) {
		t.Parallel()
		store := newMockCache(true)
		plugin := &pricingSpecPlugin{spec: hourlyPricingSpec()}
		clients := []*pluginhost.Client{{Name: "aws-plugin", API: plugin}}

		off := New(clients, pricingYAMLLoader()).WithCache(store)
		results, err := off.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Equal(t, "local-spec", results[0].Adapter)
		require.Equal(t, 0, plugin.specCalls)

		on := New(clients, pricingYAMLLoader()).WithCache(store).WithPricingSpecFallback(true)
		results, err = on.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, adapterPluginSpec, results[0].Adapter)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("the exported seeding key stays the default key", func(t *testing.T) {
		t.Parallel()
		resource := pricingResource()
		seeded, err := ProjectedResourceCacheKey(resource)
		require.NoError(t, err)
		off, err := New(nil, nil).projectedCostCacheKey(resource)
		require.NoError(t, err)
		on, err := New(nil, nil).WithPricingSpecFallback(true).projectedCostCacheKey(resource)
		require.NoError(t, err)
		assert.Equal(t, seeded, off)
		assert.NotEqual(t, seeded, on)
	})
}

func TestPricingSpecCallsAreBounded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a hung GetPricingSpec falls through to yaml", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{hangSpec: true}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		eng.pricingSpecTimeout = 50 * time.Millisecond

		start := time.Now()
		results, err := eng.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)

		assert.Less(t, time.Since(start), 2*time.Second)
		require.Len(t, results, 1)
		assert.Equal(t, "local-spec", results[0].Adapter)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("a hung GetPricingSpec does not block discovery", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{hangSpec: true}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		eng.pricingSpecTimeout = 50 * time.Millisecond

		resource := pricingResource()
		start := time.Now()
		found := eng.DiscoverPricingSpec(ctx, &resource)

		assert.Less(t, time.Since(start), 2*time.Second)
		assert.Empty(t, found.Modes)
	})
}

func TestPricingSpecFallbackHonoursRouterNoFallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	run := func(t *testing.T, fallback bool, withErrors bool) (*pricingSpecPlugin, *pricingSpecPlugin, CostResult) {
		t.Helper()
		first := &pricingSpecPlugin{projectedErr: errors.New("plugin down")}
		second := &pricingSpecPlugin{spec: hourlyPricingSpec()}
		firstClient := &pluginhost.Client{Name: "first", API: first}
		secondClient := &pluginhost.Client{Name: "second", API: second}
		router := &mockRouter{
			selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
				return []PluginMatch{
					{Client: firstClient, Priority: 20, Fallback: fallback},
					{Client: secondClient, Priority: 10, Fallback: true},
				}
			},
		}
		eng := New([]*pluginhost.Client{firstClient, secondClient}, nil).
			WithRouter(router).
			WithPricingSpecFallback(true)

		if withErrors {
			got, err := eng.GetProjectedCostWithErrors(ctx, []ResourceDescriptor{pricingResource()})
			require.NoError(t, err)
			require.Len(t, got.Results, 1)
			return first, second, got.Results[0]
		}
		results, err := eng.GetProjectedCost(ctx, []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		return first, second, results[0]
	}

	for _, withErrors := range []bool{false, true} {
		name := "GetProjectedCost"
		if withErrors {
			name = "GetProjectedCostWithErrors"
		}
		t.Run(name+" stops at a no-fallback plugin", func(t *testing.T) {
			t.Parallel()
			first, second, result := run(t, false, withErrors)
			assert.Equal(t, 0, first.specCalls)
			assert.Equal(t, 0, second.specCalls)
			assert.NotEqual(t, adapterPluginSpec, result.Adapter)
		})
		t.Run(name+" still falls back when allowed", func(t *testing.T) {
			t.Parallel()
			_, second, result := run(t, true, withErrors)
			assert.Equal(t, 1, second.specCalls)
			assert.Equal(t, adapterPluginSpec, result.Adapter)
		})
	}
}
