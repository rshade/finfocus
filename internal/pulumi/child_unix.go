//go:build !windows

package pulumi

import (
	"os"
	"os/exec"
	"time"
)

// childStopDelay bounds how long a Pulumi child gets to stop after its
// interrupt before it is killed.
const childStopDelay = 5 * time.Second

// configureChild makes cancellation interrupt the Pulumi child instead of
// killing it, so Pulumi can stop its own language host and providers; a child
// still running after childStopDelay is killed. The child stays in finfocus's
// process group and session.
func configureChild(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = childStopDelay
}
