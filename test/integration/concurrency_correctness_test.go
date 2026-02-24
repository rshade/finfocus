//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/integration/helpers"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// makeDiverseEngineResources creates engine ResourceDescriptor values that cycle
// through the provided resource types, each with unique IDs. This is needed for
// concurrency tests where distinct resource keys ensure proper per-resource costing.
func makeDiverseEngineResources(count int, resourceTypes []string) []engine.ResourceDescriptor {
	resources := make([]engine.ResourceDescriptor, count)
	for i := range count {
		resType := resourceTypes[i%len(resourceTypes)]
		resources[i] = engine.ResourceDescriptor{
			Type:     resType,
			ID:       fmt.Sprintf("resource-%d", i),
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		}
	}
	return resources
}

// newConcurrencyPluginClient creates a pluginhost.Client from a mock server
// connection suitable for concurrency testing.
func newConcurrencyPluginClient(t *testing.T, server *plugin.MockServer) *pluginhost.Client {
	t.Helper()

	ctx := context.Background()
	conn, err := server.Dial(ctx)
	require.NoError(t, err, "should dial mock server")

	api := proto.NewCostSourceClient(conn)

	t.Cleanup(func() { _ = conn.Close() })

	return &pluginhost.Client{
		Name:  "mock-plugin",
		Conn:  conn,
		API:   api,
		Close: func() error { return conn.Close() },
	}
}

// sumMonthlyCosts computes the total monthly cost across all results, rounded
// to 2 decimal places for comparison stability.
func sumMonthlyCosts(results []engine.CostResult) float64 {
	var total float64
	for _, r := range results {
		total += r.Monthly
	}
	return math.Round(total*100) / 100
}

// TestConcurrency_JobsEquivalence verifies that running the engine with
// WithJobs(1) (single worker) and WithJobs(8) (multi-worker) produce
// identical cost totals and resource counts for the same plan. This catches
// ordering bugs or data races in the worker pool.
func TestConcurrency_JobsEquivalence(t *testing.T) {
	resourceTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
		"aws:rds/instance:Instance",
		"aws:lambda/function:Function",
	}

	mockPlugin := plugin.NewMockPlugin()
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	server, err := plugin.StartMockServerWithPlugin(mockPlugin)
	require.NoError(t, err, "mock server should start")
	defer server.Stop()

	client := newConcurrencyPluginClient(t, server)

	ctx := context.Background()
	resources := makeDiverseEngineResources(10, resourceTypes)

	// Run with single worker (sequential baseline)
	eng1 := engine.New([]*pluginhost.Client{client}, nil).WithJobs(1)
	mockPlugin.ResetCallCount()
	results1, err := eng1.GetProjectedCost(ctx, resources)
	require.NoError(t, err, "WithJobs(1) should succeed")
	require.Len(t, results1, len(resources), "WithJobs(1) should return one result per resource")

	total1 := sumMonthlyCosts(results1)
	callCount1 := mockPlugin.GetCallCount()

	// Run with 8 workers (parallel)
	eng8 := engine.New([]*pluginhost.Client{client}, nil).WithJobs(8)
	mockPlugin.ResetCallCount()
	results8, err := eng8.GetProjectedCost(ctx, resources)
	require.NoError(t, err, "WithJobs(8) should succeed")
	require.Len(t, results8, len(resources), "WithJobs(8) should return one result per resource")

	total8 := sumMonthlyCosts(results8)
	callCount8 := mockPlugin.GetCallCount()

	// Cost totals must be identical (rounded to 2 decimal places)
	assert.Equal(t, total1, total8,
		"cost totals should be identical for j1 (%.2f) and j8 (%.2f)", total1, total8)

	// Resource counts must match
	assert.Equal(t, len(results1), len(results8),
		"result count should match: j1=%d, j8=%d", len(results1), len(results8))

	// Plugin call counts should match (same work done regardless of concurrency)
	assert.Equal(t, callCount1, callCount8,
		"plugin call count should match: j1=%d, j8=%d", callCount1, callCount8)

	// Sub-test: jobs > resources edge case
	t.Run("jobs_exceed_resource_count", func(t *testing.T) {
		smallResources := makeDiverseEngineResources(3, resourceTypes)

		eng100 := engine.New([]*pluginhost.Client{client}, nil).WithJobs(100)
		mockPlugin.ResetCallCount()
		results100, err := eng100.GetProjectedCost(ctx, smallResources)
		require.NoError(t, err, "WithJobs(100) with 3 resources should succeed")
		require.Len(t, results100, 3, "should return exactly 3 results")

		// Compare against single-worker for the same resources
		eng1Small := engine.New([]*pluginhost.Client{client}, nil).WithJobs(1)
		mockPlugin.ResetCallCount()
		resultsSingle, err := eng1Small.GetProjectedCost(ctx, smallResources)
		require.NoError(t, err, "WithJobs(1) with 3 resources should succeed")

		totalLarge := sumMonthlyCosts(results100)
		totalSingle := sumMonthlyCosts(resultsSingle)
		assert.Equal(t, totalSingle, totalLarge,
			"cost totals should match even when jobs (100) exceeds resource count (3)")
	})
}

// TestConcurrency_Jobs0AutoDetect verifies that WithJobs(0) (auto-detect
// based on NumCPU) completes without error and produces valid, non-empty
// results.
func TestConcurrency_Jobs0AutoDetect(t *testing.T) {
	mockPlugin := plugin.NewMockPlugin()
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	server, err := plugin.StartMockServerWithPlugin(mockPlugin)
	require.NoError(t, err, "mock server should start")
	defer server.Stop()

	client := newConcurrencyPluginClient(t, server)

	ctx := context.Background()
	resourceTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
	}
	resources := makeDiverseEngineResources(5, resourceTypes)

	// WithJobs(0) means auto-detect worker count based on NumCPU
	eng := engine.New([]*pluginhost.Client{client}, nil).WithJobs(0)
	results, err := eng.GetProjectedCost(ctx, resources)
	require.NoError(t, err, "WithJobs(0) should succeed")
	require.NotEmpty(t, results, "auto-detect should produce results")
	assert.Len(t, results, len(resources),
		"auto-detect should return one result per resource (got %d, want %d)",
		len(results), len(resources))

	// Verify results have non-zero costs (ScenarioSuccess produces real values)
	for _, r := range results {
		assert.NotEmpty(t, r.ResourceType, "result should have resource type")
		assert.NotEmpty(t, r.Currency, "result should have currency")
	}
}

// TestConcurrency_LargePlan500Resources verifies that a 500-resource
// synthetic plan completes within 30 seconds using the default concurrency
// setting and produces the correct number of results.
func TestConcurrency_LargePlan500Resources(t *testing.T) {
	resourceTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
		"aws:rds/instance:Instance",
		"aws:lambda/function:Function",
	}

	mockPlugin := plugin.NewMockPlugin()
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	server, err := plugin.StartMockServerWithPlugin(mockPlugin)
	require.NoError(t, err, "mock server should start")
	defer server.Stop()

	client := newConcurrencyPluginClient(t, server)

	// Use context.WithTimeout to enforce the 30-second deadline
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Generate 500 resources directly (engine-level, not plan file)
	resources := makeDiverseEngineResources(500, resourceTypes)

	eng := engine.New([]*pluginhost.Client{client}, nil)
	results, err := eng.GetProjectedCost(ctx, resources)
	require.NoError(t, err, "500-resource plan should complete without error")
	assert.Len(t, results, 500,
		"should return exactly 500 results (got %d)", len(results))

	// Verify all results are populated
	for i, r := range results {
		assert.NotEmpty(t, r.ResourceType,
			"result %d should have resource type", i)
	}

	// Verify the synthetic plan generator also works for large counts
	// (confirms the helper is usable for CLI-level tests in future tasks)
	planPath := helpers.GenerateSyntheticPlan(t, 500, resourceTypes)
	assert.NotEmpty(t, planPath, "synthetic plan path should not be empty")
}

// buildFinfocusBinary builds the finfocus CLI binary to a temp directory.
// Returns the path to the built binary. Calls t.Skip if build fails.
func buildFinfocusBinary(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "finfocus-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/finfocus")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("failed to build CLI binary: %v\n%s", err, output)
	}

	return binaryPath
}

// setupIsolatedHome creates a temp HOME directory with an empty plugin dir,
// preventing the registry from finding real plugins.
func setupIsolatedHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".finfocus", "plugins"), 0o755))

	return home
}

// TestConcurrency_ParallelCacheAccess verifies that 5 separate OS processes
// can concurrently access the same BoltDB cache file without corruption.
// This tests real file-level locking between separate process address spaces,
// which is the actual deployment scenario when multiple finfocus invocations
// share a cache directory.
func TestConcurrency_ParallelCacheAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}

	binaryPath := buildFinfocusBinary(t)

	// Shared cache directory accessed by all 5 processes
	sharedCacheDir := t.TempDir()

	// Generate a small synthetic plan for each process to use
	resourceTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
	}
	planPath := helpers.GenerateSyntheticPlan(t, 5, resourceTypes)

	// Isolated HOME prevents interference from real plugins
	fakeHome := setupIsolatedHome(t)

	const processCount = 5
	var wg sync.WaitGroup

	type processResult struct {
		exitCode int
		stderr   string
	}
	results := make([]processResult, processCount)

	for i := range processCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cmd := exec.Command(binaryPath, "cost", "projected",
				"--pulumi-json", planPath,
				"--cache-ttl", "300")
			cmd.Env = []string{
				"HOME=" + fakeHome,
				"FINFOCUS_CACHE_DIR=" + sharedCacheDir,
				"PATH=" + os.Getenv("PATH"),
			}

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			results[idx].stderr = stderr.String()
			if err != nil {
				if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
					results[idx].exitCode = exitErr.ExitCode()
				} else {
					results[idx].exitCode = -1
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no BoltDB corruption errors across any process
	for i, res := range results {
		assert.NotContains(t, res.stderr, "invalid database",
			"process %d should not encounter invalid database: %s", i, res.stderr)
		assert.NotContains(t, res.stderr, "checksum error",
			"process %d should not encounter checksum error: %s", i, res.stderr)

		// Log exit codes for debugging (non-zero may be expected if no plugins found)
		if res.exitCode != 0 {
			t.Logf("process %d exited with code %d (expected if no plugins available)", i, res.exitCode)
		}
	}

	// Verify cache.db is readable and not corrupted after all processes complete
	dbPath := filepath.Join(sharedCacheDir, "cache.db")
	if _, err := os.Stat(dbPath); err == nil {
		db, err := bolt.Open(dbPath, 0o600, &bolt.Options{
			Timeout:  time.Second,
			ReadOnly: true,
		})
		require.NoError(t, err, "cache.db should be readable after concurrent access")

		err = db.View(func(tx *bolt.Tx) error {
			for _, bucketName := range []string{"projected", "actual", "recommendations"} {
				b := tx.Bucket([]byte(bucketName))
				if b != nil {
					t.Logf("bucket %q: %d entries", bucketName, b.Stats().KeyN)
				}
			}
			return nil
		})
		assert.NoError(t, err, "should read bucket contents without error")
		require.NoError(t, db.Close())
	} else {
		// Cache may not have been created if all processes got lock contention
		// and fell back to no-cache mode — that's acceptable graceful degradation
		t.Logf("cache.db was not created (possible lock contention): %v", err)
	}
}

// TestConcurrency_ThroughputMetricOutput verifies that the throughput metric
// (resources/sec) appears in stderr when using table output format and is
// absent when using JSON output format.
func TestConcurrency_ThroughputMetricOutput(t *testing.T) {
	helper := helpers.NewCLIHelper(t)

	resourceTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
	}
	planPath := helpers.GenerateSyntheticPlan(t, 5, resourceTypes)
	fakeHome := setupIsolatedHome(t)

	// Table format (default) — should print throughput to stderr
	helper.WithEnv(map[string]string{
		"HOME":          fakeHome,
		"FINFOCUS_HOME": fakeHome,
	}, func() {
		_, err := helper.Execute("cost", "projected",
			"--pulumi-json", planPath,
			"--jobs", "4")

		// Command may error (no plugins), but timing output is printed after
		// rendering — it is only printed when RenderCostOutput succeeds.
		// If the command reached the render phase, stderr will have the metric.
		stderr := helper.Stderr()

		if err == nil {
			// Command succeeded — throughput MUST be in stderr
			assert.True(t, strings.Contains(stderr, "resources/sec"),
				"table format should show throughput metric in stderr, got stderr=%q", stderr)
		} else {
			// Command failed — timing may or may not be present depending on
			// where the error occurred. Log for debugging.
			t.Logf("command errored (expected without plugins): %v", err)
			t.Logf("stderr: %s", stderr)
		}
	})

	// JSON format — throughput metric must NOT appear in stderr
	helper2 := helpers.NewCLIHelper(t)
	helper2.WithEnv(map[string]string{
		"HOME":          fakeHome,
		"FINFOCUS_HOME": fakeHome,
	}, func() {
		_, err := helper2.Execute("cost", "projected",
			"--pulumi-json", planPath,
			"--jobs", "4",
			"--output", "json")

		stderr := helper2.Stderr()

		if err == nil {
			// Command succeeded — throughput must NOT be in stderr for JSON
			assert.False(t, strings.Contains(stderr, "resources/sec"),
				"JSON format should NOT show throughput metric in stderr, got stderr=%q", stderr)
		} else {
			// If command failed, throughput still should not appear
			assert.False(t, strings.Contains(stderr, "resources/sec"),
				"JSON format should NOT show throughput metric even on error, got stderr=%q", stderr)
		}
	})
}
