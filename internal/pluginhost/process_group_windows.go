//go:build windows

package pluginhost

import (
	"os/exec"

	"github.com/rshade/finfocus/internal/procjob"
)

// configureProcessGroup puts finfocus in a kill-on-close job object before the
// plugin starts, so the plugin and every descendant it creates are members and
// die with finfocus, even when finfocus is terminated without cleanup
// (issue #1231). An error leaves the earlier behavior: the plugin's main
// process is killed on close and descendants may outlive it.
func configureProcessGroup(_ *exec.Cmd) {
	_ = procjob.Adopt()
}

// killProcessGroup kills the plugin's main process. Its descendants are job
// members and end when finfocus exits; Windows offers no per-plugin group
// kill without a post-start assignment that would race the plugin's own
// children.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
