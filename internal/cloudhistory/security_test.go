package cloudhistory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

type securityLogBuffer struct {
	bytes.Buffer

	mu sync.Mutex
}

func (b *securityLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func TestSelfHostedImportProtectsEverySink(t *testing.T) {
	t.Parallel()
	const token = "SECURITY-SENTINEL-TOKEN"
	now := time.Now().Truncate(time.Second)
	export, err := os.ReadFile("../pulumicloud/testdata/export.json")
	require.NoError(t, err)
	var forbidden atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token "+token, r.Header.Get("Authorization"))
		switch {
		case r.URL.Path == "/api/user":
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/updates"):
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprintf(
					w,
					`{"updates":[{"version":1,"kind":"update","endTime":%d},{"version":2,"kind":"update","endTime":%d}]}`,
					now.Unix()-2,
					now.Unix()-1,
				)
			} else {
				_, _ = w.Write([]byte(`{"updates":[]}`))
			}
		case strings.HasSuffix(r.URL.Path, "/export/1"):
			_, _ = w.Write(export)
		default:
			forbidden.Store(true)
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintf(w, "backend echoed %s SENTINEL-CIPHERTEXT", token)
		}
	}))
	t.Cleanup(server.Close)
	home := t.TempDir()
	credentials, err := json.Marshal(
		map[string]any{
			"current":  server.URL,
			"accounts": map[string]any{server.URL: map[string]string{"accessToken": "saved-token"}},
		},
	)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, "credentials.json"), credentials, 0o600))
	session, failure := esc.DiscoverSession(func(key string) string {
		return map[string]string{"PULUMI_HOME": home, "PULUMI_BACKEND_URL": server.URL, "PULUMI_ACCESS_TOKEN": token}[key]
	}, "", "org/stack")
	require.Nil(t, failure)
	assert.Equal(t, token, session.Token)
	currentSession, currentFailure := esc.DiscoverSession(func(key string) string {
		return map[string]string{"PULUMI_HOME": home, "PULUMI_ACCESS_TOKEN": token}[key]
	}, "", "org/stack")
	require.Nil(t, currentFailure)
	assert.Equal(t, token, currentSession.Token)
	projectSession, projectFailure := esc.DiscoverSession(func(key string) string {
		return map[string]string{"PULUMI_HOME": home, "PULUMI_ACCESS_TOKEN": token}[key]
	}, server.URL, "org/stack")
	require.Nil(t, projectFailure)
	assert.Equal(t, "saved-token", projectSession.Token)

	directory := t.TempDir()
	store, err := history.NewBoltStore(context.Background(), directory, true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	var logs securityLogBuffer
	deps := Deps{
		Store:     store,
		Cache:     store,
		Logger:    zerolog.New(&logs),
		Now:       func() time.Time { return now },
		NewClient: func(s esc.Session) pulumicloud.Client { return pulumicloud.NewClient(s, server.Client()) },
		Detect: func(ctx context.Context, s esc.Session, stack pulumicloud.StackRef) pulumicloud.Availability {
			return pulumicloud.DetectWithClient(ctx, s, stack, server.Client())
		},
	}
	req := Request{
		Mode:    config.Required,
		Session: session,
		Stack:   pulumicloud.StackRef{Org: "org", Project: "project", Stack: "stack"},
		StackContext: history.StackContext{
			Organization: "org",
			Project:      "project",
			Stack:        "stack",
		},
		From:          now.Add(-time.Hour),
		To:            now,
		RetentionDays: 90,
	}
	result, importErr := Import(context.Background(), deps, req)
	require.Error(t, importErr)
	assert.True(t, forbidden.Load())
	assert.Equal(t, 1, result.Recorded)
	assert.Equal(t, 1, result.Unreadable)
	rows, err := store.GetAllForStack(req.StackContext.Hash(), req.From.Unix(), req.To.Unix())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "i-test", rows[0].CloudID)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	dbBytes, err := os.ReadFile(filepath.Join(directory, "history.db"))
	require.NoError(t, err)
	for _, sink := range []string{logs.String(), fmt.Sprint(importErr), string(encoded), string(dbBytes)} {
		assert.NotContains(t, sink, token)
		assert.NotContains(t, sink, "SENTINEL-CIPHERTEXT")
	}
	db, err := bolt.Open(
		filepath.Join(directory, "history.db"),
		0o600,
		&bolt.Options{ReadOnly: true},
	)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.View(func(tx *bolt.Tx) error {
		for _, name := range []string{history.BucketPulumiCloudUpdates, history.BucketPulumiCloudSnapshots} {
			bucket := tx.Bucket([]byte(name))
			require.NotNil(t, bucket)
			require.NoError(t, bucket.ForEach(func(k, v []byte) error {
				assert.NotContains(t, string(k)+string(v), token)
				assert.NotContains(t, string(k)+string(v), "SENTINEL-CIPHERTEXT")
				return nil
			}))
		}
		return nil
	}))
}

func TestSelfHostedRedirectNeverReachesOtherServer(t *testing.T) {
	t.Parallel()
	var reached atomic.Int64
	target := httptest.NewTLSServer(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { reached.Add(1) }),
	)
	t.Cleanup(target.Close)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/?secret=sentinel", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	session := esc.Session{Backend: server.URL, Token: "SECRET-TOKEN"}
	stack := pulumicloud.StackRef{Org: "o", Project: "p", Stack: "s"}
	available := pulumicloud.DetectWithClient(context.Background(), session, stack, server.Client())
	assert.False(t, available.Available)
	_, err := pulumicloud.NewClient(session, server.Client()).
		ExportIdentities(context.Background(), stack, 1)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "SECRET-TOKEN")
	assert.NotContains(t, err.Error(), "sentinel")
	assert.Zero(t, reached.Load())
}

func TestProjectBackendCannotReceiveUnrelatedCredentials(t *testing.T) {
	t.Parallel()
	var reached atomic.Int64
	project := httptest.NewTLSServer(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { reached.Add(1) }),
	)
	t.Cleanup(project.Close)
	home := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(home, "credentials.json"),
			[]byte(
				`{"current":"https://saved.example","accounts":{"https://saved.example":{"accessToken":"SAVED-TOKEN"}}}`,
			),
			0o600,
		),
	)
	session, failure := esc.DiscoverSession(func(key string) string {
		return map[string]string{"PULUMI_HOME": home, "PULUMI_ACCESS_TOKEN": "ENV-TOKEN"}[key]
	}, project.URL, "org/stack")
	require.NotNil(t, failure)
	assert.Empty(t, session.Token)
	availability := pulumicloud.DetectWithClient(
		context.Background(),
		session,
		pulumicloud.StackRef{Org: "org", Project: "p", Stack: "stack"},
		project.Client(),
	)
	assert.Equal(t, pulumicloud.NotLoggedIn, availability.Reason)
	assert.Zero(t, reached.Load())
}
