package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewCostProjectedCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no flags triggers auto-detection (T017)",
			args:        []string{},
			expectError: true,
			// Without pulumi-json, auto-detection kicks in and fails because
			// pulumi isn't available or no Pulumi project found.
			// The error must NOT be "required flag(s) \"pulumi-json\" not set".
			errorMsg: "",
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
				"--spec-dir", "/tmp/specs",
				"--adapter", "test-adapter",
				"--output", "json",
				"--filter", "type=aws:ec2/instance",
			},
			expectError: true, // Will fail because file doesn't exist
			errorMsg:    "loading Pulumi plan",
		},
		{
			name:        "with only pulumi-json flag",
			args:        []string{"--pulumi-json", "test.json"},
			expectError: true, // Will fail because file doesn't exist
			errorMsg:    "loading Pulumi plan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewCostProjectedCmd()
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

// TestCostProjectedWithoutPulumiJson verifies the command does not error with
// "required flag not set" when --pulumi-json is omitted (T017).
func TestCostProjectedWithoutPulumiJson(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostProjectedCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Command will error (no Pulumi project in test env), but the error
	// must NOT be about a required flag.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "required flag")
	assert.NotContains(t, err.Error(), "pulumi-json\" not set")
}

// TestCostProjectedFlagHelp verifies help text says "optional" for --pulumi-json (T018).
func TestCostProjectedFlagHelp(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewCostProjectedCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "optional")
	assert.Contains(t, output, "auto-detected")
	assert.NotContains(t, output, "(required)")
}

func TestCostProjectedCmdFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostProjectedCmd()

	// Check required flags
	pulumiJSONFlag := cmd.Flags().Lookup("pulumi-json")
	assert.NotNil(t, pulumiJSONFlag)
	assert.Equal(t, "string", pulumiJSONFlag.Value.Type())
	assert.Empty(t, pulumiJSONFlag.DefValue)

	// Check optional flags
	specDirFlag := cmd.Flags().Lookup("spec-dir")
	assert.NotNil(t, specDirFlag)
	assert.Equal(t, "string", specDirFlag.Value.Type())

	adapterFlag := cmd.Flags().Lookup("adapter")
	assert.NotNil(t, adapterFlag)
	assert.Equal(t, "string", adapterFlag.Value.Type())

	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "string", outputFlag.Value.Type())
	assert.Equal(t, "table", outputFlag.DefValue)

	filterFlag := cmd.Flags().Lookup("filter")
	assert.NotNil(t, filterFlag)
	assert.Equal(t, "stringArray", filterFlag.Value.Type())
	assert.Equal(t, "[]", filterFlag.DefValue)
}

func TestCostProjectedCmdHelp(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	var buf bytes.Buffer
	cmd := cli.NewCostProjectedCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Calculate projected costs")
	assert.Contains(t, output, "auto-detect")
	assert.Contains(t, output, "--pulumi-json")
	assert.Contains(t, output, "--spec-dir")
	assert.Contains(t, output, "--adapter")
	assert.Contains(t, output, "--output")
	assert.Contains(t, output, "--filter")
	assert.Contains(t, output, "Resource filter expressions")
}

func TestCostProjectedCmdExamples(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostProjectedCmd()

	// Check that examples are present
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "finfocus cost projected")
	assert.Contains(t, cmd.Example, "--pulumi-json plan.json")
	assert.Contains(t, cmd.Example, "--stack production")
	assert.Contains(t, cmd.Example, "--filter \"type=aws:ec2/instance\"")
	assert.Contains(t, cmd.Example, "--output json")
	assert.Contains(t, cmd.Example, "--adapter aws-plugin")
	assert.Contains(t, cmd.Example, "--spec-dir ./custom-specs")
}

// TestStackFlagExists verifies the --stack flag is present on the cost parent
// command and inherited by the projected subcommand (T028).
func TestStackFlagExists(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	root := cli.NewRootCmd("test")
	costCmd, _, err := root.Find([]string{"cost"})
	require.NoError(t, err)
	require.NotNil(t, costCmd)

	stackFlag := costCmd.PersistentFlags().Lookup("stack")
	require.NotNil(t, stackFlag, "--stack flag should be on cost parent command")
	assert.Equal(t, "string", stackFlag.Value.Type())
	assert.Equal(t, "", stackFlag.DefValue)
	assert.Contains(t, stackFlag.Usage, "auto-detection")
}

// TestStackFlagPassedThrough verifies that --stack is accepted during
// auto-detection (no --pulumi-json). The error should come from auto-detection,
// not from an unknown flag (T029).
func TestStackFlagPassedThrough(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	root := cli.NewRootCmd("test")
	root.SetArgs([]string{"cost", "projected", "--stack", "production"})

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	err := root.Execute()
	// Command errors because auto-detection fails (no pulumi binary or project),
	// but the --stack flag must be accepted without "unknown flag" error.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown flag")
	assert.NotContains(t, err.Error(), "unknown shorthand flag")
}

// TestStackFlagIgnoredWithPulumiJson verifies that --stack is ignored when
// --pulumi-json is provided. The error should be about loading the plan file,
// not about auto-detection or stack resolution (T030).
func TestStackFlagIgnoredWithPulumiJson(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	root := cli.NewRootCmd("test")
	root.SetArgs([]string{
		"cost", "projected",
		"--pulumi-json", "nonexistent.json",
		"--stack", "production",
	})

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	err := root.Execute()
	require.Error(t, err)
	// Should fail on file loading, not on stack resolution
	assert.Contains(t, err.Error(), "loading Pulumi plan")
}

func TestCostProjectedCmdErrorSummaryDisplay(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// This test verifies that the CLI correctly displays error summary after table output
	// when there are errors during cost calculation.
	//
	// Note: This is a structural test. Full integration testing with actual errors
	// requires a mock plugin that returns errors, which would be in test/integration/.
	// For now, we verify the command structure supports error display.

	cmd := cli.NewCostProjectedCmd()

	// Verify the command has the expected structure for error handling
	assert.NotNil(t, cmd.RunE, "Command should have RunE for error handling")

	// Verify output streams can be set (needed for error summary display)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Verify the command accepts the required flags
	pulumiJSONFlag := cmd.Flags().Lookup("pulumi-json")
	assert.NotNil(t, pulumiJSONFlag, "Should have pulumi-json flag for plan input")

	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "Should have output flag for format selection")
}
