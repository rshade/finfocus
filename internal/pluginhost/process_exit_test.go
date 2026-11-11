package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
)

const bindGuidance = "pluginsdk v0.5.5"

// exitingPluginScript writes a fake plugin that records each start in a file,
// prints stderr, and exits with the given status before binding its port.
func exitingPluginScript(t *testing.T, stderr string, status int) (string, string) {
	t.Helper()
	starts := filepath.Join(t.TempDir(), "starts")
	script := createScript(t, fmt.Sprintf(`#!/bin/sh
echo start >> '%s'
cat >&2 <<'EOF'
%s
EOF
exit %d
`, starts, stderr, status), ".sh")
	return script, starts
}

func countStarts(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.Count(string(data), "start")
}

//nolint:paralleltest // writes and executes a script; a parallel fork inherits the write fd (text file busy)
func TestStartWithRetry_PluginExitsBeforeBind(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not supported on Windows")
	}

	script, starts := exitingPluginScript(
		t,
		`{"level":"error","plugin":"fake","token":"never-shown","error":"config: baseUrl must be an http(s) URL with a host","message":"config"}`,
		3,
	)

	var logs lockedBuffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
	ctx, cancel := context.WithTimeout(logger.WithContext(context.Background()), 30*time.Second)
	defer cancel()

	launcher := NewProcessLauncherWithRetries(5)
	begin := time.Now()
	conn, closeFn, err := launcher.StartWithRetry(ctx, script)
	elapsed := time.Since(begin)

	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Less(t, elapsed, stdoutPortFallback, "an exited plugin must not wait for the bind timeout")

	msg := err.Error()
	assert.Contains(t, msg, "plugin exited before binding to port")
	assert.Contains(t, msg, "exit status 3")
	assert.Contains(t, msg, "config: config: baseUrl must be an http(s) URL with a host")
	assert.NotContains(t, msg, "never-shown")
	assert.NotContains(t, msg, "failed to bind")
	assert.NotContains(t, msg, "--port")
	assert.NotContains(t, msg, "deadline")

	var exitErr *pluginExitError
	require.ErrorAs(t, err, &exitErr)
	assert.False(t, exitErr.portCollision)

	assert.Equal(t, 1, countStarts(t, starts), "a plugin that exits on its own is not retried")
	logged := string(logs.Snapshot())
	assert.Contains(t, logged, "plugin exited before binding to port")
	assert.NotContains(t, logged, bindGuidance, "the --port guidance is only for a real bind timeout")
}

//nolint:paralleltest // writes and executes a script; a parallel fork inherits the write fd (text file busy)
func TestStartWithRetry_PluginExitsOnPortCollisionIsRetried(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not supported on Windows")
	}

	script, starts := exitingPluginScript(t,
		"failed to listen: listen tcp 127.0.0.1:1: bind: address already in use", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _, err := NewProcessLauncherWithRetries(3).StartWithRetry(ctx, script)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.Contains(t, err.Error(), "exit status 1")
	assert.Contains(t, err.Error(), "address already in use")
	assert.Equal(t, 3, countStarts(t, starts))
}

func TestStderrTail_KeepsOnlyTheEnd(t *testing.T) {
	t.Parallel()

	var tail stderrTail
	n, err := tail.Write([]byte("short\n"))
	require.NoError(t, err)
	assert.Equal(t, len("short\n"), n)
	data, truncated := tail.snapshot()
	assert.Equal(t, "short\n", string(data))
	assert.False(t, truncated)

	big := strings.Repeat("a", stderrTailBytes) + "the end"
	n, err = tail.Write([]byte(big))
	require.NoError(t, err)
	assert.Equal(t, len(big), n)
	data, truncated = tail.snapshot()
	assert.Len(t, data, stderrTailBytes)
	assert.True(t, strings.HasSuffix(string(data), "the end"))
	assert.True(t, truncated)
}

func TestStderrExcerpt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		data      string
		truncated bool
		secrets   []string
		want      string
	}{
		{name: "empty", data: "", want: ""},
		{name: "blank lines", data: "\n  \n", want: ""},
		{
			name: "json keeps message and error only",
			data: `{"level":"error","api_key":"k-123456","error":"config: bad url","message":"config"}` + "\n",
			want: "config: config: bad url",
		},
		{name: "json message only", data: `{"level":"fatal","message":"boom"}`, want: "boom"},
		{name: "json error only", data: `{"level":"fatal","error":"boom"}`, want: "boom"},
		{name: "json without message or error is skipped", data: `{"level":"info","port":1}`, want: ""},
		{
			name: "last lines joined",
			data: "one\ntwo\nthree\nfour\n",
			want: "two | three | four",
		},
		{
			name:    "credential values masked",
			data:    "auth failed with hunter2-secret-value\n",
			secrets: []string{"hunter2-secret-value"},
			want:    "auth failed with [redacted]",
		},
		{
			name:    "credential masked inside json",
			data:    `{"error":"bad token tok-abcdef","message":"auth"}`,
			secrets: []string{"tok-abcdef"},
			want:    "auth: bad token [redacted]",
		},
		{
			name:    "short values are not masked",
			data:    "flag is true\n",
			secrets: []string{"true"},
			want:    "flag is true",
		},
		{
			name: "url userinfo removed",
			data: "dial https://admin:pa55w0rd@cost.example.com/api failed\n",
			want: "dial https://[redacted]@cost.example.com/api failed",
		},
		{
			name: "control characters removed",
			data: "\x1b[31mERROR\x1b[0m boom\a\r\n",
			want: "ERROR boom",
		},
		{
			name:      "partial first line dropped when truncated",
			data:      "et-tail-of-a-secret\nreal error\n",
			truncated: true,
			want:      "real error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, stderrExcerpt([]byte(tt.data), tt.truncated, tt.secrets))
		})
	}
}

func TestStderrExcerpt_IsBounded(t *testing.T) {
	t.Parallel()

	got := stderrExcerpt([]byte(strings.Repeat("é", 2000)+" last words\n"), false, nil)
	assert.LessOrEqual(t, len(got), maxStderrExcerptBytes)
	assert.True(t, strings.HasPrefix(got, "..."))
	assert.True(t, strings.HasSuffix(got, "last words"))
	assert.Equal(t, strings.ToValidUTF8(got, "?"), got, "excerpt must stay valid UTF-8")
}

func TestPluginExitError_PortCollision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "exit after a port collision",
			err:  &pluginExitError{status: "exit status 1", portCollision: true},
			want: true,
		},
		{
			name: "exit for another reason mentioning a bind failure",
			err:  &pluginExitError{status: "exit status 1", stderr: "failed to bind to port 9"},
			want: false,
		},
		{
			name: "wrapped exit error",
			err:  fmt.Errorf("launch: %w", &pluginExitError{status: "exit status 1"}),
			want: false,
		},
		{name: "real bind timeout", err: errors.New("plugin failed to bind to port: timeout"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isPortCollisionError(tt.err))
		})
	}
}

func TestCredentialValues(t *testing.T) {
	t.Parallel()

	got := credentialValues([]string{
		"PATH=/usr/bin",
		"AWS_SECRET_ACCESS_KEY=aws-secret-value",
		"KUBECOST_BASE_URL=http://cost.local",
		"FROM_ESC=esc-delivered-value-long",
		"EMPTY_TOKEN=",
		"malformed",
	}, map[string]esc.Ref{"FROM_ESC": {}})

	assert.Equal(t, []string{"esc-delivered-value-long", "aws-secret-value"}, got)
}

func TestPluginExitError_Message(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"plugin exited before binding to port (exit status 2): config: bad url",
		(&pluginExitError{status: "exit status 2", stderr: "config: bad url"}).Error())
	assert.Equal(t,
		"plugin exited before binding to port (exit status 0) with no error output",
		(&pluginExitError{status: "exit status 0"}).Error())
}
