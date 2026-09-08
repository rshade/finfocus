package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
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
		checkOutput func(t *testing.T, result axtest.Result)
	}{
		{
			name:    "cost projected basic",
			command: "cost",
			args:    []string{"projected", "--pulumi-json", planPath},
			checkOutput: func(t *testing.T, result axtest.Result) {
				// Should not error, even if no plugins are available
				require.Equal(t, 0, result.ExitCode)
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
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
				// Command should complete successfully
			},
		},
		{
			name:    "cost projected with json output",
			command: "cost",
			args:    []string{"projected", "--pulumi-json", planPath, "--output", "json"},
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
				// Should produce valid output for JSON format
			},
		},
		{
			name:    "cost actual basic",
			command: "cost",
			args:    []string{"actual", "--pulumi-json", planPath, "--from", "2025-12-01", "--to", "2025-12-31"},
			checkOutput: func(t *testing.T, result axtest.Result) {
				// Should succeed with explicit date range
				require.Equal(t, 0, result.ExitCode)
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
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
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
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
				// Command should complete successfully
			},
		},
		{
			name:    "plugin list",
			command: "plugin",
			args:    []string{"list"},
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
				// Command should succeed (no error check for specific output since it prints to different streams)
			},
		},
		{
			name:    "plugin list verbose",
			command: "plugin",
			args:    []string{"list", "--verbose"},
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
				// Command should succeed
			},
		},
		{
			name:    "plugin validate",
			command: "plugin",
			args:    []string{"validate"},
			checkOutput: func(t *testing.T, result axtest.Result) {
				require.Equal(t, 0, result.ExitCode)
				// Command should succeed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCmd("test-version")

			// Build full args
			args := []string{tt.command}
			args = append(args, tt.args...)

			result := axtest.Run(context.Background(), t, cmd, args)

			switch {
			case tt.checkOutput != nil:
				tt.checkOutput(t, result)
			case tt.expectError:
				require.NotEqual(t, 0, result.ExitCode)
			default:
				require.Equal(t, 0, result.ExitCode)
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
		errorCheck  func(t *testing.T, result axtest.Result)
	}{
		{
			name:        "no flags triggers auto-detection for projected",
			args:        []string{"cost", "projected"},
			expectError: true,
			errorCheck: func(t *testing.T, result axtest.Result) {
				// Auto-detection kicks in but fails (no Pulumi project in test env).
				// The error must NOT be about a required flag.
				assert.NotContains(t, string(result.Stderr), "required flag")
			},
		},
		{
			name:        "missing required from for actual",
			args:        []string{"cost", "actual", "--pulumi-json", "test.json"},
			expectError: true,
			errorCheck: func(t *testing.T, result axtest.Result) {
				assert.Contains(t, string(result.Stderr), "--from is required when using --pulumi-json")
			},
		},
		{
			name:        "nonexistent pulumi plan file",
			args:        []string{"cost", "projected", "--pulumi-json", "/nonexistent/file.json"},
			expectError: true,
			errorCheck: func(t *testing.T, result axtest.Result) {
				assert.Contains(t, string(result.Stderr), "loading Pulumi plan")
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
			errorCheck: func(t *testing.T, result axtest.Result) {
				assert.Contains(t, string(result.Stderr), "loading Pulumi plan")
			},
		},
		{
			name:        "unknown command",
			args:        []string{"unknown-command"},
			expectError: true,
			errorCheck: func(t *testing.T, result axtest.Result) {
				assert.Contains(t, string(result.Stderr), "unknown command")
			},
		},
		{
			name:        "unknown flag",
			args:        []string{"cost", "projected", "--unknown-flag", "value"},
			expectError: true,
			errorCheck: func(t *testing.T, result axtest.Result) {
				assert.Contains(t, string(result.Stderr), "unknown flag")
			},
		},
		{
			name:        "plugin validate specific nonexistent plugin",
			args:        []string{"plugin", "validate", "--plugin", "nonexistent-plugin"},
			expectError: true, // Plugin not found returns an error
			errorCheck: func(t *testing.T, result axtest.Result) {
				assert.Contains(t, string(result.Stderr), "plugin 'nonexistent-plugin' not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCmd("test-version")
			result := axtest.Run(context.Background(), t, cmd, tt.args)

			if tt.expectError {
				require.NotEqual(t, 0, result.ExitCode)
				if tt.errorCheck != nil {
					tt.errorCheck(t, result)
				}
			} else {
				require.Equal(t, 0, result.ExitCode)
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
			cmd := cli.NewRootCmd("test-version")
			result := axtest.Run(context.Background(), t, cmd,
				[]string{"cost", "projected", "--pulumi-json", planPath, "--output", format},
			)

			require.Equal(t, 0, result.ExitCode)
			// Should succeed
		})

		t.Run("actual_output_"+format, func(t *testing.T) {
			cmd := cli.NewRootCmd("test-version")
			result := axtest.Run(context.Background(), t, cmd,
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

			require.Equal(t, 0, result.ExitCode)
			// Should succeed
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
		cmd := cli.NewRootCmd("test-version")
		result := axtest.Run(context.Background(), t, cmd, []string{
			"cost", "projected",
			"--pulumi-json", planPath,
			"--output", "json",
			"--filter", "type=aws:ec2/instance",
			"--adapter", "test-adapter",
		})

		require.Equal(t, 0, result.ExitCode)
	})

	t.Run("actual_all_flags", func(t *testing.T) {
		cmd := cli.NewRootCmd("test-version")
		result := axtest.Run(context.Background(), t, cmd, []string{
			"cost", "actual",
			"--pulumi-json", planPath,
			"--from", "2025-12-01",
			"--to", "2026-01-02",
			"--output", "json",
			"--group-by", "type",
			"--adapter", "test-adapter",
		})

		require.Equal(t, 0, result.ExitCode)
	})

	t.Run("global_debug_flag", func(t *testing.T) {
		cmd := cli.NewRootCmd("test-version")
		result := axtest.Run(context.Background(), t, cmd, []string{
			"--debug",
			"cost", "projected",
			"--pulumi-json", planPath,
		})

		require.Equal(t, 0, result.ExitCode)
	})
}
