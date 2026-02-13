package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// TestCLIIntegration tests the full CLI workflow with realistic scenarios.
func TestCLIIntegration(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set FINFOCUS_HOME to a temp directory to avoid loading local plugins
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a mock Pulumi plan file
	mockPlan := map[string]interface{}{
		"config": map[string]interface{}{},
		"planned_changes": []interface{}{
			map[string]interface{}{
				"urn":  "urn:pulumi:test::test::aws:ec2/instance:Instance::web-server",
				"type": "aws:ec2/instance:Instance",
				"inputs": map[string]interface{}{
					"instanceType": "t3.micro",
					"ami":          "ami-12345678",
				},
			},
		},
	}

	planPath := filepath.Join(tmpDir, "test-plan.json")
	planData, err := json.Marshal(mockPlan)
	require.NoError(t, err)
	err = os.WriteFile(planPath, planData, 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		command     string
		args        []string
		expectError bool
		checkOutput func(t *testing.T, output string, err error)
	}{
		{
			name:    "cost projected basic",
			command: "cost",
			args:    []string{"projected", "--pulumi-json", planPath},
			checkOutput: func(t *testing.T, _ string, err error) {
				// Should not error, even if no plugins are available
				require.NoError(t, err)
				// Command should complete successfully
			},
		},
		{
			name:    "cost projected with filter",
			command: "cost",
			args: []string{
				"projected",
				"--pulumi-json",
				planPath,
				"--filter",
				"type=aws:ec2/instance",
			},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Command should complete successfully
			},
		},
		{
			name:    "cost projected with json output",
			command: "cost",
			args:    []string{"projected", "--pulumi-json", planPath, "--output", "json"},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Should produce valid output for JSON format
			},
		},
		{
			name:    "cost actual basic",
			command: "cost",
			args:    []string{"actual", "--pulumi-json", planPath, "--from", "2025-12-01", "--to", "2025-12-31"},
			checkOutput: func(t *testing.T, _ string, err error) {
				// Should succeed with explicit date range
				require.NoError(t, err)
				// Command should complete successfully
			},
		},
		{
			name:    "cost actual with date range",
			command: "cost",
			args: []string{
				"actual",
				"--pulumi-json",
				planPath,
				"--from",
				"2025-12-01",
				"--to",
				"2026-01-02",
			},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Command should complete successfully
			},
		},
		{
			name:    "cost actual with group-by",
			command: "cost",
			args: []string{
				"actual",
				"--pulumi-json",
				planPath,
				"--from",
				"2025-12-01",
				"--to",
				"2025-12-31",
				"--group-by",
				"type",
			},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Command should complete successfully
			},
		},
		{
			name:    "plugin list",
			command: "plugin",
			args:    []string{"list"},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Command should succeed (no error check for specific output since it prints to different streams)
			},
		},
		{
			name:    "plugin list verbose",
			command: "plugin",
			args:    []string{"list", "--verbose"},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Command should succeed
			},
		},
		{
			name:    "plugin validate",
			command: "plugin",
			args:    []string{"validate"},
			checkOutput: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
				// Command should succeed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewRootCmd("test-version")
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Build full args
			args := []string{tt.command}
			args = append(args, tt.args...)
			cmd.SetArgs(args)

			execErr := cmd.Execute()

			switch {
			case tt.checkOutput != nil:
				tt.checkOutput(t, buf.String(), execErr)
			case tt.expectError:
				require.Error(t, execErr)
			default:
				require.NoError(t, execErr)
			}
		})
	}
}

// TestErrorHandlingEdgeCases tests various error conditions and edge cases.
func TestErrorHandlingEdgeCases(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set FINFOCUS_HOME to a temp directory to avoid loading local plugins
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name:        "no flags triggers auto-detection for projected",
			args:        []string{"cost", "projected"},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				// Auto-detection kicks in but fails (no Pulumi project in test env).
				// The error must NOT be about a required flag.
				assert.NotContains(t, err.Error(), "required flag")
			},
		},
		{
			name:        "missing required from for actual",
			args:        []string{"cost", "actual", "--pulumi-json", "test.json"},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "--from is required when using --pulumi-json")
			},
		},
		{
			name:        "nonexistent pulumi plan file",
			args:        []string{"cost", "projected", "--pulumi-json", "/nonexistent/file.json"},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "loading Pulumi plan")
			},
		},
		{
			name: "invalid date format",
			args: []string{
				"cost",
				"actual",
				"--pulumi-json",
				"test.json",
				"--from",
				"invalid-date",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "loading Pulumi plan")
			},
		},
		{
			name:        "unknown command",
			args:        []string{"unknown-command"},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "unknown command")
			},
		},
		{
			name:        "unknown flag",
			args:        []string{"cost", "projected", "--unknown-flag", "value"},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "unknown flag")
			},
		},
		{
			name:        "plugin validate specific nonexistent plugin",
			args:        []string{"plugin", "validate", "--plugin", "nonexistent-plugin"},
			expectError: true, // Plugin not found returns an error
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "plugin 'nonexistent-plugin' not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewRootCmd("test-version")
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestDateParsingEdgeCases tests edge cases in date parsing.
func TestDateParsingEdgeCases(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tests := []struct {
		name        string
		fromDate    string
		toDate      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid YYYY-MM-DD dates",
			fromDate:    "2025-01-01",
			toDate:      "2025-01-31",
			expectError: false,
		},
		{
			name:        "valid RFC3339 dates",
			fromDate:    "2025-01-01T00:00:00Z",
			toDate:      "2025-01-31T23:59:59Z",
			expectError: false,
		},
		{
			name:        "same date",
			fromDate:    "2025-01-15",
			toDate:      "2025-01-15",
			expectError: true,
			errorMsg:    "'to' date must be after 'from' date",
		},
		{
			name:        "to before from",
			fromDate:    "2025-01-31",
			toDate:      "2025-01-01",
			expectError: true,
			errorMsg:    "'to' date must be after 'from' date",
		},
		{
			name:        "invalid from format",
			fromDate:    "01-01-2025",
			toDate:      "2025-01-31",
			expectError: true,
			errorMsg:    "parsing 'from' date",
		},
		{
			name:        "invalid to format",
			fromDate:    "2025-01-01",
			toDate:      "31-01-2025",
			expectError: true,
			errorMsg:    "parsing 'to' date",
		},
		{
			name:        "partial RFC3339",
			fromDate:    "2025-01-01T10:30:00",
			toDate:      "2025-01-31T23:59:59",
			expectError: true,
			errorMsg:    "unable to parse date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, err := cli.ParseTimeRange(tt.fromDate, tt.toDate)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.False(t, from.IsZero())
				assert.False(t, to.IsZero())
				assert.True(t, to.After(from) || to.Equal(from))
			}
		})
	}
}

// TestOutputFormats tests different output format validation.
func TestOutputFormats(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set FINFOCUS_HOME to a temp directory to avoid loading local plugins
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	// Create a temporary plan file
	tmpDir := t.TempDir()

	planPath := filepath.Join(tmpDir, "test-plan.json")
	planContent := `{
		"config": {},
		"planned_changes": [
			{
				"urn": "urn:pulumi:test::test::aws:ec2/instance:Instance::web-server",
				"type": "aws:ec2/instance:Instance"
			}
		]
	}`
	err := os.WriteFile(planPath, []byte(planContent), 0644)
	require.NoError(t, err)

	formats := []string{"table", "json", "ndjson"}

	for _, format := range formats {
		t.Run("projected_output_"+format, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewRootCmd("test-version")
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(
				[]string{"cost", "projected", "--pulumi-json", planPath, "--output", format},
			)

			execErr := cmd.Execute()
			require.NoError(t, execErr)
			// Should succeed
			require.NoError(t, execErr)
		})

		t.Run("actual_output_"+format, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewRootCmd("test-version")
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(
				[]string{
					"cost",
					"actual",
					"--pulumi-json",
					planPath,
					"--from",
					"2025-12-01",
					"--to",
					"2025-12-31",
					"--output",
					format,
				},
			)

			execErr := cmd.Execute()
			require.NoError(t, execErr)
			// Should succeed
			require.NoError(t, execErr)
		})
	}
}

// TestFlagCombinations tests various flag combinations.
func TestFlagCombinations(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set FINFOCUS_HOME to a temp directory to avoid loading local plugins
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	// Create a temporary plan file
	tmpDir := t.TempDir()

	planPath := filepath.Join(tmpDir, "test-plan.json")
	planContent := `{
		"config": {},
		"planned_changes": [
			{
				"urn": "urn:pulumi:test::test::aws:ec2/instance:Instance::web-server",
				"type": "aws:ec2/instance:Instance"
			}
		]
	}`
	err := os.WriteFile(planPath, []byte(planContent), 0644)
	require.NoError(t, err)

	t.Run("projected_all_flags", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := cli.NewRootCmd("test-version")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{
			"cost", "projected",
			"--pulumi-json", planPath,
			"--output", "json",
			"--filter", "type=aws:ec2/instance",
			"--adapter", "test-adapter",
		})

		execErr := cmd.Execute()
		require.NoError(t, execErr)
	})

	t.Run("actual_all_flags", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := cli.NewRootCmd("test-version")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{
			"cost", "actual",
			"--pulumi-json", planPath,
			"--from", "2025-12-01",
			"--to", "2026-01-02",
			"--output", "json",
			"--group-by", "type",
			"--adapter", "test-adapter",
		})

		execErr := cmd.Execute()
		require.NoError(t, execErr)
	})

	t.Run("global_debug_flag", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := cli.NewRootCmd("test-version")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{
			"--debug",
			"cost", "projected",
			"--pulumi-json", planPath,
		})

		execErr := cmd.Execute()
		require.NoError(t, execErr)
	})
}
