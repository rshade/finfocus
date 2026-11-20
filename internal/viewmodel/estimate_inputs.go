// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package viewmodel

import (
	"context"
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
)

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
// and orders them. Hints are the usage metrics discovered for the type; an empty
// list claims no usage input. An override naming a property the descriptor lacks
// makes no row.
func BuildEstimatePropertyPlan(
	ctx context.Context,
	resource *engine.ResourceDescriptor,
	overrides map[string]string,
	hints []engine.PricingMetricHint,
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

	rows := BuildEstimatePropertyRows(editable, overrides)
	classes, curated := classifyRows(resourceType, rows, hints)
	for i := range rows {
		rows[i].Class = classes[rows[i].Key].class
		rows[i].Provenance = classes[rows[i].Key].provenance
		rows[i].Editable = true
		rows[i].Overridden = rows[i].CurrentValue != rows[i].OriginalValue
		rows[i].DefaultVisible = rows[i].Class == PropertyClassPricing || rows[i].Overridden || !curated
	}
	slices.SortStableFunc(rows, func(a, b EstimatePropertyRow) int {
		return classes[a.Key].rank - classes[b.Key].rank
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

func classifyRows(
	resourceType string,
	rows []EstimatePropertyRow,
	hints []engine.PricingMetricHint,
) (map[string]rowClass, bool) {
	present := make(map[string]bool, len(rows))
	for _, row := range rows {
		present[row.Key] = true
	}
	classes := make(map[string]rowClass, len(rows))
	curated := false
	if mapped, ok := mappingFor(resourceType); ok {
		for rank, input := range mapped.Inputs {
			if present[input.Key] {
				classes[input.Key] = rowClass{class: PropertyClassPricing, provenance: input.Provenance, rank: rank}
				curated = true
			}
		}
	}
	for _, hint := range hints {
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
