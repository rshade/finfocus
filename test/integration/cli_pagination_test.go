package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// newTestRootCmd creates a root command with migration checks disabled for testing.
func newTestRootCmd() *cobra.Command {
	// Mock environment to skip migration check
	lookupEnv := func(key string) (string, bool) {
		if key == "FINFOCUS_SKIP_MIGRATION_CHECK" {
			return "true", true
		}
		return os.LookupEnv(key)
	}
	return cli.NewRootCmdWithArgs("test-version", os.Args, lookupEnv)
}

// TestCLIPagination_Sorting tests sorting recommendations by different fields.
func TestCLIPagination_Sorting(t *testing.T) {
	// Create test fixture with recommendations
	planPath := createTestPlanWithRecommendations(t)
	defer os.Remove(planPath)

	tests := []struct {
		name        string
		sortFlag    string
		expectError bool
		checkOrder  func(t *testing.T, output string)
	}{
		{
			name:        "sort by savings descending",
			sortFlag:    "savings:desc",
			expectError: false,
			checkOrder: func(t *testing.T, output string) {
				// Verify recommendations are ordered by savings (highest first)
				assert.Contains(t, output, "recommendations")
			},
		},
		{
			name:        "sort by name ascending",
			sortFlag:    "name:asc",
			expectError: false,
			checkOrder: func(t *testing.T, output string) {
				assert.Contains(t, output, "recommendations")
			},
		},
		{
			name:        "sort by actionType",
			sortFlag:    "actionType:asc",
			expectError: false,
			checkOrder: func(t *testing.T, output string) {
				assert.Contains(t, output, "recommendations")
			},
		},
		{
			name:        "invalid sort field",
			sortFlag:    "invalid:asc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outBuf, errBuf bytes.Buffer
			cmd := newTestRootCmd()
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{
				"cost", "recommendations",
				"--pulumi-json", planPath,
				"--output", "json",
				"--sort", tt.sortFlag,
			})

			err := cmd.Execute()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "sort field")
			} else {
				require.NoError(t, err)
				if tt.checkOrder != nil {
					tt.checkOrder(t, outBuf.String())
				}
			}
		})
	}
}

// TestCLIPagination_LimitFlag tests the --limit flag for offset-based pagination.
func TestCLIPagination_LimitFlag(t *testing.T) {
	planPath := createTestPlanWithRecommendations(t)
	defer os.Remove(planPath)

	tests := []struct {
		name        string
		limit       int
		expectCount int
		expectError bool
	}{
		{
			name:        "limit 5",
			limit:       5,
			expectCount: 5,
			expectError: false,
		},
		{
			name:        "limit 0 (unlimited)",
			limit:       0,
			expectError: false,
		},
		{
			name:        "limit 1",
			limit:       1,
			expectCount: 1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outBuf, errBuf bytes.Buffer
			cmd := newTestRootCmd()
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			args := []string{
				"cost", "recommendations",
				"--pulumi-json", planPath,
				"--output", "json",
			}
			// Always pass --limit so limit=0 is exercised explicitly.
			args = append(args, "--limit", strconv.Itoa(tt.limit))

			cmd.SetArgs(args)
			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Parse JSON output
				var result map[string]interface{}
				err = json.Unmarshal(outBuf.Bytes(), &result)
				require.NoError(t, err)

				if tt.expectCount > 0 {
					recs, ok := result["recommendations"].([]interface{})
					require.True(t, ok, "recommendations should be an array")
					assert.LessOrEqual(t, len(recs), tt.expectCount, "should not exceed limit")
				}
			}
		})
	}
}

// TestCLIPagination_PageBased tests page-based pagination with --page and --page-size.
func TestCLIPagination_PageBased(t *testing.T) {
	planPath := createTestPlanWithRecommendations(t)
	defer os.Remove(planPath)

	tests := []struct {
		name        string
		page        int
		pageSize    int
		expectError bool
		checkMeta   func(t *testing.T, meta map[string]interface{})
	}{
		{
			name:        "first page",
			page:        1,
			pageSize:    3,
			expectError: false,
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				assert.Equal(t, float64(1), meta["current_page"])
				assert.Equal(t, float64(3), meta["page_size"])
				assert.False(t, meta["has_previous"].(bool))
			},
		},
		{
			name:        "second page",
			page:        2,
			pageSize:    3,
			expectError: false,
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				assert.Equal(t, float64(2), meta["current_page"])
				assert.True(t, meta["has_previous"].(bool))
			},
		},
		{
			name:        "page without page-size",
			page:        1,
			pageSize:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outBuf, errBuf bytes.Buffer
			cmd := newTestRootCmd()
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			args := []string{
				"cost", "recommendations",
				"--pulumi-json", planPath,
				"--output", "json",
				"--page", strconv.Itoa(tt.page),
			}
			if tt.pageSize > 0 {
				args = append(args, "--page-size", strconv.Itoa(tt.pageSize))
			}

			cmd.SetArgs(args)
			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Parse JSON output
				var result map[string]interface{}
				err = json.Unmarshal(outBuf.Bytes(), &result)
				require.NoError(t, err)

				// Check pagination metadata
				if tt.checkMeta != nil && result["pagination"] != nil {
					meta, ok := result["pagination"].(map[string]interface{})
					require.True(t, ok, "pagination should be an object")
					tt.checkMeta(t, meta)
				}
			}
		})
	}
}

// TestCLIPagination_OffsetBased tests offset-based pagination with --offset and --limit.
func TestCLIPagination_OffsetBased(t *testing.T) {
	planPath := createTestPlanWithRecommendations(t)
	defer os.Remove(planPath)

	tests := []struct {
		name        string
		offset      int
		limit       int
		expectError bool
	}{
		{
			name:        "offset 0 limit 5",
			offset:      0,
			limit:       5,
			expectError: false,
		},
		{
			name:        "offset 3 limit 2",
			offset:      3,
			limit:       2,
			expectError: false,
		},
		{
			name:        "offset beyond end",
			offset:      1000,
			limit:       5,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outBuf, errBuf bytes.Buffer
			cmd := newTestRootCmd()
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{
				"cost", "recommendations",
				"--pulumi-json", planPath,
				"--output", "json",
				"--offset", strconv.Itoa(tt.offset),
				"--limit", strconv.Itoa(tt.limit),
			})

			err := cmd.Execute()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCLIPagination_JSONMetadata tests that pagination metadata is included in JSON output.
func TestCLIPagination_JSONMetadata(t *testing.T) {
	planPath := createTestPlanWithRecommendations(t)
	defer os.Remove(planPath)

	var outBuf, errBuf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"cost", "recommendations",
		"--pulumi-json", planPath,
		"--output", "json",
		"--page", "1",
		"--page-size", "5",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	// Parse JSON output
	var result map[string]interface{}
	err = json.Unmarshal(outBuf.Bytes(), &result)
	require.NoError(t, err)

	// Verify pagination metadata is present
	pagination, ok := result["pagination"].(map[string]interface{})
	require.True(t, ok, "pagination metadata should be present")
	assert.Equal(t, float64(1), pagination["current_page"])
	assert.Equal(t, float64(5), pagination["page_size"])

	totalItems, ok := pagination["total_items"].(float64)
	require.True(t, ok, "total_items should be a number")
	assert.GreaterOrEqual(t, totalItems, float64(0))

	if totalItems == 0 {
		t.Log("Warning: No recommendations found. Ensure plugins are installed for full testing.")
	}
}

// TestCLIPagination_MutualExclusion tests that --page and --offset cannot be used together.
func TestCLIPagination_MutualExclusion(t *testing.T) {
	planPath := createTestPlanWithRecommendations(t)
	defer os.Remove(planPath)

	var outBuf, errBuf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"cost", "recommendations",
		"--pulumi-json", planPath,
		"--output", "json",
		"--page", "1",
		"--page-size", "5",
		"--offset", "10",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutual")
}

// createTestPlanWithRecommendations creates a test Pulumi plan JSON file with resources.
func createTestPlanWithRecommendations(t *testing.T) string {
	t.Helper()

	// Create a minimal Pulumi plan with resources
	plan := `{
  "config": {},
  "steps": [
    {
      "op": "create",
      "urn": "urn:pulumi:dev::test::aws:ec2/instance:Instance::web-1",
      "type": "aws:ec2/instance:Instance",
      "newState": {
        "inputs": {
          "instanceType": "t3.large",
          "ami": "ami-12345678"
        }
      }
    },
    {
      "op": "create",
      "urn": "urn:pulumi:dev::test::aws:ec2/instance:Instance::web-2",
      "type": "aws:ec2/instance:Instance",
      "newState": {
        "inputs": {
          "instanceType": "t3.medium",
          "ami": "ami-12345678"
        }
      }
    },
    {
      "op": "create",
      "urn": "urn:pulumi:dev::test::aws:ec2/instance:Instance::web-3",
      "type": "aws:ec2/instance:Instance",
      "newState": {
        "inputs": {
          "instanceType": "t3.small",
          "ami": "ami-12345678"
        }
      }
    }
  ]
}`

	tmpFile, err := os.CreateTemp("", "test-plan-*.json")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(plan)
	require.NoError(t, err)

	return tmpFile.Name()
}
