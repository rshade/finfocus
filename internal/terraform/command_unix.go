//go:build unix

package terraform

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

func newCommand(ctx context.Context, binary, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	const processKillWait = 2 * time.Second
	cmd.WaitDelay = processKillWait
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return cmd
}
