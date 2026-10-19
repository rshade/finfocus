package engine_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/config"
	enginepkg "github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type historyPlugin struct {
	proto.CostSourceClient

	fn    func(context.Context, *proto.GetRecommendationHistoryRequest) (*proto.GetRecommendationHistoryResponse, error)
	calls int
}

func (p *historyPlugin) GetRecommendationHistory(
	ctx context.Context,
	in *proto.GetRecommendationHistoryRequest,
	_ ...grpc.CallOption,
) (*proto.GetRecommendationHistoryResponse, error) {
	p.calls++
	if p.fn != nil {
		return p.fn(ctx, in)
	}
	return &proto.GetRecommendationHistoryResponse{}, nil
}

func historyClient(name string, plugin *historyPlugin) *pluginhost.Client {
	return &pluginhost.Client{Name: name, API: plugin}
}

func historyAt(day int) time.Time {
	return time.Date(2026, time.January, day, 12, 0, 0, 0, time.UTC)
}

func localDismiss(t *testing.T, store *config.DismissalStore, id string, when time.Time) {
	t.Helper()
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: id,
		Status:           config.StatusDismissed,
		Reason:           "business-constraint",
		DismissedAt:      when,
		History: []config.LifecycleEvent{{
			Action:    config.ActionDismissed,
			Reason:    "business-constraint",
			Timestamp: when,
		}},
	}))
	require.NoError(t, store.Save())
}

func TestEngine_GetRecommendationHistory_PluginMerge(t *testing.T) {
	t.Parallel()

	t.Run("plugin only history includes actor", func(t *testing.T) {
		t.Parallel()
		when := historyAt(4)
		plugin := &historyPlugin{fn: func(
			_ context.Context, in *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			assert.Equal(t, "rec-plugin", in.RecommendationID)
			return &proto.GetRecommendationHistoryResponse{Events: []proto.RecommendationLifecycleEvent{{
				Action:    "acknowledged",
				Reason:    "reviewed",
				Actor:     "ada",
				Timestamp: when,
			}}}, nil
		}}
		eng := enginepkg.New([]*pluginhost.Client{historyClient("aws", plugin)}, nil)
		history, err := eng.GetRecommendationHistory(context.Background(), createTestStore(t), "rec-plugin")
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, config.LifecycleAction("acknowledged"), history[0].Action)
		assert.Equal(t, "ada", history[0].Actor)
		assert.Equal(t, 1, plugin.calls)
	})

	t.Run("orders events and keeps a named actor on a duplicate", func(t *testing.T) {
		t.Parallel()
		store := createTestStore(t)
		early := historyAt(2)
		later := historyAt(10)
		localDismiss(t, store, "rec-merge", later)
		expires := later.Add(24 * time.Hour)
		plugin := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return &proto.GetRecommendationHistoryResponse{Events: []proto.RecommendationLifecycleEvent{
				{
					Action:    "dismissed",
					Reason:    "business-constraint",
					Actor:     "ada",
					Timestamp: later,
				},
				{
					Action:    "snoozed",
					Reason:    "deferred",
					Actor:     "grace",
					Timestamp: early,
					ExpiresAt: &expires,
				},
				{
					Action: "note-only",
				},
			}}, nil
		}}
		eng := enginepkg.New([]*pluginhost.Client{historyClient("aws", plugin)}, nil)

		history, err := eng.GetRecommendationHistory(context.Background(), store, "rec-merge")
		require.NoError(t, err)
		require.Len(t, history, 3)
		assert.Equal(t, config.ActionSnoozed, history[0].Action)
		assert.Equal(t, "grace", history[0].Actor)
		assert.Equal(t, config.ActionDismissed, history[1].Action)
		assert.Equal(t, "ada", history[1].Actor)
		assert.Equal(t, config.LifecycleAction("note-only"), history[2].Action)
		assert.True(t, history[2].Timestamp.IsZero())

		again, err := eng.GetRecommendationHistory(context.Background(), store, "rec-merge")
		require.NoError(t, err)
		assert.Equal(t, history, again)
	})

	t.Run("distinct actors and transitions stay", func(t *testing.T) {
		t.Parallel()
		when := historyAt(6)
		otherExpiry := when.Add(time.Hour)
		plugin := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return &proto.GetRecommendationHistoryResponse{Events: []proto.RecommendationLifecycleEvent{
				{Action: "dismissed", Reason: "cost", Actor: "ada", Timestamp: when},
				{Action: "dismissed", Reason: "cost", Actor: "grace", Timestamp: when},
				{Action: "dismissed", Reason: "cost", Actor: "ada", Timestamp: when},
				{Action: "dismissed", Reason: "risk", Actor: "ada", Timestamp: when},
				{Action: "dismissed", Reason: "cost", Actor: "ada", Timestamp: when, ExpiresAt: &otherExpiry},
			}}, nil
		}}
		eng := enginepkg.New([]*pluginhost.Client{historyClient("aws", plugin)}, nil)
		history, err := eng.GetRecommendationHistory(context.Background(), createTestStore(t), "rec-distinct")
		require.NoError(t, err)
		require.Len(t, history, 4)
		assert.Equal(t, "ada", history[0].Actor)
		assert.Equal(t, "cost", history[0].Reason)
		assert.Nil(t, history[0].ExpiresAt)
		assert.Equal(t, "grace", history[1].Actor)
		require.NotNil(t, history[2].ExpiresAt)
		assert.Equal(t, "ada", history[2].Actor)
		assert.Equal(t, "risk", history[3].Reason)
	})

	t.Run("unsupported plugin leaves local history", func(t *testing.T) {
		t.Parallel()
		store := createTestStore(t)
		when := historyAt(3)
		localDismiss(t, store, "rec-old", when)
		before, err := os.ReadFile(store.FilePath())
		require.NoError(t, err)
		var logs bytes.Buffer
		logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
		ctx := logger.WithContext(context.Background())
		old := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return nil, status.Error(codes.Unimplemented, "plugin does not support GetRecommendationHistory")
		}}
		live := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return &proto.GetRecommendationHistoryResponse{}, nil
		}}
		eng := enginepkg.New([]*pluginhost.Client{
			historyClient("old", old),
			historyClient("live", live),
		}, nil)

		history, err := eng.GetRecommendationHistory(ctx, store, "rec-old")
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, config.ActionDismissed, history[0].Action)
		assert.Equal(t, 1, old.calls)
		assert.Equal(t, 1, live.calls)
		assert.Contains(t, logs.String(), "old")
		after, err := os.ReadFile(store.FilePath())
		require.NoError(t, err)
		assert.Equal(t, before, after)
		record, ok := store.Get("rec-old")
		require.True(t, ok)
		require.Len(t, record.History, 1)
		assert.Equal(t, config.ActionDismissed, record.History[0].Action)
	})

	t.Run("backend error keeps local history", func(t *testing.T) {
		t.Parallel()
		store := createTestStore(t)
		when := historyAt(5)
		localDismiss(t, store, "rec-down", when)
		var logs bytes.Buffer
		logger := zerolog.New(&logs).Level(zerolog.WarnLevel)
		ctx := logger.WithContext(context.Background())
		plugin := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return nil, status.Error(codes.Unavailable, "history backend down")
		}}
		eng := enginepkg.New([]*pluginhost.Client{historyClient("aws", plugin)}, nil)

		history, err := eng.GetRecommendationHistory(ctx, store, "rec-down")
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Contains(t, logs.String(), "aws")
		assert.Contains(t, logs.String(), "history backend down")
		assert.NotContains(t, logs.String(), "secret-token")
	})

	t.Run("nil client is skipped", func(t *testing.T) {
		t.Parallel()
		store := createTestStore(t)
		when := historyAt(7)
		localDismiss(t, store, "rec-nil", when)
		eng := enginepkg.New([]*pluginhost.Client{{Name: "broken"}}, nil)
		history, err := eng.GetRecommendationHistory(context.Background(), store, "rec-nil")
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, config.ActionDismissed, history[0].Action)
	})

	t.Run("canceled context stops before plugins", func(t *testing.T) {
		t.Parallel()
		plugin := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return &proto.GetRecommendationHistoryResponse{Events: []proto.RecommendationLifecycleEvent{{
				Action: "should-not-run",
			}}}, nil
		}}
		eng := enginepkg.New([]*pluginhost.Client{historyClient("aws", plugin)}, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		history, err := eng.GetRecommendationHistory(ctx, createTestStore(t), "rec-cancel")
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, history)
		assert.Zero(t, plugin.calls)
	})

	t.Run("plugin cancellation keeps local history", func(t *testing.T) {
		t.Parallel()
		store := createTestStore(t)
		localDismiss(t, store, "rec-stop", historyAt(8))
		next := &historyPlugin{fn: func(
			context.Context, *proto.GetRecommendationHistoryRequest,
		) (*proto.GetRecommendationHistoryResponse, error) {
			return &proto.GetRecommendationHistoryResponse{Events: []proto.RecommendationLifecycleEvent{{
				Action: "acknowledged",
				Actor:  "ada",
			}}}, nil
		}}
		for _, code := range []codes.Code{codes.Canceled, codes.DeadlineExceeded} {
			plugin := &historyPlugin{fn: func(
				context.Context, *proto.GetRecommendationHistoryRequest,
			) (*proto.GetRecommendationHistoryResponse, error) {
				return nil, status.Error(code, "plugin stopped")
			}}
			eng := enginepkg.New([]*pluginhost.Client{
				historyClient("aws", plugin),
				historyClient("next", next),
			}, nil)
			history, err := eng.GetRecommendationHistory(context.Background(), store, "rec-stop")
			require.NoError(t, err)
			require.Len(t, history, 2)
			assert.Equal(t, config.ActionDismissed, history[0].Action)
			assert.Equal(t, config.LifecycleAction("acknowledged"), history[1].Action)
			assert.Equal(t, 1, next.calls)
			next.calls = 0
		}
	})
}

//nolint:paralleltest // shortens the process-wide recommendation history RPC timeout
func TestEngine_GetRecommendationHistory_PluginTimeoutKeepsLocal(t *testing.T) {
	enginepkg.SetRecommendationHistoryTimeoutForTest(t, 20*time.Millisecond)
	store := createTestStore(t)
	localDismiss(t, store, "rec-hang", historyAt(9))
	hung := &historyPlugin{fn: func(
		ctx context.Context, _ *proto.GetRecommendationHistoryRequest,
	) (*proto.GetRecommendationHistoryResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	next := &historyPlugin{fn: func(
		context.Context, *proto.GetRecommendationHistoryRequest,
	) (*proto.GetRecommendationHistoryResponse, error) {
		return &proto.GetRecommendationHistoryResponse{Events: []proto.RecommendationLifecycleEvent{{
			Action: "acknowledged",
			Actor:  "ada",
		}}}, nil
	}}
	eng := enginepkg.New([]*pluginhost.Client{
		historyClient("hung", hung),
		historyClient("next", next),
	}, nil)

	started := time.Now()
	history, err := eng.GetRecommendationHistory(context.Background(), store, "rec-hang")
	elapsed := time.Since(started)

	require.NoError(t, err)
	assert.Less(t, elapsed, time.Second)
	require.Len(t, history, 2)
	assert.Equal(t, config.ActionDismissed, history[0].Action)
	assert.Equal(t, "ada", history[1].Actor)
	assert.Equal(t, 1, hung.calls)
	assert.Equal(t, 1, next.calls)
}

func TestEngine_GetRecommendationHistory_CallerCancelDuringPluginCall(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	plugin := &historyPlugin{fn: func(
		ctx context.Context, _ *proto.GetRecommendationHistoryRequest,
	) (*proto.GetRecommendationHistoryResponse, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	eng := enginepkg.New([]*pluginhost.Client{historyClient("aws", plugin)}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-started
		cancel()
	}()

	history, err := eng.GetRecommendationHistory(ctx, createTestStore(t), "rec-cancel-mid")
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, history)
	assert.Equal(t, 1, plugin.calls)
}
