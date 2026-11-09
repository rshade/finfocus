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

// SpotDisplay is the one spot label every surface renders:
// "SPOT · risk 0.12 · -62% vs on-demand". The risk reads "risk unknown" when
// the score is 0.0 (proto3 cannot tell a zero score from an unset one). The
// savings part reads "spot savings unavailable" when savingsPercent is nil,
// never 0%. It returns "" for any other category, so a caller shows it only
// when it is not empty.
func SpotDisplay(category PricingCategory, risk float64, savingsPercent *float64) string {
	label := SpotRiskDisplay(category, risk)
	if label == "" {
		return ""
	}
	return label + spotSeparator + spotSavingsDisplay(savingsPercent)
}

// SpotRiskDisplay is the tag and risk part of the spot label shared by every
// surface: "SPOT · risk 0.12", or "SPOT · risk unknown" when the score is 0.0.
// It returns "" for any category other than DYNAMIC.
func SpotRiskDisplay(category PricingCategory, risk float64) string {
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
	return SpotDisplay(r.PricingCategory, r.SpotInterruptionRisk, r.SpotSavingsPercent)
}

// SpotDisplay is the spot label of a recommendation whose target is spot, or ""
// otherwise. It has no savings part: a recommendation states its saving as an
// amount of its own.
func (r Recommendation) SpotDisplay() string {
	return SpotRiskDisplay(r.PricingCategory, r.SpotInterruptionRisk)
}
