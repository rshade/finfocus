package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestValidateRoutingConfig_NilConfig(t *testing.T) {
	result := ValidateRoutingConfig(nil, nil)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestValidateRoutingConfig_EmptyConfig(t *testing.T) {
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{},
	}

	result := ValidateRoutingConfig(cfg, nil)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestValidateRoutingConfig_ValidConfig(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
		{Name: "gcp-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"gcp"}}},
	}

	cfg := &config.RoutingConfig{
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
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestValidateRoutingConfig_MissingPluginName(t *testing.T) {
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: ""}, // Missing name
		},
	}

	result := ValidateRoutingConfig(cfg, nil)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "name")
	assert.Contains(t, result.Errors[0].Error(), "required")
}

func TestValidateRoutingConfig_PluginNotFound(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "nonexistent-plugin"},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "nonexistent-plugin")
	assert.Contains(t, result.Errors[0].Error(), "not found")
}

func TestValidateRoutingConfig_NegativePriority(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "aws-public", Priority: -5},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "priority")
	assert.Contains(t, result.Errors[0].Error(), "-5")
}

func TestValidateRoutingConfig_InvalidFeature(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Features: []string{"InvalidFeature", "ProjectedCosts"},
			},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.True(t, result.Valid) // Invalid features are warnings, not errors
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0].Message, "InvalidFeature")
	assert.Contains(t, result.Warnings[0].Message, "unknown feature")
}

func TestValidateRoutingConfig_InvalidPatternType(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "aws-public",
				Patterns: []config.ResourcePattern{
					{Type: "wildcard", Pattern: "aws:*"}, // Invalid type
				},
			},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "wildcard")
	assert.Contains(t, result.Errors[0].Error(), "invalid pattern type")
}

func TestValidateRoutingConfig_EmptyPattern(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "aws-public",
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: ""}, // Empty pattern
				},
			},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "pattern")
	assert.Contains(t, result.Errors[0].Error(), "empty")
}

func TestValidateRoutingConfig_InvalidRegex(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "aws-public",
				Patterns: []config.ResourcePattern{
					{Type: "regex", Pattern: "aws:(ec2|rds"}, // Invalid regex
				},
			},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "invalid regex")
}

func TestValidateRoutingConfig_DuplicatePlugin(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "aws-public", Priority: 10},
			{Name: "aws-public", Priority: 20}, // Duplicate
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.True(t, result.Valid) // Duplicates are warnings, not errors
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0].Message, "duplicate")
	assert.Contains(t, result.Warnings[0].Message, "index 0")
}

func TestValidateRoutingConfig_MultipleErrors(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: ""},                         // Error: missing name
			{Name: "nonexistent"},              // Error: plugin not found
			{Name: "aws-public", Priority: -1}, // Error: negative priority
			{
				Name: "aws-public",
				Patterns: []config.ResourcePattern{
					{Type: "invalid", Pattern: "test"}, // Error: invalid type
				},
			},
		},
	}

	result := ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	assert.True(t, len(result.Errors) >= 4, "expected at least 4 errors, got %d", len(result.Errors))
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
