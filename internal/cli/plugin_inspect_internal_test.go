package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// writePluginBinary creates an executable aws-public plugin binary inside versionDir.
func writePluginBinary(t *testing.T, versionDir string) string {
	t.Helper()
	binPath := filepath.Join(versionDir, "finfocus-plugin-aws-public")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	require.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0o755))
	return binPath
}

// TestFindPluginPath_LatestVersionResolution verifies that inspect resolves the
// latest plugin version using the same rule as the registry (issue #1683):
// version directories with and without a "v" prefix count, and symlinks are
// followed (issue #750).
func TestFindPluginPath_LatestVersionResolution(t *testing.T) {
	t.Parallel()

	const pluginName = "aws-public"

	t.Run("bare semver dir without v prefix", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		versionDir := filepath.Join(root, pluginName, "0.1.0")
		require.NoError(t, os.MkdirAll(versionDir, 0o755))
		want := writePluginBinary(t, versionDir)

		got, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("v-prefixed dir", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		versionDir := filepath.Join(root, pluginName, "v0.1.0")
		require.NoError(t, os.MkdirAll(versionDir, 0o755))
		want := writePluginBinary(t, versionDir)

		got, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("picks latest across mixed dir forms", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		oldDir := filepath.Join(root, pluginName, "0.1.0")
		require.NoError(t, os.MkdirAll(oldDir, 0o755))
		writePluginBinary(t, oldDir)
		newDir := filepath.Join(root, pluginName, "v0.2.0")
		require.NoError(t, os.MkdirAll(newDir, 0o755))
		want := writePluginBinary(t, newDir)

		got, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "")
		require.NoError(t, err)
		assert.Equal(t, want, got, "must pick the same latest version as plugin list")
	})

	t.Run("symlinked version dir", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skipf("os.Symlink may require elevation on Windows; skipping symlink test")
		}
		root := t.TempDir()
		realDir := filepath.Join(root, pluginName, "v0.1.0")
		require.NoError(t, os.MkdirAll(realDir, 0o755))
		writePluginBinary(t, realDir)

		linkDir := filepath.Join(root, pluginName, "v0.2.0")
		require.NoError(t, os.Symlink(realDir, linkDir))
		want := writePluginBinary(t, linkDir)

		got, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "")
		require.NoError(t, err)
		assert.Equal(t, want, got, "must follow symlinked version directories")
	})

	t.Run("no valid semver versions", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		versionDir := filepath.Join(root, pluginName, "latest")
		require.NoError(t, os.MkdirAll(versionDir, 0o755))
		writePluginBinary(t, versionDir)

		_, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid semver versions found")
	})

	t.Run("plugin not installed", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		_, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
	})

	t.Run("explicit version accepts bare semver dir", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		versionDir := filepath.Join(root, pluginName, "0.1.0")
		require.NoError(t, os.MkdirAll(versionDir, 0o755))
		want := writePluginBinary(t, versionDir)

		got, err := findPluginPath(&config.Config{PluginDir: root}, pluginName, "0.1.0")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
