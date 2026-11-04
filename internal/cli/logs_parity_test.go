package cli_test

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/weblog"
)

// The dashboard stream and `finfocus logs` must produce the same entry for the
// same line (SC-009). A raw line is the one legitimate difference: a file has
// no writer tag and no arrival time, so Decode stamps "unknown" and the read
// time where a live writer stamps its own name and the arrival time.
//
//nolint:paralleltest // root command and t.Setenv
func TestLogs_NDJSONMatchesHubEntries(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)

	result := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	got := ndjsonEntries(t, result.Stdout)

	f, err := os.Open(weblogSamplePath())
	require.NoError(t, err)
	defer f.Close()
	hub := weblog.NewHub(100)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		hub.Append(weblog.NewEntry(scanner.Bytes(), "", time.Now()))
	}
	require.NoError(t, scanner.Err())
	snapshot, _, cancel := hub.Subscribe(0)
	defer cancel()

	require.Len(t, got, len(snapshot.Entries))
	for i, want := range snapshot.Entries {
		if want.Raw {
			assert.True(t, got[i].Raw)
			assert.Equal(t, want.Message, got[i].Message)
			assert.Equal(t, want.Level, got[i].Level)
			continue
		}
		assert.Equal(t, want, got[i], "entry %d", i+1)
	}
}

// The default log file is zerolog.ConsoleWriter text, not JSON. `finfocus logs`
// must decode it, or level, source, and redaction all stop working on the file
// every ordinary run produces.
//
//nolint:paralleltest // root command and t.Setenv
func TestLogs_DecodesTheDefaultConsoleFormat(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "default.log")
	out, err := os.Create(file)
	require.NoError(t, err)
	logger := logging.NewLoggerWithWriter(logging.Config{Level: "info", Format: "text"}, out)
	logger.Info().Str("component", "cli").Str("command", "overview").Msg("command started")
	logger.Warn().
		Str("component", "engine").
		Str("plugin", "scorer").
		Str("api_token", "tok-leak-9f3a").
		Msg("no API key configured")
	logger.Error().Str("component", "registry").Str("operation", "plugin discovery").Msg("discovery failed")
	require.NoError(t, out.Close())

	all := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0")
	require.Equal(t, 0, all.ExitCode, "stderr: %s", all.Stderr)
	entries := ndjsonEntries(t, all.Stdout)
	require.Len(t, entries, 3)
	for _, e := range entries {
		assert.False(t, e.Raw, "%q was not decoded", e.Message)
		assert.NotContains(t, e.Message, "\x1b")
	}
	assert.NotContains(t, string(all.Stdout), "tok-leak-9f3a")

	warn := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0", "--level", "warn")
	require.Equal(t, 0, warn.ExitCode, "stderr: %s", warn.Stderr)
	levels := []string{}
	for _, e := range ndjsonEntries(t, warn.Stdout) {
		levels = append(levels, e.Level)
	}
	assert.Equal(t, []string{"warn", "error"}, levels, "--level must filter on the decoded level")

	registry := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0", "--source", "registry")
	require.Equal(t, 0, registry.ExitCode, "stderr: %s", registry.Stderr)
	picked := ndjsonEntries(t, registry.Stdout)
	require.Len(t, picked, 1)
	assert.Equal(t, "discovery failed", picked[0].Message)
	assert.Equal(t, "plugin discovery", picked[0].Fields["operation"])
}
