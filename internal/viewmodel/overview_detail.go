package viewmodel

import (
	"fmt"

	"github.com/rshade/finfocus/internal/engine"
)

// OverviewDetailRecommendation pairs an active recommendation with its canonical
// savings display for both overview detail renderers.
type OverviewDetailRecommendation struct {
	Recommendation engine.Recommendation `json:"recommendation"`
	SavingsDisplay string                `json:"savingsDisplay"`
}

// OverviewDetailRecommendations selects the active recommendations shown in the
// overview detail. Dismissed and snoozed items remain available in list counts.
func OverviewDetailRecommendations(recs []engine.Recommendation) []OverviewDetailRecommendation {
	result := make([]OverviewDetailRecommendation, 0, len(recs))
	for _, rec := range recs {
		if rec.Status == engine.RecommendationStatusDismissed || rec.Status == engine.RecommendationStatusSnoozed {
			continue
		}
		result = append(
			result,
			OverviewDetailRecommendation{
				Recommendation: rec,
				SavingsDisplay: engine.FormatOverviewCurrency(rec.EstimatedSavings),
			},
		)
	}
	return result
}

// OverviewDetailDisplay projects the existing overview detail sections without changing source JSON.
type OverviewDetailDisplay struct {
	ActualBreakdown    []CostBreakdown `json:"actualBreakdown"`
	ProjectedBreakdown []CostBreakdown `json:"projectedBreakdown"`
	Impact             []DisplayField  `json:"impact"`
	Drift              []DisplayField  `json:"drift"`
}

// BuildOverviewDetailDisplay shares overview detail currency, impact and drift conventions.
func BuildOverviewDetailDisplay(row engine.OverviewRowResult, dayOfMonth int) OverviewDetailDisplay {
	display := OverviewDetailDisplay{Impact: OverviewImpactDisplay(row, dayOfMonth), Drift: OverviewDriftDisplay(row)}
	if row.ActualCost != nil {
		display.ActualBreakdown = BreakdownDisplay(row.ActualCost.Breakdown, false)
	}
	if row.ProjectedCost != nil {
		display.ProjectedBreakdown = BreakdownDisplay(row.ProjectedCost.Breakdown, false)
	}
	return display
}

// OverviewImpactDisplay projects status-dependent cost impact using shared engine calculations.
func OverviewImpactDisplay(row engine.OverviewRowResult, dayOfMonth int) []DisplayField {
	if row.Status == engine.StatusActive || row.Delta == nil {
		return nil
	}
	fields := []DisplayField{}
	switch row.Status { //nolint:exhaustive // Active already returned; unknown statuses retain the delta.
	case engine.StatusUpdating, engine.StatusReplacing:
		current := engine.ForceExtrapolateActual(row.Source, dayOfMonth)
		if baseline, ok := engine.GetBaselineProjectedMonthlyCost(row.Source); ok {
			current = baseline
		}
		fields = append(
			fields,
			DisplayField{Name: "Current (est. monthly)", Value: engine.FormatOverviewCurrency(current)},
			DisplayField{
				Name:  "After Change",
				Value: engine.FormatOverviewCurrency(engine.GetProjectedMonthlyCost(row.Source)),
			},
		)
	case engine.StatusCreating:
		fields = append(
			fields,
			DisplayField{
				Name:  "New Monthly Cost",
				Value: engine.FormatOverviewCurrency(engine.GetProjectedMonthlyCost(row.Source)),
			},
		)
	case engine.StatusDeleting:
		fields = append(
			fields,
			DisplayField{
				Name:  "Current (est. monthly)",
				Value: engine.FormatOverviewCurrency(engine.GetExtrapolatedActual(row.Source, dayOfMonth)),
			},
		)
	}
	return append(fields, DisplayField{Name: "Delta", Value: engine.FormatOverviewDelta(*row.Delta)})
}

// OverviewDriftDisplay projects the existing extrapolated, projected and delta amounts.
func OverviewDriftDisplay(row engine.OverviewRowResult) []DisplayField {
	if row.CostDrift == nil {
		return nil
	}
	return []DisplayField{
		{Name: "Extrapolated Monthly", Value: engine.FormatOverviewCurrency(row.CostDrift.ExtrapolatedMonthly)},
		{Name: "Projected", Value: engine.FormatOverviewCurrency(row.CostDrift.Projected)},
		{Name: "Delta", Value: engine.FormatOverviewCurrency(row.CostDrift.Delta)},
		{Name: "Drift", Value: fmt.Sprintf("%.1f%%", row.CostDrift.PercentDrift)},
	}
}
