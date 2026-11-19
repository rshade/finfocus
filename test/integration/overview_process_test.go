//go:build unix

package integration_test

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// overviewRun is a non-interactive `finfocus overview` against the fixture
// pulumi CLI, whose preview never finishes, so the run is always in flight
// when a test signals it.
type overviewRun struct {
	cmd    *exec.Cmd
	done   chan error
	stderr func() string
	// pluginPIDs and pulumiPIDs hold the process and its child.
	pluginPIDs [2]int
	pulumiPIDs [2]int
}

func startOverview(t *testing.T, ignoreTerm bool) *overviewRun {
	t.Helper()
	f := newOrphanFixture(t).withFakePulumi(t, ignoreTerm)

	cmd := exec.Command(f.binary, "overview", "--plain", "--yes", "--output", "json", "--stack", "dev")
	cmd.Dir = f.project
	cmd.Env = f.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)
	t.Cleanup(func() { _ = stderrFile.Close() })
	cmd.Stderr = stderrFile
	require.NoError(t, cmd.Start())

	run := &overviewRun{cmd: cmd, done: make(chan error, 1), stderr: func() string {
		data, _ := os.ReadFile(stderrFile.Name())
		return string(data)
	}}
	go func() { run.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		for _, pid := range f.recordedPIDs() {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})

	require.Eventually(t, func() bool {
		var ok bool
		run.pluginPIDs, ok = readPIDPair(f.pidFile)
		return ok
	}, orphanWait, 50*time.Millisecond, "fixture plugin never started; stderr:\n%s", run.stderr())
	require.Eventually(t, func() bool {
		var ok bool
		run.pulumiPIDs, ok = readPIDPair(f.pulumiPIDFile)
		return ok
	}, orphanWait, 50*time.Millisecond, "fixture preview never started; stderr:\n%s", run.stderr())
	return run
}

func (r *overviewRun) descendants() []int {
	return []int{r.pluginPIDs[0], r.pluginPIDs[1], r.pulumiPIDs[0], r.pulumiPIDs[1]}
}

func (r *overviewRun) waitExit(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(orphanWait):
		require.FailNow(t, "finfocus overview did not exit", r.stderr())
	}
}

func assertNoneAlive(t *testing.T, pids []int) {
	t.Helper()
	for _, pid := range pids {
		assert.Eventually(t, func() bool { return !processAlive(t, pid) }, orphanWait, 50*time.Millisecond,
			"pid %d survived", pid)
	}
}

// TestOverviewCancel_LeavesNoDescendant sends a graceful signal to a running
// overview and asserts the plugin, its child, the Pulumi CLI, and its child
// are all gone, while an unrelated process is untouched.
//
//nolint:paralleltest // Writes and executes binaries; a concurrent fork would hold the write fd (text file busy).
func TestOverviewCancel_LeavesNoDescendant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			bystander := exec.Command("sleep", "600")
			require.NoError(t, bystander.Start())
			t.Cleanup(func() { _ = bystander.Process.Kill(); _ = bystander.Wait() })

			run := startOverview(t, false)
			for _, pid := range run.descendants() {
				require.True(t, processAlive(t, pid), "pid %d must be running before the signal", pid)
			}

			require.NoError(t, run.cmd.Process.Signal(sig))
			run.waitExit(t)

			assertNoneAlive(t, run.descendants())
			assert.True(t, processAlive(t, bystander.Process.Pid), "an unrelated process must survive")
		})
	}
}

// TestOverviewCancel_PulumiIgnoringTerminationIsKilled covers a Pulumi CLI
// that ignores the interrupt: finfocus must still exit after the bounded wait
// and leave no plugin or Pulumi process running.
//
//nolint:paralleltest // Writes and executes binaries; a concurrent fork would hold the write fd (text file busy).
func TestOverviewCancel_PulumiIgnoringTerminationIsKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}

	run := startOverview(t, true)

	require.NoError(t, run.cmd.Process.Signal(syscall.SIGTERM))
	run.waitExit(t)

	assertNoneAlive(t, []int{run.pluginPIDs[0], run.pluginPIDs[1], run.pulumiPIDs[0]})
}

// TestOverviewKill_DescendantsStayInFinfocusSession asserts the property the
// extension's session sweep relies on: after SIGKILL of finfocus, every
// descendant is still in finfocus's session, because none calls setsid.
//
//nolint:paralleltest // Writes and executes binaries; a concurrent fork would hold the write fd (text file busy).
func TestOverviewKill_DescendantsStayInFinfocusSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-building test in short mode")
	}

	run := startOverview(t, false)
	session := run.cmd.Process.Pid

	for _, pid := range run.descendants() {
		sid, err := unix.Getsid(pid)
		require.NoError(t, err, "pid %d", pid)
		assert.Equal(t, session, sid, "pid %d left finfocus's session", pid)
	}

	require.NoError(t, run.cmd.Process.Kill())
	run.waitExit(t)
	for _, pid := range run.descendants() {
		sid, err := unix.Getsid(pid)
		if err == nil {
			assert.Equal(t, session, sid, "pid %d must stay findable by the session sweep", pid)
		}
	}
}
