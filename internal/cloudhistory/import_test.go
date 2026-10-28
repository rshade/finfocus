package cloudhistory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

type fakeClient struct {
	mu          sync.Mutex
	updates     []pulumicloud.UpdateRecord
	ids         map[int][]pulumicloud.Identity
	failures    map[int]error
	fetched     []int
	lists       int
	etag        string
	notModified bool
}

func (c *fakeClient) ListUpdates(
	_ context.Context,
	_ pulumicloud.StackRef,
	etag string,
) (pulumicloud.UpdateList, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lists++
	c.etag = etag
	return pulumicloud.UpdateList{Updates: c.updates, ETag: "tag", NotModified: c.notModified}, nil
}

func (c *fakeClient) ExportIdentities(
	_ context.Context,
	_ pulumicloud.StackRef,
	v int,
) ([]pulumicloud.Identity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetched = append(c.fetched, v)
	return c.ids[v], c.failures[v]
}

func importFixture(t *testing.T) (Deps, Request, *fakeClient, *history.BoltStore) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	store, err := history.NewBoltStore(context.Background(), t.TempDir(), true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	client := &fakeClient{ids: map[int][]pulumicloud.Identity{}, failures: map[int]error{}}
	deps := Deps{
		Store:     store,
		Cache:     store,
		NewClient: func(esc.Session) pulumicloud.Client { return client },
		Now:       func() time.Time { return now },
		Logger:    zerolog.Nop(),
		Detect: func(context.Context, esc.Session, pulumicloud.StackRef) pulumicloud.Availability {
			return pulumicloud.Availability{Available: true}
		},
	}
	req := Request{
		Mode:          config.Auto,
		Session:       esc.Session{Backend: "https://test.example", Token: "t", Org: "org"},
		Stack:         pulumicloud.StackRef{Org: "org", Project: "project", Stack: "stack"},
		StackContext:  history.StackContext{Organization: "org", Project: "project", Stack: "stack"},
		From:          now.Add(-24 * time.Hour),
		To:            now,
		RetentionDays: 90,
	}
	return deps, req, client, store
}

func TestImportObservations(t *testing.T) {
	t.Parallel()
	deps, req, client, store := importFixture(t)
	identity := pulumicloud.Identity{
		URN:      "urn",
		CloudID:  "destroyed-id",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
	}
	kinds := []string{"update", "refresh", "destroy", "import", "preview", "rename"}
	for i, kind := range kinds {
		v := i + 1
		client.updates = append(
			client.updates,
			pulumicloud.UpdateRecord{Version: v, Kind: kind, Result: "succeeded", End: req.From.Unix() + int64(v*60)},
		)
		client.ids[v] = []pulumicloud.Identity{identity}
	}
	client.updates[0].Result = "failed"
	client.updates[1].Result = "canceled"
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 6, result.Listed)
	assert.Equal(t, 2, result.Skipped)
	assert.Equal(t, 4, result.Fetched)
	assert.Equal(t, 4, result.Recorded)
	got, err := store.GetAllForStack(req.StackContext.Hash(), req.From.Unix(), req.To.Unix())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, req.From.Unix()+60, got[0].FirstSeen)
	assert.Equal(t, req.From.Unix()+240, got[0].LastSeen)
	assert.Equal(t, history.SourcePulumiCloud, got[0].Source)
	result, err = Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Zero(t, result.Fetched)
	assert.Equal(t, 4, result.FromCache)
	assert.Equal(t, 4, result.Recorded)
}

func TestImportRanges(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		from, to time.Duration
		days     int
		clamped  bool
		fetched  int
	}{
		{"clamped", -30 * 24 * time.Hour, 0, 10, true, 1}, {"future", time.Hour, 2 * time.Hour, 90, false, 0}, {"before first update", -5 * time.Hour, -4 * time.Hour, 90, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps, req, client, _ := importFixture(t)
			now := deps.Now()
			req.From = now.Add(tc.from)
			req.To = now.Add(tc.to)
			req.RetentionDays = tc.days
			client.updates = []pulumicloud.UpdateRecord{{Version: 1, Kind: "update", End: now.Add(-time.Hour).Unix()}}
			result, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Equal(t, tc.clamped, result.Clamped)
			assert.Equal(t, tc.fetched, result.Fetched)
		})
	}
}

func TestImportSingleObservationAndStateMerge(t *testing.T) {
	t.Parallel()
	deps, req, client, store := importFixture(t)
	at := req.From.Unix() + 100
	id := pulumicloud.Identity{URN: "urn", CloudID: "id", Type: "aws:ec2/instance:Instance", Provider: "aws"}
	client.updates = []pulumicloud.UpdateRecord{{Version: 3, Kind: "update", End: at}}
	client.ids[3] = []pulumicloud.Identity{id}
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Recorded)
	got, err := store.GetAllForStack(req.StackContext.Hash(), 0, req.To.Unix())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, at, got[0].FirstSeen)
	assert.Equal(t, at, got[0].LastSeen)
	entry := got[0]
	entry.FirstSeen = req.To.Unix()
	entry.LastSeen = req.To.Unix()
	entry.Source = history.SourceStateSnapshot
	require.NoError(t, store.Upsert(req.StackContext.Hash(), entry))
	_, err = Import(context.Background(), deps, req)
	require.NoError(t, err)
	got, err = store.GetAllForStack(req.StackContext.Hash(), 0, req.To.Unix())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, at, got[0].FirstSeen)
	assert.Equal(t, req.To.Unix(), got[0].LastSeen)
}

func TestImportEmptyList(t *testing.T) {
	t.Parallel()
	deps, req, _, _ := importFixture(t)
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Zero(t, result.Listed)
	assert.Zero(t, result.Recorded)
}

func TestImportAutoUnavailable(t *testing.T) {
	t.Parallel()
	for _, reason := range []pulumicloud.Reason{pulumicloud.NotLoggedIn, pulumicloud.NonCloudBackend, pulumicloud.InsecureBackend, pulumicloud.UnknownStack, pulumicloud.TokenRejected, pulumicloud.NoPermission, pulumicloud.Unreachable} {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			deps, req, client, _ := importFixture(t)
			deps.Detect = func(context.Context, esc.Session, pulumicloud.StackRef) pulumicloud.Availability {
				return pulumicloud.Availability{Reason: reason}
			}
			result, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Equal(t, string(reason), result.Reason)
			assert.Zero(t, result.Recorded)
			assert.Zero(t, client.lists)
		})
	}
	for _, tc := range []struct {
		name     string
		storeErr error
		want     string
	}{{"disabled", nil, "history_disabled"}, {"locked", history.ErrHistoryLocked, "history_locked"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps, req, client, _ := importFixture(t)
			deps.Store = nil
			deps.StoreError = tc.storeErr
			result, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.Reason)
			assert.Zero(t, client.lists)
		})
	}
}

func TestImportPartialAndResume(t *testing.T) {
	t.Parallel()
	deps, req, client, store := importFixture(t)
	for v := 1; v <= 2; v++ {
		client.updates = append(
			client.updates,
			pulumicloud.UpdateRecord{Version: v, Kind: "update", End: req.From.Unix() + int64(v)},
		)
		client.ids[v] = []pulumicloud.Identity{
			{URN: fmt.Sprint("urn", v), CloudID: fmt.Sprint("id", v), Type: "aws:s3/bucket:Bucket", Provider: "aws"},
		}
	}
	client.failures[2] = errors.New("unreadable snapshot")
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Unreadable)
	assert.True(t, result.Incomplete())
	assert.Equal(t, 1, result.Recorded)
	got, err := store.GetAllForStack(req.StackContext.Hash(), 0, req.To.Unix())
	require.NoError(t, err)
	require.Len(t, got, 1)
	delete(client.failures, 2)
	client.fetched = nil
	result, err = Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.False(t, result.Incomplete())
	assert.Equal(t, 1, result.Fetched)
	assert.Equal(t, 1, result.FromCache)
	assert.Equal(t, []int{2}, client.fetched)
}

type failingCache struct {
	history.CloudCache

	fail bool
}

func (c *failingCache) PutSnapshot(hash string, rec history.SnapshotRecord) error {
	if c.fail {
		return errors.New("cache write failed")
	}
	return c.CloudCache.PutSnapshot(hash, rec)
}

type failingStore struct {
	history.Store

	fail bool
}

func (s *failingStore) UpsertBatch(hash string, entries []history.ResourceHistoryEntry) error {
	if s.fail {
		return errors.New("history write failed")
	}
	return s.Store.UpsertBatch(hash, entries)
}

func TestImportPersistenceFailureReplay(t *testing.T) {
	t.Parallel()
	for _, sink := range []string{"cache", "history"} {
		t.Run(sink, func(t *testing.T) {
			t.Parallel()
			deps, req, client, store := importFixture(t)
			client.updates = []pulumicloud.UpdateRecord{{Version: 1, Kind: "update", End: req.From.Unix() + 1}}
			client.ids[1] = []pulumicloud.Identity{
				{URN: "urn", CloudID: "id", Type: "aws:s3/bucket:Bucket", Provider: "aws"},
			}
			cache := &failingCache{CloudCache: store, fail: sink == "cache"}
			writer := &failingStore{Store: store, fail: sink == "history"}
			deps.Cache = cache
			deps.Store = writer
			result, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Unreadable)
			assert.Zero(t, result.Recorded)
			cache.fail = false
			writer.fail = false
			client.fetched = nil
			result, err = Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Recorded)
			assert.Zero(t, result.Unreadable)
			if sink == "history" {
				assert.Empty(t, client.fetched)
				assert.Equal(t, 1, result.FromCache)
			}
		})
	}
}

func TestImportRequiredReasons(t *testing.T) {
	t.Parallel()
	for _, reason := range []pulumicloud.Reason{pulumicloud.NotLoggedIn, pulumicloud.NonCloudBackend, pulumicloud.InsecureBackend, pulumicloud.UnknownStack, pulumicloud.TokenRejected, pulumicloud.NoPermission, pulumicloud.Unreachable} {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			deps, req, _, _ := importFixture(t)
			req.Mode = config.Required
			deps.Detect = func(context.Context, esc.Session, pulumicloud.StackRef) pulumicloud.Availability {
				return pulumicloud.Availability{Reason: reason}
			}
			_, err := Import(context.Background(), deps, req)
			var required *RequiredError
			require.ErrorAs(t, err, &required)
			assert.Equal(t, string(reason), required.Reason)
			assert.NotEmpty(t, required.Fix)
			assert.Equal(t, reason == pulumicloud.NonCloudBackend, required.Validation)
		})
	}
	for _, storeErr := range []error{nil, history.ErrHistoryLocked} {
		deps, req, client, _ := importFixture(t)
		deps.Store = nil
		deps.StoreError = storeErr
		req.Mode = config.Required
		_, err := Import(context.Background(), deps, req)
		var required *RequiredError
		require.ErrorAs(t, err, &required)
		assert.Equal(t, storeErr == nil, required.Validation)
		assert.Zero(t, client.lists)
	}
}

func TestImportRequiredPartial(t *testing.T) {
	t.Parallel()
	for _, sink := range []string{"fetch", "cache", "history"} {
		t.Run(sink, func(t *testing.T) {
			t.Parallel()
			deps, req, client, store := importFixture(t)
			req.Mode = config.Required
			client.updates = []pulumicloud.UpdateRecord{{Version: 1, Kind: "update", End: req.From.Unix() + 1}}
			client.ids[1] = []pulumicloud.Identity{
				{URN: "urn", CloudID: "id", Type: "aws:s3/bucket:Bucket", Provider: "aws"},
			}
			if sink == "fetch" {
				client.failures[1] = errors.New("fetch failed")
			}
			if sink == "cache" {
				deps.Cache = &failingCache{CloudCache: store, fail: true}
			}
			if sink == "history" {
				deps.Store = &failingStore{Store: store, fail: true}
			}
			result, err := Import(context.Background(), deps, req)
			require.Equal(t, 1, result.Unreadable)
			var required *RequiredError
			require.ErrorAs(t, err, &required)
			assert.Equal(t, "unreadable_updates", required.Reason)
		})
	}
}
