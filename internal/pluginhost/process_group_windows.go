//go:build windows

package pluginhost

import "os/exec"

// configureProcessGroup is a no-op on Windows. Job Objects with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE would give the same guarantees as the
// Unix process-group setup; that is tracked as follow-up work for issue #1231.
func configureProcessGroup(_ *exec.Cmd) {}

// killProcessGroup kills only the plugin's main process; Windows has no
// process-group kill without Job Object support.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
