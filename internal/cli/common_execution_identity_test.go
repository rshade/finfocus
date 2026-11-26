package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func capturingContext() (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	return logger.WithContext(context.Background()), &buf
}

func identityClient(reported, install string, aliases ...string) *pluginhost.Client {
	return &pluginhost.Client{
		Name: reported, InstallName: install, Aliases: aliases,
		Metadata: &proto.PluginMetadata{SupportedProviders: []string{"aws"}},
	}
}

func TestWarnUnmatchedAdapter(t *testing.T) {
	t.Parallel()

	clients := []*pluginhost.Client{identityClient("finfocus-plugin-aws-public", "aws-public", "aws-public-legacy")}
	tests := []struct {
		adapter  string
		wantWarn bool
	}{
		{"", false},
		{"aws-public", false},
		{"aws-public-legacy", false},
		{"finfocus-plugin-aws-public", false},
		{"aws-publik", true},
	}
	for _, tc := range tests {
		t.Run(tc.adapter, func(t *testing.T) {
			t.Parallel()
			ctx, logs := capturingContext()

			warnUnmatchedAdapter(ctx, tc.adapter, clients)

			if !tc.wantWarn {
				assert.Empty(t, logs.String())
				return
			}
			assert.Contains(t, logs.String(), `"level":"warn"`)
			assert.Contains(t, logs.String(), `"adapter":"aws-publik"`)
			assert.Contains(t, logs.String(), "no loaded plugin answers to the --adapter value")
		})
	}
}

func TestCreateRouterForEngine_WarnsOnUnmatchedEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_PLUGIN_DIR", "")
	installed := filepath.Join(home, "plugins", "cloudzero", "v0.1.1")
	require.NoError(t, os.MkdirAll(installed, 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(installed, "finfocus-plugin-cloudzero"), []byte("#!/bin/sh\n"), 0o755),
	)

	cfg := config.New()
	cfg.Routing = &config.RoutingConfig{Plugins: []config.PluginRouting{
		{Name: "aws-public", Features: []string{"ProjectedCosts"}},
		{Name: "aws-public-legacy"},
		{Name: "aws-publik", Features: []string{"ActualCosts"}},
		{Name: "cloudzero", Features: []string{"ActualCosts"}},
	}}
	clients := []*pluginhost.Client{identityClient("finfocus-plugin-aws-public", "aws-public", "aws-public-legacy")}
	ctx, logs := capturingContext()

	require.NotNil(t, createRouterForEngine(ctx, cfg, clients))

	out := logs.String()
	assert.Contains(t, out, `"plugin":"aws-publik"`)
	assert.Contains(
		t,
		out,
		"routing entry matches no loaded plugin; its feature and pattern restrictions are not applied",
	)
	assert.NotContains(t, out, `"plugin":"aws-public"`)
	assert.NotContains(t, out, `"plugin":"aws-public-legacy"`, "an alias resolves, so it is not unmatched")
	assert.NotContains(t, out, `"plugin":"cloudzero"`,
		"an installed plugin that was not loaded (--adapter narrowed the set) is not a typo")
}

func TestUserNamedPluginChoiceAcceptsInstallName(t *testing.T) {
	t.Parallel()

	prom := capClient("finfocus-plugin-prometheus", pluginhost.CapabilityUsageStats)
	prom.InstallName = "prometheus"
	k8s := capClient("kubernetes", pluginhost.CapabilityUsageStats)
	got, err := selectCapablePlugin([]*pluginhost.Client{k8s, prom}, pluginhost.CapabilityUsageStats, "prometheus")
	require.NoError(t, err)
	assert.Same(t, prom, got, "--usage-source matches the install name")

	scorer := capClient("finfocus-plugin-scorer", pluginhost.CapabilityRecommendationScoring)
	scorer.InstallName = "scorer"
	found, warn := findScorerClient([]*pluginhost.Client{scorer}, "scorer")
	assert.Same(t, scorer, found, "scoring.plugin matches the install name")
	assert.Empty(t, warn)
}

func TestUserNamedPluginChoicePrefersInstallName(t *testing.T) {
	t.Parallel()

	impostorScorer := capClient("scorer", "projected_costs")
	impostorScorer.InstallName = "custom"
	scorer := capClient("renamed-scorer", pluginhost.CapabilityRecommendationScoring)
	scorer.InstallName = "scorer"
	found, warn := findScorerClient([]*pluginhost.Client{impostorScorer, scorer}, "scorer")
	assert.Same(t, scorer, found, "the plugin installed as scorer wins over one that only reports the name")
	assert.Empty(t, warn)

	impostorProm := capClient("prometheus", pluginhost.CapabilityUsageStats)
	impostorProm.InstallName = "custom"
	prom := capClient("finfocus-plugin-prometheus", pluginhost.CapabilityUsageStats)
	prom.InstallName = "prometheus"
	got, err := selectCapablePlugin(
		[]*pluginhost.Client{impostorProm, prom}, pluginhost.CapabilityUsageStats, "prometheus")
	require.NoError(t, err)
	assert.Same(t, prom, got)

	incapable := capClient("finfocus-plugin-prometheus", "projected_costs")
	incapable.InstallName = "prometheus"
	_, err = selectCapablePlugin(
		[]*pluginhost.Client{impostorProm, incapable}, pluginhost.CapabilityUsageStats, "prometheus")
	require.Error(
		t,
		err,
		"the plugin installed as prometheus lacks usage_stats; another plugin reporting the name must not stand in",
	)
	assert.Contains(t, err.Error(), `"prometheus" is not installed or lacks usage_stats`)
}
