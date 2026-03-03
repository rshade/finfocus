package tui

import (
	"context"
	"testing"
	"time"

	"github.com/rshade/finfocus/internal/engine"
)

// goldenDayOfMonth is the fixed day used for all golden file tests so that
// delta / cost-impact calculations produce deterministic output regardless
// of the actual date the tests are run.
const goldenDayOfMonth = 15

// goldenWidth is the terminal width used for golden file tests.
const goldenWidth = 100

// newGoldenModel creates a minimal OverviewModel in ViewStateDetail with
// fixed width for deterministic golden file output.
func newGoldenModel(t *testing.T, rows []engine.OverviewRow) OverviewModel {
	t.Helper()
	// Pre-populate ComputedDelta with a fixed day for deterministic output.
	engine.PopulateComputedDeltas(rows, goldenDayOfMonth)
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, rows, len(rows), nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.width = goldenWidth
	model.allRows = rows
	model.rows = rows
	return model
}

// TestGolden_DetailView_ActiveResource verifies the detail view for an active
// resource with both actual and projected cost data.
func TestGolden_DetailView_ActiveResource(t *testing.T) {
	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{
				MTDCost:  75.50,
				Currency: "USD",
				Period:   period,
				Breakdown: map[string]float64{
					"compute": 60.00,
					"storage": 15.50,
				},
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
				Breakdown: map[string]float64{
					"compute": 120.00,
					"storage": 30.00,
				},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_active_resource", output)
}

// TestGolden_DetailView_UpdatingResource verifies the detail view for a resource
// being updated, including property changes and cost impact.
func TestGolden_DetailView_UpdatingResource(t *testing.T) {
	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusUpdating,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "instanceType", OldValue: "t3.medium", NewValue: "t3.large"},
				{Key: "rootBlockDevice", OldValue: "20", NewValue: "50"},
			},
			ActualCost: &engine.ActualCostData{
				MTDCost:  50.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_updating_resource", output)
}

// TestGolden_DetailView_CreatingResource verifies the detail view for a new
// resource being created with projected cost and cost impact.
func TestGolden_DetailView_CreatingResource(t *testing.T) {
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:s3/bucket:Bucket::data-lake",
			Type:   "aws:s3/bucket:Bucket",
			Status: engine.StatusCreating,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 33.87,
				Currency:    "USD",
				Breakdown: map[string]float64{
					"storage":  25.00,
					"requests": 8.87,
				},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_creating_resource", output)
}

// TestGolden_DetailView_DeletingResource verifies the detail view for a resource
// being deleted with actual cost and cost impact (savings).
func TestGolden_DetailView_DeletingResource(t *testing.T) {
	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::legacy",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusDeleting,
			ActualCost: &engine.ActualCostData{
				MTDCost:  45.00,
				Currency: "USD",
				Period:   period,
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_deleting_resource", output)
}

// TestGolden_DetailView_ReplacingResource verifies the detail view for a resource
// being replaced, including property changes and cost impact.
func TestGolden_DetailView_ReplacingResource(t *testing.T) {
	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::api-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusReplacing,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "ami", OldValue: "ami-old12345", NewValue: "ami-new67890"},
				{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.xlarge"},
			},
			ActualCost: &engine.ActualCostData{
				MTDCost:  30.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 120.00,
				Currency:    "USD",
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_replacing_resource", output)
}

// TestGolden_DetailView_WithRecommendations verifies the detail view with
// active recommendations displayed.
func TestGolden_DetailView_WithRecommendations(t *testing.T) {
	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::overprovisioned",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{
				MTDCost:  200.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 400.00,
				Currency:    "USD",
			},
			Recommendations: []engine.Recommendation{
				{
					Type:             "Right-sizing",
					Description:      "Downsize from m5.2xlarge to m5.large based on CPU utilization (avg 12%)",
					EstimatedSavings: 150.00,
					Currency:         "USD",
					Status:           engine.RecommendationStatusActive,
				},
				{
					Type:             "Purchase Commitment",
					Description:      "Purchase 1-year reserved instance for 30% savings",
					EstimatedSavings: 120.00,
					Currency:         "USD",
					Status:           engine.RecommendationStatusActive,
				},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_with_recommendations", output)
}

// TestGolden_DetailView_WithError verifies the detail view when a resource
// has an error condition.
func TestGolden_DetailView_WithError(t *testing.T) {
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:rds/instance:Instance::database",
			Type:   "aws:rds/instance:Instance",
			Status: engine.StatusActive,
			Error: &engine.OverviewRowError{
				URN:       "urn:pulumi:prod::myapp::aws:rds/instance:Instance::database",
				ErrorType: engine.ErrorTypeNetwork,
				Message:   "connection refused: dial tcp 10.0.1.5:443: i/o timeout",
				Retryable: true,
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_with_error", output)
}
