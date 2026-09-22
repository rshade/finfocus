//go:build linux

package pluginhost

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup places the plugin in its own process group so the whole
// group can be killed at once, and sets Pdeathsig so the kernel SIGKILLs the
// plugin if Core dies — even when Core is force-killed and deferred cleanup
// never runs (issue #1231).
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
}
