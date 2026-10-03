package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clusterRows() []ClusterRow {
	w := func(ns, pod, node, team string, cost float64, note string) ClusterRow {
		s := map[string]string{"kind": "workload", "namespace": ns, "pod": pod, "node": node,
			"controller_kind": "Deployment", "controller": "api"}
		if team != "" {
			s["label.team"] = team
		}
		return ClusterRow{Subject: s, CPUCost: cost / 2, MemCost: cost / 2, TotalCost: cost, Note: note}
	}
	return []ClusterRow{
		w("payments", "api-1", "n1", "pay", 10, ""),
		w("payments", "api-2", "n2", "pay", 10, "spot node priced on-demand"),
		w("search", "api-1", "n1", "", 5, ""),
		{Subject: map[string]string{"kind": "__idle__", "node": "n1"}, TotalCost: 7},
		{Subject: map[string]string{"kind": "__cluster__", "cluster": "prod"}, TotalCost: 73},
	}
}

func groupTotals(gs []ClusterGroup) map[string]float64 {
	m := map[string]float64{}
	for _, g := range gs {
		m[g.Key] = g.TotalCost
	}
	return m
}

func TestGroupClusterRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		groupBy string
		want    map[string]float64
	}{
		{"namespace", map[string]float64{"payments": 20, "search": 5, "__idle__": 7, "__cluster__": 73}},
		{"controller", map[string]float64{
			"payments/Deployment/api": 20, "search/Deployment/api": 5, "__idle__": 7, "__cluster__": 73,
		}},
		{"pod", map[string]float64{
			"payments/api-1": 10, "payments/api-2": 10, "search/api-1": 5, "__idle__": 7, "__cluster__": 73,
		}},
		{"node", map[string]float64{"n1": 22, "n2": 10, "__cluster__": 73}},
		{"label:team", map[string]float64{"pay": 20, GroupKeyNone: 5, "__idle__": 7, "__cluster__": 73}},
	}
	for _, tt := range tests {
		t.Run(tt.groupBy, func(t *testing.T) {
			t.Parallel()
			gs, err := GroupClusterRows(clusterRows(), tt.groupBy)
			require.NoError(t, err)
			assert.Equal(t, tt.want, groupTotals(gs))
			var sum float64
			for _, g := range gs {
				sum += g.TotalCost
			}
			assert.InDelta(t, 105, sum, 1e-9, "grouping never loses cost")
		})
	}
}

func TestGroupClusterRows_OrderAndNotes(t *testing.T) {
	t.Parallel()

	gs, err := GroupClusterRows(clusterRows(), "namespace")
	require.NoError(t, err)
	assert.Equal(t, "__cluster__", gs[0].Key, "highest total first")
	for _, g := range gs {
		if g.Key == "payments" {
			assert.Equal(t, []string{"spot node priced on-demand"}, g.Notes)
			assert.Equal(t, 2, g.Rows)
		}
	}
}

func TestGroupClusterRows_PulumiStack(t *testing.T) {
	t.Parallel()

	const (
		apiURN    = "urn:pulumi:prod::payments::kubernetes:apps/v1:Deployment::api"
		workerURN = "urn:pulumi:prod::payments::kubernetes:apps/v1:Deployment::worker"
		otherURN  = "urn:pulumi:dev::search::kubernetes:apps/v1:Deployment::api"
		parentURN = "urn:pulumi:prod::payments::component:index:App$kubernetes:apps/v1:Deployment::child"
		badURN    = "urn:pulumi:prod::payments"
	)
	row := func(urn string, cost float64) ClusterRow {
		s := map[string]string{"kind": "workload", "namespace": "payments"}
		if urn != "" {
			s[pulumiURNSubjectKey] = urn
		}
		return ClusterRow{Subject: s, TotalCost: cost}
	}
	rows := []ClusterRow{
		row(apiURN, 10),
		row(workerURN, 4),
		row(parentURN, 1),
		row(otherURN, 5),
		row(badURN, 2),
		row("", 3),
		{Subject: map[string]string{"kind": "__idle__", "node": "n1"}, TotalCost: 7},
		{Subject: map[string]string{"kind": "__cluster__", "cluster": "prod"}, TotalCost: 8},
	}
	gs, err := GroupClusterRows(rows, "pulumi-stack")
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{
		"prod/payments": 15, "dev/search": 5, GroupKeyNone: 5, "__idle__": 7, "__cluster__": 8,
	}, groupTotals(gs))

	var none, payments ClusterGroup
	for _, g := range gs {
		switch g.Key {
		case GroupKeyNone:
			none = g
		case "prod/payments":
			payments = g
		}
	}
	assert.Equal(t, []string{parentURN, apiURN, workerURN}, payments.PulumiURNs)
	assert.Equal(t, []string{badURN}, none.PulumiURNs)
	encoded, err := json.Marshal(payments)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"pulumi_urns"`)
}

func TestValidateClusterGroupBy(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"namespace", "controller", "pod", "node", "pulumi-stack", "label:team", "label:app.kubernetes.io/name"} {
		require.NoError(t, ValidateClusterGroupBy(ok), ok)
	}
	for _, bad := range []string{"", "daily", "label:", "labels:team", "Namespace"} {
		err := ValidateClusterGroupBy(bad)
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "namespace, controller, pod, node, pulumi-stack, label:<key>")
	}
}
