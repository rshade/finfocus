package engine

import (
	"fmt"
	"math"
)

const (
	spotSavingsUnavailable = "spot savings unavailable"
	spotSavingsSuffix      = " vs on-demand"
)

// SpotSavingsPercent is how much cheaper a spot price is than the plugin's
// on-demand equivalent, as a percentage: (onDemand - spot) / onDemand * 100. A
// result above 0 means spot is cheaper; below 0 means it costs more.
//
// It is nil unless the savings can be stated: the category is DYNAMIC, the
// plugin sent an on-demand price (presence, not value), that price is finite
// and greater than 0, and the spot price is finite and not negative. An
// on-demand equivalent of 0 has no percentage and is never shown as 0%.
func SpotSavingsPercent(category PricingCategory, spot float64, onDemand *float64) *float64 {
	if !category.IsSpot() || onDemand == nil {
		return nil
	}
	od := *onDemand
	if math.IsNaN(od) || math.IsInf(od, 0) || od <= 0 {
		return nil
	}
	if math.IsNaN(spot) || math.IsInf(spot, 0) || spot < 0 {
		return nil
	}
	pct := (od - spot) / od * percentScale
	return &pct
}

// spotSavingsDisplay is the savings part of the spot label: "-62% vs
// on-demand" (the spot price is 62% below on-demand), "+15% vs on-demand", or
// the explicit unavailable marker when the percentage is nil.
func spotSavingsDisplay(savingsPercent *float64) string {
	if savingsPercent == nil {
		return spotSavingsUnavailable
	}
	change := math.Round(-*savingsPercent)
	if change == 0 {
		return "0%" + spotSavingsSuffix
	}
	return fmt.Sprintf("%+.0f%%", change) + spotSavingsSuffix
}

// applyOnDemandEquivalent records the plugin's on-demand price and the savings
// computed from it. It must run after Monthly and PricingCategory are set.
func (r *CostResult) applyOnDemandEquivalent(onDemand *float64) {
	r.OnDemandEquivalentMonthly = nil
	r.SpotSavingsPercent = nil
	if !r.PricingCategory.IsSpot() || onDemand == nil {
		return
	}
	value := *onDemand
	r.OnDemandEquivalentMonthly = &value
	r.SpotSavingsPercent = SpotSavingsPercent(r.PricingCategory, r.Monthly, &value)
}
