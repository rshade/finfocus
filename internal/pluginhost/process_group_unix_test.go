//go:build unix

package pluginhost

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureProcessGroup_SetsSetpgid(t *testing.T) {
	cmd := exec.Command("sleep", "1")
	configureProcessGroup(cmd)
	require.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.Setpgid, "plugin must be placed in its own process group")
}

func TestKillProcessGroup_NilSafe(t *testing.T) {
	assert.NoError(t, killProcessGroup(nil))
	assert.NoError(t, killProcessGroup(&exec.Cmd{}))
}

// TestKillProcessGroup_KillsEntireGroup verifies that killing the plugin's
// process group also terminates processes the plugin itself spawned, so no
// descendant can survive and hold inherited file descriptors open
// (issue #1231).
func TestKillProcessGroup_KillsEntireGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := exec.Command("sh", "-c", "sleep 300 & echo $! > \"$FF_PIDFILE\"; wait")
	cmd.Env = append(os.Environ(), "FF_PIDFILE="+pidFile)
	configureProcessGroup(cmd)
	require.NoError(t, cmd.Start())

	var childPid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		childPid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil && childPid > 0
	}, 5*time.Second, 50*time.Millisecond, "plugin child should have written its pid")

	require.NoError(t, killProcessGroup(cmd))
	_ = cmd.Wait()

	require.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(childPid, 0), syscall.ESRCH)
	}, 5*time.Second, 50*time.Millisecond, "plugin child process should be dead after group kill")
}
