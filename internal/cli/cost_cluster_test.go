package cli

import (
	"context"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// isolateClusterTestConfig mirrors the cli_test package's isolateConfig for
// internal (package cli) tests: config resolution must never touch the
// developer's real ~/.finfocus.
func isolateClusterTestConfig(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
}

func capClient(name string, caps ...string) *pluginhost.Client {
	return &pluginhost.Client{Name: name, Metadata: &proto.PluginMetadata{Capabilities: caps}}
}

func TestSelectCapablePlugin(t *testing.T) {
	t.Parallel()

	k8s := capClient("kubernetes", pluginhost.CapabilityUsageStats, pluginhost.CapabilityAllocation)
	prom := capClient("prometheus", pluginhost.CapabilityUsageStats)
	aws := capClient("aws-public", "projected_costs")

	got, err := selectCapablePlugin([]*pluginhost.Client{aws, k8s}, pluginhost.CapabilityUsageStats, "")
	require.NoError(t, err)
	assert.Equal(t, "kubernetes", got.Name)

	_, err = selectCapablePlugin([]*pluginhost.Client{aws}, pluginhost.CapabilityUsageStats, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finfocus plugin install kubernetes")

	_, err = selectCapablePlugin([]*pluginhost.Client{k8s, prom}, pluginhost.CapabilityUsageStats, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes, prometheus")
	assert.Contains(t, err.Error(), "--usage-source")

	got, err = selectCapablePlugin([]*pluginhost.Client{k8s, prom}, pluginhost.CapabilityUsageStats, "prometheus")
	require.NoError(t, err)
	assert.Equal(t, "prometheus", got.Name)

	_, err = selectCapablePlugin([]*pluginhost.Client{k8s}, pluginhost.CapabilityUsageStats, "aws-public")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"aws-public" is not installed or lacks usage_stats`)
}

//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_ValidatesFlagsBeforeLoadingPlugins(t *testing.T) {
	isolateClusterTestConfig(t)
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"cost", "cluster", "--output", "xml"}, "unsupported output format: xml"},
		{[]string{"cost", "cluster", "--group-by", "daily"}, "invalid --group-by"},
		{[]string{"cost", "cluster", "--selector", "novalue"}, "invalid --selector"},
		{[]string{"cost", "cluster", "--policy", "/nonexistent/p.hujson"}, "/nonexistent/p.hujson"},
	}
	for _, tt := range tests {
		t.Run(tt.wantErr, func(t *testing.T) {
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), tt.args)
			require.NotEqual(t, 0, res.ExitCode)
			assert.Contains(t, string(res.Stderr), tt.wantErr)
		})
	}
}

//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_NoPluginsInstalled(t *testing.T) {
	isolateClusterTestConfig(t)
	res := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"cost", "cluster"})
	require.Equal(t, 1, res.ExitCode)
	assert.Contains(t, string(res.Stderr), "usage_stats")
	assert.Contains(t, string(res.Stderr), "finfocus plugin install kubernetes")
}

// --show-policy resolves only the allocator: it never requires (or contacts) a
// usage source, so the missing-capability error names allocation, not
// usage_stats. The positive path (an allocator installed, no cluster
// reachable) is covered by the kind E2E.
//
//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_ShowPolicyNeedsOnlyAllocator(t *testing.T) {
	isolateClusterTestConfig(t)
	res := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"cost", "cluster", "--show-policy"})
	require.Equal(t, 1, res.ExitCode)
	assert.Contains(t, string(res.Stderr), "allocation")
	assert.NotContains(t, string(res.Stderr), "usage_stats")
}
