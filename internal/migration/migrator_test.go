package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeCopy(t *testing.T) {
	// Setup temporary source directory
	src := t.TempDir()
	dst := t.TempDir()

	// Create some files and directories in src
	require.NoError(t, os.MkdirAll(filepath.Join(src, "plugins", "aws"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(src, "config.yaml"), []byte("test config"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "plugins", "aws", "plugin.exe"), []byte("binary"), 0700))

	// Perform copy
	err := SafeCopy(src, dst)
	require.NoError(t, err)

	// Verify destination
	content, err := os.ReadFile(filepath.Join(dst, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "test config", string(content))

	content, err = os.ReadFile(filepath.Join(dst, "plugins", "aws", "plugin.exe"))
	require.NoError(t, err)
	assert.Equal(t, "binary", string(content))

	info, err := os.Stat(filepath.Join(dst, "plugins", "aws", "plugin.exe"))
	require.NoError(t, err)
	// On Windows, file permissions work differently - just ensure it's a regular file
	if runtime.GOOS == "windows" {
		assert.True(t, info.Mode().IsRegular(), "should be a regular file")
	} else {
		assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
	}
}
