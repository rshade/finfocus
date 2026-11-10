package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/weblog"
)

func testSourceDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func weblogSamplePath() string {
	return filepath.Join(testSourceDir(), "..", "weblog", "testdata", "sample.ndjson")
}

func runLogs(t *testing.T, args ...string) axtest.Result {
	t.Helper()
	return axtest.Run(context.Background(), t, cli.NewRootCmd("test-version"), append([]string{"logs"}, args...))
}

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_LOG_FILE", "")
	t.Chdir(t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	return home
}

func copySample(t *testing.T, dst string) {
	t.Helper()
	data, err := os.ReadFile(weblogSamplePath())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	require.NoError(t, os.WriteFile(dst, data, 0o600))
}

func ndjsonEntries(t *testing.T, out []byte) []weblog.Entry {
	t.Helper()
	var entries []weblog.Entry
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var e weblog.Entry
		require.NoError(t, json.Unmarshal([]byte(line), &e), line)
		entries = append(entries, e)
	}
	return entries
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_DefaultFileIsUsed(t *testing.T) {
	home := isolatedHome(t)
	copySample(t, filepath.Join(home, "logs", "finfocus.log"))

	result := runLogs(t, "--output", "ndjson", "--tail", "0")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, ndjsonEntries(t, result.Stdout), 8)
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_MissingDefaultFileExitsOneWithPath(t *testing.T) {
	home := isolatedHome(t)

	result := runLogs(t)
	assert.Equal(t, ax.ExitInternal, result.ExitCode)
	assert.Contains(t, string(result.Stderr), envelopeText(t, filepath.Join(home, "logs", "finfocus.log")))
	_, err := os.Stat(filepath.Join(home, "logs", "finfocus.log"))
	assert.True(t, os.IsNotExist(err), "the command must not create the log file")
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_MissingExplicitFileExitsValidation(t *testing.T) {
	isolatedHome(t)
	missing := filepath.Join(t.TempDir(), "nope.log")

	result := runLogs(t, "--file", missing)
	assert.Equal(t, ax.ExitValidation, result.ExitCode)
	assert.Contains(t, string(result.Stderr), envelopeText(t, missing))
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_BadFlagValuesExitValidation(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "f.log")
	copySample(t, file)

	for _, args := range [][]string{
		{"--level", "loud"},
		{"--tail", "-1"},
		{"--tail", "many"},
		{"--output", "xml"},
		{"--bogus"},
	} {
		result := runLogs(t, append([]string{"--file", file}, args...)...)
		assert.Equal(t, ax.ExitValidation, result.ExitCode, "%v: %s", args, result.Stderr)
	}
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_UnknownOutputIsRejectedBeforeReading(t *testing.T) {
	isolatedHome(t)

	result := runLogs(t, "--output", "xml")
	assert.Equal(t, ax.ExitValidation, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "unsupported output format: xml")
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_TailDefaultsToTwoHundredAndZeroMeansAll(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "big.log")
	var buf bytes.Buffer
	for i := 1; i <= 250; i++ {
		fmt.Fprintf(&buf, `{"level":"info","time":"2026-11-02T10:00:00Z","message":"line %d"}`+"\n", i)
	}
	require.NoError(t, os.WriteFile(file, buf.Bytes(), 0o600))

	def := ndjsonEntries(t, runLogs(t, "--file", file, "--output", "ndjson").Stdout)
	require.Len(t, def, 200)
	assert.Equal(t, "line 51", def[0].Message)
	assert.Equal(t, "line 250", def[199].Message)

	all := ndjsonEntries(t, runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0").Stdout)
	assert.Len(t, all, 250)

	three := ndjsonEntries(t, runLogs(t, "--file", file, "--output", "ndjson", "--tail", "3").Stdout)
	require.Len(t, three, 3)
	assert.Equal(t, "line 248", three[0].Message, "oldest first")
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_TailFilterReachesOlderMatches(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "filtered.log")
	var buf bytes.Buffer
	fmt.Fprintln(&buf, `{"level":"error","message":"the error"}`)
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&buf, "{\"level\":\"info\",\"message\":\"later %d\"}\n", i)
	}
	require.NoError(t, os.WriteFile(file, buf.Bytes(), 0o600))

	got := ndjsonEntries(t, runLogs(t, "--file", file, "--output", "ndjson", "--level", "error", "--tail", "1").Stdout)
	require.Len(t, got, 1)
	assert.Equal(t, "the error", got[0].Message)
	assert.Equal(t, "error", got[0].Level)
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_FiltersCombine(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)
	base := []string{"--file", file, "--output", "ndjson", "--tail", "0"}

	both := ndjsonEntries(t, runLogs(t, append(base, "--source", "aws-public", "--source", "kubernetes")...).Stdout)
	require.Len(t, both, 2)
	assert.Equal(t, "aws-public", both[0].Source)
	assert.Equal(t, "kubernetes", both[1].Source)

	warn := ndjsonEntries(t, runLogs(t, append(base, "--level", "warn")...).Stdout)
	for _, e := range warn {
		assert.Contains(t, []string{"warn", "error"}, e.Level)
	}
	assert.NotEmpty(t, warn)

	grep := ndjsonEntries(t, runLogs(t, append(base, "--grep", "US-EAST-1")...).Stdout)
	require.Len(t, grep, 1)
	assert.Equal(t, "pricing lookup", grep[0].Message)

	none := runLogs(t, append(base, "--source", "kubernetes", "--grep", "pricing")...)
	assert.Equal(t, 0, none.ExitCode)
	assert.Empty(t, ndjsonEntries(t, none.Stdout))
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_JSONOutputIsAnArray(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)

	result := runLogs(t, "--file", file, "--output", "json", "--tail", "0")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	var entries []weblog.Entry
	require.NoError(t, json.Unmarshal(result.Stdout, &entries))
	assert.Len(t, entries, 8)

	empty := runLogs(t, "--file", file, "--output", "json", "--grep", "zzz-no-match")
	var none []weblog.Entry
	require.NoError(t, json.Unmarshal(empty.Stdout, &none))
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_CredentialValuesAreRedactedInEveryFormat(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)

	for _, format := range []string{"table", "json", "ndjson"} {
		result := runLogs(t, "--file", file, "--output", format, "--tail", "0")
		require.Equal(t, 0, result.ExitCode, "%s: %s", format, result.Stderr)
		out := string(result.Stdout)
		assert.NotContains(t, out, "abc123", format)
		assert.NotContains(t, out, "hunter2", format)
		assert.Contains(t, out, "[redacted]", format)
	}
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_LeavesTheFileUntouched(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)
	before, err := os.ReadFile(file)
	require.NoError(t, err)
	infoBefore, err := os.Stat(file)
	require.NoError(t, err)

	result := runLogs(t, "--file", file, "--tail", "0")
	require.Equal(t, 0, result.ExitCode)

	after, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	infoAfter, err := os.Stat(file)
	require.NoError(t, err)
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime())
}

//nolint:paralleltest // root command, t.Setenv, and executes a script
func TestLogs_StartsNoPlugin(t *testing.T) {
	home := isolatedHome(t)
	marker := filepath.Join(t.TempDir(), "started")
	pluginDir := filepath.Join(home, "plugins", "probe", "1.0.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	script := "#!/bin/sh\ntouch " + marker + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "finfocus-plugin-probe"), []byte(script), 0o755))
	copySample(t, filepath.Join(home, "logs", "finfocus.log"))

	result := runLogs(t)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	_, err := os.Stat(marker)
	assert.True(t, os.IsNotExist(err), "logs must not launch plugins")
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_LevelWarningIsAnAliasForWarn(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)

	alias := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0", "--level", "Warning")
	require.Equal(t, 0, alias.ExitCode, "stderr: %s", alias.Stderr)
	want := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0", "--level", "warn")
	assert.Equal(t, string(want.Stdout), string(alias.Stdout))
	assert.NotEmpty(t, ndjsonEntries(t, alias.Stdout))
}

// Not parallel: t.Setenv and the root command.
func TestLogs_DefaultPathFollowsConfiguredLogFile(t *testing.T) {
	isolatedHome(t)
	custom := filepath.Join(t.TempDir(), "custom.log")
	copySample(t, custom)
	t.Setenv("FINFOCUS_LOG_FILE", custom)

	result := runLogs(t, "--output", "ndjson", "--tail", "0")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, ndjsonEntries(t, result.Stdout), 8)
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_DirectoryInsteadOfFileFailsWhileReading(t *testing.T) {
	isolatedHome(t)
	dir := t.TempDir()

	for _, args := range [][]string{
		{"--tail", "0"},
		{"--tail", "5"},
		{"--tail", "0", "--output", "json"},
	} {
		result := runLogs(t, append([]string{"--file", dir}, args...)...)
		assert.Equal(t, ax.ExitInternal, result.ExitCode, "%v: %s", args, result.Stderr)
		assert.Contains(t, string(result.Stderr), "reading log file", "%v", args)
	}
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_UnreadableFileExitCodes(t *testing.T) {
	home := isolatedHome(t)
	explicit := filepath.Join(t.TempDir(), "locked.log")
	copySample(t, explicit)
	require.NoError(t, os.Chmod(explicit, 0))
	if f, err := os.Open(explicit); err == nil {
		_ = f.Close()
		t.Skip("permission bits are not enforced for this user (root?)")
	}

	result := runLogs(t, "--file", explicit)
	assert.Equal(t, ax.ExitValidation, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Contains(t, string(result.Stderr), "cannot read log file")

	defaultPath := filepath.Join(home, "logs", "finfocus.log")
	copySample(t, defaultPath)
	require.NoError(t, os.Chmod(defaultPath, 0))
	result = runLogs(t)
	assert.Equal(t, ax.ExitInternal, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Contains(t, string(result.Stderr), "cannot read log file")
}

//nolint:paralleltest // root command and t.Setenv
func TestLogs_TableFieldValueFormatting(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "f.log")
	line := `{"level":"info","time":"2026-11-02T10:00:00Z","message":"m",` +
		`"spaced":"a b","empty":"","quoted":"x=y","plain":"ok","count":3,"ratio":1.5,` +
		`"flag":true,"nothing":null,"list":[1,"two"],"obj":{"k":"v"}}` + "\n"
	require.NoError(t, os.WriteFile(file, []byte(line), 0o600))

	result := runLogs(t, "--file", file)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	out := strings.TrimSpace(string(result.Stdout))
	want := ` m count=3 empty="" flag=true list=[1,"two"] nothing=null obj={"k":"v"}` +
		` plain=ok quoted="x=y" ratio=1.5 spaced="a b"`
	assert.True(t, strings.HasSuffix(out, want), out)
}
