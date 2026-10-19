package integration_test

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
	"github.com/rshade/finfocus/internal/tui"
	"github.com/rshade/finfocus/internal/tui/detail"
)

func TestTUILazyLoading(t *testing.T) {
	t.Parallel()

	t.Run("lazy loading triggers on detail view", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		model := tui.NewCostViewModel(context.Background(), []engine.CostResult{
			{ResourceID: "urn:web", ResourceType: "aws:ec2/instance:Instance", Monthly: 10},
			{ResourceID: "urn:db", ResourceType: "aws:rds/instance:Instance", Monthly: 20},
		}).WithHistoryLoader(func(_ context.Context, resourceID string) ([]detail.Point, error) {
			calls.Add(1)
			assert.NotEmpty(t, resourceID)
			return []detail.Point{{
				Timestamp: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				Amount:    10,
				Currency:  "USD",
			}}, nil
		})

		assert.NotContains(t, model.View().Content, "Loading cost history...")
		assert.Equal(t, int32(0), calls.Load())

		updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		model = updated.(*tui.CostViewModel)
		assert.Contains(t, model.View().Content, "Loading cost history...")
		assert.Contains(t, model.View().Content, "RESOURCE DETAIL")
		assert.Equal(t, int32(0), calls.Load())
		require.NotNil(t, cmd)

		updated, _ = model.Update(cmd())
		model = updated.(*tui.CostViewModel)
		assert.Equal(t, int32(1), calls.Load())
		assert.Contains(t, model.View().Content, "2026-06-01")
		assert.Contains(t, model.View().Content, "$10.00 USD")
	})

	t.Run("error recovery with retry", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		model, _ := tui.NewOverviewModel(context.Background(), nil, 0, nil, nil)
		model.SetHistoryLoader(func(context.Context, string, string) ([]detail.Point, error) {
			calls.Add(1)
			if calls.Load() == 1 {
				return nil, errors.New("network timeout")
			}
			return []detail.Point{{
				Timestamp: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				Amount:    6,
				Currency:  "USD",
			}}, nil
		})

		rows := []engine.OverviewRow{{
			URN:    "urn:web",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
		}}
		updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
		model = updated.(tui.OverviewModel)
		updated, _ = model.Update(tui.OverviewDataReadyMsg{Rows: rows, TotalCount: 1, StackName: "dev"})
		model = updated.(tui.OverviewModel)
		updated, _ = model.Update(tui.OverviewAllResourcesLoadedMsg{})
		model = updated.(tui.OverviewModel)
		assert.Equal(t, int32(0), calls.Load())

		updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		model = updated.(tui.OverviewModel)
		require.NotNil(t, cmd)
		assert.Contains(t, model.View().Content, "Loading cost history...")

		updated, _ = model.Update(cmd())
		model = updated.(tui.OverviewModel)
		view := model.View().Content
		assert.Contains(t, view, "❌ Error: network timeout")
		assert.Contains(t, view, "[Press 'r' to retry]")
		assert.Contains(t, view, "RESOURCE DETAIL")

		updated, cmd = model.Update(tea.KeyPressMsg{Text: "r"})
		model = updated.(tui.OverviewModel)
		require.NotNil(t, cmd)
		assert.Contains(t, model.View().Content, "Loading cost history...")
		assert.Contains(t, model.View().Content, "RESOURCE DETAIL")

		updated, _ = model.Update(cmd())
		model = updated.(tui.OverviewModel)
		view = model.View().Content
		assert.Contains(t, view, "$6.00 USD")
		assert.NotContains(t, view, "network timeout")
		assert.Contains(t, view, "RESOURCE DETAIL")
	})
}
