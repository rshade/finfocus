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

	// With no plugins, actual/projected will be empty or have placeholder values
	// The function should not panic and should complete gracefully
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
			URN:    "urn:resource-" + string(rune('a'+i)),
			Type:   "aws:ec2:Instance",
			Status: StatusActive,
		}
	}

	progressChan := make(chan OverviewRowUpdate, len(rows))
	result := EnrichOverviewRows(ctx, rows, eng, dateRange, progressChan)
	// With cancelled context, some or all rows may not be enriched
	assert.NotNil(t, result)
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
