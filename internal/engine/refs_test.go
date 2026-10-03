// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		)
		assert.Equal(t, want.GetSku(), got.GetSku())
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
