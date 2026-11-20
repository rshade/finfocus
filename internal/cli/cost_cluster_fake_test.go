package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const clusterNodeType = "aws:ec2/instance:Instance"

// fakeClusterPlugin serves UsageSourceService and AllocatorService in process,
// as the kubernetes plugin does. GetStats reports nodes n1 and n2, plus n3
// when unpriced is set, and rejects the scope "missing" as a user error.
// Allocate accepts only the policy field "idle" with value "share" or
// "separate", and splits each priced node 60/40 into a workload row in the
// node's namespace and an idle row, so rows conserve the priced total.
type fakeClusterPlugin struct {
	pbc.UnimplementedUsageSourceServiceServer
	pbc.UnimplementedAllocatorServiceServer

	warnings []string
	unpriced bool

	mu         sync.Mutex
	statsCalls int
}

func (f *fakeClusterPlugin) GetStats(_ context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error) {
	f.mu.Lock()
	f.statsCalls++
	f.mu.Unlock()
	if req.GetScope() == "missing" {
		return nil, status.Error(codes.InvalidArgument, `context "missing" not found`)
	}
	mode := pbc.StatsMode_STATS_MODE_RUN_RATE
	if req.GetStart() != nil {
		mode = pbc.StatsMode_STATS_MODE_HISTORICAL
	}
	nodes := []string{"n1", "n2"}
	if f.unpriced {
		nodes = append(nodes, "n3")
	}
	resp := &pbc.GetStatsResponse{Mode: mode, Warnings: f.warnings}
	for _, id := range nodes {
		resp.Priceable = append(resp.Priceable, &pbc.ResourceDescriptor{
			Provider: "aws", ResourceType: clusterNodeType, Sku: "m5.large", Region: "us-east-1", Id: id,
			Tags: map[string]string{"kind": "node"},
		})
	}
	return resp, nil
}

func (f *fakeClusterPlugin) Allocate(_ context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
	policy := req.GetPolicyJson()
	if len(policy) == 0 {
		policy = []byte(`{"idle":"share"}`)
	}
	var fields map[string]any
	if err := json.Unmarshal(policy, &fields); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "allocation policy: %v", err)
	}
	for key, value := range fields {
		if key != "idle" {
			return nil, status.Errorf(codes.InvalidArgument, "allocation policy: unknown field %q", key)
		}
		if value != "share" && value != "separate" {
			return nil, status.Errorf(codes.InvalidArgument, "idle: unsupported value %q", value)
		}
	}
	resp := &pbc.AllocateResponse{EffectivePolicyJson: policy, PolicyDigest: "0123456789abcdef0123"}
	for _, p := range req.GetPriced() {
		if !p.GetPriced() {
			continue
		}
		id := p.GetResource().GetId()
		resp.Rows = append(resp.Rows,
			&pbc.AllocationRow{
				Subject: map[string]string{"kind": "workload", "namespace": "ns-" + id, "pod": "p-" + id,
					"node": id, "controller_kind": "Deployment", "controller": "api"},
				CpuCost: p.GetCost() * 0.3, MemCost: p.GetCost() * 0.3, TotalCost: p.GetCost() * 0.6,
				Currency: "USD",
			},
			&pbc.AllocationRow{
				Subject: map[string]string{"kind": "__idle__", "node": id},
				CpuCost: p.GetCost() * 0.4, TotalCost: p.GetCost() * 0.4, Currency: "USD",
			},
		)
	}
	return resp, nil
}

// statsCallCount is how many GetStats calls the plugin answered.
func (f *fakeClusterPlugin) statsCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statsCalls
}

// startClusterPlugin serves plugin over bufconn and returns a client that
// declares caps.
func startClusterPlugin(t *testing.T, name string, plugin *fakeClusterPlugin, caps ...string) *pluginhost.Client {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pbc.RegisterUsageSourceServiceServer(server, plugin)
	pbc.RegisterAllocatorServiceServer(server, plugin)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///"+name,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &pluginhost.Client{Name: name, Conn: conn, Metadata: &proto.PluginMetadata{Capabilities: caps}}
}

// clusterPricer prices n1 at 70 and n2 at 30 a month, and the same amounts
// over any window. Every other node is unpriced.
type clusterPricer struct{}

var clusterNodePrices = map[string]float64{"n1": 70, "n2": 30} //nolint:gochecknoglobals // read-only fixture

func (clusterPricer) GetProjectedCostWithErrors(
	_ context.Context, resources []engine.ResourceDescriptor,
) (*engine.CostResultWithErrors, error) {
	out := &engine.CostResultWithErrors{}
	for _, r := range resources {
		if price, ok := clusterNodePrices[r.ID]; ok {
			out.Results = append(out.Results, engine.CostResult{
				ResourceType: r.Type, ResourceID: r.ID, Monthly: price, Currency: "USD",
			})
		}
	}
	return out, nil
}

func (clusterPricer) GetWindowCost(
	_ context.Context, resources []engine.ResourceDescriptor, from, to time.Time,
) ([]engine.CostResult, error) {
	if !to.After(from) {
		return nil, errors.New("empty window")
	}
	var out []engine.CostResult
	for _, r := range resources {
		if price, ok := clusterNodePrices[r.ID]; ok {
			out = append(out, engine.CostResult{
				ResourceType: r.Type, ResourceID: r.ID, TotalCost: price, Currency: "USD",
			})
		}
	}
	return out, nil
}

var _ engine.ResourcePricer = clusterPricer{}
