package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// StatusIcon
// ---------------------------------------------------------------------------

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status ResourceStatus
		want   string
	}{
		{StatusActive, "\u2713"},
		{StatusCreating, "+"},
		{StatusUpdating, "~"},
		{StatusDeleting, "-"},
		{StatusReplacing, "\u21bb"},
		{ResourceStatus(99), "?"},
	}
	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.want, StatusIcon(tt.status))
		})
	}
}

// ---------------------------------------------------------------------------
// FormatOverviewCurrency
// ---------------------------------------------------------------------------

func TestFormatOverviewCurrency(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   string
	}{
		{"zero", 0, "$0.00"},
		{"small", 1.50, "$1.50"},
		{"hundreds", 123.45, "$123.45"},
		{"thousands", 1234.56, "$1,234.56"},
		{"large", 12345678.90, "$12,345,678.90"},
		{"negative", -50.00, "-$50.00"},
		{"negative_thousands", -1234.56, "-$1,234.56"},
		{"fractional_cents", 0.99, "$0.99"},
		{"no_cents", 100.00, "$100.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatOverviewCurrency(tt.amount))
		})
	}
}

// ---------------------------------------------------------------------------
// FormatOverviewDelta
// ---------------------------------------------------------------------------

func TestFormatOverviewDelta(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   string
	}{
		{"zero", 0, "$0.00"},
		{"positive", 123.45, "+$123.45"},
		{"negative", -50.00, "-$50.00"},
		{"large_positive", 10000.00, "+$10,000.00"},
		{"large_negative", -5678.90, "-$5,678.90"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatOverviewDelta(tt.amount))
		})
	}
}

// ---------------------------------------------------------------------------
// RenderOverviewAsTable
// ---------------------------------------------------------------------------

func TestRenderOverviewAsTable_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "test-stack",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 0,
	}

	err := RenderOverviewAsTable(&buf, nil, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "RESOURCE")
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "0 resources")
}

func TestRenderOverviewAsTable_SingleActiveResource(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:   "aws:ec2/instance:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  45.67,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
			},
		},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	// Check header
	assert.Contains(t, output, "RESOURCE")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "ACTUAL(MTD)")
	assert.Contains(t, output, "PROJECTED")
	assert.Contains(t, output, "DELTA")
	assert.Contains(t, output, "DRIFT%")
	assert.Contains(t, output, "RECS")

	// Check status icon for active
	assert.Contains(t, output, "\u2713 active")

	// Check currency formatting
	assert.Contains(t, output, "$45.67")
	assert.Contains(t, output, "$150.00")
}

func TestRenderOverviewAsTable_AllStatuses(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "dev",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		HasChanges:     true,
		TotalResources: 5,
		PendingChanges: 4,
	}

	rows := []OverviewRow{
		{URN: "urn:active", Type: "aws:ec2:Instance", Status: StatusActive},
		{URN: "urn:creating", Type: "aws:s3:Bucket", Status: StatusCreating},
		{URN: "urn:updating", Type: "aws:rds:Instance", Status: StatusUpdating},
		{URN: "urn:deleting", Type: "aws:lambda:Function", Status: StatusDeleting},
		{URN: "urn:replacing", Type: "aws:ec2:SecurityGroup", Status: StatusReplacing},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "\u2713 active")
	assert.Contains(t, output, "+ creating")
	assert.Contains(t, output, "~ updating")
	assert.Contains(t, output, "- deleting")
	assert.Contains(t, output, "\u21bb replacing")
	assert.Contains(t, output, "4 pending changes")
}

func TestRenderOverviewAsTable_ErrorRow(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "test",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:error-resource",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			Error: &OverviewRowError{
				URN:       "urn:error-resource",
				ErrorType: ErrorTypeNetwork,
				Message:   "connection refused",
			},
		},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "ERR")
}

func TestRenderOverviewAsTable_DriftWarning(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:drift-resource",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  200.00,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 100.00,
				Currency:    "USD",
			},
			CostDrift: &CostDriftData{
				ExtrapolatedMonthly: 200.00,
				Projected:           100.00,
				Delta:               100.00,
				PercentDrift:        100.0,
				IsWarning:           true,
			},
		},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "+100%")
	assert.Contains(t, output, "\u26a0") // warning icon
}

func TestRenderOverviewAsTable_Recommendations(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:rec-resource",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 200.00,
				Currency:    "USD",
			},
			Recommendations: []Recommendation{
				{Type: "Right-sizing", EstimatedSavings: 50.00, Currency: "USD"},
				{Type: "Terminate", EstimatedSavings: 25.00, Currency: "USD"},
			},
		},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "2") // recommendation count
	assert.Contains(t, output, "Potential Savings")
	assert.Contains(t, output, "$75.00")
}

func TestRenderOverviewAsTable_SummaryTotals(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 2,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:r1",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  100.00,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 300.00,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:r2",
			Type:   "aws:rds:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  200.00,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 500.00,
				Currency:    "USD",
			},
		},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "$300.00") // total actual
	assert.Contains(t, output, "$800.00") // total projected
	assert.Contains(t, output, "2 resources")
}

func TestRenderOverviewAsTable_LongURNTruncation(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "test",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	longURN := "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::this-is-a-very-long-resource-name-that-exceeds-column-width"
	rows := []OverviewRow{
		{URN: longURN, Type: "aws:ec2:Instance", Status: StatusActive},
	}

	err := RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	// The URN should be truncated with "..."
	assert.Contains(t, output, "...")
	// Full URN should NOT appear
	assert.NotContains(t, output, longURN)
}

// ---------------------------------------------------------------------------
// truncateResource
// ---------------------------------------------------------------------------

func TestTruncateResource(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "abc", 10, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"truncate", "abcdefghij", 7, "abcd..."},
		{"very_small_max", "abcdef", 3, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateResource(tt.input, tt.maxLen))
		})
	}
}

// ---------------------------------------------------------------------------
// formatWithCommas
// ---------------------------------------------------------------------------

func TestFormatWithCommas(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   string
	}{
		{"zero", 0, "0.00"},
		{"small", 1.23, "1.23"},
		{"hundreds", 999.99, "999.99"},
		{"thousands", 1234.56, "1,234.56"},
		{"millions", 1234567.89, "1,234,567.89"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatWithCommas(tt.amount))
		})
	}
}

// ---------------------------------------------------------------------------
// formatDriftColumn
// ---------------------------------------------------------------------------

func TestFormatDriftColumn(t *testing.T) {
	tests := []struct {
		name string
		row  OverviewRow
		want string
	}{
		{
			name: "no_drift",
			row:  OverviewRow{},
			want: "-",
		},
		{
			name: "positive_drift_warning",
			row: OverviewRow{
				CostDrift: &CostDriftData{
					PercentDrift: 15.0,
					IsWarning:    true,
				},
			},
			want: "+15% \u26a0",
		},
		{
			name: "negative_drift_warning",
			row: OverviewRow{
				CostDrift: &CostDriftData{
					PercentDrift: -20.0,
					IsWarning:    true,
				},
			},
			want: "-20% \u26a0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDriftColumn(tt.row)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// RenderOverviewAsJSON
// ---------------------------------------------------------------------------

func TestRenderOverviewAsJSON_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "test-stack",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 0,
	}

	err := RenderOverviewAsJSON(&buf, nil, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, "test-stack", output.Metadata.StackName)
	assert.Equal(t, 0, output.Metadata.TotalResources)
	assert.Len(t, output.Resources, 0)
	assert.Empty(t, output.Errors)
	assert.Equal(t, 0.0, output.Summary.TotalActualMTD)
	assert.Equal(t, 0.0, output.Summary.ProjectedMonthly)
	assert.Equal(t, "USD", output.Summary.Currency)
}

func TestRenderOverviewAsJSON_SingleResource(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		Region:         "us-east-1",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:        "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:       "aws:ec2/instance:Instance",
			ResourceID: "i-12345",
			Status:     StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  45.67,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
			},
		},
	}

	err := RenderOverviewAsJSON(&buf, rows, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	// Verify metadata
	assert.Equal(t, "prod", output.Metadata.StackName)
	assert.Equal(t, "us-east-1", output.Metadata.Region)
	assert.Equal(t, 1, output.Metadata.TotalResources)
	assert.False(t, output.Metadata.HasChanges)

	// Verify resources
	assert.Len(t, output.Resources, 1)
	res := output.Resources[0]
	assert.Equal(t, "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server", res.URN)
	assert.Equal(t, "active", res.Status.String())
	assert.NotNil(t, res.ActualCost)
	assert.Equal(t, 45.67, res.ActualCost.MTDCost)
	assert.NotNil(t, res.ProjectedCost)
	assert.Equal(t, 150.00, res.ProjectedCost.MonthlyCost)

	// Verify summary
	assert.Equal(t, 45.67, output.Summary.TotalActualMTD)
	assert.Equal(t, 150.00, output.Summary.ProjectedMonthly)
	assert.InDelta(t, 104.33, output.Summary.ProjectedDelta, 0.01)
	assert.Equal(t, "USD", output.Summary.Currency)
}

func TestRenderOverviewAsJSON_MetadataFields(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "dev-stack",
		Region:         "eu-west-1",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		HasChanges:     true,
		TotalResources: 5,
		PendingChanges: 2,
	}

	err := RenderOverviewAsJSON(&buf, nil, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, "dev-stack", output.Metadata.StackName)
	assert.Equal(t, "eu-west-1", output.Metadata.Region)
	assert.True(t, output.Metadata.HasChanges)
	assert.Equal(t, 5, output.Metadata.TotalResources)
	assert.Equal(t, 2, output.Metadata.PendingChanges)
	assert.False(t, output.Metadata.GeneratedAt.IsZero())
}

func TestRenderOverviewAsJSON_SummaryTotals(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 2,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:r1",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  100.00,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 300.00,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:r2",
			Type:   "aws:rds:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  200.00,
				Currency: "USD",
				Period:   stackCtx.TimeWindow,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 500.00,
				Currency:    "USD",
			},
		},
	}

	err := RenderOverviewAsJSON(&buf, rows, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, 300.00, output.Summary.TotalActualMTD)
	assert.Equal(t, 800.00, output.Summary.ProjectedMonthly)
	assert.Equal(t, 500.00, output.Summary.ProjectedDelta)
}

func TestRenderOverviewAsJSON_CurrencyConsistency(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:r1",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  100.00,
				Currency: "EUR",
				Period:   stackCtx.TimeWindow,
			},
		},
	}

	err := RenderOverviewAsJSON(&buf, rows, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, "EUR", output.Summary.Currency)
}

func TestRenderOverviewAsJSON_ErrorsArray(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "test",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 2,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:r1",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			Error: &OverviewRowError{
				URN:       "urn:r1",
				ErrorType: ErrorTypeNetwork,
				Message:   "connection refused",
				Retryable: true,
			},
		},
		{
			URN:    "urn:r2",
			Type:   "aws:rds:Instance",
			Status: StatusActive,
			Error: &OverviewRowError{
				URN:       "urn:r2",
				ErrorType: ErrorTypeAuth,
				Message:   "authentication failed",
				Retryable: false,
			},
		},
	}

	err := RenderOverviewAsJSON(&buf, rows, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Len(t, output.Errors, 2)
	assert.Equal(t, "urn:r1", output.Errors[0].URN)
	assert.Equal(t, "network", output.Errors[0].ErrorType.String())
	assert.True(t, output.Errors[0].Retryable)
	assert.Equal(t, "urn:r2", output.Errors[1].URN)
	assert.Equal(t, "auth", output.Errors[1].ErrorType.String())
	assert.False(t, output.Errors[1].Retryable)
}

func TestRenderOverviewAsJSON_Recommendations(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	stackCtx := StackContext{
		StackName:      "prod",
		TimeWindow:     DateRange{Start: now.Add(-24 * time.Hour), End: now},
		TotalResources: 1,
	}

	rows := []OverviewRow{
		{
			URN:    "urn:rec-resource",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 200.00,
				Currency:    "USD",
			},
			Recommendations: []Recommendation{
				{Type: "Right-sizing", EstimatedSavings: 50.00, Currency: "USD"},
				{Type: "Terminate", EstimatedSavings: 25.00, Currency: "USD"},
			},
		},
	}

	err := RenderOverviewAsJSON(&buf, rows, stackCtx)
	require.NoError(t, err)

	var output OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, 75.00, output.Summary.PotentialSavings)
	assert.Len(t, output.Resources[0].Recommendations, 2)
}

// ---------------------------------------------------------------------------
// RenderOverviewAsNDJSON
// ---------------------------------------------------------------------------

func TestRenderOverviewAsNDJSON_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	err := RenderOverviewAsNDJSON(&buf, nil)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestRenderOverviewAsNDJSON_SingleRow(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	period := DateRange{Start: now.Add(-24 * time.Hour), End: now}

	rows := []OverviewRow{
		{
			URN:        "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web-1",
			Type:       "aws:ec2/instance:Instance",
			ResourceID: "i-12345",
			Status:     StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  42.50,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 100.00,
				Currency:    "USD",
			},
		},
	}

	err := RenderOverviewAsNDJSON(&buf, rows)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 1)

	var row OverviewRow
	err = json.Unmarshal([]byte(lines[0]), &row)
	require.NoError(t, err)

	assert.Equal(t, "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web-1", row.URN)
	assert.Equal(t, "active", row.Status.String())
	assert.NotNil(t, row.ActualCost)
	assert.Equal(t, 42.50, row.ActualCost.MTDCost)
}

func TestRenderOverviewAsNDJSON_MultipleRows(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	period := DateRange{Start: now.Add(-24 * time.Hour), End: now}

	rows := []OverviewRow{
		{
			URN:    "urn:r1",
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
			ActualCost: &ActualCostData{
				MTDCost:  10.00,
				Currency: "USD",
				Period:   period,
			},
		},
		{
			URN:    "urn:r2",
			Type:   "aws:s3:Bucket",
			Status: StatusCreating,
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: 5.00,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:r3",
			Type:   "aws:rds:Instance",
			Status: StatusUpdating,
		},
	}

	err := RenderOverviewAsNDJSON(&buf, rows)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 3)

	// Verify each line is valid JSON
	for i, line := range lines {
		var row OverviewRow
		err = json.Unmarshal([]byte(line), &row)
		require.NoError(t, err, "line %d should be valid JSON", i)
	}

	// Verify first row
	var row1 OverviewRow
	err = json.Unmarshal([]byte(lines[0]), &row1)
	require.NoError(t, err)
	assert.Equal(t, "urn:r1", row1.URN)
	assert.Equal(t, "active", row1.Status.String())

	// Verify second row
	var row2 OverviewRow
	err = json.Unmarshal([]byte(lines[1]), &row2)
	require.NoError(t, err)
	assert.Equal(t, "urn:r2", row2.URN)
	assert.Equal(t, "creating", row2.Status.String())

	// Verify third row
	var row3 OverviewRow
	err = json.Unmarshal([]byte(lines[2]), &row3)
	require.NoError(t, err)
	assert.Equal(t, "urn:r3", row3.URN)
	assert.Equal(t, "updating", row3.Status.String())
}

func TestRenderOverviewAsNDJSON_AllStatuses(t *testing.T) {
	var buf bytes.Buffer
	rows := []OverviewRow{
		{URN: "urn:active", Type: "aws:ec2:Instance", Status: StatusActive},
		{URN: "urn:creating", Type: "aws:s3:Bucket", Status: StatusCreating},
		{URN: "urn:updating", Type: "aws:rds:Instance", Status: StatusUpdating},
		{URN: "urn:deleting", Type: "aws:lambda:Function", Status: StatusDeleting},
		{URN: "urn:replacing", Type: "aws:ec2:SecurityGroup", Status: StatusReplacing},
	}

	err := RenderOverviewAsNDJSON(&buf, rows)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 5)

	expectedStatuses := []string{"active", "creating", "updating", "deleting", "replacing"}
	for i, line := range lines {
		var row OverviewRow
		err = json.Unmarshal([]byte(line), &row)
		require.NoError(t, err)
		assert.Equal(t, expectedStatuses[i], row.Status.String())
	}
}

// ---------------------------------------------------------------------------
// ResourceStatus JSON Marshaling
// ---------------------------------------------------------------------------

func TestResourceStatus_MarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		status ResourceStatus
		want   string
	}{
		{"active", StatusActive, `"active"`},
		{"creating", StatusCreating, `"creating"`},
		{"updating", StatusUpdating, `"updating"`},
		{"deleting", StatusDeleting, `"deleting"`},
		{"replacing", StatusReplacing, `"replacing"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(data))
		})
	}
}

func TestResourceStatus_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ResourceStatus
		wantErr bool
	}{
		{"active", `"active"`, StatusActive, false},
		{"creating", `"creating"`, StatusCreating, false},
		{"updating", `"updating"`, StatusUpdating, false},
		{"deleting", `"deleting"`, StatusDeleting, false},
		{"replacing", `"replacing"`, StatusReplacing, false},
		{"invalid", `"invalid"`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status ResourceStatus
			err := json.Unmarshal([]byte(tt.input), &status)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ErrorType JSON Marshaling
// ---------------------------------------------------------------------------

func TestErrorType_MarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		want      string
	}{
		{"auth", ErrorTypeAuth, `"auth"`},
		{"network", ErrorTypeNetwork, `"network"`},
		{"rate_limit", ErrorTypeRateLimit, `"rate_limit"`},
		{"unknown", ErrorTypeUnknown, `"unknown"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.errorType)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(data))
		})
	}
}
