package batch

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Default batch processing configuration.
const (
	// DefaultBatchSize is the default number of items per batch.
	DefaultBatchSize = 100

	// MinBatchSize is the minimum allowed batch size.
	MinBatchSize = 1

	// MaxBatchSize is the maximum allowed batch size.
	MaxBatchSize = 1000
)

// Common batch processing errors.
var (
	ErrInvalidBatchSize = errors.New("batch size must be between 1 and 1000")
	ErrNilCallback      = errors.New("batch callback cannot be nil")
	ErrEmptyItems       = errors.New("items slice cannot be empty")
)

// BatchCallback is a function that processes a single batch of items.
// It receives the batch items, batch index (0-based), and should return an error if processing fails.
//
//nolint:revive // BatchCallback is the canonical name for this exported type.
type BatchCallback[T any] func(ctx context.Context, batch []T, batchIndex int) error

// ProgressCallback is an optional callback invoked after each batch is processed.
// It receives progress information for UI updates or logging.
type ProgressCallback func(progress *Progress)

// Processor provides batch processing capabilities for large datasets.
// It splits data into fixed-size batches and processes them sequentially or concurrently.
type Processor[T any] struct {
	// batchSize is the number of items per batch.
	batchSize int

	// onProgress is an optional callback for progress updates.
	onProgress ProgressCallback

	// mu protects concurrent access to progress tracking.
	mu sync.Mutex
}

// NewProcessor creates a new batch processor with the given batch size.
func NewProcessor[T any](batchSize int) (*Processor[T], error) {
	if batchSize < MinBatchSize || batchSize > MaxBatchSize {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidBatchSize, batchSize)
	}

	return &Processor[T]{
		batchSize: batchSize,
	}, nil
}

// NewProcessorWithDefaults creates a processor with default batch size.
func NewProcessorWithDefaults[T any]() *Processor[T] {
	return &Processor[T]{
		batchSize: DefaultBatchSize,
	}
}

// WithProgressCallback sets a progress callback for the processor.
func (p *Processor[T]) WithProgressCallback(callback ProgressCallback) *Processor[T] {
	p.onProgress = callback
	return p
}

// Process processes items in batches using the provided callback.
// Processing is sequential and stops on the first error.
func (p *Processor[T]) Process(ctx context.Context, items []T, callback BatchCallback[T]) error {
	if len(items) == 0 {
		return ErrEmptyItems
	}

	if callback == nil {
		return ErrNilCallback
	}

	totalBatches := p.calculateTotalBatches(len(items))
	progress := NewProgress(len(items), totalBatches, p.batchSize)

	for batchIndex := range totalBatches {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate batch boundaries
		start := batchIndex * p.batchSize
		end := start + p.batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[start:end]

		// Process batch
		if err := callback(ctx, batch, batchIndex); err != nil {
			return fmt.Errorf("batch %d failed: %w", batchIndex, err)
		}

		// Update progress
		p.updateProgress(progress, len(batch))

		// Notify progress callback
		if p.onProgress != nil {
			p.onProgress(progress)
		}
	}

	return nil
}

// ProcessConcurrent processes batches concurrently with a maximum concurrency limit.
// Errors from individual batches are collected and returned as a multi-error.
func (p *Processor[T]) ProcessConcurrent(
	ctx context.Context,
	items []T,
	callback BatchCallback[T],
	maxConcurrency int,
) error {
	if len(items) == 0 {
		return ErrEmptyItems
	}

	if callback == nil {
		return ErrNilCallback
	}

	if maxConcurrency < 1 {
		maxConcurrency = 1
	}

	totalBatches := p.calculateTotalBatches(len(items))
	progress := NewProgress(len(items), totalBatches, p.batchSize)

	// Semaphore for concurrency control
	sem := make(chan struct{}, maxConcurrency)
	errChan := make(chan error, totalBatches)
	var wg sync.WaitGroup

	for batchIndex := range totalBatches {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate batch boundaries
		start := batchIndex * p.batchSize
		end := start + p.batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[start:end]
		batchIdx := batchIndex

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Process batch
			if err := callback(ctx, batch, batchIdx); err != nil {
				errChan <- fmt.Errorf("batch %d failed: %w", batchIdx, err)
				return
			}

			// Update progress
			p.updateProgress(progress, len(batch))

			// Notify progress callback
			if p.onProgress != nil {
				p.onProgress(progress)
			}
		}()
	}

	// Wait for all batches to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("batch processing failed: %d errors occurred: %v", len(errs), errs)
	}

	return nil
}

// GetBatchSize returns the configured batch size.
func (p *Processor[T]) GetBatchSize() int {
	return p.batchSize
}

// CalculateBatches returns the batch boundaries for the given items.
// Returns a slice of [start, end) index pairs.
func (p *Processor[T]) CalculateBatches(totalItems int) [][2]int {
	totalBatches := p.calculateTotalBatches(totalItems)
	batches := make([][2]int, totalBatches)

	for i := range totalBatches {
		start := i * p.batchSize
		end := start + p.batchSize
		if end > totalItems {
			end = totalItems
		}
		batches[i] = [2]int{start, end}
	}

	return batches
}

// calculateTotalBatches calculates the number of batches needed for the given item count.
func (p *Processor[T]) calculateTotalBatches(totalItems int) int {
	batches := totalItems / p.batchSize
	if totalItems%p.batchSize > 0 {
		batches++
	}
	return batches
}

// updateProgress safely updates the progress state.
func (p *Processor[T]) updateProgress(progress *Progress, itemsProcessed int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	progress.AddProcessed(itemsProcessed)
}
