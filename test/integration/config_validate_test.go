//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/router"
)

// TestConfigValidate_ValidConfig tests that valid routing configs pass validation.
func TestConfigValidate_ValidConfig(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
		{Name: "gcp-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"gcp"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Priority: 10,
				Features: []string{"ProjectedCosts", "ActualCosts"},
			},
			{
				Name:     "gcp-public",
				Priority: 20,
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "gcp:*"},
					{Type: "regex", Pattern: "gcp:compute:.*"},
				},
			},
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

// TestConfigValidate_PluginNotFound tests error on non-existent plugin.
func TestConfigValidate_PluginNotFound(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "nonexistent-plugin", Priority: 10},
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "not found")
	assert.Equal(t, "nonexistent-plugin", result.Errors[0].Plugin)
}

// TestConfigValidate_InvalidRegex tests error on invalid regex pattern.
func TestConfigValidate_InvalidRegex(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "aws-public",
				Patterns: []config.ResourcePattern{
					{Type: "regex", Pattern: "aws:(ec2|rds"}, // Unclosed parenthesis
				},
			},
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "invalid regex")
}

// TestConfigValidate_InvalidFeature tests warning on unknown feature.
func TestConfigValidate_InvalidFeature(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Features: []string{"UnknownFeature", "ProjectedCosts"},
			},
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	// Invalid features are warnings, not errors (config is still valid)
	assert.True(t, result.Valid)
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0].Message, "UnknownFeature")
	assert.Contains(t, result.Warnings[0].Message, "unknown feature")
}

// TestConfigValidate_DuplicatePlugin tests warning on duplicate plugin config.
func TestConfigValidate_DuplicatePlugin(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "aws-public", Priority: 10},
			{Name: "aws-public", Priority: 20}, // Duplicate
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	// Duplicates are warnings, not errors
	assert.True(t, result.Valid)
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0].Message, "duplicate")
}

// TestConfigValidate_NegativePriority tests error on negative priority.
func TestConfigValidate_NegativePriority(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "aws-public", Priority: -5},
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "-5")
}

// TestConfigValidate_EmptyPattern tests error on empty pattern.
func TestConfigValidate_EmptyPattern(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "aws-public",
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: ""},
				},
			},
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "empty")
}

// TestConfigValidate_InvalidPatternType tests error on invalid pattern type.
func TestConfigValidate_InvalidPatternType(t *testing.T) {
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

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "wildcard")
	assert.Contains(t, result.Errors[0].Message, "invalid pattern type")
}

// TestConfigValidate_MissingPluginName tests error on missing plugin name.
func TestConfigValidate_MissingPluginName(t *testing.T) {
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "", Priority: 10},
		},
	}

	result := router.ValidateRoutingConfig(cfg, nil)

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "required")
}

// TestConfigValidate_MultipleIssues tests multiple validation issues.
func TestConfigValidate_MultipleIssues(t *testing.T) {
	clients := []*pluginhost.Client{
		{Name: "aws-public", Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}}},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: ""},                         // Error: missing name
			{Name: "nonexistent"},              // Error: plugin not found
			{Name: "aws-public", Priority: -1}, // Error: negative priority
		},
	}

	result := router.ValidateRoutingConfig(cfg, clients)

	assert.False(t, result.Valid)
	assert.GreaterOrEqual(t, len(result.Errors), 3)
}

// TestConfigValidate_NilConfig tests that nil config is valid (automatic routing).
func TestConfigValidate_NilConfig(t *testing.T) {
	result := router.ValidateRoutingConfig(nil, nil)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

// TestConfigValidate_EmptyPlugins tests that empty plugins list is valid.
func TestConfigValidate_EmptyPlugins(t *testing.T) {
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{},
	}

	result := router.ValidateRoutingConfig(cfg, nil)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}
