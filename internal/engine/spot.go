package engine

import "fmt"

// PricingCategory is the plugin-reported pricing model of a cost result,
// copied from the spec's FocusPricingCategory. Empty means the plugin left it
// unspecified. Core attaches no meaning to a category beyond DYNAMIC.
type PricingCategory string

const (
	// PricingCategoryStandard is on-demand pricing.
	PricingCategoryStandard PricingCategory = "STANDARD"
	// PricingCategoryCommitted is commitment-based pricing.
	PricingCategoryCommitted PricingCategory = "COMMITTED"
	// PricingCategoryDynamic is market-priced, interruptible pricing (spot).
	PricingCategoryDynamic PricingCategory = "DYNAMIC"
	// PricingCategoryOther is any category the spec does not name.
	PricingCategoryOther PricingCategory = "OTHER"
)

const (
	spotTag         = "SPOT"
	spotSeparator   = " · "
	spotRiskUnknown = "risk unknown"
	spotRiskFloor   = 0.005
)

// IsSpot reports whether the category marks interruptible pricing.
func (c PricingCategory) IsSpot() bool {
	return c == PricingCategoryDynamic
}

// SpotDisplay is the one spot label every surface renders: "SPOT · risk 0.12",
// or "SPOT · risk unknown" when the score is 0.0 (proto3 cannot tell a zero
// score from an unset one). It returns "" for any other category, so a caller
// shows it only when it is not empty.
func SpotDisplay(category PricingCategory, risk float64) string {
	if !category.IsSpot() {
		return ""
	}
	if risk <= 0 {
		return spotTag + spotSeparator + spotRiskUnknown
	}
	return spotTag + spotSeparator + "risk " + formatSpotRiskValue(risk)
}

func formatSpotRiskValue(risk float64) string {
	if risk < spotRiskFloor {
		return "<0.01"
	}
	return fmt.Sprintf("%.2f", risk)
}

// SpotDisplay is the spot label for this result, or "" when it is not spot.
func (r *CostResult) SpotDisplay() string {
	if r == nil {
		return ""
	}
	return SpotDisplay(r.PricingCategory, r.SpotInterruptionRisk)
}
