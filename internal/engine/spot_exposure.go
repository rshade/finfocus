package engine

import (
	"fmt"
	"math"
)

// SpotExposureItem is one priced row as the exposure aggregation sees it:
// its monthly cost, currency, plugin pricing category, and interruption risk.
type SpotExposureItem struct {
	Cost     float64
	Currency string
	Category PricingCategory
	Risk     float64
}

// SpotExposure is how much of a total is interruptible spot spend and how
// risky that spend is. It is informational: no budget, exit code, or gate
// reads it. A nil *SpotExposure means the total holds no spot rows (or its
// currencies differ), and every surface omits the block.
type SpotExposure struct {
	// SpotCost is the summed cost of the DYNAMIC rows.
	SpotCost float64 `json:"spotCost"`
	// TotalCost is the summed cost of every row the exposure was computed over.
	TotalCost float64 `json:"totalCost"`
	// SpotShare is SpotCost / TotalCost. Absent when TotalCost is not positive.
	SpotShare *float64 `json:"spotShare,omitempty"`
	// WeightedRisk is the cost-weighted mean interruption risk over the spot
	// rows whose risk is known. Absent when no such row carries cost; absent
	// never means zero risk.
	WeightedRisk *float64 `json:"weightedRisk,omitempty"`
	// SpotResources counts the DYNAMIC rows.
	SpotResources int `json:"spotResources"`
	// RiskUnknownResources counts the DYNAMIC rows left out of WeightedRisk
	// because the plugin gave no usable risk score.
	RiskUnknownResources int    `json:"riskUnknownResources"`
	Currency             string `json:"currency"`
	// Display is the text every surface prints after SpotExposureLabel.
	Display string `json:"display"`
}

// SpotExposureChange is the spot cost of a projected diff before and after the
// change. Each side counts every entry once, so a replacement (one entry after
// collapseReplacements) is not doubled.
type SpotExposureChange struct {
	BeforeSpotCost float64 `json:"beforeSpotCost"`
	AfterSpotCost  float64 `json:"afterSpotCost"`
	DeltaSpotCost  float64 `json:"deltaSpotCost"`
	Currency       string  `json:"currency"`
	Display        string  `json:"display"`
}

const (
	percentScale     = 100
	spotNoRiskReason = "no spot resource has a risk score"

	// SpotExposureLabel and SpotChangeLabel name the Display values wherever
	// they are shown.
	SpotExposureLabel = "Spot exposure"
	SpotChangeLabel   = "Spot change"
)

// ComputeSpotExposure is the one aggregation every surface calls. items must
// hold every priced row of the total, spot or not, so the share has a
// denominator. It returns nil when no item is DYNAMIC or when the items carry
// different currencies, which are never summed.
func ComputeSpotExposure(items []SpotExposureItem) *SpotExposure {
	currency, ok := spotCommonCurrency(items)
	if !ok {
		return nil
	}

	var (
		exposure   SpotExposure
		knownCost  float64
		weighted   float64
		knownCount int
	)
	for _, item := range items {
		exposure.TotalCost += item.Cost
		if !item.Category.IsSpot() {
			continue
		}
		exposure.SpotResources++
		exposure.SpotCost += item.Cost
		if !spotRiskKnown(item.Risk) {
			exposure.RiskUnknownResources++
			continue
		}
		knownCount++
		knownCost += item.Cost
		weighted += item.Cost * item.Risk
	}
	if exposure.SpotResources == 0 {
		return nil
	}

	exposure.Currency = currency
	if exposure.TotalCost > 0 {
		share := exposure.SpotCost / exposure.TotalCost
		exposure.SpotShare = &share
	}
	if knownCount > 0 && knownCost > 0 {
		risk := weighted / knownCost
		exposure.WeightedRisk = &risk
	}
	exposure.Display = exposure.describe()
	return &exposure
}

// SpotExposureFromCosts computes the exposure of projected cost results,
// leaving out rows that failed to price.
func SpotExposureFromCosts(results []CostResult) *SpotExposure {
	return ComputeSpotExposure(spotItemsFromCosts(results))
}

// ComputeSpotExposureChange compares the spot cost before and after a diff. It
// returns nil when neither side holds a spot row or the currencies differ.
func ComputeSpotExposureChange(diff *DiffResult) *SpotExposureChange {
	if diff == nil {
		return nil
	}
	before := make([]CostResult, 0, len(diff.Entries))
	for i := range diff.Entries {
		if diff.Entries[i].Before != nil {
			before = append(before, *diff.Entries[i].Before)
		}
	}
	beforeItems := spotItemsFromCosts(before)
	afterItems := spotItemsFromCosts(diff.AfterCosts())

	currency, ok := spotCommonCurrency(append(append([]SpotExposureItem{}, beforeItems...), afterItems...))
	if !ok {
		return nil
	}
	beforeExposure := ComputeSpotExposure(beforeItems)
	afterExposure := ComputeSpotExposure(afterItems)
	if beforeExposure == nil && afterExposure == nil {
		return nil
	}

	change := &SpotExposureChange{Currency: currency}
	if beforeExposure != nil {
		change.BeforeSpotCost = beforeExposure.SpotCost
	}
	if afterExposure != nil {
		change.AfterSpotCost = afterExposure.SpotCost
	}
	change.DeltaSpotCost = change.AfterSpotCost - change.BeforeSpotCost
	change.Display = fmt.Sprintf("%+.2f %s (before %.2f, after %.2f)",
		change.DeltaSpotCost, currency, change.BeforeSpotCost, change.AfterSpotCost)
	return change
}

func spotItemsFromCosts(results []CostResult) []SpotExposureItem {
	items := make([]SpotExposureItem, 0, len(results))
	for i := range results {
		if costResultHasError(results[i]) {
			continue
		}
		items = append(items, SpotExposureItem{
			Cost:     results[i].Monthly,
			Currency: results[i].Currency,
			Category: results[i].PricingCategory,
			Risk:     results[i].SpotInterruptionRisk,
		})
	}
	return items
}

func spotCommonCurrency(items []SpotExposureItem) (string, bool) {
	currency := ""
	for _, item := range items {
		if item.Currency == "" {
			continue
		}
		if currency == "" {
			currency = item.Currency
			continue
		}
		if item.Currency != currency {
			return "", false
		}
	}
	if currency == "" {
		currency = defaultCurrency
	}
	return currency, true
}

func spotRiskKnown(risk float64) bool {
	return risk > 0 && risk <= 1 && !math.IsNaN(risk)
}

func (e *SpotExposure) describe() string {
	text := fmt.Sprintf("%.2f %s", e.SpotCost, e.Currency)
	if e.SpotShare != nil {
		text = fmt.Sprintf("%s (%.1f%% of %.2f %s)", text, *e.SpotShare*percentScale, e.TotalCost, e.Currency)
	}
	switch {
	case e.WeightedRisk != nil:
		text += ", weighted risk " + formatSpotRiskValue(*e.WeightedRisk)
	case e.RiskUnknownResources == e.SpotResources:
		text += ", weighted risk unknown (" + spotNoRiskReason + ")"
	default:
		text += ", weighted risk unknown"
	}
	if e.RiskUnknownResources > 0 && e.RiskUnknownResources < e.SpotResources {
		text += fmt.Sprintf(" (%d of %d spot resources have no risk score and are excluded)",
			e.RiskUnknownResources, e.SpotResources)
	}
	return text
}
