package webui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readSSE(t *testing.T, r *bufio.Reader) (string, map[string]any) {
	t.Helper()
	name := ""
	var data map[string]any
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			name = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data))
		}
		if line == "" && name != "" {
			return name, data
		}
	}
}
func TestSSESnapshotSequenceReconnectAndCancellation(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{DayOfMonth: 7})
	s := newRunningServer(t, Options{Session: session})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL()+"/api/overview/stream", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	reader := bufio.NewReader(resp.Body)
	name, data := readSSE(t, reader)
	assert.Equal(t, "snapshot", name)
	assert.Empty(t, data["rows"])
	for phase := 1; phase <= 6; phase++ {
		session.Phase(phase, "loading")
		if phase > 1 {
			name, _ = readSSE(t, reader)
			assert.Equal(t, "phase", name)
		}
		name, data = readSSE(t, reader)
		assert.Equal(t, "phase", name)
		assert.InDelta(t, float64(phase), data["phase"], 1e-9)
		assert.Equal(t, "active", data["status"])
	}
	session.SetData(nil, overviewFixtureRows(), session.DateRange(), "dev")
	session.Row(0, overviewFixtureRows()[0])
	name, data = readSSE(t, reader)
	assert.Equal(t, "row", name)
	assert.Equal(t, "urn::cluster", data["urn"])
	session.Error(6, "plugin unavailable")
	name, data = readSSE(t, reader)
	assert.Equal(t, "phase", name)
	assert.Equal(t, "error", data["status"])
	name, data = readSSE(t, reader)
	assert.Equal(t, "error", name)
	assert.InDelta(t, float64(6), data["phase"], 1e-9)
	session.Ready(overviewFixtureRows())
	name, _ = readSSE(t, reader)
	assert.Equal(t, "phase", name)
	name, data = readSSE(t, reader)
	assert.Equal(t, "ready", name)
	assert.NotNil(t, data["totals"])
	reconnect := overviewRequest(t, s, "GET", "/api/overview/stream", "")
	name, data = readSSE(t, bufio.NewReader(reconnect.Body))
	assert.Equal(t, "snapshot", name)
	assert.Len(t, data["rows"], 3)
	assert.Equal(t, true, data["ready"])
	require.NoError(t, reconnect.Body.Close())
	cancel()
	require.NoError(t, resp.Body.Close())
	require.Eventually(t, func() bool { return session.SubscriberCount() == 0 }, time.Second, time.Millisecond)
}
func TestSlowSubscriberDoesNotBlockPipeline(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	_, events, unsubscribe := session.subscribe()
	defer unsubscribe()
	done := make(chan struct{})
	go func() {
		for i := range 1000 {
			session.Progress(i, 1000)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		require.FailNow(t, "slow subscriber blocked enrichment")
	}
	queued := 0
	for event := range events {
		queued++
		assert.Equal(t, "progress", event.name)
		assert.True(t, json.Valid(event.data), "queued progress remains valid JSON")
	}
	assert.Positive(t, queued)
	assert.LessOrEqual(t, queued, cap(events))
	assert.Equal(t, 0, session.SubscriberCount())
	session.Close()
}

type streamFailureWriter struct {
	header  http.Header
	writes  int
	errorAt int
	onWrite func()
}

func (w *streamFailureWriter) Header() http.Header { return w.header }
func (w *streamFailureWriter) WriteHeader(int)     {}
func (w *streamFailureWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == w.errorAt {
		return 0, errors.New("client disconnected")
	}
	if w.onWrite != nil {
		w.onWrite()
	}
	return len(data), nil
}
func (w *streamFailureWriter) Flush() {}

type streamWithoutFlusher struct{ writer *streamFailureWriter }

func (w streamWithoutFlusher) Header() http.Header            { return w.writer.Header() }
func (w streamWithoutFlusher) WriteHeader(status int)         { w.writer.WriteHeader(status) }
func (w streamWithoutFlusher) Write(data []byte) (int, error) { return w.writer.Write(data) }
func TestStreamOutputFailuresDetachSubscribers(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"snapshot-write", "snapshot-flush", "event-write", "cancelled-request"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			session := NewSession(context.Background(), SessionOptions{})
			defer session.Close()
			server := &Server{session: session}
			writer := &streamFailureWriter{header: make(http.Header)}
			var response http.ResponseWriter = writer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "snapshot-write":
				writer.errorAt = 1
			case "snapshot-flush":
				response = streamWithoutFlusher{writer: writer}
			case "event-write":
				writer.errorAt = 2
				writer.onWrite = func() { session.Progress(1, 1) }
			case "cancelled-request":
				cancel()
			}
			request := httptest.NewRequest(http.MethodGet, "/api/overview/stream", nil).WithContext(ctx)
			server.handleOverviewStream(response, request)
			assert.Equal(t, 0, session.SubscriberCount())
		})
	}
}

func TestReconnectRecoversFailedPhaseAndRetryClearsFailure(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.Phase(1, "Loading stack state...")
	_, events, unsubscribe := session.subscribe()
	defer unsubscribe()
	session.Error(1, "Stack loading failed")
	select {
	case event := <-events:
		assert.Equal(t, "phase", event.name)
		assert.Contains(t, string(event.data), `"status":"error"`)
	case <-time.After(time.Second):
		require.FailNow(t, "phase error transition missing")
	}
	first, _, reconnectStop := session.subscribe()
	defer reconnectStop()
	var snapshot OverviewSnapshot
	require.NoError(t, json.Unmarshal(first.data, &snapshot))
	require.Len(t, snapshot.Phases, 1)
	assert.Equal(t, "error", snapshot.Phases[0].Status)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(first.data, &payload))
	assert.NotEmpty(t, payload["errors"])
	session.Phase(1, "Retrying stack state...")
	next, _, retryStop := session.subscribe()
	defer retryStop()
	require.NoError(t, json.Unmarshal(next.data, &snapshot))
	assert.Equal(t, "active", snapshot.Phases[0].Status)
	require.NoError(t, json.Unmarshal(next.data, &payload))
	assert.Empty(t, payload["errors"])
}
