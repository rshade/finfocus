package webui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func logEntry(source, message string) weblog.Entry {
	return weblog.Entry{
		Time:       "2026-11-02T18:04:11Z",
		Level:      "info",
		Source:     source,
		SourceKind: weblog.KindPlugin,
		Message:    message,
	}
}

type sseFrame struct {
	id    string
	event string
	data  map[string]any
}

func readFrame(t *testing.T, r *bufio.Reader) sseFrame {
	t.Helper()
	var f sseFrame
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if f.event != "" {
				return f
			}
		case strings.HasPrefix(line, "id: "):
			f.id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			f.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &f.data))
		}
	}
}

func openLogStream(t *testing.T, s *Server, lastID string) (*bufio.Reader, *http.Response) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL()+"/api/logs/stream?token=ignored", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return bufio.NewReader(resp.Body), resp
}

func TestLogsStatus(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	hub.Append(logEntry("core", "one"))
	hub.Append(logEntry("aws-public", "two"))
	s := newRunningServer(t, Options{Logs: hub})

	resp := overviewRequest(t, s, http.MethodGet, "/api/logs/status", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body weblog.Status
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.Enabled)
	assert.Equal(t, 50, body.Capacity)
	assert.Equal(t, 2, body.Retained)
	assert.Len(t, body.Sources, 2)
}

func TestLogsDisabledRoutesAreNotFound(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	for _, path := range []string{"/api/logs/status", "/api/logs/stream"} {
		resp := overviewRequest(t, s, http.MethodGet, path, "")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "not_found", body["code"], path)
	}
}

func TestLogsNonGetIsMethodNotAllowed(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{Logs: weblog.NewHub(10)})
	for _, path := range []string{"/api/logs/status", "/api/logs/stream"} {
		resp := overviewRequest(t, s, http.MethodPost, path, "{}")
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, path)
		assert.Equal(t, http.MethodGet, resp.Header.Get("Allow"), path)
	}
}

func TestLogsRequireSessionCookieAndHost(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{Logs: weblog.NewHub(10)})
	for _, path := range []string{"/api/logs/status", "/api/logs/stream"} {
		req, err := http.NewRequest(http.MethodGet, s.BaseURL()+path, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
		require.NoError(t, resp.Body.Close())

		req, err = http.NewRequest(http.MethodGet, s.BaseURL()+path, nil)
		require.NoError(t, err)
		req.Host = "evil.example:" + strconv.Itoa(s.Port())
		req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, path)
		require.NoError(t, resp.Body.Close())
	}
}

func TestLogsStreamSnapshotThenLive(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	hub.Append(logEntry("core", "first"))
	hub.Append(logEntry("aws-public", "second"))
	s := newRunningServer(t, Options{Logs: hub})

	reader, resp := openLogStream(t, s, "")
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))

	snap := readFrame(t, reader)
	assert.Equal(t, "snapshot", snap.event)
	assert.Equal(t, "2", snap.id)
	assert.Len(t, snap.data["entries"], 2)
	assert.Len(t, snap.data["sources"], 2)
	assert.InDelta(t, 50, snap.data["capacity"], 1e-9)

	hub.Append(logEntry("core", "third"))
	live := readFrame(t, reader)
	assert.Equal(t, "log", live.event)
	assert.Equal(t, "3", live.id)
	assert.Equal(t, "third", live.data["message"])
	assert.InDelta(t, 3, live.data["seq"], 1e-9)
}

func TestLogsStreamResumesAfterLastEventID(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	for _, m := range []string{"a", "b", "c", "d"} {
		hub.Append(logEntry("core", m))
	}
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "2")
	snap := readFrame(t, reader)
	assert.Equal(t, "snapshot", snap.event)
	entries, ok := snap.data["entries"].([]any)
	require.True(t, ok)
	require.Len(t, entries, 2)
	first, ok := entries[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "c", first["message"])
	assert.Equal(t, "4", snap.id)
}

func TestLogsStreamSnapshotWithNoEntriesKeepsRequestedID(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	hub.Append(logEntry("core", "a"))
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "1")
	snap := readFrame(t, reader)
	assert.Equal(t, "snapshot", snap.event)
	assert.Equal(t, "1", snap.id)
	assert.Empty(t, snap.data["entries"])
}

func TestLogsStreamSendsGapWhenEntriesWereEvicted(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(2)
	for _, m := range []string{"a", "b", "c", "d", "e"} {
		hub.Append(logEntry("core", m))
	}
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "1")
	assert.Equal(t, "snapshot", readFrame(t, reader).event)
	gap := readFrame(t, reader)
	assert.Equal(t, "gap", gap.event)
	assert.Empty(t, gap.id)
	assert.InDelta(t, 2, gap.data["missed"], 1e-9)
}

func TestLogsStreamFirstConnectionToWrappedBufferSendsNoGap(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(2)
	for _, m := range []string{"a", "b", "c", "d", "e"} {
		hub.Append(logEntry("core", m))
	}
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "")
	assert.Equal(t, "snapshot", readFrame(t, reader).event)

	hub.Append(logEntry("core", "f"))
	next := readFrame(t, reader)
	assert.Equal(t, "log", next.event, "a first connection has no earlier position to miss lines from")
}

func TestLogsStreamIgnoresMalformedLastEventID(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	hub.Append(logEntry("core", "a"))
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "not-a-number")
	snap := readFrame(t, reader)
	assert.Len(t, snap.data["entries"], 1)
}

func TestLogsStreamClosesLaggingSubscriberWithoutDelayingOthers(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(1000)
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "")
	assert.Equal(t, "snapshot", readFrame(t, reader).event)

	_, slow, cancelSlow := hub.Subscribe(0)
	defer cancelSlow()

	const total = 300
	for i := range total {
		hub.Append(logEntry("core", "m"+strconv.Itoa(i)))
		frame := readFrame(t, reader)
		assert.Equal(t, "log", frame.event)
		assert.Equal(t, strconv.Itoa(i+1), frame.id)
	}

	select {
	case _, ok := <-drain(slow):
		assert.False(t, ok, "lagging subscriber channel must be closed")
	case <-time.After(5 * time.Second):
		require.Fail(t, "lagging subscriber was not closed")
	}
}

func drain(ch <-chan weblog.Entry) <-chan weblog.Entry {
	out := make(chan weblog.Entry)
	go func() {
		defer close(out)
		for range ch {
		}
	}()
	return out
}

func TestLogsStreamEndsOnServerShutdown(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	s := newRunningServer(t, Options{Logs: hub})

	reader, _ := openLogStream(t, s, "")
	assert.Equal(t, "snapshot", readFrame(t, reader).event)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
	_, err := reader.ReadString('\n')
	require.Error(t, err)
}

func TestRequestLogSkipsLogsRoutes(t *testing.T) {
	t.Parallel()
	var buf syncBuffer
	logger := zerolog.New(&buf)
	hub := weblog.NewHub(10)
	s := newRunningServer(t, Options{Logs: hub, Logger: &logger})

	resp := overviewRequest(t, s, http.MethodGet, "/api/logs/status", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, buf.String(), "/api/logs/")

	resp = overviewRequest(t, s, http.MethodGet, "/api/nope", "")
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, buf.String(), "/api/nope")
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestViewerAssetsServedAsJavaScriptAndCSS(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	tests := []struct {
		path string
		mime string
	}{
		{"/static/zerolog-viewer/index.js", "javascript"},
		{"/static/zerolog-viewer/viewer.css", "text/css"},
	}
	for _, tc := range tests {
		resp := overviewRequest(t, s, http.MethodGet, tc.path, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode, tc.path)
		assert.Contains(t, resp.Header.Get("Content-Type"), tc.mime, tc.path)
	}
}
