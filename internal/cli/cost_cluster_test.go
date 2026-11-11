package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

	_, _, err := selectUsageSource([]*pluginhost.Client{aws}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finfocus plugin install kubernetes")

	got, name, err := selectUsageSource([]*pluginhost.Client{aws, k8s}, "")
	require.NoError(t, err)
	_, isSet := got.(*engine.UsageSourceSet)
	assert.False(t, isSet, "a single source is used directly")
	assert.Equal(t, "kubernetes", name)

	got, name, err = selectUsageSource([]*pluginhost.Client{prom, aws, k8s}, "")
	require.NoError(t, err)
	_, isSet = got.(*engine.UsageSourceSet)
	assert.True(t, isSet, "several sources are asked by mode")
	assert.Empty(t, name, "a set has no single name")

	got, name, err = selectUsageSource([]*pluginhost.Client{prom, k8s}, "prometheus")
	require.NoError(t, err)
	_, isSet = got.(*engine.UsageSourceSet)
	assert.False(t, isSet, "--usage-source wins over mode selection")
	assert.Equal(t, "prometheus", name)

	_, _, err = selectUsageSource([]*pluginhost.Client{prom, k8s}, "aws-public")
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

// A policy file that is not valid HuJSON, whether named by --policy or
// discovered, exits 2 before any plugin starts, names the file and the parse
// position, and talks about the allocation policy rather than config.hujson.
//
//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_PolicyParseErrorNamesFile(t *testing.T) {
	truncated := []byte(`{ "idle": `)
	writeFile := func(t *testing.T, dir string) string {
		t.Helper()
		require.NoError(t, os.MkdirAll(dir, 0o750))
		p := filepath.Join(dir, config.AllocationPolicyFile)
		require.NoError(t, os.WriteFile(p, truncated, 0o600))
		return p
	}
	tests := []struct {
		name string
		// setup writes the policy and returns its path and the extra args.
		setup func(t *testing.T) (string, []string)
	}{
		{"--policy", func(t *testing.T) (string, []string) {
			t.Helper()
			p := writeFile(t, t.TempDir())
			return p, []string{"--policy", p}
		}},
		{"--policy with --show-policy", func(t *testing.T) (string, []string) {
			t.Helper()
			p := writeFile(t, t.TempDir())
			return p, []string{"--policy", p, "--show-policy"}
		}},
		{"discovered project file", func(t *testing.T) (string, []string) {
			t.Helper()
			project := t.TempDir()
			return writeFile(t, filepath.Join(project, ".finfocus")), []string{"--project-dir", project}
		}},
		{"discovered global file", func(t *testing.T) (string, []string) {
			t.Helper()
			return writeFile(t, os.Getenv("FINFOCUS_HOME")), nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateClusterTestConfig(t)
			path, extra := tt.setup(t)
			args := append([]string{"cost", "cluster", "--usage-source", "kubernetes"}, extra...)

			res := axtest.Run(context.Background(), t, NewRootCmd("test"), args)

			require.Equal(t, ax.ExitValidation, res.ExitCode, "%s", res.Stderr)
			envelope := decodeErrorEnvelope(t, res.Stderr)
			assert.Equal(t, "config_invalid", envelope.ErrorCode)
			assert.Equal(t,
				"allocation policy "+path+" is not valid HuJSON: line 1, column 11: parsing value: unexpected EOF",
				envelope.Message)
			assert.Equal(t,
				"fix the HuJSON syntax at line 1, column 11 of the allocation policy "+path+" and retry",
				envelope.ActionableFix)
			assert.Equal(t, path, envelope.Context["path"])
		})
	}
}

//nolint:paralleltest // builds a root command, which sets the process-wide resolved project dir
func TestCostCluster_UnreadablePolicyNamesFile(t *testing.T) {
	isolateClusterTestConfig(t)
	dir := t.TempDir()

	res := axtest.Run(context.Background(), t, NewRootCmd("test"),
		[]string{"cost", "cluster", "--policy", dir, "--show-policy"})

	require.Equal(t, ax.ExitValidation, res.ExitCode, "%s", res.Stderr)
	envelope := decodeErrorEnvelope(t, res.Stderr)
	assert.Equal(t, "validation_error", envelope.ErrorCode)
	assert.True(t, strings.HasPrefix(envelope.Message, "cannot read allocation policy "+dir+": "), envelope.Message)
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
// both. Each plugin records the modes it was asked for, and the allocator its
// Allocate calls. The fixture returns nothing to price, so a selected run
// allocates nothing and exits 0. It stays sequential: it builds a plugin
// binary, sets FINFOCUS_HOME, and builds a root command.
func TestCostCluster_UsageSourceSelection(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a usage-source plugin binary")
	}
	installUsageSourcePlugins(t)

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
			wantAsked:   map[string]string{"kubernetes": "run-rate\nallocate\n", "prometheus": "run-rate\n"},
		},
		{
			name:        "window uses the historical source",
			args:        window,
			modeLimited: true,
			wantAsked:   map[string]string{"kubernetes": "historical\nallocate\n", "prometheus": "historical\n"},
		},
		{
			name:        "explicit usage source wins",
			args:        []string{"--usage-source", "prometheus"},
			modeLimited: true,
			wantCode:    ax.ExitValidation,
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

			assert.Equal(t, tt.wantAsked, readAskedMarkers(t, markers))
		})
	}
}

// installUsageSourcePlugins builds testdata/usagesource and installs it as
// kubernetes (also the allocator) and prometheus under a fresh FINFOCUS_HOME.
func installUsageSourcePlugins(t *testing.T) {
	t.Helper()
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
}

// readAskedMarkers returns what each plugin recorded, keyed by plugin name.
func readAskedMarkers(t *testing.T, markers string) map[string]string {
	t.Helper()
	asked := map[string]string{}
	entries, err := os.ReadDir(markers)
	require.NoError(t, err)
	for _, e := range entries {
		data, readErr := os.ReadFile(filepath.Join(markers, e.Name()))
		require.NoError(t, readErr)
		asked[e.Name()] = string(data)
	}
	return asked
}

// TestCostCluster_PluginRejectionsOfUserInput pins which plugin failures exit
// 2. A plugin name the flags cannot resolve, a GetStats InvalidArgument (the
// request holds only --context, --namespace, --selector, and the window), and
// an allocator InvalidArgument for a policy sent with no usage are the user's
// to fix. Other codes, and an allocator failure on real usage, stay exit 1.
// A policy file is checked before GetStats, so a bad one never reaches the
// usage source or pricing. kubernetes serves run-rate only and prometheus
// historical only. It stays sequential for the same reasons as
// TestCostCluster_UsageSourceSelection.
func TestCostCluster_PluginRejectionsOfUserInput(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a usage-source plugin binary")
	}
	installUsageSourcePlugins(t)
	t.Setenv("FINFOCUS_TEST_RUN_RATE_ONLY", "kubernetes")
	t.Setenv("FINFOCUS_TEST_HISTORICAL_ONLY", "prometheus")

	policyDir := t.TempDir()
	writePolicy := func(name, body string) string {
		path := filepath.Join(policyDir, name)
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		return path
	}
	unknownField := writePolicy("unknown.hujson", `{ "idle": "share", "bogus": 1 }`)
	badValue := writePolicy("value.hujson", `{ "idle": "sometimes" }`)
	good := writePolicy("good.hujson", `{ "idle": "separate" }`)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	window := []string{
		"--from", today.AddDate(0, 0, -14).Format(time.DateOnly),
		"--to", today.AddDate(0, 0, -7).Format(time.DateOnly),
	}
	tests := []struct {
		name      string
		args      []string
		env       map[string]string
		wantCode  int
		wantError string // error_code of the JSON error envelope
		wantMsg   string
		wantAsked map[string]string
	}{
		{
			name:      "unknown usage source",
			args:      []string{"--usage-source", "nosuchplugin"},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg: `plugin "nosuchplugin" is not installed or lacks usage_stats ` +
				`(available: kubernetes, prometheus)`,
			wantAsked: map[string]string{},
		},
		{
			name:      "unknown allocator",
			args:      []string{"--allocator", "nosuchplugin"},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg:   `plugin "nosuchplugin" is not installed or lacks allocation (available: kubernetes)`,
			wantAsked: map[string]string{},
		},
		{
			name:      "window sent to a run-rate source",
			args:      append([]string{"--usage-source", "kubernetes"}, window...),
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg:   "usage source kubernetes rejected the request: kubernetes usage source is run-rate only",
			wantAsked: map[string]string{"kubernetes": "historical\n"},
		},
		{
			name:      "run-rate sent to a historical source",
			args:      []string{"--usage-source", "prometheus"},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg:   "usage source prometheus rejected the request: prometheus usage source is historical only",
			wantAsked: map[string]string{"prometheus": "run-rate\n"},
		},
		{
			name:      "unknown context with one source",
			args:      []string{"--usage-source", "kubernetes", "--context", "no-such-context"},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg:   `usage source kubernetes rejected the request: context "no-such-context" not found`,
			wantAsked: map[string]string{"kubernetes": "run-rate\n"},
		},
		{
			name:      "unknown context rejected by every source",
			args:      []string{"--context", "no-such-context"},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg: `no usage source accepted the run-rate request: ` +
				`kubernetes: context "no-such-context" not found; prometheus: context "no-such-context" not found`,
			wantAsked: map[string]string{"kubernetes": "run-rate\n", "prometheus": "run-rate\n"},
		},
		{
			name:      "policy with an unknown field fails before usage and pricing",
			args:      []string{"--usage-source", "kubernetes", "--policy", unknownField},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg: "allocator kubernetes rejected the allocation policy " + unknownField +
				`: allocation policy: unknown field "bogus"`,
			wantAsked: map[string]string{"kubernetes": "allocate\n"},
		},
		{
			name:      "policy with an unsupported value",
			args:      []string{"--policy", badValue},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg: "allocator kubernetes rejected the allocation policy " + badValue +
				`: idle: unsupported value "sometimes"`,
			wantAsked: map[string]string{"kubernetes": "allocate\n"},
		},
		{
			name:      "show-policy with a bad policy",
			args:      []string{"--show-policy", "--policy", unknownField},
			wantCode:  ax.ExitValidation,
			wantError: "validation_error",
			wantMsg: "allocator kubernetes rejected the allocation policy " + unknownField +
				`: allocation policy: unknown field "bogus"`,
			wantAsked: map[string]string{"kubernetes": "allocate\n"},
		},
		{
			name:      "a valid policy is checked before usage is gathered",
			args:      []string{"--usage-source", "kubernetes", "--policy", good},
			wantAsked: map[string]string{"kubernetes": "allocate\nrun-rate\nallocate\n"},
		},
		{
			name:      "usage source failure stays internal",
			args:      []string{"--usage-source", "kubernetes"},
			env:       map[string]string{"FINFOCUS_TEST_GETSTATS_UNAVAILABLE": "kubernetes"},
			wantCode:  1,
			wantError: "internal_error",
			wantMsg:   "get usage stats: rpc error: code = Unavailable desc = cluster unreachable",
			wantAsked: map[string]string{"kubernetes": "run-rate\n"},
		},
		{
			name:      "allocator failure on the policy check stays internal",
			args:      []string{"--policy", good},
			env:       map[string]string{"FINFOCUS_TEST_ALLOCATE_INTERNAL": "kubernetes"},
			wantCode:  1,
			wantError: "internal_error",
			wantMsg:   "allocate: rpc error: code = Internal desc = allocator crashed",
			wantAsked: map[string]string{"kubernetes": "allocate\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := t.TempDir()
			t.Setenv("FINFOCUS_TEST_GETSTATS_MARKER_DIR", markers)
			t.Setenv("FINFOCUS_TEST_GETSTATS_UNAVAILABLE", "")
			t.Setenv("FINFOCUS_TEST_ALLOCATE_INTERNAL", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			args := append([]string{"cost", "cluster"}, tt.args...)
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
			require.Equal(t, tt.wantCode, res.ExitCode, "stderr: %s", res.Stderr)
			if tt.wantCode != 0 {
				code, msg := errorEnvelope(t, res.Stderr)
				assert.Equal(t, tt.wantError, code)
				assert.Equal(t, tt.wantMsg, msg)
			}
			assert.Equal(t, tt.wantAsked, readAskedMarkers(t, markers))
		})
	}
}

// errorEnvelope returns the error_code and message of the JSON error envelope
// that ax.Execute writes to stderr when the output is not a terminal.
func errorEnvelope(t *testing.T, stderr []byte) (string, string) {
	t.Helper()
	envelope := decodeErrorEnvelope(t, stderr)
	return envelope.ErrorCode, envelope.Message
}

// decodeErrorEnvelope returns the whole JSON error envelope on stderr.
func decodeErrorEnvelope(t *testing.T, stderr []byte) ax.Error {
	t.Helper()
	for line := range strings.Lines(string(stderr)) {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var envelope ax.Error
		require.NoError(t, json.Unmarshal([]byte(line), &envelope))
		return envelope
	}
	require.Failf(t, "no error envelope", "stderr: %s", stderr)
	return ax.Error{}
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
