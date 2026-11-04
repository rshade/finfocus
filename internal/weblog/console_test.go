package weblog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

// consoleLine renders one event through zerolog.ConsoleWriter configured the
// way the default logger writes its file: colour on, RFC3339 timestamps.
func consoleLine(t *testing.T, emit func(logger zerolog.Logger)) string {
	t.Helper()
	var buf bytes.Buffer
	logger := zerolog.New(zerolog.ConsoleWriter{Out: &buf, TimeFormat: time.RFC3339, NoColor: false}).
		Level(zerolog.TraceLevel).With().Timestamp().Logger()
	emit(logger)
	line := strings.TrimRight(buf.String(), "\n")
	require.NotEmpty(t, line)
	require.Contains(t, line, "\x1b[", "the default logger writes colour escapes")
	return line
}

func TestConsoleLineIsDecodedLikeItsJSONForm(t *testing.T) {
	t.Parallel()
	line := consoleLine(t, func(l zerolog.Logger) {
		l.Info().Str("component", "cli").Str("command", "finfocus").Str("trace_id", "abc123").Msg("command started")
	})

	e := weblog.NewEntry([]byte(line), "", fixedNow)

	assert.False(t, e.Raw)
	assert.Equal(t, "info", e.Level)
	assert.Equal(t, "command started", e.Message)
	assert.Equal(t, "cli", e.Source)
	assert.Equal(t, weblog.KindCore, e.SourceKind)
	assert.Equal(t, map[string]any{"command": "finfocus", "component": "cli", "trace_id": "abc123"}, e.Fields)
	assert.NotContains(t, e.Message, "\x1b")
	_, err := time.Parse(time.RFC3339Nano, e.Time)
	require.NoError(t, err)
	assert.NotEqual(t, fixedNow.Format(time.RFC3339Nano), e.Time, "the event's own time is used")
}

func TestConsoleLineLevels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level zerolog.Level
		want  string
	}{
		{zerolog.TraceLevel, "trace"},
		{zerolog.DebugLevel, "debug"},
		{zerolog.InfoLevel, "info"},
		{zerolog.WarnLevel, "warn"},
		{zerolog.ErrorLevel, "error"},
		{zerolog.FatalLevel, "fatal"},
		{zerolog.PanicLevel, "panic"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			line := consoleLine(t, func(l zerolog.Logger) { l.WithLevel(tt.level).Str("component", "engine").Msg("m") })
			e := weblog.NewEntry([]byte(line), "", fixedNow)
			assert.False(t, e.Raw)
			assert.Equal(t, tt.want, e.Level)
			assert.Equal(t, "m", e.Message)
		})
	}
}

func emitPluginName(l zerolog.Logger) { l.Warn().Str("plugin_name", "aws-public").Msg("m") }
func emitBare(l zerolog.Logger)       { l.Warn().Msg("m") }
func emitComponent(l zerolog.Logger)  { l.Warn().Str("component", "registry").Msg("m") }

func TestConsoleLineSourceFollowsTheJSONRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		emit       func(l zerolog.Logger)
		tag        string
		wantSource string
		wantKind   string
	}{
		{"plugin_name field", emitPluginName, "", "aws-public", weblog.KindPlugin},
		{"tag", emitBare, "scorer", "scorer", weblog.KindPlugin},
		{"component", emitComponent, "", "registry", weblog.KindCore},
		{"nothing", emitBare, "", "core", weblog.KindCore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := weblog.NewEntry([]byte(consoleLine(t, tt.emit)), tt.tag, fixedNow)
			assert.Equal(t, tt.wantSource, e.Source)
			assert.Equal(t, tt.wantKind, e.SourceKind)
		})
	}
}

func TestConsoleLineFieldValues(t *testing.T) {
	t.Parallel()
	line := consoleLine(t, func(l zerolog.Logger) {
		l.Error().
			Err(errors.New("rpc failed: code = Unavailable")).
			Str("operation", "stack export").
			Str("quote", `say "hi"`).
			Int("resources", 12).
			Msg("could not price resource")
	})

	e := weblog.NewEntry([]byte(line), "", fixedNow)

	assert.Equal(t, "could not price resource", e.Message)
	assert.Equal(t, "error", e.Level)
	assert.Equal(t, "rpc failed: code = Unavailable", e.Fields["error"])
	assert.Equal(t, "stack export", e.Fields["operation"])
	assert.Equal(t, `say "hi"`, e.Fields["quote"])
	assert.Equal(t, "12", e.Fields["resources"])
}

func TestConsoleLineCredentialFieldsAreRedacted(t *testing.T) {
	t.Parallel()
	line := consoleLine(t, func(l zerolog.Logger) {
		l.Info().Str("api_token", leakedToken).Str("password", leakedToken).Str("region", "us-east-1").Msg("configured")
	})

	e := weblog.NewEntry([]byte(line), "", fixedNow)

	assert.False(t, e.Raw)
	assert.Equal(t, "[redacted]", e.Fields["api_token"])
	assert.Equal(t, "[redacted]", e.Fields["password"])
	assert.Equal(t, "us-east-1", e.Fields["region"])
	encoded, err := json.Marshal(e)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), leakedToken)
}

func TestConsoleLineWithoutFieldsKeepsTheWholeMessage(t *testing.T) {
	t.Parallel()
	line := consoleLine(t, func(l zerolog.Logger) { l.Info().Msg("plugin discovery complete in 3 steps") })

	e := weblog.NewEntry([]byte(line), "", fixedNow)

	assert.Equal(t, "plugin discovery complete in 3 steps", e.Message)
	assert.Nil(t, e.Fields)
}

func TestCutConsoleLineNeverLeaksACredentialIntoTheMessage(t *testing.T) {
	t.Parallel()
	full := consoleLine(t, func(l zerolog.Logger) {
		l.Info().Str("component", "cli").Str("api_token", leakedToken).Msg("configured")
	})
	start := strings.Index(full, "api_token")
	require.Positive(t, start)

	for cut := start; cut <= len(full); cut++ {
		e := weblog.NewEntry([]byte(full[:cut]), "", fixedNow)
		encoded, err := json.Marshal(e)
		require.NoError(t, err)
		leaked := strings.Contains(string(encoded), leakedToken[:6])
		assert.False(t, leaked, "cut at %d leaked: %q", cut, full[:cut])
	}
}

func TestNonConsoleLinesStayRaw(t *testing.T) {
	t.Parallel()
	tests := []string{
		"PORT=36677",
		"2026-11-03T07:00:02-06:00 ??? something",
		"not-a-time INF something",
		"2026-11-03T07:00:02-06:00",
		"INF something",
	}
	for _, line := range tests {
		t.Run(line, func(t *testing.T) {
			t.Parallel()
			e := weblog.NewEntry([]byte(line), "", fixedNow)
			assert.True(t, e.Raw)
			assert.Equal(t, line, e.Message)
		})
	}
}

func TestDecodeReadsAFileWrittenByTheDefaultLogger(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(zerolog.ConsoleWriter{Out: &buf, TimeFormat: time.RFC3339, NoColor: false}).
		With().Timestamp().Logger()
	logger.Info().Str("component", "cli").Msg("command started")
	buf.WriteString("PORT=41499\n")
	logger.Warn().Str("component", "engine").Str("plugin", "scorer").Msg("no API key configured")

	var entries []weblog.Entry
	for e, err := range weblog.Decode(&buf, "") {
		require.NoError(t, err)
		entries = append(entries, e)
	}

	require.Len(t, entries, 3)
	var sources, levels []string
	for _, e := range entries {
		sources = append(sources, e.Source)
		levels = append(levels, e.Level)
	}
	assert.Equal(t, []string{"cli", "unknown", "engine"}, sources)
	assert.Equal(t, []string{"info", "info", "warn"}, levels)
	assert.False(t, entries[0].Raw)
	assert.True(t, entries[1].Raw)
	assert.False(t, entries[2].Raw)
}
