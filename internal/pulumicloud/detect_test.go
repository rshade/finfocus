package pulumicloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
)

func TestDetect(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		code int
		want Reason
	}{{200, ""}, {204, ""}, {401, TokenRejected}, {403, NoPermission}, {500, Unreachable}} {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/user", r.URL.Path)
				w.WriteHeader(tc.code)
			}))
			t.Cleanup(server.Close)
			got := Detect(context.Background(), esc.Session{Backend: server.URL, Token: "t"}, StackRef{"o", "p", "s"})
			assert.Equal(t, tc.want, got.Reason)
			assert.Equal(t, tc.want == "", got.Available)
		})
	}
	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { count.Add(1) }))
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		backend, token string
		stack          StackRef
		want           Reason
	}{
		{server.URL, "", StackRef{}, NotLoggedIn},
		{"file:///tmp/state", "t", StackRef{}, NonCloudBackend}, {"s3://bucket", "t", StackRef{}, NonCloudBackend},
		{"gs://bucket", "t", StackRef{}, NonCloudBackend}, {"azblob://bucket", "t", StackRef{}, NonCloudBackend},
		{"http://remote.example", "t", StackRef{}, InsecureBackend}, {server.URL, "t", StackRef{}, UnknownStack},
	} {
		got := Detect(context.Background(), esc.Session{Backend: tc.backend, Token: tc.token}, tc.stack)
		assert.Equal(t, tc.want, got.Reason)
		assert.False(t, got.Available)
	}
	assert.Zero(t, count.Load())
	server.Close()
	got := Detect(context.Background(), esc.Session{Backend: server.URL, Token: "t"}, StackRef{"o", "p", "s"})
	require.Equal(t, Unreachable, got.Reason)
}
