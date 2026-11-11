package pluginhost

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rshade/finfocus/internal/logging"
)

type recordingTap struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func (r *recordingTap) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

func (r *recordingTap) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *recordingTap) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func (r *recordingTap) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type tapFactory struct {
	mu    sync.Mutex
	names []string
	tap   *recordingTap
}

func (f *tapFactory) make(name string) io.WriteCloser {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.names = append(f.names, name)
	return f.tap
}

func writeScript(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "finfocus-plugin-demo")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
	return path
}

const (
	jsonLine  = `{"level":"info","message":"from plugin"}`
	plainLine = "plain plugin line"
)

//nolint:paralleltest // writes a script and then executes it (text file busy)
func TestStartPlugin_LogTapReceivesStderrAndKeepsDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script plugin is a shell script")
	}

	dir := t.TempDir()
	envOut := filepath.Join(dir, "env.txt")
	script := writeScript(t, dir,
		"#!/bin/sh\n"+
			"env > '"+envOut+"'\n"+
			"printf '%s\\n' '"+jsonLine+"' >&2\n"+
			"printf '%s\\n' '"+plainLine+"' >&2\n")

	factory := &tapFactory{tap: &recordingTap{}}
	var dest bytes.Buffer
	ctx := logging.ContextWithPluginLogWriter(context.Background(), &dest)
	ctx = logging.ContextWithPluginLogPath(ctx, filepath.Join(dir, "core.log"))
	ctx = logging.ContextWithPluginLogTap(ctx, factory.make)

	cmd, _, err := NewProcessLauncher().startPlugin(ctx, script, 4321, nil)
	require.NoError(t, err)
	require.NoError(t, cmd.Wait())

	want := jsonLine + "\n" + plainLine + "\n"
	assert.Equal(t, want, factory.tap.String())
	assert.Equal(t, want, dest.String())
	assert.Equal(t, []string{"demo"}, factory.names)

	env, err := os.ReadFile(envOut)
	require.NoError(t, err)
	assert.NotContains(t, string(env), "FINFOCUS_LOG_FILE=")
}

//nolint:paralleltest // writes a script and then executes it (text file busy)
func TestStartPlugin_NoTapKeepsLogFileEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script plugin is a shell script")
	}

	dir := t.TempDir()
	envOut := filepath.Join(dir, "env.txt")
	logPath := filepath.Join(dir, "core.log")
	script := writeScript(t, dir,
		"#!/bin/sh\nenv > '"+envOut+"'\n")

	ctx := logging.ContextWithPluginLogPath(context.Background(), logPath)

	cmd, _, err := NewProcessLauncher().startPlugin(ctx, script, 4321, nil)
	require.NoError(t, err)
	require.NoError(t, cmd.Wait())

	env, err := os.ReadFile(envOut)
	require.NoError(t, err)
	assert.Contains(t, string(env), "FINFOCUS_LOG_FILE="+logPath)
	_, isTapped := cmd.Stderr.(*tappedStderr)
	assert.False(t, isTapped)
}

func runSDKStyleFixture(t *testing.T, withTap bool) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "core.log")
	script := writeScript(t, dir,
		"#!/bin/sh\n"+
			"if [ -n \"$FINFOCUS_LOG_FILE\" ]; then\n"+
			"  printf '%s\\n' '"+jsonLine+"' >> \"$FINFOCUS_LOG_FILE\"\n"+
			"else\n"+
			"  printf '%s\\n' '"+jsonLine+"' >&2\n"+
			"fi\n")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = logFile.Close() })

	ctx := logging.ContextWithPluginLogWriter(context.Background(), logFile)
	ctx = logging.ContextWithPluginLogPath(ctx, logPath)
	if withTap {
		factory := &tapFactory{tap: &recordingTap{}}
		ctx = logging.ContextWithPluginLogTap(ctx, factory.make)
	}

	cmd, _, startErr := NewProcessLauncher().startPlugin(ctx, script, 4321, nil)
	require.NoError(t, startErr)
	require.NoError(t, cmd.Wait())

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	return string(data)
}

//nolint:paralleltest // writes a script and then executes it (text file busy)
func TestStartPlugin_LogFileContentIdenticalWithAndWithoutTap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script plugin is a shell script")
	}

	without := runSDKStyleFixture(t, false)
	with := runSDKStyleFixture(t, true)

	assert.Contains(t, without, "from plugin")
	assert.Equal(t, without, with)
}

//nolint:paralleltest // writes a script and then executes it (text file busy)
func TestCreateCloseFn_ClosesLogTapAfterWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script plugin is a shell script")
	}

	dir := t.TempDir()
	script := writeScript(t, dir,
		"#!/bin/sh\nprintf 'no newline' >&2\n")

	factory := &tapFactory{tap: &recordingTap{}}
	ctx := logging.ContextWithPluginLogTap(context.Background(), factory.make)

	launcher := NewProcessLauncher()
	cmd, _, err := launcher.startPlugin(ctx, script, 4321, nil)
	require.NoError(t, err)

	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return factory.tap.String() != ""
	}, 5*time.Second, 10*time.Millisecond)
	require.False(t, factory.tap.isClosed())
	require.NoError(t, launcher.createCloseFn(ctx, conn, watchProcess(cmd))())

	assert.True(t, factory.tap.isClosed())
	assert.True(t, strings.HasPrefix(factory.tap.String(), "no newline"))
}
