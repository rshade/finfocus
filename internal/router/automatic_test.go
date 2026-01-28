package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// TestAutomaticRouting_ProviderMatching tests T016: automatic provider-based routing.
func TestAutomaticRouting_ProviderMatching(t *testing.T) {
	ctx := context.Background()

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
	azureClient := &pluginhost.Client{
		Name: "azure-costs",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"azure"},
		},
	}

	router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient, gcpClient, azureClient}))
	require.NoError(t, err)

	tests := []struct {
		name         string
		resourceType string
		wantPlugin   string
	}{
		{
			name:         "AWS EC2 resource routes to aws-public",
			resourceType: "aws:ec2/instance:Instance",
			wantPlugin:   "aws-public",
		},
		{
			name:         "AWS RDS resource routes to aws-public",
			resourceType: "aws:rds/instance:Instance",
			wantPlugin:   "aws-public",
		},
		{
			name:         "GCP Compute resource routes to gcp-public",
			resourceType: "gcp:compute:Instance",
			wantPlugin:   "gcp-public",
		},
		{
			name:         "Azure VM resource routes to azure-costs",
			resourceType: "azure:compute/vm:VM",
			wantPlugin:   "azure-costs",
		},
		{
			name:         "Kubernetes resource matches no specific provider",
			resourceType: "kubernetes:core/v1:Pod",
			wantPlugin:   "", // No plugin matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := engine.ResourceDescriptor{Type: tt.resourceType}
			matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

			if tt.wantPlugin == "" {
				assert.Empty(t, matches, "expected no matches for %s", tt.resourceType)
			} else {
				require.Len(t, matches, 1, "expected exactly 1 match for %s", tt.resourceType)
				assert.Equal(t, tt.wantPlugin, matches[0].Client.Name)
				assert.Equal(t, MatchReasonAutomatic, matches[0].MatchReason)
			}
		})
	}
}

// TestAutomaticRouting_GlobalPlugins tests T017: global plugin matching.
func TestAutomaticRouting_GlobalPlugins(t *testing.T) {
	ctx := context.Background()

	t.Run("empty SupportedProviders is global", func(t *testing.T) {
		emptyClient := &pluginhost.Client{
			Name: "global-plugin",
			Metadata: &proto.PluginMetadata{
				SupportedProviders: []string{}, // Empty = global
			},
		}

		router, err := NewRouter(WithClients([]*pluginhost.Client{emptyClient}))
		require.NoError(t, err)

		// Should match any provider
		for _, resourceType := range []string{"aws:ec2:Instance", "gcp:compute:VM", "azure:vm:VM", "unknown:resource:Type"} {
			resource := engine.ResourceDescriptor{Type: resourceType}
			matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

			require.Len(t, matches, 1, "global plugin should match %s", resourceType)
			assert.Equal(t, MatchReasonGlobal, matches[0].MatchReason)
		}
	})

	t.Run("wildcard ['*'] is global", func(t *testing.T) {
		wildcardClient := &pluginhost.Client{
			Name: "recorder-plugin",
			Metadata: &proto.PluginMetadata{
				SupportedProviders: []string{"*"},
			},
		}

		router, err := NewRouter(WithClients([]*pluginhost.Client{wildcardClient}))
		require.NoError(t, err)

		// Should match any provider
		for _, resourceType := range []string{"aws:ec2:Instance", "gcp:compute:VM", "custom:resource:Type"} {
			resource := engine.ResourceDescriptor{Type: resourceType}
			matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

			require.Len(t, matches, 1, "wildcard plugin should match %s", resourceType)
			assert.Equal(t, MatchReasonGlobal, matches[0].MatchReason)
		}
	})

	t.Run("nil metadata is global", func(t *testing.T) {
		nilMetadataClient := &pluginhost.Client{
			Name:     "legacy-plugin",
			Metadata: nil,
		}

		router, err := NewRouter(WithClients([]*pluginhost.Client{nilMetadataClient}))
		require.NoError(t, err)

		resource := engine.ResourceDescriptor{Type: "any:resource:Type"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, MatchReasonGlobal, matches[0].MatchReason)
	})

	t.Run("global plugin matches alongside specific provider", func(t *testing.T) {
		awsClient := &pluginhost.Client{
			Name: "aws-public",
			Metadata: &proto.PluginMetadata{
				SupportedProviders: []string{"aws"},
			},
		}
		globalClient := &pluginhost.Client{
			Name: "recorder",
			Metadata: &proto.PluginMetadata{
				SupportedProviders: []string{"*"},
			},
		}

		router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient, globalClient}))
		require.NoError(t, err)

		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Both should match: aws-public (automatic) and recorder (global)
		require.Len(t, matches, 2)

		// Check we have both match reasons
		reasons := make(map[MatchReason]bool)
		for _, m := range matches {
			reasons[m.MatchReason] = true
		}
		assert.True(t, reasons[MatchReasonAutomatic], "should have automatic match")
		assert.True(t, reasons[MatchReasonGlobal], "should have global match")
	})
}

// TestAutomaticRouting_SourceAttribution tests T018a: source field in results.
func TestAutomaticRouting_SourceAttribution(t *testing.T) {
	ctx := context.Background()

	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	t.Run("automatic routing has 'automatic' source", func(t *testing.T) {
		router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient}))
		require.NoError(t, err)

		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, "automatic", matches[0].Source)
	})

	t.Run("pattern routing has 'config' source", func(t *testing.T) {
		cfg := &config.RoutingConfig{
			Plugins: []config.PluginRouting{
				{
					Name: "aws-public",
					Patterns: []config.ResourcePattern{
						{Type: "glob", Pattern: "aws:ec2:*"},
					},
				},
			},
		}

		router, err := NewRouter(
			WithClients([]*pluginhost.Client{awsClient}),
			WithConfig(cfg),
		)
		require.NoError(t, err)

		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, "config", matches[0].Source)
	})
}

// TestAutomaticRouting_MultiCloud tests mixed-cloud plan routing.
func TestAutomaticRouting_MultiCloud(t *testing.T) {
	ctx := context.Background()

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

	router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient, gcpClient}))
	require.NoError(t, err)

	// Simulate a multi-cloud plan
	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "aws-instance-1"},
		{Type: "aws:rds/instance:Instance", ID: "aws-rds-1"},
		{Type: "gcp:compute:Instance", ID: "gcp-instance-1"},
		{Type: "gcp:storage:Bucket", ID: "gcp-bucket-1"},
	}

	awsCount := 0
	gcpCount := 0

	for _, resource := range resources {
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")
		require.Len(t, matches, 1, "resource %s should have exactly 1 match", resource.Type)

		switch matches[0].Client.Name {
		case "aws-public":
			awsCount++
		case "gcp-public":
			gcpCount++
		}
	}

	assert.Equal(t, 2, awsCount, "2 AWS resources should route to aws-public")
	assert.Equal(t, 2, gcpCount, "2 GCP resources should route to gcp-public")
}

// TestAutomaticRouting_CaseInsensitive tests provider matching is case-insensitive.
func TestAutomaticRouting_CaseInsensitive(t *testing.T) {
	ctx := context.Background()

	// Plugin declares "aws" (lowercase)
	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient}))
	require.NoError(t, err)

	// Resource types may have different cases in practice
	testCases := []string{
		"aws:ec2:Instance",
		"AWS:ec2:Instance", // Unlikely but should handle
		"Aws:ec2:Instance", // Mixed case
	}

	for _, resourceType := range testCases {
		resource := engine.ResourceDescriptor{Type: resourceType}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1, "resource %s should match aws-public", resourceType)
		assert.Equal(t, "aws-public", matches[0].Client.Name)
	}
}

// TestAutomaticRouting_NoMatchingPlugins tests behavior when no plugins match.
func TestAutomaticRouting_NoMatchingPlugins(t *testing.T) {
	ctx := context.Background()

	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient}))
	require.NoError(t, err)

	// GCP resource with only AWS plugin available
	resource := engine.ResourceDescriptor{Type: "gcp:compute:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	assert.Empty(t, matches, "GCP resource should not match AWS-only plugin")
}
