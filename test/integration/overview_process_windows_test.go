//go:build windows

package integration_test

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// windowsProcessAlive reports whether pid is still running. A process that
// cannot be opened is gone (or not ours to see), which is what a killed
// descendant looks like.
func windowsProcessAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // Pids are small positive integers.
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var code uint32
	if err = windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	const stillActive = 259
	return code == stillActive
}

// TestOverviewKill_JobObjectEndsEveryDescendant ends a running overview with
// TerminateProcess, which runs no cleanup, and asserts the plugin, its child,
// the Pulumi CLI, and its child are all gone: the kill-on-close job object
// (internal/procjob) is the only thing that can end them. A bystander outside
// the job must survive.
//
//nolint:paralleltest // Writes and executes binaries; a concurrent fork would hold the write fd.
func TestOverviewKill_JobObjectEndsEveryDescendant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}
	f := newOrphanFixture(t).withFakePulumi(t, false)

	bystander := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 600")
	require.NoError(t, bystander.Start())
	t.Cleanup(func() { _ = bystander.Process.Kill(); _ = bystander.Wait() })

	cmd := exec.Command(f.binary, "overview", "--plain", "--yes", "--output", "json", "--stack", "dev")
	cmd.Dir = f.project
	cmd.Env = f.env
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)
	t.Cleanup(func() { _ = stderrFile.Close() })
	cmd.Stderr = stderrFile
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		for _, pid := range f.recordedPIDs() {
			if p, findErr := os.FindProcess(pid); findErr == nil {
				_ = p.Kill()
			}
		}
	})

	stderr := func() string {
		data, _ := os.ReadFile(stderrFile.Name())
		return string(data)
	}
	var plugin, pulumi [2]int
	require.Eventually(t, func() bool {
		var ok bool
		plugin, ok = readPIDPair(f.pidFile)
		return ok
	}, orphanWait, 50*time.Millisecond, "fixture plugin never started; stderr:\n%s", stderr())
	require.Eventually(t, func() bool {
		var ok bool
		pulumi, ok = readPIDPair(f.pulumiPIDFile)
		return ok
	}, orphanWait, 50*time.Millisecond, "fixture preview never started; stderr:\n%s", stderr())
	descendants := []int{plugin[0], plugin[1], pulumi[0], pulumi[1]}
	for _, pid := range descendants {
		require.True(t, windowsProcessAlive(pid), "pid %d must be running before the kill", pid)
	}
	t.Logf("descendants before kill: %v", descendants)

	started := time.Now()
	require.NoError(t, cmd.Process.Kill())
	select {
	case <-done:
	case <-time.After(orphanWait):
		require.FailNow(t, "finfocus overview did not end", stderr())
	}
	for _, pid := range descendants {
		assert.Eventually(t, func() bool { return !windowsProcessAlive(pid) }, orphanWait, 50*time.Millisecond,
			"pid %d survived the kill", pid)
	}
	t.Logf("every descendant gone %s after the kill", time.Since(started).Round(time.Millisecond))
	assert.True(t, windowsProcessAlive(bystander.Process.Pid), "an unrelated process must survive")
}
