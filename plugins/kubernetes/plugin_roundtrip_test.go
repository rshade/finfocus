package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
)

// TestPlugin_GetStatsAllocateRoundTrip exercises the full plugin flow a host
// would drive: GetStats over a fake EKS cluster with two labeled AWS nodes and
// Deployment-owned pods, then Allocate over the priced resources GetStats
// reported. It is the plugin-level counterpart to allocate's unit tests,
// proving the collector's priceable tags (usage/nodes.go) and the allocator's
// priceable-tag reads (allocate/allocate.go) actually agree end to end --
// which final-review finding 1 found they did not for EKS control planes.
func TestPlugin_GetStatsAllocateRoundTrip(t *testing.T) {
	t.Parallel()

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{
			"node.kubernetes.io/instance-type": "m5.large",
			"topology.kubernetes.io/region":    "us-east-1",
		}},
		Spec: corev1.NodeSpec{ProviderID: "aws:///us-east-1a/i-node1"},
		Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("16Gi"),
		}},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: map[string]string{
			"node.kubernetes.io/instance-type": "m5.xlarge",
			"topology.kubernetes.io/region":    "us-east-1",
		}},
		Spec: corev1.NodeSpec{ProviderID: "aws:///us-east-1b/i-node2"},
		Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory
			corev1.ResourceCPU:    resource.MustParse("8"),
			corev1.ResourceMemory: resource.MustParse("32Gi"),
		}},
	}
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name: "web-rs", Namespace: "prod",
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "apps/v1", Kind: "Deployment", Name: "web", Controller: boolPtr(true)},
		},
	}}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-1", Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-rs", Controller: boolPtr(true)},
			},
		},
		Spec: corev1.PodSpec{NodeName: "node-1", Containers: []corev1.Container{{Name: "c",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory requests
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-2", Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-rs", Controller: boolPtr(true)},
			},
		},
		Spec: corev1.PodSpec{NodeName: "node-2", Containers: []corev1.Container{{Name: "c",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory requests
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cs := fake.NewSimpleClientset(node1, node2, pod1, pod2, rs)
	p := New(func(string) (*Cluster, error) {
		return &Cluster{
			Client: cs, Context: "test-cluster",
			Host: "https://ABC123.gr7.us-east-1.eks.amazonaws.com",
		}, nil
	}, Config{})

	st, err := p.GetStats(context.Background(), &pbc.GetStatsRequest{})
	require.NoError(t, err)
	require.NoError(t, plugintesting.ValidateStatsResponse(st))

	var priced []*pbc.PricedResource
	for _, d := range st.GetPriceable() {
		cost := 70.0
		if d.GetTags()["kind"] == "cluster" {
			cost = 73.0
		}
		priced = append(priced, &pbc.PricedResource{Resource: d, Cost: cost, Currency: "USD", Priced: true})
	}
	require.Len(t, priced, 3, "two nodes plus the EKS control plane")

	req := &pbc.AllocateRequest{Priced: priced, Usage: st.GetRows()}
	resp, err := p.Allocate(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateAllocateResponse(req, resp))
	require.NoError(t, pluginsdk.CheckConservation(req, resp, pluginsdk.DefaultConservationEpsilon))

	workloadNodes := map[string]string{}
	idleNodes := map[string]bool{}
	var clusterRow *pbc.AllocationRow
	for _, r := range resp.GetRows() {
		subj := r.GetSubject()
		switch subj["kind"] {
		case "workload":
			workloadNodes[subj["pod"]] = subj["node"]
		case "__idle__":
			idleNodes[subj["node"]] = true
		case "__cluster__":
			clusterRow = r
		}
	}

	assert.Equal(t, map[string]string{"web-1": "node-1", "web-2": "node-2"}, workloadNodes)
	assert.Equal(t, map[string]bool{"node-1": true, "node-2": true}, idleNodes, "exactly one idle row per node")
	require.NotNil(t, clusterRow, "expected an EKS control-plane row")
	assert.Empty(t, clusterRow.GetNote())
	assert.InDelta(t, 73.0, clusterRow.GetTotalCost(), 1e-9)
}

func boolPtr(b bool) *bool { return &b }

// TestPlugin_ProjectedCostRoundTrip serves the plugin over gRPC the way a host
// reaches it and prices a Deployment from its attributes.
func TestPlugin_ProjectedCostRoundTrip(t *testing.T) {
	t.Parallel()

	harness := plugintesting.NewTestHarness(
		pluginsdk.NewServerWithOptions(New(noClusters(t), ratedConfig()), nil, nil, Info("v0.1.0")))
	harness.Start(t)
	t.Cleanup(harness.Stop)
	desc := deploymentDescriptor(t)

	supports, err := harness.Client().Supports(context.Background(), &pbc.SupportsRequest{Resource: desc})
	require.NoError(t, err)
	assert.True(t, supports.GetSupported(), supports.GetReason())

	resp, err := harness.Client().GetProjectedCost(context.Background(), &pbc.GetProjectedCostRequest{Resource: desc})
	require.NoError(t, err)
	assert.InDelta(t, 54.75, resp.GetCostPerMonth(), 1e-9)
	assert.Equal(t, "USD", resp.GetCurrency())
	assert.Contains(t, resp.GetBillingDetail(), "not a real node price")
}
