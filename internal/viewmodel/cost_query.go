package viewmodel

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/resourcetype"
)

// TablePageSize bounds resource and recommendation pages.
const TablePageSize = 250

// CostDisplay carries preformatted cells for an actual result or time aggregate.
type CostDisplay struct {
	ProvidersDisplay string `json:"providersDisplay,omitempty"`
	ID               string `json:"id"`
	Type             string `json:"type"`
	Cost             string `json:"costDisplay"`
	Currency         string `json:"currency"`
	Trend            string `json:"trend"`
	TrendPoints      string `json:"trendPoints"`
	CanDetail        bool   `json:"canDetail"`
}

// ActualCostSummary carries the canonical CLI aggregate plus carbon.
type ActualCostSummary struct {
	engine.CostSummary

	Carbon greenops.CarbonInput `json:"carbon"`
}

// ActualCostPage combines unchanged engine results with canonical display cells.
type ActualCostPage struct {
	SummaryDisplay CostSummaryDisplay                `json:"summaryDisplay"`
	Results        any                               `json:"results"`
	Aggregations   []engine.CrossProviderAggregation `json:"-"`
	Rows           []CostDisplay                     `json:"rows"`
	Summary        ActualCostSummary                 `json:"summary"`
	Carbon         greenops.CarbonInput              `json:"carbon"`
	CarbonDisplay  string                            `json:"carbonDisplay"`
	TotalDisplay   string                            `json:"totalDisplay"`
	Trends         map[string]string                 `json:"trends"`
	TotalTrend     string                            `json:"totalTrend"`
	Page           int                               `json:"page"`
	TotalPages     int                               `json:"totalPages"`
}

// PageBounds resolves a one-based page and slice bounds, including empty results.
func PageBounds(count, requested int) (int, int, int, int) {
	totalPages := max(1, (count+TablePageSize-1)/TablePageSize)
	page := min(max(1, requested), totalPages)
	start := min((page-1)*TablePageSize, count)
	end := min(start+TablePageSize, count)
	return page, totalPages, start, end
}

// SparklinePoints converts the shared history sparkline to SVG polyline coordinates.
func SparklinePoints(trend string) string {
	const low = '▁'
	const high = '█'
	const step = 12
	const height = 28
	const levels = 7
	points := make([]string, 0, len([]rune(trend)))
	for i, value := range []rune(trend) {
		if value < low || value > high {
			return ""
		}
		points = append(points, fmt.Sprintf("%d,%d", i*step, height-int(value-low)*height/levels))
	}
	return strings.Join(points, " ")
}

// BuildActualCostPage applies shared filtering, sorting, aggregation and formatting.
// Results have already been grouped by the engine fetch; grouping is never repeated.
func BuildActualCostPage(
	ctx context.Context,
	results []engine.CostResult,
	group engine.GroupBy,
	filter string,
	sort SortField,
	requested int,
	trends map[string]string,
	totalTrend string,
) (ActualCostPage, error) {
	filtered := FilterCostResults(append([]engine.CostResult{}, results...), filter)
	SortCostResults(filtered, sort, true)
	page := ActualCostPage{
		SummaryDisplay: BuildCostSummaryDisplay(ctx, filtered),
		Results:        []engine.CostResult{},
		Rows:           []CostDisplay{},
		Summary:        ActualCostSummary{CostSummary: engine.AggregateResults(filtered).Summary},
		Trends:         trends,
		TotalTrend:     totalTrend,
	}
	page.Carbon, _ = engine.AggregateSustainability(ctx, filtered)
	page.Summary.Carbon = page.Carbon
	if page.Carbon.Unit != "" {
		page.CarbonDisplay = fmt.Sprintf("%.2f %s", page.Carbon.Value, page.Carbon.Unit)
	}
	page.TotalDisplay = FormatMoney(engine.SumActualCosts(filtered), page.Summary.Currency)
	if len(filtered) == 0 {
		page.Page, page.TotalPages = 1, 1
		if group.IsTimeBasedGrouping() {
			page.Aggregations = []engine.CrossProviderAggregation{}
			page.Results = page.Aggregations
		}
		return page, nil
	}
	if group.IsTimeBasedGrouping() {
		aggregates, err := engine.CreateCrossProviderAggregation(filtered, group)
		if err != nil {
			return ActualCostPage{}, fmt.Errorf("aggregating costs: %w", err)
		}
		SortTimeAggregations(aggregates, sort)
		var start, end int
		page.Page, page.TotalPages, start, end = PageBounds(len(aggregates), requested)
		page.Aggregations = aggregates[start:end]
		page.Results = page.Aggregations
		for _, a := range page.Aggregations {
			page.Rows = append(
				page.Rows,
				CostDisplay{
					ProvidersDisplay: AggregationProvidersDisplay(a.Providers),
					ID:               a.Period,
					Type:             string(group),
					Cost:             FormatMoney(a.Total, a.Currency),
					Currency:         a.Currency,
				},
			)
		}
		return page, nil
	}
	var start, end int
	page.Page, page.TotalPages, start, end = PageBounds(len(filtered), requested)
	page.Results = filtered[start:end]
	for _, r := range filtered[start:end] {
		page.Rows = append(
			page.Rows,
			CostDisplay{
				ID:          r.ResourceID,
				Type:        r.ResourceType,
				Cost:        FormatMoney(r.TotalCost, r.Currency),
				Currency:    r.Currency,
				CanDetail:   true,
				Trend:       trends[r.ResourceID],
				TrendPoints: SparklinePoints(trends[r.ResourceID]),
			},
		)
	}
	return page, nil
}

// CostBreakdown is a display row using the shared monetary convention.
type CostBreakdown struct {
	Name        string `json:"name"`
	CostDisplay string `json:"costDisplay"`
}

// CostRecommendation preserves full linked recommendation details and savings.
type CostRecommendation struct {
	Recommendation engine.Recommendation `json:"recommendation"`
	SavingsDisplay string                `json:"savingsDisplay"`
}

// ActualCostDetail renders breakdown, sustainability and recommendations in Go.
type ActualCostDetail struct {
	Provider              string                                 `json:"provider"`
	PeriodDisplay         string                                 `json:"periodDisplay"`
	MonthlyDisplay        string                                 `json:"monthlyDisplay"`
	HourlyDisplay         string                                 `json:"hourlyDisplay"`
	DeltaDisplay          string                                 `json:"deltaDisplay"`
	NotesDisplay          string                                 `json:"notesDisplay"`
	SustainabilityDisplay []DisplayField                         `json:"sustainabilityDisplay"`
	Result                engine.CostResult                      `json:"result"`
	CostDisplay           string                                 `json:"costDisplay"`
	Breakdown             []CostBreakdown                        `json:"breakdown"`
	Sustainability        map[string]engine.SustainabilityMetric `json:"sustainability"`
	Recommendations       []CostRecommendation                   `json:"recommendations"`
}

// BuildActualCostDetail adds canonical monetary displays to the engine detail.
func BuildActualCostDetail(cost engine.CostResult) ActualCostDetail {
	detail := ActualCostDetail{
		Result:         cost,
		Sustainability: cost.Sustainability,
	}
	detail.Provider = resourcetype.ExtractProvider(cost.ResourceType)
	if cost.TotalCost > 0 {
		detail.CostDisplay = fmt.Sprintf("$%.2f %s", cost.TotalCost, cost.Currency)
		if !cost.StartDate.IsZero() {
			detail.PeriodDisplay = cost.StartDate.Format("2006-01-02") + " - " + cost.EndDate.Format("2006-01-02")
		}
	} else {
		detail.MonthlyDisplay = fmt.Sprintf("$%.2f %s", cost.Monthly, cost.Currency)
		detail.HourlyDisplay = fmt.Sprintf("$%.4f %s", cost.Hourly, cost.Currency)
	}
	if math.Abs(cost.Delta) > deltaEpsilon {
		detail.DeltaDisplay = CostDeltaDisplay(cost.Delta)
	}
	detail.NotesDisplay = cost.Notes
	if cost.Error != nil {
		detail.NotesDisplay = cost.Error.Message
	}
	detail.Breakdown = BreakdownDisplay(cost.Breakdown, true)
	detail.SustainabilityDisplay = SustainabilityDisplay(cost.Sustainability)
	detail.Recommendations = LinkedCostRecommendations(cost.Recommendations)
	return detail
}
