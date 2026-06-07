package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// T008 [US2] - Verify ResolvePolicyPackDir returns default path and FINFOCUS_HOME override.
func TestResolvePolicyPackDir(t *testing.T) {
	t.Run("default_path", func(t *testing.T) {
		// Clear FINFOCUS_HOME to test default behavior
		t.Setenv("FINFOCUS_HOME", "")

		dir, err := ResolvePolicyPackDir()
		require.NoError(t, err)

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		expected := filepath.Join(homeDir, ".finfocus", "analyzer")
		assert.Equal(t, expected, dir)
	})

	t.Run("FINFOCUS_HOME_override", func(t *testing.T) {
		customHome := t.TempDir()
		t.Setenv("FINFOCUS_HOME", customHome)

		dir, err := ResolvePolicyPackDir()
		require.NoError(t, err)

		expected := filepath.Join(customHome, "analyzer")
		assert.Equal(t, expected, dir)
	})
}

// T009 [US2] - Verify WritePulumiPolicyYAML creates valid YAML with required fields.
func TestWritePulumiPolicyYAML(t *testing.T) {
	dir := t.TempDir()

	err := WritePulumiPolicyYAML(dir)
	require.NoError(t, err)

	// Read and parse the written file
	data, err := os.ReadFile(filepath.Join(dir, pulumiPolicyFilename))
	require.NoError(t, err)

	var cfg PolicyPackConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "finfocus", cfg.Name, "name must be 'finfocus'")
	assert.Equal(t, "finfocus", cfg.Runtime, "runtime must be 'finfocus'")
	assert.NotEmpty(t, cfg.Description, "description must not be empty")
}

// T009 continued - Verify WritePulumiPolicyYAML is idempotent.
func TestWritePulumiPolicyYAML_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// Write twice
	err := WritePulumiPolicyYAML(dir)
	require.NoError(t, err)
	err = WritePulumiPolicyYAML(dir)
	require.NoError(t, err)

	// File should still be valid
	data, err := os.ReadFile(filepath.Join(dir, pulumiPolicyFilename))
	require.NoError(t, err)

	var cfg PolicyPackConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "finfocus", cfg.Name)
	assert.Equal(t, "finfocus", cfg.Runtime)
}

// T009 continued - Verify WritePulumiPolicyYAML fails on nonexistent directory.
func TestWritePulumiPolicyYAML_NonexistentDir(t *testing.T) {
	nonexistentDir := filepath.Join(t.TempDir(), "nonexistent", "path", "that", "does", "not", "exist")
	err := WritePulumiPolicyYAML(nonexistentDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), pulumiPolicyFilename)
}

// T010 [US2] - Verify SetupPolicyPack creates directory, binary, and YAML.
func TestSetupPolicyPack_CreatesDirectory(t *testing.T) {
	// Create a fake executable to serve as source
	srcDir := t.TempDir()
	fakeBinary := filepath.Join(srcDir, "finfocus")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho test"), 0o755))

	// Use a temp dir as FINFOCUS_HOME so we don't touch the real home
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)

	ctx := context.Background()
	ppDir, method, err := SetupPolicyPack(ctx, fakeBinary)
	require.NoError(t, err)

	// Verify directory was created
	expectedDir := filepath.Join(finfocusHome, "analyzer")
	assert.Equal(t, expectedDir, ppDir)

	info, err := os.Stat(ppDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify PulumiPolicy.yaml exists
	yamlPath := filepath.Join(ppDir, pulumiPolicyFilename)
	_, err = os.Stat(yamlPath)
	require.NoError(t, err, "PulumiPolicy.yaml should exist")

	// Verify binary reference exists
	binaryPath := filepath.Join(ppDir, policyPackBinaryName)
	_, err = os.Stat(binaryPath)
	require.NoError(t, err, "binary reference should exist")

	// Verify method is symlink or copy
	assert.Contains(t, []string{"symlink", "copy"}, method)
}

// T010 continued - Verify SetupPolicyPack is idempotent (re-setup works).
func TestSetupPolicyPack_Idempotent(t *testing.T) {
	srcDir := t.TempDir()
	fakeBinary := filepath.Join(srcDir, "finfocus")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho test"), 0o755))

	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)

	ctx := context.Background()

	// First setup
	ppDir1, method1, err := SetupPolicyPack(ctx, fakeBinary)
	require.NoError(t, err)

	// Second setup (should succeed, not fail)
	ppDir2, method2, err := SetupPolicyPack(ctx, fakeBinary)
	require.NoError(t, err)

	assert.Equal(t, ppDir1, ppDir2, "directory should be the same")
	assert.Equal(t, method1, method2, "method should be the same")

	// Verify everything still works
	yamlPath := filepath.Join(ppDir2, pulumiPolicyFilename)
	_, err = os.Stat(yamlPath)
	require.NoError(t, err)

	binaryPath := filepath.Join(ppDir2, policyPackBinaryName)
	_, err = os.Stat(binaryPath)
	require.NoError(t, err)
}

// T011 [US2] - Verify file copy path works (used on Windows or when symlinks fail).
// Since runtime.GOOS cannot be mocked, we test the copyFile path directly
// which is the code path used on Windows.
func TestSetupPolicyPack_WindowsCopy(t *testing.T) {
	t.Parallel()
	t.Run("copyBinary_creates_valid_copy", func(t *testing.T) {
		t.Parallel()
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// Create source binary with known content
		srcPath := filepath.Join(srcDir, "finfocus")
		content := []byte("#!/bin/sh\necho finfocus-binary")
		require.NoError(t, os.WriteFile(srcPath, content, 0o755))

		dstPath := filepath.Join(dstDir, policyPackBinaryName)
		err := copyBinary(srcPath, dstPath)
		require.NoError(t, err)

		// Verify the copy exists and has correct content
		copiedContent, err := os.ReadFile(dstPath)
		require.NoError(t, err)
		assert.Equal(t, content, copiedContent)

		// Verify executable permissions preserved. Windows has no Unix
		// permission bits (os.FileInfo.Mode reports 0666/0444 there), so the
		// check only applies on Unix-like systems.
		if runtime.GOOS != "windows" {
			fi, err := os.Stat(dstPath)
			require.NoError(t, err)
			assert.True(t, fi.Mode()&0o111 != 0, "copy should preserve executable permissions")
		}
	})

	t.Run("copyBinary_source_not_found", func(t *testing.T) {
		t.Parallel()
		dstPath := filepath.Join(t.TempDir(), policyPackBinaryName)
		nonexistentSrc := filepath.Join(t.TempDir(), "nonexistent_source", "binary")
		err := copyBinary(nonexistentSrc, dstPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "opening source")
	})

	t.Run("policyPackBinaryPath_returns_correct_name", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "policy", "pack")
		path := policyPackBinaryPath(dir)
		assert.Contains(t, path, policyPackBinaryName)
		assert.True(t, filepath.IsAbs(path))
	})
}

// T010 continued - Verify SetupPolicyPack fails when directory cannot be created.
func TestSetupPolicyPack_DirectoryCreationFails(t *testing.T) {
	// Point FINFOCUS_HOME to a file (not a directory) so MkdirAll fails
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking")
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0o644))

	t.Setenv("FINFOCUS_HOME", blockingFile)

	ctx := context.Background()
	_, _, err := SetupPolicyPack(ctx, "/some/binary")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating policy pack directory")
}
