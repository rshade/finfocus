package detail

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivate_LoadsOnlyWhenStarted(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		calls.Add(1)
		return []Point{{
			Timestamp: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			Amount:    12,
			Currency:  "USD",
		}}, nil
	})

	assert.Equal(t, LoadStateIdle, model.State())
	assert.Empty(t, model.View().Content)
	assert.Equal(t, int32(0), calls.Load())

	cmd := model.Activate(context.Background())
	require.NotNil(t, cmd)
	assert.Equal(t, LoadStateLoading, model.State())
	assert.Contains(t, model.View().Content, "Loading cost history...")
	assert.Equal(t, int32(0), calls.Load(), "fetch waits for the command")

	assert.Nil(t, model.Activate(context.Background()))
	assert.Equal(t, int32(0), calls.Load())

	updated, _ := model.Update(cmd())
	model = updated.(*Model)
	assert.Equal(t, LoadStateLoaded, model.State())
	assert.Equal(t, int32(1), calls.Load())
	require.Len(t, model.Points(), 1)
	assert.Contains(t, model.View().Content, "2026-01-02")
	assert.Contains(t, model.View().Content, "$12.00 USD")
	assert.Nil(t, model.Activate(context.Background()))
	assert.Equal(t, int32(1), calls.Load())
}

func TestActivate_ConcurrentOpenFetchesOnce(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		calls.Add(1)
		return []Point{{Amount: 1, Currency: "USD"}}, nil
	})

	var cmds []tea.Cmd
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := model.Activate(context.Background())
			mu.Lock()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Len(t, cmds, 1)
	_, _ = model.Update(cmds[0]())
	assert.Equal(t, int32(1), calls.Load())
}

func TestActivate_ContextCanceled(t *testing.T) {
	t.Parallel()

	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		t.Fatal("loader should not run after cancel")
		return nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := model.Activate(ctx)
	require.NotNil(t, cmd)
	updated, _ := model.Update(cmd())
	model = updated.(*Model)
	assert.Equal(t, LoadStateError, model.State())
	assert.Contains(t, model.View().Content, "context canceled")
}

func TestUpdate_IgnoresStaleReply(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		calls.Add(1)
		if calls.Load() == 1 {
			return nil, errors.New("network timeout")
		}
		return []Point{{Amount: 3, Currency: "USD"}}, nil
	})

	first := model.Activate(context.Background())
	require.NotNil(t, first)
	updated, _ := model.Update(first())
	model = updated.(*Model)
	require.Equal(t, LoadStateError, model.State())

	updated, retry := model.Update(tea.KeyPressMsg{Text: "r"})
	model = updated.(*Model)
	require.NotNil(t, retry)
	assert.Equal(t, LoadStateLoading, model.State())

	updated, _ = model.Update(first())
	model = updated.(*Model)
	assert.Equal(t, LoadStateLoading, model.State(), "the first reply is stale")

	updated, _ = model.Update(retry())
	model = updated.(*Model)
	assert.Equal(t, LoadStateLoaded, model.State())
	assert.InDelta(t, 3.0, model.Points()[0].Amount, 0)
}

func TestUpdate_SkippedSourceOmitsSection(t *testing.T) {
	t.Parallel()

	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		return nil, ErrHistorySkipped
	})
	cmd := model.Activate(context.Background())
	require.NotNil(t, cmd)
	updated, _ := model.Update(cmd())
	model = updated.(*Model)
	assert.Equal(t, LoadStateIdle, model.State())
	assert.Empty(t, model.View().Content)
}

func TestUpdate_RetryIgnoredUnlessError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	model := New("urn:web", func(context.Context, string) ([]Point, error) {
		calls.Add(1)
		return []Point{{Amount: 1}}, nil
	})
	_, cmd := model.Update(tea.KeyPressMsg{Text: "r"})
	assert.Nil(t, cmd)
	assert.Equal(t, int32(0), calls.Load())

	cmd = model.Activate(context.Background())
	_, _ = model.Update(cmd())
	_, cmd = model.Update(tea.KeyPressMsg{Text: "r"})
	assert.Nil(t, cmd)
	assert.Equal(t, int32(1), calls.Load())
}
