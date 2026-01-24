package cli_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewCostActualCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing required flags",
			args:        []string{},
			expectError: true,
			errorMsg:    "either --pulumi-json or --pulumi-state is required",
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
	assert.Contains(t, cmd.Example, "finfocus cost actual --pulumi-json plan.json --from")
	assert.Contains(t, cmd.Example, "to defaults to now")
	assert.Contains(t, cmd.Example, "--group-by type")
	assert.Contains(t, cmd.Example, "--group-by provider")
	assert.Contains(t, cmd.Example, "RFC3339 timestamps")
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
			name: "neither pulumi-json nor pulumi-state provided",
			args: []string{
				"--from", "2025-01-01",
			},
			expectError: true,
			errorMsg:    "either --pulumi-json or --pulumi-state",
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
