package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anomalyRow builds an enriched row whose drift is computed by the real
// CalculateCostDriftWithElapsedDays, so the tests exercise the same threshold
// and skip rules the overview applies.
func anomalyRow(t *testing.T, urn, currency string, mtd, projected, elapsedDays float64) OverviewRow {
	t.Helper()
	const daysInMonth = 30
	drift, err := CalculateCostDriftWithElapsedDays(mtd, projected, elapsedDays, daysInMonth)
	if elapsedDays < driftMinElapsedDays {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return OverviewRow{
		URN:  urn,
		Type: "aws:ec2/instance:Instance",
		ActualCost: &ActualCostData{
			MTDCost:  mtd,
			Currency: currency,
			Period:   DateRange{Start: start, End: start.AddDate(0, 0, 10)},
		},
		ProjectedCost: &ProjectedCostData{MonthlyCost: projected},
		CostDrift:     drift,
	}
}

func TestSelectAnomalies_Selection(t *testing.T) {
	t.Parallel()

	const (
		projected = 730.0
		// 730h basis over 30 days gives 720 for the calendar month, so an MTD of
		// 10 days at 24 per day extrapolates to exactly the projection.
		steadyMTD = 240.0
	)

	tests := []struct {
		name      string
		build     func(t *testing.T) []OverviewRow
		wantURNs  []string
		wantTotal int
	}{
		{
			name: "over threshold is listed",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{anomalyRow(t, "urn:over", "USD", steadyMTD*1.5, projected, 10)}
			},
			wantURNs:  []string{"urn:over"},
			wantTotal: 1,
		},
		{
			name: "just over threshold is listed",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{anomalyRow(t, "urn:just", "USD", steadyMTD*1.11, projected, 10)}
			},
			wantURNs:  []string{"urn:just"},
			wantTotal: 1,
		},
		{
			name: "under threshold is not listed",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{anomalyRow(t, "urn:under", "USD", steadyMTD*1.05, projected, 10)}
			},
			wantURNs:  []string{},
			wantTotal: 1,
		},
		{
			name: "elapsed under two days has no drift",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{anomalyRow(t, "urn:early", "USD", 500, projected, 1.5)}
			},
			wantURNs:  []string{},
			wantTotal: 1,
		},
		{
			name: "new resource with no actual spend has no drift",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{anomalyRow(t, "urn:new", "USD", 0, projected, 10)}
			},
			wantURNs:  []string{},
			wantTotal: 1,
		},
		{
			name: "deleted resource with no projection has no drift",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{anomalyRow(t, "urn:deleted", "USD", 100, 0, 10)}
			},
			wantURNs:  []string{},
			wantTotal: 1,
		},
		{
			name: "error row with drift is not listed",
			build: func(t *testing.T) []OverviewRow {
				const urn = "urn:err"
				row := anomalyRow(t, urn, "USD", steadyMTD*2, projected, 10)
				row.Error = &OverviewRowError{URN: urn, ErrorType: ErrorTypeUnknown, Message: "boom"}
				return []OverviewRow{row}
			},
			wantURNs:  []string{},
			wantTotal: 1,
		},
		{
			name: "row a preview marks for deletion is not listed even with drift",
			build: func(t *testing.T) []OverviewRow {
				row := anomalyRow(t, "urn:deleting", "USD", steadyMTD*2, projected, 10)
				row.Status = StatusDeleting
				return []OverviewRow{row}
			},
			wantURNs:  []string{},
			wantTotal: 1,
		},
		{
			name: "updating and replacing rows with drift are still listed",
			build: func(t *testing.T) []OverviewRow {
				updating := anomalyRow(t, "urn:updating", "USD", steadyMTD*2, projected, 10)
				updating.Status = StatusUpdating
				replacing := anomalyRow(t, "urn:replacing", "USD", steadyMTD*3, projected, 10)
				replacing.Status = StatusReplacing
				return []OverviewRow{updating, replacing}
			},
			wantURNs:  []string{"urn:replacing", "urn:updating"},
			wantTotal: 2,
		},
		{
			name: "two currencies are listed separately",
			build: func(t *testing.T) []OverviewRow {
				return []OverviewRow{
					anomalyRow(t, "urn:usd", "USD", steadyMTD*2, projected, 10),
					anomalyRow(t, "urn:eur", "EUR", steadyMTD*3, projected, 10),
				}
			},
			wantURNs:  []string{"urn:eur", "urn:usd"},
			wantTotal: 2,
		},
		{
			name:      "empty input",
			build:     func(*testing.T) []OverviewRow { return nil },
			wantURNs:  []string{},
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report := SelectAnomalies(tt.build(t))

			assert.Equal(t, tt.wantTotal, report.Considered)
			require.NotNil(t, report.Anomalies, "an empty list must marshal as [], not null")
			got := make([]string, 0, len(report.Anomalies))
			for _, a := range report.Anomalies {
				got = append(got, a.Resource)
			}
			assert.Equal(t, tt.wantURNs, got)
		})
	}
}

func TestSelectAnomalies_FieldMapping(t *testing.T) {
	t.Parallel()

	row := anomalyRow(t, "urn:map", "EUR", 480, 730, 10)
	require.NotNil(t, row.CostDrift)

	report := SelectAnomalies([]OverviewRow{row})
	require.Len(t, report.Anomalies, 1)
	got := report.Anomalies[0]

	assert.Equal(t, "urn:map", got.Resource)
	assert.Equal(t, row.Type, got.Type)
	assert.Equal(t, row.ActualCost.Period, got.Period)
	assert.InDelta(t, row.CostDrift.Projected, got.Baseline, 1e-9)
	assert.InDelta(t, row.CostDrift.ExtrapolatedMonthly, got.Observed, 1e-9)
	assert.InDelta(t, row.CostDrift.Delta, got.Deviation, 1e-9)
	assert.InDelta(t, row.CostDrift.PercentDrift, got.PercentDrift, 1e-9)
	assert.Equal(t, AnomalySeverityWarning, got.Severity)
	assert.Equal(t, AnomalyMethodDrift, got.Method)
	assert.Equal(t, "EUR", got.Currency)
	assert.InDelta(t, got.Observed-got.Baseline, got.Deviation, 1e-9, "observed is a monthly figure, not MTD")
	assert.NotEqual(t, row.ActualCost.MTDCost, got.Observed)
}

func TestSelectAnomalies_MissingActualCost(t *testing.T) {
	t.Parallel()

	row := OverviewRow{
		URN:  "urn:noactual",
		Type: "aws:s3/bucket:Bucket",
		CostDrift: &CostDriftData{
			ExtrapolatedMonthly: 200, Projected: 100, Delta: 100, PercentDrift: 100, IsWarning: true,
		},
	}

	report := SelectAnomalies([]OverviewRow{row})

	require.Len(t, report.Anomalies, 1)
	assert.Equal(t, DateRange{}, report.Anomalies[0].Period)
	assert.Empty(t, report.Anomalies[0].Currency)
}

func TestSelectAnomalies_Ordering(t *testing.T) {
	t.Parallel()

	drift := func(delta float64) *CostDriftData {
		return &CostDriftData{
			ExtrapolatedMonthly: 100 + delta, Projected: 100, Delta: delta, PercentDrift: delta, IsWarning: true,
		}
	}
	rows := []OverviewRow{
		{URN: "urn:b", CostDrift: drift(50)},
		{URN: "urn:small", CostDrift: drift(15)},
		{URN: "urn:a", CostDrift: drift(50)},
		{URN: "urn:negative", CostDrift: drift(-80)},
	}

	report := SelectAnomalies(rows)

	got := make([]string, 0, len(report.Anomalies))
	for _, a := range report.Anomalies {
		got = append(got, a.Resource)
	}
	assert.Equal(t, []string{"urn:negative", "urn:a", "urn:b", "urn:small"}, got)
	assert.Equal(t, 4, report.Considered)
}

func TestSelectAnomalies_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{
		{URN: "urn:z", CostDrift: &CostDriftData{Delta: 10, PercentDrift: 20, IsWarning: true}},
		{URN: "urn:y", CostDrift: &CostDriftData{Delta: 90, PercentDrift: 20, IsWarning: true}},
	}

	_ = SelectAnomalies(rows)

	assert.Equal(t, "urn:z", rows[0].URN)
	assert.Equal(t, "urn:y", rows[1].URN)
}
