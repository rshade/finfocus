package weblog_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func retained(h *weblog.Hub) []weblog.Entry {
	snap, _, cancel := h.Subscribe(0)
	cancel()
	return snap.Entries
}

func TestWriterTagsLines(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	w := h.Writer("aws-public")
	_, err := w.Write(
		[]byte(
			"plain line\n{\"level\":\"warn\",\"message\":\"json\"}\n{\"plugin_name\":\"own\",\"message\":\"named\"}\n",
		),
	)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	entries := retained(h)
	require.Len(t, entries, 3)
	assert.Equal(t, "aws-public", entries[0].Source)
	assert.True(t, entries[0].Raw)
	assert.Equal(t, "aws-public", entries[1].Source)
	assert.Equal(t, "warn", entries[1].Level)
	assert.Equal(t, "own", entries[2].Source)
}

func TestWriterAssemblesAcrossWritesAndFlushesOnClose(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	w := h.Writer("p")
	n, err := w.Write([]byte("par"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Empty(t, retained(h))
	_, _ = w.Write([]byte("tial"))
	require.NoError(t, w.Close())
	entries := retained(h)
	require.Len(t, entries, 1)
	assert.Equal(t, "partial", entries[0].Message)
}

func TestWriterMarksOversizeTailTruncated(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	w := h.Writer("p")
	_, _ = w.Write([]byte(strings.Repeat("a", weblog.MaxLineBytes+1)))
	entries := retained(h)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Truncated)
}

func TestWriterConcurrent(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(1000)
	w := h.Writer("p")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				_, _ = fmt.Fprintf(w, "line %d\n", i)
			}
		}()
	}
	wg.Wait()
	require.NoError(t, w.Close())
	assert.Len(t, retained(h), 400)
}

func TestTeeOneEventPerWrite(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	tee := h.Tee()
	n, err := tee.Write([]byte(`{"level":"error","component":"overview","message":"boom"}` + "\n"))
	require.NoError(t, err)
	assert.Equal(t, 58, n)
	_, _ = tee.Write([]byte(`{"level":"info","message":"no newline"}`))
	_, _ = tee.Write([]byte("\n"))
	_, _ = tee.Write([]byte("not json\r\n"))

	entries := retained(h)
	require.Len(t, entries, 3)
	assert.Equal(t, "overview", entries[0].Source)
	assert.Equal(t, "error", entries[0].Level)
	assert.Equal(t, "core", entries[1].Source)
	assert.Equal(t, "no newline", entries[1].Message)
	assert.True(t, entries[2].Raw)
	assert.Equal(t, "not json", entries[2].Message)
	assert.Equal(t, "core", entries[2].Source)
}

func TestTeeTruncatesOversizeEvent(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	_, _ = h.Tee().Write([]byte(strings.Repeat("a", weblog.MaxLineBytes+5)))
	entries := retained(h)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Truncated)
}

func TestNilHubWritersDiscard(t *testing.T) {
	t.Parallel()
	var h *weblog.Hub
	w := h.Writer("x")
	n, err := w.Write([]byte("data\n"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	require.NoError(t, w.Close())
	n, err = h.Tee().Write([]byte("data"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(1)
	ctx := weblog.ContextWithHub(context.Background(), h)
	assert.Same(t, h, weblog.HubFromContext(ctx))
	assert.Nil(t, weblog.HubFromContext(context.Background()))
}
