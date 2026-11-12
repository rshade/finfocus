package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// Default deadlines for the plugin calls of a cluster allocation run. The
// callers of cost cluster and overview expansion pass a context with no
// deadline, so without these a plugin that accepts a call and never answers
// hangs the command.
const (
	// defaultClusterStatsTimeout bounds a run-rate GetStats, which lists pods
	// and nodes once, and is the base of a windowed one. It matches
	// recommendationPageTimeout, the engine's other multi-target plugin call.
	defaultClusterStatsTimeout = 2 * time.Minute
	// defaultClusterStatsPerDay is added to a windowed GetStats for each
	// started day: a metrics source evaluates every query over the whole
	// window (the prometheus plugin at a 60 s step), so its work grows with it.
	defaultClusterStatsPerDay = 30 * time.Second
	// defaultClusterStatsMax caps a windowed GetStats. The prometheus plugin
	// issues 15 sequential queries, and Prometheus stops each at its default
	// --query.timeout of 2 minutes, so 30 minutes is the most a source on a
	// default-configured server can take and still answer.
	defaultClusterStatsMax = 30 * time.Minute
	// defaultAllocatePolicyTimeout bounds an Allocate that carries no usage and
	// nothing priced: the policy check and --show-policy. The allocator only
	// parses the policy, with no I/O.
	defaultAllocatePolicyTimeout = 10 * time.Second
	// defaultAllocateTimeout bounds the allocation itself. It is in-memory
	// arithmetic over the usage rows; two minutes leaves room for very large
	// clusters and a busy machine.
	defaultAllocateTimeout = 2 * time.Minute
)

// ClusterCallTimeouts bounds the usage-source and allocator calls of a
// cluster allocation run. Each call gets its own deadline, derived from the
// request, under the caller's context.
type ClusterCallTimeouts struct {
	// Stats bounds a run-rate GetStats and is the base of a windowed one.
	Stats time.Duration
	// StatsPerDay is added to a windowed GetStats for each started day.
	StatsPerDay time.Duration
	// StatsMax caps a windowed GetStats.
	StatsMax time.Duration
	// Policy bounds an Allocate with no usage rows and no priced resources.
	Policy time.Duration
	// Allocate bounds every other Allocate.
	Allocate time.Duration
}

// DefaultClusterCallTimeouts returns the deadlines cost cluster and overview
// cluster expansion use.
func DefaultClusterCallTimeouts() ClusterCallTimeouts {
	return ClusterCallTimeouts{
		Stats:       defaultClusterStatsTimeout,
		StatsPerDay: defaultClusterStatsPerDay,
		StatsMax:    defaultClusterStatsMax,
		Policy:      defaultAllocatePolicyTimeout,
		Allocate:    defaultAllocateTimeout,
	}
}

// statsTimeout is Stats for a run-rate request, and Stats plus StatsPerDay
// for each started day of a window, capped at StatsMax. A window whose end is
// not after its start gets Stats.
func (t ClusterCallTimeouts) statsTimeout(req *pbc.GetStatsRequest) time.Duration {
	if req.GetStart() == nil || req.GetEnd() == nil {
		return t.Stats
	}
	length := req.GetEnd().AsTime().Sub(req.GetStart().AsTime())
	if length <= 0 {
		return t.Stats
	}
	const day = hoursPerDay * time.Hour
	days := int64((length + day - 1) / day)
	return min(t.Stats+time.Duration(days)*t.StatsPerDay, max(t.StatsMax, t.Stats))
}

// allocateTimeout is Policy for a request that carries no usage and nothing
// priced, which leaves the allocator only the policy to read, and Allocate
// otherwise.
func (t ClusterCallTimeouts) allocateTimeout(req *pbc.AllocateRequest) time.Duration {
	if len(req.GetUsage()) == 0 && len(req.GetPriced()) == 0 {
		return t.Policy
	}
	return t.Allocate
}

// PluginCallTimeoutError reports a plugin call that ran out its own deadline
// while the caller's context was still active. It unwraps to the gRPC
// DeadlineExceeded error.
type PluginCallTimeoutError struct {
	// Role is "usage source" or "allocator".
	Role string
	// Plugin names the plugin.
	Plugin string
	// Call is the RPC name.
	Call string
	// Timeout is the deadline the call was given.
	Timeout time.Duration
	// Err is the error the call returned.
	Err error
}

func (e *PluginCallTimeoutError) Error() string {
	return fmt.Sprintf("%s %s did not answer %s within %s", e.Role, e.Plugin, e.Call, e.Timeout)
}

func (e *PluginCallTimeoutError) Unwrap() error { return e.Err }

// boundCall runs call under a deadline of timeout. A failure after that
// deadline passed, while ctx is still active, becomes a
// *PluginCallTimeoutError; anything else, including the caller's own
// cancellation or deadline and a DeadlineExceeded the plugin reports itself,
// is returned as it is.
func boundCall[T any](
	ctx context.Context, timeout time.Duration, call func(context.Context) (T, error),
	timeoutErr func(error) *PluginCallTimeoutError,
) (T, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := call(callCtx)
	if err != nil && ctx.Err() == nil && deadlinePassed(callCtx) {
		var zero T
		return zero, timeoutErr(err)
	}
	return resp, err
}

// deadlinePassed reports whether a call ran out its own deadline. gRPC
// enforces the deadline with its own timer and can return DeadlineExceeded
// before the context's timer has set Err(), so the clock is checked against
// the deadline too.
func deadlinePassed(ctx context.Context) bool {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	deadline, ok := ctx.Deadline()
	return ok && !time.Now().Before(deadline)
}

type boundUsageSource struct {
	name     string
	src      UsageSource
	timeouts ClusterCallTimeouts
}

// BoundUsageSource gives each GetStats call on src a deadline from timeouts.
// A call that runs out that deadline returns a *PluginCallTimeoutError that
// names the plugin.
func BoundUsageSource(name string, src UsageSource, timeouts ClusterCallTimeouts) UsageSource {
	return &boundUsageSource{name: name, src: src, timeouts: timeouts}
}

func (b *boundUsageSource) GetStats(
	ctx context.Context, in *pbc.GetStatsRequest, opts ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	timeout := b.timeouts.statsTimeout(in)
	return boundCall(ctx, timeout,
		func(callCtx context.Context) (*pbc.GetStatsResponse, error) {
			return b.src.GetStats(callCtx, in, opts...)
		},
		func(err error) *PluginCallTimeoutError {
			return &PluginCallTimeoutError{
				Role: "usage source", Plugin: b.name, Call: "GetStats", Timeout: timeout, Err: err,
			}
		})
}

type boundAllocator struct {
	name     string
	alloc    Allocator
	timeouts ClusterCallTimeouts
}

// BoundAllocator gives each Allocate call on alloc a deadline from timeouts.
// A call that runs out that deadline returns a *PluginCallTimeoutError that
// names the plugin.
func BoundAllocator(name string, alloc Allocator, timeouts ClusterCallTimeouts) Allocator {
	return &boundAllocator{name: name, alloc: alloc, timeouts: timeouts}
}

func (b *boundAllocator) Allocate(
	ctx context.Context, in *pbc.AllocateRequest, opts ...grpc.CallOption,
) (*pbc.AllocateResponse, error) {
	timeout := b.timeouts.allocateTimeout(in)
	return boundCall(ctx, timeout,
		func(callCtx context.Context) (*pbc.AllocateResponse, error) {
			return b.alloc.Allocate(callCtx, in, opts...)
		},
		func(err error) *PluginCallTimeoutError {
			return &PluginCallTimeoutError{
				Role: "allocator", Plugin: b.name, Call: "Allocate", Timeout: timeout, Err: err,
			}
		})
}
