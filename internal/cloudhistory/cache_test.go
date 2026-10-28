package cloudhistory

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

func TestImportListFreshnessAnd304(t *testing.T) {
	t.Parallel()
	deps, req, client, store := importFixture(t)
	now := deps.Now()
	deps.Now = func() time.Time { return now }
	client.updates = []pulumicloud.UpdateRecord{{Version: 1, Kind: "update", End: req.From.Unix() + 1}}
	_, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	_, err = Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 1, client.lists)
	now = now.Add(time.Hour)
	client.notModified = true
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 2, client.lists)
	assert.Equal(t, "tag", client.etag)
	assert.Equal(t, 1, result.FromCache)
	rec, found, err := store.GetUpdateList(cloudCacheNamespace(req))
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, now.Unix(), rec.FetchedAt)
	require.Len(t, rec.Updates, 1)
}

type concurrentClient struct {
	*fakeClient

	active, peak atomic.Int64
}

func (c *concurrentClient) ExportIdentities(
	ctx context.Context,
	stack pulumicloud.StackRef,
	v int,
) ([]pulumicloud.Identity, error) {
	active := c.active.Add(1)
	defer c.active.Add(-1)
	for {
		peak := c.peak.Load()
		if peak >= active || c.peak.CompareAndSwap(peak, active) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	return c.fakeClient.ExportIdentities(ctx, stack, v)
}

func TestImportBoundedConcurrency(t *testing.T) {
	t.Parallel()
	deps, req, base, _ := importFixture(t)
	client := &concurrentClient{fakeClient: base}
	deps.NewClient = func(esc.Session) pulumicloud.Client { return client }
	for v := 1; v <= 100; v++ {
		base.updates = append(
			base.updates,
			pulumicloud.UpdateRecord{Version: v, Kind: "update", End: req.From.Unix() + int64(v)},
		)
	}
	start := time.Now()
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 100, result.Fetched)
	assert.Greater(t, client.peak.Load(), int64(1))
	assert.LessOrEqual(t, client.peak.Load(), int64(5))
	assert.Less(t, time.Since(start), 60*time.Second)
	client.fetched = nil
	result, err = Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Zero(t, result.Fetched)
	assert.Equal(t, 100, result.FromCache)
	assert.Empty(t, client.fetched)
}
