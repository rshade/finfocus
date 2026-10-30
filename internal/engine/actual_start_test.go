package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// actualWindow returns an October window start and end, and a creation time
// inside it.
func actualWindow() (time.Time, time.Time, time.Time) {
	return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
}

func instanceWithProps(id string, props map[string]any) ResourceDescriptor {
	merged := map[string]any{"instanceType": "t3.micro", "availabilityZone": "us-east-1a"}
	for k, v := range props {
		merged[k] = v
	}
	return ResourceDescriptor{ID: id, Type: "aws:ec2:Instance", Provider: "aws", Properties: merged}
}

func TestResourceActualStart(t *testing.T) {
	t.Parallel()
	actualWindowStart, actualWindowEnd, createdMidWindow := actualWindow()

	tests := []struct {
		name  string
		props map[string]any
		want  time.Time
	}{
		{"no created time", nil, actualWindowStart},
		{"created inside window", map[string]any{
			PropertyPulumiCreated: createdMidWindow.Format(time.RFC3339),
		}, createdMidWindow},
		{"created before window", map[string]any{
			PropertyPulumiCreated: "2026-09-12T00:00:00Z",
		}, actualWindowStart},
		{"created after window", map[string]any{
			PropertyPulumiCreated: "2026-11-02T00:00:00Z",
		}, actualWindowStart},
		{"created at window end", map[string]any{
			PropertyPulumiCreated: actualWindowEnd.Format(time.RFC3339),
		}, actualWindowEnd},
		{"imported resource", map[string]any{
			PropertyPulumiCreated:  createdMidWindow.Format(time.RFC3339),
			PropertyPulumiExternal: "true",
		}, actualWindowStart},
		{"unparseable created time", map[string]any{
			PropertyPulumiCreated: "yesterday",
		}, actualWindowStart},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resourceActualStart(instanceWithProps("i-1", tt.props), actualWindowStart, actualWindowEnd)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got)
		})
	}
}

// startRecorder records the start each unary GetActualCost call receives.
type startRecorder struct {
	mu     sync.Mutex
	starts map[string]time.Time
}

func (r *startRecorder) getActualCost(
	_ context.Context, in *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts[in.ResourceIDs[0]] = time.Unix(in.StartTime, 0).UTC()
	return &proto.GetActualCostResponse{
		Results: []*proto.ActualCostResult{{Currency: "USD", TotalCost: 1}},
	}, nil
}

func TestActualCostStartsAtCreationTime(t *testing.T) {
	t.Parallel()
	actualWindowStart, actualWindowEnd, createdMidWindow := actualWindow()

	rec := &startRecorder{starts: map[string]time.Time{}}
	api := &mockBatchCostSourceClient{getActualCostFunc: rec.getActualCost}
	eng := New([]*pluginhost.Client{makeNonBatchClient("aws-public", api)}, nil)

	resources := []ResourceDescriptor{
		instanceWithProps("i-new", map[string]any{PropertyPulumiCreated: createdMidWindow.Format(time.RFC3339)}),
		instanceWithProps("i-old", map[string]any{PropertyPulumiCreated: "2026-09-12T00:00:00Z"}),
	}
	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: resources, From: actualWindowStart, To: actualWindowEnd,
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 2)

	assert.True(t, createdMidWindow.Equal(rec.starts["i-new"]), "got %s", rec.starts["i-new"])
	assert.True(t, actualWindowStart.Equal(rec.starts["i-old"]), "got %s", rec.starts["i-old"])
	for _, r := range result.Results {
		assert.True(t, actualWindowStart.Equal(r.StartDate), "the display window is unchanged")
	}
}

func TestBatchActualCostSendsCreatedResourcesUnary(t *testing.T) {
	t.Parallel()
	actualWindowStart, actualWindowEnd, createdMidWindow := actualWindow()

	rec := &startRecorder{starts: map[string]time.Time{}}
	var batchMu sync.Mutex
	var batched []string
	api := &mockBatchCostSourceClient{
		getActualCostFunc: rec.getActualCost,
		batchCostFunc: func(_ context.Context, in *pbc.BatchCostRequest, _ ...grpc.CallOption) (*pbc.BatchCostResponse, error) {
			batchMu.Lock()
			defer batchMu.Unlock()
			results := make([]*pbc.ResourceCostResult, 0, len(in.GetResources()))
			for _, res := range in.GetResources() {
				batched = append(batched, res.GetId())
				results = append(results, &pbc.ResourceCostResult{
					Resource: res,
					Result: &pbc.ResourceCostResult_CostData{CostData: &pbc.CostData{
						Data: &pbc.CostData_ActualCost{ActualCost: &pbc.ActualCostData{
							Results: []*pbc.ActualCostResult{{Cost: 1}},
						}},
					}},
				})
			}
			return &pbc.BatchCostResponse{Results: results}, nil
		},
	}
	eng := New([]*pluginhost.Client{makeBatchCapableClient("aws-public", api)}, nil)

	resources := []ResourceDescriptor{
		instanceWithProps("i-new", map[string]any{PropertyPulumiCreated: createdMidWindow.Format(time.RFC3339)}),
		instanceWithProps("i-old", map[string]any{PropertyPulumiCreated: "2026-09-12T00:00:00Z"}),
	}
	results, err := eng.GetActualCostWithOptions(context.Background(), ActualCostRequest{
		Resources: resources, From: actualWindowStart, To: actualWindowEnd,
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, []string{"i-old"}, batched)
	assert.True(t, createdMidWindow.Equal(rec.starts["i-new"]), "got %s", rec.starts["i-new"])
	_, unaryOld := rec.starts["i-old"]
	assert.False(t, unaryOld, "a resource that existed all window stays in the batch")
}
