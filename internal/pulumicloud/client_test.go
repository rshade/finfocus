package pulumicloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
)

func TestClientListAndExport(t *testing.T) {
	t.Parallel()
	page1, err := os.ReadFile("testdata/updates.json")
	require.NoError(t, err)
	page2, err := os.ReadFile("testdata/updates-page2.json")
	require.NoError(t, err)
	export, err := os.ReadFile("testdata/export.json")
	require.NoError(t, err)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		assert.Equal(t, "token secret", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.pulumi+8", r.Header.Get("Accept"))
		if strings.Contains(r.URL.Path, "/export/") {
			_, _ = w.Write(export)
			return
		}
		assert.Equal(t, "100", r.URL.Query().Get("pageSize"))
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("ETag", "list-tag")
			_, _ = w.Write(page1)
		case "2":
			_, _ = w.Write(page2)
		default:
			_, _ = w.Write([]byte(`{"updates":[]}`))
		}
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL, Token: "secret"}, server.Client())
	list, err := client.ListUpdates(context.Background(), StackRef{"org", "project", "stack"}, "")
	require.NoError(t, err)
	require.Len(t, list.Updates, 131)
	assert.Equal(t, "list-tag", list.ETag)
	for i := 1; i < len(list.Updates); i++ {
		assert.Greater(t, list.Updates[i-1].Version, list.Updates[i].Version)
	}
	ids, err := client.ExportIdentities(
		context.Background(),
		StackRef{"org", "project", "stack"},
		1,
	)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, int64(4), requests.Load())
}

func TestClientListStopsWhenPulumiSendsNullPage(t *testing.T) {
	t.Parallel()
	var pages atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages.Add(1)
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"updates":[{"version":2,"kind":"update","startTime":10,"endTime":20}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"updates":null}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL, Token: "secret"}, server.Client())
	list, err := client.ListUpdates(context.Background(), StackRef{"org", "project", "stack"}, "")
	require.NoError(t, err)
	require.Len(t, list.Updates, 1)
	assert.Equal(t, 2, list.Updates[0].Version)
	assert.Equal(t, int64(2), pages.Load())
}

func TestClientErrorsAndRedirect(t *testing.T) {
	t.Parallel()
	for _, code := range []int{400, 401, 403, 404, 429, 500, 503} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(code)
					_, _ = w.Write([]byte("SECRET-RESPONSE"))
				}),
			)
			t.Cleanup(server.Close)
			client := NewClient(
				esc.Session{Backend: server.URL, Token: "SECRET-TOKEN"},
				server.Client(),
			)
			_, err := client.ListUpdates(
				context.Background(),
				StackRef{"org", "project", "stack"},
				"",
			)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "SECRET")
			switch code {
			case 401:
				require.ErrorIs(t, err, ErrUnauthorized)
			case 403:
				require.ErrorIs(t, err, ErrForbidden)
			case 404:
				require.ErrorIs(t, err, ErrNotFound)
			case 400:
				require.ErrorIs(t, err, ErrBadRequest)
			}
		})
	}
	var reached atomic.Int64
	target := httptest.NewServer(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { reached.Add(1) }),
	)
	t.Cleanup(target.Close)
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) },
		),
	)
	t.Cleanup(server.Close)
	_, err := NewClient(
		esc.Session{Backend: server.URL, Token: "secret"},
		server.Client(),
	).ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "")
	require.Error(t, err)
	assert.Zero(t, reached.Load())
}

func TestClientNestedAndConditional(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == "cached" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write(
				[]byte(
					`{"updates":[{"version":3,"info":{"kind":"update","result":"succeeded","startTime":10,"endTime":20}}]}`,
				),
			)
			return
		}
		_, _ = w.Write([]byte(`{"updates":[{"version":3,"kind":"update"}]}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL, Token: "secret"}, server.Client())
	list, err := client.ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "")
	require.NoError(t, err)
	require.Len(t, list.Updates, 1)
	assert.Equal(t, int64(20), list.Updates[0].End)
	assert.Equal(t, "update", list.Updates[0].Kind)
	list, err = client.ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "cached")
	require.NoError(t, err)
	assert.True(t, list.NotModified)
}

func TestClientRetriesTransientFailures(t *testing.T) {
	t.Parallel()
	for _, code := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int64
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if attempts.Add(1) < 3 {
						w.Header().Set("Retry-After", "2")
						w.WriteHeader(code)
						return
					}
					_, _ = w.Write([]byte(`{"updates":[]}`))
				}),
			)
			t.Cleanup(server.Close)
			client := NewClient(esc.Session{Backend: server.URL, Token: "secret"}, server.Client()).(*restClient)
			var waits []time.Duration
			client.sleep = func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil }
			_, err := client.ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "")
			require.NoError(t, err)
			assert.Equal(t, int64(3), attempts.Load())
			assert.Equal(t, []time.Duration{2 * time.Second, 2 * time.Second}, waits)
		})
	}
}

func TestClientRetryExhaustionAndDelay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, header string
		wantDelay    time.Duration
	}{
		{"seconds", "2", 2 * time.Second},
		{"HTTP date", now.Add(5 * time.Second).Format(http.TimeFormat), 5 * time.Second},
		{"no header", "", 0},
		{"invalid header", "bad", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int64
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					attempts.Add(1)
					w.Header().Set("Retry-After", tc.header)
					w.WriteHeader(http.StatusServiceUnavailable)
				}),
			)
			t.Cleanup(server.Close)
			client := NewClient(esc.Session{Backend: server.URL, Token: "secret"}, server.Client()).(*restClient)
			client.now = func() time.Time { return now }
			var waits []time.Duration
			client.sleep = func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil }
			_, err := client.ExportIdentities(context.Background(), StackRef{"o", "p", "s"}, 1)
			var throttled *RateLimitError
			require.ErrorAs(t, err, &throttled)
			assert.Equal(t, int64(3), attempts.Load())
			require.Len(t, waits, 2)
			if tc.wantDelay > 0 {
				assert.Equal(t, []time.Duration{tc.wantDelay, tc.wantDelay}, waits)
				assert.Equal(t, tc.wantDelay, throttled.RetryAfter)
			} else {
				for _, delay := range waits {
					assert.GreaterOrEqual(t, delay, 250*time.Millisecond)
					assert.LessOrEqual(t, delay, time.Second)
				}
			}
		})
	}
}

func TestClientCancellationStopsRetryWait(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		cancel()
	}))
	t.Cleanup(server.Close)
	_, err := NewClient(
		esc.Session{Backend: server.URL, Token: "secret"},
		server.Client(),
	).ListUpdates(ctx, StackRef{"o", "p", "s"}, "")
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int64(1), attempts.Load())
}

func TestClientConditionalHeaderOnlyOnFirstPage(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Query().Get("page") == "1" {
			assert.Equal(t, "stored-etag", r.Header.Get("If-None-Match"))
			_, _ = w.Write([]byte(`{"updates":[{"version":1,"kind":"update"}]}`))
			return
		}
		assert.Empty(t, r.Header.Get("If-None-Match"))
		_, _ = w.Write([]byte(`{"updates":[]}`))
	}))
	t.Cleanup(server.Close)
	list, err := NewClient(
		esc.Session{Backend: server.URL, Token: "t"},
		server.Client(),
	).ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "stored-etag")
	require.NoError(t, err)
	assert.Len(t, list.Updates, 1)
	assert.Equal(t, int64(2), requests.Load())
}

func TestClientNeverRetriesPermanentFailures(t *testing.T) {
	t.Parallel()
	for _, code := range []int{400, 401, 403, 404, 500} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int64
			server := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, _ *http.Request) { attempts.Add(1); w.WriteHeader(code) },
				),
			)
			t.Cleanup(server.Close)
			_, err := NewClient(
				esc.Session{Backend: server.URL, Token: "t"},
				server.Client(),
			).ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "")
			require.Error(t, err)
			assert.Equal(t, int64(1), attempts.Load())
		})
	}
}

type countingTransport struct{ calls atomic.Int64 }

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nil, http.ErrUseLastResponse
}

func TestClientRefusesInsecureRemoteBackendBeforeAnyRequest(t *testing.T) {
	t.Parallel()
	transport := &countingTransport{}
	client := NewClient(
		esc.Session{Backend: "http://pulumi.example.test", Token: "SECRET-TOKEN"},
		&http.Client{Transport: transport},
	)
	stack := StackRef{"o", "p", "s"}
	_, listErr := client.ListUpdates(context.Background(), stack, "")
	_, exportErr := client.ExportIdentities(context.Background(), stack, 1)
	for _, err := range []error{listErr, exportErr} {
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTPS")
		assert.NotContains(t, err.Error(), "SECRET")
	}
	assert.Zero(t, transport.calls.Load())
}

func TestClientExportErrorsMapToTypedErrors(t *testing.T) {
	t.Parallel()
	for code, want := range map[int]error{
		400: ErrBadRequest,
		401: ErrUnauthorized,
		403: ErrForbidden,
		404: ErrNotFound,
	} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"code":` + strconv.Itoa(code) + `,"message":"SECRET-RESPONSE"}`))
			}))
			t.Cleanup(server.Close)
			_, err := NewClient(
				esc.Session{Backend: server.URL, Token: "SECRET-TOKEN"},
				server.Client(),
			).ExportIdentities(context.Background(), StackRef{"o", "p", "s"}, 7)
			require.ErrorIs(t, err, want)
			assert.NotContains(t, err.Error(), "SECRET")
		})
	}
}

func TestClientExportRetriesRateLimitThenSucceeds(t *testing.T) {
	t.Parallel()
	export, err := os.ReadFile("testdata/export.json")
	require.NoError(t, err)
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(export)
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL, Token: "t"}, server.Client()).(*restClient)
	var waits []time.Duration
	client.sleep = func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil }
	ids, err := client.ExportIdentities(context.Background(), StackRef{"o", "p", "s"}, 1)
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, []time.Duration{3 * time.Second}, waits)
}

func TestClientEscapesStackPathSegments(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		if strings.Contains(r.URL.Path, "/export/") {
			_, _ = w.Write([]byte(`{"version":3,"deployment":{"resources":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"updates":null}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL, Token: "t"}, server.Client())
	stack := StackRef{"o rg", "p", "a b/c"}
	_, err := client.ListUpdates(context.Background(), stack, "")
	require.NoError(t, err)
	_, err = client.ExportIdentities(context.Background(), stack, 12)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/api/stacks/o%20rg/p/a%20b%2Fc/updates",
		"/api/stacks/o%20rg/p/a%20b%2Fc/export/12",
	}, paths)
}

func TestClientKeepsBackendPathPrefix(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		if strings.Contains(r.URL.Path, "/export/") {
			_, _ = w.Write([]byte(`{"version":3,"deployment":{"resources":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"updates":null}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL + "/gateway/", Token: "t"}, server.Client())
	stack := StackRef{"o", "p", "s"}
	_, err := client.ListUpdates(context.Background(), stack, "")
	require.NoError(t, err)
	_, err = client.ExportIdentities(context.Background(), stack, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/gateway/api/stacks/o/p/s/updates",
		"/gateway/api/stacks/o/p/s/export/2",
	}, paths)
}

func TestClientExportStopsAtByteLimit(t *testing.T) {
	t.Parallel()
	const limit = 1024
	for _, tc := range []struct {
		name    string
		padding int
		wantErr bool
	}{
		{"within limit", 0, false},
		{"exactly at limit", limit - len(`{"version":3,"deployment":{"resources":[]}}`), false},
		{"one byte over", limit - len(`{"version":3,"deployment":{"resources":[]}}`) + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var sent atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				body := `{"version":3,"deployment":{"resources":[]}}` + strings.Repeat(" ", tc.padding)
				sent.Store(int64(len(body)))
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(server.Close)
			client := NewClient(esc.Session{Backend: server.URL, Token: "t"}, server.Client()).(*restClient)
			client.maxExportBytes = limit
			ids, err := client.ExportIdentities(context.Background(), StackRef{"o", "p", "s"}, 1)
			if tc.wantErr {
				require.ErrorIs(t, err, errExportTooLarge)
				assert.Contains(t, err.Error(), "exceeds 256 MiB limit")
				return
			}
			require.NoError(t, err)
			assert.Empty(t, ids)
		})
	}
}

func TestClientUpdateListStopsAtByteLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"updates":[` + strings.Repeat(`{"version":1},`, 8) + `{"version":2}]}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(esc.Session{Backend: server.URL, Token: "t"}, server.Client()).(*restClient)
	client.maxUpdateListBytes = 64
	_, err := client.ListUpdates(context.Background(), StackRef{"o", "p", "s"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Pulumi Cloud update-list response")
}
