package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/forecast"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/webui"
)

func forecastWebNow() time.Time { return time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC) }

func forecastWebCosts() []engine.CostResult {
	return []engine.CostResult{
		{ResourceID: "ec2", ResourceType: "aws:ec2:Instance", Monthly: 100, Currency: "USD", GrowthType: "none"},
		{ResourceID: "bucket", ResourceType: "aws:s3:Bucket", Monthly: 40, Currency: "USD", GrowthType: "linear"},
		{
			ResourceID:   "disk",
			ResourceType: "azure:compute:Disk",
			Monthly:      25,
			Currency:     "USD",
			GrowthType:   "exponential",
		},
		{ResourceID: "bad", ResourceType: "aws:ec2:Instance", Notes: "ERROR: plugin down"},
	}
}

func forecastWebHistory() []history.CostSnapshot {
	return []history.CostSnapshot{
		{Timestamp: forecastWebNow().AddDate(0, -1, 0), TotalMonthly: 150, Currency: "USD"},
		{Timestamp: forecastWebNow().AddDate(0, -2, 0), TotalMonthly: 140, Currency: "USD"},
	}
}

func forecastWebConfig() *config.Config {
	return &config.Config{
		Cost: config.CostConfig{Budgets: &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 300}}},
	}
}

func newForecastWebData(
	costs []engine.CostResult,
	snapshots []history.CostSnapshot,
	historyErr error,
) (*webData, *[]string) {
	data := newWebData(
		&cobra.Command{},
		overviewParams{cfg: forecastWebConfig(), stack: "dev"},
		engine.DateRange{},
		nil,
	)
	data.forecastNow = forecastWebNow()
	data.priceForecast = func(context.Context) ([]engine.CostResult, error) { return costs, nil }
	var stacks []string
	data.readHistory = func(stack string) ([]history.CostSnapshot, error) {
		stacks = append(stacks, stack)
		return snapshots, historyErr
	}
	return data, &stacks
}

func cliForecastDocument(t *testing.T, args ...string) forecastJSON {
	t.Helper()
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, append(args, "--output", "json")...)
	require.NoError(t, runForecast(cmd, forecastDeps{
		now:         forecastWebNow(),
		cfg:         forecastWebConfig(),
		skipPrice:   true,
		costs:       forecastWebCosts(),
		skipHistory: true,
		history:     forecastWebHistory(),
	}))
	var doc forecastJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	return doc
}

func webForecastDocument(t *testing.T, query webui.ForecastQuery) forecastJSON {
	t.Helper()
	data, _ := newForecastWebData(forecastWebCosts(), forecastWebHistory(), nil)
	doc, err := data.forecast(t.Context(), query)
	require.NoError(t, err)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	var decoded forecastJSON
	require.NoError(t, json.Unmarshal(raw, &decoded))
	return decoded
}

func intPtr(v int) *int { return &v }

func spendPtr(v string) *string { return &v }

func TestWebForecast_MatchesCLIJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		args  []string
		query webui.ForecastQuery
	}{
		{name: "defaults", args: nil, query: webui.ForecastQuery{}},
		{name: "plugin growth with rate", args: []string{"--months", "6", "--growth-rate", "0.05"},
			query: webui.ForecastQuery{Months: intPtr(6), GrowthRate: "0.05"}},
		{name: "linear", args: []string{"--months", "3", "--growth-type", "linear", "--growth-rate", "0.10"},
			query: webui.ForecastQuery{Months: intPtr(3), GrowthType: "linear", GrowthRate: "0.10"}},
		{name: "exponential", args: []string{"--months", "36", "--growth-type", "exponential", "--growth-rate", "0.05"},
			query: webui.ForecastQuery{Months: intPtr(36), GrowthType: "exponential", GrowthRate: "0.05"}},
		{name: "none with rate", args: []string{"--growth-type", "none", "--growth-rate", "0.2"},
			query: webui.ForecastQuery{GrowthType: "none", GrowthRate: "0.2"}},
		{name: "no history", args: []string{"--no-history"}, query: webui.ForecastQuery{NoHistory: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := cliForecastDocument(t, tc.args...)
			got := webForecastDocument(t, tc.query)
			assert.Equal(t, want, got)
			require.NotNil(t, got.Budget)
			assert.NotEmpty(t, got.Warnings)
		})
	}
}

func TestWebForecast_InputErrorsMatchCLI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		args  []string
		query webui.ForecastQuery
	}{
		{name: "months zero", args: []string{"--months", "0"}, query: webui.ForecastQuery{Months: intPtr(0)}},
		{name: "months high", args: []string{"--months", "37"}, query: webui.ForecastQuery{Months: intPtr(37)}},
		{
			name:  "unknown type",
			args:  []string{"--growth-type", "cubic"},
			query: webui.ForecastQuery{GrowthType: "cubic"},
		},
		{name: "bad rate", args: []string{"--growth-rate", "ten"}, query: webui.ForecastQuery{GrowthRate: "ten"}},
		{name: "rate below -1", args: []string{"--growth-rate", "-2"}, query: webui.ForecastQuery{GrowthRate: "-2"}},
		{name: "linear without rate", args: []string{"--growth-type", "linear"},
			query: webui.ForecastQuery{GrowthType: "linear"}},
		{name: "exponential without rate", args: []string{"--growth-type", "exponential"},
			query: webui.ForecastQuery{GrowthType: "exponential"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			cmd := forecastTestCmd(t, &buf, tc.args...)
			cliErr := runForecast(cmd, forecastDeps{skipPrice: true, skipHistory: true})
			require.Error(t, cliErr)

			data, _ := newForecastWebData(nil, nil, nil)
			priced := false
			data.priceForecast = func(context.Context) ([]engine.CostResult, error) {
				priced = true
				return nil, nil
			}
			_, webErr := data.forecast(t.Context(), tc.query)
			require.Error(t, webErr)
			assert.True(t, forecast.IsInputError(webErr))
			assert.Equal(t, cliErr.Error(), webErr.Error())
			assert.False(t, priced, "input errors are rejected before pricing")
		})
	}
}

func TestWebForecast_History(t *testing.T) {
	t.Parallel()
	t.Run("present", func(t *testing.T) {
		t.Parallel()
		data, stacks := newForecastWebData(forecastWebCosts(), forecastWebHistory(), nil)
		doc, err := data.forecast(t.Context(), webui.ForecastQuery{})
		require.NoError(t, err)
		require.NotEmpty(t, doc.Series)
		assert.Equal(t, forecast.SeriesHistory, doc.Series[0].Name)
		assert.Equal(t, []string{"dev"}, *stacks)
	})
	t.Run("missing database", func(t *testing.T) {
		t.Parallel()
		data, _ := newForecastWebData(forecastWebCosts(), nil, nil)
		doc, err := data.forecast(t.Context(), webui.ForecastQuery{})
		require.NoError(t, err)
		assert.Equal(t, forecast.SeriesForecast, doc.Series[0].Name)
	})
	t.Run("read error is a warning", func(t *testing.T) {
		t.Parallel()
		data, _ := newForecastWebData(forecastWebCosts(), nil, errors.New("locked"))
		doc, err := data.forecast(t.Context(), webui.ForecastQuery{})
		require.NoError(t, err)
		assert.Contains(t, doc.Warnings, "history: locked")
	})
	t.Run("other currency", func(t *testing.T) {
		t.Parallel()
		data, _ := newForecastWebData(forecastWebCosts(), []history.CostSnapshot{
			{Timestamp: forecastWebNow().AddDate(0, -1, 0), TotalMonthly: 90, Currency: "EUR"},
		}, nil)
		doc, err := data.forecast(t.Context(), webui.ForecastQuery{})
		require.NoError(t, err)
		assert.Equal(t, forecast.SeriesForecast, doc.Series[0].Name)
		assert.Contains(t, doc.Warnings, "history currency EUR does not match forecast currency USD")
	})
	t.Run("no history skips the read", func(t *testing.T) {
		t.Parallel()
		data, stacks := newForecastWebData(forecastWebCosts(), forecastWebHistory(), nil)
		doc, err := data.forecast(t.Context(), webui.ForecastQuery{NoHistory: true})
		require.NoError(t, err)
		assert.Equal(t, forecast.SeriesForecast, doc.Series[0].Name)
		assert.Empty(t, *stacks)
	})
}

func TestWebForecast_PricingFailureIsNotInputError(t *testing.T) {
	t.Parallel()
	data, _ := newForecastWebData(nil, nil, nil)
	data.priceForecast = func(context.Context) ([]engine.CostResult, error) {
		return nil, errors.New("plugin unavailable")
	}
	_, err := data.forecast(t.Context(), webui.ForecastQuery{})
	require.Error(t, err)
	assert.False(t, forecast.IsInputError(err))
}

func TestWebForecast_NoPricedResourceMatchesCLI(t *testing.T) {
	t.Parallel()
	costs := []engine.CostResult{{ResourceID: "bad", Notes: "VALIDATION: missing sku"}}
	var buf bytes.Buffer
	cmd := forecastTestCmd(t, &buf, "--months", "1")
	cliErr := runForecast(cmd, forecastDeps{cfg: &config.Config{}, skipPrice: true, skipHistory: true, costs: costs})
	require.ErrorIs(t, cliErr, forecast.ErrNoCostData)

	data, _ := newForecastWebData(costs, nil, nil)
	_, webErr := data.forecast(t.Context(), webui.ForecastQuery{Months: intPtr(1)})
	require.ErrorIs(t, webErr, forecast.ErrNoCostData)
	assert.True(t, forecast.IsUnprojectable(webErr))
	assert.Equal(t, cliErr.Error(), webErr.Error())
}

func TestWebForecast_SessionStackPrefersSnapshot(t *testing.T) {
	t.Parallel()
	data, stacks := newForecastWebData(forecastWebCosts(), nil, nil)
	publishWebTestState(t, data, nil)
	data.snapshot.StackName = "prod"
	_, err := data.forecast(t.Context(), webui.ForecastQuery{})
	require.NoError(t, err)
	assert.Equal(t, []string{"prod"}, *stacks)
}

func TestWebForecast_PricesPlanMembershipWithSessionEngine(t *testing.T) {
	t.Parallel()
	const (
		changed   = "urn:pulumi:dev::test::aws:ec2/instance:Instance::changed"
		unchanged = "urn:pulumi:dev::test::aws:ec2/instance:Instance::unchanged"
		created   = "urn:pulumi:dev::test::aws:s3/bucket:Bucket::created"
	)
	data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}}, engine.DateRange{}, nil)
	publishWebTestState(t, data, []ingest.StackExportResource{
		{URN: changed, Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.small"}},
		{URN: unchanged, Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.micro"}},
	})
	publishWebTestPlan(t.Context(), t, data, &ingest.PulumiPlan{Steps: []ingest.PulumiStep{
		{
			Op:  "update",
			URN: changed,
			OldState: &ingest.PulumiState{
				Type:   "aws:ec2/instance:Instance",
				Inputs: map[string]any{"instanceType": "t3.small"},
			},
			NewState: &ingest.PulumiState{
				Type:   "aws:ec2/instance:Instance",
				Inputs: map[string]any{"instanceType": "m5.large"},
			},
		},
		{Op: "create", URN: created,
			NewState: &ingest.PulumiState{Type: "aws:s3/bucket:Bucket", Inputs: map[string]any{}}},
	}})

	resources, err := data.sessionForecastResources(t.Context())
	require.NoError(t, err)
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.ID)
		assert.NotEqual(t, engine.OriginState, resource.Origin)
	}
	assert.ElementsMatch(t, []string{changed, created}, ids)
	assert.NotContains(t, ids, unchanged)

	_, err = data.sessionForecastCosts(t.Context())
	require.EqualError(t, err, "overview data unavailable", "without a session engine no plugin is opened")
}

func TestWebForecast_NoSource(t *testing.T) {
	t.Parallel()
	data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}}, engine.DateRange{}, nil)
	data.eng = engine.New(nil, nil)
	_, err := data.forecast(t.Context(), webui.ForecastQuery{})
	require.EqualError(t, err, "stack source unavailable")
}

func TestWebForecast_ArimaMatchesCLIJSON(t *testing.T) {
	t.Parallel()
	csv := spendCSV()
	path := writeSpendCSV(t, csv)
	confidence, growthCap := 0.8, 1.3
	cliOut, err := runArimaForecast(t, arimaCosts(),
		"--model", "arima", "--spend-history", path, "--months", "4", "--confidence", "0.8",
		"--growth-cap", "1.3", "--step", "2026-12-01=200", "--exclude", "2026-06-01:2026-06-07",
		"--no-history", "--output", "json")
	require.NoError(t, err)
	var want forecastJSON
	require.NoError(t, json.Unmarshal([]byte(cliOut), &want))

	data, _ := newForecastWebData(arimaCosts(), nil, nil)
	data.params.cfg = &config.Config{}
	data.forecastNow = arimaNow()
	doc, err := data.forecast(t.Context(), webui.ForecastQuery{
		Months: intPtr(4), Model: "arima", SpendHistory: &csv, Confidence: &confidence, GrowthCap: &growthCap,
		Steps: []string{"2026-12-01=200"}, Exclude: []string{"2026-06-01:2026-06-07"}, NoHistory: true,
	})
	require.NoError(t, err)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	var got forecastJSON
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, want, got)
	assert.Equal(t, forecast.ModelARIMA, got.Model)
}

func TestWebForecast_ArimaInputErrorsMatchCLI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		args  []string
		query webui.ForecastQuery
	}{
		{name: "arima without history", args: []string{"--model", "arima"},
			query: webui.ForecastQuery{Model: "arima"}},
		{name: "history without arima", args: []string{"--spend-history", "PATH"},
			query: webui.ForecastQuery{SpendHistory: spendPtr("2026-04-01,1\n")}},
		{name: "bad row", args: []string{"--model", "arima", "--spend-history", "PATH"},
			query: webui.ForecastQuery{Model: "arima", SpendHistory: spendPtr("2026-04-01,1\n2026-04-02,x\n")}},
		{name: "empty file", args: []string{"--model", "arima", "--spend-history", "PATH"},
			query: webui.ForecastQuery{Model: "arima", SpendHistory: spendPtr("")}},
		{
			name: "too short after exclusion",
			args: []string{"--model", "arima", "--spend-history", "PATH", "--exclude", "2026-04-01:2026-04-03"},
			query: webui.ForecastQuery{
				Model:        "arima",
				SpendHistory: spendPtr(shortSpendCSV),
				Exclude:      []string{"2026-04-01:2026-04-03"},
			},
		},
		{name: "bad step", args: []string{"--step", "x"}, query: webui.ForecastQuery{Steps: []string{"x"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string(nil), tc.args...)
			for i, arg := range args {
				if arg == "PATH" {
					args[i] = writeSpendCSV(t, *tc.query.SpendHistory)
				}
			}
			var buf bytes.Buffer
			cliErr := runForecast(forecastTestCmd(t, &buf, args...), forecastDeps{skipPrice: true, skipHistory: true})
			require.Error(t, cliErr)
			data, _ := newForecastWebData(nil, nil, nil)
			_, webErr := data.forecast(t.Context(), tc.query)
			require.Error(t, webErr)
			assert.True(t, forecast.IsInputError(webErr))
			assert.Equal(t, cliErr.Error(), webErr.Error())
		})
	}
}

// shortSpendCSV is twelve days, which leaves nine usable points once the
// first three are excluded.
const shortSpendCSV = "2026-04-01,1\n2026-04-02,2\n2026-04-03,3\n2026-04-04,4\n2026-04-05,5\n2026-04-06,6\n" +
	"2026-04-07,7\n2026-04-08,8\n2026-04-09,9\n2026-04-10,10\n2026-04-11,11\n2026-04-12,12\n"
