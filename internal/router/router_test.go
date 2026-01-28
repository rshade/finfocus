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

// mockClient creates a mock plugin client for testing.
func mockClient(name string, providers []string) *pluginhost.Client {
	return &pluginhost.Client{
		Name: name,
		Metadata: &proto.PluginMetadata{
			SupportedProviders: providers,
		},
	}
}

func TestNewRouter(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "no options creates empty router",
			opts:    nil,
			wantErr: false,
		},
		{
			name: "with clients",
			opts: []Option{
				WithClients([]*pluginhost.Client{
					mockClient("aws-public", []string{"aws"}),
				}),
			},
			wantErr: false,
		},
		{
			name: "with valid config",
			opts: []Option{
				WithConfig(&config.RoutingConfig{
					Plugins: []config.PluginRouting{
						{Name: "aws-public", Priority: 10},
					},
				}),
				WithClients([]*pluginhost.Client{
					mockClient("aws-public", []string{"aws"}),
				}),
			},
			wantErr: false,
		},
		{
			name: "with invalid regex pattern fails",
			opts: []Option{
				WithConfig(&config.RoutingConfig{
					Plugins: []config.PluginRouting{
						{
							Name: "bad-plugin",
							Patterns: []config.ResourcePattern{
								{Type: "regex", Pattern: "aws:(ec2|rds"},
							},
						},
					},
				}),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewRouter(tt.opts...)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, router)
		})
	}
}

func TestSelectPlugins_AutomaticRouting(t *testing.T) {
	ctx := context.Background()

	awsClient := mockClient("aws-public", []string{"aws"})
	gcpClient := mockClient("gcp-public", []string{"gcp"})
	globalClient := mockClient("recorder", []string{"*"})

	router, err := NewRouter(WithClients([]*pluginhost.Client{awsClient, gcpClient, globalClient}))
	require.NoError(t, err)

	tests := []struct {
		name        string
		resource    engine.ResourceDescriptor
		feature     string
		wantPlugins []string
		wantReasons []MatchReason
	}{
		{
			name:        "AWS resource matches AWS plugin and global",
			resource:    engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"},
			feature:     "ProjectedCosts",
			wantPlugins: []string{"aws-public", "recorder"},
			wantReasons: []MatchReason{MatchReasonAutomatic, MatchReasonGlobal},
		},
		{
			name:        "GCP resource matches GCP plugin and global",
			resource:    engine.ResourceDescriptor{Type: "gcp:compute:Instance"},
			feature:     "ProjectedCosts",
			wantPlugins: []string{"gcp-public", "recorder"},
			wantReasons: []MatchReason{MatchReasonAutomatic, MatchReasonGlobal},
		},
		{
			name:        "Unknown provider matches only global",
			resource:    engine.ResourceDescriptor{Type: "azure:compute:VM"},
			feature:     "ProjectedCosts",
			wantPlugins: []string{"recorder"},
			wantReasons: []MatchReason{MatchReasonGlobal},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := router.SelectPlugins(ctx, tt.resource, tt.feature)

			require.Len(t, matches, len(tt.wantPlugins),
				"expected %d matches, got %d", len(tt.wantPlugins), len(matches))

			for i, wantName := range tt.wantPlugins {
				assert.Equal(t, wantName, matches[i].Client.Name)
				assert.Equal(t, tt.wantReasons[i], matches[i].MatchReason)
				assert.Equal(t, "automatic", matches[i].Source)
			}
		})
	}
}

func TestSelectPlugins_PatternRouting(t *testing.T) {
	ctx := context.Background()

	awsClient := mockClient("aws-public", []string{"aws"})
	eksClient := mockClient("eks-costs", []string{"aws"})

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "eks-costs",
				Priority: 30,
				Patterns: []config.ResourcePattern{
					{Type: "regex", Pattern: "aws:eks:.*"},
				},
			},
			{
				Name:     "aws-public",
				Priority: 10,
			},
		},
	}

	router, err := NewRouter(
		WithConfig(cfg),
		WithClients([]*pluginhost.Client{awsClient, eksClient}),
	)
	require.NoError(t, err)

	tests := []struct {
		name        string
		resource    engine.ResourceDescriptor
		wantPlugins []string
		wantReasons []MatchReason
	}{
		{
			name:        "EKS resource matches pattern plugin first",
			resource:    engine.ResourceDescriptor{Type: "aws:eks:Cluster"},
			wantPlugins: []string{"eks-costs", "aws-public"},
			wantReasons: []MatchReason{MatchReasonPattern, MatchReasonAutomatic},
		},
		{
			name:        "EC2 resource matches both AWS plugins by provider (sorted by priority)",
			resource:    engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"},
			wantPlugins: []string{"eks-costs", "aws-public"}, // eks-costs has priority 30, aws-public has 10
			wantReasons: []MatchReason{MatchReasonAutomatic, MatchReasonAutomatic},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := router.SelectPlugins(ctx, tt.resource, "ProjectedCosts")

			require.Len(t, matches, len(tt.wantPlugins))

			for i, wantName := range tt.wantPlugins {
				assert.Equal(t, wantName, matches[i].Client.Name)
				assert.Equal(t, tt.wantReasons[i], matches[i].MatchReason)
			}
		})
	}
}

func TestSelectPlugins_FeatureFiltering(t *testing.T) {
	ctx := context.Background()

	awsCEClient := mockClient("aws-ce", []string{"aws"})
	awsPublicClient := mockClient("aws-public", []string{"aws"})

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-ce",
				Features: []string{"Recommendations"},
				Priority: 20,
			},
			{
				Name:     "aws-public",
				Features: []string{"ProjectedCosts", "ActualCosts"},
				Priority: 10,
			},
		},
	}

	router, err := NewRouter(
		WithConfig(cfg),
		WithClients([]*pluginhost.Client{awsCEClient, awsPublicClient}),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}

	t.Run("Recommendations feature matches aws-ce only", func(t *testing.T) {
		matches := router.SelectPlugins(ctx, resource, "Recommendations")
		require.Len(t, matches, 1)
		assert.Equal(t, "aws-ce", matches[0].Client.Name)
	})

	t.Run("ProjectedCosts feature matches aws-public only", func(t *testing.T) {
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")
		require.Len(t, matches, 1)
		assert.Equal(t, "aws-public", matches[0].Client.Name)
	})

	t.Run("Unknown feature matches neither", func(t *testing.T) {
		matches := router.SelectPlugins(ctx, resource, "Carbon")
		require.Len(t, matches, 0)
	})
}

func TestSelectPlugins_PriorityOrdering(t *testing.T) {
	ctx := context.Background()

	lowPriClient := mockClient("low-priority", []string{"aws"})
	medPriClient := mockClient("med-priority", []string{"aws"})
	highPriClient := mockClient("high-priority", []string{"aws"})

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "low-priority", Priority: 5},
			{Name: "high-priority", Priority: 30},
			{Name: "med-priority", Priority: 15},
		},
	}

	router, err := NewRouter(
		WithConfig(cfg),
		WithClients([]*pluginhost.Client{lowPriClient, medPriClient, highPriClient}),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 3)
	assert.Equal(t, "high-priority", matches[0].Client.Name)
	assert.Equal(t, 30, matches[0].Priority)
	assert.Equal(t, "med-priority", matches[1].Client.Name)
	assert.Equal(t, 15, matches[1].Priority)
	assert.Equal(t, "low-priority", matches[2].Client.Name)
	assert.Equal(t, 5, matches[2].Priority)
}

func TestSelectPlugins_SourceAttribution(t *testing.T) {
	ctx := context.Background()

	awsClient := mockClient("aws-public", []string{"aws"})
	eksClient := mockClient("eks-costs", []string{"aws"})

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "eks-costs",
				Priority: 30,
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:eks:*"},
				},
			},
			{Name: "aws-public", Priority: 10},
		},
	}

	router, err := NewRouter(
		WithConfig(cfg),
		WithClients([]*pluginhost.Client{awsClient, eksClient}),
	)
	require.NoError(t, err)

	t.Run("Pattern match has config source", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:eks:Cluster"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 2)
		assert.Equal(t, "config", matches[0].Source)    // eks-costs by pattern
		assert.Equal(t, "automatic", matches[1].Source) // aws-public by provider
	})

	t.Run("Provider match has automatic source", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
		matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Both plugins support AWS, so both match via automatic routing
		require.Len(t, matches, 2)
		assert.Equal(t, "automatic", matches[0].Source) // eks-costs (priority 30)
		assert.Equal(t, "automatic", matches[1].Source) // aws-public (priority 10)
	})
}

func TestShouldFallback(t *testing.T) {
	falseVal := false
	trueVal := true

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{Name: "fallback-true", Fallback: &trueVal},
			{Name: "fallback-false", Fallback: &falseVal},
			{Name: "fallback-default"}, // nil = true
		},
	}

	router, err := NewRouter(WithConfig(cfg))
	require.NoError(t, err)

	assert.True(t, router.ShouldFallback("fallback-true"))
	assert.False(t, router.ShouldFallback("fallback-false"))
	assert.True(t, router.ShouldFallback("fallback-default"))
	assert.True(t, router.ShouldFallback("unknown-plugin")) // unknown = default true
}

func TestAllEqualPriority(t *testing.T) {
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
			name: "all equal",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllEqualPriority(tt.matches)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchReason_String(t *testing.T) {
	assert.Equal(t, "automatic", MatchReasonAutomatic.String())
	assert.Equal(t, "pattern", MatchReasonPattern.String())
	assert.Equal(t, "global", MatchReasonGlobal.String())
	assert.Equal(t, "unknown", MatchReason(-1).String())
}

func TestSelectPlugins_WildcardProvider(t *testing.T) {
	ctx := context.Background()

	// Test that ["*"] in SupportedProviders is treated as global
	wildcardClient := mockClient("global-plugin", []string{"*"})
	emptyClient := mockClient("empty-providers", []string{})

	router, err := NewRouter(WithClients([]*pluginhost.Client{wildcardClient, emptyClient}))
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "any:resource:Type"}
	matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 2)
	for _, m := range matches {
		assert.Equal(t, MatchReasonGlobal, m.MatchReason)
	}
}
