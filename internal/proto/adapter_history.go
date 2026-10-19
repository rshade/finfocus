package proto

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/logging"
)

// GetRecommendationHistoryRequest selects one recommendation's plugin history.
type GetRecommendationHistoryRequest struct {
	// RecommendationID is the recommendation whose history is requested.
	RecommendationID string
}

// RecommendationLifecycleEvent is one plugin lifecycle event.
// Action and Reason are the plugin's own strings.
type RecommendationLifecycleEvent struct {
	Action       string
	Reason       string
	CustomReason string
	Actor        string
	Timestamp    time.Time
	ExpiresAt    *time.Time
}

// GetRecommendationHistoryResponse is the mapped plugin history.
// Events is empty when the plugin recorded nothing.
type GetRecommendationHistoryResponse struct {
	Events []RecommendationLifecycleEvent
}

// GetRecommendationHistory calls the optional plugin RPC.
// A nil event, or an event with an invalid timestamp or expiry, is skipped.
// Status codes from the plugin stay available through status.Code.
func (c *clientAdapter) GetRecommendationHistory(
	ctx context.Context,
	in *GetRecommendationHistoryRequest,
	opts ...grpc.CallOption,
) (*GetRecommendationHistoryResponse, error) {
	if in == nil || in.RecommendationID == "" {
		return nil, status.Error(codes.InvalidArgument, "recommendation_id is required")
	}

	resp, err := c.client.GetRecommendationHistory(ctx, &pbc.GetRecommendationHistoryRequest{
		RecommendationId: in.RecommendationID,
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("GetRecommendationHistory RPC failed: %w", err)
	}
	if resp == nil {
		return nil, status.Error(codes.Internal, "GetRecommendationHistory returned an empty response")
	}

	events := make([]RecommendationLifecycleEvent, 0, len(resp.GetEvents()))
	for i, ev := range resp.GetEvents() {
		mapped, ok := mapRecommendationLifecycleEvent(ev)
		if !ok {
			logging.FromContext(ctx).Debug().
				Str("component", "proto").
				Str("operation", "recommendation_history").
				Int("event_index", i).
				Msg("skipping malformed plugin lifecycle event")
			continue
		}
		events = append(events, mapped)
	}
	return &GetRecommendationHistoryResponse{Events: events}, nil
}

// mapRecommendationLifecycleEvent copies one plugin event.
// ok is false when the event is nil or a present timestamp is invalid.
func mapRecommendationLifecycleEvent(ev *pbc.LifecycleEvent) (RecommendationLifecycleEvent, bool) {
	if ev == nil {
		return RecommendationLifecycleEvent{}, false
	}
	out := RecommendationLifecycleEvent{
		Action:       ev.GetAction(),
		Reason:       ev.GetReason(),
		CustomReason: ev.GetCustomReason(),
		Actor:        ev.GetActor(),
	}
	if ts := ev.GetTimestamp(); ts != nil {
		if err := ts.CheckValid(); err != nil {
			return RecommendationLifecycleEvent{}, false
		}
		out.Timestamp = ts.AsTime()
	}
	if exp := ev.GetExpiresAt(); exp != nil {
		if err := exp.CheckValid(); err != nil {
			return RecommendationLifecycleEvent{}, false
		}
		expires := exp.AsTime()
		out.ExpiresAt = &expires
	}
	return out, true
}
