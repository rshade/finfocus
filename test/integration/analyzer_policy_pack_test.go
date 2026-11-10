//go:build unix

package integration_test

import (
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
	pulumiBin := policyPackPulumi(t)

	binary := buildPolicyPackBinary(t)
	versionOut := pulumiVersionOutput(t, pulumiBin)
	t.Logf("pulumi %s", strings.TrimSpace(versionOut))

	//nolint:paralleltest // executes the shared finfocus binary
	t.Run("language_plugin", func(t *testing.T) {
		run := newPolicyPackRun(t, pulumiBin, binary, true)
		run.writeProgram()
		run.writeProjectConfig(`{"cost": {"history": {"directory": ".finfocus/history"}}}`)
		run.install()
		previewOut := run.preview()
		assert.Contains(t, previewOut, analyzerServingMarker)
		assert.Contains(t, previewOut, "using "+languagePluginEntry)
		assert.FileExists(t, filepath.Join(run.project, ".finfocus", "history", "history.db"),
			"a relative project history directory resolves against the project")
		assert.NoDirExists(t, filepath.Join(run.finfocusHome, "history"))
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
