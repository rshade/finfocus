package kubernetes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	"github.com/rshade/finfocus/plugins/kubernetes/workload"
)

func fakeClusters(t *testing.T) ClusterFactory {
	t.Helper()
	return func(scope string) (*Cluster, error) {
		if scope == "missing" {
			return nil, status.Error(codes.InvalidArgument, `kubeconfig context "missing" not found`)
		}
		return &Cluster{Client: fake.NewSimpleClientset(), Context: "kind-test"}, nil
	}
}

func TestGetStats_RejectsHistorical(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		req  *pbc.GetStatsRequest
	}{
		{
			name: "start set",
			req:  &pbc.GetStatsRequest{Start: timestamppb.New(start)},
		},
		{
			name: "start and end with end after start",
			req: &pbc.GetStatsRequest{
				Start: timestamppb.New(start),
				End:   timestamppb.New(start.Add(24 * time.Hour)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(fakeClusters(t), Config{}).GetStats(context.Background(), tt.req)
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
			assert.Contains(t, err.Error(), "run-rate only")
		})
	}
}

func TestGetStats_UnknownContext(t *testing.T) {
	t.Parallel()

	_, err := New(fakeClusters(t), Config{}).GetStats(context.Background(), &pbc.GetStatsRequest{Scope: "missing"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetStats_ValidResponse(t *testing.T) {
	t.Parallel()

	resp, err := New(fakeClusters(t), Config{}).GetStats(context.Background(),
		&pbc.GetStatsRequest{Selector: map[string]string{"namespace": "a", "app": "web"}})
	require.NoError(t, err)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
}

// metricFilterPlugin returns a plugin over one priceable node with one
// running pod, so GetStats emits one row per served metric.
func metricFilterPlugin(t *testing.T) *Plugin {
	t.Helper()
	node := &corev1.Node{
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
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "prod"},
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
	cs := fake.NewSimpleClientset(node, pod)
	return New(func(string) (*Cluster, error) {
		return &Cluster{Client: cs, Context: "test-cluster"}, nil
	}, Config{})
}

func TestGetStats_MetricsFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		metrics       []string
		wantMetrics   []string
		wantWarnings  []string
		wantPriceable int
	}{
		{
			name:          "empty request returns the source defaults",
			metrics:       nil,
			wantMetrics:   []string{"cpu_request", "mem_request", "cpu_allocatable", "mem_allocatable"},
			wantPriceable: 1,
		},
		{
			name:          "single known metric filters rows",
			metrics:       []string{"cpu_request"},
			wantMetrics:   []string{"cpu_request"},
			wantPriceable: 1,
		},
		{
			name:          "node metrics keep the priceable node",
			metrics:       []string{"cpu_allocatable", "mem_allocatable"},
			wantMetrics:   []string{"cpu_allocatable", "mem_allocatable"},
			wantPriceable: 1,
		},
		{
			name:         "unknown metric warns and returns no rows",
			metrics:      []string{"gpu_seconds"},
			wantWarnings: []string{`unknown metric "gpu_seconds" ignored`},
		},
		{
			name:          "mixed known and unknown filters and warns",
			metrics:       []string{"mem_request", "gpu_seconds"},
			wantMetrics:   []string{"mem_request"},
			wantWarnings:  []string{`unknown metric "gpu_seconds" ignored`},
			wantPriceable: 1,
		},
		{
			name:         "duplicate unknown names warn once",
			metrics:      []string{"gpu_seconds", "gpu_seconds"},
			wantWarnings: []string{`unknown metric "gpu_seconds" ignored`},
		},
		{
			name:         "unimplemented usage metric warns as unserved",
			metrics:      []string{"cpu_usage"},
			wantWarnings: []string{`unknown metric "cpu_usage" ignored`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := metricFilterPlugin(t).GetStats(context.Background(),
				&pbc.GetStatsRequest{Metrics: tt.metrics})
			require.NoError(t, err)
			require.NoError(t, plugintesting.ValidateStatsResponse(resp))

			var got []string
			for _, row := range resp.GetRows() {
				got = append(got, row.GetMetric())
			}
			assert.ElementsMatch(t, tt.wantMetrics, got)
			assert.Equal(t, tt.wantWarnings, resp.GetWarnings())
			assert.Len(t, resp.GetPriceable(), tt.wantPriceable)
		})
	}
}

func TestGetStats_InvalidSelectorValue(t *testing.T) {
	t.Parallel()

	_, err := New(fakeClusters(t), Config{}).GetStats(context.Background(),
		&pbc.GetStatsRequest{Selector: map[string]string{"team": "a,other-label=x"}})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "team")
}

func TestLabelSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   map[string]string
		want    string
		wantErr bool
		errKey  string
	}{
		{
			name:  "nil map",
			input: nil,
			want:  "",
		},
		{
			name:  "empty map",
			input: map[string]string{},
			want:  "",
		},
		{
			name:  "valid multi-key selector is sorted",
			input: map[string]string{"tier": "backend", "app": "web"},
			want:  "app=web,tier=backend",
		},
		{
			name:    "value containing comma is rejected",
			input:   map[string]string{"team": "a,other-label=x"},
			wantErr: true,
			errKey:  "team",
		},
		{
			name:    "value containing equals is rejected",
			input:   map[string]string{"team": "a=b"},
			wantErr: true,
			errKey:  "team",
		},
		{
			name:    "invalid key is rejected",
			input:   map[string]string{"bad key!!": "v"},
			wantErr: true,
			errKey:  "bad key!!",
		},
		{
			name:    "over-long value is rejected",
			input:   map[string]string{"team": strings.Repeat("a", 64)},
			wantErr: true,
			errKey:  "team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := labelSelector(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errKey)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInfo_ExplicitCapabilitiesOnly(t *testing.T) {
	t.Parallel()

	info := Info("v0.1.0")
	assert.ElementsMatch(t, []pbc.PluginCapability{
		pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
		pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
		pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
	}, info.Capabilities, "projected cost is the only pricing capability")
}

func TestPlugin_ImplementsProviders(t *testing.T) {
	t.Parallel()

	var p any = New(fakeClusters(t), Config{})
	_, ok := p.(pluginsdk.UsageSourceProvider)
	assert.True(t, ok)
	_, ok = p.(pluginsdk.AllocatorProvider)
	assert.True(t, ok)
}

func TestAllocatorConformance(t *testing.T) {
	t.Parallel()

	plugintesting.RunAllocatorConformance(t, New(fakeClusters(t), Config{}))
}

// Hosts consult Supports before routing price queries. Since finfocus-spec
// v0.6.2 (#507) the SDK reaches SupportsProvider without a registry, so the
// plugin's own "false" is what hosts see; without it, `cost projected` would
// record a NotSupported error per resource.
func TestSupports_OptsOutOfPricingThroughSDK(t *testing.T) {
	t.Parallel()

	srv := pluginsdk.NewServerWithOptions(New(fakeClusters(t), Config{}), nil, nil, Info("v0.1.0"))
	for _, rt := range []string{"aws:ec2/instance:Instance", "ec2", "kubernetes:apps/v1:Deployment"} {
		resp, err := srv.Supports(context.Background(), &pbc.SupportsRequest{
			Resource: &pbc.ResourceDescriptor{ResourceType: rt},
		})
		require.NoError(t, err, rt)
		assert.False(t, resp.GetSupported(), rt)
	}
}

// noClusters fails the test if projected pricing ever reaches a cluster.
func noClusters(t *testing.T) ClusterFactory {
	t.Helper()
	return func(string) (*Cluster, error) {
		t.Error("projected pricing must not connect to a cluster")
		return nil, status.Error(codes.Internal, "no cluster in this test")
	}
}

func ratedConfig() Config {
	return LoadConfig(envOf(map[string]string{
		workload.EnvCPUHourlyRate:       "0.04",
		workload.EnvMemoryGiBHourlyRate: "0.005",
	}))
}

func deploymentDescriptor(t *testing.T) *pbc.ResourceDescriptor {
	t.Helper()
	attrs, err := structpb.NewStruct(map[string]any{"spec": map[string]any{
		"replicas": 3.0,
		"template": map[string]any{"spec": map[string]any{"containers": []any{
			map[string]any{"resources": map[string]any{
				"requests": map[string]any{"cpu": "500m", "memory": "1Gi"},
			}},
		}}},
	}})
	require.NoError(t, err)
	return &pbc.ResourceDescriptor{
		Id: "urn:web", Provider: "kubernetes", ResourceType: "kubernetes:apps/v1:Deployment", Attributes: attrs,
	}
}

func TestProjectedCost_PricesDeclaredDeployment(t *testing.T) {
	t.Parallel()

	p := New(noClusters(t), ratedConfig())
	desc := deploymentDescriptor(t)
	want := workload.Estimate(desc, ratedConfig())

	supports, err := p.Supports(context.Background(), &pbc.SupportsRequest{Resource: desc})
	require.NoError(t, err)
	assert.True(t, supports.GetSupported())
	assert.Empty(t, supports.GetReason())

	resp, err := p.GetProjectedCost(context.Background(), &pbc.GetProjectedCostRequest{Resource: desc})
	require.NoError(t, err)
	assert.InDelta(t, 54.75, resp.GetCostPerMonth(), 1e-9)
	assert.InDelta(t, want.PodHourly, resp.GetUnitPrice(), 1e-12)
	assert.Equal(t, "USD", resp.GetCurrency())
	assert.Equal(t, want.Note, resp.GetBillingDetail())
	require.NotNil(t, resp.GetExpiresAt(), "a config-derived price must not be cached by the host")
	assert.False(t, resp.GetExpiresAt().AsTime().After(time.Now()))
}

func TestProjectedCost_DeclinesWithoutRates(t *testing.T) {
	t.Parallel()

	p := New(noClusters(t), Config{})
	desc := deploymentDescriptor(t)
	want := workload.Estimate(desc, Config{})
	require.False(t, want.Priced())

	supports, err := p.Supports(context.Background(), &pbc.SupportsRequest{Resource: desc})
	require.NoError(t, err)
	assert.False(t, supports.GetSupported())
	assert.Equal(t, want.Reason, supports.GetReason())

	_, err = p.GetProjectedCost(context.Background(), &pbc.GetProjectedCostRequest{Resource: desc})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Contains(t, err.Error(), workload.EnvCPUHourlyRate)
}

func TestSupports_DeclinesOtherTypes(t *testing.T) {
	t.Parallel()

	p := New(noClusters(t), ratedConfig())
	for _, desc := range []*pbc.ResourceDescriptor{
		{Provider: "kubernetes", ResourceType: "kubernetes:core/v1:ConfigMap"},
		{Provider: "kubernetes", ResourceType: "kubernetes:core/v1:Service"},
		{Provider: "kubernetes", ResourceType: "kubernetes:apps/v1:ReplicaSet"},
		{Provider: "kubernetes", ResourceType: "kubernetes:apps/v1:DeploymentPatch"},
	} {
		resp, err := p.Supports(context.Background(), &pbc.SupportsRequest{Resource: desc})
		require.NoError(t, err)
		assert.False(t, resp.GetSupported(), desc.GetResourceType())
		assert.Equal(t, workload.ReasonUnsupportedKind(), resp.GetReason())
	}

	desc := &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:ec2/instance:Instance"}
	resp, err := p.Supports(context.Background(), &pbc.SupportsRequest{Resource: desc})
	require.NoError(t, err)
	assert.False(t, resp.GetSupported())
	assert.Equal(t, workload.ReasonWrongProvider, resp.GetReason())
}
