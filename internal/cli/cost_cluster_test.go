package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
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

func TestSelectUsageSource(t *testing.T) {
	t.Parallel()

	k8s := capClient("kubernetes", pluginhost.CapabilityUsageStats, pluginhost.CapabilityAllocation)
	prom := capClient("prometheus", pluginhost.CapabilityUsageStats)
	aws := capClient("aws-public", "projected_costs")

	_, err := selectUsageSource([]*pluginhost.Client{aws}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finfocus plugin install kubernetes")

	got, err := selectUsageSource([]*pluginhost.Client{aws, k8s}, "")
	require.NoError(t, err)
	_, isSet := got.(*engine.UsageSourceSet)
	assert.False(t, isSet, "a single source is used directly")

	got, err = selectUsageSource([]*pluginhost.Client{prom, aws, k8s}, "")
	require.NoError(t, err)
	_, isSet = got.(*engine.UsageSourceSet)
	assert.True(t, isSet, "several sources are asked by mode")

	got, err = selectUsageSource([]*pluginhost.Client{prom, k8s}, "prometheus")
	require.NoError(t, err)
	_, isSet = got.(*engine.UsageSourceSet)
	assert.False(t, isSet, "--usage-source wins over mode selection")

	_, err = selectUsageSource([]*pluginhost.Client{prom, k8s}, "aws-public")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"aws-public" is not installed or lacks usage_stats`)
}

//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_ValidatesFlagsBeforeLoadingPlugins(t *testing.T) {
	isolateClusterTestConfig(t)
	malformed := filepath.Join(t.TempDir(), "policy.hujson")
	require.NoError(t, os.WriteFile(malformed, []byte("{ not json"), 0o600))
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"cost", "cluster", "--output", "xml"}, "unsupported output format: xml"},
		{[]string{"cost", "cluster", "--group-by", "daily"}, "invalid --group-by"},
		{[]string{"cost", "cluster", "--selector", "novalue"}, "invalid --selector"},
		{[]string{"cost", "cluster", "--policy", "/nonexistent/p.hujson"}, "/nonexistent/p.hujson"},
		{[]string{"cost", "cluster", "--policy", malformed}, "config_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.wantErr, func(t *testing.T) {
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), tt.args)
			require.Equal(t, ax.ExitValidation, res.ExitCode, "%s", res.Stderr)
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

	today := time.Now().UTC().Truncate(24 * time.Hour)
	day := today.AddDate(0, 0, -14).Format(time.DateOnly)
	later := today.AddDate(0, 0, -7).Format(time.DateOnly)
	future := today.AddDate(0, 0, 30).Format(time.DateOnly)
	tooOld := today.AddDate(-maxPastYears-1, 0, 0)
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
			name: "older than max past",
			args: []string{
				"cost", "cluster",
				"--from", tooOld.Format(time.DateOnly), "--to", tooOld.AddDate(0, 1, 0).Format(time.DateOnly),
			},
			wantErr: "date too far in past",
		},
		{
			name:    "unparseable date",
			args:    []string{"cost", "cluster", "--from", "notadate"},
			wantErr: "unable to parse date",
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
			// Input errors exit 2. A valid window reaches plugin discovery,
			// which fails with no usage_stats plugin installed (exit 1).
			wantCode := ax.ExitValidation
			if tt.reachedPlugins {
				wantCode = 1
			}
			require.Equal(t, wantCode, res.ExitCode, "%s", stderr)
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

// TestCostCluster_UsageSourceSelection installs two usage_stats plugins,
// kubernetes (also the allocator) and prometheus, built from
// testdata/usagesource. Without --usage-source, the one source that accepts
// the request's mode is used; when both accept, the run exits 2 and names
// both. Each plugin records the modes it was asked for. The fixture returns
// nothing to price, so a selected run still exits 1 after GetStats. It stays
// sequential: it builds a plugin binary, sets FINFOCUS_HOME, and builds a
// root command.
func TestCostCluster_UsageSourceSelection(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a usage-source plugin binary")
	}
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
		file := "finfocus-plugin-" + name
		if runtime.GOOS == "windows" {
			file += ".exe" // discovery looks for the .exe on Windows
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), data, 0o755))
	}
	t.Setenv("FINFOCUS_TEST_ALLOCATOR", "kubernetes")

	today := time.Now().UTC().Truncate(24 * time.Hour)
	window := []string{
		"--from", today.AddDate(0, 0, -14).Format(time.DateOnly),
		"--to", today.AddDate(0, 0, -7).Format(time.DateOnly),
	}
	tests := []struct {
		name        string
		args        []string
		modeLimited bool
		wantCode    int
		wantStderr  []string
		wantAsked   map[string]string
	}{
		{
			name:        "run-rate uses the run-rate source",
			modeLimited: true,
			wantCode:    1,
			wantAsked:   map[string]string{"kubernetes": "run-rate\n", "prometheus": "run-rate\n"},
		},
		{
			name:        "window uses the historical source",
			args:        window,
			modeLimited: true,
			wantCode:    1,
			wantAsked:   map[string]string{"kubernetes": "historical\n", "prometheus": "historical\n"},
		},
		{
			name:        "explicit usage source wins",
			args:        []string{"--usage-source", "prometheus"},
			modeLimited: true,
			wantCode:    1,
			wantStderr:  []string{"prometheus usage source is historical only"},
			wantAsked:   map[string]string{"prometheus": "run-rate\n"},
		},
		{
			name:     "several accepting sources need --usage-source",
			wantCode: ax.ExitValidation,
			wantStderr: []string{
				"several usage sources can serve run-rate usage (kubernetes, prometheus)",
				"choose one with --usage-source",
			},
			wantAsked: map[string]string{"kubernetes": "run-rate\n", "prometheus": "run-rate\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := t.TempDir()
			t.Setenv("FINFOCUS_TEST_GETSTATS_MARKER_DIR", markers)
			runRateOnly, historicalOnly := "", ""
			if tt.modeLimited {
				runRateOnly, historicalOnly = "kubernetes", "prometheus"
			}
			t.Setenv("FINFOCUS_TEST_RUN_RATE_ONLY", runRateOnly)
			t.Setenv("FINFOCUS_TEST_HISTORICAL_ONLY", historicalOnly)

			args := append([]string{"cost", "cluster"}, tt.args...)
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
			stderr := string(res.Stderr)
			require.Equal(t, tt.wantCode, res.ExitCode, "stderr: %s", stderr)
			for _, want := range tt.wantStderr {
				assert.Contains(t, stderr, want)
			}
			if tt.wantCode != ax.ExitValidation {
				assert.NotContains(t, stderr, "several usage sources")
			}

			asked := map[string]string{}
			entries, readErr := os.ReadDir(markers)
			require.NoError(t, readErr)
			for _, e := range entries {
				data, fileErr := os.ReadFile(filepath.Join(markers, e.Name()))
				require.NoError(t, fileErr)
				asked[e.Name()] = string(data)
			}
			assert.Equal(t, tt.wantAsked, asked)
		})
	}
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
