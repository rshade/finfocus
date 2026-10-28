//go:build unix

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyPackTestVersion = "v0.0.0-policypack"
	policyPackTimeout     = 3 * time.Minute
	analyzerServingMarker = "analyzer server listening"
	languagePluginEntry   = "pulumi-language-finfocus"
	legacyAnalyzerEntry   = "pulumi-analyzer-policy-finfocus"
	emptyProgramYAML      = "name: policy-pack-smoke\nruntime: yaml\n"
)

// TestAnalyzerPolicyPack_LoadsInPulumi runs `pulumi preview --policy-pack` on a
// no-resource YAML program, using the policy pack `finfocus analyzer install`
// writes, and checks that Pulumi started the finfocus analyzer (#1895).
//
// The test is opt-in because the result depends on the Pulumi version: set
// FINFOCUS_TEST_PULUMI to the absolute path of a pulumi binary (not a mise
// shim, which resolves per directory).
//
//nolint:paralleltest // builds and executes a binary (text file busy, golang/go#22315)
func TestAnalyzerPolicyPack_LoadsInPulumi(t *testing.T) {
	pulumiBin := os.Getenv("FINFOCUS_TEST_PULUMI")
	if pulumiBin == "" {
		t.Skip("set FINFOCUS_TEST_PULUMI to a pulumi binary to run the policy pack check")
	}
	require.True(t, filepath.IsAbs(pulumiBin), "FINFOCUS_TEST_PULUMI must be an absolute path, got %q", pulumiBin)

	binary := buildPolicyPackBinary(t)
	versionOut := pulumiVersionOutput(t, pulumiBin)
	t.Logf("pulumi %s", strings.TrimSpace(versionOut))

	//nolint:paralleltest // executes the shared finfocus binary
	t.Run("language_plugin", func(t *testing.T) {
		run := newPolicyPackRun(t, pulumiBin, binary, true)
		run.writeProgram()
		run.install()
		previewOut := run.preview()
		assert.Contains(t, previewOut, analyzerServingMarker)
		assert.Contains(t, previewOut, "using "+languagePluginEntry)
		assertCostSummary(t, filepath.Join(run.project, ".finfocus"))
		assert.NoFileExists(t, filepath.Join(run.finfocusHome, "last-cost-summary.json"))
	})

	//nolint:paralleltest // executes the shared finfocus binary
	t.Run("default_acquisition", func(t *testing.T) {
		run := newPolicyPackRun(t, pulumiBin, binary, false)
		run.writeProgram()
		run.install()
		previewOut := run.preview()
		assert.Contains(t, previewOut, analyzerServingMarker)
		assert.NotContains(t, previewOut, "pulumi-finfocus")
		assert.NotContains(t, previewOut, "could not find latest version")
	})

	//nolint:paralleltest // executes the shared finfocus binary
	t.Run("legacy_entry_only", func(t *testing.T) {
		skipLegacyOnNewPulumi(t, versionOut)
		run := newPolicyPackRun(t, pulumiBin, binary, true)
		run.writeProgram()
		run.install()
		require.NoError(t, os.Remove(filepath.Join(run.packDir, languagePluginEntry)))
		previewOut := run.preview()
		assert.Contains(t, previewOut, analyzerServingMarker)
		assert.Contains(t, previewOut, "using "+legacyAnalyzerEntry)
	})

	//nolint:paralleltest // writes a plugin binary, then executes it
	t.Run("plugin_cleanup", func(t *testing.T) {
		run := newPolicyPackRun(t, pulumiBin, binary, true)
		run.writeProgram()
		pidFile := filepath.Join(run.work, "pids")
		served := filepath.Join(run.work, "served")
		installOrphanFixture(t, run.finfocusHome)
		run.extraEnv = append(run.extraEnv,
			"ORPHAN_PID_FILE="+pidFile,
			"ORPHAN_SERVED_FILE="+served,
			"FINFOCUS_HISTORY_ENABLED=false",
			"FINFOCUS_CACHE_ENABLED=false",
			"FINFOCUS_SKIP_MIGRATION_CHECK=1",
		)
		run.install()
		previewOut := run.preview()
		assert.Contains(t, previewOut, analyzerServingMarker)
		waitPIDsGone(t, pidFile)
	})
}

func buildPolicyPackBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "finfocus")
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/rshade/finfocus/pkg/version.version="+policyPackTestVersion,
		"-o", binary, "../../cmd/finfocus").CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)
	return binary
}

func pulumiVersionOutput(t *testing.T, pulumiBin string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), policyPackTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, pulumiBin, "version")
	cmd.Env = append(os.Environ(), "PULUMI_SKIP_UPDATE_CHECK=true")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "pulumi version: %s", out)
	return string(out)
}

func skipLegacyOnNewPulumi(t *testing.T, versionOut string) {
	t.Helper()
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(versionOut), "v"))
	require.NotEmpty(t, fields, "pulumi version: %q", versionOut)
	got, err := semver.NewVersion(fields[0])
	require.NoError(t, err, "parse pulumi version %q", versionOut)
	cutoff, err := semver.NewVersion("3.270.0")
	require.NoError(t, err)
	if !got.LessThan(cutoff) {
		t.Skipf("Pulumi %s starts policy packs through the language plugin", got)
	}
}

type policyPackRun struct {
	t                  *testing.T
	pulumiBin          string
	binary             string
	work               string
	finfocusHome       string
	pulumiHome         string
	packDir            string
	project            string
	disableAcquisition bool
	extraEnv           []string
}

func newPolicyPackRun(t *testing.T, pulumiBin, binary string, disableAcquisition bool) *policyPackRun {
	t.Helper()
	work := t.TempDir()
	finfocusHome := filepath.Join(work, "finfocus-home")
	project := filepath.Join(work, "project")
	require.NoError(t, os.MkdirAll(project, 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(work, "state"), 0o750))
	return &policyPackRun{
		t:                  t,
		pulumiBin:          pulumiBin,
		binary:             binary,
		work:               work,
		finfocusHome:       finfocusHome,
		pulumiHome:         filepath.Join(work, "pulumi-home"),
		packDir:            filepath.Join(finfocusHome, "analyzer"),
		project:            project,
		disableAcquisition: disableAcquisition,
	}
}

func (r *policyPackRun) writeProgram() {
	r.t.Helper()
	require.NoError(r.t, os.WriteFile(filepath.Join(r.project, "Pulumi.yaml"), []byte(emptyProgramYAML), 0o600))
}

func (r *policyPackRun) environ() []string {
	env := append(environWithoutPrefixes("FINFOCUS_", "PULUMI_"),
		"FINFOCUS_HOME="+r.finfocusHome,
		"FINFOCUS_LOG_LEVEL=info",
		"PULUMI_HOME="+r.pulumiHome,
		"PULUMI_BACKEND_URL=file://"+filepath.Join(r.work, "state"),
		"PULUMI_CONFIG_PASSPHRASE=",
		"PULUMI_SKIP_UPDATE_CHECK=true",
		"PATH="+strings.Join([]string{r.packDir, filepath.Dir(r.pulumiBin), os.Getenv("PATH")},
			string(os.PathListSeparator)),
	)
	if r.disableAcquisition {
		env = append(env, "PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true")
	}
	return append(env, r.extraEnv...)
}

func (r *policyPackRun) run(dir, name string, args ...string) (string, error) {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(r.t.Context(), policyPackTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = r.environ()
	combined, runErr := cmd.CombinedOutput()
	return string(combined), runErr
}

func (r *policyPackRun) install() {
	r.t.Helper()
	installOut, err := r.run(
		r.work, r.binary, "analyzer", "install", "--target-dir", filepath.Join(r.pulumiHome, "plugins"),
	)
	require.NoError(r.t, err, "finfocus analyzer install: %s", installOut)
	assert.Contains(r.t, installOut, `pulumi preview --policy-pack "`+r.packDir+`"`)
	stackOut, err := r.run(r.project, r.pulumiBin, "stack", "init", "dev", "--non-interactive")
	require.NoError(r.t, err, "pulumi stack init: %s", stackOut)
}

func (r *policyPackRun) preview() string {
	r.t.Helper()
	previewOut, err := r.run(r.project, r.pulumiBin, "preview", "--policy-pack", r.packDir, "--non-interactive")
	require.NoError(r.t, err, "pulumi preview --policy-pack failed:\n%s", previewOut)
	return previewOut
}

func assertCostSummary(t *testing.T, summaryDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(summaryDir, "last-cost-summary.json"))
	require.NoError(t, err)
	var summary struct {
		Stack   string `json:"stack"`
		Project string `json:"project"`
	}
	require.NoError(t, json.Unmarshal(data, &summary))
	assert.Equal(t, "dev", summary.Stack)
	assert.Equal(t, "policy-pack-smoke", summary.Project)
}

func installOrphanFixture(t *testing.T, finfocusHome string) {
	t.Helper()
	pluginDir := filepath.Join(finfocusHome, "plugins", "orphanfixture", "0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o750))
	pluginBinary := "finfocus-plugin-orphanfixture"
	pluginPath := filepath.Join(pluginDir, pluginBinary)
	out, err := exec.Command("go", "build", "-o", pluginPath, "./testdata/orphanplugin").CombinedOutput()
	require.NoError(t, err, "build fixture plugin: %s", out)
	manifest, err := json.Marshal(map[string]any{
		"name":                "orphanfixture",
		"version":             "0.1.0",
		"description":         "router-like fixture",
		"author":              "FinFocus tests",
		"supported_providers": []string{"aws"},
		"protocols":           []string{"grpc"},
		"binary":              pluginBinary,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.manifest.json"), manifest, 0o600))
}

func waitPIDsGone(t *testing.T, pidFile string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			last = string(data)
			if pidsGone(t, last) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("plugin processes still alive after 10s: %q", last)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func pidsGone(t *testing.T, text string) bool {
	t.Helper()
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		if processAlive(t, pid) {
			return false
		}
	}
	return true
}

func environWithoutPrefixes(prefixes ...string) []string {
	var kept []string
	for _, kv := range os.Environ() {
		drop := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(kv, prefix) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, kv)
		}
	}
	return kept
}
