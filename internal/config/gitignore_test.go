package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestEnsureGitignore_CreatesNewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	created, err := config.EnsureGitignore(dir)
	require.NoError(t, err)
	assert.True(t, created, "should report file was created")

	gitignorePath := filepath.Join(dir, ".gitignore")
	_, statErr := os.Stat(gitignorePath)
	require.NoError(t, statErr, ".gitignore file should exist")
}

func TestEnsureGitignore_DoesNotOverwriteExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")

	customContent := "# my custom gitignore\nnode_modules/\n"
	require.NoError(t, os.WriteFile(gitignorePath, []byte(customContent), 0o644))

	created, err := config.EnsureGitignore(dir)
	require.NoError(t, err)
	assert.False(t, created, "should report file was NOT created")

	data, readErr := os.ReadFile(gitignorePath)
	require.NoError(t, readErr)
	assert.Equal(t, customContent, string(data), "existing content must be preserved")
}

func TestEnsureGitignore_CreatesParentDirectory(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	nestedDir := filepath.Join(base, "sub", "deep", ".finfocus")

	created, err := config.EnsureGitignore(nestedDir)
	require.NoError(t, err)
	assert.True(t, created, "should report file was created")

	gitignorePath := filepath.Join(nestedDir, ".gitignore")
	data, readErr := os.ReadFile(gitignorePath)
	require.NoError(t, readErr)
	assert.Equal(t, config.GitignoreContent(), string(data), "content should match expected gitignore")
}

func TestEnsureGitignore_ContentVerification(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	created, err := config.EnsureGitignore(dir)
	require.NoError(t, err)
	require.True(t, created)

	gitignorePath := filepath.Join(dir, ".gitignore")
	data, readErr := os.ReadFile(gitignorePath)
	require.NoError(t, readErr)

	content := string(data)
	assert.Contains(t, content, "dismissed.json")
	assert.Contains(t, content, "dismissed.json.lock")
	assert.Contains(t, content, "dismissed.json.tmp")
	assert.Contains(t, content, "*.log")
	assert.Contains(t, content, "# FinFocus project-local data (auto-generated)")
}

func TestEnsureGitignore_ReturnValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// First call: should create.
	created, err := config.EnsureGitignore(dir)
	require.NoError(t, err)
	assert.True(t, created, "first call should return true")

	// Second call: should not create.
	created, err = config.EnsureGitignore(dir)
	require.NoError(t, err)
	assert.False(t, created, "second call should return false")
}

func TestEnsureGitignore_ReadOnlyParentDir(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("file permission tests not reliable on Windows")
	}

	base := t.TempDir()
	readonlyDir := filepath.Join(base, "readonly")
	require.NoError(t, os.MkdirAll(readonlyDir, 0o755))
	require.NoError(t, os.Chmod(readonlyDir, 0o444))
	t.Cleanup(func() {
		_ = os.Chmod(readonlyDir, 0o755)
	})

	created, err := config.EnsureGitignore(readonlyDir)
	require.Error(t, err, "should fail when directory is read-only")
	assert.False(t, created, "should not report creation on error")
}
