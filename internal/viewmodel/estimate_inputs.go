// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package viewmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/resourcetype"
)

// PropertyClass says how the editor treats one property.
type PropertyClass string

// Property classes. Identity rows never appear in the editable list.
const (
	PropertyClassPricing  PropertyClass = "pricing"
	PropertyClassIdentity PropertyClass = "identity"
	PropertyClassOther    PropertyClass = "other"
)

// InputProvenance says why a pricing row is trusted. It is set only on pricing rows.
type InputProvenance string

// Provenance kinds.
const (
	ProvenanceSKU         InputProvenance = "sku"
	ProvenanceRegion      InputProvenance = "region"
	ProvenancePluginInput InputProvenance = "plugin-input"
	ProvenanceUsage       InputProvenance = "usage"
)

// EstimateFallbackNote is shown when a type has no reviewed pricing inputs.
const EstimateFallbackNote = "Pricing inputs are not curated for this resource type; all safe properties are shown."

const (
	pulumiNamespace    = "pulumi:"
	terraformNamespace = "terraform:"
	typeTokenParts     = 3
	pluginNamePrefix   = "finfocus-plugin-"
	noSelectedPlugin   = "\x00"
)

// EstimateInputContext is what the policy needs beyond the descriptor: the
// pricing-spec hints and which plugin serves this estimate.
type EstimateInputContext struct {
	// Hints are the usage metrics from discovery, in mode order.
	Hints []engine.PricingMetricHint
	// Plugins are the discovered plugin names with the finfocus-plugin- prefix
	// removed. They apply only when no pricing mode is selected.
	Plugins []string
	// SelectedPlugin is the selected mode's plugin, trimmed the same way. It is
	// empty for Automatic. A mode id that does not name a plugin selects none.
	SelectedPlugin string
}

// NewEstimateInputContext builds the policy input from the discovery the
// estimate already ran and the selected pricing-mode id. An empty id is
// Automatic.
func NewEstimateInputContext(discovery engine.PricingDiscovery, pricingModeID string) EstimateInputContext {
	input := EstimateInputContext{Hints: UsageHints(discovery)}
	seen := make(map[string]struct{}, len(discovery.Modes))
	for _, mode := range discovery.Modes {
		name := trimPluginName(mode.Plugin)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		input.Plugins = append(input.Plugins, name)
	}
	if pricingModeID == "" {
		return input
	}
	var pair [2]string
	name := ""
	if err := json.Unmarshal([]byte(pricingModeID), &pair); err == nil {
		name = trimPluginName(pair[0])
	}
	if name == "" {
		name = noSelectedPlugin
	}
	input.SelectedPlugin = name
	return input
}

func trimPluginName(name string) string {
	return strings.TrimPrefix(name, pluginNamePrefix)
}

// pluginServes reports whether the mapping's plugin prices this estimate.
// A selected mode must name it. Automatic matches any discovered plugin.
func (c EstimateInputContext) pluginServes(plugin string) bool {
	if c.SelectedPlugin != "" {
		return c.SelectedPlugin == plugin
	}
	return slices.Contains(c.Plugins, plugin)
}

// EstimateDetail is one read-only identity row.
type EstimateDetail struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// EstimatePropertyPlan is the ordered, classified presentation of a descriptor.
// Rows hold pricing inputs first (mapping order, then usage matches by key),
// then the other rows sorted by key. Details hold identity metadata sorted by key.
type EstimatePropertyPlan struct {
	Rows    []EstimatePropertyRow
	Details []EstimateDetail
	Curated bool
	Note    string
}

// IdentityOverrideError reports an override that names identity metadata.
type IdentityOverrideError struct {
	Key string
}

// Error names the key and never the submitted value.
func (e *IdentityOverrideError) Error() string {
	return fmt.Sprintf("identity property %q cannot be overridden", e.Key)
}

// RejectIdentityOverrides returns an *IdentityOverrideError naming the first
// identity key in sorted order, or nil when no override names one.
func RejectIdentityOverrides(overrides map[string]string) error {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if isIdentityKey(key) {
			return &IdentityOverrideError{Key: key}
		}
	}
	return nil
}

// BuildEstimatePropertyPlan redacts the descriptor's properties, then classifies
// and orders them. input carries the discovery hints and which plugin serves the
// estimate. A reviewed plugin-input the descriptor lacks becomes a usage row
// when that plugin serves. An override for any other absent, non-hidden key
// becomes an other row so an edit survives a mode change.
func BuildEstimatePropertyPlan(
	ctx context.Context,
	resource *engine.ResourceDescriptor,
	overrides map[string]string,
	input EstimateInputContext,
) EstimatePropertyPlan {
	var resourceType string
	var raw map[string]any
	if resource != nil {
		resourceType, raw = resource.Type, resource.Properties
	}
	props := engine.RedactedProperties(ctx, raw)

	editable := make(map[string]any, len(props))
	var details []EstimateDetail
	for key, value := range props {
		if isIdentityKey(key) {
			details = append(details, EstimateDetail{Key: key, Value: fmt.Sprintf("%v", value)})
			continue
		}
		editable[key] = value
	}
	slices.SortFunc(details, func(a, b EstimateDetail) int { return strings.Compare(a.Key, b.Key) })

	rows := appendReviewedAbsences(
		resourceType, BuildEstimatePropertyRows(editable, overrides), overrides, input, removedByRedaction(raw, props),
	)
	classes, curated := classifyRows(resourceType, rows, input)
	for i := range rows {
		rows[i].Class = classes[rows[i].Key].class
		rows[i].Provenance = classes[rows[i].Key].provenance
		rows[i].Editable = true
		rows[i].refreshFlags(curated)
	}
	slices.SortStableFunc(rows, func(a, b EstimatePropertyRow) int {
		if classes[a.Key].rank != classes[b.Key].rank {
			return classes[a.Key].rank - classes[b.Key].rank
		}
		return strings.Compare(a.Key, b.Key)
	})

	plan := EstimatePropertyPlan{Rows: rows, Details: details, Curated: curated}
	if !curated {
		plan.Note = EstimateFallbackNote
	}
	return plan
}

type rowClass struct {
	class      PropertyClass
	provenance InputProvenance
	rank       int
}

const (
	usageRankBase = 1 << 16
	otherRank     = 1 << 20
)

func removedByRedaction(raw, props map[string]any) map[string]bool {
	removed := make(map[string]bool)
	for key := range raw {
		if _, kept := props[key]; !kept {
			removed[key] = true
		}
	}
	return removed
}

func appendReviewedAbsences(
	resourceType string,
	rows []EstimatePropertyRow,
	overrides map[string]string,
	input EstimateInputContext,
	redacted map[string]bool,
) []EstimatePropertyRow {
	present := make(map[string]bool, len(rows))
	for _, row := range rows {
		present[row.Key] = true
	}
	if mapped, ok := mappingFor(resourceType); ok && input.pluginServes(mapped.Plugin) {
		for _, spec := range mapped.Inputs {
			if spec.Provenance != ProvenancePluginInput || present[spec.Key] || redacted[spec.Key] {
				continue
			}
			current := ""
			if value, exists := overrides[spec.Key]; exists {
				current = value
			}
			rows = append(rows, EstimatePropertyRow{
				Key: spec.Key, OriginalValue: "", CurrentValue: current, Usage: true,
			})
			present[spec.Key] = true
		}
	}
	for key, value := range overrides {
		if present[key] || redacted[key] || isIdentityKey(key) || engine.IsHiddenPropertyKey(key) {
			continue
		}
		rows = append(rows, EstimatePropertyRow{Key: key, OriginalValue: "", CurrentValue: value})
		present[key] = true
	}
	return rows
}

func classifyMappedRows(
	mapped InputMapping,
	present, usageRow map[string]bool,
	serves bool,
	classes map[string]rowClass,
) bool {
	curated := false
	for rank, spec := range mapped.Inputs {
		if !present[spec.Key] || (spec.Provenance == ProvenancePluginInput && !serves) {
			continue
		}
		// Present pricing inputs keep mapping order. Usage rows follow them,
		// still in mapping order (FR-012).
		rowRank := rank
		if usageRow[spec.Key] {
			rowRank = usageRankBase + rank + 1
		}
		classes[spec.Key] = rowClass{class: PropertyClassPricing, provenance: spec.Provenance, rank: rowRank}
		curated = true
	}
	return curated
}

func classifyRows(
	resourceType string,
	rows []EstimatePropertyRow,
	input EstimateInputContext,
) (map[string]rowClass, bool) {
	present := make(map[string]bool, len(rows))
	for _, row := range rows {
		present[row.Key] = true
	}
	classes := make(map[string]rowClass, len(rows))
	curated := false
	usageRow := make(map[string]bool)
	for _, row := range rows {
		if row.Usage {
			usageRow[row.Key] = true
		}
	}
	if mapped, ok := mappingFor(resourceType); ok {
		curated = classifyMappedRows(mapped, present, usageRow, input.pluginServes(mapped.Plugin), classes)
	}
	for _, hint := range input.Hints {
		if _, claimed := classes[hint.Metric]; present[hint.Metric] && !claimed {
			classes[hint.Metric] = rowClass{
				class:      PropertyClassPricing,
				provenance: ProvenanceUsage,
				rank:       usageRankBase,
			}
		}
	}
	for _, row := range rows {
		if _, ok := classes[row.Key]; !ok {
			classes[row.Key] = rowClass{class: PropertyClassOther, rank: otherRank}
		}
	}
	return classes, curated
}

func isIdentityKey(key string) bool {
	switch key {
	case "id", "arn", "urn":
		return true
	}
	return strings.HasPrefix(key, pulumiNamespace) || strings.HasPrefix(key, terraformNamespace)
}

func mappingFor(resourceType string) (InputMapping, bool) {
	provider := resourcetype.ExtractProvider(resourceType)
	parts := strings.SplitN(resourceType, ":", typeTokenParts)
	if len(parts) < typeTokenParts-1 {
		return InputMapping{}, false
	}
	segment := strings.ToLower(parts[1])
	for _, mapped := range reviewedInputMappings() {
		if mapped.Provider == provider && mapped.ResourceSegment == segment {
			return mapped, true
		}
	}
	return InputMapping{}, false
}
