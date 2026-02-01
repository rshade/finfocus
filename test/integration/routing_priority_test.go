//go:build integration

// Package integration provides black-box integration tests that validate the
// interaction between multiple internal packages (router, engine, pluginhost, config).
// These tests are intentionally kept in a separate package to ensure they test
// the public API surface and cross-package integration behavior.
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

// TestPriorityRouting_HigherPriorityFirst tests US4:
// Plugins with higher priority are queried first.
func TestPriorityRouting_HigherPriorityFirst(t *testing.T) {
	ctx := context.Background()

	// Create mock plugin clients with different priorities
	lowPriClient := &pluginhost.Client{
		Name: "low-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	medPriClient := &pluginhost.Client{
		Name: "med-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	highPriClient := &pluginhost.Client{
		Name: "high-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "low-priority", Priority: 5},
			{Name: "high-priority", Priority: 30},
			{Name: "med-priority", Priority: 15},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{lowPriClient, medPriClient, highPriClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3)

	t.Run("highest priority first", func(t *testing.T) {
		assert.Equal(t, "high-priority", matches[0].Client.Name)
		assert.Equal(t, 30, matches[0].Priority)
	})

	t.Run("medium priority second", func(t *testing.T) {
		assert.Equal(t, "med-priority", matches[1].Client.Name)
		assert.Equal(t, 15, matches[1].Priority)
	})

	t.Run("lowest priority last", func(t *testing.T) {
		assert.Equal(t, "low-priority", matches[2].Client.Name)
		assert.Equal(t, 5, matches[2].Priority)
	})
}

// TestPriorityRouting_EqualPriorityQueryAll tests FR-014:
// When all plugins have equal priority, all are returned for parallel query.
func TestPriorityRouting_EqualPriorityQueryAll(t *testing.T) {
	ctx := context.Background()

	plugin1 := &pluginhost.Client{
		Name: "plugin-1",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	plugin2 := &pluginhost.Client{
		Name: "plugin-2",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	plugin3 := &pluginhost.Client{
		Name: "plugin-3",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "plugin-1", Priority: 10},
			{Name: "plugin-2", Priority: 10}, // Same priority
			{Name: "plugin-3", Priority: 10}, // Same priority
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{plugin1, plugin2, plugin3}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3, "all equal priority plugins should be returned")

	// Check all have same priority
	assert.True(t, router.AllEqualPriority(matches))

	// Verify all plugins are present
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Client.Name
	}
	assert.Contains(t, names, "plugin-1")
	assert.Contains(t, names, "plugin-2")
	assert.Contains(t, names, "plugin-3")
}

// TestPriorityRouting_ZeroPriorityDefault tests that plugins without
// explicit priority are treated as priority 0.
func TestPriorityRouting_ZeroPriorityDefault(t *testing.T) {
	ctx := context.Background()

	defaultPriClient := &pluginhost.Client{
		Name: "default-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	highPriClient := &pluginhost.Client{
		Name: "high-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "default-priority"}, // No priority = 0
			{Name: "high-priority", Priority: 20},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{defaultPriClient, highPriClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 2)

	// High priority should be first
	assert.Equal(t, "high-priority", matches[0].Client.Name)
	assert.Equal(t, 20, matches[0].Priority)

	// Default priority (0) should be last
	assert.Equal(t, "default-priority", matches[1].Client.Name)
	assert.Equal(t, 0, matches[1].Priority)
}

// TestPriorityRouting_PatternPriorityInteraction tests that pattern matches
// respect priority ordering.
func TestPriorityRouting_PatternPriorityInteraction(t *testing.T) {
	ctx := context.Background()

	lowPriPattern := &pluginhost.Client{
		Name: "low-pri-pattern",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	highPriPattern := &pluginhost.Client{
		Name: "high-pri-pattern",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "low-pri-pattern",
				Priority: 5,
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:ec2:*"},
				},
			},
			{
				Name:     "high-pri-pattern",
				Priority: 20,
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:ec2:*"},
				},
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{lowPriPattern, highPriPattern}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 2)

	// Both match by pattern, but higher priority first
	assert.Equal(t, "high-pri-pattern", matches[0].Client.Name)
	assert.Equal(t, router.MatchReasonPattern, matches[0].MatchReason)
	assert.Equal(t, 20, matches[0].Priority)

	assert.Equal(t, "low-pri-pattern", matches[1].Client.Name)
	assert.Equal(t, router.MatchReasonPattern, matches[1].MatchReason)
	assert.Equal(t, 5, matches[1].Priority)
}

// TestPriorityRouting_SourceFieldAttribution tests FR-014:
// Results include source field for tracking routing decisions.
func TestPriorityRouting_SourceFieldAttribution(t *testing.T) {
	ctx := context.Background()

	patternClient := &pluginhost.Client{
		Name: "pattern-match",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	automaticClient := &pluginhost.Client{
		Name: "automatic-match",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "pattern-match",
				Priority: 20,
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:eks:*"},
				},
			},
			{
				Name:     "automatic-match",
				Priority: 10,
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{patternClient, automaticClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	t.Run("pattern match has config source", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:eks:Cluster"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.GreaterOrEqual(t, len(matches), 1)
		patternMatch := matches[0]
		assert.Equal(t, "pattern-match", patternMatch.Client.Name)
		assert.Equal(t, "config", patternMatch.Source)
		assert.Equal(t, router.MatchReasonPattern, patternMatch.MatchReason)
	})

	t.Run("provider match has automatic source", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		// All matches should have automatic source
		for _, m := range matches {
			assert.Equal(t, "automatic", m.Source)
			assert.Equal(t, router.MatchReasonAutomatic, m.MatchReason)
		}
	})
}
