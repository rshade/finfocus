package viewmodel

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/resourcetype"
)

const (
	percentScale = 100
	centsScale   = 100
	deltaEpsilon = 0.001
)

// DisplayField is an ordered label and preformatted value shared by renderers.
type DisplayField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ProviderCostDisplay is a provider subtotal and its share of the displayed total.
type ProviderCostDisplay struct {
	Name         string `json:"name"`
	CostDisplay  string `json:"costDisplay"`
	ShareDisplay string `json:"shareDisplay"`
}

// CostSummaryDisplay preserves the TUI summary's actual-to-monthly fallback policy.
type CostSummaryDisplay struct {
	TotalDisplay        string                `json:"totalDisplay"`
	ResourceCount       int                   `json:"resourceCount"`
	RecommendationCount int                   `json:"recommendationCount"`
	Providers           []ProviderCostDisplay `json:"providers"`
	CarbonEquivalency   string                `json:"carbonEquivalency"`
}

// BuildCostSummaryDisplay aggregates the established TUI presentation in one place.
func BuildCostSummaryDisplay(ctx context.Context, results []engine.CostResult) CostSummaryDisplay {
	display := CostSummaryDisplay{ResourceCount: len(results), Providers: []ProviderCostDisplay{}}
	total := 0.0
	providers := make(map[string]float64)
	for _, result := range results {
		cost := result.Monthly
		if result.TotalCost > 0 {
			cost = result.TotalCost
		}
		total += cost
		providers[resourcetype.ExtractProvider(result.ResourceType)] += cost
		display.RecommendationCount += len(result.Recommendations)
	}
	display.TotalDisplay = fmt.Sprintf("$%.2f", total)
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.Sort(names)
	slices.SortStableFunc(names, func(a, b string) int {
		if providers[a] > providers[b] {
			return -1
		}
		if providers[a] < providers[b] {
			return 1
		}
		return 0
	})
	for _, name := range names {
		share := 0.0
		if total > 0 {
			share = providers[name] / total * percentScale
		}
		display.Providers = append(
			display.Providers,
			ProviderCostDisplay{
				Name:         name,
				CostDisplay:  fmt.Sprintf("$%.2f", providers[name]),
				ShareDisplay: fmt.Sprintf("%.1f%%", share),
			},
		)
	}
	if carbon, found := engine.AggregateSustainability(ctx, results); found {
		if output, err := greenops.Calculate(ctx, carbon); err == nil && !output.IsEmpty {
			display.CarbonEquivalency = output.DisplayText
		}
	}
	return display
}

// AggregationProvidersDisplay formats the TUI's sorted whole-dollar provider subtotals.
func AggregationProvidersDisplay(providers map[string]float64) string {
	parts := make([]string, 0, len(providers))
	for provider, cost := range providers {
		parts = append(parts, fmt.Sprintf("%s:$%.0f", provider, cost))
	}
	slices.Sort(parts)
	return strings.Join(parts, " ")
}

// BreakdownDisplay orders categories and uses each existing view's precision.
func BreakdownDisplay(breakdown map[string]float64, actualDetail bool) []CostBreakdown {
	keys := make([]string, 0, len(breakdown))
	for key := range breakdown {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	result := make([]CostBreakdown, 0, len(keys))
	for _, key := range keys {
		value := engine.FormatOverviewCurrency(breakdown[key])
		if actualDetail {
			value = fmt.Sprintf("$%.4f", breakdown[key])
		}
		result = append(result, CostBreakdown{Name: key, CostDisplay: value})
	}
	return result
}

// SustainabilityDisplay orders metrics and retains the TUI's two-decimal precision.
func SustainabilityDisplay(metrics map[string]engine.SustainabilityMetric) []DisplayField {
	keys := make([]string, 0, len(metrics))
	for key := range metrics {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	fields := make([]DisplayField, 0, len(keys))
	for _, key := range keys {
		metric := metrics[key]
		fields = append(fields, DisplayField{Name: key, Value: fmt.Sprintf("%.2f %s", metric.Value, metric.Unit)})
	}
	return fields
}

// CostDeltaDisplay preserves the TUI's rounding, sign and directional marker.
func CostDeltaDisplay(delta float64) string {
	rounded := math.Round(delta*centsScale) / centsScale
	sign, icon := "", "→"
	if rounded > 0 {
		sign, icon = "+", "↑"
	} else if rounded < 0 {
		icon = "↓"
	}
	return fmt.Sprintf("%s%s %s", sign, FormatMoneyShort(rounded), icon)
}

// LinkedCostRecommendations preserves the TUI's savings ordering and optional savings.
func LinkedCostRecommendations(recs []engine.Recommendation) []CostRecommendation {
	sorted := append([]engine.Recommendation(nil), recs...)
	slices.SortStableFunc(sorted, func(a, b engine.Recommendation) int {
		if a.EstimatedSavings > b.EstimatedSavings {
			return -1
		}
		if a.EstimatedSavings < b.EstimatedSavings {
			return 1
		}
		return 0
	})
	result := make([]CostRecommendation, 0, len(sorted))
	for _, rec := range sorted {
		savings := ""
		if rec.EstimatedSavings > 0 {
			savings = RecommendationSavingsDisplay(rec)
		}
		result = append(result, CostRecommendation{Recommendation: rec, SavingsDisplay: savings})
	}
	return result
}
