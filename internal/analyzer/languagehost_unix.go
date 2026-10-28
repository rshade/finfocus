//go:build unix

package analyzer

import (
	"os"
	"os/exec"
)

// setCmdCancel stops the analyzer with SIGINT, the signal Pulumi sent before
// 3.270.0. CommandContext's default SIGKILL skips the analyzer's plugin
// cleanup. WaitDelay kills the process if it ignores the interrupt.
func setCmdCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return cmd.Process.Signal(os.Interrupt)
	}
}
