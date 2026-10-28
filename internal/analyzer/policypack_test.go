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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	binaryPath := PolicyPackEntryPath(ppDir, policyPackBinaryName)
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

	binaryPath := PolicyPackEntryPath(ppDir2, policyPackBinaryName)
	_, err = os.Stat(binaryPath)
	require.NoError(t, err)
}

// T011 [US2] - Verify file copy path works (used on Windows or when symlinks fail).
// Since [runtime.GOOS] cannot be mocked, we test the copyFile path directly
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
			fi, statErr := os.Stat(dstPath)
			require.NoError(t, statErr)
			assert.NotEqual(t, 0, fi.Mode()&0o111, "copy should preserve executable permissions")
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

func TestPolicyPackEntryName(t *testing.T) {
	t.Parallel()

	names := []string{policyPackBinaryName, LanguagePluginBinaryName}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, name, policyPackEntryName(name, "linux"))
			assert.Equal(t, name, policyPackEntryName(name, "darwin"))
			assert.Equal(t, name+".exe", policyPackEntryName(name, "windows"))
		})
	}
}

func TestPolicyPackEntryPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{policyPackBinaryName, LanguagePluginBinaryName} {
		path := PolicyPackEntryPath(dir, name)
		assert.Equal(t, filepath.Join(dir, policyPackEntryName(name, runtime.GOOS)), path)
		assert.True(t, filepath.IsAbs(path))
	}
}

// FINFOCUS_HOME is process-wide, so this test does not run in parallel.
func TestSetupPolicyPack_BothEntries(t *testing.T) {
	srcDir := t.TempDir()
	fakeBinary := filepath.Join(srcDir, "finfocus")
	content := []byte("#!/bin/sh\necho finfocus")
	require.NoError(t, os.WriteFile(fakeBinary, content, 0o755))

	t.Setenv("FINFOCUS_HOME", t.TempDir())

	ppDir, method, err := SetupPolicyPack(context.Background(), fakeBinary)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(ppDir, pulumiPolicyFilename))
	require.NoError(t, err)

	for _, name := range policyPackEntryNames() {
		path := PolicyPackEntryPath(ppDir, name)
		info, statErr := os.Lstat(path)
		require.NoError(t, statErr)
		if runtime.GOOS == "windows" {
			assert.Equal(t, "copy", method)
			copied, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, content, copied)
			continue
		}
		assert.Equal(t, "symlink", method)
		assert.NotEqual(t, os.FileMode(0), info.Mode()&os.ModeSymlink)
		target, readErr := os.Readlink(path)
		require.NoError(t, readErr)
		assert.Equal(t, fakeBinary, target)
	}
}

// FINFOCUS_HOME is process-wide, so this test does not run in parallel.
func TestSetupPolicyPack_ReplacesDanglingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dangling symlink replacement is the Unix link path")
	}

	srcDir := t.TempDir()
	fakeBinary := filepath.Join(srcDir, "finfocus")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho finfocus"), 0o755))
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	ppDir, _, err := SetupPolicyPack(context.Background(), fakeBinary)
	require.NoError(t, err)

	for _, name := range policyPackEntryNames() {
		path := PolicyPackEntryPath(ppDir, name)
		require.NoError(t, os.Remove(path))
		require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "missing-target"), path))
	}

	_, _, err = SetupPolicyPack(context.Background(), fakeBinary)
	require.NoError(t, err)

	for _, name := range policyPackEntryNames() {
		target, readErr := os.Readlink(PolicyPackEntryPath(ppDir, name))
		require.NoError(t, readErr)
		assert.Equal(t, fakeBinary, target)
	}
}

// FINFOCUS_HOME is process-wide, so this test does not run in parallel.
func TestSetupPolicyPack_AddsLanguagePluginToLegacyPack(t *testing.T) {
	srcDir := t.TempDir()
	fakeBinary := filepath.Join(srcDir, "finfocus")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho finfocus"), 0o755))
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	ppDir, _, err := SetupPolicyPack(context.Background(), fakeBinary)
	require.NoError(t, err)

	langPath := PolicyPackEntryPath(ppDir, LanguagePluginBinaryName)
	require.NoError(t, os.Remove(langPath))
	_, err = os.Lstat(langPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	_, _, err = SetupPolicyPack(context.Background(), fakeBinary)
	require.NoError(t, err)

	for _, name := range policyPackEntryNames() {
		_, statErr := os.Lstat(PolicyPackEntryPath(ppDir, name))
		require.NoError(t, statErr)
	}
}

// FINFOCUS_HOME and HOME are process-wide, so this test does not run in parallel.
func TestSetupPolicyPack_HomeUnavailable(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	_, _, err := SetupPolicyPack(context.Background(), "/bin/finfocus")
	require.Error(t, err)
	assert.ErrorContains(t, err, "resolving policy pack directory")
}

// FINFOCUS_HOME is process-wide, so this test does not run in parallel.
func TestSetupPolicyPack_PolicyYAMLWriteFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	// WriteFile fails when the YAML path is a directory.
	require.NoError(t, os.MkdirAll(filepath.Join(home, policyPackDirName, pulumiPolicyFilename), 0o750))

	_, _, err := SetupPolicyPack(context.Background(), "/bin/finfocus")
	require.Error(t, err)
	assert.ErrorContains(t, err, "writing policy pack YAML")
}
