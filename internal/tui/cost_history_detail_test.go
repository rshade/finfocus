package tui

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui/detail"
)

func TestCostViewModel_HistoryLoadsWhenDetailOpens(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	model := NewCostViewModel(context.Background(), []engine.CostResult{
		{ResourceID: "urn:web", ResourceType: "aws:ec2/instance:Instance", Monthly: 10},
	}).WithHistoryLoader(func(context.Context, string) ([]detail.Point, error) {
		calls.Add(1)
		if calls.Load() == 1 {
			return nil, errors.New("network timeout")
		}
		return []detail.Point{{
			Timestamp: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			Amount:    8,
			Currency:  "USD",
		}}, nil
	})

	list, _ := model.Update(tea.KeyPressMsg{Text: "s"})
	model = list.(*CostViewModel)
	assert.Equal(t, int32(0), calls.Load())
	assert.NotContains(t, model.View().Content, "Loading cost history...")

	opened, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = opened.(*CostViewModel)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.Equal(t, int32(0), calls.Load())
	assert.Contains(t, model.View().Content, "Loading cost history...")
	require.NotNil(t, cmd)

	loaded, _ := model.Update(cmd())
	model = loaded.(*CostViewModel)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.Contains(t, model.View().Content, "❌ Error: network timeout")
	assert.Contains(t, model.View().Content, "[Press 'r' to retry]")
	assert.Contains(t, model.View().Content, "RESOURCE DETAIL")

	retried, cmd := model.Update(tea.KeyPressMsg{Text: "r"})
	model = retried.(*CostViewModel)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.Contains(t, model.View().Content, "Loading cost history...")
	require.NotNil(t, cmd)

	loaded, _ = model.Update(cmd())
	model = loaded.(*CostViewModel)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.Contains(t, model.View().Content, "$8.00 USD")
	assert.NotContains(t, model.View().Content, "network timeout")
}

func TestCostViewModel_HistoryNotLoadedWithoutLoader(t *testing.T) {
	t.Parallel()

	model := NewCostViewModel(context.Background(), []engine.CostResult{
		{ResourceID: "urn:web", ResourceType: "aws:ec2/instance:Instance", Monthly: 10},
	})
	opened, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = opened.(*CostViewModel)
	assert.Nil(t, cmd)
	assert.Equal(t, ViewStateDetail, model.state)
	assert.NotContains(t, model.View().Content, "COST HISTORY")
}
