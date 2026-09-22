//go:build unix && !linux

package pluginhost

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup places the plugin in its own process group so the whole
// group can be killed at once (issue #1231). Unlike Linux there is no portable
// parent-death signal on these platforms, so abnormal Core exits rely on plugin
// stdio not being inherited (see resolveStderrPassthrough).
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
