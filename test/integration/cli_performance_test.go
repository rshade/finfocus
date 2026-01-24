package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/batch"
	"github.com/rshade/finfocus/internal/engine/cache"
)

// testContext creates a context for testing with reasonable timeout.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestCLIPerformance_1000Items verifies that processing 1000 items meets performance requirements:
// - Loads in <2 seconds.
// - Memory usage <100MB.
func TestCLIPerformance_1000Items(t *testing.T) {
	// Load 1000-item test fixture
	fixtureData, err := os.ReadFile("../fixtures/large_dataset_1000.json")
	require.NoError(t, err, "failed to load 1000-item fixture")

	var recommendations []engine.Recommendation
	err = json.Unmarshal(fixtureData, &recommendations)
	require.NoError(t, err, "failed to unmarshal recommendations")
	require.Len(t, recommendations, 1000, "fixture should contain exactly 1000 items")

	// Setup: Create temporary cache directory
	tmpDir := t.TempDir()
	cacheStore, err := cache.NewFileStore(tmpDir, true, 3600, 100)
	require.NoError(t, err, "failed to create cache store")

	// Measure baseline memory usage before processing
	runtime.GC() // Force garbage collection for accurate baseline
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Start timing
	startTime := time.Now()

	// Simulate batch processing as the CLI would do
	processor, err := batch.NewProcessor[engine.Recommendation](100)
	require.NoError(t, err, "failed to create batch processor")

	processedCount := 0
	err = processor.Process(
		testContext(t),
		recommendations,
		func(ctx context.Context, batch []engine.Recommendation, batchIndex int) error {
			// Simulate processing work (filtering, aggregation, etc.)
			for range batch {
				processedCount++
			}
			return nil
		},
	)
	require.NoError(t, err, "batch processing failed")

	// Measure elapsed time
	elapsed := time.Since(startTime)

	// Measure memory usage after processing (using HeapAlloc for more reliable measurement)
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Calculate memory delta in MB using HeapAlloc (more stable than Alloc)
	// Also check TotalAlloc for cumulative allocation
	memoryUsedMB := float64(memAfter.HeapAlloc) / 1024 / 1024
	totalAllocMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024

	// Verify performance requirements
	t.Logf("Performance metrics for 1000 items:")
	t.Logf("  - Elapsed time: %v", elapsed)
	t.Logf("  - Heap memory: %.2f MB", memoryUsedMB)
	t.Logf("  - Total allocated: %.2f MB", totalAllocMB)
	t.Logf("  - Processed items: %d", processedCount)
	t.Logf("  - Batches processed: %d", (len(recommendations)+99)/100)

	// PERFORMANCE REQUIREMENTS
	assert.Less(t, elapsed, 2*time.Second, "should load 1000 items in <2 seconds")
	assert.Less(t, memoryUsedMB, 100.0, "heap memory usage should be <100MB for 1000 items")
	assert.Equal(t, 1000, processedCount, "should process all 1000 items")

	// Verify cache store is functional (bonus validation)
	assert.True(t, cacheStore.IsEnabled(), "cache should be enabled")
}

// TestCLIPerformance_BatchProcessing verifies batch processing efficiency.
func TestCLIPerformance_BatchProcessing(t *testing.T) {
	// Load 1000-item test fixture
	fixtureData, err := os.ReadFile("../fixtures/large_dataset_1000.json")
	require.NoError(t, err, "failed to load 1000-item fixture")

	var recommendations []engine.Recommendation
	err = json.Unmarshal(fixtureData, &recommendations)
	require.NoError(t, err, "failed to unmarshal recommendations")

	// Create batch processor with 100-item batches
	processor, err := batch.NewProcessor[engine.Recommendation](100)
	require.NoError(t, err, "failed to create batch processor")

	// Track batch processing metrics
	batchesProcessed := 0
	itemsProcessed := 0

	// Setup progress callback
	var progressUpdates []float64
	processor = processor.WithProgressCallback(func(progress *batch.Progress) {
		snapshot := progress.Snapshot()
		progressUpdates = append(progressUpdates, snapshot.PercentComplete)
	})

	// Process items in batches
	startTime := time.Now()
	err = processor.Process(
		testContext(t),
		recommendations,
		func(ctx context.Context, batch []engine.Recommendation, batchIndex int) error {
			batchesProcessed++
			itemsProcessed += len(batch)
			return nil
		},
	)
	require.NoError(t, err, "batch processing failed")
	elapsed := time.Since(startTime)

	// Verify batch processing efficiency
	t.Logf("Batch processing metrics:")
	t.Logf("  - Total batches: %d", batchesProcessed)
	t.Logf("  - Items processed: %d", itemsProcessed)
	t.Logf("  - Elapsed time: %v", elapsed)
	t.Logf("  - Progress updates: %d", len(progressUpdates))

	// BATCH PROCESSING REQUIREMENTS
	assert.Equal(t, 10, batchesProcessed, "should process 1000 items in 10 batches of 100")
	assert.Equal(t, 1000, itemsProcessed, "should process all 1000 items")
	assert.Len(t, progressUpdates, 10, "should have 10 progress updates (one per batch)")
	assert.InDelta(
		t,
		100.0,
		progressUpdates[len(progressUpdates)-1],
		0.1,
		"final progress should be 100%",
	)
}

// TestCLIPerformance_CacheEfficiency verifies cache performance with large datasets.
func TestCLIPerformance_CacheEfficiency(t *testing.T) {
	// Setup cache
	tmpDir := t.TempDir()
	cacheStore, err := cache.NewFileStore(tmpDir, true, 3600, 100)
	require.NoError(t, err, "failed to create cache store")

	// Generate large cache key params
	keyParams := cache.KeyParams{
		Operation:     "recommendations",
		Provider:      "multi",
		ResourceTypes: []string{"Instance", "Database", "Bucket", "LoadBalancer", "Cache"},
		Filters: map[string]string{
			"region": "us-east-1",
			"env":    "prod",
		},
		Pagination: &cache.PaginationKeyParams{
			Limit:     1000,
			Offset:    0,
			SortField: "savings",
			SortOrder: "desc",
		},
	}

	// Generate cache key
	startKeyGen := time.Now()
	cacheKey, err := cache.GenerateKey(keyParams)
	keyGenTime := time.Since(startKeyGen)
	require.NoError(t, err, "key generation failed")
	require.Len(t, cacheKey, 64, "cache key should be 64-character SHA256 hash")

	// Create test data (1000 recommendations as JSON)
	testData := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		testData[i] = map[string]interface{}{
			"resource": "aws:Instance:Instance/resource-" + string(rune(i)),
			"savings":  float64(i) * 10.5,
			"action":   "RIGHTSIZE",
		}
	}
	testDataJSON, err := json.Marshal(testData)
	require.NoError(t, err, "failed to marshal test data")

	// Test cache write performance
	startWrite := time.Now()
	err = cacheStore.Set(cacheKey, json.RawMessage(testDataJSON))
	writeTime := time.Since(startWrite)
	require.NoError(t, err, "cache write failed")

	// Test cache read performance
	startRead := time.Now()
	entry, err := cacheStore.Get(cacheKey)
	readTime := time.Since(startRead)
	require.NoError(t, err, "cache read failed")
	require.NotNil(t, entry, "cache entry should exist")

	// Verify cached data integrity
	var retrievedData []map[string]interface{}
	err = json.Unmarshal(entry.Data, &retrievedData)
	require.NoError(t, err, "failed to unmarshal cached data")
	assert.Len(t, retrievedData, 1000, "cached data should contain 1000 items")

	// Log performance metrics
	t.Logf("Cache performance metrics:")
	t.Logf("  - Key generation: %v", keyGenTime)
	t.Logf("  - Cache write: %v", writeTime)
	t.Logf("  - Cache read: %v", readTime)
	t.Logf("  - Cache key: %s (length: %d)", cacheKey, len(cacheKey))

	// CACHE PERFORMANCE REQUIREMENTS
	assert.Less(t, keyGenTime, 10*time.Millisecond, "key generation should be <10ms")
	assert.Less(t, writeTime, 100*time.Millisecond, "cache write should be <100ms for 1000 items")
	assert.Less(t, readTime, 50*time.Millisecond, "cache read should be <50ms for 1000 items")
}
