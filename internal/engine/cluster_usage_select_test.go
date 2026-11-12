package engine

import (
	"context"
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

// modeOnlyUsage serves one mode and rejects the other with InvalidArgument,
// as the spec's UsageSourceService error semantics require.
type modeOnlyUsage struct {
	historical bool
	err        error
	calls      int
}

func (m *modeOnlyUsage) GetStats(
	_ context.Context, in *pbc.GetStatsRequest, _ ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	m.calls++
	window := in.GetStart() != nil || in.GetEnd() != nil
	if window != m.historical {
		return nil, status.Error(codes.InvalidArgument, "cannot serve this mode")
	}
	if m.err != nil {
		return nil, m.err
	}
	mode := pbc.StatsMode_STATS_MODE_RUN_RATE
	if m.historical {
		mode = pbc.StatsMode_STATS_MODE_HISTORICAL
	}
	return &pbc.GetStatsResponse{Mode: mode}, nil
}

func windowRequest() *pbc.GetStatsRequest {
	end := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	return &pbc.GetStatsRequest{
		Start: timestamppb.New(end.AddDate(0, 0, -7)),
		End:   timestamppb.New(end),
	}
}

func TestUsageSourceSet_PicksTheSourceThatServesTheMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *pbc.GetStatsRequest
		wantMode pbc.StatsMode
	}{
		{name: "run-rate", req: &pbc.GetStatsRequest{}, wantMode: pbc.StatsMode_STATS_MODE_RUN_RATE},
		{name: "historical", req: windowRequest(), wantMode: pbc.StatsMode_STATS_MODE_HISTORICAL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runRate := &modeOnlyUsage{}
			historical := &modeOnlyUsage{historical: true}
			set := NewUsageSourceSet([]NamedUsageSource{
				{Name: "prometheus", Source: historical},
				{Name: "kubernetes", Source: runRate},
			})

			resp, err := set.GetStats(context.Background(), tt.req)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, resp.GetMode())
			assert.Equal(t, 1, runRate.calls)
			assert.Equal(t, 1, historical.calls)
		})
	}
}

func TestUsageSourceSet_SeveralAcceptingSourcesAreAmbiguous(t *testing.T) {
	t.Parallel()
	set := NewUsageSourceSet([]NamedUsageSource{
		{Name: "zeta", Source: &modeOnlyUsage{}},
		{Name: "prometheus", Source: &modeOnlyUsage{historical: true}},
		{Name: "kubernetes", Source: &modeOnlyUsage{}},
	})

	_, err := set.GetStats(context.Background(), &pbc.GetStatsRequest{})
	var ambiguous *AmbiguousUsageSourceError
	require.ErrorAs(t, err, &ambiguous)
	assert.Equal(t, ModeRunRate, ambiguous.Mode)
	assert.Equal(t, []string{"kubernetes", "zeta"}, ambiguous.Names)
	assert.Equal(t, "several usage sources can serve run-rate usage (kubernetes, zeta)", err.Error())
}

func TestUsageSourceSet_AcceptingSourceFailureIsReturned(t *testing.T) {
	t.Parallel()
	unavailable := status.Error(codes.Unavailable, "prometheus unreachable")
	set := NewUsageSourceSet([]NamedUsageSource{
		{Name: "kubernetes", Source: &modeOnlyUsage{}},
		{Name: "prometheus", Source: &modeOnlyUsage{historical: true, err: unavailable}},
	})

	_, err := set.GetStats(context.Background(), windowRequest())
	require.ErrorIs(t, err, unavailable)
	var ambiguous *AmbiguousUsageSourceError
	assert.NotErrorAs(t, err, &ambiguous)
}

func TestUsageSourceSet_EveryRejectionIsReported(t *testing.T) {
	t.Parallel()
	contextErr := status.Error(codes.InvalidArgument, `kubeconfig context "nope" not found`)
	set := NewUsageSourceSet([]NamedUsageSource{
		{Name: "prometheus", Source: &modeOnlyUsage{historical: true}},
		{Name: "kubernetes", Source: &fakeUsage{err: contextErr}},
	})

	_, err := set.GetStats(context.Background(), &pbc.GetStatsRequest{Scope: "nope"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	require.ErrorIs(t, err, contextErr)
	assert.Contains(t, err.Error(), "no usage source accepted the run-rate request")
	assert.Contains(t, err.Error(),
		`kubernetes: rpc error: code = InvalidArgument desc = kubeconfig context "nope" not found`)
	assert.Contains(t, err.Error(), "prometheus: rpc error: code = InvalidArgument desc = cannot serve this mode")
}

func TestUsageSourceSet_StopsWhenTheCallerCancels(t *testing.T) {
	t.Parallel()
	first := &modeOnlyUsage{historical: true}
	second := &modeOnlyUsage{}
	set := NewUsageSourceSet([]NamedUsageSource{
		{Name: "a", Source: first},
		{Name: "b", Source: second},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := set.GetStats(ctx, &pbc.GetStatsRequest{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, first.calls)
	assert.Zero(t, second.calls)
}

// cancelingUsage cancels the caller's context while its call runs.
type cancelingUsage struct {
	cancel context.CancelFunc
}

func (c *cancelingUsage) GetStats(
	ctx context.Context, _ *pbc.GetStatsRequest, _ ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	c.cancel()
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

// stallUsage blocks until the context passed to GetStats ends.
type stallUsage struct{}

func (s *stallUsage) GetStats(
	ctx context.Context, _ *pbc.GetStatsRequest, _ ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

//nolint:paralleltest // shortens the package-level usageSourceProbeTimeout
func TestUsageSourceSet_ProbeTimeoutIsNotAcceptance(t *testing.T) {
	// Shortens the package-level probe deadline.
	prev := usageSourceProbeTimeout
	usageSourceProbeTimeout = 20 * time.Millisecond
	t.Cleanup(func() { usageSourceProbeTimeout = prev })

	next := &modeOnlyUsage{}
	set := NewUsageSourceSet([]NamedUsageSource{
		{Name: "prometheus", Source: &stallUsage{}},
		{Name: "zzz", Source: next},
	})

	_, err := set.GetStats(context.Background(), &pbc.GetStatsRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	require.ErrorContains(t, err, "usage source prometheus did not answer")
	assert.Zero(t, next.calls)
	var ambiguous *AmbiguousUsageSourceError
	assert.NotErrorAs(t, err, &ambiguous)
}

func TestUsageSourceSet_CancelDuringTheLastCallIsNotAmbiguous(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	set := NewUsageSourceSet([]NamedUsageSource{
		{Name: "a", Source: &modeOnlyUsage{}},
		{Name: "b", Source: &cancelingUsage{cancel: cancel}},
	})

	_, err := set.GetStats(ctx, &pbc.GetStatsRequest{})
	require.ErrorIs(t, err, context.Canceled)
	var ambiguous *AmbiguousUsageSourceError
	assert.NotErrorAs(t, err, &ambiguous)
}
