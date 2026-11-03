package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingWriter struct {
	writes [][]byte
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes = append(w.writes, append([]byte(nil), p...))
	return len(p), nil
}

func decodeJSONLines(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var events []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var event map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &event), "line %q must be valid JSON", line)
		events = append(events, event)
	}
	return events
}

func TestTee_ConsoleDestinationTeeGetsRawJSON(t *testing.T) {
	t.Parallel()

	var dest, tee bytes.Buffer
	logger := newLoggerWithWriter(Config{Format: "console", Level: "info", Tee: &tee}, &dest)

	logger.Info().Str("plugin_name", "aws-public").Msg("plugin started")

	destText := dest.String()
	assert.NotEmpty(t, destText)
	assert.False(t, strings.HasPrefix(destText, "{"), "destination must receive console text, got %q", destText)
	assert.Contains(t, destText, "plugin started")

	events := decodeJSONLines(t, tee.Bytes())
	require.Len(t, events, 1)
	assert.Equal(t, "info", events[0]["level"])
	assert.Equal(t, "plugin started", events[0]["message"])
	assert.Equal(t, "aws-public", events[0]["plugin_name"])
}

func TestTee_JSONFormatBothReceiveJSON(t *testing.T) {
	t.Parallel()

	var dest, tee bytes.Buffer
	logger := newLoggerWithWriter(Config{Format: "json", Level: "info", Tee: &tee}, &dest)

	logger.Warn().Msg("disk almost full")

	destEvents := decodeJSONLines(t, dest.Bytes())
	teeEvents := decodeJSONLines(t, tee.Bytes())
	require.Len(t, destEvents, 1)
	require.Len(t, teeEvents, 1)
	assert.Equal(t, "warn", teeEvents[0]["level"])
	assert.Equal(t, "disk almost full", teeEvents[0]["message"])
	assert.Equal(t, destEvents[0]["message"], teeEvents[0]["message"])
}

func TestTee_LevelFilteringApplies(t *testing.T) {
	t.Parallel()

	var dest, tee bytes.Buffer
	logger := newLoggerWithWriter(Config{Format: "json", Level: "info", Tee: &tee}, &dest)

	logger.Debug().Msg("debug detail")
	logger.Info().Msg("info detail")

	assert.NotContains(t, dest.String(), "debug detail")
	assert.NotContains(t, tee.String(), "debug detail")
	teeEvents := decodeJSONLines(t, tee.Bytes())
	require.Len(t, teeEvents, 1)
	assert.Equal(t, "info detail", teeEvents[0]["message"])
}

func TestTee_NilTeeLeavesOutputUnchanged(t *testing.T) {
	t.Parallel()

	var dest bytes.Buffer
	logger := newLoggerWithWriter(Config{Format: "json", Level: "info"}, &dest)

	logger.Info().Str("component", "core").Msg("no tee configured")

	events := decodeJSONLines(t, dest.Bytes())
	require.Len(t, events, 1)
	assert.Equal(t, "no tee configured", events[0]["message"])
	assert.Equal(t, "core", events[0]["component"])
	assert.Equal(t, 1, strings.Count(dest.String(), "\n"))
}

func TestTee_OneWritePerEvent(t *testing.T) {
	t.Parallel()

	var dest bytes.Buffer
	tee := &countingWriter{}
	logger := newLoggerWithWriter(Config{Format: "console", Level: "info", Tee: tee}, &dest)

	logger.Info().Msg("first")
	logger.Error().Msg("second")

	require.Len(t, tee.writes, 2)
	for i, want := range []string{"first", "second"} {
		var event map[string]any
		require.NoError(t, json.Unmarshal(bytes.TrimSpace(tee.writes[i]), &event))
		assert.Equal(t, want, event["message"])
	}
}

func TestPluginLogTap_RoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tap := func(_ string) io.WriteCloser {
		return nopWriteCloser{Writer: &buf}
	}
	ctx := ContextWithPluginLogTap(context.Background(), tap)

	got := PluginLogTapFromContext(ctx)
	require.NotNil(t, got)
	w := got("aws-public")
	require.NotNil(t, w)
	_, err := io.WriteString(w, "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", buf.String())
}

func TestPluginLogTap_NilOnBareContext(t *testing.T) {
	t.Parallel()

	assert.Nil(t, PluginLogTapFromContext(context.Background()))
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}
