package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// =============================================================================
// T017: BuildCostSummary Tests
// =============================================================================

func TestBuildCostSummary(t *testing.T) {
	t.Run("normal costs", func(t *testing.T) {
		costs := []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web1",
				Currency:     "USD",
				Monthly:      100.0,
				Adapter:      "aws-public",
			},
			{
				ResourceType: "aws:rds/instance:Instance",
				ResourceID:   "db1",
				Currency:     "USD",
				Monthly:      250.0,
				Adapter:      "aws-public",
			},
		}

		summary := BuildCostSummary(costs, "dev", "my-infra")

		assert.Equal(t, costSummarySchemaVersion, summary.SchemaVersion)
		assert.NotEmpty(t, summary.Timestamp)
		assert.Equal(t, "dev", summary.Stack)
		assert.Equal(t, "my-infra", summary.Project)
		assert.Equal(t, 350.0, summary.TotalMonthlyCost)
		assert.Equal(t, "USD", summary.Currency)
		assert.Equal(t, 2, summary.ResourceCount)
		assert.False(t, summary.MixedCurrencies)
		require.Len(t, summary.Resources, 2)
		assert.Equal(t, "aws:ec2/instance:Instance", summary.Resources[0].Type)
		assert.Equal(t, "web1", summary.Resources[0].Name)
		assert.Equal(t, 100.0, summary.Resources[0].MonthlyCost)
		assert.Equal(t, "aws-public", summary.Resources[0].Adapter)
	})

	t.Run("mixed currencies detection", func(t *testing.T) {
		costs := []engine.CostResult{
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 100.0},
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web2", Currency: "EUR", Monthly: 200.0},
		}

		summary := BuildCostSummary(costs, "prod", "infra")

		assert.True(t, summary.MixedCurrencies)
		assert.Equal(t, 2, summary.ResourceCount)
	})

	t.Run("error resources excluded from total", func(t *testing.T) {
		costs := []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web1",
				Currency:     "USD",
				Monthly:      100.0,
				Adapter:      "aws-public",
			},
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web2",
				Currency:     "USD",
				Monthly:      0,
				Notes:        "ERROR: Plugin failed",
				Error:        &engine.StructuredError{Code: "PLUGIN_ERROR"},
			},
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web3",
				Currency:     "USD",
				Monthly:      0,
				Notes:        "VALIDATION: missing field",
			},
		}

		summary := BuildCostSummary(costs, "dev", "infra")

		assert.Equal(t, 100.0, summary.TotalMonthlyCost)
		assert.Equal(t, 1, summary.ResourceCount)
		require.Len(t, summary.Resources, 1)
		assert.Equal(t, "web1", summary.Resources[0].Name)
	})

	t.Run("empty cost list", func(t *testing.T) {
		summary := BuildCostSummary([]engine.CostResult{}, "dev", "infra")

		assert.Equal(t, costSummarySchemaVersion, summary.SchemaVersion)
		assert.Equal(t, 0.0, summary.TotalMonthlyCost)
		assert.Equal(t, "USD", summary.Currency)
		assert.Equal(t, 0, summary.ResourceCount)
		assert.False(t, summary.MixedCurrencies)
		assert.Empty(t, summary.Resources)
	})

	t.Run("resource count accuracy", func(t *testing.T) {
		costs := []engine.CostResult{
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 50.0},
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web2",
				Currency:     "USD",
				Monthly:      0,
			}, // Zero cost but valid
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web3", Currency: "USD", Monthly: 75.0},
		}

		summary := BuildCostSummary(costs, "dev", "infra")

		// All 3 are valid (no errors), all counted
		assert.Equal(t, 3, summary.ResourceCount)
		require.Len(t, summary.Resources, 3)
		assert.Equal(t, 125.0, summary.TotalMonthlyCost)
	})

	t.Run("nil cost list", func(t *testing.T) {
		summary := BuildCostSummary(nil, "dev", "infra")

		assert.Equal(t, 0.0, summary.TotalMonthlyCost)
		assert.Equal(t, 0, summary.ResourceCount)
		assert.Empty(t, summary.Resources)
	})
}

// =============================================================================
// T018: WriteCostSummary Tests
// =============================================================================

func TestWriteCostSummary(t *testing.T) {
	t.Run("successful write and read back", func(t *testing.T) {
		dir := t.TempDir()
		summary := &CostSummary{
			SchemaVersion:    "1",
			Timestamp:        "2025-06-15T10:30:00Z",
			Stack:            "dev",
			Project:          "my-infra",
			TotalMonthlyCost: 1250.50,
			Currency:         "USD",
			ResourceCount:    2,
			Resources: []ResourceCost{
				{
					Type:        "aws:ec2/instance:Instance",
					Name:        "web1",
					MonthlyCost: 100.0,
					Currency:    "USD",
					Adapter:     "aws-public",
				},
				{
					Type:        "aws:rds/instance:Instance",
					Name:        "db1",
					MonthlyCost: 1150.50,
					Currency:    "USD",
					Adapter:     "aws-public",
				},
			},
		}

		err := WriteCostSummary(summary, dir)
		require.NoError(t, err)

		// Read back and verify
		data, readErr := os.ReadFile(filepath.Join(dir, costSummaryFilename))
		require.NoError(t, readErr)

		var readBack CostSummary
		require.NoError(t, json.Unmarshal(data, &readBack))
		assert.Equal(t, "1", readBack.SchemaVersion)
		assert.Equal(t, "dev", readBack.Stack)
		assert.Equal(t, "my-infra", readBack.Project)
		assert.Equal(t, 1250.50, readBack.TotalMonthlyCost)
		assert.Equal(t, "USD", readBack.Currency)
		assert.Equal(t, 2, readBack.ResourceCount)
		require.Len(t, readBack.Resources, 2)
	})

	t.Run("atomic overwrite of existing file", func(t *testing.T) {
		dir := t.TempDir()
		first := &CostSummary{
			SchemaVersion:    "1",
			Timestamp:        "2025-06-15T10:00:00Z",
			Stack:            "dev",
			Project:          "infra",
			TotalMonthlyCost: 100.0,
			Currency:         "USD",
			ResourceCount:    1,
			Resources: []ResourceCost{
				{Type: "aws:ec2/instance:Instance", Name: "old", MonthlyCost: 100.0, Currency: "USD", Adapter: "test"},
			},
		}
		require.NoError(t, WriteCostSummary(first, dir))

		second := &CostSummary{
			SchemaVersion:    "1",
			Timestamp:        "2025-06-15T11:00:00Z",
			Stack:            "dev",
			Project:          "infra",
			TotalMonthlyCost: 200.0,
			Currency:         "USD",
			ResourceCount:    1,
			Resources: []ResourceCost{
				{Type: "aws:ec2/instance:Instance", Name: "new", MonthlyCost: 200.0, Currency: "USD", Adapter: "test"},
			},
		}
		require.NoError(t, WriteCostSummary(second, dir))

		// Should contain the second write
		data, err := os.ReadFile(filepath.Join(dir, costSummaryFilename))
		require.NoError(t, err)

		var readBack CostSummary
		require.NoError(t, json.Unmarshal(data, &readBack))
		assert.Equal(t, 200.0, readBack.TotalMonthlyCost)
		assert.Equal(t, "new", readBack.Resources[0].Name)
	})

	t.Run("directory creation if missing", func(t *testing.T) {
		base := t.TempDir()
		nestedDir := filepath.Join(base, "nested", "deep", ".finfocus")

		summary := &CostSummary{
			SchemaVersion: "1",
			Timestamp:     "2025-06-15T10:30:00Z",
			Stack:         "dev",
			Project:       "infra",
			Currency:      "USD",
			Resources:     []ResourceCost{},
		}

		err := WriteCostSummary(summary, nestedDir)
		require.NoError(t, err)

		// Verify file exists
		_, statErr := os.Stat(filepath.Join(nestedDir, costSummaryFilename))
		require.NoError(t, statErr)
	})

	t.Run("file permissions 0600", func(t *testing.T) {
		dir := t.TempDir()
		summary := &CostSummary{
			SchemaVersion: "1",
			Timestamp:     "2025-06-15T10:30:00Z",
			Stack:         "dev",
			Project:       "infra",
			Currency:      "USD",
			Resources:     []ResourceCost{},
		}

		require.NoError(t, WriteCostSummary(summary, dir))

		info, err := os.Stat(filepath.Join(dir, costSummaryFilename))
		require.NoError(t, err)
		// On Unix, verify permissions are 0600
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("JSON schema validity", func(t *testing.T) {
		dir := t.TempDir()
		summary := &CostSummary{
			SchemaVersion:    "1",
			Timestamp:        "2025-06-15T10:30:00Z",
			Stack:            "dev",
			Project:          "my-infra",
			TotalMonthlyCost: 350.0,
			Currency:         "USD",
			ResourceCount:    2,
			MixedCurrencies:  false,
			Resources: []ResourceCost{
				{
					Type:        "aws:ec2/instance:Instance",
					Name:        "web1",
					MonthlyCost: 100.0,
					Currency:    "USD",
					Adapter:     "aws-public",
				},
				{
					Type:        "aws:rds/instance:Instance",
					Name:        "db1",
					MonthlyCost: 250.0,
					Currency:    "USD",
					Adapter:     "aws-public",
				},
			},
		}

		require.NoError(t, WriteCostSummary(summary, dir))

		data, err := os.ReadFile(filepath.Join(dir, costSummaryFilename))
		require.NoError(t, err)

		// Verify required fields exist by unmarshaling into map
		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &raw))

		// All required fields from schema
		for _, field := range []string{"schema_version", "timestamp", "stack", "project", "total_monthly_cost", "currency", "resource_count", "resources"} {
			assert.Contains(t, raw, field, "missing required field: %s", field)
		}

		// Verify resource fields
		resources, ok := raw["resources"].([]interface{})
		require.True(t, ok)
		require.Len(t, resources, 2)

		res0, ok := resources[0].(map[string]interface{})
		require.True(t, ok)
		for _, field := range []string{"type", "name", "monthly_cost", "currency", "adapter"} {
			assert.Contains(t, res0, field, "missing required resource field: %s", field)
		}
	})
}
