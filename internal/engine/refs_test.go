// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protobuf "google.golang.org/protobuf/proto"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/proto"
)

func TestBatchAndDirectRequestsSharePreparedDescriptor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resources := nativePlanWithApps()
	ApplyCrossResourceRefs(ctx, resources)

	indexed := make([]indexedResource, len(resources))
	for i, resource := range resources {
		indexed[i] = indexedResource{index: i, resource: resource}
	}
	built := buildBatchCostRequest(ctx, indexed, batchOptions{
		queryType: pbc.CostQueryType_COST_QUERY_TYPE_PROJECTED,
	})
	require.NotNil(t, built.request)
	require.Len(t, built.request.GetResources(), len(resources))

	for i, ir := range built.validResources {
		got := built.request.GetResources()[i]
		want := proto.PrepareProjectedDescriptor(
			ctx, ir.resource.ID, ir.resource.Provider, ir.resource.Type, ConvertToProto(ir.resource.Properties),
			BuildAttributes(ctx, ir.resource.Properties),
		)
		assert.Equal(t, want.GetSku(), got.GetSku())
		assert.True(t, protobuf.Equal(want.GetAttributes(), got.GetAttributes()))
		assert.Equal(t, want.GetRegion(), got.GetRegion())
		assert.Equal(t, want.GetTags(), got.GetTags())
	}

	var results []CostResult
	for _, descriptor := range built.request.GetResources() {
		results = append(results, CostResult{
			ResourceID: descriptor.GetId(),
			Currency:   "USD",
			Monthly:    pricePlanSKU(descriptor.GetSku()),
		})
		for _, value := range descriptor.GetTags() {
			assert.NotContains(t, value, proto.UnknownPulumiValue)
		}
	}
	total := AggregateResults(results)
	require.NotNil(t, total)
	assert.InDelta(t, 100.0, total.Summary.TotalMonthly, 0.001)
	assert.Empty(t, built.request.GetResources()[1].GetSku())
	assert.Empty(t, built.request.GetResources()[2].GetSku())
	assert.Equal(t, "P1v3", built.request.GetResources()[1].GetTags()["ref.serverFarmId.sku"])
	assert.Equal(t, "P1v3", built.request.GetResources()[2].GetTags()["ref.serverFarmId.sku"])
}

func TestProjectedCacheKeyIncludesRefTags(t *testing.T) {
	t.Parallel()

	plain := ResourceDescriptor{
		Type: "azure-native:web:AppServicePlan", Provider: "azure-native", ID: "urn:plan",
		Properties: map[string]any{"location": "westeurope", "sku": "P1v3"},
	}
	key, err := generateProjectedCostResourceKey(plain)
	require.NoError(t, err)
	base := cache.BuildProjectedKey(plain.Provider, plain.Type, "", "P1v3")
	assert.True(t, strings.HasPrefix(key, base+"/tags-"), key)
	assert.NotContains(t, key, "/refs-")

	withRef := plain
	withRef.Properties = map[string]any{
		"location":                "westeurope",
		"ref.serverFarmId.sku":    "P1v3",
		"ref.serverFarmId.region": "westeurope",
	}
	refKey, err := generateProjectedCostResourceKey(withRef)
	require.NoError(t, err)
	assert.NotEqual(t, key, refKey)
	assert.Contains(t, refKey, "/tags-")
	assert.Contains(t, refKey, "/refs-")

	changed := withRef
	changed.Properties = map[string]any{
		"location":                "westeurope",
		"ref.serverFarmId.sku":    "S1",
		"ref.serverFarmId.region": "westeurope",
	}
	changedKey, err := generateProjectedCostResourceKey(changed)
	require.NoError(t, err)
	assert.NotEqual(t, refKey, changedKey)
}

func TestUnresolvedReferencesEmitNoTags(t *testing.T) {
	t.Parallel()

	resources := []ResourceDescriptor{
		{
			ID: "urn:child", Provider: "azure", Type: "azure:example/child:Child",
			Properties: map[string]any{"name": "child"},
			Refs: map[string][]string{
				"name":  {},
				"gone":  {"urn:missing"},
				"multi": {"urn:a", "urn:b"},
			},
		},
	}
	ApplyCrossResourceRefs(context.Background(), resources)
	for key := range resources[0].Properties {
		assert.NotContains(t, key, "ref.")
	}
}

func nativePlanWithApps() []ResourceDescriptor {
	planURN := "urn:pulumi:dev::demo::azure-native:web:AppServicePlan::nativePlan"
	return []ResourceDescriptor{
		{
			ID: planURN, Provider: "azure-native", Type: "azure-native:web:AppServicePlan",
			Properties: map[string]any{
				"location": "westeurope",
				"sku":      map[string]any{"name": "P1v3", "tier": "PremiumV3"},
			},
		},
		nativeApp("urn:app-a", planURN),
		nativeApp("urn:app-b", planURN),
	}
}

func nativeApp(urn, planURN string) ResourceDescriptor {
	return ResourceDescriptor{
		ID: urn, Provider: "azure-native", Type: "azure-native:web:WebApp",
		Properties: map[string]any{
			"location":     "westeurope",
			"serverFarmId": proto.UnknownPulumiValue,
		},
		Refs: map[string][]string{"serverFarmId": {planURN}},
	}
}

func pricePlanSKU(sku string) float64 {
	if sku == "P1v3" {
		return 100
	}
	return 0
}

// deepWorkload nests a value seven segments deep (spec.a.b.c.d.e.f), past the
// six-segment dotted-tag cap, so only attributes can tell two of them apart.
func deepWorkload(leaf string) ResourceDescriptor {
	return ResourceDescriptor{
		Type: "kubernetes:apps/v1:Deployment", Provider: "kubernetes", ID: "urn:web",
		Properties: map[string]any{"spec": map[string]any{
			"name": "web",
			"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": map[string]any{
				"e": map[string]any{"f": leaf},
			}}}},
		}},
	}
}

func TestProjectedCacheKeyIncludesAttributes(t *testing.T) {
	t.Parallel()

	t.Run("values below the tag depth cap change the key", func(t *testing.T) {
		t.Parallel()
		small, large := deepWorkload("500m"), deepWorkload("4")
		require.Equal(t,
			tagCacheSuffix(ConvertToProto(small.Properties)),
			tagCacheSuffix(ConvertToProto(large.Properties)),
			"fixture must collide on tags alone",
		)

		smallKey, err := generateProjectedCostResourceKey(small)
		require.NoError(t, err)
		largeKey, err := generateProjectedCostResourceKey(large)
		require.NoError(t, err)

		assert.NotEqual(t, smallKey, largeKey)
		assert.Contains(t, smallKey, "/attrs-")
	})

	t.Run("identical inputs give the same key", func(t *testing.T) {
		t.Parallel()
		want, err := generateProjectedCostResourceKey(deepWorkload("500m"))
		require.NoError(t, err)
		for range 20 {
			got, keyErr := ProjectedResourceCacheKey(deepWorkload("500m"))
			require.NoError(t, keyErr)
			assert.Equal(t, want, got)
		}
	})

	t.Run("no properties means no attrs suffix", func(t *testing.T) {
		t.Parallel()
		key, err := generateProjectedCostResourceKey(ResourceDescriptor{
			Type: "aws:s3/bucket:Bucket", Provider: "aws", ID: "urn:bucket",
		})
		require.NoError(t, err)
		assert.NotContains(t, key, "/attrs-")
	})

	t.Run("attrs sits after refs and before pricing-spec", func(t *testing.T) {
		t.Parallel()
		resource := ResourceDescriptor{
			Type: "azure:appservice/linuxWebApp:LinuxWebApp", Provider: "azure", ID: "urn:app",
			Properties: map[string]any{
				"location":                "westeurope",
				"siteConfig":              map[string]any{"alwaysOn": true},
				"ref.servicePlanId.sku":   "P1v3",
				"ref.servicePlanId.urn":   "urn:plan",
				"ref.servicePlanId.type":  "azure:appservice/servicePlan:ServicePlan",
				"ref.servicePlanId.other": "x",
			},
		}
		eng := New(nil, nil)
		eng.pricingSpecFallback = true

		key, err := eng.projectedCostCacheKey(resource)
		require.NoError(t, err)

		tags := strings.Index(key, "/tags-")
		refs := strings.Index(key, "/refs-")
		attrs := strings.Index(key, "/attrs-")
		spec := strings.Index(key, pricingSpecCacheSuffix)
		require.True(t, tags >= 0 && refs >= 0 && attrs >= 0 && spec >= 0, key)
		assert.Less(t, tags, refs)
		assert.Less(t, refs, attrs)
		assert.Less(t, attrs, spec)
		assert.True(t, strings.HasSuffix(key, pricingSpecCacheSuffix), key)
	})
}
