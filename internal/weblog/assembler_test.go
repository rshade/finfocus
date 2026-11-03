package weblog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type emitted struct {
	line      string
	truncated bool
}

func runAssembler(writes []string, closeAfter bool) []emitted {
	var out []emitted
	a := newAssembler(func(line []byte, truncated bool) {
		out = append(out, emitted{string(line), truncated})
	})
	for _, w := range writes {
		a.write([]byte(w))
	}
	if closeAfter {
		a.close()
	}
	return out
}

func TestAssembler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		writes []string
		close  bool
		want   []string
	}{
		{"line split across writes", []string{"hel", "lo\nwor", "ld\n"}, false, []string{"hello", "world"}},
		{"several lines in one write", []string{"a\nb\nc\n"}, false, []string{"a", "b", "c"}},
		{"crlf trimmed", []string{"a\r\nb\r\n"}, false, []string{"a", "b"}},
		{"empty lines ignored", []string{"\n\na\n\r\n\nb\n"}, false, []string{"a", "b"}},
		{"unterminated tail kept", []string{"a\nb"}, false, []string{"a"}},
		{"close flushes tail", []string{"a\nb"}, true, []string{"a", "b"}},
		{"close with empty tail", []string{"a\n"}, true, []string{"a"}},
		{"close with only carriage return", []string{"\r"}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var lines []string
			for _, e := range runAssembler(tt.writes, tt.close) {
				assert.False(t, e.truncated)
				lines = append(lines, e.line)
			}
			assert.Equal(t, tt.want, lines)
		})
	}
}

func TestAssemblerCloseFlushesOnce(t *testing.T) {
	t.Parallel()
	var out []emitted
	a := newAssembler(func(line []byte, truncated bool) { out = append(out, emitted{string(line), truncated}) })
	a.write([]byte("tail"))
	a.close()
	a.close()
	assert.Equal(t, []emitted{{"tail", false}}, out)
}

func TestAssemblerFlushesOversizeTail(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", MaxLineBytes+10)
	got := runAssembler([]string{big, "\nnext\n"}, false)
	assert.Len(t, got, 2)
	assert.True(t, got[0].truncated)
	assert.Len(t, got[0].line, MaxLineBytes)
	assert.Equal(t, emitted{"next", false}, got[1])
}

func TestAssemblerCapsOversizeLineInOneWrite(t *testing.T) {
	t.Parallel()
	got := runAssembler([]string{strings.Repeat("y", MaxLineBytes+5) + "\n"}, false)
	assert.Len(t, got, 1)
	assert.True(t, got[0].truncated)
	assert.Len(t, got[0].line, MaxLineBytes)
}
