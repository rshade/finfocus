package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func sampleClusterOutput(scoped bool) clusterOutput {
	idle := 7.0
	out := clusterOutput{
		Mode: engine.ModeRunRate, Period: "monthly", Currency: "USD", GroupBy: "namespace",
		Total: 100, Idle: &idle,
		Groups: []engine.ClusterGroup{
			{Key: "payments", CPUCost: 40, MemCost: 20, TotalCost: 60, Rows: 2,
				Notes: []string{"spot node priced on-demand"}},
			{Key: "__idle__", TotalCost: 7, Rows: 1},
		},
		Policy: clusterPolicyOutput{Source: "built-in defaults", Digest: strings.Repeat("a", 64),
			Effective: json.RawMessage(`{"version":1}`)},
	}
	if scoped {
		out.Idle, out.NamespaceScoped = nil, true
	}
	return out
}

func renderTo(t *testing.T, format string, out clusterOutput) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	require.NoError(t, renderClusterResult(cmd, format, out))
	return buf.String()
}

func TestRenderCluster_Table(t *testing.T) {
	t.Parallel()

	s := renderTo(t, outputFormatTable, sampleClusterOutput(false))
	assert.Contains(t, s, "GROUP")
	assert.Contains(t, s, "payments")
	assert.Contains(t, s, "60.00")
	assert.Contains(t, s, "spot node priced on-demand")
	assert.Contains(t, s, "run-rate (monthly, 730 h)")
	assert.Contains(t, s, "Idle:")
	assert.Contains(t, s, "7.0%")
	assert.Contains(t, s, "built-in defaults · aaaaaaaaaaaa")

	scoped := renderTo(t, outputFormatTable, sampleClusterOutput(true))
	assert.Contains(t, scoped, "omitted (--namespace scoped)")
}

func TestRenderCluster_JSON(t *testing.T) {
	t.Parallel()

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, sampleClusterOutput(false))), &got))
	assert.Equal(t, "run-rate", got["mode"])
	assert.InDelta(t, 100, got["total"], 1e-9)
	assert.Len(t, got["groups"], 2)
	assert.Equal(t, map[string]any{"version": float64(1)}, got["policy"].(map[string]any)["effective"])

	var scoped map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, sampleClusterOutput(true))), &scoped))
	_, hasIdle := scoped["idle"]
	assert.False(t, hasIdle)
	assert.Equal(t, true, scoped["namespace_scoped"])
}

func TestRenderCluster_NDJSON(t *testing.T) {
	t.Parallel()

	lines := strings.Split(strings.TrimSpace(renderTo(t, outputFormatNDJSON, sampleClusterOutput(false))), "\n")
	require.Len(t, lines, 3)
	var first, second map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
	assert.Equal(t, "summary", first["type"])
	assert.NotContains(t, first, "groups")
	assert.Equal(t, "group", second["type"])
	assert.Equal(t, "payments", second["key"])
}
