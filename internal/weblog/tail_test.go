package weblog_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func openBody(t *testing.T, body string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func infoLine(msg string) string {
	return `{"level":"info","message":"` + msg + `"}` + "\n"
}

func errorLine(msg string) string {
	return `{"level":"error","message":"` + msg + `"}` + "\n"
}

func oracleTail(t *testing.T, body string, n int, match func(weblog.Entry) bool) []weblog.Entry {
	t.Helper()
	var all []weblog.Entry
	for e, err := range weblog.Decode(strings.NewReader(body), "") {
		require.NoError(t, err)
		if match == nil || match(e) {
			all = append(all, e)
		}
	}
	if n < len(all) {
		all = all[len(all)-n:]
	}
	return all
}

func stripVolatile(e weblog.Entry) weblog.Entry {
	e.Seq = 0
	e.Time = ""
	e.Display.Time = ""
	return e
}

func TestTailDecodeMatchesFullDecode(t *testing.T) {
	t.Parallel()
	var big strings.Builder
	for i := 1; i <= 3000; i++ {
		big.WriteString(infoLine(fmt.Sprintf("n%d", i)))
	}
	isError := func(e weblog.Entry) bool { return e.Level == "error" }

	cases := []struct {
		name  string
		body  string
		n     int
		match func(weblog.Entry) bool
	}{
		{name: "empty", body: "", n: 5},
		{name: "newlines only", body: "\n\n\n", n: 5},
		{name: "shorter than tail", body: infoLine("a") + infoLine("b"), n: 10},
		{name: "no trailing newline", body: strings.TrimSuffix(infoLine("a")+infoLine("b"), "\n"), n: 1},
		{
			name: "crlf",
			body: "{\"level\":\"info\",\"message\":\"a\"}\r\n" +
				"{\"level\":\"info\",\"message\":\"b\"}\r\n",
			n: 1,
		},
		{name: "blank lines", body: infoLine("a") + "\n\n" + infoLine("b") + "\n" + infoLine("c"), n: 2},
		{name: "many lines", body: big.String(), n: 200},
		{name: "many lines no trailing newline", body: strings.TrimSuffix(big.String(), "\n"), n: 3},
		{
			name:  "level filter",
			body:  infoLine("a") + errorLine("e1") + infoLine("b") + errorLine("e2") + infoLine("c"),
			n:     2,
			match: isError,
		},
		{
			name:  "only match is near the start",
			body:  errorLine("early") + strings.Repeat(infoLine("x"), 40),
			n:     1,
			match: isError,
		},
		{
			name:  "filter matches nothing",
			body:  infoLine("a") + infoLine("b") + infoLine("c"),
			n:     2,
			match: isError,
		},
		{name: "raw text", body: "plain one\nplain two\n", n: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := weblog.TailDecode(openBody(t, tc.body), tc.n, "", tc.match)
			require.NoError(t, err)
			want := oracleTail(t, tc.body, tc.n, tc.match)
			require.Len(t, got, len(want), "messages")
			for i := range want {
				assert.Equal(t, stripVolatile(want[i]), stripVolatile(got[i]), "entry %d", i)
			}
		})
	}
}

func TestTailDecodeSequenceFollowsTheSuffix(t *testing.T) {
	t.Parallel()
	body := infoLine("1") + infoLine("2") + infoLine("3") + infoLine("4") + infoLine("5")
	got, err := weblog.TailDecode(openBody(t, body), 2, "", nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "4", got[0].Message)
	assert.Equal(t, uint64(1), got[0].Seq)
	assert.Equal(t, uint64(2), got[1].Seq)

	filtered := errorLine("e1") + infoLine("mid") + errorLine("e2")
	isError := func(e weblog.Entry) bool { return e.Level == "error" }
	got, err = weblog.TailDecode(openBody(t, filtered), 2, "", isError)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "e1", got[0].Message)
	assert.Equal(t, uint64(1), got[0].Seq)
	assert.Equal(t, "e2", got[1].Message)
	assert.Equal(t, uint64(3), got[1].Seq, "the info line between the errors keeps its suffix sequence")
}

func TestTailDecodeLongLineMatchesForwardScan(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", weblog.MaxLineBytes+100)
	body := long + "\n" + infoLine("after")

	t.Run("tail stays after the long line", func(t *testing.T) {
		t.Parallel()
		got, err := weblog.TailDecode(openBody(t, body), 1, "", nil)
		require.NoError(t, err)
		want := oracleTail(t, body, 1, nil)
		require.Len(t, got, len(want))
		assert.Equal(t, stripVolatile(want[0]), stripVolatile(got[0]))
		assert.Equal(t, "after", got[0].Message)
		assert.Equal(t, uint64(1), got[0].Seq, "the long line is outside this suffix")
	})

	t.Run("tail that includes the long line", func(t *testing.T) {
		t.Parallel()
		got, err := weblog.TailDecode(openBody(t, body), 5, "", nil)
		require.NoError(t, err)
		want := oracleTail(t, body, 5, nil)
		require.Len(t, got, len(want))
		for i := range want {
			assert.Equal(t, stripVolatile(want[i]), stripVolatile(got[i]), "entry %d", i)
			assert.Equal(t, want[i].Seq, got[i].Seq, "fallback keeps the whole-file sequence")
		}
	})
}

func TestTailDecodeRejectsNonPositiveCount(t *testing.T) {
	t.Parallel()
	_, err := weblog.TailDecode(openBody(t, infoLine("a")), 0, "", nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "tail count must be positive")
}

type errSeeker struct{}

func (errSeeker) Read([]byte) (int, error) { return 0, errors.New("disk gone") }

func (errSeeker) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return 32, nil
	}
	if offset != 0 && whence == io.SeekStart {
		return offset, nil
	}
	return 0, nil
}

func TestTailDecodeReadError(t *testing.T) {
	t.Parallel()
	_, err := weblog.TailDecode(errSeeker{}, 1, "", nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "disk gone")
}

// guardFile reports a large size but only stores the tail bytes. A read
// before that tail fails. SC-010 requires the last 200 entries of a 500 MB
// file in under 5 seconds, which holds only when the prefix is not read.
type guardFile struct {
	size int64
	tail []byte
	off  int64
}

func (g *guardFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		g.off = offset
	case io.SeekCurrent:
		g.off += offset
	case io.SeekEnd:
		g.off = g.size + offset
	default:
		return 0, errors.New("bad whence")
	}
	if g.off < 0 || g.off > g.size {
		return 0, fmt.Errorf("seek %d outside 0..%d", g.off, g.size)
	}
	return g.off, nil
}

func (g *guardFile) Read(p []byte) (int, error) {
	if g.off >= g.size {
		return 0, io.EOF
	}
	tailAt := g.size - int64(len(g.tail))
	if g.off < tailAt {
		return 0, fmt.Errorf("read prefix at %d", g.off)
	}
	n := copy(p, g.tail[g.off-tailAt:])
	g.off += int64(n)
	if g.off >= g.size {
		return n, io.EOF
	}
	return n, nil
}

func TestTailDecodeLargeFileDoesNotReadPrefix(t *testing.T) {
	t.Parallel()
	var raw []byte
	var messages []string
	for len(raw) < 64*1024 {
		msg := fmt.Sprintf("line %d", len(messages)+1)
		messages = append(messages, msg)
		raw = append(raw, []byte(infoLine(msg))...)
	}
	file := &guardFile{size: 500 << 20, tail: raw}
	start := time.Now()
	got, err := weblog.TailDecode(file, tailSize, "", nil)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Len(t, got, tailSize)
	assert.Equal(t, messages[len(messages)-tailSize], got[0].Message)
	assert.Equal(t, messages[len(messages)-1], got[tailSize-1].Message)
	assert.Equal(t, uint64(1), got[0].Seq)
	assert.Less(t, elapsed, 5*time.Second, "SC-010 allows 5 seconds; took %s", elapsed)
}

func TestTailDecodeLineAcrossProbe(t *testing.T) {
	t.Parallel()
	body := infoLine("before") +
		`{"level":"info","message":"` + strings.Repeat("q", 40*1024) + "\"}\n" +
		strings.Repeat("m", weblog.MaxLineBytes) + "\n" +
		infoLine("after")
	got, err := weblog.TailDecode(openBody(t, body), 2, "", nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, uint64(1), got[0].Seq, "a line of MaxLineBytes stays in the suffix")
	assert.Equal(t, strings.Repeat("m", weblog.MaxLineBytes), got[0].Message)
	assert.Equal(t, "after", got[1].Message)
	assert.Equal(t, uint64(2), got[1].Seq)
}

func TestTailDecodeNoMatchDoesNotReread(t *testing.T) {
	t.Parallel()
	const lines = 4000
	var body bytes.Buffer
	for i := 1; i <= lines; i++ {
		body.WriteString(infoLine(fmt.Sprintf("n%d", i)))
	}
	file := &countSeeker{data: body.Bytes()}
	isError := func(e weblog.Entry) bool { return e.Level == "error" }
	got, err := weblog.TailDecode(file, 20, "", isError)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, int64(len(file.data)), file.nbytes)
	blocks := (len(file.data) + tailProbeForTest - 1) / tailProbeForTest
	assert.LessOrEqual(t, file.seeks, blocks+3)
}

func TestTailDecodeSparseFilterSeeksByBlock(t *testing.T) {
	t.Parallel()
	const lines = 4000
	var body bytes.Buffer
	body.WriteString(errorLine("early"))
	for i := 1; i < lines; i++ {
		body.WriteString(infoLine(fmt.Sprintf("n%d", i)))
	}
	file := &countSeeker{data: body.Bytes()}
	isError := func(e weblog.Entry) bool { return e.Level == "error" }
	got, err := weblog.TailDecode(file, 1, "", isError)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "early", got[0].Message)
	assert.Equal(t, uint64(1), got[0].Seq)
	assert.Less(t, file.seeks, lines/10)
	blocks := (len(file.data) + tailProbeForTest - 1) / tailProbeForTest
	assert.LessOrEqual(t, file.seeks, 2*blocks+4)
}

// tailProbeForTest matches tailProbeSize. The seek bounds fail if the search
// starts reading one line at a time.
const tailProbeForTest = 32 * 1024

type countSeeker struct {
	data   []byte
	off    int64
	seeks  int
	nbytes int64
}

func (c *countSeeker) Seek(offset int64, whence int) (int64, error) {
	c.seeks++
	switch whence {
	case io.SeekStart:
		c.off = offset
	case io.SeekCurrent:
		c.off += offset
	case io.SeekEnd:
		c.off = int64(len(c.data)) + offset
	default:
		return 0, errors.New("bad whence")
	}
	if c.off < 0 || c.off > int64(len(c.data)) {
		return 0, errors.New("seek out of range")
	}
	return c.off, nil
}

func (c *countSeeker) Read(p []byte) (int, error) {
	if c.off >= int64(len(c.data)) {
		return 0, io.EOF
	}
	n := copy(p, c.data[c.off:])
	c.off += int64(n)
	c.nbytes += int64(n)
	if c.off >= int64(len(c.data)) {
		return n, io.EOF
	}
	return n, nil
}

func BenchmarkTailDecode(b *testing.B) {
	const lines = 20_000
	var body bytes.Buffer
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&body, "{\"level\":\"info\",\"message\":\"line %d\"}\n", i)
	}
	path := filepath.Join(b.TempDir(), "log")
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		b.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	b.SetBytes(int64(body.Len()))
	b.ResetTimer()
	for b.Loop() {
		got, tailErr := weblog.TailDecode(f, tailSize, "", nil)
		if tailErr != nil || len(got) != tailSize {
			b.Fatalf("tail: %v len %d", tailErr, len(got))
		}
	}
}
