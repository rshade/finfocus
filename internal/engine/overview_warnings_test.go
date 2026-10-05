package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveWarnings(t *testing.T) {
	t.Parallel()

	drift := &CostDriftData{PercentDrift: 18, IsWarning: true}
	rowErr := &OverviewRowError{URN: "urn:a", ErrorType: ErrorTypeNetwork, Message: "connection reset"}

	tests := []struct {
		name string
		row  *OverviewRow
		want []OverviewWarning
	}{
		{name: "nil row"},
		{
			name: "no conditions",
			row:  &OverviewRow{URN: "urn:quiet", Type: "aws:s3/bucket:Bucket", Status: StatusActive},
		},
		{
			name: "drift below the warning flag",
			row: &OverviewRow{
				URN: "urn:small", Type: "aws:s3/bucket:Bucket", Status: StatusActive,
				CostDrift: &CostDriftData{PercentDrift: 4, IsWarning: false},
			},
		},
		{
			name: "drift warning",
			row: &OverviewRow{
				URN: "urn:drift", Type: "aws:ec2/instance:Instance", Status: StatusActive,
				CostDrift: drift,
			},
			want: []OverviewWarning{WarnDrift},
		},
		{
			name: "fetch error",
			row: &OverviewRow{
				URN: "urn:err", Type: "aws:s3/bucket:Bucket", Status: StatusActive, Error: rowErr,
			},
			want: []OverviewWarning{WarnError},
		},
		{
			name: "creating resource",
			row:  &OverviewRow{URN: "urn:new", Type: "aws:s3/bucket:Bucket", Status: StatusCreating},
			want: []OverviewWarning{WarnNew},
		},
		{
			name: "drift and error",
			row: &OverviewRow{
				URN: "urn:both", Type: "aws:ec2/instance:Instance", Status: StatusUpdating,
				CostDrift: drift, Error: rowErr,
			},
			want: []OverviewWarning{WarnDrift, WarnError},
		},
		{
			name: "all derived warnings",
			row: &OverviewRow{
				URN: "urn:all", Type: "aws:ec2/instance:Instance", Status: StatusCreating,
				CostDrift: drift, Error: rowErr,
			},
			want: []OverviewWarning{WarnDrift, WarnError, WarnNew},
		},
		{
			name: "clears reserved warnings",
			row: &OverviewRow{
				URN: "urn:reserved", Type: "aws:s3/bucket:Bucket", Status: StatusActive,
				Warnings: []OverviewWarning{WarnEstimate, WarnStale},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			deriveWarnings(tt.row)
			if tt.row == nil {
				return
			}
			assert.Equal(t, tt.want, tt.row.Warnings)
			assert.NotContains(t, tt.row.Warnings, WarnEstimate)
			assert.NotContains(t, tt.row.Warnings, WarnStale)
		})
	}
}

func TestFormatOverviewWarnings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "-", FormatOverviewWarnings(nil))
	assert.Equal(t, "-", FormatOverviewWarnings([]OverviewWarning{}))
	assert.Equal(t, "drift", FormatOverviewWarnings([]OverviewWarning{WarnDrift}))
	assert.Equal(t, "drift,error,new", FormatOverviewWarnings([]OverviewWarning{WarnDrift, WarnError, WarnNew}))
}

func TestRenderOverviewAsTable_WarnColumn(t *testing.T) {
	t.Parallel()

	up := 6.2
	down := -8.4
	rows := []OverviewRow{
		{
			URN: "my-instance", Type: "aws:ec2/instance:Instance", Status: StatusActive,
			ActualCost:    &ActualCostData{MTDCost: 12.4, Currency: "USD"},
			ProjectedCost: &ProjectedCostData{MonthlyCost: 15, Currency: "USD"},
			ComputedDelta: &up,
			CostDrift:     &CostDriftData{PercentDrift: 18, IsWarning: true, Delta: up},
			Warnings:      []OverviewWarning{WarnDrift},
			Recommendations: []Recommendation{
				{ID: "r1"},
				{ID: "r2"},
			},
		},
		{
			URN: "my-bucket", Type: "aws:s3/bucket:Bucket", Status: StatusActive,
			ActualCost:    &ActualCostData{MTDCost: 0.83, Currency: "USD"},
			ProjectedCost: &ProjectedCostData{MonthlyCost: 1, Currency: "USD"},
		},
		{
			URN: "my-db", Type: "aws:rds/instance:Instance", Status: StatusActive,
			ActualCost:    &ActualCostData{MTDCost: 48.20, Currency: "USD"},
			ProjectedCost: &ProjectedCostData{MonthlyCost: 50, Currency: "USD"},
			ComputedDelta: &down,
			CostDrift:     &CostDriftData{PercentDrift: -15, IsWarning: true, Delta: down},
			Warnings:      []OverviewWarning{WarnDrift},
			Recommendations: []Recommendation{
				{ID: "r3"},
			},
		},
	}
	var buf bytes.Buffer
	err := RenderOverviewAsTable(&buf, ComputeOverviewResult(rows, testDayOfMonth),
		StackContext{StackName: "prod", TotalResources: len(rows)})
	require.NoError(t, err)
	output := buf.String()
	t.Logf("plain overview table:\n%s", output)
	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "drift")
	assert.Contains(t, output, "my-bucket")
	assert.Contains(t, output, "$61.43 USD")
	assert.Contains(t, output, "-$2.20 USD")
	assert.NotContains(t, output, "estimate")
	assert.NotContains(t, output, "stale")
	assert.NotContains(t, output, "Potential Savings")

	var errBuf bytes.Buffer
	errRow := []OverviewRow{{
		URN: "my-instance", Type: "aws:ec2/instance:Instance", Status: StatusActive,
		Error:    &OverviewRowError{URN: "my-instance", ErrorType: ErrorTypeNetwork, Message: "connection reset"},
		Warnings: []OverviewWarning{WarnError},
	}}
	require.NoError(t, RenderOverviewAsTable(&errBuf, ComputeOverviewResult(errRow, testDayOfMonth),
		StackContext{StackName: "prod", TotalResources: 1}))
	errOut := errBuf.String()
	assert.Contains(t, errOut, "ERR")
	assert.Contains(t, errOut, "error")
}

func TestEnrichOverviewRow_DerivesWarnings(t *testing.T) {
	t.Parallel()

	dateRange := DateRange{
		Start: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
	}

	t.Run("drift from fetched costs", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{
			URN: "urn:drift", Type: "aws:ec2:Instance", Status: StatusActive,
			Warnings: []OverviewWarning{WarnEstimate, WarnStale},
		}
		eng := &mockEnricher{
			actualResult:    &CostResultWithErrors{Results: []CostResult{{TotalCost: 50, Currency: "USD"}}},
			projectedResult: &CostResultWithErrors{Results: []CostResult{{Monthly: 200, Currency: "USD"}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.CostDrift)
		assert.True(t, row.CostDrift.IsWarning)
		assert.Equal(t, []OverviewWarning{WarnDrift}, row.Warnings)
	})

	t.Run("fetch error", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:err", Type: "aws:ec2:Instance", Status: StatusActive}
		eng := &mockEnricher{actualErr: errors.New("connection reset")}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.Error)
		assert.Equal(t, []OverviewWarning{WarnError}, row.Warnings)
	})

	t.Run("plugin error result", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:plugin-err", Type: "aws:ec2:Instance", Status: StatusActive}
		eng := &mockEnricher{
			actualResult: &CostResultWithErrors{Results: []CostResult{{
				Error: &StructuredError{Code: ErrCodePluginError, Message: "connection refused"},
			}}},
			projectedResult: &CostResultWithErrors{Results: []CostResult{{Monthly: 10, Currency: "USD"}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.Error)
		assert.Equal(t, ErrorTypeNetwork, row.Error.ErrorType)
		assert.Contains(t, row.Error.Message, "connection refused")
		assert.Equal(t, []OverviewWarning{WarnError}, row.Warnings)
		assert.Nil(t, row.ActualCost)
	})

	t.Run("timeout error result", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:timeout", Type: "aws:ec2:Instance", Status: StatusActive}
		eng := &mockEnricher{
			projectedResult: &CostResultWithErrors{Results: []CostResult{{
				Error: &StructuredError{Code: ErrCodeTimeoutError, Message: "context deadline exceeded"},
			}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.Error)
		assert.Equal(t, ErrorTypeNetwork, row.Error.ErrorType)
		assert.True(t, row.Error.Retryable)
		assert.Equal(t, []OverviewWarning{WarnError}, row.Warnings)
	})

	t.Run("validation note result", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:validation", Type: "aws:ec2:Instance", Status: StatusActive}
		eng := &mockEnricher{
			projectedResult: &CostResultWithErrors{Results: []CostResult{{Notes: "VALIDATION: region is required"}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.Error)
		assert.Contains(t, row.Error.Message, "region is required")
		assert.Equal(t, []OverviewWarning{WarnError}, row.Warnings)
	})

	t.Run("a resource with no cost data is not an error", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:no-data", Type: "aws:iam:Role", Status: StatusActive}
		eng := &mockEnricher{
			projectedResult: &CostResultWithErrors{Results: []CostResult{{
				Error: &StructuredError{Code: ErrCodeNoCostData, Message: "No pricing information available"},
			}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		assert.Nil(t, row.Error)
		assert.Empty(t, row.Warnings)
	})

	t.Run("a plugin error on one side keeps the other side's cost", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:half", Type: "aws:ec2:Instance", Status: StatusActive}
		eng := &mockEnricher{
			actualResult: &CostResultWithErrors{Results: []CostResult{{TotalCost: 50, Currency: "USD"}}},
			projectedResult: &CostResultWithErrors{Results: []CostResult{{
				Error: &StructuredError{Code: ErrCodePluginError, Message: "plugin down"},
			}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.ActualCost)
		assert.Nil(t, row.ProjectedCost)
		assert.Equal(t, []OverviewWarning{WarnError}, row.Warnings)
	})

	t.Run("creating resource", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:new", Type: "aws:s3:Bucket", Status: StatusCreating}
		eng := &mockEnricher{
			projectedResult: &CostResultWithErrors{Results: []CostResult{{Monthly: 10, Currency: "USD"}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		assert.Nil(t, row.ActualCost)
		assert.Equal(t, []OverviewWarning{WarnNew}, row.Warnings)
	})

	t.Run("creating resource with a fetch error", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{URN: "urn:new-err", Type: "aws:s3:Bucket", Status: StatusCreating}
		eng := &mockEnricher{projectedErr: errors.New("connection reset")}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		require.NotNil(t, row.Error)
		assert.Equal(t, []OverviewWarning{WarnError, WarnNew}, row.Warnings)
	})

	t.Run("quiet costs stay empty", func(t *testing.T) {
		t.Parallel()
		row := OverviewRow{
			URN: "urn:quiet", Type: "aws:ec2:Instance", Status: StatusActive,
			Warnings: []OverviewWarning{WarnEstimate},
		}
		// 98.63 is the month-to-date amount that tracks a $200 projection
		// across 1–16 June, so drift stays under the warning threshold.
		eng := &mockEnricher{
			actualResult:    &CostResultWithErrors{Results: []CostResult{{TotalCost: 98.63, Currency: "USD"}}},
			projectedResult: &CostResultWithErrors{Results: []CostResult{{Monthly: 200, Currency: "USD"}}},
		}
		enrichOverviewRow(context.Background(), &row, eng, dateRange)
		assert.Nil(t, row.CostDrift)
		assert.Nil(t, row.Error)
		assert.Nil(t, row.Warnings)
	})
}

func TestRenderOverviewAsJSON_IncludesWarnings(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{{
		URN: "urn:both", Type: "aws:ec2/instance:Instance", Status: StatusActive,
		ActualCost:    &ActualCostData{MTDCost: 1, Currency: "USD"},
		ProjectedCost: &ProjectedCostData{MonthlyCost: 1, Currency: "USD"},
		Warnings:      []OverviewWarning{WarnDrift, WarnError},
	}}
	var buf bytes.Buffer
	err := RenderOverviewAsJSON(context.Background(), &buf, ComputeOverviewResult(rows, testDayOfMonth),
		StackContext{StackName: "prod"}, nil)
	require.NoError(t, err)

	var parsed struct {
		Resources []struct {
			Warnings []string `json:"warnings"`
		} `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Resources, 1)
	assert.Equal(t, []string{"drift", "error"}, parsed.Resources[0].Warnings)
}

func TestApplyChangesToRows_RederivesWarnings(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{
		{
			URN: "urn:a", Status: StatusActive,
			Error:    &OverviewRowError{URN: "urn:a", Message: "plugin down"},
			Warnings: []OverviewWarning{WarnError},
		},
		{URN: "urn:b", Status: StatusCreating, Warnings: []OverviewWarning{WarnNew}},
		{URN: "urn:c", Status: StatusActive},
	}

	ApplyChangesToRows(rows, map[string]ResourceStatus{
		"urn:a": StatusCreating,
		"urn:b": StatusActive,
	})

	assert.Equal(t, []OverviewWarning{WarnError, WarnNew}, rows[0].Warnings)
	assert.Empty(t, rows[1].Warnings)
	assert.Empty(t, rows[2].Warnings)
}
