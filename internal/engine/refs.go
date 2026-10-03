// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
)

type refTarget struct {
	desc     *ResourceDescriptor
	identity ownIdentity
}

// ApplyCrossResourceRefs writes ref.<property>.{urn,type,region,sku} properties
// for each Refs entry that points at exactly one resource in resources.
// A reference outside the slice, with several URNs, or whose target has neither
// a region nor a SKU is left unresolved and logged at debug. An empty URN list
// is not a reference and is not logged. Parent is not consulted. The target's
// SKU is not copied onto the child's own SKU properties.
func ApplyCrossResourceRefs(ctx context.Context, resources []ResourceDescriptor) {
	if len(resources) == 0 {
		return
	}
	targets := refTargets(ctx, resources)
	for i := range resources {
		writeCrossResourceRefs(ctx, &resources[i], targets)
	}
}

func refTargets(ctx context.Context, resources []ResourceDescriptor) map[string]refTarget {
	identities := resolveOwnIdentities(ctx, resources)
	targets := make(map[string]refTarget, len(resources))
	for i := range resources {
		id := resources[i].ID
		if id == "" {
			continue
		}
		if _, exists := targets[id]; exists {
			continue
		}
		targets[id] = refTarget{desc: &resources[i], identity: identities[i]}
	}
	return targets
}

func writeCrossResourceRefs(
	ctx context.Context,
	resource *ResourceDescriptor,
	targets map[string]refTarget,
) {
	for prop, urns := range resource.Refs {
		if len(urns) == 0 {
			continue
		}
		target, ok := singleRefTarget(urns, targets)
		if !ok || target.desc == resource {
			logUnresolvedRef(ctx, resource.ID, prop, urns, "cross-resource reference unresolved")
			continue
		}
		if target.identity.sku == "" && target.identity.region == "" {
			logUnresolvedRef(ctx, resource.ID, prop, urns, "referenced resource has no region or SKU")
			continue
		}
		setRefProperties(resource, prop, urns[0], target.identity)
	}
}

func singleRefTarget(urns []string, targets map[string]refTarget) (refTarget, bool) {
	if len(urns) != 1 || urns[0] == "" {
		return refTarget{}, false
	}
	target, ok := targets[urns[0]]
	return target, ok
}

func setRefProperties(resource *ResourceDescriptor, prop, urn string, identity ownIdentity) {
	if resource.Properties == nil {
		resource.Properties = map[string]any{}
	}
	prefix := "ref." + prop + "."
	resource.Properties[prefix+"urn"] = urn
	resource.Properties[prefix+"type"] = identity.resourceType
	if identity.region != "" {
		resource.Properties[prefix+"region"] = identity.region
	}
	if identity.sku != "" {
		resource.Properties[prefix+"sku"] = identity.sku
	}
}

func logUnresolvedRef(ctx context.Context, id, prop string, urns []string, msg string) {
	event := logging.FromContext(ctx).Debug().
		Ctx(ctx).
		Str("component", "engine").
		Str("operation", "apply_cross_resource_refs").
		Str("resource_id", id).
		Str("property", prop)
	if len(urns) == 1 {
		event = event.Str("urn", urns[0])
	} else {
		event = event.Strs("urns", urns)
	}
	event.Msg(msg)
}

type ownIdentity struct {
	sku          string
	region       string
	resourceType string
}

func resolveOwnIdentities(ctx context.Context, resources []ResourceDescriptor) []ownIdentity {
	out := make([]ownIdentity, len(resources))
	for i := range resources {
		props := ConvertToProto(resources[i].Properties)
		sku, region := proto.ResolveSKUAndRegion(ctx, resources[i].Provider, resources[i].Type, props)
		if sku == "" || sku == proto.UnknownPulumiValue {
			if alt := props["skuName"]; alt != "" && alt != proto.UnknownPulumiValue {
				sku = alt
			}
		}
		if sku == proto.UnknownPulumiValue {
			sku = ""
		}
		if region == proto.UnknownPulumiValue {
			region = ""
		}
		out[i] = ownIdentity{
			sku:          sku,
			region:       region,
			resourceType: resources[i].Type,
		}
	}
	return out
}

// refCacheSuffix hashes resolved ref.* properties so projected cache keys change
// when a reference changes and stay unchanged when a resource has none.
func refCacheSuffix(properties map[string]any) string {
	if len(properties) == 0 {
		return ""
	}
	parts := make([]string, 0)
	for key, value := range properties {
		if !strings.HasPrefix(key, "ref.") {
			continue
		}
		parts = append(parts, key+"="+ConvertValueToString(value))
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}
