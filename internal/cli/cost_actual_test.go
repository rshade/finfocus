package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

func TestNewCostActualCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
		isolate     bool // chdir to temp dir to avoid Pulumi.yaml in repo tree
	}{
		{
			name:        "no flags triggers auto-detection (T023)",
			args:        []string{},
			expectError: true,
			// Without flags, auto-detection kicks in and fails because no Pulumi project found.
			// The error must NOT be "either --pulumi-json or --pulumi-state is required".
			errorMsg: "",
			isolate:  true,
		},
		{
			name:        "missing from flag",
			args:        []string{"--pulumi-json", "test.json"},
			expectError: true,
			errorMsg:    "--from is required when using --pulumi-json",
		},
		{
			name:        "help flag",
			args:        []string{"--help"},
			expectError: false,
		},
		{
			name: "with all flags",
			args: []string{
				"--pulumi-json", "test.json",
				"--from", "2025-01-01",
				"--to", "2025-01-31",
				"--adapter", "test-adapter",
				"--output", "json",
				"--group-by", "type",
			},
			expectError: true, // Will fail because file doesn't exist
			errorMsg:    "loading Pulumi plan",
		},
		{
			name: "with required flags only",
			args: []string{
				"--pulumi-json", "test.json",
				"--from", "2025-01-01",
				"--to", "2025-12-31",
			},
			expectError: true, // Will fail because file doesn't exist
			errorMsg:    "loading Pulumi plan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isolate {
				isolateFromPulumiProject(t)
			}
			var buf bytes.Buffer
			cmd := cli.NewCostActualCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCostActualCmdFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostActualCmd()

	// Check required flags
	pulumiJSONFlag := cmd.Flags().Lookup("pulumi-json")
	assert.NotNil(t, pulumiJSONFlag)
	assert.Equal(t, "string", pulumiJSONFlag.Value.Type())

	fromFlag := cmd.Flags().Lookup("from")
	assert.NotNil(t, fromFlag)
	assert.Equal(t, "string", fromFlag.Value.Type())

	// Check optional flags
	toFlag := cmd.Flags().Lookup("to")
	assert.NotNil(t, toFlag)
	assert.Equal(t, "string", toFlag.Value.Type())
	assert.Contains(t, toFlag.Usage, "defaults to now")

	adapterFlag := cmd.Flags().Lookup("adapter")
	assert.NotNil(t, adapterFlag)
	assert.Equal(t, "string", adapterFlag.Value.Type())

	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "string", outputFlag.Value.Type())
	assert.Equal(t, "table", outputFlag.DefValue)

	groupByFlag := cmd.Flags().Lookup("group-by")
	assert.NotNil(t, groupByFlag)
	assert.Equal(t, "string", groupByFlag.Value.Type())
	assert.Contains(t, groupByFlag.Usage, "resource, type, provider")
}

func TestCostActualCmdHelp(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Fetch actual historical costs")
	assert.Contains(t, output, "cloud provider billing APIs")
	assert.Contains(t, output, "--pulumi-json")
	assert.Contains(t, output, "--from")
	assert.Contains(t, output, "--to")
	assert.Contains(t, output, "--group-by")
	assert.Contains(t, output, "defaults to now")
}

func TestCostActualCmdExamples(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostActualCmd()

	// Check that examples are present
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "finfocus cost actual")
	assert.Contains(t, cmd.Example, "--pulumi-json plan.json")
	assert.Contains(t, cmd.Example, "--stack production")
	assert.Contains(t, cmd.Example, "--group-by type")
	assert.Contains(t, cmd.Example, "--group-by provider")
	assert.Contains(t, cmd.Example, "--estimate-confidence")
}

func TestParseTimeRange(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tests := []struct {
		name        string
		fromStr     string
		toStr       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid date range",
			fromStr:     "2025-01-01",
			toStr:       "2025-01-31",
			expectError: false,
		},
		{
			name:        "valid RFC3339 range",
			fromStr:     "2025-01-01T00:00:00Z",
			toStr:       "2025-01-31T23:59:59Z",
			expectError: false,
		},
		{
			name:        "to before from",
			fromStr:     "2025-01-31",
			toStr:       "2025-01-01",
			expectError: true,
			errorMsg:    "'to' date must be after 'from' date",
		},
		{
			name:        "invalid from date",
			fromStr:     "invalid",
			toStr:       "2025-01-31",
			expectError: true,
			errorMsg:    "parsing 'from' date",
		},
		{
			name:        "invalid to date",
			fromStr:     "2025-01-01",
			toStr:       "invalid",
			expectError: true,
			errorMsg:    "parsing 'to' date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, err := cli.ParseTimeRange(tt.fromStr, tt.toStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.True(t, to.After(from) || to.Equal(from))
			}
		})
	}
}

// TestCostActualCmdPulumiStateFlag tests the --pulumi-state flag for state-based actual cost.
func TestCostActualCmdPulumiStateFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewCostActualCmd()

	// Check --pulumi-state flag exists
	pulumiStateFlag := cmd.Flags().Lookup("pulumi-state")
	assert.NotNil(t, pulumiStateFlag, "--pulumi-state flag should exist")
	assert.Equal(t, "string", pulumiStateFlag.Value.Type())
	assert.Contains(t, pulumiStateFlag.Usage, "state")
}

// TestCostActualCmdMutuallyExclusiveInputs tests that --pulumi-json and --pulumi-state are mutually exclusive.
func TestCostActualCmdMutuallyExclusiveInputs(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
		isolate     bool
	}{
		{
			name: "both pulumi-json and pulumi-state provided",
			args: []string{
				"--pulumi-json", "test.json",
				"--pulumi-state", "state.json",
				"--from", "2025-01-01",
			},
			expectError: true,
			errorMsg:    "mutually exclusive",
		},
		{
			name: "neither pulumi-json nor pulumi-state triggers auto-detection",
			args: []string{
				"--from", "2025-01-01",
			},
			expectError: true,
			// Auto-detection kicks in but fails since no Pulumi project
			errorMsg: "",
			isolate:  true,
		},
		{
			name: "only pulumi-state provided without from (auto-detect)",
			args: []string{
				"--pulumi-state", "../../test/fixtures/state/valid-state.json",
			},
			// Command succeeds: --from is auto-detected from earliest Created timestamp in state
			// Plugin may report resource errors (missing IDs), but command completes successfully
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isolate {
				isolateFromPulumiProject(t)
			}

			var buf bytes.Buffer
			cmd := cli.NewCostActualCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCostActualCmdHelpWithStateFlag tests that help includes --pulumi-state documentation.
func TestCostActualCmdHelpWithStateFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--pulumi-state")
	assert.Contains(t, output, "state")
}

// TestCostActualCmdEstimateConfidenceFlag tests the --estimate-confidence flag.
func TestCostActualCmdEstimateConfidenceFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewCostActualCmd()

	// Check --estimate-confidence flag exists
	estimateConfidenceFlag := cmd.Flags().Lookup("estimate-confidence")
	assert.NotNil(t, estimateConfidenceFlag, "--estimate-confidence flag should exist")
	assert.Equal(t, "bool", estimateConfidenceFlag.Value.Type())
	assert.Contains(t, estimateConfidenceFlag.Usage, "confidence")
}

// TestCostActualCmdHelpWithEstimateConfidence tests that help includes confidence documentation.
func TestCostActualCmdHelpWithEstimateConfidence(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--estimate-confidence")
	assert.Contains(t, output, "confidence")
}

// TestCostActualCmdWithEstimateConfidenceFlag tests the flag is accepted without error.
func TestCostActualCmdWithEstimateConfidenceFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "estimate-confidence flag with state file",
			args: []string{
				"--pulumi-state", "../../test/fixtures/state/valid-state.json",
				"--estimate-confidence",
			},
			// Command succeeds - flag is accepted
			expectError: false,
		},
		{
			name: "estimate-confidence false (explicit)",
			args: []string{
				"--pulumi-state", "../../test/fixtures/state/valid-state.json",
				"--estimate-confidence=false",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewCostActualCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCostActualWithoutInputFlags is intentionally duplicated from the
// "no flags triggers auto-detection (T023)" case in TestNewCostActualCmd.
// This standalone test provides emphasis and standalone coverage for the
// critical auto-detection path without requiring reviewers to scan the
// full table-driven test.
func TestCostActualWithoutInputFlags(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// Isolate from any Pulumi.yaml in the repository tree.
	isolateFromPulumiProject(t)

	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	// Command will error (no Pulumi project in test env), but the error
	// must NOT be about requiring input flags.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "either --pulumi-json or --pulumi-state is required")
	assert.NotContains(t, err.Error(), "required flag")
}

// TestCostActualMutualExclusivityStillEnforced verifies that both flags
// provided together still returns an error (T024).
func TestCostActualMutualExclusivityStillEnforced(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-json", "plan.json",
		"--pulumi-state", "state.json",
		"--from", "2025-01-01",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestStackFlagExistsOnActual verifies the --stack flag is inherited by the
// actual subcommand via the cost parent (T028).
func TestStackFlagExistsOnActual(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// Isolate from any Pulumi.yaml in the repository tree.
	isolateFromPulumiProject(t)

	root := cli.NewRootCmd("test")
	costCmd, _, findErr := root.Find([]string{"cost"})
	require.NoError(t, findErr)

	stackFlag := costCmd.PersistentFlags().Lookup("stack")
	require.NotNil(t, stackFlag, "--stack flag should be on cost parent command")

	// Verify it's accepted on actual subcommand
	root.SetArgs([]string{"cost", "actual", "--stack", "production"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	execErr := root.Execute()
	// Should fail from auto-detection, not unknown flag
	require.Error(t, execErr)
	assert.NotContains(t, execErr.Error(), "unknown flag")
}

// TestStackFlagIgnoredWithPulumiStateOnActual verifies --stack is ignored when
// --pulumi-state is provided for cost actual (T030).
func TestStackFlagIgnoredWithPulumiStateOnActual(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	root := cli.NewRootCmd("test")
	root.SetArgs([]string{
		"cost", "actual",
		"--pulumi-state", "../../test/fixtures/state/valid-state.json",
		"--stack", "production",
	})

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	err := root.Execute()
	// Should succeed (state file exists and is valid), proving --stack was ignored
	require.NoError(t, err)
}

func TestCostActualCmd_JobsFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostActualCmd()

	jobsFlag := cmd.Flags().Lookup("jobs")
	require.NotNil(t, jobsFlag, "--jobs flag should exist")
	assert.Equal(t, "int", jobsFlag.Value.Type())
	assert.Equal(t, "0", jobsFlag.DefValue)

	// Check shorthand
	jobsShortFlag := cmd.Flags().ShorthandLookup("j")
	require.NotNil(t, jobsShortFlag, "-j shorthand should exist")
	assert.Equal(t, "jobs", jobsShortFlag.Name)
}

func TestCostActualCmd_NegativeJobs(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--pulumi-json", "test.json", "--from", "2025-01-01", "--jobs", "-1"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--jobs must be non-negative")
}

func TestCostActualCmd_JobsFlagInHelp(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostActualCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--jobs")
	assert.Contains(t, output, "-j")
	assert.Contains(t, output, "parallel workers")
}

func TestParseTime(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "YYYY-MM-DD format",
			input:       "2025-01-15",
			expectError: false,
		},
		{
			name:        "RFC3339 format",
			input:       time.RFC3339,
			expectError: true, // RFC3339 is a constant, not a valid date
		},
		{
			name:        "RFC3339 actual date",
			input:       "2025-01-15T10:30:00Z",
			expectError: false,
		},
		{
			name:        "invalid format",
			input:       "01/15/2025",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cli.ParseTime(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unable to parse date")
			} else {
				require.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

// getRecentDateRange returns dynamic dates within the 5-year limit for testing.
func getRecentDateRange() (string, string) {
	now := time.Now()
	fromDate := now.AddDate(0, -1, 0).Format("2006-01-02") // 1 month ago
	toDate := now.Format("2006-01-02")                     // today
	return fromDate, toDate
}

// getRecentRFC3339Range returns dynamic RFC3339 dates within the 5-year limit for testing.
func getRecentRFC3339Range() (string, string) {
	now := time.Now()
	fromDate := now.AddDate(0, -1, 0).Format(time.RFC3339) // 1 month ago
	toDate := now.Format(time.RFC3339)                     // today
	return fromDate, toDate
}

// getShortDateRange returns a 3-day dynamic date range within the 5-year limit for testing.
func getShortDateRange() (string, string) {
	now := time.Now()
	fromDate := now.AddDate(0, 0, -2).Format("2006-01-02") // 2 days ago
	toDate := now.Format("2006-01-02")                     // today
	return fromDate, toDate
}

// TestCostActualCmd_Success tests basic actual cost retrieval.
func TestCostActualCmd_Success(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	// Should succeed (without plugins, returns empty results)
	require.NoError(t, err)

	// Verify JSON output (empty without plugins)
	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	// Without plugins, actual cost returns empty array (no fallback like projected)
	assert.Len(t, results, 0) // No plugins = empty results
}

// TestCostActualCmd_MissingStartDate tests error for missing start date.
func TestCostActualCmd_MissingStartDate(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	_, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		// Missing --from
		"--to", toDate,
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--from is required when using --pulumi-json")
}

// TestCostActualCmd_DefaultEndDate tests default end date handling.
func TestCostActualCmd_DefaultEndDate(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	// Use a recent date within the max 366-day range
	recentDate, _ := getRecentDateRange()

	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", recentDate,
		// No --to (should default to now)
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results, 0) // No plugins = empty results
}

// TestCostActualCmd_InvalidDateFormat tests error for invalid date format.
func TestCostActualCmd_InvalidDateFormat(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	_, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", "invalid-date",
		"--to", toDate,
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}

// TestCostActualCmd_RFC3339DateFormat tests RFC3339 date format support.
func TestCostActualCmd_RFC3339DateFormat(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentRFC3339Range()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results, 0) // No plugins = empty results
}

// TestCostActualCmd_GroupByResource tests resource-level grouping.
func TestCostActualCmd_GroupByResource(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
		},
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-2",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--group-by", "resource",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results, 0) // No plugins = empty results
}

// TestCostActualCmd_GroupByType tests type-level grouping.
func TestCostActualCmd_GroupByType(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
		},
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-2",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--group-by", "type",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results, 0) // No plugins = empty results
}

// TestCostActualCmd_GroupByProvider tests provider-level grouping.
func TestCostActualCmd_GroupByProvider(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
		},
		{
			"type": "aws:s3/bucket:Bucket",
			"urn":  "urn:pulumi:stack::project::aws:s3/bucket:Bucket::bucket-1",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--group-by", "provider",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results, 0) // No plugins = empty results
}

// TestCostActualCmd_GroupByDaily tests daily grouping.
func TestCostActualCmd_GroupByDaily(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getShortDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--group-by", "daily",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	// Without plugins, daily grouping fails with empty results
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty results")
}

// TestCostActualCmd_TableOutput tests table format output.
func TestCostActualCmd_TableOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--output", "table",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Without plugins/specs, actual cost returns empty table
	assert.Contains(t, output, "Resource")
}

// TestCostActualCmd_NDJSONOutput tests NDJSON format output.
func TestCostActualCmd_NDJSONOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
		},
		{
			"type": "aws:s3/bucket:Bucket",
			"urn":  "urn:pulumi:stack::project::aws:s3/bucket:Bucket::bucket-1",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--output", "ndjson",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	// Without plugins/specs, NDJSON output is empty (no lines to output)
	output := out.String()
	assert.Empty(t, output) // No results = no NDJSON lines
}

// TestCostActualCmd_AdapterFilter tests adapter-specific filtering.
func TestCostActualCmd_AdapterFilter(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	isolateConfig(t)

	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	fromDate, toDate := getRecentDateRange()
	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", fromDate,
		"--to", toDate,
		"--adapter", "kubecost",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results []engine.CostResult
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	// Should succeed even without the specified adapter
	assert.Len(t, results, 0) // No plugins = empty results
}

// T016: Tests for MergeHistoricalResources merge logic.

func TestMergeHistoricalResources_NoHistory(t *testing.T) {
	current := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-current",
			},
		},
	}

	result := cli.MergeHistoricalResources(current, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "i-current", result[0].Properties["pulumi:cloudId"])
}

func TestMergeHistoricalResources_EmptyHistory(t *testing.T) {
	current := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-current",
			},
		},
	}

	result := cli.MergeHistoricalResources(current, []history.HistoricalResource{})
	assert.Len(t, result, 1)
}

func TestMergeHistoricalResources_AddsHistoricalCloudIDs(t *testing.T) {
	current := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-new",
			},
		},
	}

	historical := []history.HistoricalResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			CloudIDs: []string{"i-old", "i-new"},
		},
	}

	result := cli.MergeHistoricalResources(current, historical)

	// Should have 2: original i-new + historical i-old
	require.Len(t, result, 2)

	// First should be the original
	assert.Equal(t, "i-new", result[0].Properties["pulumi:cloudId"])

	// Second should be the historical cloud ID
	assert.Equal(t, "i-old", result[1].Properties["pulumi:cloudId"])
	assert.Equal(t, "aws:ec2/instance:Instance", result[1].Type)
	assert.Equal(t, "aws", result[1].Provider)
}

func TestMergeHistoricalResources_DeduplicatesExisting(t *testing.T) {
	current := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-12345",
			},
		},
	}

	historical := []history.HistoricalResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			CloudIDs: []string{"i-12345"}, // same as current
		},
	}

	result := cli.MergeHistoricalResources(current, historical)

	// Should NOT duplicate — i-12345 already exists
	assert.Len(t, result, 1)
	assert.Equal(t, "i-12345", result[0].Properties["pulumi:cloudId"])
}

func TestMergeHistoricalResources_TwoHistoricalCloudIDs(t *testing.T) {
	// Resource replaced mid-month: old i-aaa, new i-bbb
	// Current state only has i-bbb
	current := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-bbb",
			},
		},
	}

	historical := []history.HistoricalResource{
		{
			URN:      "urn:pulumi:aws:ec2:instance:Web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			CloudIDs: []string{"i-aaa", "i-bbb"},
		},
	}

	result := cli.MergeHistoricalResources(current, historical)

	// Should have 2: original i-bbb + historical i-aaa
	require.Len(t, result, 2)

	cloudIDs := make(map[string]bool)
	for _, r := range result {
		if cid, ok := r.Properties["pulumi:cloudId"].(string); ok {
			cloudIDs[cid] = true
		}
	}
	assert.True(t, cloudIDs["i-aaa"])
	assert.True(t, cloudIDs["i-bbb"])
}

func TestMergeHistoricalResources_NilHistoryStoreNoRegression(t *testing.T) {
	// Without history store (nil historical slice), behavior unchanged
	current := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-original",
			},
		},
		{
			Type:     "aws:s3/bucket:Bucket",
			Provider: "aws",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "my-bucket",
			},
		},
	}

	result := cli.MergeHistoricalResources(current, nil)

	// No change — nil history means no merge
	require.Len(t, result, 2)
	assert.Equal(t, "i-original", result[0].Properties["pulumi:cloudId"])
	assert.Equal(t, "my-bucket", result[1].Properties["pulumi:cloudId"])
}
