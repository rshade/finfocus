// Package batch provides utilities for processing large datasets in fixed-size batches.
//
// This package implements batch processing to prevent memory exhaustion when handling
// datasets with hundreds or thousands of items. Key features:
//   - Configurable batch size (default 100 items per batch)
//   - Progress tracking with callbacks for UI updates
//   - Context-aware cancellation support
//   - Memory-efficient processing (O(batch_size) memory overhead)
//
// Batch processing is used throughout the engine to ensure consistent memory usage
// regardless of dataset size, meeting the <100MB memory target for 1000+ items.
package batch
