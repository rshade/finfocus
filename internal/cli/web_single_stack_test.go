package cli

import (
	"bufio"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/history"
)

// A single-stack finfocus --web run holds one cache and one history handle for
// its whole life and releases both when it exits, as before the stores were
// shared across stacks.
//
// Not parallel: Root builders, cwd, env, and the web pipeline hook are process-wide.
func TestRunWebSession_SingleStackHoldsOneHandlePerStoreAndReleasesThem(t *testing.T) {
	unsetPassphraseEnv(t)
	cacheDir, historyDir := storeEnv(t)
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	t.Chdir(t.TempDir())
	webPipelineHook = func(p *OverviewPipeline) {
		p.cfg.openPlugins = fakeOpenPlugins
		p.cfg.newEngine = fakeNewEngine
		p.cfg.enrichRows = passThroughEnrichment
	}
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })
	out, errOut, cancel, wait := runWebCommandCapturingStderr(
		t,
		[]string{"--web", "--no-browser", "--pulumi-json", overviewFixture(t, "plan-mixed-changes.json")},
	)
	defer cancel()
	origin, cookie := webHTTP(t, awaitURL(t, out))
	reader := bufio.NewReader(webAPI(t, origin, cookie, "/api/overview/stream", "").Body)
	name, snapshot := webEvent(t, reader)
	require.Equal(t, "snapshot", name)
	for ready := snapshot["ready"] == true; !ready; {
		event, _ := webEvent(t, reader)
		ready = event == "ready"
	}

	_, cacheErr := cache.NewBoltStore(context.Background(), cacheDir, true, 300, 0)
	_, historyErr := history.NewBoltStore(context.Background(), historyDir, true, 90)
	require.ErrorIs(t, cacheErr, cache.ErrCacheLocked, "the run must hold the cache while serving")
	require.ErrorIs(t, historyErr, history.ErrHistoryLocked, "the run must hold the history while serving")

	cancel()
	require.NoError(t, wait())

	reopenedCache, err := cache.NewBoltStore(context.Background(), cacheDir, true, 300, 0)
	require.NoError(t, err, "exit must release the cache")
	require.NoError(t, reopenedCache.Close())
	reopenedHistory, err := history.NewBoltStore(context.Background(), historyDir, true, 90)
	require.NoError(t, err, "exit must release the history")
	require.NoError(t, reopenedHistory.Close())
	assert.NotContains(t, errOut.String(), "proceeding without")
}
