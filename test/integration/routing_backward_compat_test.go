//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/router"
)

// TestBackwardCompatibility_NoRoutingKey verifies FR-023:
// Configuration files without the 'routing:' key should use automatic
// provider-based routing (default behavior).
func TestBackwardCompatibility_NoRoutingKey(t *testing.T) {
	// Create a temp config file without routing key
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
output:
  default_format: table
  precision: 2

logging:
  level: info
  format: text

plugins:
  aws-public:
    region: us-east-1
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Load the config
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Verify routing is nil (not configured)
	assert.Nil(t, cfg.Routing, "Routing should be nil when not configured")

	// Create mock plugin clients
	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	gcpClient := &pluginhost.Client{
		Name: "gcp-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"gcp"},
		},
	}

	// Create router with nil config (automatic routing only)
	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsClient, gcpClient}),
		router.WithConfig(nil), // No routing config = automatic only
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Test AWS resource routes to AWS plugin
	awsResource := engine.ResourceDescriptor{
		Type: "aws:ec2/instance:Instance",
		ID:   "i-12345",
	}
	awsMatches := r.SelectPlugins(ctx, awsResource, "ProjectedCosts")
	require.Len(t, awsMatches, 1, "AWS resource should match exactly 1 plugin")
	assert.Equal(t, "aws-public", awsMatches[0].Client.Name)
	assert.Equal(t, router.MatchReasonAutomatic, awsMatches[0].MatchReason)

	// Test GCP resource routes to GCP plugin
	gcpResource := engine.ResourceDescriptor{
		Type: "gcp:compute:Instance",
		ID:   "gcp-instance-1",
	}
	gcpMatches := r.SelectPlugins(ctx, gcpResource, "ProjectedCosts")
	require.Len(t, gcpMatches, 1, "GCP resource should match exactly 1 plugin")
	assert.Equal(t, "gcp-public", gcpMatches[0].Client.Name)
	assert.Equal(t, router.MatchReasonAutomatic, gcpMatches[0].MatchReason)
}

// TestBackwardCompatibility_EmptyRoutingPlugins verifies that empty plugins
// list in routing config also defaults to automatic routing.
func TestBackwardCompatibility_EmptyRoutingPlugins(t *testing.T) {
	// Create a config with empty routing plugins
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{},
	}

	// Create mock clients
	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	// Create router with empty config
	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// AWS resource should still route correctly via automatic routing
	awsResource := engine.ResourceDescriptor{
		Type: "aws:ec2/instance:Instance",
		ID:   "i-12345",
	}
	matches := r.SelectPlugins(ctx, awsResource, "ProjectedCosts")
	require.Len(t, matches, 1)
	assert.Equal(t, "aws-public", matches[0].Client.Name)
}

// TestBackwardCompatibility_LegacyConfigLoads verifies that legacy config
// files (pre-routing) load without error.
func TestBackwardCompatibility_LegacyConfigLoads(t *testing.T) {
	// Use the fixture file that has no routing key
	fixturePath := "../../test/fixtures/routing/no_routing_key.yaml"

	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Verify config loaded correctly
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Nil(t, cfg.Routing, "Routing should be nil for legacy config")
}

// TestBackwardCompatibility_MixedConfigLoads verifies that config files
// with routing section coexist with existing config.
func TestBackwardCompatibility_MixedConfigLoads(t *testing.T) {
	// Use the fixture file that has routing section
	fixturePath := "../../test/fixtures/routing/valid_config.yaml"

	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Verify routing config loaded
	require.NotNil(t, cfg.Routing)
	assert.GreaterOrEqual(t, len(cfg.Routing.Plugins), 1, "Should have at least one plugin configured")
}
