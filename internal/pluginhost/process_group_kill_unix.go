//go:build unix

package pluginhost

import (
	"errors"
	"os/exec"
	"syscall"
)

// killProcessGroup SIGKILLs the plugin's entire process group so plugin
// children cannot survive and hold inherited file descriptors open after the
// plugin's main process dies (issue #1231). Falls back to killing only the
// main process if the group signal fails. Safe to call with a nil cmd or a
// command that was never started.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// The plugin is group leader (Setpgid), so pgid == pid.
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return cmd.Process.Kill()
}
