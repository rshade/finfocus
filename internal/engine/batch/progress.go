package batch

import (
	"sync"
	"time"
)

// percentMultiplier is used to convert a ratio to percentage (0-100).
const percentMultiplier = 100

// Progress tracks the progress of batch processing operations.
// It provides thread-safe access to progress metrics for UI updates.
type Progress struct {
	// TotalItems is the total number of items to process.
	TotalItems int

	// ProcessedItems is the number of items processed so far.
	ProcessedItems int

	// TotalBatches is the total number of batches.
	TotalBatches int

	// ProcessedBatches is the number of batches processed so far.
	ProcessedBatches int

	// BatchSize is the configured batch size.
	BatchSize int

	// StartTime is when processing started.
	StartTime time.Time

	// LastUpdateTime is when progress was last updated.
	LastUpdateTime time.Time

	// mu protects concurrent access to progress fields.
	mu sync.RWMutex
}

// NewProgress creates a new progress tracker.
func NewProgress(totalItems, totalBatches, batchSize int) *Progress {
	now := time.Now()
	return &Progress{
		TotalItems:       totalItems,
		ProcessedItems:   0,
		TotalBatches:     totalBatches,
		ProcessedBatches: 0,
		BatchSize:        batchSize,
		StartTime:        now,
		LastUpdateTime:   now,
	}
}

// AddProcessed increments the processed items and batches count.
// This method is thread-safe.
func (p *Progress) AddProcessed(itemsProcessed int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ProcessedItems += itemsProcessed
	p.ProcessedBatches++
	p.LastUpdateTime = time.Now()
}

// PercentComplete returns the completion percentage (0-100).
func (p *Progress) PercentComplete() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalItems == 0 {
		return 0
	}
	return (float64(p.ProcessedItems) / float64(p.TotalItems)) * percentMultiplier
}

// IsComplete returns true if all items have been processed.
func (p *Progress) IsComplete() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.ProcessedItems >= p.TotalItems
}

// ElapsedTime returns the time elapsed since processing started.
func (p *Progress) ElapsedTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return time.Since(p.StartTime)
}

// EstimatedTimeRemaining estimates the remaining processing time based on current progress.
// Returns 0 if no items have been processed yet.
func (p *Progress) EstimatedTimeRemaining() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.ProcessedItems == 0 {
		return 0
	}

	elapsed := time.Since(p.StartTime)
	avgTimePerItem := elapsed / time.Duration(p.ProcessedItems)
	remainingItems := p.TotalItems - p.ProcessedItems

	return avgTimePerItem * time.Duration(remainingItems)
}

// ItemsPerSecond returns the processing rate in items per second.
func (p *Progress) ItemsPerSecond() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	elapsed := time.Since(p.StartTime).Seconds()
	if elapsed == 0 {
		return 0
	}

	return float64(p.ProcessedItems) / elapsed
}

// BatchesPerSecond returns the processing rate in batches per second.
func (p *Progress) BatchesPerSecond() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	elapsed := time.Since(p.StartTime).Seconds()
	if elapsed == 0 {
		return 0
	}

	return float64(p.ProcessedBatches) / elapsed
}

// Snapshot returns a thread-safe copy of the current progress state.
func (p *Progress) Snapshot() ProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProgressSnapshot{
		TotalItems:       p.TotalItems,
		ProcessedItems:   p.ProcessedItems,
		TotalBatches:     p.TotalBatches,
		ProcessedBatches: p.ProcessedBatches,
		BatchSize:        p.BatchSize,
		StartTime:        p.StartTime,
		LastUpdateTime:   p.LastUpdateTime,
		PercentComplete:  p.percentCompleteUnsafe(),
		ElapsedTime:      time.Since(p.StartTime),
		ItemsPerSecond:   p.itemsPerSecondUnsafe(),
	}
}

// ProgressSnapshot is an immutable snapshot of progress state.
type ProgressSnapshot struct {
	TotalItems       int
	ProcessedItems   int
	TotalBatches     int
	ProcessedBatches int
	BatchSize        int
	StartTime        time.Time
	LastUpdateTime   time.Time
	PercentComplete  float64
	ElapsedTime      time.Duration
	ItemsPerSecond   float64
}

// percentCompleteUnsafe calculates percent complete without locking.
// Should only be called when already holding the lock.
func (p *Progress) percentCompleteUnsafe() float64 {
	if p.TotalItems == 0 {
		return 0
	}
	return (float64(p.ProcessedItems) / float64(p.TotalItems)) * percentMultiplier
}

// itemsPerSecondUnsafe calculates items per second without locking.
// Should only be called when already holding the lock.
func (p *Progress) itemsPerSecondUnsafe() float64 {
	elapsed := time.Since(p.StartTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(p.ProcessedItems) / elapsed
}

// Reset resets the progress tracker to initial state.
func (p *Progress) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	p.ProcessedItems = 0
	p.ProcessedBatches = 0
	p.StartTime = now
	p.LastUpdateTime = now
}
