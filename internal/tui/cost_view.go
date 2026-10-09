package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/resourcetype"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// Layout constants.
const (
	maxNameDisplayLen = 40
	truncateSuffix    = "..."
	truncateOffset    = maxNameDisplayLen - len(truncateSuffix)
	borderPadding     = 2
	// trendColumnWidth fits a 7-cell Unicode sparkline.
	trendColumnWidth = 8
)

// Column titles shared by the cost and overview tables.
const (
	columnTitleResource = "Resource"
	columnTitleType     = "Type"
	columnTitleDelta    = "Delta"
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

	errorMsg := result.Notes
	if result.Error != nil && result.Error.Message != "" {
		errorMsg = result.Error.Message
	}

	return ResourceRow{
		ResourceName:        name,
		ResourceType:        result.ResourceType,
		Provider:            provider,
		Monthly:             result.Monthly,
		TotalCost:           result.TotalCost,
		Delta:               result.Delta,
		Currency:            result.Currency,
		HasError:            result.Error != nil || strings.HasPrefix(result.Notes, "ERROR:"),
		ErrorMsg:            errorMsg,
		RecommendationCount: len(result.Recommendations),
	}
}

// extractProvider returns the cloud that bills a resource type string.
// e.g., "aws:ec2/instance:Instance" and "aws-native:ec2:Instance" -> "aws".
func extractProvider(resourceType string) string {
	return resourcetype.ExtractProvider(resourceType)
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

	display := viewmodel.BuildCostSummaryDisplay(ctx, results)
	// Create content.
	var content strings.Builder

	// Header.
	content.WriteString(HeaderStyle.Render("COST SUMMARY"))
	content.WriteString("\n")

	// Total Line.
	content.WriteString(LabelStyle.Render("Total Cost:    "))
	content.WriteString(ValueStyle.Render(display.TotalDisplay))
	content.WriteString(LabelStyle.Render("    Resources: "))
	content.WriteString(ValueStyle.Render(strconv.Itoa(len(results))))
	if display.RecommendationCount > 0 {
		content.WriteString(LabelStyle.Render("    Recommendations: "))
		content.WriteString(ValueStyle.Render(strconv.Itoa(display.RecommendationCount)))
	}
	content.WriteString("\n")

	var providerParts []string
	for _, provider := range display.Providers {
		providerParts = append(
			providerParts,
			fmt.Sprintf("%s: %s (%s)", provider.Name, provider.CostDisplay, provider.ShareDisplay),
		)
	}
	content.WriteString(LabelStyle.Render(strings.Join(providerParts, "  ")))
	if display.CarbonEquivalency != "" {
		content.WriteString("\n")
		content.WriteString(SubtleStyle.Render(display.CarbonEquivalency))
	}

	// Box it. Use width-2 to account for borders.
	return BoxStyle.Width(width - borderPadding).Render(content.String())
}

// NewResultTable creates and configures a new table model for cost results.
func NewResultTable(results []engine.CostResult, height int) table.Model {
	return newResultTable(results, height, nil)
}

func newResultTable(results []engine.CostResult, height int, trends map[string]string) table.Model {
	columns := []table.Column{
		{Title: columnTitleResource, Width: 40}, //nolint:mnd // Column width.
		{Title: columnTitleType, Width: 30},     //nolint:mnd // Column width.
		{Title: "Provider", Width: 10},          //nolint:mnd // Column width.
		{Title: "Cost", Width: 15},              //nolint:mnd // Column width.
	}
	if trends != nil {
		columns = append(columns, table.Column{Title: "Trend", Width: trendColumnWidth})
	}
	columns = append(columns,
		table.Column{Title: columnTitleDelta, Width: 15},  //nolint:mnd // Column width.
		table.Column{Title: "Recommendations", Width: 15}, //nolint:mnd // Column width.
	)

	rows := make([]table.Row, len(results))
	for i, r := range results {
		row := NewResourceRow(r)

		costStr := fmt.Sprintf("$%.2f", row.Monthly)
		deltaStr := RenderDelta(row.Delta)
		recsStr := engine.FormatRecommendationCount(row.RecommendationCount)

		rows[i] = table.Row{
			row.ResourceName,
			row.ResourceType,
			row.Provider,
			costStr,
		}
		if trends != nil {
			rows[i] = append(rows[i], trends[r.ResourceID])
		}
		rows[i] = append(rows[i], deltaStr, recsStr)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
		table.WithWidth(tableWidthFromColumns(columns)),
	)

	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	t.SetStyles(s)

	return t
}

// NewActualCostTable creates a table model displaying actual (total) costs for the provided cost results.
//
// Each row corresponds to an entry in the `results` slice and shows Resource, Type, Provider, Total Cost,
// and Recommendations. Total Cost is formatted as USD with two decimal places; Recommendations show the
// formatted recommendation count. The returned table is configured with a focused state, the given height,
// and standard header and selected row styles.
func NewActualCostTable(results []engine.CostResult, height int) table.Model {
	return newActualCostTable(results, height, nil)
}

func newActualCostTable(results []engine.CostResult, height int, trends map[string]string) table.Model {
	columns := []table.Column{
		{Title: columnTitleResource, Width: 40}, //nolint:mnd // Column width.
		{Title: columnTitleType, Width: 30},     //nolint:mnd // Column width.
		{Title: "Provider", Width: 10},          //nolint:mnd // Column width.
		{Title: "Total Cost", Width: 15},        //nolint:mnd // Column width.
	}
	if trends != nil {
		columns = append(columns, table.Column{Title: "Trend", Width: trendColumnWidth})
	}
	columns = append(columns, table.Column{Title: "Recommendations", Width: 15}) //nolint:mnd // Column width.

	rows := make([]table.Row, len(results))
	for i, r := range results {
		row := NewResourceRow(r)
		costStr := fmt.Sprintf("$%.2f", row.TotalCost)
		recsStr := engine.FormatRecommendationCount(row.RecommendationCount)

		rows[i] = table.Row{
			row.ResourceName,
			row.ResourceType,
			row.Provider,
			costStr,
		}
		if trends != nil {
			rows[i] = append(rows[i], trends[r.ResourceID])
		}
		rows[i] = append(rows[i], recsStr)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
		table.WithWidth(tableWidthFromColumns(columns)),
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
		rows[i] = table.Row{
			agg.Period,
			viewmodel.AggregationProvidersDisplay(agg.Providers),
			fmt.Sprintf("$%.2f", agg.Total),
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
		table.WithWidth(tableWidthFromColumns(columns)),
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
	display := viewmodel.BuildActualCostDetail(resource)
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
	content.WriteString(ValueStyle.Render(display.Provider))
	content.WriteString("\n\n")

	// Cost.
	if display.CostDisplay != "" {
		content.WriteString(LabelStyle.Render("Total Cost:    "))
		content.WriteString(ValueStyle.Render(display.CostDisplay))
		content.WriteString("\n")

		if display.PeriodDisplay != "" {
			content.WriteString(LabelStyle.Render("Period:        "))
			content.WriteString(ValueStyle.Render(display.PeriodDisplay))
			content.WriteString("\n")
		}
	} else {
		content.WriteString(LabelStyle.Render("Monthly Cost:  "))
		content.WriteString(ValueStyle.Render(display.MonthlyDisplay))
		content.WriteString("\n")

		content.WriteString(LabelStyle.Render("Hourly Cost:   "))
		content.WriteString(ValueStyle.Render(display.HourlyDisplay))
		content.WriteString("\n")
	}

	if display.DeltaDisplay != "" {
		content.WriteString(LabelStyle.Render("Delta:         "))
		content.WriteString(RenderDelta(resource.Delta))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Breakdown.
	if len(resource.Breakdown) > 0 {
		content.WriteString(HeaderStyle.Render("BREAKDOWN"))
		content.WriteString("\n")

		for _, item := range display.Breakdown {
			fmt.Fprintf(&content, "- %s: %s\n", item.Name, item.CostDisplay)
		}
		content.WriteString("\n")
	}

	// Sustainability metrics.
	renderSustainabilitySection(&content, resource.Sustainability)

	// Recommendations (FR-008: after sustainability, before notes).
	renderRecommendationsSection(&content, resource.Recommendations)

	// Notes/Errors.
	if resource.Notes != "" || resource.Error != nil {
		content.WriteString(HeaderStyle.Render("NOTES"))
		content.WriteString("\n")
		if resource.Error != nil || strings.HasPrefix(resource.Notes, "ERROR:") {
			content.WriteString(CriticalStyle.Render(display.NotesDisplay))
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

	for _, metric := range viewmodel.SustainabilityDisplay(sustainability) {
		fmt.Fprintf(content, "- %s: %s\n", metric.Name, metric.Value)
	}
	content.WriteString("\n")
}

// renderRecommendationsSection writes a "RECOMMENDATIONS" section to content when
// recommendations are present. Recommendations are sorted by estimated savings in
// descending order (FR-009). Each recommendation shows its action type, description,
// and optional savings. Reasoning entries are rendered as indented warning lines
// renderRecommendationsSection writes a formatted "RECOMMENDATIONS" section into content for the
// provided recommendations slice. If recommendations is empty the function returns without writing
// anything. Recommendations are rendered in descending order by EstimatedSavings; each entry is
// written as "- [<Type>] <Description> (<savings> per month)" when EstimatedSavings is greater than
// zero, using defaultCurrency if the recommendation's Currency is empty. Each Reasoning line is
// written on its own indented line and styled with WarningStyle. The section ends with a trailing
// blank line.
func renderRecommendationsSection(content *strings.Builder, recommendations []engine.Recommendation) {
	if len(recommendations) == 0 {
		return
	}

	content.WriteString(HeaderStyle.Render("RECOMMENDATIONS"))
	content.WriteString("\n")

	for _, linked := range viewmodel.LinkedCostRecommendations(recommendations) {
		rec := linked.Recommendation
		savingsStr := ""
		if linked.SavingsDisplay != "" {
			savingsStr = " (" + linked.SavingsDisplay + "/mo savings)"
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
