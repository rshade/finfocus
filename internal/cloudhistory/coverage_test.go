package cloudhistory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

type failureCloudCache struct {
	history.CloudCache

	listReadErr, listWriteErr, snapshotReadErr error
}

func (c failureCloudCache) GetUpdateList(hash string) (history.UpdateListRecord, bool, error) {
	if c.listReadErr != nil {
		return history.UpdateListRecord{}, false, c.listReadErr
	}
	return c.CloudCache.GetUpdateList(hash)
}
func (c failureCloudCache) PutUpdateList(hash string, rec history.UpdateListRecord) error {
	if c.listWriteErr != nil {
		return c.listWriteErr
	}
	return c.CloudCache.PutUpdateList(hash, rec)
}

func (c failureCloudCache) GetSnapshot(
	hash string,
	version int,
) (history.SnapshotRecord, bool, error) {
	if c.snapshotReadErr != nil {
		return history.SnapshotRecord{}, false, c.snapshotReadErr
	}
	return c.CloudCache.GetSnapshot(hash, version)
}

type failureListClient struct {
	pulumicloud.Client

	err          error
	afterList    func()
	cancelExport context.CancelFunc
}

func (c failureListClient) ListUpdates(
	ctx context.Context,
	stack pulumicloud.StackRef,
	etag string,
) (pulumicloud.UpdateList, error) {
	if c.err != nil {
		return pulumicloud.UpdateList{}, c.err
	}
	list, err := c.Client.ListUpdates(ctx, stack, etag)
	if c.afterList != nil {
		c.afterList()
	}
	return list, err
}

func (c failureListClient) ExportIdentities(
	ctx context.Context,
	stack pulumicloud.StackRef,
	v int,
) ([]pulumicloud.Identity, error) {
	if c.cancelExport != nil {
		c.cancelExport()
		return nil, ctx.Err()
	}
	return c.Client.ExportIdentities(ctx, stack, v)
}

func TestImportListFailuresExplainRequiredFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		reason string
	}{
		{"unauthorized", pulumicloud.ErrUnauthorized, "token_rejected"},
		{"forbidden", pulumicloud.ErrForbidden, "no_permission"},
		{"missing stack", pulumicloud.ErrNotFound, "unknown_stack"},
		{"transport", errors.New("connection failed"), "unreachable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps, req, client, store := importFixture(t)
			deps.NewClient = func(esc.Session) pulumicloud.Client {
				return failureListClient{
					Client: client,
					err:    fmt.Errorf("list updates: %w", tc.err),
				}
			}
			req.Mode = config.Required
			result, err := Import(context.Background(), deps, req)
			var unavailable *RequiredError
			require.ErrorAs(t, err, &unavailable)
			assert.Equal(t, tc.reason, unavailable.Reason)
			assert.Equal(t, tc.reason, result.Reason)
			assert.NotEmpty(t, unavailable.Fix)
			rows, readErr := store.GetAllForStack(
				req.StackContext.Hash(),
				req.From.Unix(),
				req.To.Unix(),
			)
			require.NoError(t, readErr)
			assert.Empty(t, rows)
		})
	}
}

func TestImportCacheReadAndListWriteFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"read list", "write list", "read snapshot"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			deps, req, client, store := importFixture(t)
			client.updates = []pulumicloud.UpdateRecord{
				{Version: 1, Kind: "update", End: req.To.Unix() - 1},
			}
			cache := failureCloudCache{CloudCache: store}
			failure := errors.New("storage unavailable")
			switch stage {
			case "read list":
				cache.listReadErr = failure
			case "write list":
				cache.listWriteErr = failure
			case "read snapshot":
				cache.snapshotReadErr = failure
			}
			deps.Cache = cache
			result, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Zero(t, result.Recorded)
			assert.Empty(t, client.fetched)
			if stage == "read snapshot" {
				assert.Equal(t, 1, result.Unreadable)
				assert.True(t, result.Incomplete())
			} else {
				assert.Equal(t, "unreachable", result.Reason)
			}
		})
	}
}

func TestImportPrerequisitesPreventNetwork(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"off", "missing cache", "unavailable store"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			deps, req, client, _ := importFixture(t)
			switch stage {
			case "off":
				req.Mode = config.Off
				deps.Store = nil
			case "missing cache":
				deps.Cache = nil
			case "unavailable store":
				deps.Store = nil
				deps.StoreError = errors.New("cannot open history.db")
			}
			result, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			assert.Zero(t, client.lists)
			if stage != "off" {
				assert.Equal(t, "history_unavailable", result.Reason)
			}
		})
	}
}

func TestImportRejects304WithoutCachedList(t *testing.T) {
	t.Parallel()
	deps, req, client, _ := importFixture(t)
	client.notModified = true
	req.Mode = config.Required
	result, err := Import(context.Background(), deps, req)
	require.Error(t, err)
	assert.Equal(t, "unreachable", result.Reason)
	assert.Zero(t, result.Recorded)
}

func TestImportCancellationDuringExportAndEmptyList(t *testing.T) {
	t.Parallel()
	for _, hasUpdate := range []bool{true, false} {
		t.Run(strconv.FormatBool(hasUpdate), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deps, req, client, _ := importFixture(t)
			wrapped := failureListClient{Client: client}
			if hasUpdate {
				client.updates = []pulumicloud.UpdateRecord{
					{Version: 1, Kind: "update", End: req.To.Unix() - 1},
				}
				wrapped.cancelExport = cancel
			} else {
				wrapped.afterList = cancel
			}
			deps.NewClient = func(esc.Session) pulumicloud.Client { return wrapped }
			result, err := Import(ctx, deps, req)
			require.ErrorIs(t, err, context.Canceled)
			assert.Zero(t, result.Recorded)
		})
	}
}

func TestImportDefaultNetworkDependencies(t *testing.T) {
	t.Parallel()
	deps, req, _, store := importFixture(t)
	at := time.Now().Add(-time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token default-dependencies", r.Header.Get("Authorization"))
		switch {
		case r.URL.Path == "/api/user":
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Query().Get("page") == "1":
			_, _ = fmt.Fprintf(w, `{"updates":[{"version":1,"kind":"update","startTime":%d}]}`, at)
		case r.URL.Query().Get("page") != "":
			_, _ = w.Write([]byte(`{"updates":[]}`))
		default:
			_, _ = w.Write(
				[]byte(
					`{"deployment":{"resources":[{"urn":"urn:pulumi:stack::project::aws:s3/bucket:Bucket::test","id":"bucket-id","type":"aws:s3/bucket:Bucket","custom":true}]}}`,
				),
			)
		}
	}))
	t.Cleanup(server.Close)
	deps.Detect = nil
	deps.NewClient = nil
	deps.Now = nil
	req.Session = esc.Session{Backend: server.URL, Token: "default-dependencies"}
	result, err := Import(context.Background(), deps, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Recorded)
	cached, found, err := store.GetSnapshot(cloudCacheNamespace(req), 1)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, at, cached.ObservedAt)
	assert.Len(t, cached.Resources, 1)
}
