package detail

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewError_RetryHint(t *testing.T) {
	t.Parallel()

	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		return nil, errors.New("network timeout")
	})
	assert.Empty(t, model.ViewError())

	cmd := model.Activate(context.Background())
	updated, _ := model.Update(cmd())
	model = updated.(*Model)

	view := model.ViewError()
	assert.Contains(t, view, "❌ Error: network timeout")
	assert.Contains(t, view, "[Press 'r' to retry]")
	assert.NotContains(t, view, "Retries:")
	assert.Contains(t, model.View().Content, "COST HISTORY")
	assert.Equal(t, view, model.View().Content)
}

func TestViewError_ShowsRetryCountAfterSecondRetry(t *testing.T) {
	t.Parallel()

	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		return nil, errors.New("network timeout")
	})
	updated, _ := model.Update(model.Activate(context.Background())())
	model = updated.(*Model)
	assert.Equal(t, 0, model.RetryCount())
	assert.NotContains(t, model.ViewError(), "Retries:")

	retry := func() {
		t.Helper()
		var cmd tea.Cmd
		updated, cmd = model.Update(tea.KeyPressMsg{Text: "r"})
		model = updated.(*Model)
		require.NotNil(t, cmd)
		assert.Equal(t, LoadStateLoading, model.State())
		updated, _ = model.Update(cmd())
		model = updated.(*Model)
		require.Equal(t, LoadStateError, model.State())
	}

	retry()
	assert.Equal(t, 1, model.RetryCount())
	assert.NotContains(t, model.ViewError(), "Retries:")

	retry()
	assert.Equal(t, 2, model.RetryCount())
	assert.Contains(t, model.ViewError(), "Retries: 2")
	assert.Contains(t, model.ViewError(), "[Press 'r' to retry]")
}

func TestViewLoaded_TruncatesToLatest(t *testing.T) {
	t.Parallel()

	points := make([]Point, maxHistoryRows+2)
	for i := range points {
		points[i] = Point{
			Timestamp: time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC),
			Amount:    float64(i + 1),
			Currency:  "USD",
		}
	}
	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		return points, nil
	})
	updated, _ := model.Update(model.Activate(context.Background())())
	model = updated.(*Model)

	view := model.View().Content
	assert.NotContains(t, view, "2026-01-01")
	assert.NotContains(t, view, "2026-01-02")
	assert.Contains(t, view, "2026-01-14")
	assert.Contains(t, view, "Showing latest 12 of 14.")
}

func TestViewLoaded_EmptySeries(t *testing.T) {
	t.Parallel()

	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		return nil, nil
	})
	updated, _ := model.Update(model.Activate(context.Background())())
	model = updated.(*Model)
	assert.Contains(t, model.View().Content, "No cost history.")
	assert.Empty(t, model.ViewError())
}

func TestViewError_NilLoader(t *testing.T) {
	t.Parallel()

	model := New("urn:web", nil)
	assert.Nil(t, model.Activate(context.Background()))
	assert.Contains(t, model.ViewError(), "cost history loader is not configured")
}
