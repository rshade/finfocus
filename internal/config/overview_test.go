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
