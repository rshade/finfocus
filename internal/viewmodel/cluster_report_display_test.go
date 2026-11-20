package viewmodel

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestClusterModeLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "run-rate (monthly, 730 h)", ClusterModeLine(engine.ModeRunRate, "monthly"))
	assert.Equal(t, "historical (2026-09-28 to 2026-10-05, 7 days)",
		ClusterModeLine(engine.ModeHistorical, "2026-09-28 to 2026-10-05, 7 days"))
}

func TestClusterShare(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "20.0%", ClusterShare(20, 100))
	assert.Equal(t, "33.3%", ClusterShare(1, 3))
	assert.Equal(t, "0.0%", ClusterShare(5, 0), "a zero total has no share")
}

func TestClusterPolicyLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "built-in defaults · 0123456789ab", ClusterPolicyLine("built-in defaults", "0123456789abcdef"))
}

func TestShortDigest(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abcdefabcdef", ShortDigest(strings.Repeat("abcdef", 10)))
	assert.Equal(t, "abc", ShortDigest("abc"))
}

func TestSummarizeClusterWarnings(t *testing.T) {
	t.Parallel()

	var warnings []string
	for i := range 5 {
		warnings = append(warnings, fmt.Sprintf("incomplete: ns/pod-%d: gap in cpu_usage", i))
	}
	warnings = append(warnings,
		"incomplete: ns/pod-0: gap in cpu_usage", // a repeated subject counts as an entry, not a resource
		"incomplete: ns/lonely: gap in mem_usage",
		"incomplete: store-wide retention is short",
		"plain warning",
	)

	summary := SummarizeClusterWarnings(warnings)
	assert.Equal(t, []string{
		"incomplete: ns/lonely: gap in mem_usage",
		"incomplete: store-wide retention is short",
		"plain warning",
		"incomplete: gap in cpu_usage for 5 resources (ns/pod-0, ns/pod-1, ns/pod-2, and 2 more)",
	}, summary.Lines())
	assert.Equal(t, 6, summary.Summarized())
	assert.Equal(t, []string{
		"ns/lonely: gap in mem_usage",
		"store-wide retention is short",
		"gap in cpu_usage for 5 resources",
	}, summary.UsageCauses())

	empty := SummarizeClusterWarnings(nil)
	assert.Empty(t, empty.Lines())
	assert.Zero(t, empty.Summarized())
}

func TestClusterUnpriced(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"n2: no price for m9.huge", "n3"}, ClusterUnpriced([]engine.PricedSummary{
		{ID: "n1", Priced: true},
		{ID: "n2", Note: "no price for m9.huge"},
		{ID: "n3"},
	}))
	assert.Empty(t, ClusterUnpriced([]engine.PricedSummary{{ID: "n1", Priced: true}}))
}

func TestBuildClusterReportDisplay(t *testing.T) {
	t.Parallel()

	idle := 60.0
	spotIdle := 3.0
	report := engine.ClusterReport{
		Mode: engine.ModeRunRate, Period: "monthly", Currency: "USD", Total: 300, Idle: &idle,
		Groups: []engine.ClusterGroup{
			{Key: "payments", CPUCost: 120, MemCost: 120, TotalCost: 240},
			{Key: "__idle__", TotalCost: 60},
		},
		Capacity: []engine.CapacityGroup{
			{CapacityType: "spot", TotalCost: 30, IdleCost: &spotIdle, Nodes: 1},
			{CapacityType: engine.CapacityUnlabeled, TotalCost: 270, Nodes: 2},
		},
		Priced:     []engine.PricedSummary{{ID: "n1", Priced: true}, {ID: "n2", Note: "no price"}},
		Policy:     engine.ClusterPolicy{Source: "built-in defaults", Digest: strings.Repeat("a", 64)},
		Incomplete: true,
		Warnings:   []string{"incomplete: store-wide retention is short"},
	}

	got := BuildClusterReportDisplay(report)
	assert.Equal(t, "$300.00 USD", got.Total)
	assert.Equal(t, "$60.00 USD", got.Idle)
	assert.Equal(t, "20.0%", got.IdleShare)
	assert.Empty(t, got.Scope)
	assert.Equal(t, "run-rate (monthly, 730 h)", got.Mode)
	assert.Equal(t, "built-in defaults · aaaaaaaaaaaa", got.Policy)
	require.Len(t, got.Groups, 2)
	assert.Equal(t, ClusterGroupDisplay{
		Key: "payments", CPU: "$120.00 USD", Memory: "$120.00 USD", Total: "$240.00 USD", Share: "80.0%",
	}, got.Groups[0])
	assert.Equal(t, "__idle__", got.Groups[1].Key, "groups keep the report's order")
	require.Len(t, got.Capacity, 2)
	assert.Equal(t, ClusterCapacityDisplay{
		CapacityType: "spot", Total: "$30.00 USD", Share: "10.0%", Idle: "$3.00 USD",
	}, got.Capacity[0])
	assert.Empty(t, got.Capacity[1].Idle, "unmeasured idle has no amount")
	assert.Equal(t, []string{"incomplete: store-wide retention is short"}, got.Warnings)
	assert.Equal(t, []string{"store-wide retention is short"}, got.IncompleteCauses)
	assert.Equal(t, []string{"n2: no price"}, got.Unpriced)
}

func TestBuildClusterReportDisplay_Scoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace bool
		selector  bool
		want      string
	}{
		{name: "namespace", namespace: true, want: "namespace"},
		{name: "selector", selector: true, want: "selector"},
		{name: "both", namespace: true, selector: true, want: "namespace and selector"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := BuildClusterReportDisplay(engine.ClusterReport{
				Total: 10, NamespaceScoped: tt.namespace, SelectorScoped: tt.selector,
			})
			assert.Empty(t, got.Idle)
			assert.Empty(t, got.IdleShare)
			assert.Equal(t, tt.want, got.Scope)
			assert.NotNil(t, got.Groups, "lists are [] in JSON, never null")
			assert.NotNil(t, got.Capacity)
			assert.NotNil(t, got.Warnings)
			assert.NotNil(t, got.IncompleteCauses)
			assert.NotNil(t, got.Unpriced)
		})
	}
}
