//go:build nightly

// Package e2e_test provides end-to-end tests for the finfocus CLI workflows.
package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testBinaryPath string //nolint:gochecknoglobals // Required for TestMain pattern
var testTempDir string    //nolint:gochecknoglobals // Required for TestMain cleanup

func TestMain(m *testing.M) {
	// Build the binary once for all tests
	var err error
	testBinaryPath, testTempDir, err = buildFinFocusBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup: remove the entire temp directory (includes the binary)
	if testTempDir != "" {
		os.RemoveAll(testTempDir)
	}

	os.Exit(code)
}

// buildFinFocusBinary builds the finfocus binary from source
// for testing purposes, returning the path to the compiled executable and
// the temporary directory (for cleanup).
func buildFinFocusBinary() (string, string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "finfocus-e2e-*")
	if err != nil {
		return "", "", fmt.Errorf("creating temp dir: %w", err)
	}

	binaryName := "finfocus"
	if runtime.GOOS == "windows" {
		binaryName = "finfocus.exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)

	// Build the binary
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get the project root directory (3 levels up from test/integration/e2e)
	// Note: We assume the test is running from the package directory
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		return "", "", fmt.Errorf("resolving project root: %w", err)
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "./cmd/finfocus")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up temp dir on build failure
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("build failed: %w\nOutput: %s", err, string(output))
	}

	// Verify binary was created and is executable
	info, err := os.Stat(binaryPath)
	if err != nil {
		// Clean up temp dir on verification failure
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("binary not found after build: %w", err)
	}
	if !info.Mode().IsRegular() {
		// Clean up temp dir on verification failure
		os.RemoveAll(tempDir)
		return "", "", errors.New("binary is not a regular file")
	}

	return binaryPath, tempDir, nil
}

// TestE2E_ProjectedCostWorkflow tests the complete projected cost workflow from
// building the binary, loading a Pulumi plan, and outputting JSON results.
func TestE2E_ProjectedCostWorkflow(t *testing.T) {
	// Create isolated HOME directory to ensure no plugins are found
	tempHome := t.TempDir()

	// Use test fixture plan
	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")
	planPath, err := filepath.Abs(planFile)
	require.NoError(t, err)

	// Verify plan file exists
	_, err = os.Stat(planPath)
	require.NoError(t, err, "Test plan file not found: %s", planPath)

	// Run projected cost command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		testBinaryPath,
		"cost",
		"projected",
		"--pulumi-json",
		planPath,
		"--output",
		"json",
	)
	cmd.Env = append(os.Environ(), "HOME="+tempHome)
	output, err := cmd.Output()
	require.NoError(t, err, "Command failed: %v", err)

	// Parse JSON output (aggregated format wrapped in "finfocus" key)
	var aggregated map[string]interface{}
	err = json.Unmarshal(output, &aggregated)
	require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

	// Validate output structure - data is wrapped in "finfocus" key
	assert.Contains(t, aggregated, "finfocus")
	finfocusData, ok := aggregated["finfocus"].(map[string]interface{})
	require.True(t, ok, "finfocus field should be an object")
	assert.Contains(t, finfocusData, "summary")
	assert.Contains(t, finfocusData, "resources")

	results, ok := finfocusData["resources"].([]interface{})
	require.True(t, ok, "Resources field should be an array")
	assert.NotEmpty(t, results, "Expected cost results")

	for _, r := range results {
		result, resultOk := r.(map[string]interface{})
		require.True(t, resultOk, "Each result should be a map")

		assert.Contains(t, result, "resourceType")
		assert.Contains(t, result, "resourceId")
		assert.Contains(t, result, "adapter")
		assert.Contains(t, result, "currency")
		assert.Contains(t, result, "monthly")
		assert.Contains(t, result, "hourly")

		// Since no plugins are available, should use "none" adapter
		assert.Equal(t, "none", result["adapter"])
		assert.Equal(t, "USD", result["currency"])
	}
}

// TestE2E_ProjectedCostWorkflow_TableOutput tests the projected cost workflow with
// table output format to ensure formatting and resource type display are correct.
func TestE2E_ProjectedCostWorkflow_TableOutput(t *testing.T) {
	// Use multi-resource test plan
	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-multi-resource-plan.json")
	planPath, err := filepath.Abs(planFile)
	require.NoError(t, err)

	// Run projected cost command with table output
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		testBinaryPath,
		"cost",
		"projected",
		"--pulumi-json",
		planPath,
		"--output",
		"table",
	)
	output, err := cmd.Output()
	require.NoError(t, err, "Command failed: %v", err)

	outputStr := string(output)

	// Validate table output contains expected elements
	assert.Contains(t, outputStr, "Resource")
	assert.Contains(t, outputStr, "Adapter")
	assert.Contains(t, outputStr, "Monthly")
	assert.Contains(t, outputStr, "Currency")
	assert.Contains(t, outputStr, "COST SUMMARY")

	// Should contain resource types from the plan (in the format aws:service/type:Type)
	assert.Contains(t, outputStr, "aws:ec2/instance:Instance")
	assert.Contains(t, outputStr, "aws:s3/bucket:Bucket")
	assert.Contains(t, outputStr, "aws:rds/instance:Instance")
	assert.Contains(t, outputStr, "aws:lambda/function:Function")
}

// TestE2E_ActualCostWorkflow tests the actual cost calculation workflow with
// a time range query to validate actual cost command functionality.
func TestE2E_ActualCostWorkflow(t *testing.T) {
	// Run actual cost command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		testBinaryPath,
		"cost",
		"actual",
		"--from",
		"2024-01-01",
		"--to",
		"2024-01-31",
		"--output",
		"json",
	)
	output, err := cmd.Output()

	// This might fail if no plugins are available, which is expected
	if err != nil {
		t.Logf("Actual cost command failed as expected (no plugins): %v", err)
		return
	}

	// If it succeeds, validate the output (aggregated format)
	var aggregated map[string]interface{}
	err = json.Unmarshal(output, &aggregated)
	require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

	// Validate output structure if resources exist
	if resourcesArray, ok := aggregated["resources"].([]interface{}); ok {
		for _, r := range resourcesArray {
			if result, resultOk := r.(map[string]interface{}); resultOk {
				assert.Contains(t, result, "resourceType")
				assert.Contains(t, result, "resourceId")
				assert.Contains(t, result, "currency")
				assert.Contains(t, result, "monthly")
			}
		}
	}
}

// TestE2E_PluginListWorkflow tests the plugin list command to ensure it reports
// correctly when no plugins are installed.
func TestE2E_PluginListWorkflow(t *testing.T) {
	// Create an isolated HOME directory to ensure no plugins are found
	tempHome := t.TempDir()

	// Run plugin list command with isolated HOME
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, "plugin", "list")
	cmd.Env = append(os.Environ(), "HOME="+tempHome)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Plugin list command failed: %v\nOutput: %s", err, string(output))

	outputStr := string(output)

	// Should indicate no plugins are installed
	assert.Contains(t, outputStr, "No plugins installed")
}

// TestE2E_PluginValidateWorkflow tests the plugin validate command to ensure it reports
// correctly when no plugins are installed or available for validation.
func TestE2E_PluginValidateWorkflow(t *testing.T) {
	// Create an isolated HOME directory to ensure no plugins are found
	tempHome := t.TempDir()

	// Run plugin validate command with isolated HOME
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, "plugin", "validate")
	cmd.Env = append(os.Environ(), "HOME="+tempHome)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Plugin validate command failed: %v\nOutput: %s", err, string(output))

	outputStr := string(output)

	// Should indicate no plugins to validate
	assert.Contains(t, outputStr, "No plugins to validate")
}

// TestE2E_HelpCommands tests various help commands to ensure proper documentation
// and usage information is available for all major CLI commands.
func TestE2E_HelpCommands(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedContent string
	}{
		{
			name:            "root help",
			args:            []string{"--help"},
			expectedContent: "Calculate projected and actual cloud costs via plugins",
		},
		{
			name:            "cost help",
			args:            []string{"cost", "--help"},
			expectedContent: "Cost calculation commands",
		},
		{
			name:            "projected cost help",
			args:            []string{"cost", "projected", "--help"},
			expectedContent: "Calculate projected costs",
		},
		{
			name:            "actual cost help",
			args:            []string{"cost", "actual", "--help"},
			expectedContent: "Fetch actual historical costs",
		},
		{
			name:            "plugin help",
			args:            []string{"plugin", "--help"},
			expectedContent: "Plugin management commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, testBinaryPath, tt.args...)
			output, err := cmd.Output()
			require.NoError(t, err, "Help command failed: %v", err)

			outputStr := string(output)
			assert.Contains(t, outputStr, tt.expectedContent)
		})
	}
}

// TestE2E_InvalidInputHandling tests error handling for invalid inputs such as
// missing flags, invalid file paths, and invalid date formats.
func TestE2E_InvalidInputHandling(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "invalid command",
			args:        []string{"invalid-command"},
			expectError: true,
		},
		{
			name:        "missing required flag",
			args:        []string{"cost", "projected"},
			expectError: true,
		},
		{
			name:        "invalid file path",
			args:        []string{"cost", "projected", "--pulumi-json", "/nonexistent/file.json"},
			expectError: true,
		},
		{
			name:        "invalid date format",
			args:        []string{"cost", "actual", "--from", "invalid-date", "--to", "2024-01-31"},
			expectError: true,
		},
		{
			name: "invalid output format",
			args: []string{
				"cost",
				"projected",
				"--pulumi-json",
				"/dev/null",
				"--output",
				"invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, testBinaryPath, tt.args...)
			_, err := cmd.Output()

			if tt.expectError {
				assert.Error(t, err, "Expected command to fail")
			} else {
				assert.NoError(t, err, "Expected command to succeed")
			}
		})
	}
}

// TestE2E_MultipleOutputFormats tests that all supported output formats (json, table, ndjson)
// produce valid output and can be properly parsed.
func TestE2E_MultipleOutputFormats(t *testing.T) {
	// Use test fixture plan
	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")
	planPath, err := filepath.Abs(planFile)
	require.NoError(t, err)

	formats := []string{"json", "table", "ndjson"}

	for _, format := range formats {
		t.Run(format+"_format", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, testBinaryPath, "cost", "projected",
				"--pulumi-json", planPath, "--output", format)
			output, cmdErr := cmd.Output()
			require.NoError(t, cmdErr, "Command failed for format %s: %v", format, cmdErr)

			assert.NotEmpty(t, output, "Output should not be empty for format %s", format)

			if format == "json" || format == "ndjson" {
				// Verify it's valid JSON
				var jsonData interface{}
				if format == "json" {
					err = json.Unmarshal(output, &jsonData)
				} else {
					// For NDJSON, try to parse the first line
					lines := []byte{}
					for _, b := range output {
						if b == '\n' {
							break
						}
						lines = append(lines, b)
					}
					err = json.Unmarshal(lines, &jsonData)
				}
				assert.NoError(t, err, "Invalid JSON output for format %s", format)
			}
		})
	}
}
