package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

const k8sWorkloadsPlan = "../../examples/plans/k8s-workloads-plan.json"

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

// installKubernetesPlugin builds the real kubernetes plugin from
// plugins/kubernetes and installs it into an isolated home.
func installKubernetesPlugin(t *testing.T) {
	t.Helper()
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
}

func setKubernetesRates(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_KUBERNETES_CPU_HOURLY_RATE", "0.04")
	t.Setenv("FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE", "0.005")
}

// unsetKubernetesSettings clears every projected-pricing variable for the
// test, so a developer's own settings cannot change the result.
func unsetKubernetesSettings(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"FINFOCUS_KUBERNETES_CPU_HOURLY_RATE", "FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE",
		"FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT", "FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH",
	} {
		t.Setenv(name, "")
	}
}

type projectedEntry struct {
	ResourceType string  `json:"resourceType"`
	Adapter      string  `json:"adapter"`
	Currency     string  `json:"currency"`
	Monthly      float64 `json:"monthly"`
	Hourly       float64 `json:"hourly"`
	Notes        string  `json:"notes"`
	Error        *struct {
		Code string `json:"code"`
	} `json:"error"`
}

// projectedEntries runs cost projected on the workload fixture and returns its
// entries by the resource name at the end of each URN.
func projectedEntries(t *testing.T) map[string]projectedEntry {
	t.Helper()
	out, err := helpers.NewCLIHelper(t).Execute(
		"cost", "projected", "--pulumi-json", k8sWorkloadsPlan, "--output", "json")
	require.NoError(t, err)
	var parsed struct {
		Finfocus struct {
			Summary struct {
				Resources []struct {
					projectedEntry

					ResourceID string `json:"resourceId"`
				} `json:"resources"`
			} `json:"summary"`
		} `json:"finfocus"`
	}
	require.NoErrorf(t, json.Unmarshal([]byte(out), &parsed), "output: %s", out)
	entries := map[string]projectedEntry{}
	for _, r := range parsed.Finfocus.Summary.Resources {
		entries[r.ResourceID[strings.LastIndex(r.ResourceID, "::")+2:]] = r.projectedEntry
	}
	return entries
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
	unsetKubernetesSettings(t)
	installKubernetesPlugin(t)

	h := helpers.NewCLIHelper(t)
	list := h.ExecuteOrFail("plugin", "list")
	assert.Contains(t, list, "kubernetes")

	out, err := h.Execute(
		"cost",
		"projected",
		"--pulumi-json",
		"../../examples/plans/aws-simple-plan.json",
	)
	require.NoError(t, err)
	assert.NotContains(t, out, "not supported")
	assert.NotContains(t, out, "ERROR:")
	assert.Contains(t, out, "declined by kubernetes",
		"the kubernetes plugin must decline AWS resources via Supports, not price them")

	// The instance's note carries the decline suffix with the plugin installed,
	// as it did before workload pricing, so it is compared with a rate-free run
	// in full and with a plugin-free run on its cost fields.
	t.Run("other resources in a workload plan are unchanged", func(t *testing.T) {
		withoutRates := projectedEntries(t)
		setKubernetesRates(t)
		withRates := projectedEntries(t)

		assert.Contains(
			t,
			withRates["settings"].Notes,
			"declined by kubernetes: kubernetes plugin prices Deployment, StatefulSet, DaemonSet, Job, and CronJob only",
		)
		assert.Equal(t, withoutRates["settings"], withRates["settings"])
		assert.Equal(t, withoutRates["bastion"], withRates["bastion"],
			"pricing workloads must not change the EC2 instance's entry")
		assert.Contains(t, withRates["bastion"].Notes,
			"declined by kubernetes: kubernetes plugin prices only kubernetes:* resources")

		isolatedClusterHome(t)
		noPlugin := projectedEntries(t)["bastion"]
		bastion := withRates["bastion"]
		assert.Equal(t, noPlugin.Adapter, bastion.Adapter)
		assert.Equal(t, noPlugin.Currency, bastion.Currency)
		assert.InDelta(t, noPlugin.Monthly, bastion.Monthly, 1e-9)
		assert.Equal(t, noPlugin.Error, bastion.Error)
	})
}

func TestKubernetesPlugin_PricesDeclaredWorkloads(t *testing.T) {
	unsetKubernetesSettings(t)
	installKubernetesPlugin(t)
	setKubernetesRates(t)

	entries := projectedEntries(t)

	web := entries["web"]
	assert.Equal(t, "kubernetes", web.Adapter)
	assert.Nil(t, web.Error)
	assert.InDelta(t, 54.75, web.Monthly, 1e-9)
	assert.Contains(t, web.Notes, "declared requests × configured rates")
	assert.Contains(t, web.Notes, "3 pods (spec.replicas)")
	assert.Contains(t, web.Notes, "not a real node price")
	assert.InDelta(t, 73.0, entries["db"].Monthly, 1e-9)

	t.Run("hinted kinds", func(t *testing.T) {
		t.Setenv("FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT", "4")
		t.Setenv("FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH", "10")

		hinted := projectedEntries(t)

		for name, monthly := range map[string]float64{"log-agent": 13.505, "migrate": 0.9, "report": 0.225} {
			assert.Nil(t, hinted[name].Error, name)
			assert.Equal(t, "kubernetes", hinted[name].Adapter, name)
			assert.InDelta(t, monthly, hinted[name].Monthly, 1e-9, name)
		}
		assert.Contains(t, hinted["log-agent"].Notes, "FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT")
		assert.Contains(t, hinted["migrate"].Notes, "FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH")
	})
}

//nolint:paralleltest // isolatedClusterHome uses t.Setenv
func TestKubernetesPlugin_ExplainsUnpricedWorkloads(t *testing.T) {
	unsetKubernetesSettings(t)
	installKubernetesPlugin(t)

	entries := projectedEntries(t)

	for _, name := range []string{"web", "db", "log-agent", "migrate", "report"} {
		entry := entries[name]
		require.NotNil(t, entry.Error, "%s must not be presented as a price", name)
		assert.Equal(t, "NO_COST_DATA", entry.Error.Code, name)
		assert.NotEqual(
			t,
			"kubernetes",
			entry.Adapter,
			"%s has no price, so no adapter priced it",
			name,
		)
		assert.Contains(t, entry.Notes, "FINFOCUS_KUBERNETES_CPU_HOURLY_RATE", name)
		assert.Contains(t, entry.Notes, "finfocus cost cluster", name)
	}
}
