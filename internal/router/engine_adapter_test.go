package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestEngineAdapter_SelectPlugins(t *testing.T) {
	tests := []struct {
		name           string
		matches        []PluginMatch
		wantReasons    []string
		wantPriorities []int
		wantFallbacks  []bool
		wantSources    []string
	}{
		{
			name:           "empty matches returns empty slice",
			matches:        []PluginMatch{},
			wantReasons:    []string{},
			wantPriorities: []int{},
			wantFallbacks:  []bool{},
			wantSources:    []string{},
		},
		{
			name: "automatic match reason converts correctly",
			matches: []PluginMatch{
				{
					Client:      mockClient("aws-public", []string{"aws"}),
					Priority:    0,
					Fallback:    true,
					MatchReason: MatchReasonAutomatic,
					Source:      "automatic",
				},
			},
			wantReasons:    []string{"automatic"},
			wantPriorities: []int{0},
			wantFallbacks:  []bool{true},
			wantSources:    []string{"automatic"},
		},
		{
			name: "pattern match reason converts correctly",
			matches: []PluginMatch{
				{
					Client:      mockClient("aws-ce", []string{"aws"}),
					Priority:    10,
					Fallback:    false,
					MatchReason: MatchReasonPattern,
					Source:      "config",
				},
			},
			wantReasons:    []string{"pattern"},
			wantPriorities: []int{10},
			wantFallbacks:  []bool{false},
			wantSources:    []string{"config"},
		},
		{
			name: "global match reason converts correctly",
			matches: []PluginMatch{
				{
					Client:      mockClient("recorder", []string{"*"}),
					Priority:    0,
					Fallback:    true,
					MatchReason: MatchReasonGlobal,
					Source:      "automatic",
				},
			},
			wantReasons:    []string{"global"},
			wantPriorities: []int{0},
			wantFallbacks:  []bool{true},
			wantSources:    []string{"automatic"},
		},
		{
			name: "no_match reason converts correctly",
			matches: []PluginMatch{
				{
					Client:      mockClient("orphan", nil),
					Priority:    0,
					Fallback:    true,
					MatchReason: MatchReasonNoMatch,
					Source:      "automatic",
				},
			},
			wantReasons:    []string{"no_match"},
			wantPriorities: []int{0},
			wantFallbacks:  []bool{true},
			wantSources:    []string{"automatic"},
		},
		{
			name: "multiple matches preserve order and fields",
			matches: []PluginMatch{
				{
					Client:      mockClient("aws-ce", []string{"aws"}),
					Priority:    10,
					Fallback:    false,
					MatchReason: MatchReasonPattern,
					Source:      "config",
				},
				{
					Client:      mockClient("aws-public", []string{"aws"}),
					Priority:    5,
					Fallback:    true,
					MatchReason: MatchReasonAutomatic,
					Source:      "automatic",
				},
				{
					Client:      mockClient("recorder", []string{"*"}),
					Priority:    0,
					Fallback:    true,
					MatchReason: MatchReasonGlobal,
					Source:      "automatic",
				},
			},
			wantReasons:    []string{"pattern", "automatic", "global"},
			wantPriorities: []int{10, 5, 0},
			wantFallbacks:  []bool{false, true, true},
			wantSources:    []string{"config", "automatic", "automatic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubRouter{matches: tt.matches, fallback: true}
			adapter := NewEngineAdapter(stub)
			ctx := context.Background()
			resource := engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}

			result := adapter.SelectPlugins(ctx, resource, "ProjectedCosts")

			require.Len(t, result, len(tt.matches))
			for i, m := range result {
				assert.Equal(t, tt.wantReasons[i], m.MatchReason, "MatchReason mismatch at index %d", i)
				assert.Equal(t, tt.wantPriorities[i], m.Priority, "Priority mismatch at index %d", i)
				assert.Equal(t, tt.wantFallbacks[i], m.Fallback, "Fallback mismatch at index %d", i)
				assert.Equal(t, tt.wantSources[i], m.Source, "Source mismatch at index %d", i)
				assert.Equal(t, tt.matches[i].Client, m.Client, "Client mismatch at index %d", i)
			}
		})
	}
}

func TestEngineAdapter_SelectPlugins_FeaturePassthrough(t *testing.T) {
	stub := &stubRouter{
		matches:  []PluginMatch{},
		fallback: true,
	}
	adapter := NewEngineAdapter(stub)
	ctx := context.Background()
	resource := engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}

	features := []string{"ProjectedCosts", "ActualCosts", "Recommendations", ""}
	for _, feature := range features {
		adapter.SelectPlugins(ctx, resource, feature)
		assert.Equal(t, feature, stub.lastFeature, "feature should be passed through unchanged")
	}
}

func TestEngineAdapter_ShouldFallback(t *testing.T) {
	tests := []struct {
		name       string
		fallback   bool
		pluginName string
	}{
		{
			name:       "delegates true",
			fallback:   true,
			pluginName: "aws-public",
		},
		{
			name:       "delegates false",
			fallback:   false,
			pluginName: "aws-ce",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubRouter{fallback: tt.fallback}
			adapter := NewEngineAdapter(stub)

			result := adapter.ShouldFallback(tt.pluginName)

			assert.Equal(t, tt.fallback, result)
			assert.Equal(t, tt.pluginName, stub.lastFallbackName)
		})
	}
}

func TestEngineAdapter_WithRealRouter(t *testing.T) {
	clients := []*pluginhost.Client{
		mockClient("aws-public", []string{"aws"}),
	}

	r, err := NewRouter(
		WithClients(clients),
		WithConfig(&config.RoutingConfig{
			Plugins: []config.PluginRouting{
				{Name: "aws-public", Priority: 10},
			},
		}),
	)
	require.NoError(t, err)

	adapter := NewEngineAdapter(r)
	ctx := context.Background()
	resource := engine.ResourceDescriptor{
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
	}

	matches := adapter.SelectPlugins(ctx, resource, "ProjectedCosts")
	require.Len(t, matches, 1)
	assert.Equal(t, "aws-public", matches[0].Client.Name)
	assert.Equal(t, 10, matches[0].Priority)
	assert.Equal(t, "automatic", matches[0].MatchReason)
}

func TestEngineAdapter_PriorityAndFallbackPreservation(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		fallback bool
	}{
		{"zero priority with fallback", 0, true},
		{"high priority without fallback", 100, false},
		{"negative priority with fallback", -1, true},
		{"priority 10 without fallback", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubRouter{
				matches: []PluginMatch{
					{
						Client:      mockClient("test-plugin", []string{"aws"}),
						Priority:    tt.priority,
						Fallback:    tt.fallback,
						MatchReason: MatchReasonAutomatic,
						Source:      "automatic",
					},
				},
				fallback: tt.fallback,
			}
			adapter := NewEngineAdapter(stub)
			ctx := context.Background()
			resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}

			matches := adapter.SelectPlugins(ctx, resource, "ProjectedCosts")

			require.Len(t, matches, 1)
			assert.Equal(t, tt.priority, matches[0].Priority, "Priority should be preserved")
			assert.Equal(t, tt.fallback, matches[0].Fallback, "Fallback should be preserved")
		})
	}
}

func TestEngineAdapter_MatchReasonNoMatch_EdgeCase(t *testing.T) {
	// MatchReasonNoMatch has value -1 (iota - 1), verify it converts correctly.
	stub := &stubRouter{
		matches: []PluginMatch{
			{
				Client:      mockClient("test", nil),
				MatchReason: MatchReasonNoMatch,
				Source:      "automatic",
			},
		},
	}
	adapter := NewEngineAdapter(stub)
	ctx := context.Background()

	matches := adapter.SelectPlugins(ctx, engine.ResourceDescriptor{Type: "unknown:resource"}, "")
	require.Len(t, matches, 1)
	assert.Equal(t, "no_match", matches[0].MatchReason)
}

func TestNewEngineAdapter_NilRouter(t *testing.T) {
	adapter := NewEngineAdapter(nil)
	assert.Nil(t, adapter, "NewEngineAdapter(nil) should return nil")
}

// stubRouter is a test double for router.Router.
type stubRouter struct {
	matches          []PluginMatch
	fallback         bool
	lastFeature      string
	lastFallbackName string
}

func (s *stubRouter) SelectPlugins(_ context.Context, _ engine.ResourceDescriptor, feature string) []PluginMatch {
	s.lastFeature = feature
	return s.matches
}

func (s *stubRouter) ShouldFallback(pluginName string) bool {
	s.lastFallbackName = pluginName
	return s.fallback
}

func (s *stubRouter) Validate(_ context.Context) ValidationResult {
	return ValidationResult{}
}
