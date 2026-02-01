package router

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// validationTestCase defines a single test case for ValidateRoutingConfig.
type validationTestCase struct {
	name          string
	cfg           *config.RoutingConfig
	clients       []*pluginhost.Client
	wantValid     bool
	wantErrCount  int
	wantWarnCount int
	errContains   []string
	warnContains  []string
}

// containsAnyErrorMessage checks if any error message contains the given substring.
func containsAnyErrorMessage(errors []ValidationError, substr string) bool {
	for _, err := range errors {
		if strings.Contains(err.Error(), substr) {
			return true
		}
	}
	return false
}

// containsAnyWarningMessage checks if any warning message contains the given substring.
func containsAnyWarningMessage(warnings []ValidationWarning, substr string) bool {
	for _, warn := range warnings {
		if strings.Contains(warn.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidateRoutingConfig(t *testing.T) {
	tests := []validationTestCase{
		{
			name:          "NilConfig",
			cfg:           nil,
			clients:       nil,
			wantValid:     true,
			wantErrCount:  0,
			wantWarnCount: 0,
		},
		{
			name: "EmptyConfig",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{},
			},
			clients:       nil,
			wantValid:     true,
			wantErrCount:  0,
			wantWarnCount: 0,
		},
		{
			name: "ValidConfig",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{
						Name:     "aws-public",
						Features: []string{"ProjectedCosts", "ActualCosts"},
						Priority: 10,
					},
					{
						Name:     "gcp-public",
						Features: []string{"ProjectedCosts"},
						Patterns: []config.ResourcePattern{
							{Type: "glob", Pattern: "gcp:*"},
						},
						Priority: 10,
					},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
				{Name: "gcp-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"gcp"}}},
			},
			wantValid:     true,
			wantErrCount:  0,
			wantWarnCount: 0,
		},
		{
			name: "MissingPluginName",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{Name: ""},
				},
			},
			clients:       nil,
			wantValid:     false,
			wantErrCount:  1,
			wantWarnCount: 0,
			errContains:   []string{"name", "required"},
		},
		{
			name: "PluginNotFound",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{Name: "nonexistent-plugin"},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     false,
			wantErrCount:  1,
			wantWarnCount: 0,
			errContains:   []string{"nonexistent-plugin", "not found"},
		},
		{
			name: "NegativePriority",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{Name: "aws-public", Priority: -5},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     false,
			wantErrCount:  1,
			wantWarnCount: 0,
			errContains:   []string{"priority", "-5"},
		},
		{
			name: "InvalidFeature",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{
						Name:     "aws-public",
						Features: []string{"InvalidFeature", "ProjectedCosts"},
					},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     true, // Invalid features are warnings, not errors
			wantErrCount:  0,
			wantWarnCount: 1,
			warnContains:  []string{"InvalidFeature", "unknown feature"},
		},
		{
			name: "InvalidPatternType",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{
						Name: "aws-public",
						Patterns: []config.ResourcePattern{
							{Type: "wildcard", Pattern: "aws:*"},
						},
					},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     false,
			wantErrCount:  1,
			wantWarnCount: 0,
			errContains:   []string{"wildcard", "invalid pattern type"},
		},
		{
			name: "EmptyPattern",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{
						Name: "aws-public",
						Patterns: []config.ResourcePattern{
							{Type: "glob", Pattern: ""},
						},
					},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     false,
			wantErrCount:  1,
			wantWarnCount: 0,
			errContains:   []string{"pattern", "empty"},
		},
		{
			name: "InvalidRegex",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{
						Name: "aws-public",
						Patterns: []config.ResourcePattern{
							{Type: "regex", Pattern: "aws:(ec2|rds"},
						},
					},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     false,
			wantErrCount:  1,
			wantWarnCount: 0,
			errContains:   []string{"invalid regex"},
		},
		{
			name: "DuplicatePlugin",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{Name: "aws-public", Priority: 10},
					{Name: "aws-public", Priority: 20},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     true, // Duplicates are warnings, not errors
			wantErrCount:  0,
			wantWarnCount: 1,
			warnContains:  []string{"duplicate", "index 0"},
		},
		{
			name: "MultipleErrors",
			cfg: &config.RoutingConfig{
				Plugins: []config.PluginRouting{
					{Name: ""},
					{Name: "nonexistent"},
					{Name: "aws-public", Priority: -1},
					{
						Name: "aws-public",
						Patterns: []config.ResourcePattern{
							{Type: "invalid", Pattern: "test"},
						},
					},
				},
			},
			clients: []*pluginhost.Client{
				{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
			},
			wantValid:     false,
			wantErrCount:  -4, // At least 4 errors
			wantWarnCount: 1,  // Duplicate plugin warning
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateRoutingConfig(tc.cfg, tc.clients)

			assert.Equal(t, tc.wantValid, result.Valid, "valid mismatch")

			if tc.wantErrCount >= 0 {
				require.Len(t, result.Errors, tc.wantErrCount, "error count mismatch")
			} else {
				// For "at least N errors" cases
				minCount := -tc.wantErrCount
				assert.True(
					t,
					len(result.Errors) >= minCount,
					"expected at least %d errors, got %d",
					minCount,
					len(result.Errors),
				)
			}

			require.Len(t, result.Warnings, tc.wantWarnCount, "warning count mismatch")

			for _, substr := range tc.errContains {
				assert.True(
					t,
					containsAnyErrorMessage(result.Errors, substr),
					"expected error containing %q",
					substr,
				)
			}

			for _, substr := range tc.warnContains {
				assert.True(
					t,
					containsAnyWarningMessage(result.Warnings, substr),
					"expected warning containing %q",
					substr,
				)
			}
		})
	}
}

func TestValidationResult_Methods(t *testing.T) {
	t.Run("HasErrors", func(t *testing.T) {
		result := ValidationResult{
			Valid:  false,
			Errors: []ValidationError{{Message: "test error"}},
		}
		assert.True(t, result.HasErrors())

		emptyResult := ValidationResult{Valid: true}
		assert.False(t, emptyResult.HasErrors())
	})

	t.Run("HasWarnings", func(t *testing.T) {
		result := ValidationResult{
			Valid:    true,
			Warnings: []ValidationWarning{{Message: "test warning"}},
		}
		assert.True(t, result.HasWarnings())

		emptyResult := ValidationResult{Valid: true}
		assert.False(t, emptyResult.HasWarnings())
	})

	t.Run("ErrorMessages", func(t *testing.T) {
		result := ValidationResult{
			Errors: []ValidationError{
				{Plugin: "p1", Field: "f1", Message: "m1"},
				{Field: "f2", Message: "m2"},
			},
		}
		messages := result.ErrorMessages()
		assert.Len(t, messages, 2)
		assert.Equal(t, "p1.f1: m1", messages[0])
		assert.Equal(t, "f2: m2", messages[1])
	})

	t.Run("WarningMessages", func(t *testing.T) {
		result := ValidationResult{
			Warnings: []ValidationWarning{
				{Plugin: "p1", Field: "f1", Message: "m1"},
				{Field: "f2", Message: "m2"},
			},
		}
		messages := result.WarningMessages()
		assert.Len(t, messages, 2)
		assert.Equal(t, "p1.f1: m1", messages[0])
		assert.Equal(t, "f2: m2", messages[1])
	})
}

func TestValidationError_Error(t *testing.T) {
	withPlugin := ValidationError{Plugin: "test-plugin", Field: "name", Message: "is required"}
	assert.Equal(t, "test-plugin.name: is required", withPlugin.Error())

	withoutPlugin := ValidationError{Field: "config", Message: "is invalid"}
	assert.Equal(t, "config: is invalid", withoutPlugin.Error())
}
