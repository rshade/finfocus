package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

// isolatedClusterHome points FINFOCUS_HOME and HOME at an empty temp tree so
// plugin discovery sees exactly what the test installs and nothing from the
// developer's real ~/.finfocus.
func isolatedClusterHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "plugins"), 0o750))
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("HOME", home)
	return home
}

//nolint:paralleltest // isolatedClusterHome uses t.Setenv
func TestCostCluster_NoPluginsInstalled(t *testing.T) {
	isolatedClusterHome(t)
	h := helpers.NewCLIHelper(t)
	_, err := h.Execute("cost", "cluster")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finfocus plugin install kubernetes")
}

// TestKubernetesPlugin_DoesNotPolluteCostProjected installs the real
// kubernetes plugin (built from plugins/kubernetes) and verifies that
// `cost projected` for AWS resources records no per-resource errors from it:
// the plugin declines Supports and is never called for pricing. The engine's
// deliberate "declined by kubernetes: ..." note documents the skip.
//
//nolint:paralleltest // isolatedClusterHome uses t.Setenv
func TestKubernetesPlugin_DoesNotPolluteCostProjected(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the kubernetes plugin")
	}
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	binName := "finfocus-plugin-kubernetes" + exe
	// Build before isolatedClusterHome overrides HOME: with HOME in a temp dir,
	// go would put GOPATH (and its read-only module cache) there, which is slow
	// and makes t.TempDir cleanup fail with "permission denied".
	built := filepath.Join(t.TempDir(), binName)
	build := exec.Command("go", "-C", "../../plugins/kubernetes", "build", "-o", built, "./cmd")
	buildOut, err := build.CombinedOutput()
	require.NoErrorf(t, err, "build kubernetes plugin: %s", buildOut)

	home := isolatedClusterHome(t)
	dir := filepath.Join(home, "plugins", "kubernetes", "0.1.0")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.Rename(built, filepath.Join(dir, binName)))
	manifest, err := os.ReadFile("../../plugins/kubernetes/plugin.manifest.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), manifest, 0o644))

	h := helpers.NewCLIHelper(t)
	list := h.ExecuteOrFail("plugin", "list")
	assert.Contains(t, list, "kubernetes")

	out, err := h.Execute("cost", "projected", "--pulumi-json", "../../examples/plans/aws-simple-plan.json")
	require.NoError(t, err)
	assert.NotContains(t, out, "not supported")
	assert.NotContains(t, out, "ERROR:")
	assert.Contains(t, out, "declined by kubernetes",
		"the kubernetes plugin must decline AWS resources via Supports, not price them")
}
