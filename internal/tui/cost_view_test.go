package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func TestRenderCostSummary(t *testing.T) {
	tests := []struct {
		name     string
		results  []engine.CostResult
		width    int
		contains []string
	}{
		{
			name:     "empty results",
			results:  []engine.CostResult{},
			width:    80,
			contains: []string{"No results to display"},
		},
		{
			name: "single resource",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "i-123",
					Monthly:      100.0,
					Currency:     "USD",
				},
			},
			width: 80,
			contains: []string{
				"COST SUMMARY",
				"Total Cost:", "$100.00",
				"aws:", "$100.00",
			},
		},
		{
			name: "multiple providers",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					Monthly:      100.0,
				},
				{
					ResourceType: "azure:compute/vm",
					Monthly:      50.0,
				},
			},
			width: 80,
			contains: []string{
				"Total Cost:", "$150.00",
				"aws:", "$100.00",
				"azure:", "$50.00",
			},
		},
		{
			name: "actual costs",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					TotalCost:    123.45,
					Monthly:      0, // Projected
					Currency:     "USD",
				},
			},
			width: 80,
			contains: []string{
				"COST SUMMARY",
				"Total Cost:", "$123.45",
				"aws:", "$123.45",
			},
		},
		{
			name: "very large costs",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					Monthly:      1234567.89,
					Currency:     "USD",
				},
			},
			width: 80,
			contains: []string{
				"COST SUMMARY",
				"$1234567.89",
			},
		},
		{
			name: "zero costs",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					Monthly:      0.0,
					Currency:     "USD",
				},
			},
			width: 80,
			contains: []string{
				"Total Cost:", "$0.00",
			},
		},
		{
			name: "resource without provider prefix",
			results: []engine.CostResult{
				{
					ResourceType: "bucket",
					Monthly:      25.0,
					Currency:     "USD",
				},
			},
			width: 80,
			contains: []string{
				"COST SUMMARY",
				"bucket:", "$25.00",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderCostSummary(context.Background(), tt.results, tt.width)
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
		})
	}
}

func TestRenderDetailView(t *testing.T) {
	tests := []struct {
		name        string
		resource    engine.CostResult
		width       int
		contains    []string
		notContains []string
	}{
		{
			name: "basic resource",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				ResourceID:   "i-123",
				Monthly:      50.0,
				Hourly:       0.06,
				Currency:     "USD",
			},
			width: 80,
			contains: []string{
				"RESOURCE DETAIL",
				"i-123",
				"aws:ec2/instance",
				"Monthly Cost:", "$50.00",
			},
		},
		{
			name: "actual cost resource",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				TotalCost:    75.0,
				Currency:     "USD",
			},
			width: 80,
			contains: []string{
				"Total Cost:", "$75.00",
			},
		},
		{
			name: "resource with delta",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				Monthly:      50.0,
				Delta:        10.0,
			},
			width: 80,
			contains: []string{
				"Delta:", "+$10.00 ↑",
			},
		},
		{
			name: "resource with negative delta",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				Monthly:      50.0,
				Delta:        -5.0,
			},
			width: 80,
			contains: []string{
				"Delta:", "-$5.00 ↓",
			},
		},
		{
			name: "resource with very small delta (below epsilon)",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				Monthly:      50.0,
				Delta:        0.0001, // Below epsilon threshold (0.001).
			},
			width:       80,
			notContains: []string{"Delta:"},
		},
		{
			name: "resource with sustainability metrics",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				ResourceID:   "i-123",
				Monthly:      50.0,
				Currency:     "USD",
				Sustainability: map[string]engine.SustainabilityMetric{
					"carbon_footprint": {Value: 150.0, Unit: "kg"},
				},
			},
			width: 80,
			contains: []string{
				"SUSTAINABILITY",
				"carbon_footprint",
			},
		},
		{
			name: "resource with breakdown",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				Monthly:      50.0,
				Breakdown: map[string]float64{
					"compute": 40.0,
					"storage": 10.0,
				},
			},
			width: 80,
			contains: []string{
				"BREAKDOWN",
				"compute:", "$40.00",
				"storage:", "$10.00",
			},
		},
		{
			name: "resource with error notes",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				Monthly:      0.0,
				Notes:        "ERROR: Failed to calculate cost",
			},
			width: 80,
			contains: []string{
				"NOTES",
				"ERROR: Failed to calculate cost",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderDetailView(tt.resource, tt.width)
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, output, s)
			}
		})
	}
}

// TestRenderCostSummary_WithCarbonEquivalencies tests User Story 2:
// Carbon equivalencies in TUI summary view.
func TestRenderCostSummary_WithCarbonEquivalencies(t *testing.T) {
	tests := []struct {
		name        string
		results     []engine.CostResult
		width       int
		contains    []string
		notContains []string
	}{
		{
			name: "displays equivalencies for carbon footprint",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "i-123",
					Monthly:      100.0,
					Currency:     "USD",
					Sustainability: map[string]engine.SustainabilityMetric{
						"carbon_footprint": {Value: 150.0, Unit: "kg"},
					},
				},
			},
			width: 100,
			contains: []string{
				"COST SUMMARY",
				"Equivalent to",
				"miles",
				"smartphones",
				"18,248", // 150 / 0.00822 = 18248 smartphones
			},
		},
		{
			name: "omits equivalencies when below threshold",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "i-123",
					Monthly:      100.0,
					Currency:     "USD",
					Sustainability: map[string]engine.SustainabilityMetric{
						"carbon_footprint": {Value: 0.5, Unit: "kg"}, // Below 1kg threshold
					},
				},
			},
			width:       100,
			notContains: []string{"Equivalent to", "miles", "smartphones"},
		},
		{
			name: "omits equivalencies when no carbon data",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "i-123",
					Monthly:      100.0,
					Currency:     "USD",
					Sustainability: map[string]engine.SustainabilityMetric{
						"energy_consumption": {Value: 2000.0, Unit: "kWh"},
					},
				},
			},
			width:       100,
			notContains: []string{"Equivalent to"},
		},
		{
			name: "aggregates carbon from multiple resources",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "i-123",
					Monthly:      50.0,
					Currency:     "USD",
					Sustainability: map[string]engine.SustainabilityMetric{
						"carbon_footprint": {Value: 75.0, Unit: "kg"},
					},
				},
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "i-456",
					Monthly:      50.0,
					Currency:     "USD",
					Sustainability: map[string]engine.SustainabilityMetric{
						"carbon_footprint": {Value: 75.0, Unit: "kg"},
					},
				},
			},
			width: 100,
			contains: []string{
				"Equivalent to",
				"18,248", // Total 150kg -> 18248 phones
			},
		},
		{
			name: "handles large carbon values with million scaling",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance",
					ResourceID:   "datacenter-1",
					Monthly:      1000000.0,
					Currency:     "USD",
					Sustainability: map[string]engine.SustainabilityMetric{
						"carbon_footprint": {Value: 10000000.0, Unit: "kg"}, // 10 million kg
					},
				},
			},
			width: 100,
			contains: []string{
				"million",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderCostSummary(context.Background(), tt.results, tt.width)

			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, output, s)
			}
		})
	}
}

// TestRenderRecommendationsSection verifies the rendering of recommendation items.
func TestRenderRecommendationsSection(t *testing.T) {
	tests := []struct {
		name        string
		recs        []engine.Recommendation
		contains    []string
		notContains []string
	}{
		{
			name: "single recommendation with savings",
			recs: []engine.Recommendation{
				{Type: "RIGHTSIZE", Description: "Switch to t3.small", EstimatedSavings: 5.00, Currency: "USD"},
			},
			contains: []string{
				"RECOMMENDATIONS",
				"[RIGHTSIZE] Switch to t3.small ($5.00 USD/mo savings)",
			},
		},
		{
			name: "recommendation without savings",
			recs: []engine.Recommendation{
				{Type: "TERMINATE", Description: "Resource is idle during weekends"},
			},
			contains:    []string{"[TERMINATE] Resource is idle during weekends"},
			notContains: []string{"savings"},
		},
		{
			name: "multiple recommendations sorted by savings descending",
			recs: []engine.Recommendation{
				{Type: "TERMINATE", Description: "Idle", EstimatedSavings: 2.0, Currency: "USD"},
				{Type: "MIGRATE", Description: "Graviton", EstimatedSavings: 8.0, Currency: "USD"},
				{Type: "RIGHTSIZE", Description: "Downsize", EstimatedSavings: 5.0, Currency: "USD"},
			},
			contains: []string{
				"[MIGRATE] Graviton ($8.00 USD/mo savings)",
				"[RIGHTSIZE] Downsize ($5.00 USD/mo savings)",
				"[TERMINATE] Idle ($2.00 USD/mo savings)",
			},
		},
		{
			name: "recommendations with reasoning display indented warning lines",
			recs: []engine.Recommendation{
				{
					Type: "MIGRATE", Description: "Switch to Graviton",
					EstimatedSavings: 8.0, Currency: "USD",
					Reasoning: []string{"Ensure application compatibility with ARM64 architecture"},
				},
			},
			contains: []string{
				"[MIGRATE] Switch to Graviton ($8.00 USD/mo savings)",
				"Ensure application compatibility with ARM64 architecture",
			},
		},
		{
			name:        "empty recommendations slice produces no output",
			recs:        []engine.Recommendation{},
			notContains: []string{"RECOMMENDATIONS"},
		},
		{
			name:        "nil recommendations produces no output",
			recs:        nil,
			notContains: []string{"RECOMMENDATIONS"},
		},
		{
			name: "default currency USD when currency field is empty",
			recs: []engine.Recommendation{
				{Type: "RIGHTSIZE", Description: "Resize", EstimatedSavings: 3.50, Currency: ""},
			},
			contains: []string{"($3.50 USD/mo savings)"},
		},
		{
			name: "unrecognized action type rendered as-is in brackets",
			recs: []engine.Recommendation{
				{Type: "UNKNOWN_TYPE", Description: "Some action"},
			},
			contains: []string{"[UNKNOWN_TYPE] Some action"},
		},
		{
			name: "10+ recommendations all render without truncation",
			recs: func() []engine.Recommendation {
				recs := make([]engine.Recommendation, 12)
				for i := range recs {
					recs[i] = engine.Recommendation{
						Type:        "RIGHTSIZE",
						Description: "Recommendation " + strings.Repeat("X", i+1),
					}
				}
				return recs
			}(),
			contains: []string{
				"Recommendation X",
				"Recommendation XXXXXXXXXXXX",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content strings.Builder
			renderRecommendationsSection(&content, tt.recs)
			output := content.String()
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, output, s)
			}
		})
	}
}

// TestRenderRecommendationsSection_SortStability verifies that recommendations with
// equal savings maintain their original order.
func TestRenderRecommendationsSection_SortStability(t *testing.T) {
	recs := []engine.Recommendation{
		{Type: "FIRST", Description: "First with 5", EstimatedSavings: 5.0, Currency: "USD"},
		{Type: "SECOND", Description: "Second with 5", EstimatedSavings: 5.0, Currency: "USD"},
		{Type: "THIRD", Description: "Third with 5", EstimatedSavings: 5.0, Currency: "USD"},
	}
	var content strings.Builder
	renderRecommendationsSection(&content, recs)
	output := content.String()

	idxFirst := strings.Index(output, "[FIRST]")
	idxSecond := strings.Index(output, "[SECOND]")
	idxThird := strings.Index(output, "[THIRD]")
	assert.True(t, idxFirst < idxSecond, "FIRST should appear before SECOND")
	assert.True(t, idxSecond < idxThird, "SECOND should appear before THIRD")
}

// TestRenderDetailViewRecommendations verifies the RECOMMENDATIONS section renders
// correctly in the detail view for various recommendation states.
func TestRenderDetailViewRecommendations(t *testing.T) {
	tests := []struct {
		name        string
		resource    engine.CostResult
		width       int
		contains    []string
		notContains []string
		expectOrder []string // optional: verify these strings appear in order
	}{
		{
			name: "RECOMMENDATIONS section appears after SUSTAINABILITY and before NOTES",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				ResourceID:   "i-123",
				Monthly:      50.0,
				Currency:     "USD",
				Sustainability: map[string]engine.SustainabilityMetric{
					"carbon_footprint": {Value: 10.0, Unit: "kg"},
				},
				Recommendations: []engine.Recommendation{
					{Type: "RIGHTSIZE", Description: "Switch to t3.small", EstimatedSavings: 5.0, Currency: "USD"},
				},
				Notes: "Some note",
			},
			width: 100,
			contains: []string{
				"RECOMMENDATIONS",
				"[RIGHTSIZE] Switch to t3.small",
			},
			expectOrder: []string{"SUSTAINABILITY", "RECOMMENDATIONS", "NOTES"},
		},
		{
			name: "section absent when Recommendations is nil",
			resource: engine.CostResult{
				ResourceType:    "aws:ec2/instance",
				ResourceID:      "i-123",
				Monthly:         50.0,
				Currency:        "USD",
				Recommendations: nil,
			},
			width:       100,
			notContains: []string{"RECOMMENDATIONS"},
		},
		{
			name: "nil Recommendations with other sections still renders correctly",
			resource: engine.CostResult{
				ResourceType:    "aws:ec2/instance",
				ResourceID:      "i-123",
				Monthly:         50.0,
				Currency:        "USD",
				Recommendations: nil,
				Sustainability: map[string]engine.SustainabilityMetric{
					"carbon_footprint": {Value: 10.0, Unit: "kg"},
				},
				Notes: "Some notes",
			},
			width:       100,
			contains:    []string{"RESOURCE DETAIL", "SUSTAINABILITY", "NOTES"},
			notContains: []string{"RECOMMENDATIONS"},
		},
		{
			name: "empty Recommendations slice produces no RECOMMENDATIONS section",
			resource: engine.CostResult{
				ResourceType:    "aws:ec2/instance",
				ResourceID:      "i-456",
				Monthly:         75.0,
				Currency:        "USD",
				Recommendations: []engine.Recommendation{},
				Breakdown: map[string]float64{
					"compute": 60.0,
					"storage": 15.0,
				},
			},
			width:       100,
			contains:    []string{"RESOURCE DETAIL", "BREAKDOWN"},
			notContains: []string{"RECOMMENDATIONS"},
		},
		{
			name: "all other sections render normally without recommendations",
			resource: engine.CostResult{
				ResourceType: "aws:ec2/instance",
				ResourceID:   "i-789",
				Monthly:      100.0,
				Hourly:       0.14,
				Currency:     "USD",
				Breakdown: map[string]float64{
					"compute": 80.0,
					"storage": 20.0,
				},
				Sustainability: map[string]engine.SustainabilityMetric{
					"carbon_footprint": {Value: 50.0, Unit: "kg"},
				},
				Recommendations: nil,
				Notes:           "Test notes for resource",
			},
			width: 100,
			contains: []string{
				"RESOURCE DETAIL", "i-789", "aws:ec2/instance",
				"Monthly Cost:", "$100.00",
				"BREAKDOWN", "compute:",
				"SUSTAINABILITY", "carbon_footprint",
				"NOTES", "Test notes for resource",
			},
			notContains: []string{"RECOMMENDATIONS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderDetailView(tt.resource, tt.width)

			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, output, s)
			}
			if len(tt.expectOrder) > 1 {
				prevIdx := -1
				for _, s := range tt.expectOrder {
					idx := strings.Index(output, s)
					assert.Greater(t, idx, prevIdx, "%q should appear after previous ordered item", s)
					prevIdx = idx
				}
			}
		})
	}
}

// TestRenderCostSummary_EquivalencyStyling tests that equivalencies use consistent TUI styling.
func TestRenderCostSummary_EquivalencyStyling(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance",
			ResourceID:   "i-123",
			Monthly:      100.0,
			Currency:     "USD",
			Sustainability: map[string]engine.SustainabilityMetric{
				"carbon_footprint": {Value: 150.0, Unit: "kg"},
			},
		},
	}

	output := RenderCostSummary(context.Background(), results, 100)

	// The equivalency text should appear on its own line after the provider breakdown
	lines := strings.Split(output, "\n")
	foundEquivalency := false
	for _, line := range lines {
		if strings.Contains(line, "Equivalent to") {
			foundEquivalency = true
			break
		}
	}
	assert.True(t, foundEquivalency, "Equivalency text should appear in output")
}
