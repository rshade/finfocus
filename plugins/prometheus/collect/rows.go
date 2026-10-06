package collect

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/prometheus/common/model"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/prometheus/identity"
)

// missingStepsOK is the largest gap, in 60s steps, that still counts as
// continuous. More than one missing step inside the series' own span is a hole.
const missingStepsOK = 1

type podKey struct {
	namespace string
	pod       string
	node      string
}

func addWorkloads(resp *pbc.GetStatsResponse, data vectors, opts Options) {
	owners := newOwnerIndex(data)
	labels := indexPodLabels(data.podLabels)
	nodes := indexPodNodes(data.podInfo)
	warned := make(map[string]bool)
	kept := make(map[string]bool)
	for _, key := range workloadKeys(data.cpu, data.mem, nodes) {
		podLabels := labels[key.namespace+"/"+key.pod]
		if !labelsMatch(podLabels, opts.PodLabels) {
			continue
		}
		kept[key.namespace+"/"+key.pod] = true
		subject := workloadSubject(key, podLabels, opts.Cluster)
		if ref, ok := owners.resolve(key.namespace, key.pod); ok {
			subject[pluginsdk.SubjectControllerKind] = ref.kind
			subject[pluginsdk.SubjectController] = ref.name
		} else if !warned[key.namespace+"/"+key.pod] {
			warned[key.namespace+"/"+key.pod] = true
			resp.Warnings = append(resp.Warnings,
				fmt.Sprintf("pod %s/%s: owner unknown", key.namespace, key.pod))
		}
		appendRow(resp, subject, data.cpu, podMatch(key), pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours)
		appendRow(resp, subject, data.mem, podMatch(key), pluginsdk.MetricMemUsage, pluginsdk.UnitGiBHours)
	}
	addGaps(resp, data.cpuGap, pluginsdk.MetricCPUUsage, kept)
	addGaps(resp, data.memGap, pluginsdk.MetricMemUsage, kept)
}

func workloadSubject(key podKey, labels map[string]string, cluster string) map[string]string {
	subject := map[string]string{
		pluginsdk.SubjectKind:      pluginsdk.KindWorkload,
		pluginsdk.SubjectNamespace: key.namespace,
		pluginsdk.SubjectPod:       key.pod,
		pluginsdk.SubjectNode:      key.node,
	}
	if cluster != "" {
		subject[pluginsdk.SubjectCluster] = cluster
	}
	for name, value := range labels {
		subject[pluginsdk.SubjectLabelPrefix+name] = value
	}
	return subject
}

func addNodes(resp *pbc.GetStatsResponse, cpu, mem model.Vector, cluster string) {
	names := map[string]struct{}{}
	collectNodeNames(names, cpu)
	collectNodeNames(names, mem)
	for _, name := range slices.Sorted(maps.Keys(names)) {
		subject := map[string]string{
			pluginsdk.SubjectKind: pluginsdk.KindNode,
			pluginsdk.SubjectNode: name,
		}
		if cluster != "" {
			subject[pluginsdk.SubjectCluster] = cluster
		}
		appendRow(resp, subject, cpu, nodeMatch(name), pluginsdk.MetricCPUAllocatable, pluginsdk.UnitCoreHours)
		appendRow(resp, subject, mem, nodeMatch(name), pluginsdk.MetricMemAllocatable, pluginsdk.UnitGiBHours)
	}
}

func collectNodeNames(names map[string]struct{}, vec model.Vector) {
	for _, sample := range vec {
		if node := string(sample.Metric["node"]); node != "" {
			names[node] = struct{}{}
		}
	}
}

func addGaps(resp *pbc.GetStatsResponse, vec model.Vector, metric string, kept map[string]bool) {
	for _, sample := range vec {
		if float64(sample.Value) <= missingStepsOK {
			continue
		}
		namespace := string(sample.Metric["namespace"])
		pod := string(sample.Metric["pod"])
		if !kept[namespace+"/"+pod] {
			continue
		}
		resp.Warnings = append(resp.Warnings,
			fmt.Sprintf("%s %s/%s: gap in %s", identity.IncompletePrefix, namespace, pod, metric))
	}
}

func workloadKeys(cpu, mem model.Vector, info map[string]string) []podKey {
	seen := make(map[podKey]bool)
	var keys []podKey
	for _, vec := range []model.Vector{cpu, mem} {
		for _, sample := range vec {
			key, ok := sampleKey(sample, info)
			if !ok || seen[key] {
				continue
			}
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func sampleKey(sample *model.Sample, info map[string]string) (podKey, bool) {
	namespace := string(sample.Metric["namespace"])
	pod := string(sample.Metric["pod"])
	if namespace == "" || pod == "" {
		return podKey{}, false
	}
	node := string(sample.Metric["node"])
	if node == "" {
		node = info[namespace+"/"+pod]
	}
	return podKey{namespace: namespace, pod: pod, node: node}, true
}

func appendRow(
	resp *pbc.GetStatsResponse,
	subject map[string]string,
	vec model.Vector,
	match func(model.Metric) bool,
	metric, unit string,
) {
	value, ok := valueFor(vec, match)
	if !ok {
		return
	}
	resp.Rows = append(resp.Rows, &pbc.UsageRow{
		Subject: maps.Clone(subject),
		Metric:  metric,
		Amount:  value,
		Unit:    unit,
	})
}

func podMatch(key podKey) func(model.Metric) bool {
	return func(m model.Metric) bool {
		return string(m["namespace"]) == key.namespace && string(m["pod"]) == key.pod && string(m["node"]) == key.node
	}
}

func nodeMatch(node string) func(model.Metric) bool {
	return func(m model.Metric) bool {
		return string(m["node"]) == node
	}
}

func valueFor(vec model.Vector, match func(model.Metric) bool) (float64, bool) {
	for _, sample := range vec {
		if !match(sample.Metric) {
			continue
		}
		value := float64(sample.Value)
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		return value, true
	}
	return 0, false
}
