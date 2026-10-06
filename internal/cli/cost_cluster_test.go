package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_Window(t *testing.T) {
	isolateClusterTestConfig(t)

	const (
		day    = "2026-09-01"
		later  = "2026-09-08"
		future = "2026-11-01"
		tooOld = "2020-01-01"
	)
	reachedPlugins := "usage_stats"

	tests := []struct {
		name           string
		args           []string
		wantErr        string
		reachedPlugins bool
	}{
		{
			name:           "date-only window",
			args:           []string{"cost", "cluster", "--from", day, "--to", later},
			reachedPlugins: true,
		},
		{
			name:           "rfc3339 window",
			args:           []string{"cost", "cluster", "--from", day + "T00:00:00Z", "--to", later + "T00:00:00Z"},
			reachedPlugins: true,
		},
		{
			name:           "to defaults to now",
			args:           []string{"cost", "cluster", "--from", day},
			reachedPlugins: true,
		},
		{
			name:    "from required when to is set",
			args:    []string{"cost", "cluster", "--to", later},
			wantErr: "--from is required when --to is set",
		},
		{
			name:    "same-day date-only pair",
			args:    []string{"cost", "cluster", "--from", day, "--to", day},
			wantErr: "'to' date must be after 'from' date",
		},
		{
			name:    "reversed range",
			args:    []string{"cost", "cluster", "--from", later, "--to", day},
			wantErr: "'to' date must be after 'from' date",
		},
		{
			name:    "future time",
			args:    []string{"cost", "cluster", "--from", day, "--to", future},
			wantErr: "date cannot be in the future",
		},
		{
			name:    "older than max past",
			args:    []string{"cost", "cluster", "--from", tooOld, "--to", "2020-02-01"},
			wantErr: "date too far in past",
		},
		{
			name:    "output still validated before plugins",
			args:    []string{"cost", "cluster", "--output", "xml", "--from", day, "--to", later},
			wantErr: "unsupported output format: xml",
		},
		{
			name:    "group-by still validated before plugins",
			args:    []string{"cost", "cluster", "--group-by", "daily", "--from", day, "--to", later},
			wantErr: "invalid --group-by",
		},
	}
	for _, tt := range tests {
		//nolint:paralleltest // NewRootCmd sets the process-wide resolved project directory
		t.Run(tt.name, func(t *testing.T) {
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), tt.args)
			stderr := string(res.Stderr)
			require.Equal(t, 1, res.ExitCode)
			if tt.reachedPlugins {
				assert.Contains(t, stderr, reachedPlugins)
				assert.NotContains(t, stderr, "parsing")
				assert.NotContains(t, stderr, "must be after")
				return
			}
			assert.Contains(t, stderr, tt.wantErr)
			assert.NotContains(t, stderr, reachedPlugins)
		})
	}
}

// TestCostCluster_WindowKeepsUsageSourceSelection installs two usage_stats
// plugins and asks for a window without --usage-source. Selection still
// fails closed and lists both candidates. The plugins record a marker if
// GetStats runs. It stays sequential: it builds a plugin binary and sets
// FINFOCUS_HOME.
func TestCostCluster_WindowKeepsUsageSourceSelection(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "usage-source")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("testdata", "usagesource")
	out, err := build.CombinedOutput()
	require.NoErrorf(t, err, "build usage source: %s", out)

	isolateClusterTestConfig(t)
	home := os.Getenv("FINFOCUS_HOME")
	for _, name := range []string{"kubernetes", "prometheus"} {
		dir := filepath.Join(home, "plugins", name, "0.1.0")
		require.NoError(t, os.MkdirAll(dir, 0o750))
		data, readErr := os.ReadFile(bin)
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "finfocus-plugin-"+name), data, 0o755))
	}

	marker := filepath.Join(t.TempDir(), "getstats")
	t.Setenv("FINFOCUS_TEST_GETSTATS_MARKER", marker)

	res := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{
		"cost", "cluster", "--from", "2026-09-01", "--to", "2026-09-08",
	})
	require.Equal(t, 1, res.ExitCode, "stderr: %s", res.Stderr)
	stderr := string(res.Stderr)
	assert.Contains(t, stderr, "kubernetes, prometheus")
	assert.Contains(t, stderr, "--usage-source")
	assert.NotContains(t, stderr, "run-rate only")
	assert.NotContains(t, stderr, "historical only")
	assert.NotContains(t, stderr, "GetStats must not be called")
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestCostCluster_NoWindowLeavesBoundsUnset(t *testing.T) {
	t.Parallel()

	cmd := NewCostClusterCmd()
	from := cmd.Flags().Lookup("from")
	to := cmd.Flags().Lookup("to")
	require.NotNil(t, from)
	require.NotNil(t, to)
	assert.Empty(t, from.DefValue)
	assert.Empty(t, to.DefValue)
}

//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_WindowHelpNamesDateForms(t *testing.T) {
	isolateClusterTestConfig(t)
	res := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"cost", "cluster", "--help"})
	require.Equal(t, 0, res.ExitCode)
	help := string(res.Stdout)
	assert.Contains(t, help, "2006-01-02")
	assert.Contains(t, help, "RFC3339")
}
