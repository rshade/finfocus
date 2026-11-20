package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClusterReport(t *testing.T) {
	t.Parallel()

	policy := ClusterPolicy{Source: "built-in defaults", Digest: "d1", Effective: json.RawMessage(`{"idle":"share"}`)}
	groups := []ClusterGroup{{Key: "payments", TotalCost: 60}}

	tests := []struct {
		name      string
		res       *ClusterResult
		wantIdle  *float64
		wantCapty int
	}{
		{
			name:     "unscoped reports idle, even when zero",
			res:      &ClusterResult{Mode: ModeRunRate, Period: "monthly", Currency: "USD", Total: 100},
			wantIdle: new(float64),
		},
		{
			name: "namespace scoped omits idle",
			res:  &ClusterResult{Total: 60, Idle: 40, NamespaceScoped: true},
		},
		{
			name: "selector scoped omits idle",
			res:  &ClusterResult{Total: 60, Idle: 40, SelectorScoped: true},
		},
		{
			name:      "capacity is carried",
			res:       &ClusterResult{Idle: 5, Capacity: []CapacityGroup{{CapacityType: "spot", TotalCost: 1}}},
			wantIdle:  new(5.0),
			wantCapty: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := NewClusterReport(tt.res, groups, "namespace", policy)
			assert.Equal(t, tt.wantIdle, report.Idle)
			require.NotNil(t, report.Capacity, "capacity is never nil, so JSON is [] not null")
			assert.Len(t, report.Capacity, tt.wantCapty)
			assert.Equal(t, "namespace", report.GroupBy)
			assert.Equal(t, groups, report.Groups)
			assert.Equal(t, policy, report.Policy)
			assert.Equal(t, tt.res.NamespaceScoped, report.NamespaceScoped)
			assert.Equal(t, tt.res.SelectorScoped, report.SelectorScoped)
		})
	}
}

func TestClusterReportJSONFields(t *testing.T) {
	t.Parallel()

	res := &ClusterResult{
		Mode: ModeRunRate, Period: "monthly", Currency: "USD", Total: 10, Incomplete: true,
		Priced: []PricedSummary{{ID: "n1", Priced: true}}, Warnings: []string{"w"},
	}
	data, err := json.Marshal(NewClusterReport(res, []ClusterGroup{}, "node", ClusterPolicy{}))
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.ElementsMatch(t, []string{
		"mode", "period", "currency", "group_by", "total", "idle", "namespace_scoped", "selector_scoped",
		"incomplete", "groups", "capacity", "priced", "policy", "warnings",
	}, mapKeys(got))
	policy, ok := got["policy"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"source", "digest", "effective"}, mapKeys(policy))
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
