package promql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueries_Golden(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	const dur = "604800s"

	plain := Selectors{}
	selected := Selectors{Namespace: "payments", Cluster: "prod"}

	tests := []struct {
		name string
		got  Query
		expr string
	}{
		{
			name: "cpu usage",
			got:  CPUUsage(from, to, plain),
			expr: `sum by (namespace, pod, node) (
  sum_over_time(
    rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])
    [` + dur + `:60s]
  )
) * 60 / 3600`,
		},
		{
			name: "cpu usage with namespace and cluster",
			got:  CPUUsage(from, to, selected),
			expr: `sum by (namespace, pod, node) (
  sum_over_time(
    rate(container_cpu_usage_seconds_total{container!="",container!="POD",namespace="payments",cluster="prod"}[5m])
    [` + dur + `:60s]
  )
) * 60 / 3600`,
		},
		{
			name: "memory usage",
			got:  MemoryUsage(from, to, plain),
			expr: `sum by (namespace, pod, node) (
  sum_over_time(
    container_memory_working_set_bytes{container!="",container!="POD"}[` + dur + `:60s]
  )
) * 60 / 1073741824 / 3600`,
		},
		{
			name: "memory usage with namespace and cluster",
			got:  MemoryUsage(from, to, selected),
			expr: `sum by (namespace, pod, node) (
  sum_over_time(
    container_memory_working_set_bytes{container!="",container!="POD",namespace="payments",cluster="prod"}[` + dur + `:60s]
  )
) * 60 / 1073741824 / 3600`,
		},
		{
			name: "cpu allocatable",
			got:  CPUAllocatable(from, to, plain),
			expr: `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{resource="cpu"}))[` + dur + `:60s]
  )
) * 60 / 3600`,
		},
		{
			name: "cpu allocatable keeps cluster and drops namespace",
			got:  CPUAllocatable(from, to, Selectors{Namespace: "payments", Cluster: "prod"}),
			expr: `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{resource="cpu",cluster="prod"}))[` + dur + `:60s]
  )
) * 60 / 3600`,
		},
		{
			name: "memory allocatable",
			got:  MemoryAllocatable(from, to, plain),
			expr: `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{resource="memory"}))[` + dur + `:60s]
  )
) * 60 / 1073741824 / 3600`,
		},
		{
			name: "memory allocatable keeps cluster and drops namespace",
			got:  MemoryAllocatable(from, to, Selectors{Namespace: "payments", Cluster: "prod"}),
			expr: `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{resource="memory",cluster="prod"}))[` + dur + `:60s]
  )
) * 60 / 1073741824 / 3600`,
		},
		{
			name: "store start",
			got:  StoreStart(from, to, plain),
			expr: `min(min_over_time(timestamp(kube_node_status_allocatable{resource="cpu"})[` + dur + `:60s]))`,
		},
		{
			name: "store start keeps cluster and drops namespace",
			got:  StoreStart(from, to, selected),
			expr: `min(min_over_time(timestamp(kube_node_status_allocatable{resource="cpu",cluster="prod"})[` +
				dur + `:60s]))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expr, tt.got.Expr)
			assert.True(t, to.Equal(tt.got.Time), "evaluation time is the window end")
			assert.Contains(t, tt.got.Expr, dur)
			assert.Contains(t, tt.got.Expr, "60s")
		})
	}
}

func TestLastOverTime_Golden(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	const dur = "604800s"

	podMetrics := []string{
		"kube_pod_owner",
		"kube_replicaset_owner",
		"kube_job_owner",
		"kube_pod_info",
		"kube_pod_labels",
	}
	nodeMetrics := []string{"kube_node_labels", "kube_node_info"}

	for _, metric := range podMetrics {
		t.Run(metric, func(t *testing.T) {
			t.Parallel()
			plain := LastOverTime(metric, from, to, Selectors{})
			assert.Equal(t, "last_over_time("+metric+"["+dur+"])", plain.Expr)
			assert.True(t, to.Equal(plain.Time))

			selected := LastOverTime(metric, from, to, Selectors{Namespace: "payments", Cluster: "prod"})
			assert.Equal(t,
				"last_over_time("+metric+`{namespace="payments",cluster="prod"}[`+dur+"])",
				selected.Expr)
			assert.True(t, to.Equal(selected.Time))
		})
	}
	for _, metric := range nodeMetrics {
		t.Run(metric, func(t *testing.T) {
			t.Parallel()
			plain := LastOverTime(metric, from, to, Selectors{})
			assert.Equal(t, "last_over_time("+metric+"["+dur+"])", plain.Expr)
			assert.True(t, to.Equal(plain.Time))

			// Node series are not namespaced. A namespace selector is not copied
			// onto them. A selected cluster is.
			selected := LastOverTime(metric, from, to, Selectors{Namespace: "payments", Cluster: "prod"})
			assert.Equal(t, "last_over_time("+metric+`{cluster="prod"}[`+dur+"])", selected.Expr)
			assert.True(t, to.Equal(selected.Time))
			assert.NotContains(t, selected.Expr, "namespace=")
		})
	}
}

func TestDurationSeconds(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 9, 1, 0, 0, 30, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 2, 0, 0, time.UTC)
	require.Equal(t, 90, DurationSeconds(from, to))
}

func TestClusterDiscovery_Golden(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 7)
	got := ClusterDiscovery(from, to)
	assert.Equal(t,
		`group by (cluster) (last_over_time({__name__=~"kube_node_labels|kube_node_status_allocatable"}[604800s]))`,
		got.Expr)
	assert.True(t, to.Equal(got.Time), "evaluation time is the window end")
}
