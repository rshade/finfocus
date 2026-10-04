// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/proto"
)

// podSpecProperties returns a Deployment's inputs with enough containers and
// environment variables to encode to about 40 KiB.
func podSpecProperties() map[string]any {
	containers := make([]any, 0, 8)
	for c := range 8 {
		env := make([]any, 0, 40)
		for e := range 40 {
			env = append(env, map[string]any{
				"name":  fmt.Sprintf("VAR_%d_%d", c, e),
				"value": strings.Repeat("v", 96),
			})
		}
		containers = append(containers, map[string]any{
			"name":  fmt.Sprintf("app-%d", c),
			"image": "registry.example.com/app:1.2.3",
			"env":   env,
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "500m", "memory": "1Gi"},
				"limits":   map[string]any{"cpu": "1", "memory": "2Gi"},
			},
		})
	}
	return map[string]any{
		"metadata": map[string]any{"name": "web", "labels": map[string]any{"app": "web"}},
		"spec": map[string]any{
			"replicas": float64(3),
			"template": map[string]any{"spec": map[string]any{"containers": containers}},
		},
	}
}

func BenchmarkBuildAttributes(b *testing.B) {
	ctx := context.Background()
	props := podSpecProperties()
	b.ReportAllocs()
	for b.Loop() {
		_ = attrsCacheSuffix(BuildAttributes(ctx, props))
	}
}

// BenchmarkSupportsCacheKeys reports how many Supports calls a plan needs per
// plugin: distinct keys without the attributes digest against the engine's
// keys with it.
func BenchmarkSupportsCacheKeys(b *testing.B) {
	resources := planResources(b, filepath.Join("..", "..", "examples", "plans", "aws-simple-plan.json"))
	ctx := context.Background()
	var before, after int
	for b.Loop() {
		api := &attrsSupportsClient{}
		e, client := supportsEngine(api)
		withoutAttrs := make(map[string]struct{}, len(resources))
		for _, resource := range resources {
			descriptor := proto.PrepareProjectedDescriptor(ctx, resource.ID, resource.Provider, resource.Type,
				ConvertToProto(resource.Properties), nil)
			withoutAttrs[strings.Join([]string{
				resource.Provider, resource.Type, descriptor.GetRegion(), descriptor.GetSku(),
			}, ":")] = struct{}{}
			e.checkPluginSupports(ctx, client, resource, "ProjectedCosts")
		}
		before, after = len(withoutAttrs), len(api.seen)
	}
	b.ReportMetric(float64(before), "keys_before")
	b.ReportMetric(float64(after), "keys_after")
}

// planResources reads a `pulumi preview --json` fixture. ingest imports
// engine, so this reads only the fields the benchmark needs.
func planResources(tb testing.TB, path string) []ResourceDescriptor {
	tb.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(tb, err)
	var plan struct {
		Steps []struct {
			URN    string         `json:"urn"`
			Type   string         `json:"type"`
			Inputs map[string]any `json:"inputs"`
		} `json:"steps"`
	}
	require.NoError(tb, json.Unmarshal(raw, &plan))
	resources := make([]ResourceDescriptor, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		provider, _, _ := strings.Cut(step.Type, ":")
		resources = append(resources, ResourceDescriptor{
			ID: step.URN, Type: step.Type, Provider: provider, Properties: step.Inputs,
		})
	}
	return resources
}
