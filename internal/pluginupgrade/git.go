package pluginupgrade

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	// ErrNotGitRepo means the plugin directory is not inside a git working tree.
	ErrNotGitRepo = errors.New("not a git repository")
	// ErrDirtyTree means the plugin's working tree has uncommitted changes.
	ErrDirtyTree = errors.New("working tree has uncommitted changes")
)

// maxDirtyPaths bounds how many dirty paths CheckClean names in its error.
const maxDirtyPaths = 5

// CheckClean reports whether dir is inside a git working tree and has no
// uncommitted or untracked changes under it, so an upgrade lands as one
// reviewable, revertible diff. Changes elsewhere in the repository do not
// count: a plugin can be a nested module of a larger repository.
func CheckClean(ctx context.Context, dir string) error {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain", "--", ".").Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("git is not installed; pass --allow-dirty to apply without the clean-tree check")
		}
		if exitErr, isExit := errors.AsType[*exec.ExitError](err); isExit {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "not a git repository") {
				return fmt.Errorf("%w: %s (pass --allow-dirty to apply anyway)", ErrNotGitRepo, dir)
			}
			return fmt.Errorf("git status failed in %s: %s (pass --allow-dirty to skip this check)", dir, stderr)
		}
		return fmt.Errorf("running git status: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	shown := lines
	if len(shown) > maxDirtyPaths {
		shown = append(shown[:maxDirtyPaths:maxDirtyPaths], fmt.Sprintf("… and %d more", len(lines)-maxDirtyPaths))
	}
	return fmt.Errorf("%w (commit or stash them, or pass --allow-dirty):\n%s",
		ErrDirtyTree, strings.Join(shown, "\n"))
}
