package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/forecast"
	"github.com/rshade/finfocus/internal/history"
)

func TestCostForecastCmd_Name(t *testing.T) {
	t.Parallel()
	cmd := NewCostForecastCmd()
	assert.Equal(t, "forecast", cmd.Name())
	assert.NotNil(t, cmd.Flags().Lookup("months"))
	assert.NotNil(t, cmd.Flags().Lookup("growth-rate"))
}

func TestRunForecast_RejectsFlagsBeforePricing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want error
		text string
	}{
		{name: "months", args: []string{"--months", "0"}, want: forecast.ErrInvalidMonths},
		{name: "months high", args: []string{"--months", "37"}, want: forecast.ErrInvalidMonths},
		{name: "type without rate", args: []string{"--growth-type", "linear"}, want: forecast.ErrMissingGrowthRate},
		{name: "rate too low", args: []string{"--growth-rate", "-1.1"}, want: forecast.ErrInvalidGrowthRate},
		{name: "bad type", args: []string{"--growth-type", "quadratic"}, text: "invalid growth type"},
		{name: "bad rate", args: []string{"--growth-rate", "fast"}, text: "invalid growth rate"},
		{name: "bad output", args: []string{"--output", "yaml"}, text: "unsupported output format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCostForecastCmd()
			require.NoError(t, cmd.ParseFlags(tc.args))
			err := runForecast(cmd, forecastDeps{skipPrice: true, skipHistory: true})
			require.Error(t, err)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			}
			if tc.text != "" {
				assert.Contains(t, err.Error(), tc.text)
			}
		})
	}
}

func TestRunForecast_JSONProjectsEachResource(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, "--months", "1", "--growth-rate", "0.10", "--output", "json")
	err := runForecast(cmd, forecastDeps{
		now:         time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		cfg:         &config.Config{},
		skipPrice:   true,
		skipHistory: true,
		costs: []engine.CostResult{
			{ResourceID: "ec2", ResourceType: "aws:ec2:Instance", Monthly: 100, Currency: "USD", GrowthType: "none"},
			{ResourceID: "bucket", ResourceType: "aws:s3:Bucket", Monthly: 100, Currency: "USD", GrowthType: "linear"},
			{ResourceID: "bad", ResourceType: "aws:ec2:Instance", Notes: "ERROR: plugin down"},
		},
	})
	require.NoError(t, err)

	var doc forecastJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	assert.Equal(t, "USD", doc.Currency)
	assert.Equal(t, 1, doc.Months)
	assert.Nil(t, doc.Budget)
	require.Len(t, doc.Series, 2)
	assert.Equal(t, "forecast", doc.Series[0].Name)
	require.Len(t, doc.Series[0].Points, 2)
	assert.InDelta(t, 200, doc.Series[0].Points[0].Value, 1e-9)
	assert.InDelta(t, 210, doc.Series[0].Points[1].Value, 1e-9)
	assert.Equal(t, "aws", doc.Series[1].Name)
	assert.Contains(t, doc.Warnings[0], "bad")
}

func TestRunForecast_HistoryAndBudget(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, "--months", "1", "--output", "json")
	err := runForecast(cmd, forecastDeps{
		now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		cfg: &config.Config{
			Cost: config.CostConfig{Budgets: &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 250}}},
		},
		skipPrice:   true,
		skipHistory: true,
		history: []history.CostSnapshot{
			{Timestamp: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), TotalMonthly: 80, Currency: "USD"},
			{Timestamp: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), TotalMonthly: 90, Currency: "usd"},
		},
		costs: []engine.CostResult{
			{ResourceID: "ec2", ResourceType: "aws:ec2:Instance", Monthly: 100, Currency: "USD"},
		},
	})
	require.NoError(t, err)
	var doc forecastJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	require.NotNil(t, doc.Budget)
	assert.InDelta(t, 250, *doc.Budget, 1e-9)
	require.GreaterOrEqual(t, len(doc.Series), 2)
	assert.Equal(t, "history", doc.Series[0].Name)
	assert.InDelta(t, 80, doc.Series[0].Points[0].Value, 1e-9)
	assert.InDelta(t, 90, doc.Series[0].Points[1].Value, 1e-9)
	assert.Equal(t, "forecast", doc.Series[1].Name)
}

func TestRunForecast_HistoryCurrencyMismatch(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, "--months", "1", "--output", "json")
	err := runForecast(cmd, forecastDeps{
		now:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		cfg:         &config.Config{},
		skipPrice:   true,
		skipHistory: true,
		history:     []history.CostSnapshot{{Timestamp: time.Now(), TotalMonthly: 10, Currency: "EUR"}},
		costs:       []engine.CostResult{{ResourceID: "ec2", Monthly: 10, Currency: "USD"}},
	})
	require.NoError(t, err)
	var doc forecastJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	require.NotEmpty(t, doc.Series)
	assert.Equal(t, "forecast", doc.Series[0].Name)
	assert.Contains(t, doc.Warnings[0], "EUR")
}

func TestRunForecast_NDJSONAndPlain(t *testing.T) {
	t.Parallel()
	var ndjson bytes.Buffer
	cmd := forecastTestCmd(t, &ndjson, "--months", "1", "--output", "ndjson", "--no-history")
	err := runForecast(cmd, forecastDeps{
		now:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		cfg:         &config.Config{},
		skipPrice:   true,
		skipHistory: true,
		costs: []engine.CostResult{
			{ResourceID: "ec2", ResourceType: "aws:ec2:Instance", Monthly: 5, Currency: "USD"},
		},
	})
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSpace(ndjson.Bytes()), []byte("\n"))
	require.GreaterOrEqual(t, len(lines), 3)
	assert.Contains(t, string(lines[0]), `"kind":"meta"`)
	assert.Contains(t, string(lines[1]), `"series":"forecast"`)

	var plain bytes.Buffer
	plainCmd := forecastTestCmd(
		t,
		&plain,
		"--months",
		"1",
		"--output",
		"plain",
		"--no-color",
		"--width",
		"40",
		"--height",
		"6",
	)
	err = runForecast(plainCmd, forecastDeps{
		now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		cfg: &config.Config{
			Cost: config.CostConfig{Budgets: &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 20}}},
		},
		skipPrice:   true,
		skipHistory: true,
		costs:       []engine.CostResult{{ResourceID: "ec2", Monthly: 5, Currency: "USD"}},
	})
	require.NoError(t, err)
	text := plain.String()
	assert.Contains(t, text, "Forecast monthly cost (USD)")
	assert.Contains(t, text, "Jan 2026")
	assert.Contains(t, text, "Feb 2026")
	assert.Contains(t, text, "Legend: Forecast  Budget")
	assert.NotContains(t, text, "\x1b[")
}

func TestRunForecast_NoPricedResource(t *testing.T) {
	t.Parallel()
	cmd := NewCostForecastCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--months", "1"}))
	err := runForecast(cmd, forecastDeps{
		cfg:         &config.Config{},
		skipPrice:   true,
		skipHistory: true,
		costs:       []engine.CostResult{{ResourceID: "bad", Notes: "VALIDATION: missing sku"}},
	})
	require.ErrorIs(t, err, forecast.ErrNoCostData)
}

func TestRunForecast_BudgetCurrencyMismatchOmitsLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, "--months", "1", "--output", "json", "--no-history")
	err := runForecast(cmd, forecastDeps{
		now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		cfg: &config.Config{
			Cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 250, Currency: "USD"}},
			},
		},
		skipPrice:   true,
		skipHistory: true,
		costs:       []engine.CostResult{{ResourceID: "vm", Monthly: 10, Currency: "EUR"}},
	})
	require.NoError(t, err)
	var doc forecastJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	assert.Nil(t, doc.Budget)
	assert.Equal(t, "EUR", doc.Currency)
	require.NotEmpty(t, doc.Warnings)
	assert.Contains(t, doc.Warnings[0], "USD")
	assert.Contains(t, doc.Warnings[0], "EUR")
}

func TestRunForecast_PlainOverridesJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, "--months", "1", "--output", "json", "--plain", "--no-history")
	err := runForecast(cmd, forecastDeps{
		now:         time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		cfg:         &config.Config{},
		skipPrice:   true,
		skipHistory: true,
		costs:       []engine.CostResult{{Monthly: 3, Currency: "USD"}},
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Forecast monthly cost")
	assert.NotContains(t, buf.String(), `"series"`)
}

func forecastTestCmd(t *testing.T, buf *bytes.Buffer, args ...string) *cobra.Command {
	t.Helper()
	cmd := NewCostForecastCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(t.Context())
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}
