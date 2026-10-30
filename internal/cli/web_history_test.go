package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/webui"
)

const historySecretType = "aws:secretsmanager/secret:Secret"

func historyWebData(dir, stack string, cfg *config.Config) *webData {
	if cfg == nil {
		cfg = &config.Config{}
	}
	data := newWebData(nil, overviewParams{stack: stack, cfg: cfg}, engine.DateRange{}, nil)
	data.historyDirOverride = dir
	return data
}

// seedParityHistory writes three USD snapshots and one EUR snapshot. A
// resource type with a credential word in it checks that breakdown keys
// survive the web redactor.
func seedParityHistory(t *testing.T, dir string) {
	t.Helper()
	first := viewSnap(1, time.January, 10, map[string]float64{"aws": 10})
	first.ByType = map[string]float64{"aws:s3/bucket:Bucket": 10}
	first.Resources = []history.CostResource{
		{URN: "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket", MonthlyCost: 10},
	}
	second := viewSnap(2, time.February, 40.4, map[string]float64{"aws": 40.4})
	second.ByType = map[string]float64{"aws:s3/bucket:Bucket": 40, historySecretType: 0.4}
	second.Resources = []history.CostResource{
		{URN: "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket", MonthlyCost: 40},
		{URN: "urn:pulumi:dev::app::" + historySecretType + "::db", Type: historySecretType, MonthlyCost: 0.4},
	}
	third := viewSnap(3, time.March, 0.4, map[string]float64{"aws": 0.4})
	third.ByType = map[string]float64{historySecretType: 0.4}
	third.Resources = second.Resources[1:]
	eur := viewSnap(4, time.April, 9, map[string]float64{"aws": 9})
	eur.Currency = "EUR"
	seedHistory(t, dir, "dev", first, second, third, eur)
}

func redacted(t *testing.T, value any) string {
	t.Helper()
	data, err := webui.RedactJSON(value)
	require.NoError(t, err)
	return string(data)
}

func TestWebHistoryView_MatchesCLIJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedParityHistory(t, dir)
	data := historyWebData(dir, "dev", nil)

	cases := []struct {
		name  string
		args  []string
		query webui.HistoryViewQuery
	}{
		{name: "all"},
		{
			name:  "range",
			args:  []string{"--from", "2025-02-01", "--to", "2025-03-31"},
			query: webui.HistoryViewQuery{From: "2025-02-01", To: "2025-03-31"},
		},
		{name: "currency", args: []string{"--currency", "eur"}, query: webui.HistoryViewQuery{Currency: "eur"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--stack", "dev", "--output", "json"}, tc.args...)
			cmd, stdout := preparedHistoryCmd(t, NewCostHistoryViewCmd(), args...)
			require.NoError(t, runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}}))

			result, err := data.historyView(context.Background(), tc.query)

			require.NoError(t, err)
			assert.JSONEq(t, stdout.String(), redacted(t, result.Document))
		})
	}

	result, err := data.historyView(context.Background(), webui.HistoryViewQuery{})
	require.NoError(t, err)
	assert.Contains(t, result.Document.CurrencyWarning, "Showing USD snapshots only.")
	assert.Contains(t, redacted(t, result.Document), historySecretType)
	assert.Nil(t, result.Budget)
}

func TestWebHistoryView_RefusesLikeCLI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedParityHistory(t, dir)
	data := historyWebData(dir, "dev", nil)

	cases := []struct {
		name  string
		args  []string
		query webui.HistoryViewQuery
	}{
		{name: "strict", args: []string{"--strict"}, query: webui.HistoryViewQuery{Strict: true}},
		{name: "bad date", args: []string{"--from", "soon"}, query: webui.HistoryViewQuery{From: "soon"}},
		{
			name:  "reversed",
			args:  []string{"--from", "2025-03-01", "--to", "2025-01-01"},
			query: webui.HistoryViewQuery{From: "2025-03-01", To: "2025-01-01"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--stack", "dev", "--output", "json"}, tc.args...)
			cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd(), args...)
			cliErr := runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}})
			require.Error(t, cliErr)

			_, err := data.historyView(context.Background(), tc.query)

			require.Error(t, err)
			assert.True(t, history.IsQueryError(err))
			assert.Equal(t, cliErr.Error(), err.Error())
		})
	}
}

func TestWebHistoryView_CarriesGlobalBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedParityHistory(t, dir)
	cfg := &config.Config{Cost: config.CostConfig{Budgets: &config.BudgetsConfig{
		Global: &config.ScopedBudget{Amount: 150, Currency: "USD"},
	}}}

	result, err := historyWebData(dir, "dev", cfg).historyView(context.Background(), webui.HistoryViewQuery{})

	require.NoError(t, err)
	require.NotNil(t, result.Budget)
	amount, ok := globalBudget(cfg)
	require.True(t, ok)
	assert.InDelta(t, amount, *result.Budget, 0)
	assert.Equal(t, "USD", result.BudgetCurrency)
}

func TestWebHistoryDiff_MatchesCLIJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedParityHistory(t, dir)
	data := historyWebData(dir, "dev", nil)

	cases := []struct {
		name  string
		args  []string
		query webui.HistoryDiffQuery
	}{
		{
			name:  "versions",
			args:  []string{"--from", "v1", "--to", "v2"},
			query: webui.HistoryDiffQuery{From: "v1", To: "v2"},
		},
		{name: "default to", args: []string{"--from", "2025-04-20"}, query: webui.HistoryDiffQuery{From: "2025-04-20"}},
		{
			name:  "date and threshold",
			args:  []string{"--from", "2025-02-14", "--to", "v3", "--threshold", "1"},
			query: webui.HistoryDiffQuery{From: "2025-02-14", To: "v3", Threshold: 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--stack", "dev", "--output", "json"}, tc.args...)
			cmd, stdout := preparedHistoryCmd(t, NewCostHistoryDiffCmd(), args...)
			require.NoError(t, runDiff(cmd, dir))

			doc, err := data.historyDiff(context.Background(), tc.query)

			require.NoError(t, err)
			assert.JSONEq(t, stdout.String(), redacted(t, doc))
		})
	}
}

func TestWebHistoryDiff_RefusesLikeCLI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedParityHistory(t, dir)
	data := historyWebData(dir, "dev", nil)

	cases := []struct {
		name  string
		args  []string
		query webui.HistoryDiffQuery
	}{
		{name: "missing from", query: webui.HistoryDiffQuery{}},
		{name: "negative threshold", args: []string{"--from", "v1", "--threshold", "-1"},
			query: webui.HistoryDiffQuery{From: "v1", Threshold: -1}},
		{name: "unknown version", args: []string{"--from", "v9"}, query: webui.HistoryDiffQuery{From: "v9"}},
		{name: "bad selector", args: []string{"--from", "last week"}, query: webui.HistoryDiffQuery{From: "last week"}},
		{name: "two currencies", args: []string{"--from", "v1", "--to", "v4"},
			query: webui.HistoryDiffQuery{From: "v1", To: "v4"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--stack", "dev", "--output", "json"}, tc.args...)
			cmd, _ := preparedHistoryCmd(t, NewCostHistoryDiffCmd(), args...)
			cliErr := runDiff(cmd, dir)
			require.Error(t, cliErr)

			_, err := data.historyDiff(context.Background(), tc.query)

			require.Error(t, err)
			assert.True(t, history.IsQueryError(err), "%v", err)
			assert.Equal(t, cliErr.Error(), err.Error())
		})
	}
}

func TestWebHistoryResource_MatchesDetailLoader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	urn := "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"
	seedScopedHistory(t, dir, "app", "dev",
		history.CostSnapshot{
			Timestamp: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Version: 3, TotalMonthly: 12, Currency: "USD",
			Resources: []history.CostResource{{URN: urn, Type: "aws:ec2/instance:Instance", MonthlyCost: 12}},
		},
		history.CostSnapshot{
			Timestamp: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Version: 4, TotalMonthly: 20, Currency: "USD",
			Resources: []history.CostResource{{URN: urn, Type: "aws:ec2/instance:Instance", MonthlyCost: 20}},
		},
	)
	data := historyWebData(dir, "state", nil)

	want, err := loadDetailHistory(context.Background(), dir, "state", urn)
	require.NoError(t, err)
	got, err := data.historyResource(context.Background(), urn)
	require.NoError(t, err)

	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, webui.HistoryPoint{
			Timestamp: want[i].Timestamp, Amount: want[i].Amount, Currency: want[i].Currency,
		}, got[i])
	}

	unmentioned, err := data.historyResource(context.Background(), "urn:pulumi:dev::app::aws:s3/bucket:Bucket::x")
	require.NoError(t, err)
	assert.Empty(t, unmentioned)
}

func TestWebHistory_NoHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, data := range map[string]*webData{
		"missing database": historyWebData(t.TempDir(), "dev", nil),
		"no stack":         historyWebData(t.TempDir(), "", nil),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := data.historyView(ctx, webui.HistoryViewQuery{})
			assert.True(t, history.IsNoHistory(err), "view: %v", err)
			_, err = data.historyDiff(ctx, webui.HistoryDiffQuery{From: "v1"})
			assert.True(t, history.IsNoHistory(err), "diff: %v", err)
			_, err = data.historyResource(ctx, "i-0abc")
			assert.True(t, history.IsNoHistory(err), "resource: %v", err)
		})
	}

	emptyDir := t.TempDir()
	seedHistory(t, emptyDir, "dev")
	_, err := historyWebData(emptyDir, "dev", nil).historyDiff(ctx, webui.HistoryDiffQuery{From: "v1"})
	require.ErrorIs(t, err, history.ErrNoSnapshots)
}

func TestWebHistory_CanceledContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedParityHistory(t, dir)
	data := historyWebData(dir, "dev", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := data.historyView(ctx, webui.HistoryViewQuery{})
	require.ErrorIs(t, err, context.Canceled)
	_, err = data.historyDiff(ctx, webui.HistoryDiffQuery{From: "v1"})
	require.ErrorIs(t, err, context.Canceled)
	_, err = data.historyResource(ctx, "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs")
	require.ErrorIs(t, err, context.Canceled)
}

func TestWebHistory_CorruptDatabaseIsAFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := historyPath(dir, "dev")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("not a bolt database"), 0o600))
	scoped, err := history.CostFileNameFor("app", "dev")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, scoped), []byte("not a bolt database"), 0o600))
	data := historyWebData(dir, "dev", nil)
	ctx := context.Background()

	_, viewErr := data.historyView(ctx, webui.HistoryViewQuery{})
	_, diffErr := data.historyDiff(ctx, webui.HistoryDiffQuery{From: "v1"})
	_, resourceErr := data.historyResource(ctx, "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs")
	for name, err := range map[string]error{"view": viewErr, "diff": diffErr, "resource": resourceErr} {
		require.Error(t, err, name)
		assert.False(t, history.IsNoHistory(err), name)
		assert.False(t, history.IsQueryError(err), name)
	}
}

func TestWebSessionOptions_WiresHistory(t *testing.T) {
	t.Parallel()
	options := webSessionOptions(context.Background(), historyWebData(t.TempDir(), "dev", nil), nil, nil, nil)
	assert.NotNil(t, options.HistoryView)
	assert.NotNil(t, options.HistoryDiff)
	assert.NotNil(t, options.HistoryResource)
}

func TestWebHistoryView_NoHistoryBeforeDateErrorsLikeCLI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd(), "--stack", "dev", "--from", "soon")
	cliErr := runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}})
	require.ErrorIs(t, cliErr, history.ErrNoCostHistory)

	_, err := historyWebData(dir, "dev", nil).historyView(context.Background(), webui.HistoryViewQuery{From: "soon"})

	require.ErrorIs(t, err, history.ErrNoCostHistory)
	assert.Equal(t, cliErr.Error(), err.Error())
}
