// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package pluginhost_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestClientAdvertisesCapability(t *testing.T) {
	t.Parallel()

	projected := pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS
	tests := []struct {
		name       string
		client     *pluginhost.Client
		capability pbc.PluginCapability
		want       bool
	}{
		{"nil client lists nothing", nil, projected, true},
		{"nil metadata lists nothing", &pluginhost.Client{}, projected, true},
		{"empty list lists nothing", clientWithCaps(), projected, true},
		{"listed snake_case name", clientWithCaps("projected_costs"), projected, true},
		{"listed PascalCase alias", clientWithCaps("ProjectedCosts"), projected, true},
		{"batch PascalCase alias", clientWithCaps("BatchCost"),
			pbc.PluginCapability_PLUGIN_CAPABILITY_BATCH_COST, true},
		{"dry run PascalCase alias", clientWithCaps("DryRun"),
			pbc.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN, true},
		{"pricing spec snake_case", clientWithCaps("pricing_spec"),
			pbc.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC, true},
		{"pricing spec PascalCase alias", clientWithCaps("PricingSpec"),
			pbc.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC, true},
		{"pricing spec not listed", clientWithCaps("projected_costs"),
			pbc.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC, false},
		{"listed only capabilities core drops", &pluginhost.Client{
			Metadata: &proto.PluginMetadata{CapabilitiesDeclared: true},
		}, projected, false},
		{"usage only lacks projected", clientWithCaps(pluginhost.CapabilityUsageStats), projected, false},
		{"projected lacks actual", clientWithCaps("projected_costs"),
			pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS, false},
		{"usage stats listed", clientWithCaps(pluginhost.CapabilityUsageStats),
			pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS, true},
		{"unspecified asks nothing", clientWithCaps(pluginhost.CapabilityUsageStats),
			pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED, true},
		{"capability core does not name", clientWithCaps(pluginhost.CapabilityUsageStats),
			pbc.PluginCapability_PLUGIN_CAPABILITY_CONTRACT_COMMITMENTS, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.client.AdvertisesCapability(tt.capability))
		})
	}
}

func clientWithCaps(caps ...string) *pluginhost.Client {
	return &pluginhost.Client{Metadata: &proto.PluginMetadata{Capabilities: caps}}
}
