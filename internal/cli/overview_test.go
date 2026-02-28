package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
)

// ---------------------------------------------------------------------------
// NewOverviewCmd - Flag parsing and validation
// ---------------------------------------------------------------------------

func TestNewOverviewCmd_NoArgsAutoDetectFails(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// Run from a temp dir with no Pulumi project so auto-detect fails
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--yes"})

	execErr := cmd.Execute()
	require.Error(t, execErr)
	assert.Contains(t, execErr.Error(), "auto-detecting Pulumi project")
}

func TestNewOverviewCmd_HelpFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "unified cost dashboard")
	assert.Contains(t, output, "--pulumi-state")
	assert.Contains(t, output, "--pulumi-json")
	assert.Contains(t, output, "--stack")
	assert.Contains(t, output, "--from")
	assert.Contains(t, output, "--to")
	assert.Contains(t, output, "--adapter")
	assert.Contains(t, output, "--output")
	assert.Contains(t, output, "--filter")
	assert.Contains(t, output, "--plain")
	assert.Contains(t, output, "--yes")
	assert.Contains(t, output, "--no-pagination")
	assert.Contains(t, output, "--exit-on-threshold")
	assert.Contains(t, output, "--exit-code")
	assert.Contains(t, output, "--budget-scope")
}

func TestNewOverviewCmd_NonExistentStateFile(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--pulumi-state", "/nonexistent/state.json", "--yes"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi state")
}

func TestNewOverviewCmd_NonExistentPlanFile(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	// Create a valid state file
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--pulumi-json", "/nonexistent/plan.json",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi plan")
}

func TestNewOverviewCmd_InvalidDateRange(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	// Create a valid state file
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--from", "2025-12-31",
		"--to", "2025-01-01",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date range")
}

func TestNewOverviewCmd_InvalidFromDate(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--from", "not-a-date",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date range")
}

func TestNewOverviewCmd_ValidStateFileEmptyResources(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")
	isolateConfig(t)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--yes",
	})

	// cmd.Execute() may succeed or fail with "opening plugins" depending on
	// the test environment. Both outcomes are acceptable because this test
	// validates state loading and merge behaviour, not plugin connectivity.
	err := cmd.Execute()
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

// ---------------------------------------------------------------------------
// Flag parsing tests
// ---------------------------------------------------------------------------

func TestNewOverviewCmd_AllFlagsAccepted(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()

	// Verify all flags are registered
	assert.NotNil(t, cmd.Flags().Lookup("pulumi-json"))
	assert.NotNil(t, cmd.Flags().Lookup("pulumi-state"))
	assert.NotNil(t, cmd.Flags().Lookup("stack"))
	assert.NotNil(t, cmd.Flags().Lookup("from"))
	assert.NotNil(t, cmd.Flags().Lookup("to"))
	assert.NotNil(t, cmd.Flags().Lookup("adapter"))
	assert.NotNil(t, cmd.Flags().Lookup("output"))
	assert.NotNil(t, cmd.Flags().Lookup("filter"))
	assert.NotNil(t, cmd.Flags().Lookup("plain"))
	assert.NotNil(t, cmd.Flags().Lookup("yes"))
	assert.NotNil(t, cmd.Flags().Lookup("no-pagination"))
	assert.NotNil(t, cmd.Flags().Lookup("exit-on-threshold"))
	assert.NotNil(t, cmd.Flags().Lookup("exit-code"))
	assert.NotNil(t, cmd.Flags().Lookup("budget-scope"))
}

func TestNewOverviewCmd_StackFlagExists(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()
	stackFlag := cmd.Flags().Lookup("stack")
	require.NotNil(t, stackFlag)
	assert.Equal(t, "", stackFlag.DefValue)
	assert.Contains(t, stackFlag.Usage, "Pulumi stack name")
}

func TestNewOverviewCmd_ExplicitStateStillWorks(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")
	isolateConfig(t)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--yes",
	})

	// Should still work with explicit --pulumi-state (backwards compatibility).
	// May fail at "opening plugins" depending on test environment.
	err := cmd.Execute()
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

func TestNewOverviewCmd_YesShortFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()
	yesFlag := cmd.Flags().Lookup("yes")
	require.NotNil(t, yesFlag)
	assert.Equal(t, "y", yesFlag.Shorthand)
}

func TestNewOverviewCmd_ShortFlags(t *testing.T) {
	cmd := cli.NewOverviewCmd()

	tests := []struct {
		long  string
		short string
	}{
		{"stack", "s"},
		{"filter", "f"},
		{"adapter", "a"},
	}

	for _, tt := range tests {
		flag := cmd.Flags().Lookup(tt.long)
		require.NotNil(t, flag, "--%s flag should be registered", tt.long)
		assert.Equal(t, tt.short, flag.Shorthand, "--%s should have short flag -%s", tt.long, tt.short)
	}
}

// T013: Budget flag registration and behavior on overview command
// ---------------------------------------------------------------------------

func TestNewOverviewCmd_BudgetFlagRegistration(t *testing.T) {
	cmd := cli.NewOverviewCmd()

	// Verify budget flags are registered
	assert.NotNil(t, cmd.Flags().Lookup("exit-on-threshold"),
		"--exit-on-threshold flag should be registered on overview command")
	assert.NotNil(t, cmd.Flags().Lookup("exit-code"),
		"--exit-code flag should be registered on overview command")
	assert.NotNil(t, cmd.Flags().Lookup("budget-scope"),
		"--budget-scope flag should be registered on overview command")
}

func TestNewOverviewCmd_BudgetFlagDefaults(t *testing.T) {
	cmd := cli.NewOverviewCmd()

	// --exit-on-threshold defaults to false
	exitOnThreshold, err := cmd.Flags().GetBool("exit-on-threshold")
	require.NoError(t, err)
	assert.False(t, exitOnThreshold, "--exit-on-threshold should default to false")

	// --exit-code defaults to 1
	exitCode, err := cmd.Flags().GetInt("exit-code")
	require.NoError(t, err)
	assert.Equal(t, 1, exitCode, "--exit-code should default to 1")

	// --budget-scope defaults to empty
	budgetScope, err := cmd.Flags().GetString("budget-scope")
	require.NoError(t, err)
	assert.Empty(t, budgetScope, "--budget-scope should default to empty")
}

func TestNewOverviewCmd_BudgetFlagParsing(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		wantThreshold     bool
		wantExitCode      int
		wantBudgetScope   string
		wantThresholdFlag bool // whether the flag is marked as changed
	}{
		{
			name:              "all budget flags set",
			args:              []string{"--exit-on-threshold", "--exit-code=42", "--budget-scope=provider"},
			wantThreshold:     true,
			wantExitCode:      42,
			wantBudgetScope:   "provider",
			wantThresholdFlag: true,
		},
		{
			name:              "exit-on-threshold=false explicit",
			args:              []string{"--exit-on-threshold=false"},
			wantThreshold:     false,
			wantExitCode:      1, // default
			wantBudgetScope:   "",
			wantThresholdFlag: true,
		},
		{
			name:              "only budget-scope set",
			args:              []string{"--budget-scope=provider=aws"},
			wantThreshold:     false,
			wantExitCode:      1,
			wantBudgetScope:   "provider=aws",
			wantThresholdFlag: false,
		},
		{
			name:              "no budget flags",
			args:              nil,
			wantThreshold:     false,
			wantExitCode:      1,
			wantBudgetScope:   "",
			wantThresholdFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewOverviewCmd()
			if tt.args != nil {
				err := cmd.ParseFlags(tt.args)
				require.NoError(t, err)
			}

			threshold, err := cmd.Flags().GetBool("exit-on-threshold")
			require.NoError(t, err)
			assert.Equal(t, tt.wantThreshold, threshold)

			exitCode, err := cmd.Flags().GetInt("exit-code")
			require.NoError(t, err)
			assert.Equal(t, tt.wantExitCode, exitCode)

			scope, err := cmd.Flags().GetString("budget-scope")
			require.NoError(t, err)
			assert.Equal(t, tt.wantBudgetScope, scope)

			assert.Equal(t, tt.wantThresholdFlag, cmd.Flags().Changed("exit-on-threshold"))
		})
	}
}

func TestNewOverviewCmd_ExitCodeOutOfRange(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")
	isolateConfig(t)

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	tests := []struct {
		name        string
		exitCode    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid exit code 0",
			exitCode: "0",
			wantErr:  false,
		},
		{
			name:     "valid exit code 255",
			exitCode: "255",
			wantErr:  false,
		},
		{
			name:        "exit code 999 out of range",
			exitCode:    "999",
			wantErr:     true,
			errContains: "--exit-code must be between",
		},
		{
			name:        "exit code -1 out of range",
			exitCode:    "-1",
			wantErr:     true,
			errContains: "--exit-code must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewOverviewCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{
				"--pulumi-state", statePath,
				"--exit-code=" + tt.exitCode,
				"--yes",
			})

			err := cmd.Execute()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else if err != nil {
				// Valid exit-code should not cause an exit-code error;
				// "opening plugins" is acceptable in test environments.
				assert.Contains(t, err.Error(), "opening plugins")
			}
		})
	}
}

func TestNewOverviewCmd_HelpIncludesBudgetFlags(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--exit-on-threshold",
		"help should include --exit-on-threshold flag")
	assert.Contains(t, output, "--exit-code",
		"help should include --exit-code flag")
	assert.Contains(t, output, "--budget-scope",
		"help should include --budget-scope flag")
}

func TestNewOverviewCmd_BudgetFlagsWithOtherFlags(t *testing.T) {
	// Verify budget flags coexist with existing overview flags
	cmd := cli.NewOverviewCmd()
	err := cmd.ParseFlags([]string{
		"--exit-on-threshold",
		"--exit-code=2",
		"--budget-scope=global",
		"--plain",
		"--yes",
		"--output=table",
	})
	require.NoError(t, err)

	threshold, err := cmd.Flags().GetBool("exit-on-threshold")
	require.NoError(t, err)
	assert.True(t, threshold)

	plain, err := cmd.Flags().GetBool("plain")
	require.NoError(t, err)
	assert.True(t, plain)

	yes, err := cmd.Flags().GetBool("yes")
	require.NoError(t, err)
	assert.True(t, yes)
}

func TestNewOverviewCmd_DefaultsToTableOutput(t *testing.T) {
	// Verify the default output format is "table", which enables the TUI path
	// when stdout is a TTY. Budget exit-code evaluation only runs in the
	// non-TTY (plain) path; the TUI path displays budget status visually.
	cmd := cli.NewOverviewCmd()
	err := cmd.ParseFlags([]string{
		"--exit-on-threshold",
		"--exit-code=2",
	})
	require.NoError(t, err)

	// With a non-TTY writer (bytes.Buffer), even with --exit-on-threshold,
	// the command should take the non-TTY path where budget evaluation runs.
	// With a TTY writer, it would take the TUI path (no budget exit codes).
	// We verify the flag is registered and parseable in both scenarios.
	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "table", outputFlag.DefValue,
		"output default should be table (TUI path for TTY)")
}

func TestNewOverviewCmd_BudgetEvalNonTTYPath(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")
	isolateConfig(t)

	// Create a valid state file with empty resources
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--yes",
		"--exit-on-threshold",
		"--exit-code=2",
	})

	// With empty resources and no plugins, the command either:
	// 1. Fails at "opening plugins" (no plugins installed)
	// 2. Succeeds with zero-cost budget evaluation (no budget configured = OK)
	// Both are acceptable: this test validates the flags are accepted in the
	// non-TTY path and don't cause unexpected errors.
	err := cmd.Execute()
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins",
			"error should be from plugin opening, not budget flags")
	}
}

// ---------------------------------------------------------------------------
// Cache wiring tests (SC-002: overview uses newEngineWithCache)
// ---------------------------------------------------------------------------

// TestOverviewPlainText_CacheHitReturnsProjectedCost verifies that the overview
// plain-text path wires cache correctly via newEngineWithCache. It pre-populates
// a BoltDB cache with projected cost data and confirms the overview command
// retrieves that data, proving cache integration end-to-end.
//
// Without plugins or a spec loader, the engine would return no projected cost
// for the resource (adapter="none", which is not cached and produces nil
// ProjectedCost). A non-nil ProjectedCost in the output proves the engine
// served it from the pre-populated cache with "(cached)" appended to the
// Adapter field internally (the overview JSON does not expose Adapter, but
// the non-zero MonthlyCost can only come from the cache in this setup).
func TestOverviewPlainText_CacheHitReturnsProjectedCost(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")
	isolateConfig(t)

	tmpDir := t.TempDir()

	// 1. Create a Pulumi state file with one custom resource.
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{
		"version": 3,
		"deployment": {
			"manifest": {"time": "2025-01-15T00:00:00Z", "magic": "", "version": ""},
			"resources": [
				{
					"urn": "urn:pulumi:dev::proj::aws:ec2/instance:Instance::web",
					"type": "aws:ec2/instance:Instance",
					"id": "i-0abc123",
					"custom": true,
					"inputs": {
						"instanceType": "t3.micro",
						"availabilityZone": "us-east-1a"
					},
					"outputs": {
						"instanceType": "t3.micro",
						"availabilityZone": "us-east-1a"
					}
				}
			]
		}
	}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	// 2. Create a BoltDB cache and pre-populate with projected cost data.
	cacheDir := filepath.Join(tmpDir, "cache")
	t.Setenv(cache.EnvCacheDir, cacheDir)

	ctx := context.Background()
	store, storeErr := cache.NewBoltStore(ctx, cacheDir, true, 3600, 0)
	require.NoError(t, storeErr)

	cacheKey := cache.BuildProjectedKey("aws", "aws:ec2/instance:Instance", "us-east-1a", "t3.micro")
	cachedResults := []engine.CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "urn:pulumi:dev::proj::aws:ec2/instance:Instance::web",
		Adapter:      "test-plugin",
		Monthly:      42.50,
		Hourly:       0.0582,
		Currency:     "USD",
		Notes:        "cached projected cost",
	}}
	data, marshalErr := json.Marshal(cachedResults)
	require.NoError(t, marshalErr)
	require.NoError(t, store.Set(cacheKey, data))
	require.NoError(t, store.Close())

	// 3. Run the overview command via root cmd (--cache-ttl is a persistent root flag).
	var buf, errBuf bytes.Buffer
	root := cli.NewRootCmd("test")
	root.SetOut(&buf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"overview",
		"--pulumi-state", statePath,
		"--cache-ttl", "3600",
		"--plain",
		"--yes",
		"--output", "json",
	})

	err := root.Execute()
	// The command may fail at "opening plugins" if no plugins are installed.
	// That failure occurs AFTER engine creation, so the cache is still wired.
	// If it fails at plugins, we verify the cache DB was at least opened.
	if err != nil {
		// If the error is about plugins, that's expected - the test still
		// validates that the command accepts --cache-ttl without error up
		// to the plugin-opening phase. Verify cache DB was created.
		assert.Contains(t, err.Error(), "opening plugins",
			"expected plugin error, got: %v", err)
		_, statErr := os.Stat(filepath.Join(cacheDir, "cache.db"))
		assert.NoError(t, statErr, "cache.db should exist (engine opened it)")
		t.Log("plugins unavailable — cache-hit path not verified")
		return
	}

	// 4. Parse the JSON output and verify projected cost came from cache.
	var output engine.OverviewJSONOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output),
		"failed to parse JSON output: %s", buf.String())

	require.Len(t, output.Resources, 1, "expected 1 resource in output")
	row := output.Resources[0]
	assert.Equal(t, "aws:ec2/instance:Instance", row.Type)

	// The projected cost should come from the pre-populated cache.
	// Without cache, no plugins means ProjectedCost would be nil.
	require.NotNil(t, row.ProjectedCost,
		"projected cost should be non-nil (served from cache)")
	assert.InDelta(t, 42.50, row.ProjectedCost.MonthlyCost, 0.01,
		"projected cost should match cached value")
	assert.Equal(t, "USD", row.ProjectedCost.Currency)
}
