package esc

import (
	"context"
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
	assert.Equal(t, 1, srv.Heads("acme", "billing", "prod"))
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
