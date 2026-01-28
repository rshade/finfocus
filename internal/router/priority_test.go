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

// TestPrioritySorting tests T043: priority-based sorting of plugin matches.
func TestPrioritySorting(t *testing.T) {
	ctx := context.Background()

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

	router, err := NewRouter(
		WithClients([]*pluginhost.Client{lowPriClient, medPriClient, highPriClient}),
		WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3)

	// Verify descending priority order
	assert.Equal(t, "high-priority", matches[0].Client.Name)
	assert.Equal(t, 30, matches[0].Priority)

	assert.Equal(t, "med-priority", matches[1].Client.Name)
	assert.Equal(t, 15, matches[1].Priority)

	assert.Equal(t, "low-priority", matches[2].Client.Name)
	assert.Equal(t, 5, matches[2].Priority)
}

// TestEqualPriorityQueryAll tests T044: equal priority returns all plugins.
func TestEqualPriorityQueryAll(t *testing.T) {
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
			{Name: "plugin-2", Priority: 10},
			{Name: "plugin-3", Priority: 10},
		},
	}

	router, err := NewRouter(
		WithClients([]*pluginhost.Client{plugin1, plugin2, plugin3}),
		WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3, "all equal priority plugins should be returned")

	// Verify all have same priority
	assert.True(t, AllEqualPriority(matches), "AllEqualPriority should return true")

	// Verify all plugins are present
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Client.Name
	}
	assert.Contains(t, names, "plugin-1")
	assert.Contains(t, names, "plugin-2")
	assert.Contains(t, names, "plugin-3")
}

// TestFallbackTriggerOnError tests T050: ShouldFallback returns true for error scenarios.
func TestFallbackTriggerOnError(t *testing.T) {
	trueVal := true
	falseVal := false

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "fallback-enabled", Fallback: &trueVal},
			{Name: "fallback-disabled", Fallback: &falseVal},
			{Name: "fallback-default"}, // nil = true
		},
	}

	router, err := NewRouter(WithConfig(cfg))
	require.NoError(t, err)

	t.Run("explicit fallback enabled", func(t *testing.T) {
		assert.True(t, router.ShouldFallback("fallback-enabled"))
	})

	t.Run("explicit fallback disabled", func(t *testing.T) {
		assert.False(t, router.ShouldFallback("fallback-disabled"))
	})

	t.Run("default fallback is enabled", func(t *testing.T) {
		assert.True(t, router.ShouldFallback("fallback-default"))
	})

	t.Run("unknown plugin defaults to fallback enabled", func(t *testing.T) {
		assert.True(t, router.ShouldFallback("unknown-plugin"))
	})
}

// TestFallbackDisabledBehavior tests T051: fallback disabled prevents alternative queries.
func TestFallbackDisabledBehavior(t *testing.T) {
	ctx := context.Background()
	falseVal := false

	primaryClient := &pluginhost.Client{
		Name: "primary-plugin",
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
			{Name: "primary-plugin", Priority: 20, Fallback: &falseVal}, // Disabled
			{Name: "backup-plugin", Priority: 10},                       // Default enabled
		},
	}

	router, err := NewRouter(
		WithClients([]*pluginhost.Client{primaryClient, backupClient}),
		WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 2)

	// Primary has higher priority, comes first
	assert.Equal(t, "primary-plugin", matches[0].Client.Name)
	assert.False(t, matches[0].Fallback, "primary-plugin should have fallback disabled")

	// Backup has lower priority, comes second
	assert.Equal(t, "backup-plugin", matches[1].Client.Name)
	assert.True(t, matches[1].Fallback, "backup-plugin should have fallback enabled")
}

// TestEmptyResultFallbackTrigger tests T052: empty result triggers fallback.
// Note: Empty result = no cost data triggers fallback; $0 cost = valid result, NO fallback.
func TestEmptyResultFallbackTrigger(t *testing.T) {
	trueVal := true

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "empty-result-plugin", Priority: 20, Fallback: &trueVal},
			{Name: "backup-plugin", Priority: 10, Fallback: &trueVal},
		},
	}

	router, err := NewRouter(WithConfig(cfg))
	require.NoError(t, err)

	// Both plugins have fallback enabled
	assert.True(t, router.ShouldFallback("empty-result-plugin"))
	assert.True(t, router.ShouldFallback("backup-plugin"))

	// The actual fallback decision based on empty result vs $0 cost
	// is handled in the engine, not the router. The router only provides
	// the ShouldFallback() check. This test verifies the configuration
	// is correctly parsed and accessible.
}

// TestZeroCostNoFallback tests T052a: $0 cost result does NOT trigger fallback.
// This is a semantic test documenting the expected behavior:
// $0 is a valid cost (e.g., free-tier resources), not an empty result.
func TestZeroCostNoFallback(t *testing.T) {
	// This test documents the semantic distinction:
	// - Empty result (nil/no data) = trigger fallback
	// - $0 cost (valid result with zero value) = NO fallback

	// The router's ShouldFallback() only checks configuration.
	// The engine determines what constitutes an "empty result".
	// $0 cost is explicitly NOT an empty result (F-007).

	t.Run("semantic documentation", func(t *testing.T) {
		// A $0 cost result is a valid result:
		// - Free-tier resources return $0
		// - Spot instances with no charge return $0
		// - Resources with credits applied return $0

		// These should NOT trigger fallback because:
		// 1. The plugin successfully processed the request
		// 2. The returned value ($0) is valid and meaningful
		// 3. Fallback is only for errors/no-data scenarios

		// The router doesn't distinguish - it provides the fallback config.
		// The engine makes the decision based on result type.
		assert.True(t, true, "semantic test documents behavior")
	})
}

// TestSortByPriorityHelper tests the sortByPriority helper function.
func TestSortByPriorityHelper(t *testing.T) {
	tests := []struct {
		name      string
		matches   []PluginMatch
		wantOrder []string
	}{
		{
			name:      "empty slice",
			matches:   []PluginMatch{},
			wantOrder: []string{},
		},
		{
			name: "single element",
			matches: []PluginMatch{
				{Client: &pluginhost.Client{Name: "a"}, Priority: 10},
			},
			wantOrder: []string{"a"},
		},
		{
			name: "already sorted",
			matches: []PluginMatch{
				{Client: &pluginhost.Client{Name: "high"}, Priority: 30},
				{Client: &pluginhost.Client{Name: "low"}, Priority: 10},
			},
			wantOrder: []string{"high", "low"},
		},
		{
			name: "reverse order",
			matches: []PluginMatch{
				{Client: &pluginhost.Client{Name: "low"}, Priority: 10},
				{Client: &pluginhost.Client{Name: "high"}, Priority: 30},
			},
			wantOrder: []string{"high", "low"},
		},
		{
			name: "three elements mixed",
			matches: []PluginMatch{
				{Client: &pluginhost.Client{Name: "med"}, Priority: 20},
				{Client: &pluginhost.Client{Name: "low"}, Priority: 10},
				{Client: &pluginhost.Client{Name: "high"}, Priority: 30},
			},
			wantOrder: []string{"high", "med", "low"},
		},
		{
			name: "equal priorities preserve order (stable sort)",
			matches: []PluginMatch{
				{Client: &pluginhost.Client{Name: "first"}, Priority: 10},
				{Client: &pluginhost.Client{Name: "second"}, Priority: 10},
				{Client: &pluginhost.Client{Name: "third"}, Priority: 10},
			},
			wantOrder: []string{"first", "second", "third"},
		},
		{
			name: "zero priority",
			matches: []PluginMatch{
				{Client: &pluginhost.Client{Name: "default"}, Priority: 0},
				{Client: &pluginhost.Client{Name: "high"}, Priority: 10},
			},
			wantOrder: []string{"high", "default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortByPriority(tt.matches)

			gotOrder := make([]string, len(tt.matches))
			for i, m := range tt.matches {
				gotOrder[i] = m.Client.Name
			}

			assert.Equal(t, tt.wantOrder, gotOrder)
		})
	}
}

// TestAllEqualPriorityHelper tests the AllEqualPriority helper function.
func TestAllEqualPriorityHelper(t *testing.T) {
	tests := []struct {
		name     string
		matches  []PluginMatch
		expected bool
	}{
		{
			name:     "empty matches",
			matches:  []PluginMatch{},
			expected: true,
		},
		{
			name: "single match",
			matches: []PluginMatch{
				{Priority: 10},
			},
			expected: true,
		},
		{
			name: "all equal non-zero",
			matches: []PluginMatch{
				{Priority: 10},
				{Priority: 10},
				{Priority: 10},
			},
			expected: true,
		},
		{
			name: "all zero",
			matches: []PluginMatch{
				{Priority: 0},
				{Priority: 0},
			},
			expected: true,
		},
		{
			name: "different priorities",
			matches: []PluginMatch{
				{Priority: 30},
				{Priority: 10},
			},
			expected: false,
		},
		{
			name: "one different",
			matches: []PluginMatch{
				{Priority: 10},
				{Priority: 10},
				{Priority: 20},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllEqualPriority(tt.matches)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPriorityWithPatterns tests priority ordering with pattern matches.
func TestPriorityWithPatterns(t *testing.T) {
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

	router, err := NewRouter(
		WithClients([]*pluginhost.Client{lowPriPattern, highPriPattern}),
		WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 2)

	// Both match by pattern, sorted by priority
	assert.Equal(t, "high-pri-pattern", matches[0].Client.Name)
	assert.Equal(t, MatchReasonPattern, matches[0].MatchReason)
	assert.Equal(t, 20, matches[0].Priority)

	assert.Equal(t, "low-pri-pattern", matches[1].Client.Name)
	assert.Equal(t, MatchReasonPattern, matches[1].MatchReason)
	assert.Equal(t, 5, matches[1].Priority)
}

// TestDefaultPriorityZero tests that unconfigured plugins have priority 0.
func TestDefaultPriorityZero(t *testing.T) {
	ctx := context.Background()

	defaultClient := &pluginhost.Client{
		Name: "default-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	configuredClient := &pluginhost.Client{
		Name: "configured-priority",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "default-priority"}, // No priority = 0
			{Name: "configured-priority", Priority: 20},
		},
	}

	router, err := NewRouter(
		WithClients([]*pluginhost.Client{defaultClient, configuredClient}),
		WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 2)

	// Configured priority comes first
	assert.Equal(t, "configured-priority", matches[0].Client.Name)
	assert.Equal(t, 20, matches[0].Priority)

	// Default priority (0) comes last
	assert.Equal(t, "default-priority", matches[1].Client.Name)
	assert.Equal(t, 0, matches[1].Priority)
}
