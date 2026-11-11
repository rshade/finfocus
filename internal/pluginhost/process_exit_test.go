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

	script, starts := exitingPluginScript(
		t,
		`{"level":"fatal","error":"failed to listen: listen tcp 127.0.0.1:1: bind: address already in use","message":"serve"}`,
		1,
	)

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
			name: "last structured lines joined",
			data: `{"message":"one"}` + "\n" + `{"message":"two"}` + "\n" +
				`{"message":"three"}` + "\n" + `{"message":"four"}` + "\n",
			want: "two | three | four",
		},
		{
			name: "free-form lines are never shown",
			data: "flag provided but not defined: -port\npanic: token=hunter2-secret-value\n",
			want: "",
		},
		{
			name: "free-form line with an encoded secret is never shown",
			// base64 of "hunter2-secret-value", which masking cannot recognize.
			data:    "auth header Basic aHVudGVyMi1zZWNyZXQtdmFsdWU=\n",
			secrets: []string{"hunter2-secret-value"},
			want:    "",
		},
		{
			name:    "free-form lines skipped around a structured line",
			data:    "raw secret-ish text\n" + `{"error":"boom","message":"serve"}` + "\nmore raw text\n",
			secrets: nil,
			want:    "serve: boom",
		},
		{
			name: "json that does not parse is not shown",
			data: `{"error":"cut off line with secret abc` + "\n",
			want: "",
		},
		{
			name:    "credential masked inside json",
			data:    `{"error":"bad token tok-abcdef","message":"auth"}`,
			secrets: []string{"tok-abcdef"},
			want:    "auth: bad token [redacted]",
		},
		{
			name:    "short credentials are masked",
			data:    `{"error":"login rejected password 12345"}`,
			secrets: []string{"12345"},
			want:    "login rejected password [redacted]",
		},
		{
			name:    "secret with json escapes masked in decoded form",
			data:    `{"error":"bad key a\"b\\c-secret"}`,
			secrets: []string{`a"b\c-secret`},
			want:    "bad key [redacted]",
		},
		{
			name: "url userinfo removed",
			data: `{"error":"dial https://admin:pa55w0rd@cost.example.com/api failed"}`,
			want: "dial https://[redacted]@cost.example.com/api failed",
		},
		{
			name: "url userinfo with @ in the password removed whole",
			data: `{"error":"dial https://admin:first@second@cost.example.com/api failed"}`,
			want: "dial https://[redacted]@cost.example.com/api failed",
		},
		{
			name: "ansi-prefixed json is still reduced to its fields",
			data: "\x1b[31m" + `{"api_key":"field-secret-value","message":"startup failed"}` + "\x1b[0m\n",
			want: "startup failed",
		},
		{
			name: "incomplete json event is withheld",
			data: `{"api_key":"field-secret-value","message":"startup failed"` + "\n",
			want: "",
		},
		{
			name: "control characters removed",
			data: `{"message":"\u001b[31mERROR\u001b[0m boom\u0007\r"}`,
			want: "ERROR boom",
		},
		{
			name:      "partial first line dropped when truncated",
			data:      `"error":"et-tail-of-a-secret"}` + "\n" + `{"error":"real error"}` + "\n",
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

	line := `{"error":"` + strings.Repeat("é", 2000) + ` last words"}` + "\n"
	got := stderrExcerpt([]byte(line), false, nil)
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
	assert.Equal(
		t,
		"plugin exited before binding to port (exit status 2); its stderr is not a structured log, see the FinFocus log file",
		(&pluginExitError{status: "exit status 2", hadOutput: true}).Error(),
	)
}

//nolint:paralleltest // writes and executes a script; a parallel fork inherits the write fd (text file busy)
func TestStartWithRetry_PluginExitsWithFreeFormStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not supported on Windows")
	}

	// A secret under a name core cannot recognize, raw and base64-encoded.
	script, starts := exitingPluginScript(t,
		"MY_DB_PASS=opaque-value-123\nauth Basic b3BhcXVlLXZhbHVlLTEyMw==\naddress already in use", 4)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _, err := NewProcessLauncherWithRetries(1).StartWithRetry(ctx, script)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "opaque-value-123")
	assert.NotContains(t, err.Error(), "b3BhcXVlLXZhbHVlLTEyMw==")
	assert.Contains(t, err.Error(), "exit status 4")
	assert.Contains(t, err.Error(), "not a structured log")
	var exitErr *pluginExitError
	require.ErrorAs(t, err, &exitErr)
	assert.True(t, exitErr.portCollision, "the collision is read from raw stderr even when it is not shown")
	assert.Equal(t, 1, countStarts(t, starts))
}
