package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// historyWebPlugin is the dashboard engine's history RPC for one plugin.
type historyWebPlugin struct {
	proto.CostSourceClient

	events []proto.RecommendationLifecycleEvent
	err    error
	calls  int
}

func (p *historyWebPlugin) GetRecommendationHistory(
	context.Context,
	*proto.GetRecommendationHistoryRequest,
	...grpc.CallOption,
) (*proto.GetRecommendationHistoryResponse, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &proto.GetRecommendationHistoryResponse{Events: p.events}, nil
}

func TestWebSessionOptionsWiresRecommendationHistory(t *testing.T) {
	t.Parallel()
	data := newWebData(nil, overviewParams{}, engine.DateRange{}, nil)
	opts := webSessionOptions(context.Background(), data, nil, nil, nil)
	require.NotNil(t, opts.RecommendationHistory)
}

func TestRecommendationHistoryUsesQueryEngine(t *testing.T) {
	// Dismissal storage and the resolved project directory are process-wide.
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	previous := config.GetResolvedProjectDir()
	config.SetResolvedProjectDir("")
	t.Cleanup(func() { config.SetResolvedProjectDir(previous) })

	when := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	later := when.Add(24 * time.Hour)
	store, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, store.Load())
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: "rec-1",
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

	live := &historyWebPlugin{events: []proto.RecommendationLifecycleEvent{{
		Action:    "acknowledged",
		Reason:    "reviewed",
		Actor:     "ada",
		Timestamp: later,
	}}}
	down := &historyWebPlugin{err: status.Error(codes.Unavailable, "history backend down secret-token")}
	clients := []*pluginhost.Client{
		{Name: "aws", API: live},
		{Name: "azure", API: down},
	}
	eng := engine.New(clients, nil)
	data := newWebData(nil, overviewParams{}, engine.DateRange{}, nil)
	data.eng = eng

	events, warnings, err := data.recommendationHistory(context.Background(), "rec-1")
	require.NoError(t, err)
	directEvents, directWarnings, err := eng.GetRecommendationHistory(context.Background(), store, "rec-1")
	require.NoError(t, err)
	assert.Equal(t, directEvents, events)
	assert.Equal(t, directWarnings, warnings)
	assert.Equal(t, []string{"azure: recommendation history unavailable"}, warnings)
	assert.NotContains(t, strings.Join(warnings, " "), "secret-token")
	require.Len(t, events, 2)
	assert.Equal(t, config.ActionDismissed, events[0].Action)
	assert.Equal(t, "ada", events[1].Actor)
	assert.Equal(t, 2, live.calls)
	assert.Equal(t, 2, down.calls)

	replacement := &historyWebPlugin{}
	data.eng = engine.New([]*pluginhost.Client{{Name: "next", API: replacement}}, nil)
	_, _, err = data.recommendationHistory(context.Background(), "rec-1")
	require.NoError(t, err)
	assert.Equal(t, 2, live.calls)
	assert.Equal(t, 1, replacement.calls)
}
