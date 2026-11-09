// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestFallbackHonorsAdvertisedProviders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		providers        []string
		nilMeta          bool
		resourceType     string
		resourceProvider string
		want             bool
	}{
		{
			name:         "excluded provider",
			providers:    []string{"aws"},
			resourceType: "tailscale:index/device:Device",
		},
		{
			name:         "matching provider",
			providers:    []string{"aws"},
			resourceType: "aws:ec2/instance:Instance",
			want:         true,
		},
		{
			name:         "alias package",
			providers:    []string{"aws"},
			resourceType: "aws-native:ec2:Instance",
			want:         true,
		},
		{
			name:         "terraform type",
			providers:    []string{"azure"},
			resourceType: "azurerm_linux_virtual_machine",
			want:         true,
		},
		{
			name:             "terraform custom label",
			providers:        []string{"custom"},
			resourceType:     "cloudflare_zone",
			resourceProvider: "custom",
			want:             true,
		},
		{
			name:             "terraform custom label excludes other clouds",
			providers:        []string{"aws"},
			resourceType:     "cloudflare_zone",
			resourceProvider: "custom",
		},
		{
			name:             "type prefix still matches beside a custom label",
			providers:        []string{"cloudflare"},
			resourceType:     "cloudflare_zone",
			resourceProvider: "custom",
			want:             true,
		},
		{
			name:         "provider case",
			providers:    []string{"AWS"},
			resourceType: "aws:ec2/instance:Instance",
			want:         true,
		},
		{
			name:         "listed package name",
			providers:    []string{"aws-native"},
			resourceType: "aws:ec2/instance:Instance",
			want:         true,
		},
		{
			name:         "multi provider includes",
			providers:    []string{"aws", "gcp"},
			resourceType: "gcp:compute:Instance",
			want:         true,
		},
		{
			name:         "multi provider excludes",
			providers:    []string{"aws", "gcp"},
			resourceType: "civo:index/firewall:Firewall",
		},
		{
			name:         "empty list is unrestricted",
			providers:    []string{},
			resourceType: "random:index/randomString:RandomString",
			want:         true,
		},
		{
			name:         "nil metadata is unrestricted",
			nilMeta:      true,
			resourceType: "command:local:Command",
			want:         true,
		},
		{
			name:         "wildcard is unrestricted",
			providers:    []string{"*"},
			resourceType: "tailscale:index/device:Device",
			want:         true,
		},
		{
			name:         "blank entry is unrestricted",
			providers:    []string{""},
			resourceType: "tailscale:index/device:Device",
			want:         true,
		},
	}

	for _, routed := range []bool{false, true} {
		name := "no router"
		if routed {
			name = "router returned no matches"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					runAdvertisedProviderCase(t, routed, tc)
				})
			}
		})
	}
}

func runAdvertisedProviderCase(t *testing.T, routed bool, tc struct {
	name             string
	providers        []string
	nilMeta          bool
	resourceType     string
	resourceProvider string
	want             bool
}) {
	t.Helper()
	api := newCallClient(true, "")
	var meta *proto.PluginMetadata
	if !tc.nilMeta {
		meta = &proto.PluginMetadata{SupportedProviders: tc.providers}
	}
	client := clientWithMeta("plugin", meta, api)
	eng := New([]*pluginhost.Client{client}, nil)
	if routed {
		eng = eng.WithRouter(&mockRouter{})
	}
	resource := ResourceDescriptor{
		Type: tc.resourceType, ID: "id", Provider: tc.resourceProvider,
	}
	matches, declines := eng.selectPluginMatchesForResource(
		context.Background(), resource, "ProjectedCosts",
	)
	supports, projected, actual, batch := api.counts()
	if tc.want {
		assert.Equal(t, []string{"plugin"}, matchNames(matches))
		assert.Equal(t, 1, supports)
		return
	}
	require.NotNil(t, matches)
	assert.Empty(t, matches)
	assert.Empty(t, declines)
	assert.Zero(t, supports)
	assert.Zero(t, projected)
	assert.Zero(t, actual)
	assert.Zero(t, batch)
}

func TestFallbackKeepsUnrestrictedPlugin(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{Type: "tailscale:index/device:Device", ID: "n1"}
	for _, routed := range []bool{false, true} {
		name := "no router"
		if routed {
			name = "router returned no matches"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			awsAPI := newCallClient(true, "")
			openAPI := newCallClient(true, "")
			aws := clientWithMeta("aws-public", &proto.PluginMetadata{SupportedProviders: []string{"aws"}}, awsAPI)
			open := clientWithMeta("recorder", &proto.PluginMetadata{}, openAPI)
			eng := New([]*pluginhost.Client{aws, open}, nil)
			if routed {
				eng = eng.WithRouter(&mockRouter{})
			}
			matches, _ := eng.selectPluginMatchesForResource(context.Background(), resource, "ProjectedCosts")
			assert.Equal(t, []string{"recorder"}, matchNames(matches))
			assertNoCostRPCs(t, awsAPI)
			supports, _, _, _ := openAPI.counts()
			assert.Equal(t, 1, supports)
		})
	}
}

func TestExplicitRouteStillQueriesExcludedProvider(t *testing.T) {
	t.Parallel()

	api := newCallClient(true, "")
	client := clientWithMeta("aws-public", &proto.PluginMetadata{SupportedProviders: []string{"aws"}}, api)
	router := &mockRouter{
		selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
			return []PluginMatch{{Client: client, Priority: 10, Fallback: true}}
		},
	}
	eng := New([]*pluginhost.Client{client}, nil).WithRouter(router)
	resource := ResourceDescriptor{Type: "tailscale:index/device:Device", ID: "device"}

	matches, _ := eng.selectPluginMatchesForResource(context.Background(), resource, "ProjectedCosts")
	assert.Equal(t, []string{"aws-public"}, matchNames(matches))
	supports, _, _, _ := api.counts()
	assert.Equal(t, 1, supports)
}

func TestGetProjectedCostSkipsForeignProvider(t *testing.T) {
	t.Parallel()

	awsAPI := newCallClient(true, "")
	openAPI := newCallClient(true, "")
	aws := clientWithMeta("aws-public", &proto.PluginMetadata{SupportedProviders: []string{"aws"}}, awsAPI)
	open := clientWithMeta("recorder", nil, openAPI)
	eng := New([]*pluginhost.Client{aws, open}, nil)
	resource := ResourceDescriptor{Type: "tailscale:index/device:Device", ID: "n1"}

	results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{resource})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assertNoCostRPCs(t, awsAPI)
	supports, _, _, _ := openAPI.counts()
	assert.Positive(t, supports)
}
