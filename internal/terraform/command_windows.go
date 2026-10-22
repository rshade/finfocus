//go:build windows

package terraform

import (
	"context"
	"os/exec"
)

func newCommand(ctx context.Context, binary, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	return cmd
}
