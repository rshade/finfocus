package pluginupgrade_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginupgrade"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{
		"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
	}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestCheckClean(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	t.Run("not a repository", func(t *testing.T) {
		t.Parallel()
		err := pluginupgrade.CheckClean(t.Context(), t.TempDir())
		require.ErrorIs(t, err, pluginupgrade.ErrNotGitRepo)
	})

	t.Run("only the plugin directory counts", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		plugin := filepath.Join(repo, "plugins", "demo")
		require.NoError(t, os.MkdirAll(plugin, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(plugin, "go.mod"), []byte("module demo\n"), 0o600))
		git(t, repo, "init", "-q")
		git(t, repo, "add", ".")
		git(t, repo, "commit", "-q", "-m", "init")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("x"), 0o600))

		require.NoError(t, pluginupgrade.CheckClean(t.Context(), plugin))
		require.ErrorIs(t, pluginupgrade.CheckClean(t.Context(), repo), pluginupgrade.ErrDirtyTree)
	})

	t.Run("clean and dirty", func(t *testing.T) {
		t.Parallel()
		dir := copyFixture(t, "finfocus-v0.7.0")
		git(t, dir, "init", "-q")
		git(t, dir, "add", ".")
		git(t, dir, "commit", "-q", "-m", "init")
		require.NoError(t, pluginupgrade.CheckClean(t.Context(), dir))

		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600))
		err := pluginupgrade.CheckClean(t.Context(), dir)
		require.ErrorIs(t, err, pluginupgrade.ErrDirtyTree)
		assert.Contains(t, err.Error(), "main.go")
	})
}

// TestCheckCleanWithoutGit clears PATH with t.Setenv, which also keeps it
// sequential.
func TestCheckCleanWithoutGit(t *testing.T) {
	t.Setenv("PATH", "")

	err := pluginupgrade.CheckClean(t.Context(), t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--allow-dirty")
}
