package cli

import (
	"context"
	"encoding/json"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/router"
)

// capabilityResolveResourceTypes is the plugin capability string (as produced
// by pluginhost.ConvertCapabilities) gating ResolveResourceTypes support.
const capabilityResolveResourceTypes = "resolve_resource_types"

// sourceFormatTerraform is the cache-key segment for Terraform source formats.
const sourceFormatTerraform = "terraform"

// cachedTypeMapping is the JSON shape stored in the resolve_types cache bucket.
// Only fields the resolution pipeline consumes are persisted.
type cachedTypeMapping struct {
	PulumiToken      string            `json:"pulumi_token"`
	PropertyMappings map[string]string `json:"property_mappings,omitempty"`
}

// resolveResourceTypes resolves Terraform type strings on the given
// descriptors to Pulumi tokens via plugins advertising the
// resolve_resource_types capability. Descriptors whose Type already contains a
// colon (Pulumi tokens) are untouched, so Pulumi-sourced input is a no-op.
// Plugins without the capability, and RPC failures, leave the raw TF type in
// place (fallback mode) — the adapter's SKU/region extraction still works off
// the camelCased properties. Results are cached in store (may be nil; entries
// use the 7-day max TTL).
func resolveResourceTypes(
	ctx context.Context,
	clients []*pluginhost.Client,
	store cache.Cache,
	resources []engine.ResourceDescriptor,
) []engine.ResourceDescriptor {
	tfTypes := uniqueTerraformTypes(resources)
	if len(tfTypes) == 0 {
		return resources
	}

	mappings := loadCachedTypeMappings(store, tfTypes)
	resolveTypesViaPlugins(ctx, clients, store, tfTypes, mappings)
	return applyTypeMappings(resources, mappings)
}

// loadCachedTypeMappings reads cached type mappings for the given TF types
// from store, returning only entries with a non-empty Pulumi token. A nil or
// disabled store yields an empty map.
func loadCachedTypeMappings(store cache.Cache, tfTypes []string) map[string]cachedTypeMapping {
	mappings := make(map[string]cachedTypeMapping, len(tfTypes))
	if store == nil || !store.IsEnabled() {
		return mappings
	}
	for _, t := range tfTypes {
		key := cache.BuildResolveTypesKey(sourceFormatTerraform, t)
		entry, err := store.Get(key)
		if err != nil {
			continue
		}
		var m cachedTypeMapping
		if json.Unmarshal(entry.Data, &m) == nil && m.PulumiToken != "" {
			mappings[t] = m
		}
	}
	return mappings
}

// resolveTypesViaPlugins fans out ResolveResourceTypes RPCs to plugins
// advertising the capability, one call per provider, skipping types already
// present in mappings. Successful results are stored in mappings and cached.
func resolveTypesViaPlugins(
	ctx context.Context,
	clients []*pluginhost.Client,
	store cache.Cache,
	tfTypes []string,
	mappings map[string]cachedTypeMapping,
) {
	log := logging.FromContext(ctx)

	byProvider := make(map[string][]string)
	for _, t := range tfTypes {
		if _, ok := mappings[t]; ok {
			continue
		}
		provider := router.ExtractProviderFromType(t)
		byProvider[provider] = append(byProvider[provider], t)
	}

	for provider, types := range byProvider {
		client := findClientForProvider(clients, provider)
		if client == nil || !client.HasCapability(capabilityResolveResourceTypes) {
			log.Debug().Ctx(ctx).Str("provider", provider).Int("type_count", len(types)).
				Msg("no plugin with resolve_resource_types capability; using raw terraform types")
			continue
		}
		resp, err := client.API.ResolveResourceTypes(ctx, &pbc.ResolveResourceTypesRequest{
			SourceTypes:  types,
			SourceFormat: pbc.SourceFormat_SOURCE_FORMAT_TERRAFORM,
		})
		if err != nil {
			log.Warn().Ctx(ctx).Err(err).Str("plugin", client.Name).Str("provider", provider).
				Msg("ResolveResourceTypes failed; falling back to raw terraform types")
			continue
		}
		cacheResolvedTypes(ctx, store, types, resp, mappings)
	}
}

// cacheResolvedTypes merges the RPC response mappings for types into mappings
// and persists them to store (best-effort; failures are logged and skipped).
func cacheResolvedTypes(
	ctx context.Context,
	store cache.Cache,
	types []string,
	resp *pbc.ResolveResourceTypesResponse,
	mappings map[string]cachedTypeMapping,
) {
	log := logging.FromContext(ctx)
	for _, t := range types {
		mapping, ok := resp.GetMappings()[t]
		if !ok || mapping.GetPulumiToken() == "" {
			continue
		}
		cm := cachedTypeMapping{
			PulumiToken:      mapping.GetPulumiToken(),
			PropertyMappings: mapping.GetPropertyMappings(),
		}
		mappings[t] = cm
		if store == nil || !store.IsEnabled() {
			continue
		}
		data, marshalErr := json.Marshal(cm)
		if marshalErr != nil {
			continue
		}
		if setErr := store.SetWithTTL(
			cache.BuildResolveTypesKey(sourceFormatTerraform, t), data, cache.MaxTTLSeconds,
		); setErr != nil {
			log.Warn().Ctx(ctx).Err(setErr).Str("source_type", t).
				Msg("failed to cache type resolution result")
		}
	}
}

// applyTypeMappings returns a copy of resources with raw TF types replaced by
// their resolved Pulumi tokens; property mappings are applied by copying the
// property map and adding the Pulumi-key alias. The raw snake_case key is
// checked first, then its camelCase form, because the Terraform mapper already
// camelCased every key at ingestion.
func applyTypeMappings(
	resources []engine.ResourceDescriptor,
	mappings map[string]cachedTypeMapping,
) []engine.ResourceDescriptor {
	out := make([]engine.ResourceDescriptor, len(resources))
	copy(out, resources)
	for i := range out {
		if strings.Contains(out[i].Type, ":") {
			continue
		}
		m, ok := mappings[out[i].Type]
		if !ok || m.PulumiToken == "" {
			continue
		}
		out[i].Type = m.PulumiToken
		if len(m.PropertyMappings) == 0 {
			continue
		}
		props := make(map[string]interface{}, len(out[i].Properties)+len(m.PropertyMappings))
		for k, v := range out[i].Properties {
			props[k] = v
		}
		for tfKey, pulumiKey := range m.PropertyMappings {
			if v, exists := props[tfKey]; exists {
				props[pulumiKey] = v
				continue
			}
			if v, exists := props[ingest.SnakeToCamel(tfKey)]; exists {
				props[pulumiKey] = v
			}
		}
		out[i].Properties = props
	}
	return out
}

// uniqueTerraformTypes returns the distinct colon-less (raw TF) type strings
// across the descriptors, preserving first-seen order.
func uniqueTerraformTypes(resources []engine.ResourceDescriptor) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, r := range resources {
		if strings.Contains(r.Type, ":") {
			continue
		}
		if _, ok := seen[r.Type]; ok {
			continue
		}
		seen[r.Type] = struct{}{}
		out = append(out, r.Type)
	}
	return out
}

// findClientForProvider returns the first plugin client whose declared
// supported providers match the given provider, or nil.
func findClientForProvider(clients []*pluginhost.Client, provider string) *pluginhost.Client {
	for _, c := range clients {
		if c == nil || c.Metadata == nil {
			continue
		}
		for _, sp := range c.Metadata.SupportedProviders {
			if router.ProviderMatches(provider, sp) {
				return c
			}
		}
	}
	return nil
}
