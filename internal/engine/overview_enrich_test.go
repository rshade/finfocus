package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// classifyError
// ---------------------------------------------------------------------------

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  ErrorType
		wantRetry bool
	}{
		{
			name:      "auth_error",
			err:       errors.New("authentication failed: invalid token"),
			wantType:  ErrorTypeAuth,
			wantRetry: false,
		},
		{
			name:      "permission_error",
			err:       errors.New("permission denied for resource"),
			wantType:  ErrorTypeAuth,
			wantRetry: false,
		},
		{
			name:      "network_error",
			err:       errors.New("connection refused"),
			wantType:  ErrorTypeNetwork,
			wantRetry: true,
		},
		{
			name:      "timeout_error",
			err:       errors.New("request timeout after 10s"),
			wantType:  ErrorTypeNetwork,
			wantRetry: true,
		},
		{
			name:      "rate_limit_error",
			err:       errors.New("rate limit exceeded"),
			wantType:  ErrorTypeRateLimit,
			wantRetry: true,
		},
		{
			name:      "throttle_error",
			err:       errors.New("throttle: too many requests"),
			wantType:  ErrorTypeRateLimit,
			wantRetry: true,
		},
		{
			name:      "context_canceled",
			err:       context.Canceled,
			wantType:  ErrorTypeNetwork,
			wantRetry: true,
		},
		{
			name:      "context_deadline_exceeded",
			err:       context.DeadlineExceeded,
			wantType:  ErrorTypeNetwork,
			wantRetry: true,
		},
		{
			name:      "wrapped_context_canceled",
			err:       fmt.Errorf("plugin call failed: %w", context.Canceled),
			wantType:  ErrorTypeNetwork,
			wantRetry: true,
		},
		{
			name:      "wrapped_context_deadline_exceeded",
			err:       fmt.Errorf("plugin call failed: %w", context.DeadlineExceeded),
			wantType:  ErrorTypeNetwork,
			wantRetry: true,
		},
		{
			name:      "unknown_error",
			err:       errors.New("something unexpected happened"),
			wantType:  ErrorTypeUnknown,
			wantRetry: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError("urn:test", tt.err)
			require.NotNil(t, result)
			assert.Equal(t, "urn:test", result.URN)
			assert.Equal(t, tt.wantType, result.ErrorType)
			assert.Equal(t, tt.wantRetry, result.Retryable)
			assert.Contains(t, result.Message, tt.err.Error())
		})
	}
}

func TestClassifyError_LongMessage(t *testing.T) {
	longMsg := make([]byte, maxMessageLen+100)
	for i := range longMsg {
		longMsg[i] = 'x'
	}
	result := classifyError("urn:test", errors.New(string(longMsg)))
	require.NotNil(t, result)
	assert.Len(t, result.Message, maxMessageLen)
}

// ---------------------------------------------------------------------------
// daysInCurrentMonth
// ---------------------------------------------------------------------------

func TestDaysInCurrentMonth(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		want int
	}{
		{"january", time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), 31},
		{"february_non_leap", time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC), 28},
		{"february_leap", time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC), 29},
		{"april", time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC), 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, daysInCurrentMonth(tt.time))
		})
	}
}

// ---------------------------------------------------------------------------
// EnrichOverviewRow (unit tests with real Engine - no plugins)
// ---------------------------------------------------------------------------

func TestEnrichOverviewRow_NoPlugins(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil) // No plugins, no spec loader

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:pulumi:prod::app::aws:ec2:Instance::web",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	// With no plugins and no spec loader, the engine returns empty result sets
	// (len(result.Results) == 0), so the enrichment functions never set
	// ActualCost or ProjectedCost — both remain nil. This is the expected
	// behaviour; asserting nil here makes the intent explicit.
	assert.Nil(t, row.ActualCost, "ActualCost must be nil with no plugins")
	assert.Nil(t, row.ProjectedCost, "ProjectedCost must be nil with no plugins")
	assert.Empty(t, row.Recommendations, "Recommendations should be empty with no plugins")
}

func TestEnrichOverviewRow_PropertiesPassedToEngine(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	props := map[string]interface{}{
		"instanceType":     "t3.micro",
		"availabilityZone": "us-east-1a",
	}

	row := OverviewRow{
		URN:        "urn:pulumi:prod::app::aws:ec2:Instance::web",
		Type:       "aws:ec2:Instance",
		Status:     StatusActive,
		Properties: props,
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	// Properties should still be present on the row after enrichment
	assert.Equal(t, props, row.Properties)
	assert.Equal(t, "t3.micro", row.Properties["instanceType"])
	assert.Equal(t, "us-east-1a", row.Properties["availabilityZone"])
}

func TestEnrichOverviewRow_CreatingStatus_SkipsActualCost(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:pulumi:prod::app::aws:s3:Bucket::new-bucket",
		Type:   "aws:s3:Bucket",
		Status: StatusCreating,
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	// Creating resources should not have actual cost
	assert.Nil(t, row.ActualCost)
	// Cost drift requires both actual and projected; skipping actual means no drift.
	assert.Nil(t, row.CostDrift, "no drift without actual cost")
}

// ---------------------------------------------------------------------------
// EnrichOverviewRows (concurrent enrichment)
// ---------------------------------------------------------------------------

func TestEnrichOverviewRows_EmptySlice(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	progressChan := make(chan OverviewRowUpdate, 10)
	result := EnrichOverviewRows(ctx, nil, eng, dateRange, progressChan)
	assert.Empty(t, result)
}

func TestEnrichOverviewRows_ProgressUpdates(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	rows := []OverviewRow{
		{URN: "urn:r1", Type: "aws:ec2:Instance", Status: StatusActive},
		{URN: "urn:r2", Type: "aws:s3:Bucket", Status: StatusCreating},
		{URN: "urn:r3", Type: "aws:rds:Instance", Status: StatusUpdating},
	}

	progressChan := make(chan OverviewRowUpdate, len(rows))
	result := EnrichOverviewRows(ctx, rows, eng, dateRange, progressChan)

	// Verify all rows were processed
	assert.Len(t, result, 3)

	// Verify progress updates were sent
	updates := make(map[int]bool)
	for update := range progressChan {
		updates[update.Index] = true
	}
	assert.Len(t, updates, 3)
	assert.True(t, updates[0])
	assert.True(t, updates[1])
	assert.True(t, updates[2])
}

func TestEnrichOverviewRows_NilProgressChan(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	rows := []OverviewRow{
		{URN: "urn:r1", Type: "aws:ec2:Instance", Status: StatusActive},
	}

	// Should not panic with nil progressChan
	result := EnrichOverviewRows(ctx, rows, eng, dateRange, nil)
	assert.Len(t, result, 1)
}

func TestEnrichOverviewRows_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	rows := make([]OverviewRow, 20)
	for i := range rows {
		rows[i] = OverviewRow{
			URN:    fmt.Sprintf("urn:resource-%d", i),
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
		}
	}

	progressChan := make(chan OverviewRowUpdate, len(rows))
	start := time.Now()
	result := EnrichOverviewRows(ctx, rows, eng, dateRange, progressChan)
	elapsed := time.Since(start)

	// With cancelled context, should complete quickly
	assert.NotNil(t, result)
	assert.Less(t, elapsed, 200*time.Millisecond, "cancelled context should not block")

	// Count progress updates - should be fewer than total rows or equal
	// (some goroutines may still send updates before seeing cancellation)
	updateCount := len(progressChan)
	assert.LessOrEqual(t, updateCount, len(rows))
}

func TestEnrichOverviewRows_ConcurrencyLimit(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	// Create more rows than the concurrency limit
	rows := make([]OverviewRow, overviewConcurrencyLimit+5)
	for i := range rows {
		rows[i] = OverviewRow{
			URN:    fmt.Sprintf("urn:r%d", i),
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
		}
	}

	progressChan := make(chan OverviewRowUpdate, len(rows))
	result := EnrichOverviewRows(ctx, rows, eng, dateRange, progressChan)

	// All rows should be processed despite exceeding concurrency limit
	assert.Len(t, result, overviewConcurrencyLimit+5)
}

func TestEnrichOverviewRows_WorkerPoolBound(t *testing.T) {
	// Verify that the fixed worker pool pattern processes many rows
	// without creating unbounded goroutines. A cancelled context ensures
	// workers exit immediately without calling EnrichOverviewRow.
	const rowCount = 50
	rows := make([]OverviewRow, rowCount)
	for i := range rows {
		rows[i] = OverviewRow{
			URN:    fmt.Sprintf("urn:test:resource:%d", i),
			Type:   "test:resource:Type",
			Status: StatusActive,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately so workers skip processing

	progressChan := make(chan OverviewRowUpdate, rowCount)

	// This should not hang or create unbounded goroutines
	result := EnrichOverviewRows(ctx, rows, New(nil, nil), DateRange{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}, progressChan)

	assert.Len(t, result, rowCount)
}

func TestEnrichOverviewRows_ClosesProgressChan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	rows := []OverviewRow{
		{URN: "urn:test:1", Type: "test:Type", Status: StatusActive},
	}
	progressChan := make(chan OverviewRowUpdate, 10)

	EnrichOverviewRows(ctx, rows, New(nil, nil), DateRange{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}, progressChan)

	// progressChan should be closed after EnrichOverviewRows returns.
	// Receiving from a closed channel returns the zero value and false.
	_, open := <-progressChan
	assert.False(t, open, "progress channel should be closed")
}

// ---------------------------------------------------------------------------
// Parallel enrichment (sub-call concurrency within EnrichOverviewRow)
// ---------------------------------------------------------------------------

func TestEnrichOverviewRow_ParallelConsistencyNoPlugins(t *testing.T) {
	// Verify that running all three enrichment sub-calls concurrently does not
	// introduce data races and leaves the row in a consistent state. With no
	// plugins or spec loader the cost fields remain nil; the assertions here
	// confirm safe completion and field preservation, not cost population.
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:pulumi:prod::app::aws:ec2:Instance::parallel-test",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
		Properties: map[string]interface{}{
			"instanceType":     "t3.micro",
			"availabilityZone": "us-east-1a",
		},
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	// With no plugins and no spec loader, the engine returns empty results
	// without errors. The enrichment functions populate no cost data.
	// The key assertion is that the function completes without panics or
	// races and the row is in a consistent state.
	assert.Nil(t, row.Error, "no error expected with no plugins")
	assert.Equal(t, "aws:ec2:Instance", row.Type)
	assert.Equal(t, "urn:pulumi:prod::app::aws:ec2:Instance::parallel-test", row.URN)
	assert.Equal(t, StatusActive, row.Status)
	// Properties must be preserved after concurrent enrichment
	assert.Equal(t, "t3.micro", row.Properties["instanceType"])
}

func TestEnrichOverviewRow_ErrorMerge_NoErrors(t *testing.T) {
	// Verify that when neither enrichment call returns an error,
	// row.Error remains nil after the merge.
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:pulumi:prod::app::aws:ec2:Instance::no-error",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	assert.Nil(t, row.Error, "row.Error must be nil when no enrichment errors occur")
}

func TestEnrichOverviewRow_RaceDetector(t *testing.T) {
	// Stress-test concurrent enrichment with the race detector by processing
	// many rows through EnrichOverviewRows (worker pool), where each row
	// internally runs 3 concurrent goroutines. This exercises both the
	// row-level parallelism (worker pool) and sub-call parallelism
	// (goroutines within EnrichOverviewRow).
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	const rowCount = 30
	rows := make([]OverviewRow, rowCount)
	for i := range rows {
		status := StatusActive
		if i%3 == 0 {
			status = StatusCreating
		}
		rows[i] = OverviewRow{
			URN:    fmt.Sprintf("urn:pulumi:prod::app::aws:ec2:Instance::race-%d", i),
			Type:   "aws:ec2:Instance",
			Status: status,
		}
	}

	progressChan := make(chan OverviewRowUpdate, rowCount)
	result := EnrichOverviewRows(ctx, rows, eng, dateRange, progressChan)

	// All rows must be processed
	assert.Len(t, result, rowCount)

	// Verify progress channel closed and all updates received
	updates := make(map[int]bool)
	for update := range progressChan {
		updates[update.Index] = true
	}
	assert.Len(t, updates, rowCount)

	// Verify creating resources have no actual cost
	for i, row := range result {
		if i%3 == 0 {
			assert.Nil(t, row.ActualCost, "creating resource %d must skip actual cost", i)
		}
		assert.Nil(t, row.Error, "no errors expected with no plugins for row %d", i)
	}
}

func TestEnrichOverviewRow_CostDrift_AfterParallelCompletion(t *testing.T) {
	// Verify that cost drift is calculated correctly after both actual
	// and projected costs complete in parallel. With no plugins, both
	// enrichment calls return empty results (no cost data), so drift
	// should not be calculated.
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:pulumi:prod::app::aws:ec2:Instance::drift-test",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	// With no plugins returning cost data, drift cannot be calculated
	// (requires both ActualCost != nil && ProjectedCost != nil)
	assert.Nil(t, row.CostDrift, "drift requires both actual and projected cost data")
}

func TestEnrichOverviewRow_CostDrift_SkippedWhenMissingCost(t *testing.T) {
	// Verify that cost drift is nil when actual cost is missing
	// (StatusCreating skips actual cost enrichment).
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:pulumi:prod::app::aws:s3:Bucket::drift-skip",
		Type:   "aws:s3:Bucket",
		Status: StatusCreating,
	}

	EnrichOverviewRow(ctx, &row, eng, dateRange)

	assert.Nil(t, row.ActualCost, "creating resource must not have actual cost")
	assert.Nil(t, row.CostDrift, "drift must be nil when actual cost is missing")
}

// ---------------------------------------------------------------------------
// enrichActualCost / enrichProjectedCost return value tests
// ---------------------------------------------------------------------------

func TestEnrichActualCost_ReturnsNilOnSuccess(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	now := time.Now()
	dateRange := DateRange{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	}

	row := OverviewRow{
		URN:    "urn:test:actual-return",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
	}

	resource := ResourceDescriptor{
		Type:     row.Type,
		ID:       row.URN,
		Provider: extractProviderFromType(row.Type),
	}

	result := enrichActualCost(ctx, &row, eng, resource, dateRange)
	assert.Nil(t, result, "enrichActualCost must return nil when no error occurs")
}

func TestEnrichProjectedCost_ReturnsNilOnSuccess(t *testing.T) {
	ctx := context.Background()
	eng := New(nil, nil)

	row := OverviewRow{
		URN:    "urn:test:projected-return",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
	}

	resource := ResourceDescriptor{
		Type:     row.Type,
		ID:       row.URN,
		Provider: extractProviderFromType(row.Type),
	}

	result := enrichProjectedCost(ctx, &row, eng, resource)
	assert.Nil(t, result, "enrichProjectedCost must return nil when no error occurs")
}

// ---------------------------------------------------------------------------
// mockEnricher — lightweight test double for overviewEnricher
// ---------------------------------------------------------------------------

// mockEnricher implements overviewEnricher with configurable return values.
type mockEnricher struct {
	actualResult    *CostResultWithErrors
	actualErr       error
	projectedResult *CostResultWithErrors
	projectedErr    error
	recommendResult *RecommendationsResult
	recommendErr    error
}

func (m *mockEnricher) GetActualCostWithOptionsAndErrors(
	_ context.Context,
	_ ActualCostRequest,
) (*CostResultWithErrors, error) {
	return m.actualResult, m.actualErr
}

func (m *mockEnricher) GetProjectedCostWithErrors(
	_ context.Context,
	_ []ResourceDescriptor,
) (*CostResultWithErrors, error) {
	return m.projectedResult, m.projectedErr
}

func (m *mockEnricher) GetRecommendationsForResources(
	_ context.Context,
	_ []ResourceDescriptor,
) (*RecommendationsResult, error) {
	return m.recommendResult, m.recommendErr
}

// ---------------------------------------------------------------------------
// Error-precedence tests using mockEnricher
// ---------------------------------------------------------------------------

func TestEnrichOverviewRow_ErrorPrecedence(t *testing.T) {
	// Verify the deterministic error-merge logic: actualErr wins over
	// projectedErr; if only one is non-nil, that one is used; if neither
	// is non-nil, row.Error remains nil.
	errActual := errors.New("actual cost fetch failed")
	errProjected := errors.New("projected cost fetch failed")

	tests := []struct {
		name         string
		actualErr    error
		projectedErr error
		wantNilErr   bool
		wantErrMsg   string
	}{
		{
			name:         "actual error only",
			actualErr:    errActual,
			projectedErr: nil,
			wantNilErr:   false,
			wantErrMsg:   "actual cost fetch failed",
		},
		{
			name:         "projected error only",
			actualErr:    nil,
			projectedErr: errProjected,
			wantNilErr:   false,
			wantErrMsg:   "projected cost fetch failed",
		},
		{
			name:         "both errors — actual wins",
			actualErr:    errActual,
			projectedErr: errProjected,
			wantNilErr:   false,
			wantErrMsg:   "actual cost fetch failed",
		},
		{
			name:         "no errors",
			actualErr:    nil,
			projectedErr: nil,
			wantNilErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mock := &mockEnricher{
				actualErr:    tt.actualErr,
				projectedErr: tt.projectedErr,
			}

			row := OverviewRow{
				URN:    "urn:test:error-precedence",
				Type:   "aws:ec2:Instance",
				Status: StatusActive,
			}
			dateRange := DateRange{
				Start: time.Now().Add(-24 * time.Hour),
				End:   time.Now(),
			}

			enrichOverviewRow(ctx, &row, mock, dateRange)

			if tt.wantNilErr {
				assert.Nil(t, row.Error, "row.Error must be nil when no enrichment errors occur")
			} else {
				require.NotNil(t, row.Error, "row.Error must be set")
				assert.Contains(t, row.Error.Message, tt.wantErrMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Positive-path CostDrift test using mockEnricher
// ---------------------------------------------------------------------------

func TestEnrichOverviewRow_CostDrift_ComputedOnCompletion(t *testing.T) {
	// Verify that CostDrift is computed when both actual and projected costs
	// are returned by the engine. The mock returns deterministic values so
	// the expected drift can be calculated exactly.
	//
	// Setup: dateRange ends on day 15 of a 30-day month.
	//   actualMTD   = 50.0
	//   projected   = 200.0
	//   extrapolated = 50 * (30/15) = 100.0
	//   delta        = 100 - 200 = -100.0
	//   percentDrift = -100/200 * 100 = -50% → abs > 10%, so CostDrift is non-nil.
	ctx := context.Background()

	// Use a fixed mid-month reference date for determinism.
	refTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	dateRange := DateRange{
		Start: refTime.Add(-24 * time.Hour),
		End:   refTime,
	}

	mock := &mockEnricher{
		actualResult: &CostResultWithErrors{
			Results: []CostResult{{TotalCost: 50.0, Currency: "USD"}},
		},
		projectedResult: &CostResultWithErrors{
			Results: []CostResult{{Monthly: 200.0, Currency: "USD"}},
		},
	}

	row := OverviewRow{
		URN:    "urn:test:drift-computed",
		Type:   "aws:ec2:Instance",
		Status: StatusActive,
	}

	enrichOverviewRow(ctx, &row, mock, dateRange)

	require.NotNil(t, row.ActualCost, "ActualCost must be set by mock")
	require.NotNil(t, row.ProjectedCost, "ProjectedCost must be set by mock")
	assert.Equal(t, 50.0, row.ActualCost.MTDCost)
	assert.Equal(t, 200.0, row.ProjectedCost.MonthlyCost)

	// With day=15, daysInMonth=30, actual=50, projected=200:
	// percentDrift = -50% which exceeds the 10% warning threshold.
	require.NotNil(t, row.CostDrift, "CostDrift must be computed when both costs are present")
	assert.InDelta(t, 100.0, row.CostDrift.ExtrapolatedMonthly, 0.001)
	assert.InDelta(t, 200.0, row.CostDrift.Projected, 0.001)
	assert.InDelta(t, -100.0, row.CostDrift.Delta, 0.001)
	assert.InDelta(t, -50.0, row.CostDrift.PercentDrift, 0.001)
	assert.True(t, row.CostDrift.IsWarning)
}
