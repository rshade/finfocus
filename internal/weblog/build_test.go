package weblog_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

//nolint:gochecknoglobals // Immutable shared test clock.
var fixedNow = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func TestNewEntryLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
		want string
	}{
		{"trace", `{"level":"trace","message":"m"}`, "trace"},
		{"debug", `{"level":"debug","message":"m"}`, "debug"},
		{"info", `{"level":"info","message":"m"}`, "info"},
		{"warn", `{"level":"warn","message":"m"}`, "warn"},
		{"warning alias", `{"level":"warning","message":"m"}`, "warn"},
		{"error", `{"level":"error","message":"m"}`, "error"},
		{"fatal", `{"level":"fatal","message":"m"}`, "fatal"},
		{"panic", `{"level":"panic","message":"m"}`, "panic"},
		{"unknown", `{"level":"verbose","message":"m"}`, "info"},
		{"missing", `{"message":"m"}`, "info"},
		{"non-string", `{"level":3,"message":"m"}`, "info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := weblog.NewEntry([]byte(tt.line), "", fixedNow)
			assert.Equal(t, tt.want, e.Level)
		})
	}
}

func TestNewEntryTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
		want string
	}{
		{"rfc3339", `{"time":"2026-01-02T03:04:05Z","message":"m"}`, "2026-01-02T03:04:05Z"},
		{"rfc3339nano", `{"time":"2026-01-02T03:04:05.123456789Z","message":"m"}`, "2026-01-02T03:04:05.123456789Z"},
		{"offset converted to UTC", `{"time":"2026-01-02T03:04:05+02:00","message":"m"}`, "2026-01-02T01:04:05Z"},
		{"invalid falls back", `{"time":"yesterday","message":"m"}`, "2026-11-02T12:00:00Z"},
		{"missing falls back", `{"message":"m"}`, "2026-11-02T12:00:00Z"},
		{"raw falls back", `not json`, "2026-11-02T12:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := weblog.NewEntry([]byte(tt.line), "", fixedNow)
			assert.Equal(t, tt.want, e.Time)
		})
	}
}

func TestNewEntrySource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		line     string
		tag      string
		want     string
		wantKind string
	}{
		{"plugin_name wins", `{"plugin_name":"aws","component":"c"}`, "tagged", "aws", weblog.KindPlugin},
		{"tag over component", `{"component":"c"}`, "tagged", "tagged", weblog.KindPlugin},
		{"component", `{"component":"overview"}`, "", "overview", weblog.KindCore},
		{"core default", `{"message":"m"}`, "", "core", weblog.KindCore},
		{"raw with tag", `plain`, "tagged", "tagged", weblog.KindPlugin},
		{"raw without tag", `plain`, "", "core", weblog.KindCore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := weblog.NewEntry([]byte(tt.line), tt.tag, fixedNow)
			assert.Equal(t, tt.want, e.Source)
			assert.Equal(t, tt.wantKind, e.SourceKind)
		})
	}
}

func TestNewEntryFieldsAndMessage(t *testing.T) {
	t.Parallel()
	e := weblog.NewEntry(
		[]byte(`{"level":"info","time":"2026-01-02T03:04:05Z","message":"hello","region":"us","n":2}`),
		"", fixedNow)
	assert.Equal(t, "hello", e.Message)
	assert.False(t, e.Raw)
	assert.Equal(t, map[string]any{"region": "us", "n": float64(2)}, e.Fields)
	assert.NotContains(t, e.Fields, "time")
	assert.NotContains(t, e.Fields, "level")
	assert.NotContains(t, e.Fields, "message")
}

func TestNewEntryOmitsEmptyFields(t *testing.T) {
	t.Parallel()
	e := weblog.NewEntry([]byte(`{"message":"only"}`), "", fixedNow)
	assert.Nil(t, e.Fields)
}

func TestNewEntryRaw(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"plain text", `[1,2]`, `null`, `42`} {
		e := weblog.NewEntry([]byte(line), "", fixedNow)
		assert.True(t, e.Raw, line)
		assert.Equal(t, "info", e.Level, line)
		assert.Equal(t, line, e.Message, line)
		assert.Nil(t, e.Fields, line)
	}
}

func TestNewEntryRawObjectLookalikeIsWithheld(t *testing.T) {
	t.Parallel()
	for _, line := range []string{`{"a":1} trailing`, `{"broken":`} {
		e := weblog.NewEntry([]byte(line), "", fixedNow)
		assert.True(t, e.Raw, line)
		assert.Contains(t, e.Message, "withheld", line)
		assert.NotContains(t, e.Message, "broken", line)
		assert.Nil(t, e.Fields, line)
	}
}

func TestNewEntryTruncatesLongMessage(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", weblog.MaxLineBytes+100)
	e := weblog.NewEntry([]byte(`{"message":"`+long+`"}`), "", fixedNow)
	assert.True(t, e.Truncated)
	assert.Len(t, e.Message, weblog.MaxLineBytes)

	raw := weblog.NewEntry([]byte(long), "", fixedNow)
	assert.True(t, raw.Raw)
	assert.True(t, raw.Truncated)
	assert.Len(t, raw.Message, weblog.MaxLineBytes)
}

func TestNewEntryTruncationKeepsValidUTF8(t *testing.T) {
	t.Parallel()
	msg := strings.Repeat("a", weblog.MaxLineBytes-1) + "é" + "tail"
	e := weblog.NewEntry([]byte(msg), "", fixedNow)
	require.True(t, e.Truncated)
	assert.Len(t, e.Message, weblog.MaxLineBytes-1)
}

func TestNewEntryAtLimitIsNotTruncated(t *testing.T) {
	t.Parallel()
	e := weblog.NewEntry([]byte(strings.Repeat("a", weblog.MaxLineBytes)), "", fixedNow)
	assert.False(t, e.Truncated)
}

func TestNewEntryDisplay(t *testing.T) {
	t.Parallel()
	e := weblog.NewEntry(
		[]byte(`{"level":"warn","plugin_name":"aws","time":"2026-01-02T03:04:05.250Z","message":"m"}`),
		"",
		fixedNow,
	)
	assert.Equal(t, weblog.Display(e), e.Display)
	assert.Equal(t, "WARN ", e.Display.Level)
	assert.Equal(t, "aws", e.Display.Source)
}
