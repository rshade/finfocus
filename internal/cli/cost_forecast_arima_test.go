package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/ax-go"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/forecast"
)

var errArimaTestPriced = errors.New("pricing ran")

func arimaNow() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }

// spendCSV is 180 days of spend through 2026-09-30 with a trend and a
// weekend dip.
func spendCSV() string {
	var b strings.Builder
	b.WriteString("date,value\n")
	first := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	for i := range 180 {
		d := first.AddDate(0, 0, i)
		v := 50 + 0.1*float64(i) + float64((i*37)%11)/10 - 0.5
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			v -= 15
		}
		fmt.Fprintf(&b, "%s,%.2f\n", d.Format("2006-01-02"), v)
	}
	return b.String()
}

func writeSpendCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spend.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func arimaCosts() []engine.CostResult {
	return []engine.CostResult{
		{ResourceID: "ec2", ResourceType: "aws:ec2:Instance", Monthly: 1500, Currency: "USD", GrowthType: "none"},
	}
}

func runArimaForecast(t *testing.T, costs []engine.CostResult, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, args...)
	err := runForecast(cmd, forecastDeps{
		now: arimaNow(), cfg: &config.Config{}, skipPrice: true, costs: costs, skipHistory: true,
	})
	return buf.String(), err
}

func TestRunForecast_ModelInputsRejectedBeforePricing(t *testing.T) {
	t.Parallel()
	good := writeSpendCSV(t, spendCSV())
	bad := writeSpendCSV(t, "date,value\n2026-04-01,1\n2026-04-02,oops\n")
	short := writeSpendCSV(t, strings.Join(strings.Split(spendCSV(), "\n")[:10], "\n")+"\n")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown model", args: []string{"--model", "prophet"},
			want: `invalid model "prophet" (supported: growth, arima)`},
		{name: "arima without history", args: []string{"--model", "arima"},
			want: "--model arima requires --spend-history"},
		{name: "history without arima", args: []string{"--spend-history", good},
			want: "--spend-history requires --model arima"},
		{name: "confidence without arima", args: []string{"--confidence", "0.8"},
			want: "--confidence requires --model arima"},
		{name: "exclude without arima", args: []string{"--exclude", "2026-05-01:2026-05-02"},
			want: "--exclude requires --model arima"},
		{name: "bad confidence", args: []string{"--model", "arima", "--spend-history", good, "--confidence", "1"},
			want: "confidence must be between 0 and 1 (exclusive), got 1"},
		{name: "bad cap", args: []string{"--growth-cap", "0"},
			want: "growth cap must be a positive factor such as 1.25, got 0"},
		{name: "bad step", args: []string{"--step", "June=5"},
			want: `invalid step "June=5": use DATE=AMOUNT with DATE as 2006-01-02`},
		{name: "bad exclude", args: []string{"--model", "arima", "--spend-history", good, "--exclude", "2026-05-01"},
			want: `invalid exclude range "2026-05-01": use FROM:TO with dates as 2006-01-02`},
		{name: "bad csv row", args: []string{"--model", "arima", "--spend-history", bad},
			want: `spend history line 3: invalid value "oops"`},
		{name: "too short", args: []string{"--model", "arima", "--spend-history", short},
			want: "spend history has 9 usable points; at least 10 are required"},
		{name: "too short after exclusion",
			args: []string{"--model", "arima", "--spend-history", good, "--exclude", "2026-04-04:2026-09-25"},
			want: "spend history has 5 usable points; at least 10 are required"},
		{name: "bad growth type", args: []string{"--growth-type", "cubic"},
			want: `invalid growth type "cubic" (supported: none, linear, exponential)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			cmd := forecastTestCmd(t, &buf, tc.args...)
			err := runForecast(cmd, forecastDeps{skipPrice: true, costErr: errArimaTestPriced, skipHistory: true})
			require.EqualError(t, err, tc.want)
			assert.True(t, forecast.IsInputError(err))
			assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err), "input errors exit 2")
		})
	}

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		missing := filepath.Join(t.TempDir(), "absent.csv")
		cmd := forecastTestCmd(t, &buf, "--model", "arima", "--spend-history", missing)
		err := runForecast(cmd, forecastDeps{skipPrice: true, costErr: errArimaTestPriced, skipHistory: true})
		require.ErrorIs(t, err, os.ErrNotExist)
		assert.True(t, forecast.IsInputError(err))
		assert.NotErrorIs(t, err, errArimaTestPriced)
	})
}

func TestRunForecast_ArimaJSON(t *testing.T) {
	t.Parallel()
	path := writeSpendCSV(t, spendCSV())
	out, err := runArimaForecast(t, arimaCosts(),
		"--model", "arima", "--spend-history", path, "--months", "3", "--confidence", "0.8", "--output", "json")
	require.NoError(t, err)

	var doc forecastJSON
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.Equal(t, forecast.ModelARIMA, doc.Model)
	assert.Contains(t, doc.ModelOrder, "[7]")
	require.NotNil(t, doc.Confidence)
	assert.InDelta(t, 0.8, *doc.Confidence, 1e-12)
	assert.Equal(t, "USD", doc.Currency)
	names := make([]string, len(doc.Series))
	for i, s := range doc.Series {
		names[i] = s.Name
	}
	assert.Equal(t, []string{"forecast", "lower", "upper", "baseline", "spend"}, names)
	require.Len(t, doc.Series[0].Points, 4)
	assert.Len(t, doc.Series[4].Points, 180)
	for i, point := range doc.Series[0].Points {
		assert.LessOrEqual(t, doc.Series[1].Points[i].Value, point.Value)
		assert.GreaterOrEqual(t, doc.Series[2].Points[i].Value, point.Value)
		assert.GreaterOrEqual(t, doc.Series[1].Points[i].Value, 0.0)
	}

	growthOut, err := runArimaForecast(t, arimaCosts(), "--months", "3", "--output", "json")
	require.NoError(t, err)
	var growth forecastJSON
	require.NoError(t, json.Unmarshal([]byte(growthOut), &growth))
	assert.Equal(t, growth.Series[0].Points, doc.Series[3].Points, "baseline is the growth forecast")
	assert.Empty(t, growth.Model)
	assert.Nil(t, growth.Confidence)
}

func TestRunForecast_ArimaWithoutPricedPlanKeepsForecast(t *testing.T) {
	t.Parallel()
	path := writeSpendCSV(t, spendCSV())
	out, err := runArimaForecast(t, []engine.CostResult{{ResourceID: "bad", Notes: "VALIDATION: missing sku"}},
		"--model", "arima", "--spend-history", path, "--months", "1", "--output", "json")
	require.NoError(t, err)
	var doc forecastJSON
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.Contains(t, doc.Warnings, "baseline: no cost data available")
	for _, s := range doc.Series {
		assert.NotEqual(t, forecast.SeriesBaseline, s.Name)
	}
	assert.Equal(t, "USD", doc.Currency)
}

func TestRunForecast_ArimaNDJSONAndPlain(t *testing.T) {
	t.Parallel()
	path := writeSpendCSV(t, spendCSV())
	out, err := runArimaForecast(t, arimaCosts(),
		"--model", "arima", "--spend-history", path, "--months", "2", "--output", "ndjson")
	require.NoError(t, err)
	scanner := bufio.NewScanner(strings.NewReader(out))
	require.True(t, scanner.Scan())
	var meta map[string]any
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &meta))
	assert.Equal(t, "meta", meta["kind"])
	assert.Equal(t, forecast.ModelARIMA, meta["model"])
	assert.Contains(t, meta["modelOrder"], "SARIMA")
	assert.InDelta(t, forecast.DefaultConfidence, meta["confidence"], 1e-12)
	series := map[string]int{}
	for scanner.Scan() {
		var line forecastLine
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &line))
		series[line.Series]++
	}
	assert.Equal(t, 3, series["lower"])
	assert.Equal(t, 3, series["upper"])
	assert.Equal(t, 180, series["spend"])

	plain, err := runArimaForecast(t, arimaCosts(),
		"--model", "arima", "--spend-history", path, "--months", "2", "--plain", "--no-color", "--width", "40",
		"--split-providers")
	require.NoError(t, err)
	assert.Contains(t, plain, "Legend: Forecast  Lower  Upper  Baseline")
	assert.Contains(t, plain, "bounds at 95% confidence.")
	assert.Contains(t, plain, "Spend history: 180 points, 2026-04-04 – 2026-09-30.")
	assert.Contains(t, plain, "--split-providers is ignored: spend history has no providers")
}

func TestRunForecast_DriversOnGrowthModel(t *testing.T) {
	t.Parallel()
	costs := []engine.CostResult{
		{ResourceID: "bucket", ResourceType: "aws:s3:Bucket", Monthly: 100, Currency: "USD", GrowthType: "linear"},
	}
	out, err := runArimaForecast(t, costs, "--months", "3", "--growth-rate", "0.10",
		"--step", "2026-11-01=50", "--growth-cap", "1.4", "--output", "json")
	require.NoError(t, err)
	var doc forecastJSON
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	// Growth: 100, 110, 120, 130; +50 from November; capped at 140.
	values := make([]float64, 0, 4)
	for _, point := range doc.Series[0].Points {
		values = append(values, point.Value)
	}
	assert.InDeltaSlice(t, []float64{100, 140, 140, 140}, values, 1e-9)
	assert.Contains(t, doc.Warnings, "growth cap 1.4 limited 3 months to 140.00")
	assert.InDeltaSlice(t, []float64{100, 110, 120, 130}, []float64{
		doc.Series[1].Points[0].Value, doc.Series[1].Points[1].Value,
		doc.Series[1].Points[2].Value, doc.Series[1].Points[3].Value,
	}, 1e-9, "provider lines stay undriven")
}

func TestRunForecast_ArimaExcludeAndStep(t *testing.T) {
	t.Parallel()
	path := writeSpendCSV(t, spendCSV())
	base, err := runArimaForecast(t, arimaCosts(),
		"--model", "arima", "--spend-history", path, "--months", "2", "--output", "json")
	require.NoError(t, err)
	stepped, err := runArimaForecast(t, arimaCosts(),
		"--model", "arima", "--spend-history", path, "--months", "2", "--output", "json",
		"--step", "2026-11-01=500", "--exclude", "2026-06-01:2026-06-07")
	require.NoError(t, err)
	var a, b forecastJSON
	require.NoError(t, json.Unmarshal([]byte(base), &a))
	require.NoError(t, json.Unmarshal([]byte(stepped), &b))
	assert.Contains(t, b.Warnings, "spend history: interpolated 7 points in excluded ranges")
	assert.Greater(t, b.Series[0].Points[1].Value-a.Series[0].Points[1].Value, 400.0)
	spend := b.Series[len(b.Series)-1]
	assert.Equal(t, forecast.SeriesSpend, spend.Name)
	assert.Len(t, spend.Points, 180, "the spend series is the input, before exclusion")
}
