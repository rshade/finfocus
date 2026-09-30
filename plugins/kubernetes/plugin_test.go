package kubernetes

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
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
	p := New(fakeClusters(t))
	_, err := p.GetStats(context.Background(), &pbc.GetStatsRequest{Start: timestamppb.Now()})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "run-rate only")
}

func TestGetStats_UnknownContext(t *testing.T) {
	_, err := New(fakeClusters(t)).GetStats(context.Background(), &pbc.GetStatsRequest{Scope: "missing"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetStats_ValidResponse(t *testing.T) {
	resp, err := New(fakeClusters(t)).GetStats(context.Background(),
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
	})
}

func TestGetStats_MetricsFilter(t *testing.T) {
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
	_, err := New(fakeClusters(t)).GetStats(context.Background(),
		&pbc.GetStatsRequest{Selector: map[string]string{"team": "a,other-label=x"}})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "team")
}

func TestLabelSelector(t *testing.T) {
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
	info := Info("v0.1.0")
	assert.ElementsMatch(t, []pbc.PluginCapability{
		pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
		pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
	}, info.Capabilities)
}

func TestPlugin_ImplementsProviders(t *testing.T) {
	var p any = New(fakeClusters(t))
	_, ok := p.(pluginsdk.UsageSourceProvider)
	assert.True(t, ok)
	_, ok = p.(pluginsdk.AllocatorProvider)
	assert.True(t, ok)
}

func TestAllocatorConformance(t *testing.T) {
	plugintesting.RunAllocatorConformance(t, New(fakeClusters(t)))
}

// Hosts consult Supports before routing price queries. Since finfocus-spec
// v0.6.2 (#507) the SDK reaches SupportsProvider without a registry, so the
// plugin's own "false" is what hosts see; without it, `cost projected` would
// record a NotSupported error per resource.
func TestSupports_OptsOutOfPricingThroughSDK(t *testing.T) {
	srv := pluginsdk.NewServerWithOptions(New(fakeClusters(t)), nil, nil, Info("v0.1.0"))
	for _, rt := range []string{"aws:ec2/instance:Instance", "ec2", "kubernetes:apps/v1:Deployment"} {
		resp, err := srv.Supports(context.Background(), &pbc.SupportsRequest{
			Resource: &pbc.ResourceDescriptor{ResourceType: rt},
		})
		require.NoError(t, err, rt)
		assert.False(t, resp.GetSupported(), rt)
	}
}
