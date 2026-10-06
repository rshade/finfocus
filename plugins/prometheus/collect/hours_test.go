package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	"github.com/rshade/finfocus/plugins/prometheus/identity"
)

// controlPlaneDisconnected is the warning when Collect has no live client.
// identity keeps the same text unexported.
const controlPlaneDisconnected = "control plane omitted: live API is not connected"

// Checked hours for a 2 hour window at a 60s step.
//
// Counter reset: 60 samples at 1 core and 60 samples at 2 cores.
// sum_over_time(rate) * 60 / 3600 = 3 core-hours.
// last(counter)-first(counter) is 2 core-hours and drops the hour before the reset.
//
// Partial lifetime: samples exist for one hour. The same rate over the full
// window would be twice the hours. A late start is not a hole.
//
// Node allocatable: 4 cores and 16 GiB present for both hours, so 8 core-hours
// and 32 GiB-hours. A point read of the gauge is 4 and 16.
const (
	resetCoreHours      = 3.0
	resetDroppedHours   = 2.0
	resetMemoryHours    = 3.0
	partialCoreHours    = 1.0
	partialFullHours    = 2.0
	partialMemoryHours  = 2.0
	partialMemoryFull   = 4.0
	gappedCoreHours     = 1.5
	edgeCoreHours       = 0.25
	namespaceCoreHours  = 1.25
	namespaceMemHours   = 0.5
	otherNamespaceHours = 9.0
	labelCoreHours      = 0.5
	labelMemHours       = 0.25
	webCoreHours        = 4.0
	nodeCoreHours       = 8.0
	nodeMemoryHours     = 32.0
	nodeCorePoint       = 4.0
	nodeMemoryPoint     = 16.0
	holeSteps           = 3.0
	edgeSteps           = 1.0
)

func TestCollect_CounterResetSumsBothSides(t *testing.T) {
	t.Parallel()

	resp, queries := collectFixture(t, counterResetSamples, Options{})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertPods(t, resp, "api")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, resetCoreHours)
	assertUsage(t, resp, "api", pluginsdk.MetricMemUsage, pluginsdk.UnitGiBHours, resetMemoryHours)
	assertNode(t, resp)
	assertNoRequestMetrics(t, resp)
	assert.Equal(t, []string{controlPlaneDisconnected}, resp.GetWarnings())

	row := findRow(t, resp, pluginsdk.KindWorkload, "api", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "Deployment", row.GetSubject()[pluginsdk.SubjectControllerKind])
	assert.Equal(t, "api", row.GetSubject()[pluginsdk.SubjectController])
	assert.Equal(t, "checkout", row.GetSubject()[pluginsdk.SubjectLabelPrefix+"app"])
	assert.Equal(t, "payments", row.GetSubject()[pluginsdk.SubjectNamespace])
	assert.Equal(t, "node-a", row.GetSubject()[pluginsdk.SubjectNode])

	cpu := queryText(t, queries, "cpu")
	assert.Contains(t, cpu, "sum_over_time")
	assert.Contains(t, cpu, "rate(")
	assert.NotContains(t, cpu, "increase(")
}

func TestCollect_PartialLifetimeCountsExistingSamples(t *testing.T) {
	t.Parallel()

	resp, queries := collectFixture(t, partialSamples, Options{})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertPods(t, resp, "late")
	assertUsage(t, resp, "late", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, partialCoreHours)
	assertUsage(t, resp, "late", pluginsdk.MetricMemUsage, pluginsdk.UnitGiBHours, partialMemoryHours)
	assert.NotContains(t, strings.Join(resp.GetWarnings(), "\n"), "incomplete:")
	assert.Equal(t, []string{"pod payments/late: owner unknown", controlPlaneDisconnected}, resp.GetWarnings())
	assert.Empty(t, findRow(t, resp, pluginsdk.KindWorkload, "late", pluginsdk.MetricCPUUsage).
		GetSubject()[pluginsdk.SubjectControllerKind])
	assertQuerySent(t, queries, "count_over_time")
}

func TestCollect_HoleInsideSeriesSpan(t *testing.T) {
	t.Parallel()

	resp, queries := collectFixture(t, holeSamples, Options{})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertUsage(t, resp, "gapped", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, gappedCoreHours)
	assertUsage(t, resp, "edge", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, edgeCoreHours)
	assert.Equal(t, []string{
		"incomplete: payments/gapped: gap in cpu_usage", controlPlaneDisconnected,
	}, resp.GetWarnings())
	assertQuerySent(t, queries, "count_over_time")
}

func TestCollect_NamespaceMatcherLimitsRows(t *testing.T) {
	t.Parallel()

	resp, queries := collectFixture(t, namespaceSamples, Options{Namespace: "payments"})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertPods(t, resp, "api")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, namespaceCoreHours)
	assertUsage(t, resp, "api", pluginsdk.MetricMemUsage, pluginsdk.UnitGiBHours, namespaceMemHours)
	assertQuerySent(t, queries, `namespace="payments"`)
}

func TestCollect_LabelSelectorAppliedInGo(t *testing.T) {
	t.Parallel()

	resp, queries := collectFixture(t, labelSamples, Options{PodLabels: map[string]string{"app": "api"}})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertPods(t, resp, "api")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, labelCoreHours)
	assertUsage(t, resp, "api", pluginsdk.MetricMemUsage, pluginsdk.UnitGiBHours, labelMemHours)
	for _, q := range queries {
		assert.NotContains(t, q, "app=")
	}
}

func TestCollect_EmptyStore(t *testing.T) {
	t.Parallel()

	resp, _ := collectFixture(t, emptySamples, Options{})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertPods(t, resp)
	assertNode(t, resp)
	assert.Equal(t, []string{controlPlaneDisconnected}, resp.GetWarnings())
	assertNoRequestMetrics(t, resp)
}

func TestCollect_JoinsOwners(t *testing.T) {
	t.Parallel()

	resp, _ := collectFixture(t, ownerSamples, Options{})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))

	deploy := findRow(t, resp, pluginsdk.KindWorkload, "api", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "Deployment", deploy.GetSubject()[pluginsdk.SubjectControllerKind])
	assert.Equal(t, "api", deploy.GetSubject()[pluginsdk.SubjectController])

	cron := findRow(t, resp, pluginsdk.KindWorkload, "nightly", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "CronJob", cron.GetSubject()[pluginsdk.SubjectControllerKind])
	assert.Equal(t, "nightly", cron.GetSubject()[pluginsdk.SubjectController])

	bare := findRow(t, resp, pluginsdk.KindWorkload, "db", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "StatefulSet", bare.GetSubject()[pluginsdk.SubjectControllerKind])
	assert.Equal(t, "db", bare.GetSubject()[pluginsdk.SubjectController])

	missing := findRow(t, resp, pluginsdk.KindWorkload, "orphan", pluginsdk.MetricCPUUsage)
	assert.Empty(t, missing.GetSubject()[pluginsdk.SubjectControllerKind])
	assert.Empty(t, missing.GetSubject()[pluginsdk.SubjectController])
	assert.Contains(t, resp.GetWarnings(), "pod payments/orphan: owner unknown")
	for _, warning := range resp.GetWarnings() {
		assert.False(t, strings.HasPrefix(warning, "incomplete:"), warning)
	}
}

func TestCollect_ContextCanceled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			writeVector(w, nil)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := api.NewClient(api.Config{Address: srv.URL})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	from, to := hourWindow()
	_, err = Collect(ctx, v1.NewAPI(client), from, to, Options{})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCollect_RetentionShorterThanWindow(t *testing.T) {
	t.Parallel()

	from, _ := hourWindow()
	retentionWarning := "incomplete: stored data starts at 2026-09-28T00:30:00Z, after the window start " +
		"2026-09-28T00:00:00Z; Prometheus retention may be shorter than the window"
	tests := []struct {
		name  string
		start *time.Time
		want  []string
	}{
		{
			name:  "data starts inside the window",
			start: new(from.Add(30 * time.Minute)),
			want:  []string{retentionWarning, controlPlaneDisconnected},
		},
		{
			name:  "data starts within one step of the window start",
			start: new(from.Add(time.Minute)),
			want:  []string{controlPlaneDisconnected},
		},
		{
			name:  "data starts two steps after the window start",
			start: new(from.Add(2 * time.Minute)),
			want: []string{
				"incomplete: stored data starts at 2026-09-28T00:02:00Z, after the window start " +
					"2026-09-28T00:00:00Z; Prometheus retention may be shorter than the window",
				controlPlaneDisconnected,
			},
		},
		{
			name: "no node series",
			want: []string{controlPlaneDisconnected},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, queries := collectFixture(t, retentionSamples(tt.start), Options{})
			require.NoError(t, plugintesting.ValidateStatsResponse(resp))
			assert.Equal(t, tt.want, resp.GetWarnings())
			assertQuerySent(t, queries, "min_over_time(timestamp(kube_node_status_allocatable")
		})
	}
}

func TestCollect_NodelessUsageUsesPodInfoNode(t *testing.T) {
	t.Parallel()

	resp, _ := collectFixture(t, nodelessSamples, Options{})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertPods(t, resp, "api")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, 1.5)
	assertUsage(t, resp, "api", pluginsdk.MetricMemUsage, pluginsdk.UnitGiBHours, 2.5)
	row := findRow(t, resp, pluginsdk.KindWorkload, "api", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "node-a", row.GetSubject()[pluginsdk.SubjectNode])
}

func hourWindow() (time.Time, time.Time) {
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	return from, from.Add(2 * time.Hour)
}

type fixtureSample struct {
	labels map[string]string
	value  float64
}

type recorded struct {
	mu      sync.Mutex
	queries []string
	times   []string
}

func collectFixture(
	t *testing.T, respond func(string) []fixtureSample, opts Options,
) (*pbc.GetStatsResponse, []string) {
	t.Helper()
	resp, queries, err := collectRaw(t, respond, opts)
	require.NoError(t, err)
	// Collect leaves mode unset. The plugin stamps historical before validation.
	resp.Mode = pbc.StatsMode_STATS_MODE_HISTORICAL
	assertNodePriceable(t, resp, "node-a")
	return resp, queries
}

func collectRaw(
	t *testing.T, respond func(string) []fixtureSample, opts Options,
) (*pbc.GetStatsResponse, []string, error) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		query := r.Form.Get("query")
		rec.mu.Lock()
		rec.queries = append(rec.queries, query)
		rec.times = append(rec.times, r.Form.Get("time"))
		rec.mu.Unlock()
		writeVector(w, respond(query))
	}))
	t.Cleanup(srv.Close)

	client, err := api.NewClient(api.Config{Address: srv.URL})
	require.NoError(t, err)
	from, to := hourWindow()
	resp, err := Collect(context.Background(), v1.NewAPI(client), from, to, opts)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	wantTime := strconv.FormatInt(to.Unix(), 10)
	require.NotEmpty(t, rec.queries)
	for _, got := range rec.times {
		assert.Equal(t, wantTime, got, "evaluation time is the window end")
	}
	return resp, append([]string(nil), rec.queries...), err
}

func writeVector(w http.ResponseWriter, samples []fixtureSample) {
	type pair struct {
		Metric map[string]string `json:"metric"`
		Value  []any             `json:"value"`
	}
	result := make([]pair, len(samples))
	for i, sample := range samples {
		result[i] = pair{
			Metric: sample.labels,
			Value:  []any{time.Now().Unix(), strconv.FormatFloat(sample.value, 'f', -1, 64)},
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "success",
		"data":   map[string]any{"resultType": "vector", "result": result},
	})
}

func queryKind(query string) string {
	switch {
	case strings.Contains(query, "group by (cluster)"):
		return "cluster_discovery"
	case strings.Contains(query, "min_over_time(timestamp(kube_node_status_allocatable"):
		return "store_start"
	case strings.Contains(query, "count_over_time"), strings.Contains(query, "timestamp("):
		if strings.Contains(query, "container_memory_working_set_bytes") {
			return "mem_gap"
		}
		return "cpu_gap"
	case strings.Contains(query, "container_cpu_usage_seconds_total"):
		return "cpu"
	case strings.Contains(query, "container_memory_working_set_bytes"):
		return "mem"
	case strings.Contains(query, "kube_node_status_allocatable") && strings.Contains(query, `resource="memory"`):
		return "mem_alloc"
	case strings.Contains(query, "kube_node_status_allocatable"):
		return "cpu_alloc"
	case strings.Contains(query, "kube_replicaset_owner"):
		return "rs_owner"
	case strings.Contains(query, "kube_job_owner"):
		return "job_owner"
	case strings.Contains(query, "kube_pod_owner"):
		return "pod_owner"
	case strings.Contains(query, "kube_pod_labels"):
		return "pod_labels"
	case strings.Contains(query, "kube_pod_info"):
		return "pod_info"
	case strings.Contains(query, "kube_node_labels"):
		return "node_labels"
	case strings.Contains(query, "kube_node_info"):
		return "node_info"
	default:
		return "other"
	}
}

func nodeSeries(query string) ([]fixtureSample, bool) {
	switch queryKind(query) {
	case "cpu_alloc":
		value := nodeCorePoint
		if strings.Contains(query, "sum_over_time") {
			value = nodeCoreHours
		}
		return []fixtureSample{{labels: map[string]string{"node": "node-a"}, value: value}}, true
	case "mem_alloc":
		value := nodeMemoryPoint
		if strings.Contains(query, "sum_over_time") {
			value = nodeMemoryHours
		}
		return []fixtureSample{{labels: map[string]string{"node": "node-a"}, value: value}}, true
	case "node_labels":
		return []fixtureSample{{labels: recordedNodeLabels("node-a"), value: 1}}, true
	case "node_info":
		return []fixtureSample{{labels: recordedNodeInfo("node-a"), value: 1}}, true
	default:
		return nil, false
	}
}

func recordedNodeLabels(node string) map[string]string {
	return map[string]string{
		"node": node,
		identity.PrometheusLabelKey("node.kubernetes.io/instance-type"): "m5.large",
		identity.PrometheusLabelKey("topology.kubernetes.io/region"):    "us-east-1",
		identity.PrometheusLabelKey("finfocus.dev/provider"):            "aws",
	}
}

func recordedNodeInfo(node string) map[string]string {
	return map[string]string{
		"node":        node,
		"provider_id": "aws:///us-east-1a/i-" + node,
	}
}

func integrated(query string, summed, point float64) float64 {
	if strings.Contains(query, "sum_over_time") {
		return summed
	}
	return point
}

func counterResetSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	switch queryKind(query) {
	case "cpu":
		value := resetDroppedHours
		if strings.Contains(query, "sum_over_time") && strings.Contains(query, "rate(") &&
			!strings.Contains(query, "increase(") {
			value = resetCoreHours
		}
		return []fixtureSample{podSample("payments", "api", value)}
	case "mem":
		return []fixtureSample{podSample("payments", "api", integrated(query, resetMemoryHours, resetDroppedHours))}
	case "pod_owner":
		return []fixtureSample{ownerSample("api", "ReplicaSet", "api-7d")}
	case "rs_owner":
		return []fixtureSample{replicaOwner("payments", "api-7d", "Deployment", "api")}
	case "pod_labels":
		return []fixtureSample{labelSample("payments", "api", "app", "checkout")}
	default:
		return nil
	}
}

func partialSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	switch queryKind(query) {
	case "cpu":
		return []fixtureSample{podSample("payments", "late", integrated(query, partialCoreHours, partialFullHours))}
	case "mem":
		return []fixtureSample{podSample("payments", "late", integrated(query, partialMemoryHours, partialMemoryFull))}
	case "cpu_gap", "mem_gap":
		return []fixtureSample{podSample("payments", "late", 0)}
	default:
		return nil
	}
}

func holeSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	switch queryKind(query) {
	case "cpu":
		return []fixtureSample{
			podSample("payments", "gapped", gappedCoreHours),
			podSample("payments", "edge", edgeCoreHours),
		}
	case "mem":
		return []fixtureSample{
			podSample("payments", "gapped", edgeCoreHours),
			podSample("payments", "edge", edgeCoreHours),
		}
	case "cpu_gap":
		return []fixtureSample{
			podSample("payments", "gapped", holeSteps),
			podSample("payments", "edge", edgeSteps),
		}
	case "pod_owner":
		return []fixtureSample{
			ownerSample("gapped", "StatefulSet", "gapped"),
			ownerSample("edge", "StatefulSet", "edge"),
		}
	default:
		return nil
	}
}

func namespaceSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	limited := strings.Contains(query, `namespace="payments"`)
	switch queryKind(query) {
	case "cpu":
		rows := []fixtureSample{podSample("payments", "api", namespaceCoreHours)}
		if !limited {
			rows = append(rows, podSample("default", "other", otherNamespaceHours))
		}
		return rows
	case "mem":
		rows := []fixtureSample{podSample("payments", "api", namespaceMemHours)}
		if !limited {
			rows = append(rows, podSample("default", "other", otherNamespaceHours))
		}
		return rows
	case "pod_owner":
		return []fixtureSample{ownerSample("api", "ReplicaSet", "api-7d")}
	case "rs_owner":
		return []fixtureSample{replicaOwner("payments", "api-7d", "Deployment", "api")}
	default:
		return nil
	}
}

func labelSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	switch queryKind(query) {
	case "cpu":
		return []fixtureSample{
			podSample("payments", "api", labelCoreHours),
			podSample("payments", "web", webCoreHours),
		}
	case "mem":
		return []fixtureSample{
			podSample("payments", "api", labelMemHours),
			podSample("payments", "web", webCoreHours),
		}
	case "pod_labels":
		return []fixtureSample{
			labelSample("payments", "api", "app", "api"),
			labelSample("payments", "web", "app", "web"),
		}
	case "pod_owner":
		return []fixtureSample{
			ownerSample("api", "Deployment", "api"),
			ownerSample("web", "Deployment", "web"),
		}
	default:
		return nil
	}
}

func retentionSamples(start *time.Time) func(string) []fixtureSample {
	return func(query string) []fixtureSample {
		if queryKind(query) == "store_start" && start != nil {
			return []fixtureSample{{labels: map[string]string{}, value: float64(start.Unix())}}
		}
		return emptySamples(query)
	}
}

// nodelessSamples is cAdvisor scraped from the kubelet without a node label.
// kube_pod_info supplies the node.
func nodelessSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	nodeless := func(value float64) []fixtureSample {
		return []fixtureSample{{labels: map[string]string{"namespace": "payments", "pod": "api"}, value: value}}
	}
	switch queryKind(query) {
	case "cpu":
		return nodeless(1.5)
	case "mem":
		return nodeless(2.5)
	case "pod_info":
		return []fixtureSample{
			{labels: map[string]string{"namespace": "payments", "pod": "api", "node": "node-a"}, value: 1},
		}
	default:
		return nil
	}
}

func emptySamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	return nil
}

func ownerSamples(query string) []fixtureSample {
	if rows, ok := nodeSeries(query); ok {
		return rows
	}
	switch queryKind(query) {
	case "cpu", "mem":
		return []fixtureSample{
			podSample("payments", "api", 1),
			podSample("payments", "nightly", 1),
			podSample("payments", "db", 1),
			podSample("payments", "orphan", 1),
		}
	case "pod_owner":
		return []fixtureSample{
			ownerSample("api", "ReplicaSet", "api-7d"),
			ownerSample("nightly", "Job", "nightly-28421"),
			ownerSample("db", "StatefulSet", "db"),
		}
	case "rs_owner":
		return []fixtureSample{replicaOwner("payments", "api-7d", "Deployment", "api")}
	case "job_owner":
		return []fixtureSample{jobOwnerSample("payments", "nightly-28421", "CronJob", "nightly")}
	default:
		return nil
	}
}

func podSample(namespace, pod string, value float64) fixtureSample {
	return fixtureSample{labels: map[string]string{"namespace": namespace, "pod": pod, "node": "node-a"}, value: value}
}

func ownerSample(pod, kind, name string) fixtureSample {
	return fixtureSample{labels: map[string]string{
		"namespace": "payments", "pod": pod,
		"owner_kind": kind, "owner_name": name, "owner_is_controller": "true",
	}, value: 1}
}

func replicaOwner(namespace, replicaSet, kind, name string) fixtureSample {
	return fixtureSample{labels: map[string]string{
		"namespace": namespace, "replicaset": replicaSet,
		"owner_kind": kind, "owner_name": name, "owner_is_controller": "true",
	}, value: 1}
}

func jobOwnerSample(namespace, job, kind, name string) fixtureSample {
	return fixtureSample{labels: map[string]string{
		"namespace": namespace, "job_name": job,
		"owner_kind": kind, "owner_name": name, "owner_is_controller": "true",
	}, value: 1}
}

func labelSample(namespace, pod, key, value string) fixtureSample {
	return fixtureSample{labels: map[string]string{
		"namespace": namespace, "pod": pod, "label_" + key: value,
	}, value: 1}
}

func assertUsage(t *testing.T, resp *pbc.GetStatsResponse, pod, metric, unit string, amount float64) {
	t.Helper()
	row := findRow(t, resp, pluginsdk.KindWorkload, pod, metric)
	assert.InDelta(t, amount, row.GetAmount(), 1e-9)
	assert.Equal(t, unit, row.GetUnit())
	assert.Equal(t, pluginsdk.KindWorkload, row.GetSubject()[pluginsdk.SubjectKind])
}

func assertNodePriceable(t *testing.T, resp *pbc.GetStatsResponse, node string) {
	t.Helper()
	var found *pbc.ResourceDescriptor
	for _, item := range resp.GetPriceable() {
		if item.GetTags()[pluginsdk.SubjectKind] != pluginsdk.KindNode {
			continue
		}
		require.Nil(t, found, "duplicate node priceable %s", item.GetId())
		found = item
	}
	require.NotNil(t, found, "missing priceable %s", node)
	assert.Equal(t, node, found.GetId())
	row := findRow(t, resp, pluginsdk.KindNode, node, pluginsdk.MetricCPUAllocatable)
	assert.Equal(t, row.GetSubject()[pluginsdk.SubjectNode], found.GetId())
}

func assertNode(t *testing.T, resp *pbc.GetStatsResponse) {
	t.Helper()
	cpu := findRow(t, resp, pluginsdk.KindNode, "node-a", pluginsdk.MetricCPUAllocatable)
	mem := findRow(t, resp, pluginsdk.KindNode, "node-a", pluginsdk.MetricMemAllocatable)
	assert.InDelta(t, nodeCoreHours, cpu.GetAmount(), 1e-9)
	assert.Equal(t, pluginsdk.UnitCoreHours, cpu.GetUnit())
	assert.InDelta(t, nodeMemoryHours, mem.GetAmount(), 1e-9)
	assert.Equal(t, pluginsdk.UnitGiBHours, mem.GetUnit())
	assert.Empty(t, cpu.GetSubject()[pluginsdk.SubjectNamespace])
}

func assertPods(t *testing.T, resp *pbc.GetStatsResponse, want ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, row := range resp.GetRows() {
		if row.GetSubject()[pluginsdk.SubjectKind] != pluginsdk.KindWorkload {
			continue
		}
		got[row.GetSubject()[pluginsdk.SubjectPod]] = true
	}
	assert.Len(t, got, len(want))
	for _, pod := range want {
		assert.True(t, got[pod], "missing pod %s", pod)
	}
}

func assertNoRequestMetrics(t *testing.T, resp *pbc.GetStatsResponse) {
	t.Helper()
	for _, row := range resp.GetRows() {
		assert.NotEqual(t, pluginsdk.MetricCPURequest, row.GetMetric())
		assert.NotEqual(t, pluginsdk.MetricMemRequest, row.GetMetric())
	}
}

func findRow(t *testing.T, resp *pbc.GetStatsResponse, kind, name, metric string) *pbc.UsageRow {
	t.Helper()
	nameKey := pluginsdk.SubjectPod
	if kind == pluginsdk.KindNode {
		nameKey = pluginsdk.SubjectNode
	}
	var found *pbc.UsageRow
	for _, row := range resp.GetRows() {
		if row.GetSubject()[pluginsdk.SubjectKind] != kind || row.GetSubject()[nameKey] != name ||
			row.GetMetric() != metric {
			continue
		}
		require.Nil(t, found, "duplicate %s %s %s", kind, name, metric)
		found = row
	}
	require.NotNil(t, found, "missing %s %s %s", kind, name, metric)
	return found
}

func queryText(t *testing.T, queries []string, kind string) string {
	t.Helper()
	for _, query := range queries {
		if queryKind(query) == kind {
			return query
		}
	}
	require.Fail(t, "query not sent", kind)
	return ""
}

func assertQuerySent(t *testing.T, queries []string, fragment string) {
	t.Helper()
	for _, query := range queries {
		if strings.Contains(query, fragment) {
			return
		}
	}
	require.Fail(t, "query fragment not sent", fragment)
}
