// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package pluginhost

import (
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// AdvertisesCapability reports whether core may ask the plugin for capability.
// A plugin with no metadata, or one that listed no capability at all, predates
// GetPluginInfo capabilities and is treated as advertising every capability. A
// plugin that listed only capabilities core drops (COST_PARSE) made a
// declaration and advertises none of the cost features.
// UNSPECIFIED, and a capability core has no name for, ask nothing and return
// true. Listed names are compared after normalization, so a legacy feature
// name such as "ProjectedCosts" counts as projected_costs.
func (c *Client) AdvertisesCapability(capability pbc.PluginCapability) bool {
	if capability == pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED ||
		c == nil || c.Metadata == nil ||
		(len(c.Metadata.Capabilities) == 0 && !c.Metadata.CapabilitiesDeclared) {
		return true
	}
	required := ConvertCapabilities([]pbc.PluginCapability{capability})
	if len(required) == 0 {
		return true
	}
	for _, listed := range c.Metadata.Capabilities {
		if normalizeCapabilityName(listed) == required[0] {
			return true
		}
	}
	return false
}

// normalizeCapabilityName maps a listed capability to the name
// ConvertCapabilities gives it. Names it does not know are returned as is.
func normalizeCapabilityName(name string) string {
	capability, ok := capabilityFromName(name)
	if !ok {
		return name
	}
	if converted := ConvertCapabilities([]pbc.PluginCapability{capability}); len(converted) == 1 {
		return converted[0]
	}
	return name
}

// capabilityFromName accepts both the routing feature name and the normalized
// capability name for the capabilities a routing feature can name.
func capabilityFromName(name string) (pbc.PluginCapability, bool) {
	switch name {
	case "ProjectedCosts", "projected_costs":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS, true
	case "ActualCosts", "actual_costs":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS, true
	case "Recommendations", "recommendations":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS, true
	case "Carbon", "carbon":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON, true
	case "DryRun", "dry_run":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN, true
	case "Budgets", CapabilityBudgets:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_BUDGETS, true
	case "BatchCost", "batch_cost":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_BATCH_COST, true
	case "PricingSpec", "pricing_spec":
		return pbc.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC, true
	default:
		return pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED, false
	}
}
