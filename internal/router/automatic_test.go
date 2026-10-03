package router_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	routerpkg "github.com/rshade/finfocus/internal/router"
)

// TestAutomaticRouting_ProviderMatching tests T016: automatic provider-based routing.
func TestAutomaticRouting_ProviderMatching(t *testing.T) {
	t.Parallel()

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

	router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient, gcpClient, azureClient}))
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
			t.Parallel()
			resource := engine.ResourceDescriptor{Type: tt.resourceType}
			matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

			if tt.wantPlugin == "" {
				assert.Empty(t, matches, "expected no matches for %s", tt.resourceType)
			} else {
				require.Len(t, matches, 1, "expected exactly 1 match for %s", tt.resourceType)
				assert.Equal(t, tt.wantPlugin, matches[0].Client.Name)
				assert.Equal(t, routerpkg.MatchReasonAutomatic, matches[0].MatchReason)
			}
		})
	}
}

func TestAutomaticRouting_ProviderAliases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name         string
		supported    string
		resourceType string
		wantMatch    bool
	}{
		{"aws plugin gets aws-native", "aws", "aws-native:ec2:Instance", true},
		{"azure plugin gets azure-native", "azure", "azure-native:compute:VirtualMachine", true},
		{"azure plugin gets classic azure", "azure", "azure:compute/virtualMachine:VirtualMachine", true},
		{"azure plugin gets azurerm", "azure", "azurerm_linux_virtual_machine", true},
		{"gcp plugin gets google-native", "gcp", "google-native:compute/v1:Instance", true},
		{"gcp plugin gets google terraform type", "gcp", "google_compute_instance", true},
		{"plugin listing azure-native keeps azure-native", "azure-native", "azure-native:compute:VirtualMachine", true},
		{
			"plugin listing azure-native gets classic azure",
			"azure-native",
			"azure:compute/virtualMachine:VirtualMachine",
			true,
		},
		{"aws plugin does not get azure-native", "aws", "azure-native:compute:VirtualMachine", false},
		{"azure plugin does not get aws-native", "azure", "aws-native:ec2:Instance", false},
		{"aws plugin skips native provider resource", "aws", "pulumi:providers:aws-native", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &pluginhost.Client{
				Name:     "plugin",
				Metadata: &proto.PluginMetadata{SupportedProviders: []string{tt.supported}},
			}
			router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{client}))
			require.NoError(t, err)

			matches := router.SelectPlugins(ctx, engine.ResourceDescriptor{Type: tt.resourceType}, "ProjectedCosts")
			if tt.wantMatch {
				require.Len(t, matches, 1)
				assert.Equal(t, routerpkg.MatchReasonAutomatic, matches[0].MatchReason)
				return
			}
			assert.Empty(t, matches)
		})
	}
}

func TestPatternRoutingKeepsRawPackage(t *testing.T) {
	t.Parallel()

	client := &pluginhost.Client{
		Name:     "native-only",
		Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{{
			Name:     "native-only",
			Patterns: []config.ResourcePattern{{Type: "glob", Pattern: "aws-native:*"}},
		}},
	}
	router, err := routerpkg.NewRouter(
		routerpkg.WithClients([]*pluginhost.Client{client}),
		routerpkg.WithConfig(cfg),
	)
	require.NoError(t, err)

	matches := router.SelectPlugins(
		context.Background(), engine.ResourceDescriptor{Type: "aws-native:ec2:Instance"}, "ProjectedCosts",
	)
	require.Len(t, matches, 1)
	assert.Equal(t, routerpkg.MatchReasonPattern, matches[0].MatchReason)
}

// TestAutomaticRouting_GlobalPlugins tests T017: global plugin matching.
func TestAutomaticRouting_GlobalPlugins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("empty SupportedProviders is global", func(t *testing.T) {
		t.Parallel()
		emptyClient := &pluginhost.Client{
			Name: "global-plugin",
			Metadata: &proto.PluginMetadata{
				SupportedProviders: []string{}, // Empty = global
			},
		}

		router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{emptyClient}))
		require.NoError(t, err)

		// Should match any provider
		for _, resourceType := range []string{"aws:ec2:Instance", "gcp:compute:VM", "azure:vm:VM", "unknown:resource:Type"} {
			resource := engine.ResourceDescriptor{Type: resourceType}
			matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

			require.Len(t, matches, 1, "global plugin should match %s", resourceType)
			assert.Equal(t, routerpkg.MatchReasonGlobal, matches[0].MatchReason)
		}
	})

	t.Run("wildcard ['*'] is global", func(t *testing.T) {
		t.Parallel()
		wildcardClient := &pluginhost.Client{
			Name: "recorder-plugin",
			Metadata: &proto.PluginMetadata{
				SupportedProviders: []string{"*"},
			},
		}

		router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{wildcardClient}))
		require.NoError(t, err)

		// Should match any provider
		for _, resourceType := range []string{"aws:ec2:Instance", "gcp:compute:VM", "custom:resource:Type"} {
			resource := engine.ResourceDescriptor{Type: resourceType}
			matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

			require.Len(t, matches, 1, "wildcard plugin should match %s", resourceType)
			assert.Equal(t, routerpkg.MatchReasonGlobal, matches[0].MatchReason)
		}
	})

	t.Run("nil metadata is global", func(t *testing.T) {
		t.Parallel()
		nilMetadataClient := &pluginhost.Client{
			Name:     "legacy-plugin",
			Metadata: nil,
		}

		router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{nilMetadataClient}))
		require.NoError(t, err)

		resource := engine.ResourceDescriptor{Type: "any:resource:Type"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, routerpkg.MatchReasonGlobal, matches[0].MatchReason)
	})

	t.Run("global plugin matches alongside specific provider", func(t *testing.T) {
		t.Parallel()
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

		router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient, globalClient}))
		require.NoError(t, err)

		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Both should match: aws-public (automatic) and recorder (global)
		require.Len(t, matches, 2)

		// Check we have both match reasons
		reasons := make(map[routerpkg.MatchReason]bool)
		for _, m := range matches {
			reasons[m.MatchReason] = true
		}
		assert.True(t, reasons[routerpkg.MatchReasonAutomatic], "should have automatic match")
		assert.True(t, reasons[routerpkg.MatchReasonGlobal], "should have global match")
	})
}

// TestAutomaticRouting_SourceAttribution tests T018a: source field in results.
//
//nolint:paralleltest // subtests share the parent-scoped fixture awsClient
func TestAutomaticRouting_SourceAttribution(t *testing.T) {
	ctx := context.Background()

	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	t.Run("automatic routing has 'automatic' source", func(t *testing.T) {
		router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient}))
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

		router, err := routerpkg.NewRouter(
			routerpkg.WithClients([]*pluginhost.Client{awsClient}),
			routerpkg.WithConfig(cfg),
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
	t.Parallel()

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

	router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient, gcpClient}))
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
	t.Parallel()

	ctx := context.Background()

	// Plugin declares "aws" (lowercase)
	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient}))
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
	t.Parallel()

	ctx := context.Background()

	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient}))
	require.NoError(t, err)

	// GCP resource with only AWS plugin available
	resource := engine.ResourceDescriptor{Type: "gcp:compute:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	assert.Empty(t, matches, "GCP resource should not match AWS-only plugin")
}

// TestAutomaticRouting_InternalPulumiTypes tests that internal Pulumi types
// are not matched by automatic or global plugins.
func TestAutomaticRouting_InternalPulumiTypes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

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

	router, err := routerpkg.NewRouter(routerpkg.WithClients([]*pluginhost.Client{awsClient, globalClient}))
	require.NoError(t, err)

	internalTypes := []string{
		"pulumi:pulumi:Stack",
		"pulumi:providers:aws",
		"pulumi:providers:azure",
		"pulumi:providers:gcp",
	}

	for _, resourceType := range internalTypes {
		resource := engine.ResourceDescriptor{Type: resourceType}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		assert.Empty(t, matches,
			"internal Pulumi type %s should not match any plugin (including global)", resourceType)
	}
}
