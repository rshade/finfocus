package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverviewConfigResolveClusterContext(t *testing.T) {
	t.Parallel()
	cfg := &OverviewConfig{ClusterContexts: map[string]string{
		"prod-cluster": "prod-ctx",
		"urn:pulumi:prod::app::aws:eks/cluster:Cluster::other": "urn-ctx",
	}}

	assert.Equal(t, "prod-ctx", cfg.ResolveClusterContext("prod-cluster", "urn:pulumi:x"))
	assert.Equal(t, "urn-ctx",
		cfg.ResolveClusterContext("other", "urn:pulumi:prod::app::aws:eks/cluster:Cluster::other"),
		"full URN wins over bare name")
	assert.Empty(t, cfg.ResolveClusterContext("missing", "urn:pulumi:y"))

	var nilCfg *OverviewConfig
	assert.Empty(t, nilCfg.ResolveClusterContext("prod-cluster", "urn:pulumi:x"))
}

func TestConfigOverviewSectionMerge(t *testing.T) {
	t.Parallel()
	target := New()
	require.Nil(t, target.Overview)

	err := unmarshalSection(target, keyOverview,
		[]byte(`{"cluster_contexts":{"prod-cluster":"prod-ctx"}}`))
	require.NoError(t, err)
	require.NotNil(t, target.Overview)
	assert.Equal(t, "prod-ctx", target.Overview.ClusterContexts["prod-cluster"])
}

func TestConfigOverviewGetSetList(t *testing.T) {
	t.Parallel()
	cfg := New()

	contexts, err := cfg.Get("overview.cluster_contexts")
	require.NoError(t, err)
	assert.Empty(t, contexts)

	require.NoError(t, cfg.Set("overview.cluster_contexts.prod-cluster", "prod-ctx"))
	require.NoError(t, cfg.Set("overview.cluster_contexts.app.example", "dotted-ctx"))

	got, err := cfg.Get("overview.cluster_contexts.prod-cluster")
	require.NoError(t, err)
	assert.Equal(t, "prod-ctx", got)
	got, err = cfg.Get("overview.cluster_contexts.app.example")
	require.NoError(t, err)
	assert.Equal(t, "dotted-ctx", got)
	assert.Equal(t, cfg.Overview, cfg.List()[keyOverview])

	require.NoError(t, cfg.Set("overview.cluster_contexts.prod-cluster", ""))
	_, err = cfg.Get("overview.cluster_contexts.prod-cluster")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no overview.cluster_contexts entry")

	_, err = cfg.Get("overview.unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown overview setting")

	err = cfg.Set("overview.cluster_contexts", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overview key required")
}

func TestValidateConfigSourceAcceptsOverview(t *testing.T) {
	t.Parallel()
	result, cfg := ValidateConfigSource("config.hujson",
		[]byte(`{"overview":{"cluster_contexts":{"prod-cluster":"prod-ctx"}}}`))
	assert.True(t, result.Valid)
	assert.Empty(t, result.Warnings)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Overview)
	assert.Equal(t, "prod-ctx", cfg.Overview.ClusterContexts["prod-cluster"])
}
