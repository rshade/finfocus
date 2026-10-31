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
		name    string
		props   map[string]any
		want    time.Time
		existed bool
	}{
		{"no created time", nil, actualWindowStart, true},
		{"created inside window", map[string]any{
			PropertyPulumiCreated: createdMidWindow.Format(time.RFC3339),
		}, createdMidWindow, true},
		{"created before window", map[string]any{
			PropertyPulumiCreated: "2026-09-12T00:00:00Z",
		}, actualWindowStart, true},
		{"created at window start", map[string]any{
			PropertyPulumiCreated: actualWindowStart.Format(time.RFC3339),
		}, actualWindowStart, true},
		{"created at window end", map[string]any{
			PropertyPulumiCreated: actualWindowEnd.Format(time.RFC3339),
		}, actualWindowEnd, false},
		{"created after window", map[string]any{
			PropertyPulumiCreated: "2026-11-02T00:00:00Z",
		}, actualWindowEnd, false},
		{"imported resource", map[string]any{
			PropertyPulumiCreated:  createdMidWindow.Format(time.RFC3339),
			PropertyPulumiExternal: "true",
		}, actualWindowStart, true},
		{"unparseable created time", map[string]any{
			PropertyPulumiCreated: "yesterday",
		}, actualWindowStart, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, existed := resourceActualStart(instanceWithProps("i-1", tt.props), actualWindowStart, actualWindowEnd)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got)
			assert.Equal(t, tt.existed, existed)
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
		require.Len(t, r.DailyCosts, 29, "the daily series covers the whole window")
		assert.InDelta(t, 1.0, sum(r.DailyCosts), 1e-9)
	}
	byID := map[string]CostResult{}
	for _, r := range result.Results {
		byID[r.ResourceID] = r
	}
	created := byID["i-new"].DailyCosts
	assert.Zero(t, sum(created[:16]), "no spend before the Oct 17 creation day")
	assert.InDelta(t, 1.0/13, created[16], 1e-9, "spend starts on the creation day")
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

func sum(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}

func TestActualCostSkipsResourceCreatedAfterWindow(t *testing.T) {
	t.Parallel()
	actualWindowStart, actualWindowEnd, _ := actualWindow()

	rec := &startRecorder{starts: map[string]time.Time{}}
	api := &mockBatchCostSourceClient{getActualCostFunc: rec.getActualCost}
	eng := New([]*pluginhost.Client{makeNonBatchClient("aws-public", api)}, nil)

	resources := []ResourceDescriptor{
		instanceWithProps("i-at-end", map[string]any{PropertyPulumiCreated: actualWindowEnd.Format(time.RFC3339)}),
		instanceWithProps("i-later", map[string]any{PropertyPulumiCreated: "2026-11-02T00:00:00Z"}),
	}
	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: resources, From: actualWindowStart, To: actualWindowEnd,
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 2)
	assert.Empty(t, rec.starts, "a resource with no time in the window is not sent to the plugin")
	for _, r := range result.Results {
		assert.Zero(t, r.TotalCost)
		assert.True(t, actualWindowStart.Equal(r.StartDate))
	}
}

func TestWindowDailyCostsBilledUnderOneDay(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)

	daily := windowDailyCosts(2.5, from, to.Add(-6*time.Hour), to)
	require.Len(t, daily, 29)
	assert.InDelta(t, 2.5, daily[28], 1e-9, "a resource billed for hours keeps its spend on the last day")
	assert.InDelta(t, 2.5, sum(daily), 1e-9)
	assert.Nil(t, windowDailyCosts(2.5, from, from, from.Add(time.Hour)), "a window under a day has no series")
}
