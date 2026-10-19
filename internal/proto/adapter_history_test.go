package proto

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

func TestClientAdapter_GetRecommendationHistory(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	expires := when.Add(48 * time.Hour)

	t.Run("preserves event fields and context", func(t *testing.T) {
		t.Parallel()
		type ctxKey struct{}
		ctx := context.WithValue(context.Background(), ctxKey{}, "trace")
		var gotTrace string
		var gotID string
		mock := &mockPbcCostSourceServiceClient{
			getRecommendationHistoryFunc: func(
				callCtx context.Context,
				in *pbc.GetRecommendationHistoryRequest,
				_ ...grpc.CallOption,
			) (*pbc.GetRecommendationHistoryResponse, error) {
				if trace, ok := callCtx.Value(ctxKey{}).(string); ok {
					gotTrace = trace
				}
				gotID = in.GetRecommendationId()
				return &pbc.GetRecommendationHistoryResponse{
					Events: []*pbc.LifecycleEvent{{
						Action:       "acknowledged",
						Reason:       "reviewed",
						CustomReason: "queued for later",
						Actor:        "ada@example.com",
						Timestamp:    timestamppb.New(when),
						ExpiresAt:    timestamppb.New(expires),
					}},
				}, nil
			},
		}
		adapter := &clientAdapter{client: mock}

		resp, err := adapter.GetRecommendationHistory(ctx, &GetRecommendationHistoryRequest{
			RecommendationID: "rec-rich",
		})
		require.NoError(t, err)
		assert.Equal(t, "trace", gotTrace)
		assert.Equal(t, "rec-rich", gotID)
		require.Len(t, resp.Events, 1)
		got := resp.Events[0]
		assert.Equal(t, "acknowledged", got.Action)
		assert.Equal(t, "reviewed", got.Reason)
		assert.Equal(t, "queued for later", got.CustomReason)
		assert.Equal(t, "ada@example.com", got.Actor)
		assert.True(t, when.Equal(got.Timestamp))
		require.NotNil(t, got.ExpiresAt)
		assert.True(t, expires.Equal(*got.ExpiresAt))
	})

	t.Run("empty events", func(t *testing.T) {
		t.Parallel()
		adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{}}
		resp, err := adapter.GetRecommendationHistory(context.Background(), &GetRecommendationHistoryRequest{
			RecommendationID: "rec-empty",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.Events)
	})

	t.Run("nil response", func(t *testing.T) {
		t.Parallel()
		adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
			getRecommendationHistoryFunc: func(
				context.Context, *pbc.GetRecommendationHistoryRequest, ...grpc.CallOption,
			) (*pbc.GetRecommendationHistoryResponse, error) {
				return nil, nil //nolint:nilnil // the plugin contract under test is a nil body and a nil error
			},
		}}
		_, err := adapter.GetRecommendationHistory(context.Background(), &GetRecommendationHistoryRequest{
			RecommendationID: "rec-nil",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("empty id does not call the plugin", func(t *testing.T) {
		t.Parallel()
		called := false
		adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
			getRecommendationHistoryFunc: func(
				context.Context, *pbc.GetRecommendationHistoryRequest, ...grpc.CallOption,
			) (*pbc.GetRecommendationHistoryResponse, error) {
				called = true
				return &pbc.GetRecommendationHistoryResponse{}, nil
			},
		}}
		_, err := adapter.GetRecommendationHistory(context.Background(), &GetRecommendationHistoryRequest{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.False(t, called)
	})

	t.Run("status codes pass through", func(t *testing.T) {
		t.Parallel()
		adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
			getRecommendationHistoryFunc: func(
				context.Context, *pbc.GetRecommendationHistoryRequest, ...grpc.CallOption,
			) (*pbc.GetRecommendationHistoryResponse, error) {
				return nil, status.Error(codes.Unimplemented, "plugin does not support GetRecommendationHistory")
			},
		}}
		_, err := adapter.GetRecommendationHistory(context.Background(), &GetRecommendationHistoryRequest{
			RecommendationID: "rec-old",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})

	t.Run("malformed events are skipped", func(t *testing.T) {
		t.Parallel()
		adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
			getRecommendationHistoryFunc: func(
				context.Context, *pbc.GetRecommendationHistoryRequest, ...grpc.CallOption,
			) (*pbc.GetRecommendationHistoryResponse, error) {
				return &pbc.GetRecommendationHistoryResponse{
					Events: []*pbc.LifecycleEvent{
						nil,
						{
							Action:    "dismissed",
							Timestamp: &timestamppb.Timestamp{Seconds: 1 << 62},
						},
						{
							Action:    "snoozed",
							Timestamp: timestamppb.New(when),
							ExpiresAt: &timestamppb.Timestamp{Seconds: 1 << 62},
						},
						{
							Action:    "kept",
							Actor:     "ada",
							Timestamp: timestamppb.New(when),
						},
						{
							Action: "undated",
							Actor:  "no-time",
						},
					},
				}, nil
			},
		}}
		resp, err := adapter.GetRecommendationHistory(context.Background(), &GetRecommendationHistoryRequest{
			RecommendationID: "rec-mixed",
		})
		require.NoError(t, err)
		require.Len(t, resp.Events, 2)
		assert.Equal(t, "kept", resp.Events[0].Action)
		assert.Equal(t, "ada", resp.Events[0].Actor)
		assert.Equal(t, "undated", resp.Events[1].Action)
		assert.True(t, resp.Events[1].Timestamp.IsZero())
	})
}
