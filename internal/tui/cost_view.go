package tui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/logging"
)

// Layout constants.
const (
	maxNameDisplayLen = 40
	truncateSuffix    = "..."
	truncateOffset    = maxNameDisplayLen - len(truncateSuffix)
	borderPadding     = 2
	// deltaEpsilon is the minimum absolute delta value to display (avoids floating-point noise).
	deltaEpsilon = 0.001
)

// ResourceRow represents a single row in the interactive resource table.
type ResourceRow struct {
	ResourceName        string // Truncated to 40 chars.
	ResourceType        string // e.g., "aws:ec2:Instance".
	Provider            string // e.g., "aws".
	Monthly             float64
	TotalCost           float64 // For actual costs.
	Delta               float64
	Currency            string
	HasError            bool
	ErrorMsg            string
	RecommendationCount int // Number of recommendations for this resource.
}

// NewResourceRow converts an engine.CostResult into a display-ready ResourceRow.
func NewResourceRow(result engine.CostResult) ResourceRow {
	name := fmt.Sprintf("%s/%s", result.ResourceType, result.ResourceID)
	if len(name) > maxNameDisplayLen {
		name = name[:truncateOffset] + truncateSuffix
	}
	provider := extractProvider(result.ResourceType)

	return ResourceRow{
		ResourceName:        name,
		ResourceType:        result.ResourceType,
		Provider:            provider,
		Monthly:             result.Monthly,
		TotalCost:           result.TotalCost,
		Delta:               result.Delta,
		Currency:            result.Currency,
		HasError:            strings.HasPrefix(result.Notes, "ERROR:"),
		ErrorMsg:            result.Notes,
		RecommendationCount: len(result.Recommendations),
	}
}

// extractProvider extracts the provider name from a Pulumi resource type string.
// e.g., "aws:ec2/instance:Instance" -> "aws".
func extractProvider(resourceType string) string {
	if resourceType == "" {
		return "unknown"
	}
	parts := strings.Split(resourceType, ":")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "unknown"
}

// RenderCostSummary renders a boxed, styled cost summary for the provided cost results.
// It aggregates costs per provider (using `TotalCost` when > 0, otherwise `Monthly`), computes the overall total and resource count, and lists providers sorted by descending cost share.
// If aggregated carbon data is available it appends a carbon-equivalency line.
// The width parameter controls the total box width used for rendering.
// If results is empty, the function returns a "No results to display." message.
// The ctx parameter enables trace ID propagation for contextual logging.
func RenderCostSummary(ctx context.Context, results []engine.CostResult, width int) string {
	if len(results) == 0 {
		return InfoStyle.Render("No results to display.")
	}

	totalCost := 0.0
	providerCosts := make(map[string]float64)

	recCount := 0
	for _, r := range results {
		// Use TotalCost if present (Actual), otherwise Monthly (Projected).
		cost := r.Monthly
		if r.TotalCost > 0 {
			cost = r.TotalCost
		}

		totalCost += cost
		provider := extractProvider(r.ResourceType)
		providerCosts[provider] += cost
		recCount += len(r.Recommendations)
	}

	// Create content.
	var content strings.Builder

	// Header.
	content.WriteString(HeaderStyle.Render("COST SUMMARY"))
	content.WriteString("\n")

	// Total Line.
	content.WriteString(LabelStyle.Render("Total Cost:    "))
	content.WriteString(ValueStyle.Render(fmt.Sprintf("$%.2f", totalCost)))
	content.WriteString(LabelStyle.Render("    Resources: "))
	content.WriteString(ValueStyle.Render(strconv.Itoa(len(results))))
	if recCount > 0 {
		content.WriteString(LabelStyle.Render("    Recommendations: "))
		content.WriteString(ValueStyle.Render(strconv.Itoa(recCount)))
	}
	content.WriteString("\n")

	// Provider Breakdown (sorted by cost desc).
	type pCost struct {
		Name string
		Cost float64
	}
	var pCosts []pCost
	for p, c := range providerCosts {
		pCosts = append(pCosts, pCost{p, c})
	}
	sort.Slice(pCosts, func(i, j int) bool {
		return pCosts[i].Cost > pCosts[j].Cost
	})

	var providerParts []string
	for _, pc := range pCosts {
		pct := 0.0
		if totalCost > 0 {
			pct = (pc.Cost / totalCost) * 100 //nolint:mnd // Percentage calculation.
		}
		part := fmt.Sprintf("%s: $%.2f (%.1f%%)", pc.Name, pc.Cost, pct)
		providerParts = append(providerParts, part)
	}
	content.WriteString(LabelStyle.Render(strings.Join(providerParts, "  ")))

	// Add carbon equivalency if present.
	if carbonInput, found := aggregateCarbonFromResults(ctx, results); found {
		output, err := greenops.Calculate(ctx, carbonInput)
		if err == nil && !output.IsEmpty {
			content.WriteString("\n")
			content.WriteString(SubtleStyle.Render(output.DisplayText))
		}
	}

	// Box it. Use width-2 to account for borders.
	return BoxStyle.Width(width - borderPadding).Render(content.String())
}

// NewResultTable creates and configures a new table model for cost results.
func NewResultTable(results []engine.CostResult, height int) table.Model {
	columns := []table.Column{
		{Title: "Resource", Width: 40}, //nolint:mnd // Column width.
		{Title: "Type", Width: 30},     //nolint:mnd // Column width.
		{Title: "Provider", Width: 10}, //nolint:mnd // Column width.
		{Title: "Cost", Width: 15},     //nolint:mnd // Column width.
		{Title: "Delta", Width: 15},    //nolint:mnd // Column width.
		{Title: "Recommendations", Width: 15}, //nolint:mnd // Column width.
	}

	rows := make([]table.Row, len(results))
	for i, r := range results {
		row := NewResourceRow(r)

		costStr := fmt.Sprintf("$%.2f", row.Monthly)
		deltaStr := RenderDelta(row.Delta)
		recsStr := formatRecsColumn(row.RecommendationCount)

		rows[i] = table.Row{
			row.ResourceName,
			row.ResourceType,
			row.Provider,
			costStr,
			deltaStr,
			recsStr,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	t.SetStyles(s)

	return t
}

// NewActualCostTable creates a table for actual cost results (using TotalCost).
func NewActualCostTable(results []engine.CostResult, height int) table.Model {
	columns := []table.Column{
		{Title: "Resource", Width: 40},   //nolint:mnd // Column width.
		{Title: "Type", Width: 30},       //nolint:mnd // Column width.
		{Title: "Provider", Width: 10},   //nolint:mnd // Column width.
		{Title: "Total Cost", Width: 15}, //nolint:mnd // Column width.
		{Title: "Recommendations", Width: 15}, //nolint:mnd // Column width.
	}

	rows := make([]table.Row, len(results))
	for i, r := range results {
		row := NewResourceRow(r)
		costStr := fmt.Sprintf("$%.2f", row.TotalCost)
		recsStr := formatRecsColumn(row.RecommendationCount)

		rows[i] = table.Row{
			row.ResourceName,
			row.ResourceType,
			row.Provider,
			costStr,
			recsStr,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	t.SetStyles(s)

	return t
}

// NewAggregationTable creates a table for cross-provider aggregations.
func NewAggregationTable(aggs []engine.CrossProviderAggregation, height int) table.Model {
	columns := []table.Column{
		{Title: "Period", Width: 20},    //nolint:mnd // Column width.
		{Title: "Providers", Width: 40}, //nolint:mnd // Column width.
		{Title: "Total", Width: 15},     //nolint:mnd // Column width.
	}

	rows := make([]table.Row, len(aggs))
	for i, agg := range aggs {
		var providerSummary []string
		for p, cost := range agg.Providers {
			providerSummary = append(providerSummary, fmt.Sprintf("%s:$%.0f", p, cost))
		}
		sort.Strings(providerSummary) // Consistent order.

		rows[i] = table.Row{
			agg.Period,
			strings.Join(providerSummary, " "),
			fmt.Sprintf("$%.2f", agg.Total),
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	t.SetStyles(s)

	return t
}

// RenderDetailView renders a boxed, human-readable detail view for the given resource.
// It includes resource ID, type, provider, cost (total or monthly/hourly), an optional period,
// delta (shown only when its magnitude exceeds deltaEpsilon), a sorted breakdown section,
// a sustainability section when metrics are present, and a notes section where messages
// prefixed with "ERROR:" are rendered with critical styling.
// The resulting content is wrapped to the provided width (accounting for border padding)
// and returned as a string.
func RenderDetailView(resource engine.CostResult, width int) string {
	var content strings.Builder

	// Header.
	content.WriteString(HeaderStyle.Render("RESOURCE DETAIL"))
	content.WriteString("\n\n")

	// ID and Type.
	content.WriteString(LabelStyle.Render("Resource ID:   "))
	content.WriteString(ValueStyle.Render(resource.ResourceID))
	content.WriteString("\n")

	content.WriteString(LabelStyle.Render("Type:          "))
	content.WriteString(ValueStyle.Render(resource.ResourceType))
	content.WriteString("\n")

	content.WriteString(LabelStyle.Render("Provider:      "))
	content.WriteString(ValueStyle.Render(extractProvider(resource.ResourceType)))
	content.WriteString("\n\n")

	// Cost.
	if resource.TotalCost > 0 {
		content.WriteString(LabelStyle.Render("Total Cost:    "))
		content.WriteString(ValueStyle.Render(fmt.Sprintf("$%.2f %s", resource.TotalCost, resource.Currency)))
		content.WriteString("\n")

		if !resource.StartDate.IsZero() {
			content.WriteString(LabelStyle.Render("Period:        "))
			content.WriteString(ValueStyle.Render(fmt.Sprintf("%s - %s",
				resource.StartDate.Format("2006-01-02"),
				resource.EndDate.Format("2006-01-02"))))
			content.WriteString("\n")
		}
	} else {
		content.WriteString(LabelStyle.Render("Monthly Cost:  "))
		content.WriteString(ValueStyle.Render(fmt.Sprintf("$%.2f %s", resource.Monthly, resource.Currency)))
		content.WriteString("\n")

		content.WriteString(LabelStyle.Render("Hourly Cost:   "))
		content.WriteString(ValueStyle.Render(fmt.Sprintf("$%.4f %s", resource.Hourly, resource.Currency)))
		content.WriteString("\n")
	}

	if math.Abs(resource.Delta) > deltaEpsilon {
		content.WriteString(LabelStyle.Render("Delta:         "))
		content.WriteString(RenderDelta(resource.Delta))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Breakdown.
	if len(resource.Breakdown) > 0 {
		content.WriteString(HeaderStyle.Render("BREAKDOWN"))
		content.WriteString("\n")

		// Sort keys.
		keys := make([]string, 0, len(resource.Breakdown))
		for k := range resource.Breakdown {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			content.WriteString(fmt.Sprintf("- %s: $%.4f\n", k, resource.Breakdown[k]))
		}
		content.WriteString("\n")
	}

	// Sustainability metrics.
	renderSustainabilitySection(&content, resource.Sustainability)

	// Recommendations (FR-008: after sustainability, before notes).
	renderRecommendationsSection(&content, resource.Recommendations)

	// Notes/Errors.
	if resource.Notes != "" {
		content.WriteString(HeaderStyle.Render("NOTES"))
		content.WriteString("\n")
		if strings.HasPrefix(resource.Notes, "ERROR:") {
			content.WriteString(CriticalStyle.Render(resource.Notes))
		} else {
			content.WriteString(resource.Notes)
		}
		content.WriteString("\n")
	}

	return BoxStyle.Width(width - borderPadding).Render(content.String())
}

// RenderLoading returns the string to display for a loading screen.
// If loading is nil, it returns the plain text "Loading...". Otherwise it
// returns a string combining the loading spinner view and the loading message.
func RenderLoading(loading *LoadingState) string {
	if loading == nil {
		return "Loading..."
	}
	return fmt.Sprintf("\n %s %s\n\n", loading.spinner.View(), loading.message)
}

// renderSustainabilitySection writes a "SUSTAINABILITY" section to content when sustainability
// metrics are present.
//
// The section begins with a header and a newline, followed by one line per metric in the
// form "- <name>: <value> <unit>" where value is formatted with two decimal places. Metric
// keys are rendered in sorted order for deterministic output. If the sustainability map is
// empty the function returns without writing anything.
//
// Parameters:
//   - content: destination builder to which the section will be written.
//   - sustainability: map of metric name to SustainabilityMetric to render.
func renderSustainabilitySection(content *strings.Builder, sustainability map[string]engine.SustainabilityMetric) {
	if len(sustainability) == 0 {
		return
	}

	content.WriteString(HeaderStyle.Render("SUSTAINABILITY"))
	content.WriteString("\n")

	// Sort metric keys for deterministic output.
	keys := make([]string, 0, len(sustainability))
	for k := range sustainability {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		metric := sustainability[k]
		fmt.Fprintf(content, "- %s: %.2f %s\n", k, metric.Value, metric.Unit)
	}
	content.WriteString("\n")
}

// renderRecommendationsSection writes a "RECOMMENDATIONS" section to content when
// recommendations are present. Recommendations are sorted by estimated savings in
// descending order (FR-009). Each recommendation shows its action type, description,
// and optional savings. Reasoning entries are rendered as indented warning lines
// beneath the description (FR-002).
func renderRecommendationsSection(content *strings.Builder, recommendations []engine.Recommendation) {
	if len(recommendations) == 0 {
		return
	}

	// Copy to avoid mutating the caller's slice during sort.
	sorted := make([]engine.Recommendation, len(recommendations))
	copy(sorted, recommendations)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].EstimatedSavings > sorted[j].EstimatedSavings
	})

	content.WriteString(HeaderStyle.Render("RECOMMENDATIONS"))
	content.WriteString("\n")

	for _, rec := range sorted {
		savingsStr := ""
		if rec.EstimatedSavings > 0 {
			currency := rec.Currency
			if currency == "" {
				currency = defaultCurrency
			}
			savingsStr = fmt.Sprintf(" ($%.2f %s/mo savings)",
				rec.EstimatedSavings, currency)
		}
		fmt.Fprintf(content, "- [%s] %s%s\n",
			rec.Type, rec.Description, savingsStr)

		for _, reason := range rec.Reasoning {
			fmt.Fprintf(content, "    %s\n",
				WarningStyle.Render(reason))
		}
	}
	content.WriteString("\n")
}

// formatRecsColumn returns the recommendation count as a string for TUI table display.
// Returns "-" when the count is zero so the column stays visually clean.
func formatRecsColumn(count int) string {
	if count == 0 {
		return "-"
	}
	return strconv.Itoa(count)
}

// aggregateCarbonFromResults extracts and sums carbon_footprint metrics from all results.
// aggregateCarbonFromResults aggregates carbon footprint metrics from the given cost results.
// It scans each result's Sustainability map for the canonical carbon metric key or a deprecated
// fallback, normalizes found values to kilograms, and sums them.
// ctx enables trace ID propagation for warning logs.
// Invalid or unnormalizable units are logged and skipped.
// It returns a CarbonInput containing the total carbon in kilograms and `true` if any carbon
// data was found; otherwise it returns a zero-value CarbonInput and `false`.
func aggregateCarbonFromResults(ctx context.Context, results []engine.CostResult) (greenops.CarbonInput, bool) {
	totalCarbon := 0.0
	found := false

	for _, r := range results {
		if r.Sustainability == nil {
			continue
		}

		// Check for canonical key first.
		metric, ok := r.Sustainability[greenops.CarbonMetricKey]
		if !ok {
			// Fallback to deprecated key.
			metric, ok = r.Sustainability[greenops.DeprecatedCarbonKey]
		}

		if ok {
			// Normalize to kg before summing.
			kg, err := greenops.NormalizeToKg(metric.Value, metric.Unit)
			if err != nil {
				logging.FromContext(ctx).Warn().
					Ctx(ctx).
					Str("resource_type", r.ResourceType).
					Str("resource_id", r.ResourceID).
					Str("unit", metric.Unit).
					Err(err).
					Msg("skipped resource due to NormalizeToKg error")
				continue
			}
			totalCarbon += kg
			found = true
		}
	}

	// Always use kg as we normalize all values to kilograms.
	unit := ""
	if found {
		unit = "kg"
	}

	return greenops.CarbonInput{Value: totalCarbon, Unit: unit}, found
}
