package cli

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/history"
)

// storeEnv points the cache and history at temp directories with both enabled.
func storeEnv(t *testing.T) (string, string) {
	t.Helper()
	cacheDir, historyDir := t.TempDir(), t.TempDir()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.CacheEnvDir, cacheDir)
	t.Setenv(config.CacheEnvTTLSeconds, "300")
	t.Setenv(config.HistoryEnvDir, historyDir)
	t.Setenv(config.HistoryEnvEnabled, "true")
	return cacheDir, historyDir
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestOpenSharedStores_Live(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()

	stores := openSharedStores(ctx, &cobra.Command{Use: "test"}, config.New())
	t.Cleanup(func() { stores.Close(ctx) })

	require.NotNil(t, stores.cacheStore())
	require.NotNil(t, stores.historyStore())
	assert.True(t, stores.cacheStore().IsEnabled())
	assert.True(t, stores.historyStore().IsEnabled())
}

// TestSharedStores_TwoEnginesShareOneLiveHandle is the regression for #2038: a
// second engine in one process must not lose its cache and history to the
// Bolt file lock.
//
//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestSharedStores_TwoEnginesShareOneLiveHandle(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()
	cmd := &cobra.Command{Use: "test"}
	cfg := config.New()

	stores := openSharedStores(ctx, cmd, cfg)
	t.Cleanup(func() { stores.Close(ctx) })
	first := newEngineWithStores(ctx, nil, cfg, stores)
	second := newEngineWithStores(ctx, nil, cfg, stores)

	require.NotNil(t, first)
	require.NotNil(t, second)
	require.NotNil(t, stores.cacheStore())
	require.NotNil(t, stores.historyStore())

	require.NoError(t, stores.cacheStore().Set(cache.BucketProjected+"/k", json.RawMessage(`"v"`)))
	entry, err := stores.cacheStore().Get(cache.BucketProjected + "/k")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.JSONEq(t, `"v"`, string(entry.Data))
}

// TestPerEngineOpen_SecondEngineLosesStores documents the failure the shared
// stores fix: two per-engine opens in one process leave the second without a
// cache and history.
//
//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestPerEngineOpen_SecondEngineLosesStores(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()
	cmd := &cobra.Command{Use: "test"}
	cfg := config.New()

	_, firstHistory, firstCache, firstCleanup := newEngineWithCacheAndHistory(ctx, cmd, nil, nil, cfg)
	t.Cleanup(firstCleanup)
	_, secondHistory, secondCache, secondCleanup := newEngineWithCacheAndHistory(ctx, cmd, nil, nil, cfg)
	t.Cleanup(secondCleanup)

	require.NotNil(t, firstCache)
	require.NotNil(t, firstHistory)
	assert.Nil(t, secondCache, "second per-engine open times out on the cache lock")
	assert.Nil(t, secondHistory, "second per-engine open times out on the history lock")
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestSharedStores_CloseIsIdempotentAndReleasesLocks(t *testing.T) {
	cacheDir, historyDir := storeEnv(t)
	ctx := context.Background()

	stores := openSharedStores(ctx, &cobra.Command{Use: "test"}, config.New())
	stores.Close(ctx)
	stores.Close(ctx)

	reopenedCache, err := cache.NewBoltStore(ctx, cacheDir, true, 300, 0)
	require.NoError(t, err)
	require.NoError(t, reopenedCache.Close())
	reopenedHistory, err := history.NewBoltStore(ctx, historyDir, true, 90)
	require.NoError(t, err)
	require.NoError(t, reopenedHistory.Close())
}

// Not parallel: Uses t.Setenv.
func TestOpenSharedStores_Disabled(t *testing.T) {
	storeEnv(t)
	t.Setenv(config.CacheEnvTTLSeconds, "0")
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()

	stores := openSharedStores(ctx, &cobra.Command{Use: "test"}, config.New())
	t.Cleanup(func() { stores.Close(ctx) })

	assert.Nil(t, stores.cacheStore())
	assert.Nil(t, stores.historyStore())
	assert.NotNil(t, newEngineWithStores(ctx, nil, config.New(), stores))
}

func TestSharedStores_NilReceiver(t *testing.T) {
	t.Parallel()
	var stores *sharedStores

	assert.Nil(t, stores.cacheStore())
	assert.Nil(t, stores.historyStore())
	assert.NotPanics(t, func() { stores.Close(context.Background()) })
	assert.NotNil(t, newEngineWithStores(context.Background(), nil, config.New(), nil))
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestSharedStores_ConcurrentUse(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()
	stores := openSharedStores(ctx, &cobra.Command{Use: "test"}, config.New())
	t.Cleanup(func() { stores.Close(ctx) })
	require.NotNil(t, stores.cacheStore())
	require.NotNil(t, stores.historyStore())

	var wg sync.WaitGroup
	for stack := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := string(rune('a' + stack))
			assert.NoError(t, stores.cacheStore().Set(cache.BucketProjected+"/"+key, json.RawMessage(`1`)))
			assert.NoError(t, stores.historyStore().Upsert(key, history.ResourceHistoryEntry{
				URN: "urn:" + key, CloudID: "id-" + key, Source: history.SourceStateSnapshot,
				FirstSeen: 1, LastSeen: 2,
			}))
		}()
	}
	wg.Wait()

	for stack := range 4 {
		key := string(rune('a' + stack))
		entry, err := stores.cacheStore().Get(cache.BucketProjected + "/" + key)
		require.NoError(t, err)
		assert.NotNil(t, entry)
	}
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestOverviewPipeline_OpenHistoryUsesInjectedStores(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()
	cfg := config.New()
	stores := openSharedStores(ctx, &cobra.Command{Use: "test"}, cfg)
	t.Cleanup(func() { stores.Close(ctx) })
	require.NotNil(t, stores.historyStore())

	pipe := newOverviewPipeline(overviewPipelineConfig{stores: stores})
	first, firstCleanup := pipe.openHistory(ctx, cfg)
	second, secondCleanup := pipe.openHistory(ctx, cfg)
	firstCleanup()
	secondCleanup()

	assert.Same(t, stores.historyStore(), first)
	assert.Same(t, stores.historyStore(), second)
	assert.NoError(t, first.Upsert("stack", history.ResourceHistoryEntry{
		URN: "urn:a", CloudID: "id-a", Source: history.SourceStateSnapshot, FirstSeen: 1, LastSeen: 2,
	}), "pipeline cleanup must not close the shared handle")
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestOverviewPipeline_OpenHistoryWithDisabledStoresDoesNotReopen(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()
	cfg := config.New()

	pipe := newOverviewPipeline(overviewPipelineConfig{stores: &sharedStores{}})
	store, cleanup := pipe.openHistory(ctx, cfg)
	cleanup()

	assert.Nil(t, store)
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestOverviewPipeline_OpenHistoryWithoutStoresOpensPerCall(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()

	pipe := newOverviewPipeline(overviewPipelineConfig{})
	store, cleanup := pipe.openHistory(ctx, config.New())
	require.NotNil(t, store)
	cleanup()
	again, againCleanup := pipe.openHistory(ctx, config.New())
	t.Cleanup(againCleanup)

	assert.NotNil(t, again, "a per-call open must release its lock on cleanup")
}

//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestWebData_NewEngineUsesInjectedStoresAndLeavesThemOpen(t *testing.T) {
	storeEnv(t)
	ctx := context.Background()
	cmd := &cobra.Command{Use: "test"}
	cfg := config.New()
	stores := openSharedStores(ctx, cmd, cfg)
	t.Cleanup(func() { stores.Close(ctx) })
	require.NotNil(t, stores.cacheStore())
	require.NotNil(t, stores.historyStore())

	newStack := func() *webData {
		return newWebData(cmd, overviewParams{cfg: cfg}, engine.DateRange{}, nil).withStores(stores)
	}
	first, second := newStack(), newStack()
	_, firstCleanup := first.newEngine(ctx, cmd, nil)
	_, secondCleanup := second.newEngine(ctx, cmd, nil)

	assert.Same(t, stores.cacheStore(), first.cache)
	assert.Same(t, stores.cacheStore(), second.cache)
	assert.Same(t, stores.historyStore(), first.history)
	assert.Same(t, stores.historyStore(), second.history)

	firstCleanup()

	require.NoError(t, stores.cacheStore().Set(cache.BucketProjected+"/after", json.RawMessage(`1`)),
		"one stack's cleanup must not close the stores another stack uses")
	assert.NoError(t, second.history.Upsert("stack", history.ResourceHistoryEntry{
		URN: "urn:b", CloudID: "id-b", Source: history.SourceStateSnapshot, FirstSeen: 1, LastSeen: 2,
	}))
	secondCleanup()
}

// A store that cannot open (here, held by another handle) leaves its field nil
// and does not stop the other store or the engines from being built.
//
//nolint:paralleltest // Uses t.Setenv through storeEnv.
func TestOpenSharedStores_FailedOpenLeavesNilAndOtherStoreLive(t *testing.T) {
	_, historyDir := storeEnv(t)
	ctx := context.Background()
	held, err := history.NewBoltStore(ctx, historyDir, true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })

	stores := openSharedStores(ctx, &cobra.Command{Use: "test"}, config.New())
	t.Cleanup(func() { stores.Close(ctx) })

	assert.Nil(t, stores.historyStore(), "a locked history file leaves the history store nil")
	assert.NotNil(t, stores.cacheStore(), "the cache is independent of the history failure")
	assert.NotNil(t, newEngineWithStores(ctx, nil, config.New(), stores))
}
