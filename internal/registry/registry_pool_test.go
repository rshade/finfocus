package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestRegistry_Open_SharesProcessesThroughContextPool(t *testing.T) {
	t.Parallel()

	names := pluginNames(3)
	launcher := newFakeLauncher(t, names, nil)
	reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}
	pool := pluginhost.NewPool(context.Background())
	t.Cleanup(pool.Close)
	ctx := pluginhost.WithPool(context.Background(), pool)

	first, releaseFirst, err := reg.Open(ctx, "")
	require.NoError(t, err)
	second, releaseSecond, err := reg.Open(ctx, "")
	require.NoError(t, err)

	assert.Equal(t, names, clientNames(first))
	assert.Equal(t, names, clientNames(second))
	assert.ElementsMatch(t, names, launcher.startedNames(), "each plugin launches once for both opens")

	releaseFirst()
	assert.Zero(t, launcher.closed.Load(), "the second open still holds every process")
	releaseSecond()
	assert.Equal(t, int32(len(names)), launcher.closed.Load())
}

func TestRegistry_Open_WithoutPoolLaunchesPerOpen(t *testing.T) {
	t.Parallel()

	names := pluginNames(2)
	launcher := newFakeLauncher(t, names, nil)
	reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}

	_, releaseFirst, err := reg.Open(context.Background(), "")
	require.NoError(t, err)
	t.Cleanup(releaseFirst)
	_, releaseSecond, err := reg.Open(context.Background(), "")
	require.NoError(t, err)
	t.Cleanup(releaseSecond)

	assert.Len(t, launcher.startedNames(), 2*len(names))
}
