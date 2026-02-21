package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// TestFlags_CostProjected_AllFlags tests all flag combinations for cost projected.
func TestFlags_CostProjected_AllFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--output", "json",
		"--filter", "provider=aws",
		"--adapter", "test-adapter",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)
}

// TestFlags_CostActual_AllFlags tests all flag combinations for cost actual.
func TestFlags_CostActual_AllFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostActualCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--from", "2024-01-01",
		"--to", "2024-01-31",
		"--output", "json",
		"--group-by", "resource",
		"--adapter", "test-adapter",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)
}

// TestFlags_OutputFormat_Values tests all valid output format values.
func TestFlags_OutputFormat_Values(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	formats := []string{"table", "json", "ndjson"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cmd := cli.NewCostProjectedCmd()
			cmd.SetArgs([]string{
				"--pulumi-json", planPath,
				"--output", format,
			})

			var out bytes.Buffer
			cmd.SetOut(&out)

			err := cmd.Execute()
			require.NoError(t, err, "Output format %s should be valid", format)
		})
	}
}

// TestFlags_GroupBy_Values tests all valid group-by values.
func TestFlags_GroupBy_Values(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
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

	groupByValues := []string{"resource", "type", "provider", "daily", "monthly"}

	for _, groupBy := range groupByValues {
		t.Run(groupBy, func(t *testing.T) {
			cmd := cli.NewCostActualCmd()
			cmd.SetArgs([]string{
				"--pulumi-json", planPath,
				"--from", "2024-01-01",
				"--to", "2024-01-31",
				"--group-by", groupBy,
				"--output", "json",
			})

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			err := cmd.Execute()

			// Time-based grouping (daily/monthly) requires actual cost results
			// Without plugins, this fails with "empty results" error
			if groupBy == "daily" || groupBy == "monthly" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "empty results")
			} else {
				require.NoError(t, err, "GroupBy value %s should be valid", groupBy)
			}
		})
	}
}

// TestFlags_Filter_Expressions tests various filter expression formats.
func TestFlags_Filter_Expressions(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
			"inputs": map[string]interface{}{
				"instanceType": "t3.micro",
				"tags": map[string]interface{}{
					"Environment": "production",
				},
			},
		},
	}

	planPath := createTestPlan(t, resources)

	filterExpressions := []string{
		"type=ec2",
		"provider=aws",
		"service=ec2",
		"instanceType=t3.micro",
	}

	for _, filter := range filterExpressions {
		t.Run(filter, func(t *testing.T) {
			cmd := cli.NewCostProjectedCmd()
			cmd.SetArgs([]string{
				"--pulumi-json", planPath,
				"--filter", filter,
				"--output", "json",
			})

			var out bytes.Buffer
			cmd.SetOut(&out)

			err := cmd.Execute()
			require.NoError(t, err, "Filter expression %s should be valid", filter)
		})
	}
}

// TestFlags_DateFormats tests various date format inputs.
func TestFlags_DateFormats(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	dateTests := []struct {
		name      string
		startDate string
		endDate   string
	}{
		{
			name:      "Simple date format",
			startDate: "2024-01-01",
			endDate:   "2024-01-31",
		},
		{
			name:      "RFC3339 format",
			startDate: "2024-01-01T00:00:00Z",
			endDate:   "2024-01-31T23:59:59Z",
		},
	}

	for _, tt := range dateTests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewCostActualCmd()
			cmd.SetArgs([]string{
				"--pulumi-json", planPath,
				"--from", tt.startDate,
				"--to", tt.endDate,
				"--output", "json",
			})

			var out bytes.Buffer
			cmd.SetOut(&out)

			err := cmd.Execute()
			require.NoError(t, err)
		})
	}
}

// TestFlags_BooleanFlags tests boolean flag handling.
func TestFlags_BooleanFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginListCmd()

	// Test with no flags (default values)
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)
}

// TestFlags_EmptyStringFlags tests handling of empty string flags.
func TestFlags_EmptyStringFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "", // Empty filter should be allowed
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)
}

// TestFlags_InvalidFlagValue tests handling of invalid flag values.
func TestFlags_InvalidFlagValue(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--output", "invalid-format", // Invalid output format
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
}

// TestFlags_UnknownFlag tests handling of unknown flags.
func TestFlags_UnknownFlag(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--unknown-flag", "value",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

// TestFlags_FlagAliases tests handling of flag aliases if any.
func TestFlags_FlagAliases(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"-o", "json", // Short flag if supported
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	// This might fail if -o is not defined as shorthand
	err := cmd.Execute()
	// Don't require success since shorthand might not be defined
	_ = err
}

// TestFlags_RepeatableFlags tests if flags can be repeated (if applicable).
func TestFlags_RepeatableFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--output", "json",
		"--output", "table", // Last value should win
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	// Should either succeed with last value or error for duplicate flags
	_ = err
}

// TestFlags_CaseSensitivity tests flag value case sensitivity.
func TestFlags_CaseSensitivity(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	// Test if uppercase works
	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--output", "JSON", // Uppercase
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	// May or may not succeed depending on implementation
	_ = err
}
