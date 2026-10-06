//go:build e2e_kind

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixedTotal  = "120"
	fixedAmount = 120.0
	gib         = 1073741824.0
)

// TestCostCluster_KindHistorical prices one kind node from a remote-written
// Prometheus window. The unreachable-Prometheus check lives in this function
// so the Makefile regex covers it. The test calls the built finfocus binary.
//
//nolint:paralleltest // executes built plugins, kubectl, and a real finfocus binary
func TestCostCluster_KindHistorical(t *testing.T) {
	from, to := histBounds()
	home := installIsolatedPlugins(t)
	assertUnreachablePrometheus(t, home, from, to)

	kubectl(t, "apply", "-f", "kind/prometheus.yaml")
	kubectl(t, "-n", "monitoring", "rollout", "status", "deployment/prometheus", "--timeout=180s")
	base := forwardPrometheus(t)
	waitPrometheusReady(t, base)

	node := kindNodeName(t)
	writeHistoricalSeries(t, base, node, from, to)
	assertFixtureHours(t, base, to)
	assertHistoricalCost(t, home, base, node, from, to)
}

// histBounds is a whole UTC day three days back, inside Prometheus's default
// 15-day retention whenever the test runs.
func histBounds() (time.Time, time.Time) {
	from := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -3)
	return from, from.Add(24 * time.Hour)
}

// gaugeHours is the integral of a constant gauge over the 24h window.
// A 60s subquery that includes both endpoints has 1441 points.
func gaugeHours(gauge float64) float64 {
	const steps = float64(24*60 + 1)
	return gauge * steps * 60 / 3600
}

func installIsolatedPlugins(t *testing.T) string {
	t.Helper()
	kubernetesBin := buildPlugin(t, "../../plugins/kubernetes", "finfocus-plugin-kubernetes")
	prometheusBin := buildPlugin(t, "../../plugins/prometheus", "finfocus-plugin-prometheus")
	fixedBin := buildPlugin(t, "kind/fixedcost", "finfocus-plugin-fixedcost")
	home := t.TempDir()
	installBuilt(t, home, "kubernetes", kubernetesBin, "../../plugins/kubernetes/plugin.manifest.json")
	installBuilt(t, home, "prometheus", prometheusBin, "../../plugins/prometheus/plugin.manifest.json")
	installBuilt(t, home, "fixedcost", fixedBin, "kind/fixedcost/plugin.manifest.json")
	return home
}

func buildPlugin(t *testing.T, module, name string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "-C", module, "build", "-o", out, "./cmd")
	built, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "build %s: %s", module, built)
	return out
}

func installBuilt(t *testing.T, home, name, binary, manifestPath string) {
	t.Helper()
	dir := filepath.Join(home, "plugins", name, "0.1.0")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	contents, err := os.ReadFile(binary)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.Base(binary)), contents, 0o755))
	manifest, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), manifest, 0o644))
}

func assertUnreachablePrometheus(t *testing.T, home string, from, to time.Time) {
	t.Helper()
	stdout, _, err := runHistorical(t, home, "http://127.0.0.1:1", from, to)
	require.Error(t, err)
	assert.NotContains(t, string(stdout), `"mode"`)
}

func assertFixtureHours(t *testing.T, base string, at time.Time) {
	t.Helper()
	requireInstant(t, base, cpuUsageQuery(), at, gaugeHours(1))
	requireInstant(t, base, memoryUsageQuery(), at, gaugeHours(1))
	requireInstant(t, base, cpuAllocQuery(), at, gaugeHours(4))
	requireInstant(t, base, memoryAllocQuery(), at, gaugeHours(16))
}

func assertHistoricalCost(t *testing.T, home, base, node string, from, to time.Time) {
	t.Helper()
	stdout, stderr, err := runHistorical(t, home, base, from, to)
	require.NoErrorf(t, err, "stdout: %s\nstderr: %s", stdout, stderr)
	var res histClusterJSON
	require.NoError(t, json.Unmarshal(stdout, &res), string(stdout))
	assert.Equal(t, "historical", res.Mode)
	assert.Equal(t, from.Format(time.DateOnly)+" to "+to.Format(time.DateOnly)+", 1 day", res.Period)
	assert.Equal(t, "USD", res.Currency)
	assert.False(t, res.Incomplete, "warnings: %v\nstderr: %s", res.Warnings, stderr)
	assert.InDelta(t, fixedAmount, res.Total, 1e-6)
	var groupSum float64
	groups := map[string]bool{}
	for _, group := range res.Groups {
		groupSum += group.TotalCost
		groups[group.Key] = true
	}
	assert.InDelta(t, fixedAmount, groupSum, 1e-6)
	assert.True(t, groups["e2e"], "groups: %v", groups)
	assert.True(t, groups["__idle__"], "groups: %v", groups)
	var nodes int
	for _, priced := range res.Priced {
		if priced.Kind != "node" {
			continue
		}
		nodes++
		assert.True(t, priced.Priced, priced.ID)
		assert.Equal(t, node, priced.ID)
		assert.InDelta(t, fixedAmount, priced.Monthly, 1e-6)
	}
	assert.Equal(t, 1, nodes)
	for _, warning := range res.Warnings {
		assert.False(t, strings.HasPrefix(warning, "incomplete:"), warning)
	}
}

type histClusterJSON struct {
	Mode       string   `json:"mode"`
	Period     string   `json:"period"`
	Currency   string   `json:"currency"`
	Total      float64  `json:"total"`
	Incomplete bool     `json:"incomplete"`
	Warnings   []string `json:"warnings"`
	Groups     []struct {
		Key       string  `json:"key"`
		TotalCost float64 `json:"total_cost"`
	} `json:"groups"`
	Priced []struct {
		Kind    string  `json:"kind"`
		ID      string  `json:"id"`
		Monthly float64 `json:"monthly"`
		Priced  bool    `json:"priced"`
	} `json:"priced"`
}

func runHistorical(t *testing.T, home, promURL string, from, to time.Time) ([]byte, []byte, error) {
	t.Helper()
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary, "set FINFOCUS_BINARY or build bin/finfocus")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, histArgs(from, to)...)
	cmd.Env = append(os.Environ(),
		"FINFOCUS_HOME="+home,
		"FINFOCUS_PROMETHEUS_URL="+promURL,
		"FINFOCUS_FIXEDCOST_TOTAL="+fixedTotal,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func histArgs(from, to time.Time) []string {
	return []string{
		"cost", "cluster",
		"--from", from.Format(time.DateOnly), "--to", to.Format(time.DateOnly),
		"--usage-source", "prometheus", "--allocator", "kubernetes",
		"--context", kindContext(), "--output", "json", "--group-by", "namespace",
	}
}

func kubectl(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("kubectl", append([]string{"--context", kindContext()}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "kubectl %s: %s", strings.Join(args, " "), out)
	return out
}

func kindNodeName(t *testing.T) string {
	t.Helper()
	out := kubectl(t, "get", "nodes", "-o", "jsonpath={.items[0].metadata.name}")
	name := strings.TrimSpace(string(out))
	require.NotEmpty(t, name)
	return name
}

func forwardPrometheus(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "kubectl", "--context", kindContext(),
		"-n", "monitoring", "port-forward", "svc/prometheus", "0:9090")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})
	return readForwardAddr(t, stdout, &stderr)
}

func readForwardAddr(t *testing.T, stdout io.Reader, stderr *bytes.Buffer) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "127.0.0.1:") {
					done <- "http://" + field
					return
				}
			}
		}
		done <- ""
	}()
	select {
	case addr := <-done:
		require.NotEmptyf(t, addr, "port-forward stderr: %s", stderr.String())
		return addr
	case <-time.After(30 * time.Second):
		require.FailNowf(t, "port-forward timed out", "no local address printed: %s", stderr.String())
		return ""
	}
}

func waitPrometheusReady(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/-/ready", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Second)
	}
	require.FailNowf(t, "prometheus not ready", "no 200 from %s/-/ready within 30s", base)
}

func writeHistoricalSeries(t *testing.T, base, node string, from, to time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, series := range historicalSeries(node, from, to) {
		require.NoError(t, remoteWrite(ctx, base, []rwSeries{series}))
	}
}

func historicalSeries(node string, from, to time.Time) []rwSeries {
	window := sampleEvery(from, to, time.Minute)
	counter := sampleEvery(from.Add(-5*time.Minute), to, time.Minute)
	cpu := make([]rwPoint, len(counter))
	for i, at := range counter {
		cpu[i] = rwPoint{at: at, value: float64(at.Unix())}
	}
	pod := map[string]string{"namespace": "e2e", "pod": "web-0", "container": "web", "node": node}
	return []rwSeries{
		{labels: withName("container_cpu_usage_seconds_total", pod), points: cpu},
		{labels: withName("container_memory_working_set_bytes", pod), points: constantPoints(window, gib)},
		{labels: withName("kube_node_status_allocatable", map[string]string{"node": node, "resource": "cpu"}),
			points: constantPoints(window, 4)},
		{labels: withName("kube_node_status_allocatable", map[string]string{"node": node, "resource": "memory"}),
			points: constantPoints(window, 16*gib)},
		// A second kube-state-metrics replica exports the same node again.
		// Capacity must not double.
		{labels: withName("kube_node_status_allocatable", map[string]string{
			"node": node, "resource": "cpu", "instance": "kube-state-metrics-replica-2",
		}), points: constantPoints(window, 4)},
		{labels: withName("kube_node_status_allocatable", map[string]string{
			"node": node, "resource": "memory", "instance": "kube-state-metrics-replica-2",
		}), points: constantPoints(window, 16*gib)},
		{labels: withName("kube_node_labels", map[string]string{
			"node":                                   node,
			"label_node_kubernetes_io_instance_type": "m5.large",
			"label_topology_kubernetes_io_region":    "us-east-1",
			"label_finfocus_dev_provider":            "aws",
		}), points: []rwPoint{{at: to, value: 1}}},
		{labels: withName("kube_node_info", map[string]string{
			"node": node, "provider_id": "aws:///us-east-1a/i-kind",
		}), points: []rwPoint{{at: to, value: 1}}},
	}
}

func withName(name string, labels map[string]string) map[string]string {
	out := make(map[string]string, len(labels)+1)
	for key, value := range labels {
		out[key] = value
	}
	out["__name__"] = name
	return out
}

func sampleEvery(from, to time.Time, step time.Duration) []time.Time {
	var out []time.Time
	for at := from; !at.After(to); at = at.Add(step) {
		out = append(out, at)
	}
	return out
}

func constantPoints(times []time.Time, value float64) []rwPoint {
	out := make([]rwPoint, len(times))
	for i, at := range times {
		out[i] = rwPoint{at: at, value: value}
	}
	return out
}

func requireInstant(t *testing.T, base, expr string, at time.Time, want float64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	got, err := queryInstant(ctx, base, expr, at)
	require.NoError(t, err)
	require.Len(t, got, 1, expr)
	assert.InDelta(t, want, got[0].value, 1e-6, expr)
}

func cpuUsageQuery() string {
	return `sum by (namespace, pod, node) (
  sum_over_time(
    rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])
    [86400s:60s]
  )
) * 60 / 3600`
}

func memoryUsageQuery() string {
	return `sum by (namespace, pod, node) (
  sum_over_time(
    container_memory_working_set_bytes{container!="",container!="POD"}[86400s:60s]
  )
) * 60 / 1073741824 / 3600`
}

func cpuAllocQuery() string {
	return `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{resource="cpu"}))[86400s:60s]
  )
) * 60 / 3600`
}

func memoryAllocQuery() string {
	return `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{resource="memory"}))[86400s:60s]
  )
) * 60 / 1073741824 / 3600`
}
