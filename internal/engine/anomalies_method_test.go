package engine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

// The robust-median fixtures use one arithmetic-progression baseline: 80 to
// 145 in steps of 5 across 14 earlier days. Its median is 112.5 and its MAD
// is 17.5 (six deviations of 7.5, four of 17.5, and four of 27.5 with the
// middle two at 17.5), both hand-checkable, so every z-score below is too.
const (
	medianFixture = 112.5
	madFixture    = 17.5
)

// testDay is the window's first UTC day. Offsets count days from it.
func testDay() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }

func day(offset int) time.Time { return testDay().AddDate(0, 0, offset) }

// dailyAmounts builds series points for one resource: one amount per day
// offset, and no point for an offset that is not given. A missing offset is a
// missing day, the way the store behaves.
func dailyAmounts(urn, currency string, amounts map[int]float64) []history.DailyPoint {
	points := make([]history.DailyPoint, 0, len(amounts))
	for offset, amount := range amounts {
		points = append(points, history.DailyPoint{
			ResourceID: urn,
			Day:        day(offset),
			Currency:   currency,
			Amount:     amount,
		})
	}
	return points
}

// methodBaseline builds the 80..145 progression on offsets 0..13 (fourteen
// earlier points) with the observation on offset 14.
func methodBaseline(urn, currency string, amount float64) map[int]float64 {
	amounts := make(map[int]float64, 15)
	for offset := 0; offset < 14; offset++ {
		amounts[offset] = 80 + 5*float64(offset)
	}
	amounts[14] = amount
	return amounts
}

// driftAnomalyRow returns a row carrying a real warning-level drift, so the
// fallback path is the one the overview computes.
func driftAnomalyRow(urn, currency string, delta float64) OverviewRow {
	return OverviewRow{
		URN:  urn,
		Type: "aws:ec2/instance:Instance",
		CostDrift: &CostDriftData{
			ExtrapolatedMonthly: 100 + delta, Projected: 100, Delta: delta, PercentDrift: delta, IsWarning: true,
		},
		ActualCost: &ActualCostData{
			Currency: currency,
			Period:   DateRange{Start: testDay(), End: day(14)},
		},
	}
}

func TestSelectAnomaliesRobustMedian_QualifyingRowHasMethod(t *testing.T) {
	t.Parallel()

	observed := medianFixture + 137.5 // z = 0.6745 * 137.5 / 17.5 ~= 5.29
	series := dailyAmounts("urn:spike", "USD", methodBaseline("urn:spike", "USD", observed))
	// No drift at all: the row exists only because the daily series qualifies.
	row := OverviewRow{URN: "urn:spike", Type: "aws:ec2/instance:Instance"}

	report := SelectAnomalies([]OverviewRow{row}, series)

	require.Len(t, report.Anomalies, 1)
	anomaly := report.Anomalies[0]
	assert.Equal(t, AnomalyMethodRobustMedian, anomaly.Method)
	assert.Equal(t, "urn:spike", anomaly.Resource)
	assert.Equal(t, row.Type, anomaly.Type)
	assert.InDelta(t, medianFixture, anomaly.Baseline, 1e-9, "the baseline is the median of the earlier days")
	assert.InDelta(t, observed, anomaly.Observed, 1e-9, "the observed is the latest complete day")
	assert.InDelta(t, observed-medianFixture, anomaly.Deviation, 1e-9)
	assert.InDelta(t, (observed-medianFixture)/medianFixture*100, anomaly.PercentDrift, 1e-9)
	assert.Equal(t, AnomalySeverityWarning, anomaly.Severity)
	assert.Equal(t, "USD", anomaly.Currency)
	assert.Equal(t, DateRange{Start: day(14), End: day(14)}, anomaly.Period, "the period is the observation day")
	require.NotNil(t, anomaly.BaselineWindow)
	assert.Equal(t, DateRange{Start: day(0), End: day(13)}, *anomaly.BaselineWindow)
}

func TestSelectAnomaliesRobustMedian_AtAndOverTheCutoff(t *testing.T) {
	t.Parallel()

	// The exact cutoff: 3.5 * 17.5 / 0.6745. The escape clause is verified by
	// the engine formula, and the 1e-9 margins keep the float compare stable.
	boundary := robustMedianZCutoff * madFixture / robustZNormalScore

	tests := []struct {
		name     string
		offset   float64 // relative nudge around the exact cutoff
		wantURNs []string
	}{
		{name: "exactly at the cutoff is emitted", offset: 0, wantURNs: []string{"urn:edge"}},
		{name: "just under the cutoff is not", offset: -1e-9, wantURNs: []string{}},
		{name: "just over the cutoff is emitted", offset: 1e-9, wantURNs: []string{"urn:edge"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseline := methodBaseline("urn:edge", "USD", medianFixture+boundary*(1+tt.offset))
			series := dailyAmounts("urn:edge", "USD", baseline)
			report := SelectAnomalies([]OverviewRow{{URN: "urn:edge"}}, series)
			got := make([]string, 0, len(report.Anomalies))
			for _, a := range report.Anomalies {
				got = append(got, a.Resource)
			}
			assert.Equal(t, tt.wantURNs, got)
		})
	}
}

func TestSelectAnomaliesRobustMedian_MinimumHistory(t *testing.T) {
	t.Parallel()

	// 80..145 on offsets 0..12 is thirteen points: an earlier-day range of
	// fourteen days minus one missing day still falls short, a whole-month
	// fifteen-day gap would too. The observation stays huge, so only the
	// fourteen-point count, not the z-score, decides these. A warning-level
	// drift row is present in every case, so the short-series outcomes show
	// the fallback: the drift row stays when the series does not qualify.
	build := func(offsets []int) []history.DailyPoint {
		amounts := make(map[int]float64, len(offsets)+1)
		for _, offset := range offsets {
			amounts[offset] = 80 + 5*float64(offset)
		}
		amounts[14] = medianFixture + 137.5
		return dailyAmounts("urn:short", "USD", amounts)
	}

	tests := []struct {
		name       string
		offsets    []int
		wantMethod string
	}{
		{
			name:       "fourteen earlier points qualify",
			offsets:    []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13},
			wantMethod: AnomalyMethodRobustMedian,
		},
		{
			name:       "thirteen earlier points keep the drift row",
			offsets:    []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			wantMethod: AnomalyMethodDrift,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rows := []OverviewRow{driftAnomalyRow("urn:short", "USD", 50)}
			series := build(tt.offsets)
			report := SelectAnomalies(rows, series)
			require.Len(t, report.Anomalies, 1)
			assert.Equal(t, tt.wantMethod, report.Anomalies[0].Method)
		})
	}
}

func TestSelectAnomaliesRobustMedian_GapCountsNotZero(t *testing.T) {
	t.Parallel()

	// Fourteen baseline days laid out over a sixteen-day span with days 8 and
	// 9 missing, holding one ascending value per stored day (50, 55, ...).
	// The median of 82.5 and the MAD of 17.5 come from the stored days only:
	// zero-filling the two gaps would move the median to 67.5 and the
	// deviation to 132.5, so the asserted numbers prove the reader did not
	// invent zeros.
	baseline := make(map[int]float64, 16)
	offsets := []int{0, 1, 2, 3, 4, 5, 6, 7, 10, 11, 12, 13, 14, 15}
	for i, offset := range offsets {
		baseline[offset] = 50 + 5*float64(i)
	}
	baseline[16] = 200 // deviation 117.5, z = 0.6745 * 117.5 / 17.5 ~= 4.53
	series := dailyAmounts("urn:gap", "USD", baseline)

	report := SelectAnomalies([]OverviewRow{{URN: "urn:gap"}}, series)
	require.Len(t, report.Anomalies, 1)
	anomaly := report.Anomalies[0]
	assert.Equal(t, AnomalyMethodRobustMedian, anomaly.Method)
	assert.InDelta(t, 82.5, anomaly.Baseline, 1e-9, "a missing day is a gap, never a zero")
	assert.InDelta(t, 200.0, anomaly.Observed, 1e-9)
	assert.InDelta(t, 117.5, anomaly.Deviation, 1e-9)
	require.NotNil(t, anomaly.BaselineWindow)
	assert.Equal(t, day(15), anomaly.BaselineWindow.End)
}

func TestSelectAnomaliesRobustMedian_GapFallsUnderTheMinimum(t *testing.T) {
	t.Parallel()

	// A fourteen-day span with one missing day is thirteen qualifying points:
	// the gap must not be filled in to reach the minimum.
	amounts := make(map[int]float64)
	for offset := 0; offset < 14; offset++ {
		if offset == 7 {
			continue
		}
		amounts[offset] = 100 + 5*float64(offset)
	}
	amounts[14] = 500
	series := dailyAmounts("urn:shortgap", "USD", amounts)

	report := SelectAnomalies([]OverviewRow{OverviewRow{URN: "urn:shortgap"}}, series)
	assert.Empty(t, report.Anomalies)
}

func TestSelectAnomaliesRobustMedian_ZeroMADStaysSilent(t *testing.T) {
	t.Parallel()

	// Fourteen identical days: a flat series hides a spike from the MAD, so
	// no statistical row is emitted and the drift row remains.
	amounts := make(map[int]float64)
	for offset := 0; offset < 14; offset++ {
		amounts[offset] = 100
	}
	amounts[14] = 1000
	series := dailyAmounts("urn:flat", "USD", amounts)

	tests := []struct {
		name       string
		rows       []OverviewRow
		wantMethod string
	}{
		{
			name:       "the drift row survives a flat series",
			rows:       []OverviewRow{driftAnomalyRow("urn:flat", "USD", 50)},
			wantMethod: AnomalyMethodDrift,
		},
		{
			name: "no drift row and no statistical row means no rows",
			rows: []OverviewRow{OverviewRow{URN: "urn:flat"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report := SelectAnomalies(tt.rows, series)
			if tt.wantMethod == "" {
				assert.Empty(t, report.Anomalies)
				return
			}
			require.Len(t, report.Anomalies, 1)
			assert.Equal(t, tt.wantMethod, report.Anomalies[0].Method)
		})
	}
}

func TestSelectAnomaliesRobustMedian_PricingFailureIsNotScored(t *testing.T) {
	t.Parallel()

	// Enrichment records a plugin pricing failure through
	// ProjectedPricingFailed while row.Error stays nil, and the resource's
	// stored daily series may still qualify. The row's costs are missing, so
	// neither method may report it.
	observed := medianFixture + 137.5
	series := dailyAmounts("urn:unpriced", "USD", methodBaseline("urn:unpriced", "USD", observed))
	row := OverviewRow{
		URN:                    "urn:unpriced",
		Type:                   "aws:ec2/instance:Instance",
		ProjectedPricingFailed: true,
	}

	report := SelectAnomalies([]OverviewRow{row}, series)

	assert.Empty(t, report.Anomalies, "a row the engines never priced is no statistical anomaly")
	assert.Equal(t, 1, report.Considered)
}

func TestSelectAnomaliesRobustMedian_ZeroMedianOmitsPercentDrift(t *testing.T) {
	t.Parallel()

	// A zero median with a non-zero MAD needs negative amounts: for
	// non-negative daily costs a median of zero always collapses the MAD to
	// zero too. The synthetic series holds seven −10 days, one 0 day, and
	// seven 40 days around offset 7, so the median is exactly 0 and the MAD
	// is 10. The percent drift is undefined, so it stays out of the JSON
	// while the deviation and every other field remain.
	amounts := make(map[int]float64)
	for offset := 0; offset < 7; offset++ {
		amounts[offset] = -10
	}
	amounts[7] = 0
	for offset := 8; offset < 15; offset++ {
		amounts[offset] = 40
	}
	amounts[15] = 100 // deviation 100, z = 0.6745 * 100 / 10 = 6.745
	series := dailyAmounts("urn:zeromedian", "USD", amounts)

	report := SelectAnomalies([]OverviewRow{{URN: "urn:zeromedian"}}, series)
	require.Len(t, report.Anomalies, 1)
	anomaly := report.Anomalies[0]
	assert.Equal(t, AnomalyMethodRobustMedian, anomaly.Method)

	var decoded struct {
		Anomalies []map[string]any `json:"anomalies"`
	}
	require.NoError(t, json.NewDecoder(strings.NewReader(anomalyJSON(t, report))).Decode(&decoded))
	require.Len(t, decoded.Anomalies, 1)
	assert.NotContains(t, decoded.Anomalies[0], "percentDrift")
	assert.Equal(t, float64(100), decoded.Anomalies[0]["deviation"])
}

func TestRenderAnomaliesRobustMedianFields(t *testing.T) {
	t.Parallel()

	t.Run("json carries the baseline window and omits it on drift rows", func(t *testing.T) {
		t.Parallel()
		observed := medianFixture + 137.5
		series := dailyAmounts("urn:render", "USD", methodBaseline("urn:render", "USD", observed))
		rows := []OverviewRow{
			OverviewRow{URN: "urn:render", Type: "aws:ec2/instance:Instance"},
			driftAnomalyRow("urn:still", "USD", 50),
		}
		report := SelectAnomalies(rows, series)
		require.Len(t, report.Anomalies, 2)

		text := anomalyJSON(t, report)
		assert.Contains(t, text, `"method": "robust-median"`)
		assert.Contains(t, text, `"baselineWindow"`)

		var decoded struct {
			Anomalies []map[string]any `json:"anomalies"`
		}
		require.NoError(t, json.NewDecoder(strings.NewReader(text)).Decode(&decoded))
		require.Len(t, decoded.Anomalies, 2)
		byResource := make(map[string]map[string]any, 2)
		for _, item := range decoded.Anomalies {
			byResource[item["resource"].(string)] = item
		}
		robust, ok := byResource["urn:render"]
		require.True(t, ok)
		assert.Contains(t, robust, "baselineWindow")
		window := robust["baselineWindow"].(map[string]any)
		assert.Equal(t, "2026-10-01T00:00:00Z", window["start"])
		assert.Equal(t, "2026-10-14T00:00:00Z", window["end"])
		assert.Contains(t, robust, "percentDrift")

		still, ok := byResource["urn:still"]
		require.True(t, ok)
		assert.NotContains(t, still, "baselineWindow", "drift rows leave the window unset")
		assert.Contains(t, still, "percentDrift")
	})

	t.Run("ndjson writes one robust-median line", func(t *testing.T) {
		t.Parallel()
		observed := medianFixture + 137.5
		series := dailyAmounts("urn:nd", "USD", methodBaseline("urn:nd", "USD", observed))
		report := SelectAnomalies([]OverviewRow{{URN: "urn:nd"}}, series)

		var buf strings.Builder
		require.NoError(t, RenderAnomaliesAsNDJSON(&buf, report))

		lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
		require.Len(t, lines, 1)
		var anomaly Anomaly
		require.NoError(t, json.Unmarshal([]byte(lines[0]), &anomaly))
		assert.Equal(t, AnomalyMethodRobustMedian, anomaly.Method)
		require.NotNil(t, anomaly.BaselineWindow)
		assert.Equal(t, day(13), anomaly.BaselineWindow.End)
		assert.InDelta(t, observed, anomaly.Observed, 1e-9)
	})
}

func anomalyJSON(t *testing.T, report AnomalyReport) string {
	t.Helper()
	var buf strings.Builder
	require.NoError(t, RenderAnomaliesAsJSON(&buf, report))
	return buf.String()
}

func TestSelectAnomaliesRobustMedian_TwoCurrenciesStaySeparate(t *testing.T) {
	t.Parallel()

	// Two currencies of one resource are two series. Each would qualify on
	// its own, but the resource gets one row: the deviation decides which.
	amounts := make(map[int]float64)
	for offset := 0; offset < 14; offset++ {
		amounts[offset] = 80 + 5*float64(offset)
	}
	usdAmounts := cloneAmounts(amounts)
	usdAmounts[14] = medianFixture + 137.5 // deviation 137.5
	eurAmounts := cloneAmounts(amounts)
	eurAmounts[14] = 20.

	var series []history.DailyPoint
	series = append(series, dailyAmounts("urn:two", "EUR", eurAmounts)...)
	series = append(series, dailyAmounts("urn:two", "USD", usdAmounts)...)

	report := SelectAnomalies([]OverviewRow{{URN: "urn:two"}}, series)
	require.Len(t, report.Anomalies, 1, "not one row per currency")
	anomaly := report.Anomalies[0]
	assert.Equal(t, "USD", anomaly.Currency, "the larger absolute deviation wins")
	assert.InDelta(t, 137.5, anomaly.Deviation, 1e-9)
}

func cloneAmounts(in map[int]float64) map[int]float64 {
	out := make(map[int]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func TestSelectAnomaliesRobustMedian_NegativeSpike(t *testing.T) {
	t.Parallel()

	observed := medianFixture - 87.5 // |z| = 0.6745 * 87.5 / 17.5 ~= 3.37 -> under
	series := dailyAmounts("urn:under", "USD", methodBaseline("urn:under", "USD", observed))
	report := SelectAnomalies([]OverviewRow{{URN: "urn:under"}}, series)
	assert.Empty(t, report.Anomalies)
}

func TestSelectAnomaliesRobustMedian_ReplacesDriftRowWithoutSecondRow(t *testing.T) {
	t.Parallel()

	observed := medianFixture + 137.5
	series := dailyAmounts("urn:both", "USD", methodBaseline("urn:both", "USD", observed))
	rows := []OverviewRow{driftAnomalyRow("urn:both", "USD", 50)}

	report := SelectAnomalies(rows, series)

	require.Len(t, report.Anomalies, 1)
	anomaly := report.Anomalies[0]
	assert.Equal(t, AnomalyMethodRobustMedian, anomaly.Method)
	assert.InDelta(t, medianFixture, anomaly.Baseline, 1e-9, "the projected month is not the baseline")
	assert.InDelta(t, observed, anomaly.Observed, 1e-9)
	assert.Equal(t, "urn:both", anomaly.Resource)
}

func TestSelectAnomaliesRobustMedian_KeepsDriftRowWhenSeriesFallsShort(t *testing.T) {
	t.Parallel()

	amounts := methodBaseline("urn:fallback", "USD", medianFixture+137.5)
	delete(amounts, 13) // thirteen earlier points
	series := dailyAmounts("urn:fallback", "USD", amounts)

	report := SelectAnomalies([]OverviewRow{driftAnomalyRow("urn:fallback", "USD", 50)}, series)
	require.Len(t, report.Anomalies, 1)
	assert.Equal(t, AnomalyMethodDrift, report.Anomalies[0].Method)
}

func TestSelectAnomaliesRobustMedian_SecondRowNeverAppears(t *testing.T) {
	t.Parallel()

	observed := medianFixture + 137.5
	series := dailyAmounts("urn:one-row", "USD", methodBaseline("urn:one-row", "USD", observed))
	rows := []OverviewRow{
		driftAnomalyRow("urn:one-row", "USD", 50),
		driftAnomalyRow("urn:one-row", "USD", 80), // duplicated URN edge
	}

	report := SelectAnomalies(rows, series)
	require.Len(t, report.Anomalies, 1)
	assert.Equal(t, AnomalyMethodRobustMedian, report.Anomalies[0].Method)
	assert.Equal(t, 2, report.Considered, "the considered count is unchanged by the method")
}

func TestSelectAnomalies_EqualInputsGiveEqualOutput(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{
		driftAnomalyRow("urn:a", "USD", 50),
		driftAnomalyRow("urn:b", "USD", 80),
	}
	observed := medianFixture + 137.5
	series := dailyAmounts("urn:a", "USD", methodBaseline("urn:a", "USD", observed))

	first := SelectAnomalies(rows, series)
	second := SelectAnomalies(rows, series)
	require.Len(t, first.Anomalies, len(second.Anomalies))
	for i := range first.Anomalies {
		assert.Equal(t, first.Anomalies[i].Resource, second.Anomalies[i].Resource)
		assert.Equal(t, first.Anomalies[i].Method, second.Anomalies[i].Method)
	}
}

func TestSelectAnomalies_EmptySeriesMatchesDriftOutput(t *testing.T) {
	t.Parallel()

	rows := []OverviewRow{
		driftAnomalyRow("urn:drift", "USD", 50),
		OverviewRow{URN: "urn:plain"},
	}
	withSeries := SelectAnomalies(rows, []history.DailyPoint{})
	withoutSeries := SelectAnomalies(rows, []history.DailyPoint{})
	require.Len(t, withoutSeries.Anomalies, 1)
	require.Len(t, withSeries.Anomalies, len(withoutSeries.Anomalies))
	assert.Equal(t, AnomalyMethodDrift, withSeries.Anomalies[0].Method)
}
