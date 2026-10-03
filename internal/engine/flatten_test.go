// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/proto"
)

func TestConvertToProtoDottedKeys(t *testing.T) {
	t.Parallel()

	sentinel := proto.UnknownPulumiValue
	okKey := strings.Repeat("a", 126)
	longKey := strings.Repeat("b", 127)

	tests := []struct {
		name       string
		properties map[string]any
		want       map[string]string
		has        map[string]string
		absent     []string
		wantLen    int
	}{
		{
			name:       "scalar leaf adds no dotted key",
			properties: map[string]any{"instanceType": "t3.micro", "region": "us-east-1"},
			want: map[string]string{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		},
		{
			name: "nested map",
			properties: map[string]any{
				"sku": map[string]any{"name": "P1v3", "tier": "PremiumV3"},
			},
			want: map[string]string{
				"sku":      "P1v3",
				"sku.name": "P1v3",
				"sku.tier": "PremiumV3",
			},
		},
		{
			name: "array index",
			properties: map[string]any{
				"securityGroups": []any{"sg-1", "sg-2"},
			},
			want: map[string]string{
				"securityGroups":   "sg-1,sg-2",
				"securityGroups.0": "sg-1",
				"securityGroups.1": "sg-2",
			},
		},
		{
			name: "empty map and array",
			properties: map[string]any{
				"emptyMap": map[string]any{},
				"emptyArr": []any{},
				"nested":   map[string]any{"empty": map[string]any{}, "items": []any{}},
			},
			has: map[string]string{
				"emptyMap": "map[]",
				"emptyArr": "",
			},
			absent: []string{"emptyMap.x", "nested.empty", "nested.items", "nested.items.0"},
		},
		{
			name: "nil leaf and empty string",
			properties: map[string]any{
				"box": map[string]any{"gone": nil, "stay": "yes", "note": ""},
			},
			has:    map[string]string{"box.stay": "yes", "box.note": ""},
			absent: []string{"box.gone"},
		},
		{
			name: "number and bool formatting",
			properties: map[string]any{
				"sku": map[string]any{"capacity": float64(2), "ratio": 1.5, "ready": true},
			},
			has: map[string]string{
				"sku.capacity": "2",
				"sku.ratio":    "1.5",
				"sku.ready":    "true",
			},
		},
		{
			name: "underscore segments stay collapsed only",
			properties: map[string]any{
				"__defaults": map[string]any{"x": "hidden"},
				"ok":         map[string]any{"__meta": map[string]any{"y": "no"}, "z": "yes"},
			},
			has:    map[string]string{"__defaults": "hidden", "ok.z": "yes"},
			absent: []string{"__defaults.x", "ok.__meta", "ok.__meta.y"},
		},
		{
			name: "unknown sentinel",
			properties: map[string]any{
				"ref": map[string]any{"id": sentinel, "name": "keep"},
			},
			has:    map[string]string{"ref": sentinel, "ref.name": "keep"},
			absent: []string{"ref.id"},
		},
		{
			name: "credential-like keys",
			properties: map[string]any{
				"osProfile": map[string]any{
					"adminPassword": "hunter2",
					"AdminPassword": "nope",
					"privateKey":    "pem",
					"sessionToken":  "tok",
					"dbSecret":      "s",
					"cipherText":    "c",
					"credentialId":  "id",
					"computerName":  "vm",
					"adminUsername": "azureuser",
				},
			},
			has: map[string]string{
				"osProfile.computerName":  "vm",
				"osProfile.adminUsername": "azureuser",
			},
			absent: []string{
				"osProfile.adminPassword",
				"osProfile.AdminPassword",
				"osProfile.privateKey",
				"osProfile.sessionToken",
				"osProfile.dbSecret",
				"osProfile.cipherText",
				"osProfile.credentialId",
			},
		},
		{
			name: "skipped tag containers",
			properties: map[string]any{
				"tags":    map[string]any{"env": "prod"},
				"tagsAll": map[string]any{"env": "prod"},
				"meta": map[string]any{
					"labels":      map[string]any{"app": "web"},
					"annotations": map[string]any{"note": "x"},
					"name":        "ok",
					"tags":        "scalar-tags",
				},
			},
			has: map[string]string{
				"tags":      "prod",
				"meta.name": "ok",
				"meta.tags": "scalar-tags",
			},
			absent: []string{
				"tags.env",
				"tagsAll.env",
				"meta.labels.app",
				"meta.annotations.note",
			},
		},
		{
			name: "depth cap",
			properties: map[string]any{
				"deep6": nestedValue(6, "kept"),
				"deep7": nestedValue(7, "dropped"),
			},
			has:    map[string]string{"deep6.n.n.n.n.n": "kept"},
			absent: []string{"deep7.n.n.n.n.n.n"},
		},
		{
			name: "key and value length caps",
			properties: map[string]any{
				"p": map[string]any{
					okKey:   "ok",
					longKey: "no",
				},
				"wrap": map[string]any{
					"ok":  strings.Repeat("x", 256),
					"big": strings.Repeat("y", 257),
				},
			},
			has: map[string]string{
				"p." + okKey: "ok",
				"wrap.ok":    strings.Repeat("x", 256),
			},
			absent: []string{"p." + longKey, "wrap.big"},
		},
		{
			name:       "cap keeps shallower dotted keys",
			properties: dottedCapProperties(),
			wantLen:    maxFlattenedTags,
			has:        map[string]string{"n.a00": "v", "n.a48": "v"},
			absent:     []string{"n.deep.x"},
		},
		{
			name: "dotted key does not replace an existing key",
			properties: map[string]any{
				"sku.name": "keep",
				"sku":      map[string]any{"name": "other", "capacity": float64(2)},
			},
			want: map[string]string{
				"sku.name":     "keep",
				"sku":          "other",
				"sku.capacity": "2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ConvertToProto(tt.properties)
			again := ConvertToProto(tt.properties)
			assert.Equal(t, got, again)
			if tt.want != nil {
				assert.Equal(t, tt.want, got)
			}
			for key, value := range tt.has {
				assert.Equal(t, value, got[key], key)
			}
			for _, key := range tt.absent {
				assert.NotContains(t, got, key)
			}
			if tt.wantLen != 0 {
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestConvertToProtoGoldenPreviewObjects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		collapsed map[string]string
		dotted    map[string]string
	}{
		{
			name: "azure-native sku",
			raw:  `{"sku":{"name":"P1v3","capacity":2,"tier":"PremiumV3"}}`,
			collapsed: map[string]string{
				"sku": "P1v3",
			},
			dotted: map[string]string{
				"sku.name":     "P1v3",
				"sku.capacity": "2",
				"sku.tier":     "PremiumV3",
			},
		},
		{
			name: "hardwareProfile",
			raw:  `{"hardwareProfile":{"vmSize":"Standard_D4s_v5"}}`,
			collapsed: map[string]string{
				"hardwareProfile": "Standard_D4s_v5",
			},
			dotted: map[string]string{
				"hardwareProfile.vmSize": "Standard_D4s_v5",
			},
		},
		{
			name: "rootBlockDevice",
			raw:  `{"rootBlockDevice":[{"volumeSize":20,"volumeType":"gp3"}]}`,
			collapsed: map[string]string{
				"rootBlockDevice": "map[volumeSize:20 volumeType:gp3]",
			},
			dotted: map[string]string{
				"rootBlockDevice.0.volumeSize": "20",
				"rootBlockDevice.0.volumeType": "gp3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var properties map[string]any
			require.NoError(t, json.Unmarshal([]byte(tt.raw), &properties))
			got := ConvertToProto(properties)
			for key, value := range tt.collapsed {
				assert.Equal(t, value, got[key])
			}
			for key, value := range tt.dotted {
				assert.Equal(t, value, got[key])
			}
		})
	}
}

func TestConvertToProtoKeepsCollapsedKeys(t *testing.T) {
	t.Parallel()

	properties := map[string]any{
		"instanceType": "t3.micro",
		"__defaults":   map[string]any{"hidden": true},
		"sku":          map[string]any{"name": "P1v3", "capacity": float64(2), "tier": "Premium"},
		"hardwareProfile": map[string]any{
			"vmSize": "Standard_D4s_v5",
		},
		"rootBlockDevice": []any{
			map[string]any{"volumeSize": float64(20), "volumeType": "gp3"},
		},
		"tags":     map[string]any{"env": "prod"},
		"emptyMap": map[string]any{},
		"emptyArr": []any{},
		"count":    float64(3),
		"enabled":  false,
	}

	got := ConvertToProto(properties)
	for key, value := range properties {
		assert.Equal(t, ConvertValueToString(value), got[key], key)
	}
	assert.Equal(t, "2", got["sku.capacity"])
	assert.Equal(t, "Standard_D4s_v5", got["hardwareProfile.vmSize"])
	assert.Equal(t, "gp3", got["rootBlockDevice.0.volumeType"])
	assert.NotContains(t, got, "tags.env")
	assert.Greater(t, len(got), len(properties))
}

func TestConvertToProtoTagCapKeepsExistingKeys(t *testing.T) {
	t.Parallel()

	nested := map[string]any{}
	for i := 0; i < 80; i++ {
		nested[fmt.Sprintf("k%02d", i)] = "v"
	}
	properties := map[string]any{"nested": nested}
	for i := 0; i < 10; i++ {
		properties[fmt.Sprintf("s%02d", i)] = "scalar"
	}

	got := ConvertToProto(properties)
	assert.Len(t, got, maxFlattenedTags)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("s%02d", i)
		assert.Equal(t, "scalar", got[key])
	}
	assert.Equal(t, ConvertValueToString(nested), got["nested"])
	for i := 0; i < 39; i++ {
		assert.Equal(t, "v", got[fmt.Sprintf("nested.k%02d", i)])
	}
	assert.NotContains(t, got, "nested.k39")
	assert.NotContains(t, got, "nested.k79")
}

func TestConvertToProtoKeepsExistingKeysPastCap(t *testing.T) {
	t.Parallel()

	properties := map[string]any{}
	for i := 0; i < 60; i++ {
		properties[fmt.Sprintf("s%02d", i)] = "v"
	}
	properties["nested"] = map[string]any{"a": "b"}

	got := ConvertToProto(properties)
	assert.Len(t, got, 61)
	assert.Equal(t, "b", got["nested"])
	assert.NotContains(t, got, "nested.a")
	for i := 0; i < 60; i++ {
		assert.Equal(t, "v", got[fmt.Sprintf("s%02d", i)])
	}
}

func TestProjectedCacheKeySplitsNestedSKUCapacity(t *testing.T) {
	t.Parallel()

	resource := func(capacity float64) ResourceDescriptor {
		return ResourceDescriptor{
			Type:     "azure-native:web:AppServicePlan",
			Provider: "azure-native",
			ID:       "urn:plan",
			Properties: map[string]any{
				"sku": map[string]any{"name": "P1v3", "capacity": capacity, "tier": "PremiumV3"},
			},
		}
	}

	left, err := generateProjectedCostResourceKey(resource(2))
	require.NoError(t, err)
	right, err := generateProjectedCostResourceKey(resource(4))
	require.NoError(t, err)

	prefix := cache.BuildProjectedKey("azure-native", "azure-native:web:AppServicePlan", "", "")
	assert.True(t, strings.HasPrefix(left, prefix+"/tags-"), left)
	assert.True(t, strings.HasPrefix(right, prefix+"/tags-"), right)
	assert.NotContains(t, left, "/refs-")
	assert.NotEqual(t, left, right)
}

func TestProjectedCacheKeyLengthPrefixAvoidsDelimiterCollision(t *testing.T) {
	t.Parallel()

	resource := func(properties map[string]any) ResourceDescriptor {
		return ResourceDescriptor{
			Type:       "azure-native:web:AppServicePlan",
			Provider:   "azure-native",
			ID:         "urn:plan",
			Properties: properties,
		}
	}
	// Joining key=value with "|" hashes these two maps to the same digest.
	joined := resource(map[string]any{"a": "b|c=d"})
	split := resource(map[string]any{"a": "b", "c": "d"})

	left, err := generateProjectedCostResourceKey(joined)
	require.NoError(t, err)
	right, err := generateProjectedCostResourceKey(split)
	require.NoError(t, err)
	assert.NotEqual(t, left, right)
}

func TestRecommendationsCacheKeyDiffersByNestedSKUCapacity(t *testing.T) {
	t.Parallel()

	// Existing recommendation cache entries miss once. The key hashes
	// ConvertToProto, which now includes dotted leaves such as sku.capacity.
	eng := &Engine{}
	resource := func(capacity float64) ResourceDescriptor {
		return ResourceDescriptor{
			ID:       "urn:plan",
			Provider: "azure-native",
			Type:     "azure-native:web:AppServicePlan",
			Properties: map[string]any{
				"sku": map[string]any{"name": "P1v3", "capacity": capacity},
			},
		}
	}

	left := eng.generateRecommendationsCacheKey([]ResourceDescriptor{resource(2)}, nil)
	right := eng.generateRecommendationsCacheKey([]ResourceDescriptor{resource(4)}, nil)
	assert.NotEqual(t, left, right)
	assert.Equal(t, "P1v3", ConvertToProto(resource(2).Properties)["sku"])
	assert.Equal(t, "P1v3", ConvertToProto(resource(4).Properties)["sku"])
	assert.Equal(t, "2", ConvertToProto(resource(2).Properties)["sku.capacity"])
	assert.Equal(t, "4", ConvertToProto(resource(4).Properties)["sku.capacity"])
}

func nestedValue(depth int, leaf any) any {
	if depth <= 1 {
		return leaf
	}
	return map[string]any{"n": nestedValue(depth-1, leaf)}
}

func dottedCapProperties() map[string]any {
	nested := map[string]any{}
	for i := 0; i < 49; i++ {
		nested[fmt.Sprintf("a%02d", i)] = "v"
	}
	nested["deep"] = map[string]any{"x": "lose"}
	return map[string]any{"n": nested}
}
