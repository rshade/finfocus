package batch_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/batch"
)

// TestNewProcessor verifies processor creation with various batch sizes.
func TestNewProcessor(t *testing.T) {
	tests := []struct {
		name      string
		batchSize int
		wantErr   bool
	}{
		{
			name:      "valid default batch size",
			batchSize: 100,
			wantErr:   false,
		},
		{
			name:      "minimum batch size",
			batchSize: 1,
			wantErr:   false,
		},
		{
			name:      "maximum batch size",
			batchSize: 1000,
			wantErr:   false,
		},
		{
			name:      "batch size too small",
			batchSize: 0,
			wantErr:   true,
		},
		{
			name:      "batch size too large",
			batchSize: 1001,
			wantErr:   true,
		},
		{
			name:      "negative batch size",
			batchSize: -1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := batch.NewProcessor[int](tt.batchSize)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, p)
				assert.ErrorIs(t, err, batch.ErrInvalidBatchSize)
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, tt.batchSize, p.GetBatchSize())
			}
		})
	}
}

// TestNewProcessorWithDefaults verifies default processor creation.
func TestNewProcessorWithDefaults(t *testing.T) {
	p := batch.NewProcessorWithDefaults[string]()
	require.NotNil(t, p)
	assert.Equal(t, batch.DefaultBatchSize, p.GetBatchSize())
}

// TestProcess_1000Items verifies processing of 1000-item dataset in 100-item batches.
func TestProcess_1000Items(t *testing.T) {
	ctx := context.Background()

	// Generate 1000-item test dataset
	items := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = i
	}

	// Track processed items and batches
	var processedItems []int
	var batchSizes []int
	var batchIndices []int
	var mu sync.Mutex

	// Callback to collect batch information
	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		mu.Lock()
		defer mu.Unlock()
		processedItems = append(processedItems, batch...)
		batchSizes = append(batchSizes, len(batch))
		batchIndices = append(batchIndices, batchIndex)
		return nil
	}

	// Create processor with 100-item batches
	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	// Process items
	err = p.Process(ctx, items, callback)
	require.NoError(t, err)

	// Verify all items were processed
	assert.Len(t, processedItems, 1000)
	assert.ElementsMatch(t, items, processedItems)

	// Verify batch count (1000 items / 100 per batch = 10 batches)
	assert.Len(t, batchSizes, 10)

	// Verify each batch has 100 items
	for i, size := range batchSizes {
		assert.Equal(t, 100, size, "Batch %d should have 100 items", i)
	}

	// Verify batch indices are sequential
	for i, idx := range batchIndices {
		assert.Equal(t, i, idx, "Batch index should be %d", i)
	}
}

// TestProcess_UnevenBatch verifies handling of dataset not evenly divisible by batch size.
func TestProcess_UnevenBatch(t *testing.T) {
	ctx := context.Background()

	// Generate 1050-item dataset (not evenly divisible by 100)
	items := make([]int, 1050)
	for i := 0; i < 1050; i++ {
		items[i] = i
	}

	var batchSizes []int
	var mu sync.Mutex

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		mu.Lock()
		defer mu.Unlock()
		batchSizes = append(batchSizes, len(batch))
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.Process(ctx, items, callback)
	require.NoError(t, err)

	// Verify 11 batches (10 full + 1 partial)
	require.Len(t, batchSizes, 11)

	// Verify first 10 batches have 100 items
	for i := 0; i < 10; i++ {
		assert.Equal(t, 100, batchSizes[i], "Batch %d should have 100 items", i)
	}

	// Verify last batch has remaining 50 items
	assert.Equal(t, 50, batchSizes[10], "Last batch should have 50 items")
}

// TestProcess_ProgressCallback verifies progress callback invocation.
func TestProcess_ProgressCallback(t *testing.T) {
	ctx := context.Background()

	items := make([]int, 300)
	for i := 0; i < 300; i++ {
		items[i] = i
	}

	var progressUpdates []struct {
		current int
		total   int
		percent float64
	}
	var mu sync.Mutex

	progressCallback := func(progress *batch.Progress) {
		mu.Lock()
		defer mu.Unlock()
		snapshot := progress.Snapshot()
		progressUpdates = append(progressUpdates, struct {
			current int
			total   int
			percent float64
		}{
			current: snapshot.ProcessedItems,
			total:   snapshot.TotalItems,
			percent: snapshot.PercentComplete,
		})
	}

	batchCallback := func(ctx context.Context, batch []int, batchIndex int) error {
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	p = p.WithProgressCallback(progressCallback)

	err = p.Process(ctx, items, batchCallback)
	require.NoError(t, err)

	// Verify progress callback was called 3 times (one per batch)
	require.Len(t, progressUpdates, 3)

	// Verify progress values
	assert.Equal(t, 100, progressUpdates[0].current)
	assert.Equal(t, 300, progressUpdates[0].total)
	assert.InDelta(t, 33.33, progressUpdates[0].percent, 0.1)

	assert.Equal(t, 200, progressUpdates[1].current)
	assert.InDelta(t, 66.67, progressUpdates[1].percent, 0.1)

	assert.Equal(t, 300, progressUpdates[2].current)
	assert.InDelta(t, 100.0, progressUpdates[2].percent, 0.1)
}

// TestProcess_ErrorHandling verifies error handling and early termination.
func TestProcess_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	items := make([]int, 500)
	for i := 0; i < 500; i++ {
		items[i] = i
	}

	var processedBatches int
	testErr := errors.New("batch processing error")

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		processedBatches++
		// Fail on batch 3
		if batchIndex == 2 {
			return testErr
		}
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.Process(ctx, items, callback)
	require.Error(t, err)
	assert.ErrorIs(t, err, testErr)
	assert.Contains(t, err.Error(), "batch 2 failed")

	// Verify processing stopped after batch 2 (batches 0, 1, 2 processed)
	assert.Equal(t, 3, processedBatches)
}

// TestProcess_ContextCancellation verifies context cancellation handling.
func TestProcess_ContextCancellation(t *testing.T) {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	items := make([]int, 500)
	for i := 0; i < 500; i++ {
		items[i] = i
	}

	var processedBatches int
	var mu sync.Mutex

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		mu.Lock()
		processedBatches++
		mu.Unlock()

		// Cancel context after batch 2
		if batchIndex == 2 {
			cancel()
		}

		// Simulate processing time
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.Process(ctx, items, callback)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	// Verify processing stopped after cancellation (batches 0, 1, 2 processed)
	mu.Lock()
	assert.Equal(t, 3, processedBatches)
	mu.Unlock()
}

// TestProcess_EmptyItems verifies error handling for empty input.
func TestProcess_EmptyItems(t *testing.T) {
	ctx := context.Background()

	items := []int{}

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		t.Fatal("callback should not be called for empty items")
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.Process(ctx, items, callback)
	require.Error(t, err)
	assert.ErrorIs(t, err, batch.ErrEmptyItems)
}

// TestProcess_NilCallback verifies error handling for nil callback.
func TestProcess_NilCallback(t *testing.T) {
	ctx := context.Background()

	items := []int{1, 2, 3}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.Process(ctx, items, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, batch.ErrNilCallback)
}

// TestProcessConcurrent_1000Items verifies concurrent processing of 1000-item dataset.
func TestProcessConcurrent_1000Items(t *testing.T) {
	ctx := context.Background()

	items := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = i
	}

	var processedItems []int
	var mu sync.Mutex

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		// Simulate processing time
		time.Sleep(1 * time.Millisecond)

		mu.Lock()
		processedItems = append(processedItems, batch...)
		mu.Unlock()
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	// Process with concurrency of 4
	err = p.ProcessConcurrent(ctx, items, callback, 4)
	require.NoError(t, err)

	// Verify all items were processed
	mu.Lock()
	assert.Len(t, processedItems, 1000)
	// Note: items may not be in order due to concurrent processing
	assert.ElementsMatch(t, items, processedItems)
	mu.Unlock()
}

// TestProcessConcurrent_ErrorCollection verifies error collection in concurrent processing.
func TestProcessConcurrent_ErrorCollection(t *testing.T) {
	ctx := context.Background()

	items := make([]int, 500)
	for i := 0; i < 500; i++ {
		items[i] = i
	}

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		// Fail batches 1 and 3
		if batchIndex == 1 || batchIndex == 3 {
			return fmt.Errorf("batch %d error", batchIndex)
		}
		return nil
	}

	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.ProcessConcurrent(ctx, items, callback, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch processing failed")
	assert.Contains(t, err.Error(), "2 errors occurred")
}

// TestCalculateBatches verifies batch boundary calculation.
func TestCalculateBatches(t *testing.T) {
	tests := []struct {
		name       string
		batchSize  int
		totalItems int
		want       [][2]int
	}{
		{
			name:       "evenly divisible",
			batchSize:  100,
			totalItems: 300,
			want: [][2]int{
				{0, 100},
				{100, 200},
				{200, 300},
			},
		},
		{
			name:       "not evenly divisible",
			batchSize:  100,
			totalItems: 350,
			want: [][2]int{
				{0, 100},
				{100, 200},
				{200, 300},
				{300, 350},
			},
		},
		{
			name:       "single batch",
			batchSize:  100,
			totalItems: 50,
			want: [][2]int{
				{0, 50},
			},
		},
		{
			name:       "exact batch size",
			batchSize:  100,
			totalItems: 100,
			want: [][2]int{
				{0, 100},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := batch.NewProcessor[int](tt.batchSize)
			require.NoError(t, err)

			batches := p.CalculateBatches(tt.totalItems)
			assert.Equal(t, tt.want, batches)
		})
	}
}

// TestWithProgressCallback_Chaining verifies method chaining.
func TestWithProgressCallback_Chaining(t *testing.T) {
	called := false
	callback := func(progress *batch.Progress) {
		called = true
	}

	p := batch.NewProcessorWithDefaults[int]()
	p2 := p.WithProgressCallback(callback)

	// Verify chaining returns the same processor
	assert.Equal(t, p, p2)

	// Verify callback was set by processing items
	ctx := context.Background()
	items := []int{1, 2, 3}
	batchCallback := func(ctx context.Context, batch []int, batchIndex int) error {
		return nil
	}

	err := p.Process(ctx, items, batchCallback)
	require.NoError(t, err)
	assert.True(t, called, "Progress callback should have been called")
}

// TestProcess_LargeBatchSize verifies handling of batch size larger than dataset.
func TestProcess_LargeBatchSize(t *testing.T) {
	ctx := context.Background()

	items := []int{1, 2, 3, 4, 5}

	var batchSizes []int
	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		batchSizes = append(batchSizes, len(batch))
		return nil
	}

	// Create processor with batch size larger than dataset
	p, err := batch.NewProcessor[int](100)
	require.NoError(t, err)

	err = p.Process(ctx, items, callback)
	require.NoError(t, err)

	// Verify only one batch was created
	require.Len(t, batchSizes, 1)
	assert.Equal(t, 5, batchSizes[0])
}

// BenchmarkProcess_1000Items benchmarks processing of 1000 items.
func BenchmarkProcess_1000Items(b *testing.B) {
	ctx := context.Background()

	items := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = i
	}

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		// Simulate minimal processing
		return nil
	}

	p, _ := batch.NewProcessor[int](100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Process(ctx, items, callback)
	}
}

// BenchmarkProcessConcurrent_1000Items benchmarks concurrent processing.
func BenchmarkProcessConcurrent_1000Items(b *testing.B) {
	ctx := context.Background()

	items := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = i
	}

	callback := func(ctx context.Context, batch []int, batchIndex int) error {
		// Simulate minimal processing
		time.Sleep(100 * time.Microsecond)
		return nil
	}

	p, _ := batch.NewProcessor[int](100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.ProcessConcurrent(ctx, items, callback, 4)
	}
}
