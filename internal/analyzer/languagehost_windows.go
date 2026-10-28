//go:build windows

package analyzer

import (
	"os"
	"os/exec"
)

// setCmdCancel kills the analyzer. Windows cannot deliver os.Interrupt to
// another process, matching Pulumi's forcible stop. A killed analyzer does
// not run plugin cleanup.
func setCmdCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return cmd.Process.Kill()
	}
}
