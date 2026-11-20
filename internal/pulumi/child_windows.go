//go:build windows

package pulumi

import (
	"os/exec"

	"github.com/rshade/finfocus/internal/procjob"
)

// configureChild joins the kill-on-close job object before the Pulumi child
// starts, so the child and everything it spawns die with finfocus. Windows
// cannot deliver an interrupt to a child, so cancellation kills it.
func configureChild(_ *exec.Cmd) {
	_ = procjob.Adopt()
}
