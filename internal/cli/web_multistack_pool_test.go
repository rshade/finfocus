package cli

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// Not parallel: sets process-wide config and the web pipeline hook.
func TestNewWebStacks_ShareOnePluginPool(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	var mu sync.Mutex
	var openPools []*pluginhost.Pool
	webPipelineHook = func(p *OverviewPipeline) {
		p.cfg.newEngine = fakeNewEngine
		p.cfg.openPlugins = func(
			ctx context.Context, adapter string, audit *auditContext,
		) ([]*pluginhost.Client, func(), error) {
			pool, _ := pluginhost.PoolFromContext(ctx)
			mu.Lock()
			openPools = append(openPools, pool)
			mu.Unlock()
			return fakeOpenPlugins(ctx, adapter, audit)
		}
		p.cfg.loader = func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
			return nil, errors.New("load failed")
		}
	}
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })

	stacks, stop := newWebStacks(context.Background(), cmd, []string{"dev", "prod"},
		overviewParams{cfg: config.New()}, engine.DateRange{}, nil, nil)
	t.Cleanup(stop)
	require.Len(t, stacks, 2)

	pool, ok := pluginhost.PoolFromContext(stacks[0].session.Context())
	require.True(t, ok, "reloads run under the session context and must reach the pool")
	other, ok := pluginhost.PoolFromContext(stacks[1].session.Context())
	require.True(t, ok)
	assert.Same(t, pool, other, "every stack shares one pool")

	for _, stack := range stacks {
		stack.start()
		select {
		case <-stack.pipeDone:
		case <-time.After(10 * time.Second):
			require.Fail(t, "pipeline never finished")
		}
	}
	mu.Lock()
	require.Len(t, openPools, 2)
	assert.Same(t, pool, openPools[0], "the pipeline's plugin open must reach the pool")
	assert.Same(t, pool, openPools[1])
	mu.Unlock()

	stop()
	_, err := pool.Acquire(context.Background(), nil, "/plugins/any")
	require.ErrorIs(t, err, pluginhost.ErrPoolClosed, "stopping the stacks closes the pool")
}
