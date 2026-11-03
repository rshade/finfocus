package weblog_test

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func collect(t *testing.T, r io.Reader, tag string) []weblog.Entry {
	t.Helper()
	var out []weblog.Entry
	for e, err := range weblog.Decode(r, tag) {
		require.NoError(t, err)
		out = append(out, e)
	}
	return out
}

func TestDecodeSample(t *testing.T) {
	t.Parallel()
	f, err := os.Open("testdata/sample.ndjson")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	entries := collect(t, f, "")
	require.Len(t, entries, 8)
	for i, e := range entries {
		assert.Equal(t, uint64(i+1), e.Seq)
	}

	assert.Equal(t, "overview", entries[0].Source)
	assert.Equal(t, "overview started", entries[0].Message)
	assert.Equal(t, "aws-public", entries[1].Source)
	assert.Equal(t, weblog.KindPlugin, entries[1].SourceKind)

	assert.True(t, entries[2].Raw)
	assert.Equal(t, "unknown", entries[2].Source)
	assert.Equal(t, weblog.KindCore, entries[2].SourceKind)

	assert.Equal(t, "[redacted]", entries[3].Fields["api_token"])
	assert.Equal(t, map[string]any{"password": "[redacted]", "host": "example.test"}, entries[3].Fields["nested"])
	assert.Equal(t, "[redacted]", entries[4].Fields["instanceType"])
	assert.NotContains(t, entries[5].Fields, "__defaults")
	assert.Equal(t, "yes", entries[5].Fields["keep"])
	assert.Equal(t, "info", entries[6].Level)
	assert.Equal(t, "warn", entries[7].Level)
	assert.Equal(t, "kubernetes", entries[7].Source)
}

func TestDecodeTagsRawLines(t *testing.T) {
	t.Parallel()
	entries := collect(t, strings.NewReader("plain\n"), "aws")
	require.Len(t, entries, 1)
	assert.Equal(t, "aws", entries[0].Source)
}

func TestDecodeJSONLineWithoutSourceIsCore(t *testing.T) {
	t.Parallel()
	entries := collect(t, strings.NewReader(`{"message":"m"}`), "")
	require.Len(t, entries, 1)
	assert.Equal(t, "core", entries[0].Source)
}

func TestDecodeLongLine(t *testing.T) {
	t.Parallel()
	long := `{"level":"info","message":"` + strings.Repeat("z", weblog.MaxLineBytes+500) + `"}` + "\n" + "after\n"
	entries := collect(t, strings.NewReader(long), "")
	require.Len(t, entries, 2)
	assert.True(t, entries[0].Truncated)
	assert.Equal(t, "after", entries[1].Message)
}

type failingReader struct{ sent bool }

func (f *failingReader) Read(p []byte) (int, error) {
	if !f.sent {
		f.sent = true
		return copy(p, "one\n"), nil
	}
	return 0, errors.New("disk gone")
}

func TestDecodeReadError(t *testing.T) {
	t.Parallel()
	var entries []weblog.Entry
	var gotErr error
	for e, err := range weblog.Decode(&failingReader{}, "") {
		if err != nil {
			gotErr = err
			break
		}
		entries = append(entries, e)
	}
	require.Len(t, entries, 1)
	require.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "disk gone")
}

func TestDecodeStopsEarly(t *testing.T) {
	t.Parallel()
	count := 0
	for range weblog.Decode(strings.NewReader("a\nb\nc\nd\n"), "") {
		count++
		if count == 2 {
			break
		}
	}
	assert.Equal(t, 2, count)
}

type generatedLines struct {
	total, written int
	pending        []byte
}

func (g *generatedLines) Read(p []byte) (int, error) {
	if len(g.pending) == 0 {
		if g.written >= g.total {
			return 0, io.EOF
		}
		g.written++
		g.pending = []byte(`{"level":"info","message":"line ` + itoa(g.written) + `"}` + "\n")
	}
	n := copy(p, g.pending)
	g.pending = g.pending[n:]
	return n, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

const tailSize = 200

func tailOf(r io.Reader, n int) []weblog.Entry {
	ring := make([]weblog.Entry, 0, n)
	for e, err := range weblog.Decode(r, "") {
		if err != nil {
			break
		}
		if len(ring) == n {
			copy(ring, ring[1:])
			ring = ring[:n-1]
		}
		ring = append(ring, e)
	}
	return ring
}

func TestDecodeLargeInputWithTailRing(t *testing.T) {
	t.Parallel()
	ring := tailOf(&generatedLines{total: 200_000}, tailSize)
	require.Len(t, ring, tailSize)
	assert.Equal(t, uint64(200_000), ring[tailSize-1].Seq)
	assert.Equal(t, "line 200000", ring[tailSize-1].Message)
	assert.Equal(t, uint64(200_000-tailSize+1), ring[0].Seq)
}

// BenchmarkDecodeTail measures streaming a generated log through Decode with a
// 200-entry tail ring (SC-010). Run `go test -bench DecodeTail -benchmem
// ./internal/weblog/` to see MB/s; each iteration decodes 100,000 lines.
// Recorded 2026-11-02 (AMD Ryzen AI 7 PRO 350, WSL2): about 18 MB/s, roughly
// 220 ms per 100,000 short JSON lines.
func BenchmarkDecodeTail(b *testing.B) {
	const lines = 100_000
	probe := &generatedLines{total: lines}
	size, _ := io.Copy(io.Discard, probe)
	b.SetBytes(size)
	b.ResetTimer()
	for b.Loop() {
		ring := tailOf(&generatedLines{total: lines}, tailSize)
		if len(ring) != tailSize {
			b.Fatalf("ring holds %d entries", len(ring))
		}
	}
}
