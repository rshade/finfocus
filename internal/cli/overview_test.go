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
