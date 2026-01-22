package batch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_Process(t *testing.T) {
	items := make([]int, 25)
	for i := range items {
		items[i] = i
	}

	t.Run("Sequential", func(t *testing.T) {
		p, _ := NewProcessor[int](10)
		var processedCount int32
		var batches int32

		callback := func(ctx context.Context, batch []int, batchIndex int) error {
			atomic.AddInt32(&batches, 1)
			atomic.AddInt32(&processedCount, int32(len(batch)))
			return nil
		}

		err := p.Process(context.Background(), items, callback)
		require.NoError(t, err)
		assert.Equal(t, int32(25), processedCount)
		assert.Equal(t, int32(3), batches)
	})

	t.Run("Concurrent", func(t *testing.T) {
		p, _ := NewProcessor[int](5)
		var processedCount int32

		callback := func(ctx context.Context, batch []int, batchIndex int) error {
			atomic.AddInt32(&processedCount, int32(len(batch)))
			return nil
		}

		err := p.ProcessConcurrent(context.Background(), items, callback, 2)
		require.NoError(t, err)
		assert.Equal(t, int32(25), processedCount)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		p, _ := NewProcessor[int](10)
		callback := func(ctx context.Context, batch []int, batchIndex int) error {
			if batchIndex == 1 {
				return errors.New("fail")
			}
			return nil
		}

		err := p.Process(context.Background(), items, callback)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch 1 failed")
	})

	t.Run("EmptyItems", func(t *testing.T) {
		p := NewProcessorWithDefaults[int]()
		err := p.Process(context.Background(), nil, nil)
		assert.Equal(t, ErrEmptyItems, err)
	})

	t.Run("NilCallback", func(t *testing.T) {
		p := NewProcessorWithDefaults[int]()
		err := p.Process(context.Background(), items, nil)
		assert.Equal(t, ErrNilCallback, err)
	})

	t.Run("InvalidBatchSize", func(t *testing.T) {
		_, err := NewProcessor[int](0)
		assert.Error(t, err)
		_, err = NewProcessor[int](2000)
		assert.Error(t, err)
	})
}

func TestProgress(t *testing.T) {
	totalItems := 100
	totalBatches := 10
	batchSize := 10
	p := NewProgress(totalItems, totalBatches, batchSize)

	assert.Equal(t, 0.0, p.PercentComplete())
	assert.False(t, p.IsComplete())

	p.AddProcessed(10)
	assert.Equal(t, 10.0, p.PercentComplete())
	assert.Equal(t, 10, p.ProcessedItems)
	assert.Equal(t, 1, p.ProcessedBatches)

	p.AddProcessed(90)
	assert.Equal(t, 100.0, p.PercentComplete())
	assert.True(t, p.IsComplete())
	assert.Greater(t, p.ElapsedTime(), time.Duration(0))

	t.Run("Estimates", func(t *testing.T) {
		p.Reset()
		p.AddProcessed(50)
		assert.Greater(t, p.ItemsPerSecond(), 0.0)
		assert.Greater(t, p.BatchesPerSecond(), 0.0)
		// Estimated remaining should be around elapsed time since 50% done
		assert.NotNil(t, p.EstimatedTimeRemaining())
	})

	t.Run("Snapshot", func(t *testing.T) {
		snap := p.Snapshot()
		assert.Equal(t, p.TotalItems, snap.TotalItems)
		assert.Equal(t, p.ProcessedItems, snap.ProcessedItems)
	})
}

func TestProcessor_CalculateBatches(t *testing.T) {
	p, _ := NewProcessor[int](10)
	batches := p.CalculateBatches(25)
	require.Len(t, batches, 3)
	assert.Equal(t, [2]int{0, 10}, batches[0])
	assert.Equal(t, [2]int{10, 20}, batches[1])
	assert.Equal(t, [2]int{20, 25}, batches[2])
	assert.Equal(t, 10, p.GetBatchSize())
}
