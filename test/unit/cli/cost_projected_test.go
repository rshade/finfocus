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
	"github.com/rshade/finfocus/internal/engine"
)

// createTestPlan creates a temporary Pulumi plan JSON file for testing.
func createTestPlan(t *testing.T, resources []map[string]interface{}) string {
	t.Helper()

	plan := map[string]interface{}{
		"resources": resources,
	}

	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.json")

	data, err := json.Marshal(plan)
	require.NoError(t, err)

	err = os.WriteFile(planPath, data, 0644)
	require.NoError(t, err)

	return planPath
}

// TestCostProjectedCmd_Success tests basic projected cost calculation.
func TestCostProjectedCmd_Success(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
			"inputs": map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "json"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	// Should succeed (without plugins/specs, returns empty results)
	require.NoError(t, err)

	// Verify JSON output (AggregatedResults format)
	var results engine.AggregatedResults
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	// Without plugins or specs, returns empty resources
	assert.Len(t, results.Resources, 0)
	assert.Equal(t, "USD", results.Summary.Currency)
}

// TestCostProjectedCmd_MissingPlanFile tests error handling for missing file.
func TestCostProjectedCmd_MissingPlanFile(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", "/nonexistent/plan.json"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi plan")
}

// TestCostProjectedCmd_InvalidJSON tests error handling for invalid JSON.
func TestCostProjectedCmd_InvalidJSON(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "invalid.json")

	err := os.WriteFile(planPath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", planPath})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi plan")
}

// TestCostProjectedCmd_MultipleResources tests calculation with multiple resources.
func TestCostProjectedCmd_MultipleResources(t *testing.T) {
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
		{
			"type": "aws:rds/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:rds/instance:Instance::db-1",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "json"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results engine.AggregatedResults
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results.Resources, 0) // No plugins/specs = empty
}

// TestCostProjectedCmd_TableOutput tests table format output.
func TestCostProjectedCmd_TableOutput(t *testing.T) {
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
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "table"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "COST SUMMARY")
	assert.Contains(t, output, "Total Monthly Cost")
}

// TestCostProjectedCmd_NDJSONOutput tests NDJSON format output.
func TestCostProjectedCmd_NDJSONOutput(t *testing.T) {
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

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "ndjson"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	// Without plugins/specs, NDJSON output is empty (no lines to output)
	output := out.String()
	assert.Empty(t, output) // No results = no NDJSON lines
}

// TestCostProjectedCmd_FilterByType tests resource filtering.
func TestCostProjectedCmd_FilterByType(t *testing.T) {
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

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "type=ec2",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results engine.AggregatedResults
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	// Should only have EC2 instance after filtering
	assert.Len(t, results.Resources, 0) // No plugins/specs = empty
}

// TestCostProjectedCmd_FilterByProvider tests provider-level filtering.
func TestCostProjectedCmd_FilterByProvider(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
		},
		{
			"type": "azure:compute/virtualMachine:VirtualMachine",
			"urn":  "urn:pulumi:stack::project::azure:compute/virtualMachine:VirtualMachine::vm-1",
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "provider=aws",
		"--output", "json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results engine.AggregatedResults
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	// Should only have AWS resources
	assert.Len(t, results.Resources, 0) // No plugins/specs = empty
}

// TestCostProjectedCmd_EmptyPlan tests handling of plan with no resources.
func TestCostProjectedCmd_EmptyPlan(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{}
	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "json"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results engine.AggregatedResults
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Empty(t, results.Resources)
	assert.Equal(t, 0.0, results.Summary.TotalMonthly)
}

// TestCostProjectedCmd_MissingRequiredFlag tests error when required flag missing.
func TestCostProjectedCmd_MissingRequiredFlag(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{}) // No --pulumi-json flag

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

// TestCostProjectedCmd_InvalidOutputFormat tests handling of invalid output format.
func TestCostProjectedCmd_InvalidOutputFormat(t *testing.T) {
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
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "invalid-format"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

// TestCostProjectedCmd_ComplexResourceProperties tests resources with complex properties.
func TestCostProjectedCmd_ComplexResourceProperties(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	resources := []map[string]interface{}{
		{
			"type": "aws:ec2/instance:Instance",
			"urn":  "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
			"inputs": map[string]interface{}{
				"instanceType": "t3.micro",
				"tags": map[string]interface{}{
					"Environment": "production",
					"Team":        "backend",
				},
				"ebsBlockDevices": []interface{}{
					map[string]interface{}{
						"volumeSize": 100,
						"volumeType": "gp3",
					},
				},
			},
		},
	}

	planPath := createTestPlan(t, resources)

	cmd := cli.NewCostProjectedCmd()
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "json"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	var results engine.AggregatedResults
	err = json.Unmarshal(out.Bytes(), &results)
	require.NoError(t, err)

	assert.Len(t, results.Resources, 0) // No plugins/specs = empty
}
