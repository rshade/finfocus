package collect

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/rshade/finfocus/plugins/prometheus/promql"
)

const (
	queryCPU        = "cpu"
	queryMem        = "mem"
	queryCPUGap     = "cpu_gap"
	queryMemGap     = "mem_gap"
	queryPodOwner   = "pod_owner"
	queryRSOwner    = "rs_owner"
	queryJobOwner   = "job_owner"
	queryPodInfo    = "pod_info"
	queryPodLabels  = "pod_labels"
	queryCPUAlloc   = "cpu_alloc"
	queryMemAlloc   = "mem_alloc"
	queryNodeLabels = "node_labels"
	queryNodeInfo   = "node_info"
	queryStoreStart = "store_start"

	metricCPU    = "container_cpu_usage_seconds_total"
	metricMemory = "container_memory_working_set_bytes"
)

// vectors holds one instant-query result for each series the collector reads.
type vectors struct {
	cpu        model.Vector
	mem        model.Vector
	cpuGap     model.Vector
	memGap     model.Vector
	podOwner   model.Vector
	rsOwner    model.Vector
	jobOwner   model.Vector
	podInfo    model.Vector
	podLabels  model.Vector
	cpuAlloc   model.Vector
	memAlloc   model.Vector
	nodeLabels model.Vector
	nodeInfo   model.Vector
	storeStart model.Vector
}

func fetch(ctx context.Context, api v1.API, from, to time.Time, sel promql.Selectors) (vectors, error) {
	queries := []struct {
		name  string
		query promql.Query
	}{
		{queryCPU, promql.CPUUsage(from, to, sel)},
		{queryMem, promql.MemoryUsage(from, to, sel)},
		{queryCPUGap, promql.SeriesGap(metricCPU, from, to, sel)},
		{queryMemGap, promql.SeriesGap(metricMemory, from, to, sel)},
		{queryPodOwner, promql.LastOverTime("kube_pod_owner", from, to, sel)},
		{queryRSOwner, promql.LastOverTime("kube_replicaset_owner", from, to, sel)},
		{queryJobOwner, promql.LastOverTime("kube_job_owner", from, to, sel)},
		{queryPodInfo, promql.LastOverTime("kube_pod_info", from, to, sel)},
		{queryPodLabels, promql.LastOverTime("kube_pod_labels", from, to, sel)},
		{queryCPUAlloc, promql.CPUAllocatable(from, to, sel)},
		{queryMemAlloc, promql.MemoryAllocatable(from, to, sel)},
		{queryNodeLabels, promql.LastOverTime("kube_node_labels", from, to, sel)},
		{queryNodeInfo, promql.LastOverTime("kube_node_info", from, to, sel)},
		{queryStoreStart, promql.StoreStart(from, to, sel)},
	}
	got := make(map[string]model.Vector, len(queries))
	for _, item := range queries {
		vec, err := instant(ctx, api, item.query)
		if err != nil {
			return vectors{}, fmt.Errorf("query %s: %w", item.name, err)
		}
		got[item.name] = vec
	}
	return vectors{
		cpu: got[queryCPU], mem: got[queryMem], cpuGap: got[queryCPUGap], memGap: got[queryMemGap],
		podOwner: got[queryPodOwner], rsOwner: got[queryRSOwner], jobOwner: got[queryJobOwner],
		podInfo: got[queryPodInfo], podLabels: got[queryPodLabels],
		cpuAlloc: got[queryCPUAlloc], memAlloc: got[queryMemAlloc],
		nodeLabels: got[queryNodeLabels], nodeInfo: got[queryNodeInfo],
		storeStart: got[queryStoreStart],
	}, nil
}

func instant(ctx context.Context, api v1.API, query promql.Query) (model.Vector, error) {
	value, _, _, err := api.Query(ctx, query.Expr, query.Time)
	if err != nil {
		return nil, err
	}
	vec, ok := value.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("prometheus query returned %s", value.Type())
	}
	return vec, nil
}
