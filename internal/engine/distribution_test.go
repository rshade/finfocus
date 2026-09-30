package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDistributeDailyCosts_DailyGrouping verifies daily cost distribution with GroupByDaily.
// Each daily cost should be assigned to its corresponding date period.
func TestDistributeDailyCosts_DailyGrouping(t *testing.T) {
	t.Parallel()

	periods := make(map[string]map[string]float64)
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := CostResult{
		ResourceType: "aws:ec2:Instance",
		StartDate:    startDate,
		DailyCosts:   []float64{10.0, 20.0, 30.0},
	}

	distributeDailyCosts(periods, result, "aws", GroupByDaily)

	assert.Len(t, periods, 3)
	assert.InDelta(t, 10.0, periods["2024-01-15"]["aws"], 1e-9)
	assert.InDelta(t, 20.0, periods["2024-01-16"]["aws"], 1e-9)
	assert.InDelta(t, 30.0, periods["2024-01-17"]["aws"], 1e-9)
}

// TestDistributeDailyCosts_MonthlyGrouping verifies costs are grouped by month.
// Multiple days in same month should accumulate to single month period.
func TestDistributeDailyCosts_MonthlyGrouping(t *testing.T) {
	t.Parallel()

	periods := make(map[string]map[string]float64)
	startDate := time.Date(2024, 1, 28, 0, 0, 0, 0, time.UTC)
	result := CostResult{
		ResourceType: "aws:s3:Bucket",
		StartDate:    startDate,
		DailyCosts:   []float64{5.0, 5.0, 5.0, 5.0}, // 4 days starting Jan 28
	}

	distributeDailyCosts(periods, result, "aws", GroupByMonthly)

	// Jan 28-31 = 4 days, all in January
	assert.Len(t, periods, 1)
	assert.InDelta(t, 20.0, periods["2024-01"]["aws"], 1e-9)
}

// TestDistributeDailyCosts_CrossMonthBoundary verifies correct distribution across month boundaries.
// Costs spanning month end should split correctly between months.
func TestDistributeDailyCosts_CrossMonthBoundary(t *testing.T) {
	t.Parallel()

	periods := make(map[string]map[string]float64)
	startDate := time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC)
	result := CostResult{
		ResourceType: "azure:compute:VM",
		StartDate:    startDate,
		DailyCosts:   []float64{10.0, 10.0, 10.0, 10.0}, // Jan 30, 31, Feb 1, 2
	}

	distributeDailyCosts(periods, result, "azure", GroupByMonthly)

	assert.Len(t, periods, 2)
	assert.InDelta(t, 20.0, periods["2024-01"]["azure"], 1e-9) // Jan 30 + 31
	assert.InDelta(t, 20.0, periods["2024-02"]["azure"], 1e-9) // Feb 1 + 2
}

// TestDistributeDailyCosts_EmptyDailyCosts verifies no entries are added for empty array.
func TestDistributeDailyCosts_EmptyDailyCosts(t *testing.T) {
	t.Parallel()

	periods := make(map[string]map[string]float64)
	result := CostResult{
		ResourceType: "gcp:compute:Instance",
		StartDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		DailyCosts:   []float64{},
	}

	distributeDailyCosts(periods, result, "gcp", GroupByDaily)

	assert.Empty(t, periods)
}

// TestDistributeDailyCosts_MultipleProviders verifies costs accumulate per provider.
func TestDistributeDailyCosts_MultipleProviders(t *testing.T) {
	t.Parallel()

	periods := make(map[string]map[string]float64)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	awsResult := CostResult{
		ResourceType: "aws:ec2:Instance",
		StartDate:    startDate,
		DailyCosts:   []float64{100.0},
	}
	azureResult := CostResult{
		ResourceType: "azure:compute:VM",
		StartDate:    startDate,
		DailyCosts:   []float64{50.0},
	}

	distributeDailyCosts(periods, awsResult, "aws", GroupByDaily)
	distributeDailyCosts(periods, azureResult, "azure", GroupByDaily)

	assert.Len(t, periods, 1)
	assert.InDelta(t, 100.0, periods["2024-01-01"]["aws"], 1e-9)
	assert.InDelta(t, 50.0, periods["2024-01-01"]["azure"], 1e-9)
}
