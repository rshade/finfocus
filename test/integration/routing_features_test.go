//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/router"
)

// TestFeatureRouting_DifferentPluginsPerFeature tests US2:
// Configure different plugins for different features.
func TestFeatureRouting_DifferentPluginsPerFeature(t *testing.T) {
	ctx := context.Background()

	// Create mock plugin clients
	awsPublicClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-public",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}
	awsCEClient := &pluginhost.Client{
		Name: "aws-ce",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-ce",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}

	// Configure feature-specific routing
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Features: []string{"ProjectedCosts", "ActualCosts"},
				Priority: 10,
			},
			{
				Name:     "aws-ce",
				Features: []string{"Recommendations"},
				Priority: 10,
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsPublicClient, awsCEClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{
		Type:     "aws:ec2/instance:Instance",
		ID:       "i-12345",
		Provider: "aws",
	}

	t.Run("ProjectedCosts routes to aws-public", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")
		require.Len(t, matches, 1)
		assert.Equal(t, "aws-public", matches[0].Client.Name)
	})

	t.Run("ActualCosts routes to aws-public", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "ActualCosts")
		require.Len(t, matches, 1)
		assert.Equal(t, "aws-public", matches[0].Client.Name)
	})

	t.Run("Recommendations routes to aws-ce", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "Recommendations")
		require.Len(t, matches, 1)
		assert.Equal(t, "aws-ce", matches[0].Client.Name)
	})
}

// TestFeatureRouting_DryRun tests F-001:
// DryRun feature routing works correctly.
func TestFeatureRouting_DryRun(t *testing.T) {
	ctx := context.Background()

	dryRunClient := &pluginhost.Client{
		Name: "dryrun-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "dryrun-plugin",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}
	costClient := &pluginhost.Client{
		Name: "cost-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "cost-plugin",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "dryrun-plugin",
				Features: []string{"DryRun"},
			},
			{
				Name:     "cost-plugin",
				Features: []string{"ProjectedCosts", "ActualCosts"},
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{dryRunClient, costClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}

	t.Run("DryRun feature routes correctly", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "DryRun")
		require.Len(t, matches, 1)
		assert.Equal(t, "dryrun-plugin", matches[0].Client.Name)
	})

	t.Run("DryRun does not route to cost plugin", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "DryRun")
		for _, m := range matches {
			assert.NotEqual(t, "cost-plugin", m.Client.Name)
		}
	})
}

// TestFeatureRouting_Budgets tests F-001:
// Budgets feature routing works correctly.
func TestFeatureRouting_Budgets(t *testing.T) {
	ctx := context.Background()

	budgetsClient := &pluginhost.Client{
		Name: "budgets-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "budgets-plugin",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}
	costClient := &pluginhost.Client{
		Name: "cost-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "cost-plugin",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "budgets-plugin",
				Features: []string{"Budgets"},
			},
			{
				Name:     "cost-plugin",
				Features: []string{"ProjectedCosts", "ActualCosts"},
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{budgetsClient, costClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}

	t.Run("Budgets feature routes to budgets-plugin", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "Budgets")
		require.Len(t, matches, 1)
		assert.Equal(t, "budgets-plugin", matches[0].Client.Name)
	})

	t.Run("ProjectedCosts does not route to budgets-plugin", func(t *testing.T) {
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")
		for _, m := range matches {
			assert.NotEqual(t, "budgets-plugin", m.Client.Name)
		}
	})
}

// TestFeatureRouting_NoFeaturesConfigured tests that plugins without
// explicit feature configuration match all features.
func TestFeatureRouting_NoFeaturesConfigured(t *testing.T) {
	ctx := context.Background()

	genericClient := &pluginhost.Client{
		Name: "generic-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "generic-plugin",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}

	// No feature restriction in config
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "generic-plugin",
				Priority: 10,
				// No Features specified = matches all
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{genericClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}

	// Should match ALL features
	features := []string{"ProjectedCosts", "ActualCosts", "Recommendations", "DryRun", "Budgets", "Carbon"}

	for _, feature := range features {
		t.Run("matches_"+feature, func(t *testing.T) {
			matches := r.SelectPlugins(ctx, resource, feature)
			require.Len(t, matches, 1, "plugin with no features should match %s", feature)
			assert.Equal(t, "generic-plugin", matches[0].Client.Name)
		})
	}
}

// TestFeatureRouting_MultiplePluginsSameFeature tests that multiple plugins
// can serve the same feature (query all).
func TestFeatureRouting_MultiplePluginsSameFeature(t *testing.T) {
	ctx := context.Background()

	plugin1 := &pluginhost.Client{
		Name: "cost-plugin-1",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	plugin2 := &pluginhost.Client{
		Name: "cost-plugin-2",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "cost-plugin-1",
				Features: []string{"ProjectedCosts"},
				Priority: 10,
			},
			{
				Name:     "cost-plugin-2",
				Features: []string{"ProjectedCosts"},
				Priority: 10, // Same priority = query both
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{plugin1, plugin2}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}

	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")
	require.Len(t, matches, 2, "both plugins should match ProjectedCosts")

	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Client.Name
	}
	assert.Contains(t, names, "cost-plugin-1")
	assert.Contains(t, names, "cost-plugin-2")
}
