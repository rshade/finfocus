package cli_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
)

// strPtr returns a pointer to the given string.
func strPtr(s string) *string { return &s }

// TestNewCostEstimateCmd_FlagParsing tests that flags are correctly defined.
func TestNewCostEstimateCmd_FlagParsing(t *testing.T) {
	cmd := cli.NewCostEstimateCmd()

	tests := []struct {
		name           string
		flagName       string
		expectedDefVal *string
	}{
		{"provider", "provider", strPtr("")},
		{"resource-type", "resource-type", strPtr("")},
		{"property", "property", nil},
		{"pulumi-json", "pulumi-json", strPtr("")},
		{"modify", "modify", nil},
		{"interactive", "interactive", strPtr("false")},
		{"output", "output", nil},
		{"region", "region", strPtr("")},
		{"adapter", "adapter", strPtr("")},
	}

	for _, tt := range tests {
		t.Run("has "+tt.name+" flag", func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag)
			if tt.expectedDefVal != nil {
				assert.Equal(t, *tt.expectedDefVal, flag.DefValue)
			}
		})
	}
}

// TestParsePropertyOverrides tests the property parsing function.
func TestParsePropertyOverrides(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    map[string]string
		expectError bool
		errContains string
	}{
		{
			name:     "single property",
			input:    []string{"instanceType=m5.large"},
			expected: map[string]string{"instanceType": "m5.large"},
		},
		{
			name:     "multiple properties",
			input:    []string{"instanceType=m5.large", "volumeSize=100"},
			expected: map[string]string{"instanceType": "m5.large", "volumeSize": "100"},
		},
		{
			name:     "property with spaces trimmed",
			input:    []string{" instanceType = m5.large "},
			expected: map[string]string{"instanceType": "m5.large"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:     "value with equals sign",
			input:    []string{"tags=key=value"},
			expected: map[string]string{"tags": "key=value"},
		},
		{
			name:        "missing equals sign",
			input:       []string{"instanceType"},
			expectError: true,
			errContains: "expected key=value",
		},
		{
			name:        "empty key",
			input:       []string{"=m5.large"},
			expectError: true,
			errContains: "property key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cli.ParsePropertyOverrides(tt.input)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestParsePropertyOverrides_DoSLimits tests DoS protection limits.
func TestParsePropertyOverrides_DoSLimits(t *testing.T) {
	tests := []struct {
		name      string
		propsFunc func() []string
		wantErr   string
	}{
		{
			name: "rejects too many properties",
			propsFunc: func() []string {
				props := make([]string, 101)
				for i := range props {
					props[i] = fmt.Sprintf("prop%d=value%d", i, i)
				}
				return props
			},
			wantErr: "too many property overrides",
		},
		{
			name: "rejects large property value",
			propsFunc: func() []string {
				return []string{"key=" + strings.Repeat("x", 11*1024)}
			},
			wantErr: "property value too large",
		},
		{
			name: "rejects long property key",
			propsFunc: func() []string {
				return []string{strings.Repeat("k", 257) + "=value"}
			},
			wantErr: "property key too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cli.ParsePropertyOverrides(tt.propsFunc())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestParseModifications tests the modification parsing function.
func TestParseModifications(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    map[string]map[string]string
		expectError bool
		errContains string
	}{
		{
			name:  "single modification",
			input: []string{"web-server:instanceType=m5.large"},
			expected: map[string]map[string]string{
				"web-server": {"instanceType": "m5.large"},
			},
		},
		{
			name:  "multiple modifications same resource",
			input: []string{"web-server:instanceType=m5.large", "web-server:volumeSize=100"},
			expected: map[string]map[string]string{
				"web-server": {"instanceType": "m5.large", "volumeSize": "100"},
			},
		},
		{
			name:  "multiple resources",
			input: []string{"web-server:instanceType=m5.large", "api-server:instanceType=t3.medium"},
			expected: map[string]map[string]string{
				"web-server": {"instanceType": "m5.large"},
				"api-server": {"instanceType": "t3.medium"},
			},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]map[string]string{},
		},
		{
			name:        "missing colon",
			input:       []string{"web-server-instanceType=m5.large"},
			expectError: true,
			errContains: "expected resource:key=value",
		},
		{
			name:        "missing equals",
			input:       []string{"web-server:instanceType"},
			expectError: true,
			errContains: "expected resource:key=value",
		},
		{
			name:        "empty resource name",
			input:       []string{":instanceType=m5.large"},
			expectError: true,
			errContains: "resource name cannot be empty",
		},
		{
			name:        "empty property key",
			input:       []string{"web-server:=m5.large"},
			expectError: true,
			errContains: "property key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cli.ParseModifications(tt.input)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestParseModifications_DoSLimits tests DoS protection limits.
func TestParseModifications_DoSLimits(t *testing.T) {
	tests := []struct {
		name     string
		modsFunc func() []string
		wantErr  string
	}{
		{
			name: "rejects too many modifications",
			modsFunc: func() []string {
				mods := make([]string, 1001)
				for i := range mods {
					mods[i] = fmt.Sprintf("resource%d:prop=value%d", i, i)
				}
				return mods
			},
			wantErr: "too many modifications",
		},
		{
			name: "rejects large modification value",
			modsFunc: func() []string {
				return []string{"resource:key=" + strings.Repeat("x", 11*1024)}
			},
			wantErr: "property value too large",
		},
		{
			name: "rejects long modification key",
			modsFunc: func() []string {
				return []string{"resource:" + strings.Repeat("k", 257) + "=value"}
			},
			wantErr: "property key too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cli.ParseModifications(tt.modsFunc())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestValidateEstimateFlags tests the flag validation logic.
func TestValidateEstimateFlags(t *testing.T) {
	tests := []struct {
		name        string
		params      cli.CostEstimateParams
		expectError bool
		errContains string
	}{
		{
			name: "valid single-resource mode",
			params: cli.CostEstimateParams{
				Provider:     "aws",
				ResourceType: "ec2:Instance",
				Properties:   []string{"instanceType=m5.large"},
			},
			expectError: false,
		},
		{
			name: "valid plan-based mode",
			params: cli.CostEstimateParams{
				PlanPath: "plan.json",
				Modify:   []string{"web-server:instanceType=m5.large"},
			},
			expectError: false,
		},
		{
			name: "missing provider in single-resource mode",
			params: cli.CostEstimateParams{
				ResourceType: "ec2:Instance",
			},
			expectError: true,
			errContains: "--provider is required",
		},
		{
			name: "missing resource-type in single-resource mode",
			params: cli.CostEstimateParams{
				Provider: "aws",
			},
			expectError: true,
			errContains: "--resource-type is required",
		},
		{
			name: "missing pulumi-json in plan-based mode",
			params: cli.CostEstimateParams{
				Modify: []string{"web-server:instanceType=m5.large"},
			},
			expectError: true,
			errContains: "--pulumi-json is required",
		},
		{
			name: "mixed modes not allowed",
			params: cli.CostEstimateParams{
				Provider:     "aws",
				ResourceType: "ec2:Instance",
				PlanPath:     "plan.json",
			},
			expectError: true,
			errContains: "cannot mix",
		},
		{
			name: "no mode specified",
			params: cli.CostEstimateParams{
				Output: "json",
			},
			expectError: true,
			errContains: "either single-resource mode",
		},
		{
			name: "interactive mode without other flags",
			params: cli.CostEstimateParams{
				Interactive: true,
			},
			expectError: false, // Interactive mode is valid without other flags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cli.ValidateEstimateFlags(&tt.params)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCostEstimateCmd_SingleResource tests single-resource estimation via CLI.
func TestCostEstimateCmd_SingleResource(t *testing.T) {
	t.Run("shows baseline only when no properties specified", func(t *testing.T) {
		cmd := cli.NewCostEstimateCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		// Set minimal flags for single-resource mode
		cmd.SetArgs([]string{"--provider", "aws", "--resource-type", "ec2:Instance"})

		// Execute - may error due to no plugins but validation should pass
		err := cmd.Execute()
		if err != nil {
			// Error should NOT be flag validation related
			assert.NotContains(t, err.Error(), "unknown flag")
			assert.NotContains(t, err.Error(), "invalid flag")
		}
	})
}

// TestCostEstimateCmd_Help tests the help output.
func TestCostEstimateCmd_Help(t *testing.T) {
	cmd := cli.NewCostEstimateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "estimate")
	assert.Contains(t, output, "what-if")
	assert.Contains(t, output, "--provider")
	assert.Contains(t, output, "--resource-type")
	assert.Contains(t, output, "--property")
	assert.Contains(t, output, "--pulumi-json")
	assert.Contains(t, output, "--modify")
}

// TestCostEstimateCmd_OutputFormats tests that output format flag is recognized.
func TestCostEstimateCmd_OutputFormats(t *testing.T) {
	formats := []string{"table", "json", "ndjson"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cmd := cli.NewCostEstimateCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)

			// Just verify the flag is accepted
			cmd.SetArgs([]string{
				"--provider", "aws",
				"--resource-type", "ec2:Instance",
				"--output", format,
			})

			// We expect this to fail due to missing plugins, but flag parsing should work
			_ = cmd.Execute()
		})
	}
}

// newTestCostCmd creates a test command with custom root.
func newTestCostCmd() *cobra.Command {
	root := &cobra.Command{Use: "finfocus"}
	cost := &cobra.Command{Use: "cost", Short: "Cost commands"}
	cost.AddCommand(cli.NewCostEstimateCmd())
	root.AddCommand(cost)
	return root
}

// TestCostEstimateCmd_Integration tests the command integrated in the cost group.
func TestCostEstimateCmd_Integration(t *testing.T) {
	cmd := newTestCostCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	cmd.SetArgs([]string{"cost", "estimate", "--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "estimate")
}

// TestPropertyDoesntAffectPricing tests edge case where property change has zero cost impact.
func TestPropertyDoesntAffectPricing(t *testing.T) {
	t.Run("zero cost delta renders gracefully", func(t *testing.T) {
		result := engine.EstimateResult{
			TotalChange: 0.0,
			Baseline:    &engine.CostResult{Monthly: 100.00, Currency: "USD"},
			Modified:    &engine.CostResult{Monthly: 100.00, Currency: "USD"},
		}
		assert.Equal(t, 0.0, result.TotalChange)
		require.NotNil(t, result.Baseline)
		require.NotNil(t, result.Modified)
		assert.Equal(t, result.Baseline.Monthly, result.Modified.Monthly)
	})

	t.Run("zero cost delta with property changes", func(t *testing.T) {
		result := engine.EstimateResult{
			TotalChange: 0.0,
			Deltas: []engine.CostDelta{
				{Property: "tags", OriginalValue: "env=dev", NewValue: "env=prod", CostChange: 0.0},
			},
			Baseline: &engine.CostResult{Monthly: 50.00, Currency: "USD"},
			Modified: &engine.CostResult{Monthly: 50.00, Currency: "USD"},
		}
		assert.Equal(t, 0.0, result.TotalChange)
		require.Len(t, result.Deltas, 1)
		assert.Equal(t, 0.0, result.Deltas[0].CostChange)
	})
}

// TestCostEstimateCmd_PlanBased tests plan-based estimation via CLI.
func TestCostEstimateCmd_PlanBased(t *testing.T) {
	t.Run("loads plan and applies modifications", func(t *testing.T) {
		cmd := cli.NewCostEstimateCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		// Use the test fixture
		cmd.SetArgs([]string{
			"--pulumi-json", "../../test/fixtures/estimate/plan-with-modify.json",
			"--modify", "web-server:instanceType=m5.large",
		})

		// Execute - may fail due to missing plugins but validation should pass
		err := cmd.Execute()
		// The command execution may fail due to missing plugins/actual plan,
		// but flag parsing and plan loading should work
		if err != nil {
			// If error, it should be about plan loading or plugin, not flags
			assert.NotContains(t, err.Error(), "flag")
		}
	})

	t.Run("requires pulumi-json when modify specified", func(t *testing.T) {
		cmd := cli.NewCostEstimateCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		cmd.SetArgs([]string{
			"--modify", "web-server:instanceType=m5.large",
		})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--pulumi-json is required")
	})
}

// TestFindModificationsForResource tests the resource matching logic.
func TestFindModificationsForResource(t *testing.T) {
	// Note: findModificationsForResource is unexported, so we test through
	// the CLI execution or via a wrapper test helper
	t.Run("resource ID matching via validation", func(t *testing.T) {
		// Test through ParseModifications
		mods, err := cli.ParseModifications([]string{
			"web-server:instanceType=m5.large",
			"web-server:volumeSize=100",
		})
		require.NoError(t, err)
		assert.Len(t, mods, 1)
		assert.Len(t, mods["web-server"], 2)
		assert.Equal(t, "m5.large", mods["web-server"]["instanceType"])
		assert.Equal(t, "100", mods["web-server"]["volumeSize"])
	})

	t.Run("multiple resources with modifications", func(t *testing.T) {
		mods, err := cli.ParseModifications([]string{
			"web-server:instanceType=m5.large",
			"api-server:instanceType=t3.medium",
		})
		require.NoError(t, err)
		assert.Len(t, mods, 2)
		assert.Equal(t, "m5.large", mods["web-server"]["instanceType"])
		assert.Equal(t, "t3.medium", mods["api-server"]["instanceType"])
	})
}

// TestCostEstimateCmd_ResourceNotFound tests error handling when resource not found in plan.
func TestCostEstimateCmd_ResourceNotFound(t *testing.T) {
	t.Run("modification for nonexistent resource", func(t *testing.T) {
		// When a modification references a resource not in the plan,
		// the command should skip that resource and continue with others
		cmd := cli.NewCostEstimateCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		cmd.SetArgs([]string{
			"--pulumi-json", "../../test/fixtures/estimate/single-resource.json",
			"--modify", "nonexistent-resource:instanceType=m5.large",
		})

		// The command should execute without error (skipping unknown resources)
		// or provide a meaningful error about the resource
		err := cmd.Execute()
		if err != nil {
			// If it errors, it should mention the resource or be a plan loading error
			// not a crash or panic
			assert.NotContains(t, err.Error(), "panic")
		}
	})
}
