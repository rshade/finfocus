package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// shortTimeouts gives every cluster plugin call a deadline a test can reach.
func shortTimeouts() ClusterCallTimeouts {
	return ClusterCallTimeouts{
		Stats:       20 * time.Millisecond,
		StatsPerDay: time.Millisecond,
		StatsMax:    time.Second,
		Policy:      20 * time.Millisecond,
		Allocate:    20 * time.Millisecond,
	}
}

func TestDefaultClusterCallTimeouts_StatsScaleWithTheWindow(t *testing.T) {
	t.Parallel()
	end := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	window := func(d time.Duration) *pbc.GetStatsRequest {
		return &pbc.GetStatsRequest{Start: timestamppb.New(end.Add(-d)), End: timestamppb.New(end)}
	}
	day := 24 * time.Hour

	tests := []struct {
		name string
		req  *pbc.GetStatsRequest
		want time.Duration
	}{
		{name: "run-rate", req: &pbc.GetStatsRequest{}, want: 2 * time.Minute},
		{name: "one day", req: window(day), want: 2*time.Minute + 30*time.Second},
		{name: "a started day counts whole", req: window(36 * time.Hour), want: 3 * time.Minute},
		{name: "seven days", req: window(7 * day), want: 5*time.Minute + 30*time.Second},
		{name: "41 days", req: window(41 * day), want: 22*time.Minute + 30*time.Second},
		{name: "long windows stop at the cap", req: window(365 * day), want: 30 * time.Minute},
		{
			name: "an end before the start gets the base",
			req:  &pbc.GetStatsRequest{Start: timestamppb.New(end), End: timestamppb.New(end.Add(-day))},
			want: 2 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, DefaultClusterCallTimeouts().statsTimeout(tt.req))
		})
	}
}

func TestDefaultClusterCallTimeouts_AllocateByRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  *pbc.AllocateRequest
		want time.Duration
	}{
		{name: "policy only", req: &pbc.AllocateRequest{PolicyJson: []byte(`{}`)}, want: 10 * time.Second},
		{
			name: "usage rows",
			req:  &pbc.AllocateRequest{Usage: []*pbc.UsageRow{{}}},
			want: 2 * time.Minute,
		},
		{
			name: "priced resources",
			req:  &pbc.AllocateRequest{Priced: []*pbc.PricedResource{{}}},
			want: 2 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, DefaultClusterCallTimeouts().allocateTimeout(tt.req))
		})
	}
}

func TestBoundUsageSource(t *testing.T) {
	t.Parallel()

	t.Run("a stalled call names the plugin and the call", func(t *testing.T) {
		t.Parallel()
		src := BoundUsageSource("prometheus", &stallUsage{}, shortTimeouts())
		_, err := src.GetStats(context.Background(), &pbc.GetStatsRequest{})
		require.Error(t, err)
		timeout, ok := errorsAsTimeout(err)
		require.True(t, ok, "err: %v", err)
		assert.Equal(t, "usage source", timeout.Role)
		assert.Equal(t, "prometheus", timeout.Plugin)
		assert.Equal(t, "GetStats", timeout.Call)
		assert.Equal(t, 20*time.Millisecond, timeout.Timeout)
		assert.Equal(t, "usage source prometheus did not answer GetStats within 20ms", err.Error())
		assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	})

	t.Run("caller cancellation is not a timeout", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		timeouts := shortTimeouts()
		timeouts.Stats = time.Minute
		src := BoundUsageSource("prometheus", &cancelingUsage{cancel: cancel}, timeouts)
		_, err := src.GetStats(ctx, &pbc.GetStatsRequest{})
		require.Error(t, err)
		_, ok := errorsAsTimeout(err)
		assert.False(t, ok)
		assert.Equal(t, codes.Canceled, status.Code(err))
	})

	t.Run("a deadline the plugin reports itself passes through", func(t *testing.T) {
		t.Parallel()
		own := status.Error(codes.DeadlineExceeded, "prometheus query timed out")
		src := BoundUsageSource("prometheus", &modeOnlyUsage{err: own}, shortTimeouts())
		_, err := src.GetStats(context.Background(), &pbc.GetStatsRequest{})
		require.ErrorIs(t, err, own)
		_, ok := errorsAsTimeout(err)
		assert.False(t, ok)
	})

	t.Run("an answer inside the deadline is returned", func(t *testing.T) {
		t.Parallel()
		src := BoundUsageSource("kubernetes", &modeOnlyUsage{}, shortTimeouts())
		resp, err := src.GetStats(context.Background(), &pbc.GetStatsRequest{})
		require.NoError(t, err)
		assert.Equal(t, pbc.StatsMode_STATS_MODE_RUN_RATE, resp.GetMode())
	})
}

func TestBoundCall_TransportDeadlineBeforeContextTimer(t *testing.T) {
	t.Parallel()
	const timeout = 20 * time.Millisecond
	transport := status.Error(codes.DeadlineExceeded, "stream terminated by RST_STREAM")

	_, err := boundCall(context.Background(), timeout,
		func(context.Context) (struct{}, error) {
			time.Sleep(timeout)
			return struct{}{}, transport
		},
		func(err error) *PluginCallTimeoutError { return &PluginCallTimeoutError{Err: err} },
	)

	_, ok := errorsAsTimeout(err)
	assert.True(t, ok, "err: %v", err)
}

// slowAllocator answers after delay, or fails when its context ends first.
type slowAllocator struct{ delay time.Duration }

func (s *slowAllocator) Allocate(
	ctx context.Context, _ *pbc.AllocateRequest, _ ...grpc.CallOption,
) (*pbc.AllocateResponse, error) {
	select {
	case <-time.After(s.delay):
		return &pbc.AllocateResponse{PolicyDigest: "d"}, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}

func TestBoundAllocator(t *testing.T) {
	t.Parallel()

	t.Run("a stalled policy check uses the policy deadline", func(t *testing.T) {
		t.Parallel()
		timeouts := shortTimeouts()
		timeouts.Allocate = time.Minute
		alloc := BoundAllocator("kubernetes", &slowAllocator{delay: time.Minute}, timeouts)
		_, _, err := ShowAllocationPolicy(context.Background(), alloc, []byte(`{}`))
		require.Error(t, err)
		timeout, ok := errorsAsTimeout(err)
		require.True(t, ok, "err: %v", err)
		assert.Equal(t, "allocator", timeout.Role)
		assert.Equal(t, "Allocate", timeout.Call)
		assert.Equal(t, "allocate: allocator kubernetes did not answer Allocate within 20ms", err.Error())
		var rejected *PolicyRejectedError
		assert.NotErrorAs(t, err, &rejected)
	})

	t.Run("an allocation with usage gets the longer deadline", func(t *testing.T) {
		t.Parallel()
		timeouts := shortTimeouts()
		timeouts.Allocate = time.Minute
		alloc := BoundAllocator("kubernetes", &slowAllocator{delay: 50 * time.Millisecond}, timeouts)
		resp, err := alloc.Allocate(context.Background(), &pbc.AllocateRequest{Usage: []*pbc.UsageRow{{}}})
		require.NoError(t, err)
		assert.Equal(t, "d", resp.GetPolicyDigest())
	})

	t.Run("a stalled allocation fails the run with the allocator named", func(t *testing.T) {
		t.Parallel()
		usage := &fakeUsage{resp: &pbc.GetStatsResponse{
			Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
			Rows: []*pbc.UsageRow{{Subject: map[string]string{"kind": "workload", "pod": "p"}}},
		}}
		alloc := BoundAllocator("kubernetes", &slowAllocator{delay: time.Minute}, shortTimeouts())
		_, err := RunClusterAllocation(context.Background(), usage, alloc, fakePricer{}, ClusterRequest{})
		require.Error(t, err)
		assert.EqualError(t, err, "allocate: allocator kubernetes did not answer Allocate within 20ms")
	})
}

func errorsAsTimeout(err error) (*PluginCallTimeoutError, bool) {
	return errors.AsType[*PluginCallTimeoutError](err)
}
