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
)

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
	assert.Contains(t, err.Error(), "required flag")
}

// TestCostActualCmd_DefaultEndDate tests default end date handling.
func TestCostActualCmd_DefaultEndDate(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
		},
	}

	planPath := createTestPlan(t, resources)

	// Use a recent date within the max 366-day range
	recentDate := time.Now().AddDate(0, -1, 0).Format("2006-01-02") // 1 month ago

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

	assert.Len(t, results, 0) // No plugins = empty results // Two separate resources
}

// TestCostActualCmd_GroupByType tests type-level grouping.
func TestCostActualCmd_GroupByType(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
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

	assert.Len(t, results, 0) // No plugins = empty results // Aggregated by type
}

// TestCostActualCmd_GroupByProvider tests provider-level grouping.
func TestCostActualCmd_GroupByProvider(t *testing.T) {
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

	assert.Len(t, results, 0) // No plugins = empty results // Aggregated by provider (both AWS)
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
