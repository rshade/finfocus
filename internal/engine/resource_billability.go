package engine

import (
	"context"
	"strings"
)

// ResourceBillability describes evidence about billability, independent of spend
// or whether a plugin can currently price a resource.
type ResourceBillability string

const (
	// BillabilityUnknown preserves candidates without reliable pricing metadata.
	BillabilityUnknown ResourceBillability = "unknown"
	// NonBillable identifies bookkeeping or unanimous explicit zero-cost metadata.
	NonBillable ResourceBillability = "non_billable"
	// PotentiallyBillable identifies explicit billing modes, even with a zero rate.
	PotentiallyBillable ResourceBillability = "potentially_billable"
)

// IsPulumiInternalType reports whether a type belongs to Pulumi bookkeeping.
func IsPulumiInternalType(resourceType string) bool {
	return strings.HasPrefix(resourceType, pulumiInternalPrefix)
}

// ResourceBillability uses only already-discovered metadata. It never issues
// pricing or Supports RPCs. Missing or conflicting effective-provider metadata
// leaves a cloud resource unknown, so it remains selectable.
func (e *Engine) ResourceBillability(ctx context.Context, resource ResourceDescriptor) ResourceBillability {
	if IsPulumiInternalType(resource.Type) {
		return NonBillable
	}
	if e == nil || ctx == nil || ctx.Err() != nil {
		return BillabilityUnknown
	}
	found, ok := e.cachedPricingDiscovery(pricingDiscoveryKey(ctx, &resource, strings.TrimSpace(resource.Type)))
	if !ok || len(found.Modes) == 0 {
		return BillabilityUnknown
	}
	// EstimateCost tries the engine's clients before projected-cost fallback,
	// which may route to additional clients. Metadata must agree across both
	// paths; a projected route alone cannot establish the estimate provider.
	matches := automaticMatches(e.clients)
	if e.router != nil {
		matches = append(matches, matchesWithoutScorers(e.router.SelectPlugins(ctx, resource, "ProjectedCosts"))...)
	}
	if len(matches) == 0 {
		return BillabilityUnknown
	}
	classification := BillabilityUnknown
	for _, match := range matches {
		if match.Client == nil || match.Client.API == nil {
			return BillabilityUnknown
		}
		clientClass := pricingModesBillability(found.Modes, strings.TrimSpace(match.Client.Name))
		if clientClass == BillabilityUnknown {
			return BillabilityUnknown
		}
		if classification != BillabilityUnknown && classification != clientClass {
			return BillabilityUnknown
		}
		classification = clientClass
	}
	return classification
}

// pricingModesBillability requires agreement among every mode from one client.
func pricingModesBillability(modes []PricingMode, pluginName string) ResourceBillability {
	classification := BillabilityUnknown
	for _, mode := range modes {
		if mode.Plugin != pluginName {
			continue
		}
		name := strings.TrimSpace(mode.BillingMode)
		if name == "" || strings.EqualFold(name, billingNotImplemented) {
			return BillabilityUnknown
		}
		next := PotentiallyBillable
		if name == "zero_cost" {
			next = NonBillable
		}
		if classification != BillabilityUnknown && classification != next {
			return BillabilityUnknown
		}
		classification = next
	}
	return classification
}

// EstimateResourceSelection is the safe list with stable source identities and
// a recommended initial selection. Billability covers the complete input list.
type EstimateResourceSelection struct {
	Resources    []ResourceDescriptor           `json:"resources"`
	Billability  map[string]ResourceBillability `json:"billability"  datakey:"true"`
	HiddenCount  int                            `json:"hiddenCount"`
	PreferredURN string                         `json:"preferredURN"`
}

// SelectEstimateResources shares visibility and initial-selection policy between
// the browser and interactive CLI. It preserves source order and never mutates
// descriptors. Unknown resources with editable inputs precede bookkeeping when
// all resources are shown; explicit billing metadata has the highest preference.
func (e *Engine) SelectEstimateResources(
	ctx context.Context,
	resources []ResourceDescriptor,
	showAll bool,
) EstimateResourceSelection {
	selection := EstimateResourceSelection{
		Resources:   []ResourceDescriptor{},
		Billability: map[string]ResourceBillability{},
	}
	best := -1
	for _, resource := range resources {
		class := e.ResourceBillability(ctx, resource)
		selection.Billability[resource.ID] = class
		if !showAll && class == NonBillable {
			selection.HiddenCount++
			continue
		}
		selection.Resources = append(selection.Resources, resource)
		score := 0
		if class != NonBillable {
			score = 1
			if len(resource.Properties) > 0 {
				score = 2
			}
		}
		if class == PotentiallyBillable {
			score = 3
		}
		if score > best {
			selection.PreferredURN = resource.ID
			best = score
		}
	}
	return selection
}
