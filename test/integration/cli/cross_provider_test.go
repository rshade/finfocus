package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

// TestCrossProvider_MultiProviderPlan tests cross-provider aggregation with a multi-provider plan.
func TestCrossProvider_MultiProviderPlan(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Collect unique providers from resource types
	providers := make(map[string]bool)
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		if ok {
			parts := strings.Split(resType, ":")
			if len(parts) > 0 {
				providers[parts[0]] = true
			}
		}
	}

	// The multi-resource-plan.json should have multiple providers
	assert.GreaterOrEqual(t, len(providers), 2, "Should have multiple providers in results")
}

// TestCrossProvider_GroupByProvider tests cross-provider aggregation grouped by provider.
func TestCrossProvider_GroupByProvider(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "provider", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Results should be grouped by provider
	assert.NotEmpty(t, resources, "Should have provider-grouped results")
}

// TestCrossProvider_GroupByMonthly tests cross-provider aggregation with monthly grouping.
func TestCrossProvider_GroupByMonthly(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-03-31",
		"--group-by", "monthly", "--output", "json",
	)
	require.NoError(t, err)

	var aggregations []map[string]any
	err = json.Unmarshal([]byte(output), &aggregations)
	require.NoError(t, err)

	// Monthly grouping should produce time-based aggregations
	assert.NotEmpty(t, aggregations, "Should have monthly aggregations")
}

// TestCrossProvider_StateFile tests cross-provider aggregation with a multi-provider state file.
func TestCrossProvider_StateFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	stateFile := filepath.Join("..", "..", "..", "test", "fixtures", "state", "multi-provider.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", stateFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Should produce valid results from state file
	assert.NotNil(t, resources, "Should produce valid JSON output from state file")
}

// TestCrossProvider_FilterThenAggregate tests filtering followed by cross-provider aggregation.
func TestCrossProvider_FilterThenAggregate(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--filter", "tag:env=prod",
		"--group-by", "provider",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Should have filtered and then aggregated results
	assert.NotEmpty(t, resources, "Should have filtered and aggregated results")
}

// TestCrossProvider_CurrencyConsistency tests that all results use consistent currency.
func TestCrossProvider_CurrencyConsistency(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// All results should use the same currency (typically USD)
	currencies := make(map[string]bool)
	for _, res := range resources {
		currency, ok := res["currency"].(string)
		if ok && currency != "" {
			currencies[currency] = true
		}
	}

	// Should have at most one unique currency (or none if not specified)
	assert.LessOrEqual(t, len(currencies), 1, "All results should use consistent currency")
}

// TestCrossProvider_TableOutput tests cross-provider results in table format.
func TestCrossProvider_TableOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "provider",
		"--output", "table",
	)
	require.NoError(t, err)

	// Table output should have headers
	assert.Contains(t, output, "Resource", "Table should have Resource header")
	assert.Contains(t, output, "Total Cost", "Table should have Total Cost header")
}

// TestCrossProvider_NDJSONOutput tests cross-provider results in NDJSON format.
func TestCrossProvider_NDJSONOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "provider",
		"--output", "ndjson",
	)
	require.NoError(t, err)

	// NDJSON should be newline-delimited JSON
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.NotEmpty(t, lines, "Should have NDJSON lines")

	for _, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]any
		err = json.Unmarshal([]byte(line), &obj)
		assert.NoError(t, err, "Each NDJSON line should be valid JSON")
	}
}
