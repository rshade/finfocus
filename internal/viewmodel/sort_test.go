package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func floatPtr(v float64) *float64 { return &v }

func TestSortOverviewRows(t *testing.T) {
	t.Parallel()

	rows := func() []engine.OverviewRowResult {
		return []engine.OverviewRowResult{
			{URN: "urn:b", Type: "aws:ec2:Instance", Projected: floatPtr(20), Delta: floatPtr(5)},
			{URN: "urn:a", Type: "aws:s3:Bucket", Projected: floatPtr(10), Delta: floatPtr(-1)},
			// No projected/actual cost and nil delta: sorts as 0.
			{URN: "urn:c", Type: "aws:lambda:Function"},
			{URN: "urn:d", Type: "aws:rds:Instance", ActualMTD: floatPtr(15), Delta: floatPtr(2)},
		}
	}

	tests := []struct {
		name  string
		field SortField
		want  []string // expected URN order
	}{
		{
			name:  "cost descending uses projected then actual then zero",
			field: SortByCost,
			want:  []string{"urn:b", "urn:d", "urn:a", "urn:c"},
		},
		{
			name:  "name ascending by URN",
			field: SortByName,
			want:  []string{"urn:a", "urn:b", "urn:c", "urn:d"},
		},
		{
			name:  "type ascending",
			field: SortByType,
			want:  []string{"urn:b", "urn:c", "urn:d", "urn:a"},
		},
		{
			name:  "delta descending with nil delta as zero",
			field: SortByDelta,
			want:  []string{"urn:b", "urn:d", "urn:c", "urn:a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := rows()
			SortOverviewRows(got, tt.field)
			urns := make([]string, len(got))
			for i, r := range got {
				urns[i] = r.URN
			}
			assert.Equal(t, tt.want, urns)
		})
	}
}

func TestOverviewRowSortCost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  engine.OverviewRowResult
		want float64
	}{
		{name: "projected preferred", row: engine.OverviewRowResult{
			Projected: floatPtr(12), ActualMTD: floatPtr(99),
		}, want: 12},
		{name: "actual when no projected", row: engine.OverviewRowResult{
			ActualMTD: floatPtr(7.5),
		}, want: 7.5},
		{name: "zero when neither", row: engine.OverviewRowResult{}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, OverviewRowSortCost(tt.row), 1e-9)
		})
	}
}

func TestOverviewRowSortDelta(t *testing.T) {
	t.Parallel()

	assert.InDelta(t, 3.25, OverviewRowSortDelta(engine.OverviewRowResult{Delta: floatPtr(3.25)}), 1e-9)
	assert.InDelta(t, 0.0, OverviewRowSortDelta(engine.OverviewRowResult{}), 1e-9)
}

func TestSortCostResults(t *testing.T) {
	t.Parallel()

	results := func() []engine.CostResult {
		return []engine.CostResult{
			{ResourceID: "res-b", ResourceType: "aws:ec2:Instance", Monthly: 20, TotalCost: 8, Delta: 1},
			{ResourceID: "res-a", ResourceType: "aws:s3:Bucket", Monthly: 10, TotalCost: 30, Delta: -2},
			{ResourceID: "res-c", ResourceType: "aws:lambda:Function", Monthly: 30, TotalCost: 15, Delta: 4},
		}
	}

	tests := []struct {
		name     string
		field    SortField
		isActual bool
		want     []string // expected ResourceID order
	}{
		{
			name:  "cost descending by monthly for projected",
			field: SortByCost, isActual: false,
			want: []string{"res-c", "res-b", "res-a"},
		},
		{
			name:  "cost descending by total for actual",
			field: SortByCost, isActual: true,
			want: []string{"res-a", "res-c", "res-b"},
		},
		{
			name:  "name ascending by resource ID",
			field: SortByName,
			want:  []string{"res-a", "res-b", "res-c"},
		},
		{
			name:  "type ascending",
			field: SortByType,
			want:  []string{"res-b", "res-c", "res-a"},
		},
		{
			name:  "delta descending",
			field: SortByDelta,
			want:  []string{"res-c", "res-b", "res-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := results()
			SortCostResults(got, tt.field, tt.isActual)
			ids := make([]string, len(got))
			for i, r := range got {
				ids[i] = r.ResourceID
			}
			assert.Equal(t, tt.want, ids)
		})
	}
}

func TestSortRecommendations(t *testing.T) {
	t.Parallel()

	recs := func() []engine.Recommendation {
		return []engine.Recommendation{
			{ResourceID: "res-b", Type: "Terminate", EstimatedSavings: 5},
			{ResourceID: "res-a", Type: "Right-sizing", EstimatedSavings: 50},
			{ResourceID: "res-c", Type: "Delete Unused", EstimatedSavings: 20},
		}
	}

	tests := []struct {
		name  string
		field RecommendationSortField
		want  []string // expected ResourceID order
	}{
		{
			name:  "savings descending",
			field: SortBySavings,
			want:  []string{"res-a", "res-c", "res-b"},
		},
		{
			name:  "resource ID ascending",
			field: SortByResourceID,
			want:  []string{"res-a", "res-b", "res-c"},
		},
		{
			name:  "action type ascending",
			field: SortByActionType,
			want:  []string{"res-c", "res-a", "res-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := recs()
			SortRecommendations(got, tt.field)
			ids := make([]string, len(got))
			for i, r := range got {
				ids[i] = r.ResourceID
			}
			assert.Equal(t, tt.want, ids)
		})
	}
}
