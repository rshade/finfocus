//go:build unix

package pluginhost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowPluginScript writes a fake plugin that records its pid and then sleeps
// without binding, like a large binary still paging in when its deadline ends.
func slowPluginScript(t *testing.T) (string, string) {
	t.Helper()
	pids := filepath.Join(t.TempDir(), "pids")
	script := createScript(t, fmt.Sprintf(`#!/bin/sh
echo $$ >> '%s'
exec sleep 30
`, pids), ".sh")
	return script, pids
}

func startedPIDs(t *testing.T, path string) []int {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var pids []int
	for field := range strings.FieldsSeq(string(data)) {
		pid, convErr := strconv.Atoi(field)
		require.NoError(t, convErr)
		pids = append(pids, pid)
	}
	return pids
}

//nolint:paralleltest // writes and executes a script; a parallel fork inherits the write fd (text file busy)
func TestStartWithRetry_SlowBindStopsAtDeadline(t *testing.T) {
	script, pids := slowPluginScript(t)
	launcher := NewProcessLauncherWithRetries(5)
	launcher.stdoutFallback = 200 * time.Millisecond

	var logs lockedBuffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
	ctx, cancel := context.WithTimeout(logger.WithContext(context.Background()), time.Second)
	defer cancel()

	begin := time.Now()
	conn, closeFn, err := launcher.StartWithRetry(ctx, script)
	elapsed := time.Since(begin)

	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Less(t, elapsed, 2*time.Second, "an expired context must not be retried")

	msg := err.Error()
	assert.Contains(t, msg, "plugin failed to bind to port within 1s")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotContains(t, msg, "starting plugin", "the bind timeout must not be replaced by a retry's start failure")
	assert.NotContains(t, msg, "failed after")

	started := startedPIDs(t, pids)
	require.Len(t, started, 1, "a plugin whose deadline passed is started once")
	require.ErrorIs(t, syscall.Kill(started[0], 0), syscall.ESRCH, "the timed-out plugin must not be left running")
	assert.NotContains(t, string(logs.Snapshot()), bindGuidance,
		"a plugin cut off by the caller's deadline may only be slow, so the --port guidance does not apply")
}

//nolint:paralleltest // writes and executes a script; a parallel fork inherits the write fd (text file busy)
func TestStartWithRetry_BindTimeoutIsNotRetried(t *testing.T) {
	script, pids := slowPluginScript(t)
	launcher := NewProcessLauncherWithRetries(5)
	launcher.stdoutFallback = 100 * time.Millisecond
	launcher.bindTimeout = 300 * time.Millisecond

	var logs lockedBuffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
	ctx, cancel := context.WithCancel(logger.WithContext(context.Background()))
	defer cancel()

	conn, closeFn, err := launcher.StartWithRetry(ctx, script)

	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	msg := err.Error()
	assert.Contains(t, msg, "plugin failed to bind to port within")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotContains(t, msg, "failed after", "a bind timeout is not a port collision")

	started := startedPIDs(t, pids)
	require.Len(t, started, 1, "a plugin that never binds is started once, even with no caller deadline")
	require.ErrorIs(t, syscall.Kill(started[0], 0), syscall.ESRCH, "the timed-out plugin must not be left running")

	logged := string(logs.Snapshot())
	assert.NotContains(t, logged, "retrying plugin launch")
	assert.Contains(t, logged, bindGuidance, "the full bind timeout passed with nothing listening on --port")
}

//nolint:paralleltest // writes and executes a script; a parallel fork inherits the write fd (text file busy)
func TestStartWithRetry_CollisionBackoffStopsAtDeadline(t *testing.T) {
	script, starts := exitingPluginScript(
		t,
		`{"level":"fatal","error":"failed to listen: listen tcp 127.0.0.1:1: bind: address already in use","message":"serve"}`,
		1,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	begin := time.Now()
	_, _, err := NewProcessLauncherWithRetries(10).StartWithRetry(ctx, script)
	elapsed := time.Since(begin)

	require.Error(t, err)
	assert.Less(t, elapsed, 600*time.Millisecond, "the retry backoff must end with the context")
	assert.Contains(t, err.Error(), "address already in use")
	assert.NotContains(t, err.Error(), "starting plugin")
	var exitErr *pluginExitError
	require.ErrorAs(t, err, &exitErr, "the last collision is reported")
	assert.Less(t, countStarts(t, starts), 10)
}
