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

// TestFallbackChain_PriorityOrdering tests US5:
// When primary plugin fails, fallback to next priority plugin.
func TestFallbackChain_PriorityOrdering(t *testing.T) {
	ctx := context.Background()

	trueVal := true

	primaryClient := &pluginhost.Client{
		Name: "primary-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	secondaryClient := &pluginhost.Client{
		Name: "secondary-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	tertiaryClient := &pluginhost.Client{
		Name: "tertiary-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "primary-plugin", Priority: 30, Fallback: &trueVal},
			{Name: "secondary-plugin", Priority: 20, Fallback: &trueVal},
			{Name: "tertiary-plugin", Priority: 10, Fallback: &trueVal},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{primaryClient, secondaryClient, tertiaryClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3)

	t.Run("plugins ordered by priority for fallback chain", func(t *testing.T) {
		// First in chain: primary (priority 30)
		assert.Equal(t, "primary-plugin", matches[0].Client.Name)
		assert.Equal(t, 30, matches[0].Priority)
		assert.True(t, matches[0].Fallback)

		// Second in chain: secondary (priority 20)
		assert.Equal(t, "secondary-plugin", matches[1].Client.Name)
		assert.Equal(t, 20, matches[1].Priority)
		assert.True(t, matches[1].Fallback)

		// Third in chain: tertiary (priority 10)
		assert.Equal(t, "tertiary-plugin", matches[2].Client.Name)
		assert.Equal(t, 10, matches[2].Priority)
		assert.True(t, matches[2].Fallback)
	})

	t.Run("all plugins have fallback enabled", func(t *testing.T) {
		for _, m := range matches {
			assert.True(t, r.ShouldFallback(m.Client.Name), "plugin %s should have fallback enabled", m.Client.Name)
		}
	})
}

// TestFallbackChain_DisabledInterrupts tests that disabled fallback stops the chain.
func TestFallbackChain_DisabledInterrupts(t *testing.T) {
	ctx := context.Background()

	trueVal := true
	falseVal := false

	primaryClient := &pluginhost.Client{
		Name: "primary-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	noFallbackClient := &pluginhost.Client{
		Name: "no-fallback-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	backupClient := &pluginhost.Client{
		Name: "backup-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "primary-plugin", Priority: 30, Fallback: &trueVal},
			{Name: "no-fallback-plugin", Priority: 20, Fallback: &falseVal}, // Chain breaker
			{Name: "backup-plugin", Priority: 10, Fallback: &trueVal},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{primaryClient, noFallbackClient, backupClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3)

	t.Run("middle plugin has fallback disabled", func(t *testing.T) {
		assert.True(t, matches[0].Fallback, "primary should have fallback")
		assert.False(t, matches[1].Fallback, "no-fallback-plugin should NOT have fallback")
		assert.True(t, matches[2].Fallback, "backup should have fallback")
	})

	t.Run("ShouldFallback returns correct values", func(t *testing.T) {
		assert.True(t, r.ShouldFallback("primary-plugin"))
		assert.False(t, r.ShouldFallback("no-fallback-plugin"))
		assert.True(t, r.ShouldFallback("backup-plugin"))
	})
}

// TestFallbackChain_ZeroCostValid tests that $0 cost is valid and doesn't trigger fallback.
// Per F-007: $0 is a valid result (e.g., free-tier), NOT an empty result.
func TestFallbackChain_ZeroCostValid(t *testing.T) {
	// This integration test documents the expected behavior:
	// When a plugin returns a $0 cost, the engine should NOT fallback
	// because $0 is a valid, meaningful cost (e.g., free-tier resources).

	// The router only provides the fallback configuration.
	// The engine decides what triggers fallback vs what's a valid result.

	trueVal := true

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "free-tier-plugin", Priority: 20, Fallback: &trueVal},
			{Name: "backup-plugin", Priority: 10, Fallback: &trueVal},
		},
	}

	r, err := router.NewRouter(router.WithConfig(cfg))
	require.NoError(t, err)

	// Verify fallback configuration is available
	assert.True(t, r.ShouldFallback("free-tier-plugin"))
	assert.True(t, r.ShouldFallback("backup-plugin"))

	// The actual $0 vs empty logic is tested in engine tests
	t.Log("$0 cost semantics: free-tier resources return $0, which is valid (NO fallback)")
	t.Log("Empty result semantics: nil/no-data triggers fallback to next plugin")
}

// TestFallbackChain_DefaultEnabled tests that default fallback is enabled.
func TestFallbackChain_DefaultEnabled(t *testing.T) {
	ctx := context.Background()

	client := &pluginhost.Client{
		Name: "default-fallback",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "default-fallback", Priority: 10}, // No Fallback field = default true
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{client}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 1)
	assert.True(t, matches[0].Fallback, "default fallback should be true")
	assert.True(t, r.ShouldFallback("default-fallback"))
}

// TestFallbackChain_MixedPatternAndAutomatic tests fallback across match types.
func TestFallbackChain_MixedPatternAndAutomatic(t *testing.T) {
	ctx := context.Background()

	trueVal := true

	patternClient := &pluginhost.Client{
		Name: "pattern-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	automaticClient := &pluginhost.Client{
		Name: "automatic-plugin",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "pattern-plugin",
				Priority: 20,
				Fallback: &trueVal,
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:eks:*"},
				},
			},
			{Name: "automatic-plugin", Priority: 10, Fallback: &trueVal},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{patternClient, automaticClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	t.Run("EKS resource gets pattern match first", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:eks:Cluster"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.GreaterOrEqual(t, len(matches), 1)
		assert.Equal(t, "pattern-plugin", matches[0].Client.Name)
		assert.Equal(t, router.MatchReasonPattern, matches[0].MatchReason)
		assert.True(t, matches[0].Fallback)
	})

	t.Run("EC2 resource gets automatic match from both", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Both plugins support AWS, so both match via automatic routing
		// pattern-plugin has higher priority (20) so comes first
		// automatic-plugin has lower priority (10) so comes second
		require.Len(t, matches, 2)

		// First: pattern-plugin (priority 20, but matched via automatic since no pattern for ec2)
		assert.Equal(t, "pattern-plugin", matches[0].Client.Name)
		assert.Equal(t, router.MatchReasonAutomatic, matches[0].MatchReason)
		assert.True(t, matches[0].Fallback)

		// Second: automatic-plugin (priority 10)
		assert.Equal(t, "automatic-plugin", matches[1].Client.Name)
		assert.Equal(t, router.MatchReasonAutomatic, matches[1].MatchReason)
		assert.True(t, matches[1].Fallback)
	})
}

// TestFallbackChain_UnknownPlugin tests unknown plugin defaults to fallback enabled.
func TestFallbackChain_UnknownPlugin(t *testing.T) {
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "known-plugin", Priority: 10},
		},
	}

	r, err := router.NewRouter(router.WithConfig(cfg))
	require.NoError(t, err)

	// Unknown plugins default to fallback enabled
	assert.True(t, r.ShouldFallback("unknown-plugin"))
	assert.True(t, r.ShouldFallback("another-unknown"))
}
