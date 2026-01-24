package tui

import (
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
			output := RenderCostSummary(tt.results, tt.width)
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
