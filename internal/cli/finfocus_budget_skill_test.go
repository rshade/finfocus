package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestFinfocusBudgetSkillDocumentsLiveContract(t *testing.T) {
	t.Parallel()

	cost := newCostCmd()
	overview := NewOverviewCmd()
	assertBudgetFlags(t, cost, true)
	assertBudgetFlags(t, overview, false)
	assert.Equal(t,
		cost.PersistentFlags().Lookup("exit-code").DefValue,
		overview.Flags().Lookup("exit-code").DefValue,
	)
	assert.Contains(t, overview.Flags().Lookup("exit-on-threshold").Usage, "non-TTY")

	names := map[string]bool{}
	for _, sub := range cost.Commands() {
		names[sub.Name()] = true
	}
	assert.True(t, names["projected"], "cost projected is a real subcommand")
	assert.True(t, names["actual"], "cost actual is a real subcommand")
	assert.Equal(t, "overview", overview.Name())

	docs := readBudgetSkill(t)

	codeDefault := cost.PersistentFlags().Lookup("exit-code").DefValue
	thresholdDefault := cost.PersistentFlags().Lookup("exit-on-threshold").DefValue
	assert.Empty(t, cost.PersistentFlags().Lookup("budget-scope").DefValue)
	assert.Contains(t, docs, "| `--exit-on-threshold` | `"+thresholdDefault+"` |")
	assert.Contains(t, docs, "| `--exit-code` | `"+codeDefault+"` |")
	assert.Contains(t, docs, "--budget-scope")
	assert.Contains(t, docs, "non-TTY")
	assert.Contains(t, docs, "finfocus cost projected")
	assert.Contains(t, docs, "finfocus cost actual")
	assert.Contains(t, docs, "finfocus overview")

	for _, cmd := range []*cobra.Command{
		NewConfigInitCmd(), NewConfigListCmd(), NewConfigGetCmd(), NewConfigSetCmd(),
	} {
		assert.Contains(t, docs, "finfocus config "+strings.Fields(cmd.Use)[0])
	}
	require.NotNil(t, NewConfigInitCmd().Flags().Lookup("global"))
	assert.Contains(t, docs, "finfocus config init --global")
	assert.NotContains(t, docs, "finfocus config path")
	assert.NotContains(t, docs, "export FINFOCUS_BUDGET_AMOUNT")
	assert.Contains(t, docs, "There is no `FINFOCUS_BUDGET_AMOUNT`")
	assert.Contains(t, docs, "FINFOCUS_BUDGET_EXIT_ON_THRESHOLD")
	assert.Contains(t, docs, "FINFOCUS_BUDGET_EXIT_CODE")

	for _, sentinel := range []error{
		config.ErrBudgetAmountNegative,
		config.ErrUnsupportedBudgetPeriod,
		config.ErrExitCodeOutOfRange,
		config.ErrGlobalBudgetRequired,
		config.ErrCurrencyMismatch,
		config.ErrInvalidTagSelector,
	} {
		assert.Contains(t, docs, sentinel.Error())
	}

	warnings, err := (&config.BudgetsConfig{
		Global: &config.ScopedBudget{Amount: 100, Currency: "USD"},
		Tags: []config.TagBudget{{
			ScopedBudget: config.ScopedBudget{Amount: 10},
			Selector:     "team:platform",
		}},
	}).Validate()
	require.NoError(t, err)
	require.NotEmpty(t, warnings)
	assert.Contains(t, docs, warnings[len(warnings)-1])

	sample := "provider=aws,tag=team:platform,type=aws:ec2/instance:Instance"
	filter := NewBudgetScopeFilter(sample)
	require.Equal(t, []string{"aws"}, filter.ProviderFilter)
	require.Equal(t, []string{"team:platform"}, filter.TagFilter)
	require.Equal(t, []string{"aws:ec2/instance:Instance"}, filter.TypeFilter)
	for _, part := range strings.Split(sample, ",") {
		assert.Contains(t, docs, part)
	}

	warn := strconv.FormatFloat(engine.HealthThresholdWarning, 'f', -1, 64)
	crit := strconv.FormatFloat(engine.HealthThresholdCritical, 'f', -1, 64)
	exceeded := strconv.FormatFloat(engine.HealthThresholdExceeded, 'f', -1, 64)
	assert.Contains(t, docs, "OK below "+warn+"%")
	assert.Contains(t, docs, "WARNING from "+warn+"% up to "+crit+"%")
	assert.Contains(t, docs, "CRITICAL from "+crit+"% up to "+exceeded+"%")
	assert.Contains(t, docs, "EXCEEDED at "+exceeded+"%")

	for _, threshold := range engine.DefaultThresholds() {
		pct := strconv.FormatFloat(threshold.GetPercentage(), 'f', -1, 64)
		assert.Contains(t, docs, pct+"% actual")
	}
	buffer := strconv.FormatFloat(engine.ApproachingThresholdBuffer, 'f', -1, 64)
	assert.Contains(t, docs, "within "+buffer+" points")
	assert.Contains(t, docs, "evaluateThreshold")
	assert.Contains(t, docs, "CalculateForecastedSpendAt")
	assert.Contains(t, docs, fmt.Sprintf("exit code %d", engine.ExitCodeBudgetEvaluationError))

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	now := start.Add(10 * 24 * time.Hour)
	forecast := engine.CalculateForecastedSpendAt(100, start, end, now)
	assert.Contains(t, docs, fmt.Sprintf(
		"$100 spent forecasts $%s",
		strconv.FormatFloat(forecast, 'f', -1, 64),
	))
	assert.Contains(t, docs, "(currentSpend / elapsed) * periodLength")
	zero := engine.CalculateForecastedSpendAt(0, start, end, now)
	assert.Contains(t, docs, "Zero spend forecasts "+strconv.FormatFloat(zero, 'f', -1, 64))
	idleLimit := engine.CalculateForecastedPercentage(forecast, 0)
	assert.Contains(t, docs, "yields forecast utilization "+strconv.FormatFloat(idleLimit, 'f', -1, 64))

	when := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	days := time.Date(when.Year(), when.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	monthEngine := engine.NewBudgetEngineWithTime(func() time.Time { return when })
	monthStatus, evalErr := monthEngine.Evaluate(config.BudgetConfig{
		Amount: 1000, Currency: "USD", Period: "monthly",
	}, 300, "USD")
	require.NoError(t, evalErr)
	assert.Contains(t, docs, fmt.Sprintf(
		"On day %d of a %d-day month, $300 spent forecasts $%s.",
		when.Day(), days, strconv.FormatFloat(monthStatus.ForecastedSpend, 'f', -1, 64),
	))
	assert.Contains(t, docs, "(currentSpend / dayOfMonth) * daysInMonth")
	assert.Contains(t, docs, "checkBudgetExit")
	assert.Contains(t, docs, "checkScopedBudgetExit")
	assert.Contains(t, docs, "cost.budgets.exit_on_threshold")
	assert.Contains(t, docs, fmt.Sprintf("scoped gate can fail at %s%% actual", crit))

	liveEngine := engine.NewBudgetEngine()
	liveStatus, liveErr := liveEngine.Evaluate(config.BudgetConfig{
		Amount: 1000, Currency: "USD", Period: "monthly",
	}, 300, "USD")
	require.NoError(t, liveErr)
	scoped := engine.CalculateProviderBudgetStatus("aws", &config.ScopedBudget{
		Amount: 1000, Currency: "USD",
	}, 300)
	assert.InDelta(t, liveStatus.ForecastedSpend, scoped.ForecastedSpend, 1e-9)
	critical := engine.CalculateProviderBudgetStatus("aws", &config.ScopedBudget{
		Amount: 100, Currency: "USD",
		Alerts: []config.AlertConfig{{Threshold: 100, Type: config.AlertTypeForecasted}},
	}, 90)
	assert.Equal(t, engine.CalculateHealthFromPercentage(90), critical.Health)

	cfg := &config.Config{}
	require.NoError(t, cfg.Set("cost.budgets.amount", "10"))
	require.NoError(t, cfg.Set("cost.budgets.currency", "USD"))
	require.NoError(t, cfg.Set("cost.budgets.period", "monthly"))
	for _, key := range []string{"cost.budgets.amount", "cost.budgets.currency", "cost.budgets.period"} {
		assert.Contains(t, docs, "`"+key+"`")
	}
	nestedErr := cfg.Set("cost.budgets.global.amount", "10")
	require.EqualError(t, nestedErr, "invalid cost.budgets key")
	assert.Contains(t, docs, nestedErr.Error())
	alertsErr := cfg.Set("cost.budgets.alerts", "[]")
	require.Error(t, alertsErr)
	assert.Contains(t, docs, alertsErr.Error())

	parentOff := false
	parent := &config.BudgetsConfig{ExitOnThreshold: parentOff}
	assert.False(t, parent.GetEffectiveExitOnThreshold(nil))
	assert.Contains(t, docs, "nil scope override reads `cost.budgets.exit_on_threshold`")
	exitCode := 4
	parent.ExitCode = &exitCode
	assert.Equal(t, exitCode, parent.GetEffectiveExitCode(nil))
	assert.Contains(t, docs, "nil exit-code override reads `cost.budgets.exit_code`")
}

//nolint:paralleltest // SetGlobalConfig replaces the process-wide global config singleton
func TestScopedBudgetExitFollowsParentFlag(t *testing.T) {
	globalOn := true
	globalCode := 7
	cfg := &config.Config{
		Cost: config.CostConfig{
			Budgets: &config.BudgetsConfig{
				ExitOnThreshold: false,
				Global: &config.ScopedBudget{
					Amount:          100,
					Currency:        "USD",
					ExitOnThreshold: &globalOn,
					ExitCode:        &globalCode,
				},
			},
		},
	}
	config.SetGlobalConfig(cfg)
	t.Cleanup(config.ResetGlobalConfigForTest)

	status := engine.CalculateProviderBudgetStatus("aws", &config.ScopedBudget{
		Amount: 100, Currency: "USD",
	}, 90)
	result := &engine.ScopedBudgetResult{
		ByProvider:    map[string]*engine.ScopedBudgetStatus{"aws": status},
		OverallHealth: status.Health,
	}
	cmd := &cobra.Command{}
	require.NoError(t, checkScopedBudgetExit(cmd, result))

	cfg.Cost.Budgets.ExitOnThreshold = true
	err := checkScopedBudgetExit(cmd, result)
	var budgetErr *BudgetExitError
	require.ErrorAs(t, err, &budgetErr)
	assert.Equal(t, 1, budgetErr.ExitCode)

	parentCode := 4
	cfg.Cost.Budgets.ExitCode = &parentCode
	err = checkScopedBudgetExit(cmd, result)
	require.ErrorAs(t, err, &budgetErr)
	assert.Equal(t, parentCode, budgetErr.ExitCode)
}

func TestFinfocusBudgetSkillArchive(t *testing.T) {
	t.Parallel()

	archive, err := zip.OpenReader(filepath.Join(budgetSkillDir(t), "finfocus-budget.skill"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, archive.Close())
	})

	dir := budgetSkillDir(t)
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
		assert.NotContains(t, file.Name, ".skill")
		assert.Positive(t, file.UncompressedSize64)
		rel := strings.TrimPrefix(file.Name, "finfocus-budget/")
		packed, readErr := file.Open()
		require.NoError(t, readErr)
		got, readErr := io.ReadAll(packed)
		require.NoError(t, packed.Close())
		require.NoError(t, readErr)
		want, readErr := os.ReadFile(filepath.Join(dir, rel))
		require.NoError(t, readErr)
		assert.Equal(t, string(want), string(got), file.Name)
	}
	for _, name := range []string{
		"finfocus-budget/SKILL.md",
		"finfocus-budget/references/budget-config.md",
		"finfocus-budget/references/budget-cli.md",
		"finfocus-budget/references/budget-health.md",
	} {
		assert.Truef(t, names[name], "archive missing %s", name)
	}
}

func assertBudgetFlags(t *testing.T, cmd *cobra.Command, persistent bool) {
	t.Helper()
	flags := cmd.Flags()
	if persistent {
		flags = cmd.PersistentFlags()
	}
	threshold := flags.Lookup("exit-on-threshold")
	require.NotNil(t, threshold)
	assert.Equal(t, "false", threshold.DefValue)
	code := flags.Lookup("exit-code")
	require.NotNil(t, code)
	assert.Equal(t, "1", code.DefValue)
	scope := flags.Lookup("budget-scope")
	require.NotNil(t, scope)
	assert.Empty(t, scope.DefValue)
}

func readBudgetSkill(t *testing.T) string {
	t.Helper()
	dir := budgetSkillDir(t)
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	return b.String()
}

func budgetSkillDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", "finfocus-budget")
}
