//go:build e2e_kind

package e2e

import (
	"cmp"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

// kindContext matches the cluster test/e2e/kind/setup.sh creates.
func kindContext() string {
	return "kind-" + cmp.Or(os.Getenv("KIND_CLUSTER"), "finfocus-e2e")
}

type clusterJSON struct {
	Total           float64  `json:"total"`
	Idle            *float64 `json:"idle"`
	NamespaceScoped bool     `json:"namespace_scoped"`
	Incomplete      bool     `json:"incomplete"`
	Groups          []struct {
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

func runCluster(t *testing.T, args ...string) (clusterJSON, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, findFinFocusBinary(),
		append([]string{"cost", "cluster", "--context", kindContext(), "--output", "json"}, args...)...)
	out, err := cmd.Output()
	var res clusterJSON
	if err == nil {
		require.NoError(t, json.Unmarshal(out, &res), string(out))
	}
	return res, out, err
}

type nodeAlloc struct{ cpu, memGiB float64 }

func kubectlJSON(t *testing.T, v any, args ...string) {
	t.Helper()
	out, err := exec.Command("kubectl", append([]string{"--context", kindContext()}, args...)...).Output()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(out, v))
}

func nodeAllocatable(t *testing.T) map[string]nodeAlloc {
	var list struct {
		Items []struct {
			Metadata struct{ Name string } `json:"metadata"`
			Status   struct {
				Allocatable map[string]string `json:"allocatable"`
			} `json:"status"`
		} `json:"items"`
	}
	kubectlJSON(t, &list, "get", "nodes", "-o", "json")
	out := map[string]nodeAlloc{}
	for _, n := range list.Items {
		cpu := resource.MustParse(n.Status.Allocatable["cpu"])
		mem := resource.MustParse(n.Status.Allocatable["memory"])
		out[n.Metadata.Name] = nodeAlloc{float64(cpu.MilliValue()) / 1000, float64(mem.Value()) / (1 << 30)}
	}
	return out
}

func webPodNodes(t *testing.T) []string {
	var list struct {
		Items []struct {
			Spec struct{ NodeName string } `json:"spec"`
		} `json:"items"`
	}
	kubectlJSON(t, &list, "-n", "e2e", "get", "pods", "-l", "app=web", "-o", "json")
	var nodes []string
	for _, p := range list.Items {
		nodes = append(nodes, p.Spec.NodeName)
	}
	return nodes
}

func TestCostCluster_Kind(t *testing.T) {
	res, out, err := runCluster(t, "--group-by", "controller")
	require.NoError(t, err, string(out))
	require.False(t, res.Incomplete, "every kind node is labeled m5.large/us-east-1")

	nodePrice := map[string]float64{}
	var pricedTotal float64
	for _, p := range res.Priced {
		require.True(t, p.Priced, p.ID)
		nodePrice[p.ID] = p.Monthly
		pricedTotal += p.Monthly
		assert.InDelta(t, 70, p.Monthly, 15, "m5.large us-east-1 monthly price is roughly $70")
	}

	var groupSum float64
	groups := map[string]float64{}
	for _, g := range res.Groups {
		groupSum += g.TotalCost
		groups[g.Key] = g.TotalCost
	}
	assert.InDelta(t, pricedTotal, groupSum, pricedTotal*1e-6, "rows add up to the bill")
	assert.InDelta(t, pricedTotal, res.Total, pricedTotal*1e-6)
	require.NotNil(t, res.Idle)
	assert.Greater(t, *res.Idle, 0.0)
	assert.InDelta(t, *res.Idle, groups["__idle__"], 1e-9)

	// Deployment web: 2 × (250m, 256Mi). Default split weights 0.031611/core-h, 0.004237/GiB-h.
	alloc := nodeAllocatable(t)
	var want float64
	for _, node := range webPodNodes(t) {
		a := alloc[node]
		cpuW, memW := a.cpu*0.031611, a.memGiB*0.004237
		price := nodePrice[node]
		want += price*cpuW/(cpuW+memW)*(0.25/a.cpu) + price*memW/(cpuW+memW)*(0.25/a.memGiB)
	}
	assert.InDelta(t, want, groups["e2e/Deployment/web"], want*1e-6)
}

func TestCostCluster_Kind_NamespaceScope(t *testing.T) {
	res, out, err := runCluster(t, "--namespace", "e2e", "--group-by", "namespace")
	require.NoError(t, err, string(out))
	assert.True(t, res.NamespaceScoped)
	assert.Nil(t, res.Idle)
	require.Len(t, res.Groups, 1)
	assert.Equal(t, "e2e", res.Groups[0].Key)
	assert.Greater(t, res.Groups[0].TotalCost, 0.0)
	for _, g := range res.Groups {
		assert.NotEqual(t, "__idle__", g.Key)
	}
}

func TestCostCluster_Kind_BadPolicyFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "allocation.hujson")
	require.NoError(t, os.WriteFile(p, []byte(`{"idel": "share"}`), 0o600))
	_, _, err := runCluster(t, "--policy", p)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	// stderr carries an ax JSON error envelope, so the policy key appears with
	// escaped quotes; match the decoder's wording and the offending key.
	assert.Contains(t, string(exitErr.Stderr), "unknown field")
	assert.Contains(t, string(exitErr.Stderr), "idel")
}

func TestCostCluster_Kind_ShowPolicyNeedsNoCluster(t *testing.T) {
	out, err := exec.Command(findFinFocusBinary(), "cost", "cluster", "--context", "does-not-exist",
		"--show-policy", "--output", "json").Output()
	require.NoError(t, err)
	var p struct {
		Digest    string         `json:"digest"`
		Effective map[string]any `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(out, &p))
	assert.Len(t, p.Digest, 64)
	assert.Equal(t, float64(1), p.Effective["version"])
}
