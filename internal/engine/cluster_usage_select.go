package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// NamedUsageSource is one installed usage-source plugin.
type NamedUsageSource struct {
	Name   string
	Source UsageSource
}

// AmbiguousUsageSourceError reports a GetStats request that more than one
// usage source accepted, so none can be chosen without the user.
type AmbiguousUsageSourceError struct {
	// Mode is ModeRunRate or ModeHistorical.
	Mode string
	// Names lists every source that accepted the request, sorted.
	Names []string
}

func (e *AmbiguousUsageSourceError) Error() string {
	return fmt.Sprintf("several usage sources can serve %s usage (%s)", e.Mode, strings.Join(e.Names, ", "))
}

// usageRejectedError reports a request that every usage source rejected. It
// unwraps to each rejection, so the gRPC status of the first stays readable.
type usageRejectedError struct {
	mode    string
	names   []string
	reasons []error
}

func (e *usageRejectedError) Error() string {
	parts := make([]string, len(e.names))
	for i, name := range e.names {
		parts[i] = name + ": " + e.reasons[i].Error()
	}
	return fmt.Sprintf("no usage source accepted the %s request: %s", e.mode, strings.Join(parts, "; "))
}

func (e *usageRejectedError) Unwrap() []error { return e.reasons }

// UsageSourceSet routes GetStats to the one installed usage source that can
// serve the request. The usage contract (finfocus-spec UsageSourceService)
// answers INVALID_ARGUMENT when a source cannot serve the requested mode, and
// sources advertise no mode up front, so every source is asked in name order:
// a source that answers INVALID_ARGUMENT cannot serve the request. When
// exactly one source accepts, its answer (success or failure) is returned.
// When several accept, the result is an *AmbiguousUsageSourceError; when none
// does, the error lists every rejection.
type UsageSourceSet struct {
	sources []NamedUsageSource
}

// NewUsageSourceSet returns a set that asks sources in name order.
func NewUsageSourceSet(sources []NamedUsageSource) *UsageSourceSet {
	sorted := slices.Clone(sources)
	slices.SortFunc(sorted, func(a, b NamedUsageSource) int { return strings.Compare(a.Name, b.Name) })
	return &UsageSourceSet{sources: sorted}
}

// GetStats implements UsageSource.
func (s *UsageSourceSet) GetStats(
	ctx context.Context, in *pbc.GetStatsRequest, opts ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	mode := ModeRunRate
	if in.GetStart() != nil || in.GetEnd() != nil {
		mode = ModeHistorical
	}
	var (
		accepted []string
		resp     *pbc.GetStatsResponse
		respErr  error
		rejected = &usageRejectedError{mode: mode}
	)
	for _, src := range s.sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r, err := src.Source.GetStats(ctx, in, opts...)
		if ctxErr := ctx.Err(); ctxErr != nil {
			// A call cut short by the caller is neither an acceptance nor a rejection.
			return nil, ctxErr
		}
		if status.Code(err) == codes.InvalidArgument {
			rejected.names = append(rejected.names, src.Name)
			rejected.reasons = append(rejected.reasons, err)
			continue
		}
		accepted = append(accepted, src.Name)
		resp, respErr = r, err
	}
	switch len(accepted) {
	case 0:
		return nil, rejected
	case 1:
		return resp, respErr
	default:
		return nil, &AmbiguousUsageSourceError{Mode: mode, Names: accepted}
	}
}
