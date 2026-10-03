// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// PricingTierView is one volume breakpoint copied from a plugin PricingSpec.
type PricingTierView struct {
	MinQuantity float64 `json:"min_quantity,omitempty"`
	MaxQuantity float64 `json:"max_quantity,omitempty"`
	RatePerUnit float64 `json:"rate_per_unit"`
	Description string  `json:"description,omitempty"`
}

// PricingSpecView is the GetPricingSpec detail shown by cost projected --explain.
// It is display only. Monthly stays the cost the pricing path already calculated.
type PricingSpecView struct {
	Assumptions  []string          `json:"assumptions,omitempty"`
	BillingMode  string            `json:"billing_mode,omitempty"`
	Unit         string            `json:"unit,omitempty"`
	PricingTiers []PricingTierView `json:"pricing_tiers,omitempty"`
	RatePerUnit  float64           `json:"rate_per_unit"`
	Source       string            `json:"source,omitempty"`
}

// WithExplainPricing attaches GetPricingSpec detail to a projected diff.
// The default is off, and a disabled engine does not call the RPC.
func (e *Engine) WithExplainPricing(enabled bool) *Engine {
	if e == nil {
		return nil
	}
	e.explainPricing = enabled
	return e
}

func (e *Engine) attachPricingExplanations(
	ctx context.Context,
	resources []ResourceDescriptor,
	plan diffPricingPlan,
	diff *DiffResult,
) {
	if e == nil || !e.explainPricing || diff == nil {
		return
	}
	entry := 0
	for i, resource := range resources {
		if entry >= len(diff.Entries) {
			return
		}
		got := &diff.Entries[entry]
		if got.ResourceType != resource.Type || got.ResourceID != resource.ID {
			continue
		}
		if desc, ok := explainDescriptor(plan, i); ok {
			got.PricingSpec = e.pricingExplanation(ctx, desc)
		}
		entry++
	}
}

func explainDescriptor(plan diffPricingPlan, index int) (ResourceDescriptor, bool) {
	for j, at := range plan.afterAt {
		if at == index && j < len(plan.after) {
			return plan.after[j], true
		}
	}
	for j, at := range plan.beforeAt {
		if at == index && j < len(plan.before) {
			return plan.before[j], true
		}
	}
	return ResourceDescriptor{}, false
}

func (e *Engine) pricingExplanation(ctx context.Context, resource ResourceDescriptor) *PricingSpecView {
	matches, _ := e.selectPluginMatchesForResource(ctx, resource, "ProjectedCosts")
	log := logging.FromContext(ctx)
	for _, match := range matches {
		if match.Client == nil || match.Client.API == nil {
			continue
		}
		spec, err := fetchPluginPricingSpec(ctx, match.Client, resource, e.pricingSpecDeadline())
		if err != nil || spec == nil {
			log.Debug().
				Ctx(ctx).
				Err(err).
				Str("component", "engine").
				Str("operation", "explain_pricing").
				Str("plugin", match.Client.Name).
				Str("resource_id", resource.ID).
				Msg("plugin pricing spec unavailable")
			continue
		}
		return pricingSpecView(spec)
	}
	return nil
}

func pricingSpecView(spec *pbc.PricingSpec) *PricingSpecView {
	if spec == nil {
		return nil
	}
	view := &PricingSpecView{
		Assumptions: append([]string(nil), spec.GetAssumptions()...),
		BillingMode: spec.GetBillingMode(),
		Unit:        spec.GetUnit(),
		RatePerUnit: spec.GetRatePerUnit(),
		Source:      spec.GetSource(),
	}
	for _, tier := range spec.GetPricingTiers() {
		if tier == nil {
			continue
		}
		view.PricingTiers = append(view.PricingTiers, PricingTierView{
			MinQuantity: tier.GetMinQuantity(),
			MaxQuantity: tier.GetMaxQuantity(),
			RatePerUnit: tier.GetRatePerUnit(),
			Description: tier.GetDescription(),
		})
	}
	return view
}
