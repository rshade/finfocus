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

// renamedAWSPublic is aws-public v0.2.5: installed as aws-public, reporting
// finfocus-plugin-aws-public.
func renamedAWSPublic() *pluginhost.Client {
	return &pluginhost.Client{
		Name:        "finfocus-plugin-aws-public",
		InstallName: "aws-public",
		Metadata:    &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
}

// postRenameAWSPublic is aws-public after it reports its install name, with
// the catalog alias for its former name.
func postRenameAWSPublic() *pluginhost.Client {
	return &pluginhost.Client{
		Name:        "aws-public",
		InstallName: "aws-public",
		Aliases:     []string{"finfocus-plugin-aws-public"},
		Metadata:    &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
}

func matchedNames(matches []PluginMatch) []string {
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Client.Name)
	}
	return names
}

func TestRouter_EntryMatchesEveryPluginName(t *testing.T) {
	t.Parallel()

	ec2 := engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}
	eks := engine.ResourceDescriptor{Type: "aws:eks:Cluster"}
	noFallback := false

	cases := []struct {
		name     string
		client   func() *pluginhost.Client
		spelling string
	}{
		{"install name", renamedAWSPublic, "aws-public"},
		{"reported name", renamedAWSPublic, "finfocus-plugin-aws-public"},
		{"alias after rename", postRenameAWSPublic, "finfocus-plugin-aws-public"},
		{"install name after rename", postRenameAWSPublic, "aws-public"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			t.Run("feature rule restricts the plugin", func(t *testing.T) {
				t.Parallel()
				awsPublic := tc.client()
				r, err := NewRouter(
					WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
						{Name: tc.spelling, Features: []string{"ProjectedCosts"}},
					}}),
					WithClients([]*pluginhost.Client{awsPublic, mockClient("aws-ce", []string{"aws"})}),
				)
				require.NoError(t, err)

				assert.Equal(t, []string{awsPublic.Name, "aws-ce"},
					matchedNames(r.SelectPlugins(context.Background(), ec2, "ProjectedCosts")))
				assert.Equal(t, []string{"aws-ce"},
					matchedNames(r.SelectPlugins(context.Background(), ec2, "ActualCosts")))
				assert.Empty(t, r.UnmatchedEntries())
			})

			t.Run("pattern rule selects the plugin once", func(t *testing.T) {
				t.Parallel()
				awsPublic := tc.client()
				r, err := NewRouter(
					WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
						{
							Name:     tc.spelling,
							Priority: 30,
							Patterns: []config.ResourcePattern{{Type: "regex", Pattern: "aws:eks:.*"}},
						},
					}}),
					WithClients([]*pluginhost.Client{awsPublic, mockClient("aws-ce", []string{"aws"})}),
				)
				require.NoError(t, err)

				matches := r.SelectPlugins(context.Background(), eks, "ProjectedCosts")
				require.Len(t, matches, 2)
				assert.Same(t, awsPublic, matches[0].Client)
				assert.Equal(t, MatchReasonPattern, matches[0].MatchReason)
				assert.Equal(t, "aws-ce", matches[1].Client.Name)
			})

			t.Run("fallback follows the entry", func(t *testing.T) {
				t.Parallel()
				awsPublic := tc.client()
				r, err := NewRouter(
					WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
						{Name: tc.spelling, Fallback: &noFallback},
					}}),
					WithClients([]*pluginhost.Client{awsPublic}),
				)
				require.NoError(t, err)

				assert.False(t, r.ShouldFallback(awsPublic.Name), "the engine asks with the reported name")
				assert.False(t, r.ShouldFallback(tc.spelling))
			})
		})
	}
}

func TestRouter_UnmatchedEntries(t *testing.T) {
	t.Parallel()

	r, err := NewRouter(
		WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
			{Name: "aws-public", Features: []string{"ProjectedCosts"}},
			{Name: "aws-publik", Features: []string{"ActualCosts"}},
			{Name: "aws-ce"},
		}}),
		WithClients([]*pluginhost.Client{renamedAWSPublic(), mockClient("aws-ce", []string{"aws"})}),
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"aws-publik"}, r.UnmatchedEntries())
}

func TestRouter_InstallNameWinsOverReportedName(t *testing.T) {
	t.Parallel()

	impostor := &pluginhost.Client{
		Name:        "aws-public",
		InstallName: "custom",
		Metadata:    &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
	awsPublic := renamedAWSPublic()
	r, err := NewRouter(
		WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
			{Name: "aws-public", Features: []string{"ActualCosts"}},
		}}),
		WithClients([]*pluginhost.Client{impostor, awsPublic}),
	)
	require.NoError(t, err)

	matches := r.SelectPlugins(context.Background(),
		engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}, "ProjectedCosts")
	assert.Equal(t, []string{"aws-public"}, matchedNames(matches),
		"the rule applies to the plugin installed as aws-public, so only the impostor answers ProjectedCosts")
}

func TestRouter_ReportedNameEntryRestrictsEveryPluginReportingIt(t *testing.T) {
	t.Parallel()

	first := &pluginhost.Client{
		Name: "shared", InstallName: "shared-us-east-1",
		Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
	second := &pluginhost.Client{
		Name: "shared", InstallName: "shared-us-west-2",
		Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
	other := mockClient("aws-ce", []string{"aws"})
	r, err := NewRouter(
		WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
			{Name: "shared", Features: []string{"ProjectedCosts"}},
		}}),
		WithClients([]*pluginhost.Client{first, second, other}),
	)
	require.NoError(t, err)

	ec2 := engine.ResourceDescriptor{Type: "aws:ec2/instance:Instance"}
	assert.Equal(t, []string{"aws-ce"}, matchedNames(r.SelectPlugins(context.Background(), ec2, "ActualCosts")),
		"the rule written with the shared reported name restricts both plugins, as it did before install names")
	assert.Equal(t, []string{"shared", "aws-ce"},
		matchedNames(r.SelectPlugins(context.Background(), ec2, "ProjectedCosts")),
		"plugins sharing a reported name match once, as before: the engine groups work by reported name")
}

func TestRouter_TwoEntriesForOnePluginUseTheLastOne(t *testing.T) {
	t.Parallel()

	awsPublic := renamedAWSPublic()
	awsPublic.Aliases = []string{"aws-public-legacy"}
	glob := []config.ResourcePattern{{Type: "glob", Pattern: "aws:eks:*"}}
	r, err := NewRouter(
		WithConfig(&config.RoutingConfig{Plugins: []config.PluginRouting{
			{Name: "aws-public", Patterns: glob, Features: []string{"ProjectedCosts"}},
			{Name: "aws-public-legacy", Patterns: glob, Features: []string{"ActualCosts"}},
		}}),
		WithClients([]*pluginhost.Client{awsPublic}),
	)
	require.NoError(t, err)

	eks := engine.ResourceDescriptor{Type: "aws:eks:Cluster"}
	assert.Empty(t, r.SelectPlugins(context.Background(), eks, "ProjectedCosts"),
		"the earlier entry is superseded, so its feature list grants nothing")
	matches := r.SelectPlugins(context.Background(), eks, "ActualCosts")
	require.Len(t, matches, 1, "the plugin is matched once")
	assert.Equal(t, MatchReasonPattern, matches[0].MatchReason)
}
