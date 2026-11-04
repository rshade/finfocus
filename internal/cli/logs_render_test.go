package cli_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ansiEscape = "\x1b["
)

// clockPattern matches the table's HH:MM:SS.mmm column, which is rendered in the
// machine's local zone. Goldens mask it so no test has to reassign [time.Local],
// a process-wide variable that background goroutines read.
var clockPattern = regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d{3}`)

func maskClock(s string) string {
	return clockPattern.ReplaceAllString(s, "HH:MM:SS.mmm")
}

func renderLogs(t *testing.T, extra ...string) string {
	t.Helper()
	isolatedHome(t)

	fixture := filepath.Join(testSourceDir(), "testdata", "logs_render.ndjson")
	result := runLogs(t, append([]string{"--file", fixture, "--tail", "0"}, extra...)...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	return string(result.Stdout)
}

//nolint:paralleltest // root command and t.Setenv
func TestLogsRender_PlainTableGolden(t *testing.T) {
	out := renderLogs(t)

	assert.NotContains(t, out, ansiEscape)
	for _, label := range []string{"TRACE", "DEBUG", "INFO ", "WARN ", "ERROR", "FATAL"} {
		assert.Contains(t, out, label, "level must be readable from the text badge alone")
	}
	assert.Contains(t, out, "aws-public")
	assert.Contains(t, out, "resources=12")
	assertGoldenFile(t, filepath.Join(testSourceDir(), "testdata", "logs_table_nocolor.golden"), maskClock(out))
}

//nolint:paralleltest // root command and t.Setenv
func TestLogsRender_StyledTableGolden(t *testing.T) {
	out := renderLogs(t, "--color")

	assert.Contains(t, out, ansiEscape)
	assert.Equal(t, plainText(t, renderLogs(t)), plainText(t, out),
		"styling adds escapes only; the text is the plain table")
	assertGoldenFile(t, filepath.Join(testSourceDir(), "testdata", "logs_table.golden"), maskClock(out))
}

// Not parallel: t.Setenv and the root command.
func TestLogsRender_NoColorWins(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "1")
	assert.NotContains(t, renderLogs(t), ansiEscape)

	assert.NotContains(t, renderLogs(t, "--no-color"), ansiEscape)
	assert.NotContains(t, renderLogs(t, "--plain"), ansiEscape)
}

//nolint:paralleltest // root command and t.Setenv
func TestLogsRender_NotATerminalIsPlain(t *testing.T) {
	assert.NotContains(t, renderLogs(t), ansiEscape)
}

func plainText(t *testing.T, s string) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		for i < len(s) && s[i] != 'm' {
			i++
		}
	}
	return b.String()
}
