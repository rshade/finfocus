package esc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc/esctest"
)

func TestClientPreservesLoopbackPort(t *testing.T) {
	t.Parallel()
	srv := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/prod": {Variables: map[string]esctest.Variable{"TOKEN": {Value: "v"}}, ETag: "etag-1"},
	})
	t.Cleanup(srv.Close)
	client, err := newSDKClient(srv.URL, "tok", nil)
	require.NoError(t, err)
	vars, key, err := client.open(context.Background(), Ref{Org: "acme", Project: "billing", Name: "prod"})
	require.NoError(t, err)
	assert.Equal(t, "v", vars["TOKEN"].value)
	assert.Equal(t, "etag-1", key)
	assert.Equal(t, 2, srv.Heads("acme", "billing", "prod"), "latest checks its ETag before and after the open")
}

func TestClientRevisionKeys(t *testing.T) {
	t.Parallel()
	env := &esctest.Environment{
		Variables: map[string]esctest.Variable{"TOKEN": {Value: "v"}},
		ETag:      "etag-stable",
		Tags:      map[string]int{"stable": 4},
	}
	srv := esctest.New("tok", map[string]*esctest.Environment{"acme/billing/prod": env})
	t.Cleanup(srv.Close)
	client, err := newSDKClient(srv.URL, "tok", nil)
	require.NoError(t, err)
	ref := Ref{Org: "acme", Project: "billing", Name: "prod"}

	pinned := ref
	pinned.Version = "7"
	_, key, err := client.open(context.Background(), pinned)
	require.NoError(t, err)
	assert.Equal(t, "7", key)
	assert.Equal(t, 0, srv.Heads("acme", "billing", "prod"))
	assert.Equal(t, 0, srv.TagReads("acme", "billing", "prod"))

	_, key, err = client.open(context.Background(), Ref{
		Org: "acme", Project: "billing", Name: "prod", Version: "stable",
	})
	require.NoError(t, err)
	assert.Equal(t, "4", key)
	assert.Equal(t, 1, srv.TagReads("acme", "billing", "prod"))

	env.Tags["stable"] = 9
	_, moved, err := client.open(context.Background(), Ref{
		Org: "acme", Project: "billing", Name: "prod", Version: "stable",
	})
	require.NoError(t, err)
	assert.Equal(t, "9", moved)
	_, latest, err := client.open(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, "etag-stable", latest)
}

func TestClientMalformedTraceAndDeadline(t *testing.T) {
	t.Parallel()
	t.Run("malformed trace", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/prod": {
				Variables:      map[string]esctest.Variable{"TOKEN": {Value: "v", Secret: true}},
				MalformedTrace: true,
			},
		})
		t.Cleanup(srv.Close)
		client, err := newSDKClient(srv.URL, "tok", nil)
		require.NoError(t, err)
		_, _, openErr := client.open(context.Background(), Ref{Org: "acme", Project: "billing", Name: "prod"})
		require.Error(t, openErr)
		assert.Equal(t, CategoryUnavailable, classify(context.Background(), openErr).Category)
	})

	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/prod": {Delay: 300 * time.Millisecond, ETag: "e"},
		})
		t.Cleanup(srv.Close)
		client, err := newSDKClient(srv.URL, "tok", nil)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer cancel()
		_, _, openErr := client.open(ctx, Ref{Org: "acme", Project: "billing", Name: "prod"})
		require.Error(t, openErr)
		assert.Equal(t, CategoryTimedOut, classify(ctx, openErr).Category)
	})

	t.Run("caller cancel", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/prod": {Delay: 2 * time.Second, ETag: "e"},
		})
		t.Cleanup(srv.Close)
		client, err := newSDKClient(srv.URL, "tok", nil)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		_, _, openErr := client.open(ctx, Ref{Org: "acme", Project: "billing", Name: "prod"})
		require.Error(t, openErr)
		assert.Less(t, time.Since(start), time.Second)
	})
}

func TestClientRevisionIsBoundToTheOpenedValues(t *testing.T) {
	t.Parallel()
	ref := Ref{Org: "acme", Project: "billing", Name: "prod"}
	newClient := func(t *testing.T, env *esctest.Environment) *sdkClient {
		t.Helper()
		srv := esctest.New("tok", map[string]*esctest.Environment{"acme/billing/prod": env})
		t.Cleanup(srv.Close)
		client, err := newSDKClient(srv.URL, "tok", nil)
		require.NoError(t, err)
		return client
	}

	t.Run("a tag that moves after lookup keeps the values of the revision it resolved to", func(t *testing.T) {
		t.Parallel()
		env := &esctest.Environment{
			Variables: map[string]esctest.Variable{"TOKEN": {Value: "rev-9"}},
			Versions: map[int]map[string]esctest.Variable{
				4: {"TOKEN": {Value: "rev-4"}},
				9: {"TOKEN": {Value: "rev-9"}},
			},
			Tags: map[string]int{"stable": 4},
		}
		srv := esctest.New("tok", map[string]*esctest.Environment{"acme/billing/prod": env})
		t.Cleanup(srv.Close)
		env.AfterTag = func() {
			srv.Update("acme", "billing", "prod", func(e *esctest.Environment) { e.Tags["stable"] = 9 })
		}
		client, err := newSDKClient(srv.URL, "tok", nil)
		require.NoError(t, err)
		tagged := ref
		tagged.Version = "stable"
		vars, revision, err := client.open(context.Background(), tagged)
		require.NoError(t, err)
		assert.Equal(t, "4", revision)
		assert.Equal(t, "rev-4", vars["TOKEN"].value)
	})

	t.Run("latest edited during the open is rejected", func(t *testing.T) {
		t.Parallel()
		env := &esctest.Environment{
			Variables: map[string]esctest.Variable{"TOKEN": {Value: "v"}},
			ETag:      "etag-1",
		}
		srv := esctest.New("tok", map[string]*esctest.Environment{"acme/billing/prod": env})
		t.Cleanup(srv.Close)
		var once sync.Once
		env.AfterHead = func() {
			once.Do(func() {
				srv.Update("acme", "billing", "prod", func(e *esctest.Environment) { e.ETag = "etag-2" })
			})
		}
		client, err := newSDKClient(srv.URL, "tok", nil)
		require.NoError(t, err)
		vars, revision, err := client.open(context.Background(), ref)
		require.ErrorIs(t, err, errRevisionMoved)
		assert.Nil(t, vars)
		assert.Empty(t, revision)
	})

	t.Run("a missing ETag is a failure", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, &esctest.Environment{Variables: map[string]esctest.Variable{"TOKEN": {Value: "v"}}})
		vars, revision, err := client.open(context.Background(), ref)
		require.Error(t, err)
		assert.Nil(t, vars)
		assert.Empty(t, revision)
	})

	t.Run("an unknown tag is a failure", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, &esctest.Environment{
			Variables: map[string]esctest.Variable{"TOKEN": {Value: "v"}},
			ETag:      "e",
		})
		unknown := ref
		unknown.Version = "missing"
		_, revision, err := client.open(context.Background(), unknown)
		require.Error(t, err)
		assert.Empty(t, revision)
	})
}
