package collect

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
)

const controlPlaneNotEKS = "control plane omitted: API host is not an EKS cluster"

func TestCollect_ReplacedNodeUsesRecordedIdentity(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset()
	live.PrependReactor("get", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		assert.Fail(t, "live API must not be read when recorded identity is complete")
		return true, nil, errors.New("live API must not be read when recorded identity is complete")
	})
	resp, _, err := collectRaw(t, counterResetSamples, Options{Live: live, APIHost: "https://127.0.0.1:6443"})
	require.NoError(t, err)
	stampHistorical(resp)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertNodePriceable(t, resp, "node-a")
	assert.Equal(t, "m5.large", nodePriceable(t, resp, "node-a").GetSku())
	assert.NotContains(t, strings.Join(resp.GetWarnings(), "\n"), "cannot determine")
	assert.Contains(t, resp.GetWarnings(), controlPlaneNotEKS)
}

func TestCollect_MissingIdentityOmitsPriceable(t *testing.T) {
	t.Parallel()

	resp, _, err := collectRaw(t, missingIdentitySamples, Options{})
	require.NoError(t, err)
	stampHistorical(resp)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assert.Contains(t, resp.GetWarnings(),
		"incomplete: node node-x: cannot determine provider, instance type, or region")
	for _, item := range resp.GetPriceable() {
		assert.NotEqual(t, pluginsdk.KindNode, item.GetTags()[pluginsdk.SubjectKind])
	}
}

func TestCollect_MultipleClustersRequireScope(t *testing.T) {
	t.Parallel()

	_, queries, err := collectRaw(t, multiClusterSamples, Options{})
	requireClusterChoice(t, err, queries)
	assert.Contains(t, err.Error(), "east")
	assert.Contains(t, err.Error(), "west")
}

func TestCollect_ScopeOutsideStoredClusters(t *testing.T) {
	t.Parallel()

	_, queries, err := collectRaw(t, multiClusterSamples, Options{Cluster: "north"})
	requireClusterChoice(t, err, queries)
	assert.Contains(t, err.Error(), "north")
	assert.Contains(t, err.Error(), "east")
	assert.Contains(t, err.Error(), "west")
}

func TestCollect_SelectsMatchingCluster(t *testing.T) {
	t.Parallel()

	resp, queries, err := collectRaw(t, multiClusterSamples, Options{Cluster: "west"})
	require.NoError(t, err)
	stampHistorical(resp)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertDiscoveryThenFilter(t, queries, "west")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, 1)
	row := findRow(t, resp, pluginsdk.KindWorkload, "api", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "west", row.GetSubject()[pluginsdk.SubjectCluster])
	assert.Equal(t, "node-w", row.GetSubject()[pluginsdk.SubjectNode])
	assertNodePriceable(t, resp, "node-w")
	for _, item := range resp.GetPriceable() {
		assert.NotEqual(t, "node-e", item.GetId())
	}
}

func TestCollect_OneClusterLabelBecomesSubject(t *testing.T) {
	t.Parallel()

	resp, queries, err := collectRaw(t, eastClusterSamples, Options{})
	require.NoError(t, err)
	stampHistorical(resp)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	assertDiscoveryThenFilter(t, queries, "east")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, 1)
	row := findRow(t, resp, pluginsdk.KindNode, "node-e", pluginsdk.MetricCPUAllocatable)
	assert.Equal(t, "east", row.GetSubject()[pluginsdk.SubjectCluster])
	assertNodePriceable(t, resp, "node-e")
}

func TestCollect_ScopeWithoutClusterLabelDoesNotFilter(t *testing.T) {
	t.Parallel()

	resp, queries := collectFixture(t, counterResetSamples, Options{Cluster: "prod"})
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
	for _, query := range queries {
		assert.NotContains(t, query, "cluster=")
	}
	row := findRow(t, resp, pluginsdk.KindWorkload, "api", pluginsdk.MetricCPUUsage)
	assert.Equal(t, "prod", row.GetSubject()[pluginsdk.SubjectCluster])
	node := findRow(t, resp, pluginsdk.KindNode, "node-a", pluginsdk.MetricCPUAllocatable)
	assert.Equal(t, "prod", node.GetSubject()[pluginsdk.SubjectCluster])
}

// Without kube_node_labels the clusters are still discovered from capacity
// series, so the selected cluster's matcher is applied and the other cluster
// is not added in.
func TestCollect_DiscoversClustersWithoutNodeLabels(t *testing.T) {
	t.Parallel()

	noNodeLabels := func(query string) []fixtureSample {
		switch queryKind(query) {
		case "node_labels", "node_info":
			return nil
		case "cluster_discovery":
			return []fixtureSample{
				{labels: map[string]string{"cluster": "east"}, value: 1},
				{labels: map[string]string{"cluster": "west"}, value: 1},
			}
		default:
			return multiClusterSamples(query)
		}
	}
	resp, queries, err := collectRaw(t, noNodeLabels, Options{Cluster: "west"})
	require.NoError(t, err)
	assertDiscoveryThenFilter(t, queries, "west")
	assertUsage(t, resp, "api", pluginsdk.MetricCPUUsage, pluginsdk.UnitCoreHours, 1)
}

func requireClusterChoice(t *testing.T, err error, queries []string) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Len(t, queries, 1)
	assert.Contains(t, queries[0], "kube_node_labels")
	assert.NotContains(t, queries[0], "cluster=")
	assert.NotContains(t, queries[0], "container_cpu")
}

func assertDiscoveryThenFilter(t *testing.T, queries []string, cluster string) {
	t.Helper()
	require.GreaterOrEqual(t, len(queries), 2)
	assert.Contains(t, queries[0], "kube_node_labels")
	assert.NotContains(t, queries[0], "cluster=")
	matcher := `cluster="` + cluster + `"`
	for _, query := range queries[1:] {
		assert.Contains(t, query, matcher)
	}
}

func stampHistorical(resp *pbc.GetStatsResponse) {
	resp.Mode = pbc.StatsMode_STATS_MODE_HISTORICAL
}

func nodePriceable(t *testing.T, resp *pbc.GetStatsResponse, id string) *pbc.ResourceDescriptor {
	t.Helper()
	for _, item := range resp.GetPriceable() {
		if item.GetId() == id {
			return item
		}
	}
	require.Fail(t, "missing priceable", id)
	return nil
}

func missingIdentitySamples(query string) []fixtureSample {
	switch queryKind(query) {
	case "cpu_alloc":
		return []fixtureSample{{labels: map[string]string{"node": "node-x"}, value: nodeCoreHours}}
	case "mem_alloc":
		return []fixtureSample{{labels: map[string]string{"node": "node-x"}, value: nodeMemoryHours}}
	default:
		return nil
	}
}

func multiClusterSamples(query string) []fixtureSample {
	return clusterSamples(query, "west", "node-w", true)
}

func eastClusterSamples(query string) []fixtureSample {
	return clusterSamples(query, "east", "node-e", false)
}

// clusterSamples answers discovery with every cluster, and a filtered query
// with one cluster's hours. An unfiltered sum returns 5 so a missing matcher
// cannot pass an assertion that expects 1.
func clusterSamples(query, selected, node string, includeWest bool) []fixtureSample {
	unfiltered := !strings.Contains(query, "cluster=")
	matched := strings.Contains(query, `cluster="`+selected+`"`)
	switch queryKind(query) {
	case "cluster_discovery", "node_labels":
		return clusterLabelSamples(unfiltered, matched, selected, node, includeWest)
	case "node_info":
		return clusterInfoSamples(unfiltered, matched, selected, node, includeWest)
	case "cpu":
		return clusterUsage(unfiltered, matched, node, 1)
	case "mem":
		return clusterUsage(unfiltered, matched, node, 1)
	case "cpu_alloc":
		return clusterAlloc(unfiltered, matched, selected, node, nodeCoreHours)
	case "mem_alloc":
		return clusterAlloc(unfiltered, matched, selected, node, nodeMemoryHours)
	default:
		return nil
	}
}

func clusterLabelSamples(unfiltered, matched bool, selected, node string, includeWest bool) []fixtureSample {
	if unfiltered && includeWest {
		return []fixtureSample{
			{labels: withCluster(recordedNodeLabels("node-e"), "east"), value: 1},
			{labels: withCluster(recordedNodeLabels("node-w"), "west"), value: 1},
		}
	}
	if unfiltered || matched {
		return []fixtureSample{{labels: withCluster(recordedNodeLabels(node), selected), value: 1}}
	}
	return nil
}

func clusterInfoSamples(unfiltered, matched bool, selected, node string, includeWest bool) []fixtureSample {
	if unfiltered && includeWest {
		return []fixtureSample{
			{labels: withCluster(recordedNodeInfo("node-e"), "east"), value: 1},
			{labels: withCluster(recordedNodeInfo("node-w"), "west"), value: 1},
		}
	}
	if unfiltered || matched {
		return []fixtureSample{{labels: withCluster(recordedNodeInfo(node), selected), value: 1}}
	}
	return nil
}

func clusterUsage(unfiltered, matched bool, node string, filtered float64) []fixtureSample {
	if unfiltered {
		return []fixtureSample{podOnNode("payments", "api", "node-e", 5)}
	}
	if matched {
		return []fixtureSample{podOnNode("payments", "api", node, filtered)}
	}
	return nil
}

func clusterAlloc(unfiltered, matched bool, cluster, node string, value float64) []fixtureSample {
	if unfiltered {
		return []fixtureSample{{labels: map[string]string{"node": "node-e", "cluster": "east"}, value: value}}
	}
	if matched {
		return []fixtureSample{{labels: map[string]string{"node": node, "cluster": cluster}, value: value}}
	}
	return nil
}

func podOnNode(namespace, pod, node string, value float64) fixtureSample {
	return fixtureSample{
		labels: map[string]string{"namespace": namespace, "pod": pod, "node": node}, value: value,
	}
}

func withCluster(labels map[string]string, cluster string) map[string]string {
	out := maps.Clone(labels)
	out["cluster"] = cluster
	return out
}
