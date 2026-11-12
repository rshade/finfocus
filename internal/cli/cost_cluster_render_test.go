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

func TestRenderCluster_ScopeFooter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace bool
		selector  bool
		want      string
	}{
		{name: "namespace", namespace: true, want: "Idle:    omitted (--namespace scoped)"},
		{name: "selector", selector: true, want: "Idle:    omitted (--selector scoped)"},
		{name: "both", namespace: true, selector: true, want: "Idle:    omitted (--namespace and --selector scoped)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := &engine.ClusterResult{
				Mode: engine.ModeRunRate, Period: "monthly", Currency: "USD", Total: 60, Idle: 0,
				NamespaceScoped: tt.namespace, SelectorScoped: tt.selector,
			}
			out := newClusterOutput(res, nil, "namespace", clusterPolicyOutput{})
			assert.Nil(t, out.Idle)
			assert.Contains(t, renderTo(t, outputFormatTable, out), tt.want)

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, out)), &got))
			assert.NotContains(t, got, "idle")
			assert.Equal(t, tt.namespace, got["namespace_scoped"], "namespace_scoped keeps its meaning")
			assert.Equal(t, tt.selector, got["selector_scoped"])

			lines := strings.Split(strings.TrimSpace(renderTo(t, outputFormatNDJSON, out)), "\n")
			var summary map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[0]), &summary))
			assert.NotContains(t, summary, "idle")
			assert.Equal(t, tt.selector, summary["selector_scoped"])
		})
	}
}

func TestRenderCluster_UnscopedJSONReportsIdle(t *testing.T) {
	t.Parallel()

	res := &engine.ClusterResult{Mode: engine.ModeRunRate, Period: "monthly", Currency: "USD", Total: 100, Idle: 40}
	var got map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(renderTo(t, outputFormatJSON, newClusterOutput(res, nil, "namespace", clusterPolicyOutput{}))), &got))
	assert.InDelta(t, 40, got["idle"], 1e-9)
	assert.Equal(t, false, got["namespace_scoped"])
	assert.Equal(t, false, got["selector_scoped"])
}

func TestRenderCluster_HistoricalPeriod(t *testing.T) {
	t.Parallel()

	const windowTotal = 18.5
	out := sampleClusterOutput(false)
	out.Mode = engine.ModeHistorical
	out.Period = "2026-09-28 to 2026-10-05, 7 days"
	out.Priced = []engine.PricedSummary{{
		Kind: "node", ID: "n1", ResourceType: "aws:ec2/instance:Instance",
		Monthly: windowTotal, Priced: true,
	}}

	table := renderTo(t, outputFormatTable, out)
	assert.Contains(t, table, "Mode:    historical (2026-09-28 to 2026-10-05, 7 days)")
	assert.NotContains(t, table, "monthly")
	assert.NotContains(t, table, "730")

	runRate := renderTo(t, outputFormatTable, sampleClusterOutput(false))
	assert.Contains(t, runRate, "run-rate (monthly, 730 h)")
	assert.Contains(t, runRate, "730")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, out)), &got))
	assert.Equal(t, "historical", got["mode"])
	assert.Equal(t, "2026-09-28 to 2026-10-05, 7 days", got["period"])
	priced, ok := got["priced"].([]any)
	require.True(t, ok)
	require.Len(t, priced, 1)
	row, ok := priced[0].(map[string]any)
	require.True(t, ok)
	_, hasMonthly := row["monthly"]
	assert.True(t, hasMonthly)
	assert.InDelta(t, windowTotal, row["monthly"], 1e-9)

	res := &engine.ClusterResult{
		Mode: engine.ModeHistorical, Period: "2026-09-28 to 2026-10-05, 7 days",
		Priced: []engine.PricedSummary{{ID: "n1", Monthly: windowTotal, Priced: true}},
	}
	copied := newClusterOutput(res, nil, "namespace", clusterPolicyOutput{})
	assert.Equal(t, engine.ModeHistorical, copied.Mode)
	assert.Equal(t, "2026-09-28 to 2026-10-05, 7 days", copied.Period)
	require.Len(t, copied.Priced, 1)
	assert.InDelta(t, windowTotal, copied.Priced[0].Monthly, 1e-9)
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

func capacityClusterOutput() clusterOutput {
	out := sampleClusterOutput(false)
	out.Capacity = []engine.CapacityGroup{
		{CapacityType: "on-demand", TotalCost: 55, IdleCost: ptrFloat(3), Nodes: 2, Rows: 4},
		{CapacityType: "spot", TotalCost: 35, IdleCost: ptrFloat(3), Nodes: 1, Rows: 2,
			Notes: []string{"spot node priced on-demand"}},
		{CapacityType: engine.CapacityUnlabeled, TotalCost: 10, IdleCost: ptrFloat(1), Nodes: 1, Rows: 2},
	}
	return out
}

func TestRenderCluster_CapacityTable(t *testing.T) {
	t.Parallel()

	s := renderTo(t, outputFormatTable, capacityClusterOutput())
	lines := strings.Split(s, "\n")
	header := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "CAPACITY TYPE") {
			header = i
		}
	}
	require.GreaterOrEqual(t, header, 0, s)
	assert.Regexp(t, `^on-demand\s+2\s+55\.00\s+55\.0%\s+3\.00\s*$`, lines[header+1])
	assert.Regexp(t, `^spot\s+1\s+35\.00\s+35\.0%\s+3\.00\s+spot node priced on-demand$`, lines[header+2])
	assert.Regexp(t, `^unlabeled\s+1\s+10\.00\s+10\.0%\s+1\.00\s*$`, lines[header+3])
}

func ptrFloat(v float64) *float64 { return &v }

func scopedCapacityOutput() clusterOutput {
	out := capacityClusterOutput()
	out.Idle, out.NamespaceScoped = nil, true
	for i := range out.Capacity {
		out.Capacity[i].IdleCost = nil
	}
	return out
}

func TestRenderCluster_CapacityNamespaceScoped(t *testing.T) {
	t.Parallel()

	table := renderTo(t, outputFormatTable, scopedCapacityOutput())
	assert.Contains(t, table, "omitted (--namespace scoped)")
	assert.Regexp(t, `(?m)^on-demand\s+2\s+55\.00\s+55\.0%\s+-\s*$`, table)
	assert.Regexp(t, `(?m)^spot\s+1\s+35\.00\s+35\.0%\s+-\s+spot node priced on-demand$`, table)

	var got struct {
		Capacity []map[string]any `json:"capacity"`
	}
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, scopedCapacityOutput())), &got))
	require.Len(t, got.Capacity, 3)
	for _, g := range got.Capacity {
		assert.NotContains(t, g, "idle_cost")
	}
}

func TestRenderCluster_CapacityTableOmittedWhenAllUnlabeled(t *testing.T) {
	t.Parallel()

	out := sampleClusterOutput(false)
	out.Capacity = []engine.CapacityGroup{{CapacityType: engine.CapacityUnlabeled, TotalCost: 100, Nodes: 1, Rows: 3}}
	assert.NotContains(t, renderTo(t, outputFormatTable, out), "CAPACITY TYPE")
}

func TestRenderCluster_CapacityJSON(t *testing.T) {
	t.Parallel()

	var got struct {
		Capacity []map[string]any `json:"capacity"`
	}
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, capacityClusterOutput())), &got))
	require.Len(t, got.Capacity, 3)
	assert.Equal(t, "spot", got.Capacity[1]["capacity_type"])
	assert.InDelta(t, 35, got.Capacity[1]["total_cost"], 1e-9)
	assert.InDelta(t, 3, got.Capacity[1]["idle_cost"], 1e-9)
	assert.InDelta(t, 1, got.Capacity[1]["nodes"], 1e-9)
	assert.Equal(t, []any{"spot node priced on-demand"}, got.Capacity[1]["notes"])
	assert.Equal(t, engine.CapacityUnlabeled, got.Capacity[2]["capacity_type"])
	assert.NotContains(t, got.Capacity[0], "notes")

	var empty map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON,
		newClusterOutput(&engine.ClusterResult{}, nil, "namespace", clusterPolicyOutput{}))), &empty))
	assert.Equal(t, []any{}, empty["capacity"], "capacity is an empty array, never null")
}

func TestNewClusterOutput_CarriesCapacity(t *testing.T) {
	t.Parallel()

	res := &engine.ClusterResult{Capacity: []engine.CapacityGroup{{CapacityType: "spot", TotalCost: 1}}}
	out := newClusterOutput(res, nil, "namespace", clusterPolicyOutput{})
	require.Len(t, out.Capacity, 1)
	assert.Equal(t, "spot", out.Capacity[0].CapacityType)
	assert.NotNil(t, newClusterOutput(&engine.ClusterResult{}, nil, "namespace", clusterPolicyOutput{}).Capacity)
}

func TestRenderCluster_NDJSONSummaryCarriesCapacity(t *testing.T) {
	t.Parallel()

	lines := strings.Split(strings.TrimSpace(renderTo(t, outputFormatNDJSON, capacityClusterOutput())), "\n")
	var first map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	assert.Len(t, first["capacity"], 3)
}
