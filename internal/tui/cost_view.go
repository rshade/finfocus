package tui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/rshade/finfocus/internal/engine"
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
	ResourceName string // Truncated to 40 chars.
	ResourceType string // e.g., "aws:ec2:Instance".
	Provider     string // e.g., "aws".
	Monthly      float64
	TotalCost    float64 // For actual costs.
	Delta        float64
	Currency     string
	HasError     bool
	ErrorMsg     string
}

// NewResourceRow converts an engine.CostResult into a display-ready ResourceRow.
func NewResourceRow(result engine.CostResult) ResourceRow {
	name := fmt.Sprintf("%s/%s", result.ResourceType, result.ResourceID)
	if len(name) > maxNameDisplayLen {
		name = name[:truncateOffset] + truncateSuffix
	}
	provider := extractProvider(result.ResourceType)

	return ResourceRow{
		ResourceName: name,
		ResourceType: result.ResourceType,
		Provider:     provider,
		Monthly:      result.Monthly,
		TotalCost:    result.TotalCost,
		Delta:        result.Delta,
		Currency:     result.Currency,
		HasError:     strings.HasPrefix(result.Notes, "ERROR:"),
		ErrorMsg:     result.Notes,
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

// RenderCostSummary renders a styled summary of the cost results using Lip Gloss.
func RenderCostSummary(results []engine.CostResult, width int) string {
	if len(results) == 0 {
		return InfoStyle.Render("No results to display.")
	}

	totalCost := 0.0
	providerCosts := make(map[string]float64)

	for _, r := range results {
		// Use TotalCost if present (Actual), otherwise Monthly (Projected).
		cost := r.Monthly
		if r.TotalCost > 0 {
			cost = r.TotalCost
		}

		totalCost += cost
		provider := extractProvider(r.ResourceType)
		providerCosts[provider] += cost
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
	}

	rows := make([]table.Row, len(results))
	for i, r := range results {
		row := NewResourceRow(r)

		costStr := fmt.Sprintf("$%.2f", row.Monthly)
		deltaStr := RenderDelta(row.Delta)

		rows[i] = table.Row{
			row.ResourceName,
			row.ResourceType,
			row.Provider,
			costStr,
			deltaStr,
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
	}

	rows := make([]table.Row, len(results))
	for i, r := range results {
		row := NewResourceRow(r)
		costStr := fmt.Sprintf("$%.2f", row.TotalCost)

		rows[i] = table.Row{
			row.ResourceName,
			row.ResourceType,
			row.Provider,
			costStr,
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

// RenderDetailView renders the detailed view for a single resource.
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

// RenderLoading renders the loading screen with spinner.
func RenderLoading(loading *LoadingState) string {
	if loading == nil {
		return "Loading..."
	}
	return fmt.Sprintf("\n %s %s\n\n", loading.spinner.View(), loading.message)
}
