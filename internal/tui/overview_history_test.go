package tui

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui/detail"
)

func TestOverviewModel_HistoryLoadsWhenDetailOpens(t *testing.T) {
	t.Parallel()

	model, _ := NewOverviewModel(context.Background(), nil, 0, nil, nil)
	model.height = 80
	var calls atomic.Int32
	model.SetHistoryLoader(func(_ context.Context, stack, resourceID string) ([]detail.Point, error) {
		calls.Add(1)
		assert.Equal(t, "dev", stack)
		assert.Equal(t, "urn:web", resourceID)
		return nil, detail.ErrHistorySkipped
	})

	rows := []engine.OverviewRow{{
		URN:    "urn:web",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusActive,
	}}
	updated, _ := model.Update(OverviewDataReadyMsg{Rows: rows, TotalCount: 1, StackName: "dev"})
	model = updated.(OverviewModel)
	updated, _ = model.Update(OverviewAllResourcesLoadedMsg{})
	model = updated.(OverviewModel)
	assert.Equal(t, int32(0), calls.Load())

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(OverviewModel)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.Contains(t, model.View().Content, "Loading cost history...")
	require.NotNil(t, cmd)
	assert.Equal(t, int32(0), calls.Load())

	updated, _ = model.Update(cmd())
	model = updated.(OverviewModel)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.Equal(t, int32(1), calls.Load())
	assert.NotContains(t, model.View().Content, "COST HISTORY")
	assert.Contains(t, model.View().Content, "RESOURCE DETAIL")
}

func TestOverviewModel_DetailScrollReachesHistory(t *testing.T) {
	t.Parallel()

	model, _ := NewOverviewModel(context.Background(), nil, 0, nil, nil)
	model.SetHistoryLoader(func(context.Context, string, string) ([]detail.Point, error) {
		return nil, errors.New("network timeout")
	})
	rows := []engine.OverviewRow{{
		URN:    "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:   "aws:ec2/instance:Instance",
		Status: engine.StatusActive,
	}}
	updated, _ := model.Update(OverviewDataReadyMsg{Rows: rows, TotalCount: 1, StackName: "state"})
	model = updated.(OverviewModel)
	updated, _ = model.Update(OverviewAllResourcesLoadedMsg{})
	model = updated.(OverviewModel)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	model = updated.(OverviewModel)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(OverviewModel)
	require.NotNil(t, cmd)
	updated, _ = model.Update(cmd())
	model = updated.(OverviewModel)

	assert.NotContains(t, model.View().Content, "[Press 'r' to retry]")
	assert.Contains(t, model.View().Content, detailScrollHint)
	for range 30 {
		updated, _ = model.Update(tea.KeyPressMsg{Text: "down"})
		model = updated.(OverviewModel)
	}
	assert.Contains(t, model.View().Content, "[Press 'r' to retry]")
}
