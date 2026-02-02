//go:build integration

// Package integration provides black-box integration tests that validate the
// interaction between multiple internal packages (router, engine, pluginhost).
// These tests are intentionally kept in a separate package to ensure they test
// the public API surface and cross-package integration behavior.
package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/router"
)

// TestAutomaticRouting_IntegrationWithEngine tests the full integration
// of automatic routing with the engine for a multi-cloud scenario.
func TestAutomaticRouting_IntegrationWithEngine(t *testing.T) {
	ctx := context.Background()

	// Create mock plugin clients simulating installed plugins
	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-public",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}
	gcpClient := &pluginhost.Client{
		Name: "gcp-public",
		Metadata: &proto.PluginMetadata{
			Name:               "gcp-public",
			Version:            "1.0.0",
			SupportedProviders: []string{"gcp"},
		},
	}

	// Create router with automatic routing only (no config)
	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsClient, gcpClient}),
	)
	require.NoError(t, err)

	// Test a mixed-cloud plan
	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-12345", Provider: "aws"},
		{Type: "aws:rds/instance:Instance", ID: "db-67890", Provider: "aws"},
		{Type: "gcp:compute:Instance", ID: "gcp-instance-1", Provider: "gcp"},
		{Type: "gcp:storage:Bucket", ID: "gcp-bucket-1", Provider: "gcp"},
	}

	// Verify each resource routes to the correct plugin
	routingResults := make(map[string]string) // resource ID -> plugin name

	for _, resource := range resources {
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")
		require.Len(t, matches, 1, "resource %s should have exactly 1 match", resource.ID)

		routingResults[resource.ID] = matches[0].Client.Name
	}

	// Verify AWS resources route to aws-public
	assert.Equal(t, "aws-public", routingResults["i-12345"], "EC2 instance should route to aws-public")
	assert.Equal(t, "aws-public", routingResults["db-67890"], "RDS instance should route to aws-public")

	// Verify GCP resources route to gcp-public
	assert.Equal(t, "gcp-public", routingResults["gcp-instance-1"], "GCP instance should route to gcp-public")
	assert.Equal(t, "gcp-public", routingResults["gcp-bucket-1"], "GCP bucket should route to gcp-public")
}

// TestAutomaticRouting_WildcardProvider tests F-009:
// Plugins with ["*"] provider should match all resources.
func TestAutomaticRouting_WildcardProvider(t *testing.T) {
	ctx := context.Background()

	// Create a recorder-style plugin with wildcard provider
	recorderClient := &pluginhost.Client{
		Name: "recorder",
		Metadata: &proto.PluginMetadata{
			Name:               "recorder",
			Version:            "0.1.0",
			SupportedProviders: []string{"*"},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{recorderClient}),
	)
	require.NoError(t, err)

	// Wildcard plugin should match any provider
	testResources := []string{
		"aws:ec2:Instance",
		"gcp:compute:VM",
		"azure:vm:VirtualMachine",
		"kubernetes:core/v1:Pod",
		"custom:resource:Type",
	}

	for _, resourceType := range testResources {
		resource := engine.ResourceDescriptor{Type: resourceType}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1, "wildcard plugin should match %s", resourceType)
		assert.Equal(t, "recorder", matches[0].Client.Name)
		assert.Equal(t, router.MatchReasonGlobal, matches[0].MatchReason)
	}
}

// TestAutomaticRouting_MixedProviderAndWildcard tests that both
// specific and wildcard providers work together correctly.
func TestAutomaticRouting_MixedProviderAndWildcard(t *testing.T) {
	ctx := context.Background()

	// Specific provider plugin
	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-public",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}

	// Wildcard plugin (like recorder for debugging)
	recorderClient := &pluginhost.Client{
		Name: "recorder",
		Metadata: &proto.PluginMetadata{
			Name:               "recorder",
			Version:            "0.1.0",
			SupportedProviders: []string{"*"},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsClient, recorderClient}),
	)
	require.NoError(t, err)

	// AWS resource should match BOTH aws-public and recorder
	awsResource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	awsMatches := r.SelectPlugins(ctx, awsResource, "ProjectedCosts")

	require.Len(t, awsMatches, 2, "AWS resource should match both plugins")

	pluginNames := make([]string, len(awsMatches))
	for i, m := range awsMatches {
		pluginNames[i] = m.Client.Name
	}
	assert.Contains(t, pluginNames, "aws-public")
	assert.Contains(t, pluginNames, "recorder")

	// Non-AWS resource should only match recorder
	gcpResource := engine.ResourceDescriptor{Type: "gcp:compute:Instance"}
	gcpMatches := r.SelectPlugins(ctx, gcpResource, "ProjectedCosts")

	require.Len(t, gcpMatches, 1, "GCP resource should only match recorder")
	assert.Equal(t, "recorder", gcpMatches[0].Client.Name)
}

// TestAutomaticRouting_EmptyProviders tests that plugins with
// empty SupportedProviders are treated as global (like wildcard).
func TestAutomaticRouting_EmptyProviders(t *testing.T) {
	ctx := context.Background()

	// Plugin with empty providers (should be global)
	legacyClient := &pluginhost.Client{
		Name: "legacy-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "legacy-plugin",
			Version:            "0.1.0",
			SupportedProviders: []string{}, // Empty = global
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{legacyClient}),
	)
	require.NoError(t, err)

	// Should match any provider
	resource := engine.ResourceDescriptor{Type: "any:resource:Type"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 1)
	assert.Equal(t, router.MatchReasonGlobal, matches[0].MatchReason)
}

// TestAutomaticRouting_NoPluginsAvailable tests behavior when
// no plugins are available at all.
func TestAutomaticRouting_NoPluginsAvailable(t *testing.T) {
	ctx := context.Background()

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{}), // No plugins
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	assert.Empty(t, matches, "no plugins should mean no matches")
}

// TestAutomaticRouting_SourceFieldAttribution tests F-002:
// Results should include source field for tracking routing decisions.
func TestAutomaticRouting_SourceFieldAttribution(t *testing.T) {
	ctx := context.Background()

	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsClient}),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 1)
	assert.Equal(t, "automatic", matches[0].Source, "automatic routing should have 'automatic' source")
	assert.Equal(t, router.MatchReasonAutomatic, matches[0].MatchReason)
}
