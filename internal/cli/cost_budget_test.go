package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// T037: Unit test for warning log when exit_code: 0 and threshold exceeded.
func TestCheckBudgetExit_WarningOnlyMode(t *testing.T) {
	// Create a test command to capture output
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	// Create budget status with warning-only mode (exit_code: 0)
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        0, // Warning-only mode
		},
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusExceeded},
		},
	}

	// checkBudgetExit should return nil (no exit) but log a warning
	err := checkBudgetExit(cmd, status, nil)

	// Should not return an error (no exit)
	assert.NoError(t, err, "warning-only mode should not return an error")

	// Should have logged a warning to stderr
	assert.Contains(t, errBuf.String(), "WARNING:")
	assert.Contains(t, errBuf.String(), "budget threshold exceeded")
}

// T036: Unit test for exit_code: 0 with exit_on_threshold: true returns exit 0.
func TestBudgetStatus_ExitCode_WarningOnly(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        0, // Warning-only mode
		},
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusExceeded},
		},
	}

	// ShouldExit should return true (threshold is exceeded and exit is enabled)
	assert.True(t, status.ShouldExit())

	// GetExitCode should return 0 (warning-only mode)
	assert.Equal(t, 0, status.GetExitCode())
}

// T038: Integration test for warning-only mode (tested at CLI level).
func TestCheckBudgetExit_WarningOnlyNoExitError(t *testing.T) {
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        0, // Warning-only mode
		},
		CurrentSpend: 1500.0,
		Percentage:   150.0,
		Alerts: []engine.ThresholdStatus{
			{Threshold: 100.0, Status: engine.ThresholdStatusExceeded},
		},
	}

	err := checkBudgetExit(cmd, status, nil)

	// Warning-only mode should NOT return a BudgetExitError
	assert.NoError(t, err)

	// But should log the warning
	warningOutput := errBuf.String()
	assert.Contains(t, warningOutput, "WARNING:")
}

// Test checkBudgetExit returns BudgetExitError for non-zero exit codes.
func TestCheckBudgetExit_ReturnsExitError(t *testing.T) {
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        2,
		},
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusExceeded},
		},
	}

	err := checkBudgetExit(cmd, status, nil)

	require.Error(t, err)
	var budgetErr *BudgetExitError
	require.ErrorAs(t, err, &budgetErr, "error should be *BudgetExitError")
	assert.Equal(t, 2, budgetErr.ExitCode)
	assert.Contains(t, budgetErr.Reason, "budget threshold exceeded")
}

// Test checkBudgetExit returns nil when no budget status.
func TestCheckBudgetExit_NilStatus(t *testing.T) {
	cmd := &cobra.Command{}

	err := checkBudgetExit(cmd, nil, nil)

	assert.NoError(t, err, "nil status should return nil")
}

// Test checkBudgetExit returns nil when exit is disabled.
func TestCheckBudgetExit_ExitDisabled(t *testing.T) {
	cmd := &cobra.Command{}

	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: false, // Disabled
			ExitCode:        2,
		},
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusExceeded},
		},
	}

	err := checkBudgetExit(cmd, status, nil)

	assert.NoError(t, err, "disabled exit should return nil")
}

// Test checkBudgetExit returns nil when no thresholds exceeded.
func TestCheckBudgetExit_NoExceeded(t *testing.T) {
	cmd := &cobra.Command{}

	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        2,
		},
		Alerts: []engine.ThresholdStatus{
			{Threshold: 80.0, Status: engine.ThresholdStatusOK},
		},
	}

	err := checkBudgetExit(cmd, status, nil)

	assert.NoError(t, err, "no exceeded thresholds should return nil")
}

// T020a/T027a: Test checkBudgetExit returns exit code 1 for evaluation errors (FR-009).
func TestCheckBudgetExit_EvaluationError(t *testing.T) {
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	// Evaluation error should return exit code 1 regardless of configuration
	evalErr := engine.ErrCurrencyMismatch

	err := checkBudgetExit(cmd, nil, evalErr)

	require.Error(t, err)
	var budgetErr *BudgetExitError
	require.ErrorAs(t, err, &budgetErr, "error should be *BudgetExitError")
	assert.Equal(t, engine.ExitCodeBudgetEvaluationError, budgetErr.ExitCode)
	assert.Equal(t, 1, budgetErr.ExitCode, "evaluation error should return exit code 1")
	assert.Contains(t, budgetErr.Reason, "budget evaluation failed")
}

// T040: ExitReason indicates warning-only mode.
func TestBudgetStatus_ExitReason_WarningOnly(t *testing.T) {
	status := &engine.BudgetStatus{
		Budget: config.BudgetConfig{
			Amount:          1000.0,
			Currency:        "USD",
			ExitOnThreshold: true,
			ExitCode:        0, // Warning-only mode
		},
		Alerts: []engine.ThresholdStatus{
			{Threshold: 100.0, Status: engine.ThresholdStatusExceeded},
		},
	}

	reason := status.ExitReason()
	assert.Contains(t, reason, "warning only")
	assert.Contains(t, reason, "exit code 0")
}

// T041: Unit test for --exit-on-threshold flag parsing.
func TestCostCmd_ExitOnThresholdFlag(t *testing.T) {
	// Create a fresh cost command
	cmd := newCostCmd()

	// Parse the --exit-on-threshold flag
	err := cmd.ParseFlags([]string{"--exit-on-threshold"})
	require.NoError(t, err)

	// Verify the flag was parsed correctly
	exitOnThreshold, err := cmd.Flags().GetBool("exit-on-threshold")
	require.NoError(t, err)
	assert.True(t, exitOnThreshold, "--exit-on-threshold should be true when set")

	// Verify the flag was marked as changed
	assert.True(t, cmd.Flags().Changed("exit-on-threshold"))
}

// T041b: Unit test for --exit-on-threshold=false flag parsing.
func TestCostCmd_ExitOnThresholdFlagFalse(t *testing.T) {
	cmd := newCostCmd()

	err := cmd.ParseFlags([]string{"--exit-on-threshold=false"})
	require.NoError(t, err)

	exitOnThreshold, err := cmd.Flags().GetBool("exit-on-threshold")
	require.NoError(t, err)
	assert.False(t, exitOnThreshold, "--exit-on-threshold=false should be false")
}

// T042: Unit test for --exit-code flag parsing.
func TestCostCmd_ExitCodeFlag(t *testing.T) {
	cmd := newCostCmd()

	// Parse the --exit-code flag with value 42
	err := cmd.ParseFlags([]string{"--exit-code=42"})
	require.NoError(t, err)

	// Verify the flag was parsed correctly
	exitCode, err := cmd.Flags().GetInt("exit-code")
	require.NoError(t, err)
	assert.Equal(t, 42, exitCode, "--exit-code should be 42")

	// Verify the flag was marked as changed
	assert.True(t, cmd.Flags().Changed("exit-code"))
}

// T042b: Unit test for --exit-code flag default value.
func TestCostCmd_ExitCodeFlagDefault(t *testing.T) {
	cmd := newCostCmd()

	// Don't set any flags - check default via PersistentFlags
	exitCode, err := cmd.PersistentFlags().GetInt("exit-code")
	require.NoError(t, err)
	assert.Equal(t, 1, exitCode, "default --exit-code should be 1")

	// Flag should NOT be changed when not explicitly set
	assert.False(t, cmd.PersistentFlags().Changed("exit-code"))
}

// T043: Unit test for CLI flags overriding environment variables.
func TestCostCmd_CLIFlagsOverrideEnv(t *testing.T) {
	// Set environment variables
	t.Setenv("FINFOCUS_BUDGET_EXIT_ON_THRESHOLD", "false")
	t.Setenv("FINFOCUS_BUDGET_EXIT_CODE", "5")

	// Save and restore global config
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })

	// Initialize config from env (simulating what happens in real CLI)
	cfg := &config.Config{
		Cost: config.CostConfig{
			Budgets: config.BudgetConfig{
				ExitOnThreshold: false, // From env
				ExitCode:        5,     // From env
			},
		},
	}
	config.SetGlobalConfig(cfg)

	// Create cost command and parse CLI flags that override env
	cmd := newCostCmd()
	err := cmd.ParseFlags([]string{"--exit-on-threshold", "--exit-code=99"})
	require.NoError(t, err)

	// Simulate PersistentPreRunE execution
	preRunE := cmd.PersistentPreRunE
	require.NotNil(t, preRunE, "PersistentPreRunE should be set")
	err = preRunE(cmd, []string{})
	require.NoError(t, err)

	// Verify CLI flags overrode the env values
	globalCfg := config.GetGlobalConfig()
	require.NotNil(t, globalCfg)
	assert.True(t, globalCfg.Cost.Budgets.ExitOnThreshold, "CLI flag should override env to true")
	assert.Equal(t, 99, globalCfg.Cost.Budgets.ExitCode, "CLI flag should override env to 99")
}

// T044: Unit test for CLI flags overriding config file values.
func TestCostCmd_CLIFlagsOverrideConfig(t *testing.T) {
	// Save and restore global config
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })

	// Simulate config loaded from file
	cfg := &config.Config{
		Cost: config.CostConfig{
			Budgets: config.BudgetConfig{
				Amount:          1000.0,
				Currency:        "USD",
				ExitOnThreshold: false, // From config file
				ExitCode:        3,     // From config file
			},
		},
	}
	config.SetGlobalConfig(cfg)

	// Create cost command and parse CLI flags
	cmd := newCostCmd()
	err := cmd.ParseFlags([]string{"--exit-on-threshold=true", "--exit-code=7"})
	require.NoError(t, err)

	// Execute PersistentPreRunE
	err = cmd.PersistentPreRunE(cmd, []string{})
	require.NoError(t, err)

	// Verify CLI flags overrode the config file values
	globalCfg := config.GetGlobalConfig()
	require.NotNil(t, globalCfg)
	assert.True(t, globalCfg.Cost.Budgets.ExitOnThreshold, "CLI flag should override config to true")
	assert.Equal(t, 7, globalCfg.Cost.Budgets.ExitCode, "CLI flag should override config to 7")

	// Verify other config values were preserved
	assert.Equal(t, 1000.0, globalCfg.Cost.Budgets.Amount, "budget amount should be preserved")
	assert.Equal(t, "USD", globalCfg.Cost.Budgets.Currency, "currency should be preserved")
}

// T045: Integration test for CLI flag overrides - only changed flags are applied.
func TestCostCmd_OnlyChangedFlagsApplied(t *testing.T) {
	// Save and restore global config
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })

	// Simulate config with specific values
	cfg := &config.Config{
		Cost: config.CostConfig{
			Budgets: config.BudgetConfig{
				ExitOnThreshold: true,
				ExitCode:        42,
			},
		},
	}
	config.SetGlobalConfig(cfg)

	// Create cost command but only set --exit-code flag
	cmd := newCostCmd()
	err := cmd.ParseFlags([]string{"--exit-code=99"})
	require.NoError(t, err)

	// Execute PersistentPreRunE
	err = cmd.PersistentPreRunE(cmd, []string{})
	require.NoError(t, err)

	// Verify only the changed flag was applied
	globalCfg := config.GetGlobalConfig()
	require.NotNil(t, globalCfg)
	assert.True(t, globalCfg.Cost.Budgets.ExitOnThreshold, "unchanged flag should preserve config value")
	assert.Equal(t, 99, globalCfg.Cost.Budgets.ExitCode, "changed flag should override config value")
}

// T045b: Test that PersistentPreRunE handles nil global config gracefully.
func TestCostCmd_NilGlobalConfig(t *testing.T) {
	// Save and restore global config
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })

	// Ensure no global config is set
	config.SetGlobalConfig(nil)

	cmd := newCostCmd()
	err := cmd.ParseFlags([]string{"--exit-on-threshold", "--exit-code=5"})
	require.NoError(t, err)

	// PersistentPreRunE should not panic or error with nil config
	err = cmd.PersistentPreRunE(cmd, []string{})
	assert.NoError(t, err, "should handle nil global config gracefully")
}
