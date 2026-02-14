package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/pkg/version"
)

// --- T003: ResolvePulumiPluginDir precedence ---

func TestResolvePulumiPluginDir_TargetDirOverride(t *testing.T) {
	t.Parallel()

	dir, err := ResolvePulumiPluginDir("/custom/path")
	require.NoError(t, err)
	assert.Equal(t, "/custom/path", dir)
}

func TestResolvePulumiPluginDir_PulumiHomeEnv(t *testing.T) {
	t.Setenv("PULUMI_HOME", "/opt/pulumi")

	dir, err := ResolvePulumiPluginDir("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/opt/pulumi", "plugins"), dir)
}

func TestResolvePulumiPluginDir_DefaultHome(t *testing.T) {
	t.Setenv("PULUMI_HOME", "")

	dir, err := ResolvePulumiPluginDir("")
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".pulumi", "plugins"), dir)
}

func TestResolvePulumiPluginDir_OverrideTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv("PULUMI_HOME", "/opt/pulumi")

	dir, err := ResolvePulumiPluginDir("/override/path")
	require.NoError(t, err)
	assert.Equal(t, "/override/path", dir)
}

// --- T004: IsInstalled (consolidated table-driven) ---

func TestIsInstalled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		want    bool
		wantErr bool
	}{
		{
			name: "nonexistent directory",
			setup: func(_ *testing.T) string {
				return "/nonexistent/path"
			},
			want: false,
		},
		{
			name: "empty directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: false,
		},
		{
			name: "with analyzer directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.2.0"), 0o755))
				return dir
			},
			want: true,
		},
		{
			name: "other dirs only",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "resource-aws-v4.0.0"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-other-v1.0.0"), 0o755))
				return dir
			},
			want: false,
		},
		{
			name: "multiple versions",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.1.0"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.2.0"), 0o755))
				return dir
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.setup(t)
			got, err := IsInstalled(dir)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- T005: InstalledVersion (consolidated table-driven) ---

func TestInstalledVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		want    string
		wantErr bool
	}{
		{
			name: "not installed",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: "",
		},
		{
			name: "no directory",
			setup: func(_ *testing.T) string {
				return "/nonexistent/path"
			},
			want: "",
		},
		{
			name: "parses version",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.2.0"), 0o755))
				return dir
			},
			want: "0.2.0",
		},
		{
			name: "parses prerelease",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v1.0.0-beta.1"), 0o755))
				return dir
			},
			want: "1.0.0-beta.1",
		},
		{
			name: "ignores non-analyzer dirs",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "resource-aws-v4.0.0"), 0o755))
				return dir
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.setup(t)
			got, err := InstalledVersion(dir)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- T006: NeedsUpdate ---

func TestNeedsUpdate_NotInstalled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	needs, err := NeedsUpdate(dir)
	require.NoError(t, err)
	assert.False(t, needs)
}

func TestNeedsUpdate_SameVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	currentVer := version.GetVersion()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, analyzerDirPrefix+currentVer), 0o755))

	needs, err := NeedsUpdate(dir)
	require.NoError(t, err)
	assert.False(t, needs)
}

func TestNeedsUpdate_DifferentVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.0.1-old"), 0o755))

	needs, err := NeedsUpdate(dir)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestNeedsUpdate_NonexistentDir(t *testing.T) {
	t.Parallel()

	needs, err := NeedsUpdate("/nonexistent/path")
	require.NoError(t, err)
	assert.False(t, needs)
}

// --- T012-T014: Install function tests ---

func TestInstall_FreshInstall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Installed)
	assert.Equal(t, version.GetVersion(), result.Version)
	assert.Contains(t, result.Path, analyzerBinaryName)
	assert.NotEmpty(t, result.Method)
	assert.False(t, result.NeedsUpdate)
	assert.Equal(t, version.GetVersion(), result.CurrentVersion)
	assert.Equal(t, ActionInstalled, result.Action)

	// Verify the file exists
	_, statErr := os.Lstat(result.Path)
	require.NoError(t, statErr)
}

func TestInstall_AlreadyInstalled_SameVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// First install
	_, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)

	// Second install - should be a no-op
	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Installed)
	assert.Equal(t, version.GetVersion(), result.Version)
	assert.False(t, result.NeedsUpdate)
	assert.Empty(t, result.Method) // No method since no action taken
	assert.Equal(t, ActionAlreadyCurrent, result.Action)
}

func TestInstall_AlreadyInstalled_DifferentVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// Simulate an old version installed
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.0.1-old"), 0o755))

	// Install without force - should return status, not install
	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Installed)
	assert.Equal(t, "0.0.1-old", result.Version)
	assert.True(t, result.NeedsUpdate)
	assert.Empty(t, result.Method) // No action taken
	assert.Equal(t, ActionUpdateAvailable, result.Action)
	assert.Equal(t, version.GetVersion(), result.CurrentVersion)
}

func TestInstall_ForceReplace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// Simulate an old version installed with a dummy binary
	oldDir := filepath.Join(dir, "analyzer-finfocus-v0.0.1-old")
	require.NoError(t, os.MkdirAll(oldDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, analyzerBinaryName), []byte("old"), 0o755))

	// Force install
	result, err := Install(ctx, InstallOptions{TargetDir: dir, Force: true})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Installed)
	assert.Equal(t, version.GetVersion(), result.Version)
	assert.NotEmpty(t, result.Method)
	assert.False(t, result.NeedsUpdate)
	assert.Equal(t, ActionInstalled, result.Action)

	// Old directory should be removed
	_, statErr := os.Stat(oldDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestInstall_CreatesDirectoryIfNeeded(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "plugins")
	ctx := context.Background()

	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Installed)
}

func TestInstall_SymlinkMethod_Unix(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink test only applies to Unix")
	}

	dir := t.TempDir()
	ctx := context.Background()

	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "symlink", result.Method)

	// Verify it's actually a symlink
	fi, err := os.Lstat(result.Path)
	require.NoError(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink != 0)
}

func TestInstall_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission test")
	}

	// Create a read-only directory so MkdirAll fails inside it
	parentDir := t.TempDir()
	readOnlyDir := filepath.Join(parentDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755)
	})

	ctx := context.Background()
	_, err := Install(ctx, InstallOptions{TargetDir: filepath.Join(readOnlyDir, "nested")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating plugin directory")
}

// --- T019-T020: Uninstall function tests ---

func TestUninstall_RemovesAnalyzerDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// Create analyzer directories
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.1.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.2.0"), 0o755))

	err := Uninstall(ctx, dir)
	require.NoError(t, err)

	// Verify all analyzer dirs are removed
	installed, checkErr := IsInstalled(dir)
	require.NoError(t, checkErr)
	assert.False(t, installed)
}

func TestUninstall_NoOp_WhenNotInstalled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	err := Uninstall(ctx, dir)
	require.NoError(t, err)
}

func TestUninstall_PreservesOtherPlugins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// Create both analyzer and non-analyzer directories
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.1.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "resource-aws-v4.0.0"), 0o755))

	err := Uninstall(ctx, dir)
	require.NoError(t, err)

	// Non-analyzer dir should still exist
	_, statErr := os.Stat(filepath.Join(dir, "resource-aws-v4.0.0"))
	require.NoError(t, statErr)
}

func TestUninstall_NonexistentDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Should not error on nonexistent dir (resolves via ResolvePulumiPluginDir)
	err := Uninstall(ctx, filepath.Join(t.TempDir(), "nonexistent"))
	require.NoError(t, err)
}

// --- T028-T029: Custom target directory tests ---

func TestInstall_CustomTargetDir(t *testing.T) {
	t.Parallel()

	customDir := filepath.Join(t.TempDir(), "custom-plugins")
	ctx := context.Background()

	result, err := Install(ctx, InstallOptions{TargetDir: customDir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Installed)
	assert.Contains(t, result.Path, customDir)
}

func TestUninstall_CustomTargetDir(t *testing.T) {
	t.Parallel()

	customDir := filepath.Join(t.TempDir(), "custom-plugins")
	ctx := context.Background()

	// Install first
	_, err := Install(ctx, InstallOptions{TargetDir: customDir})
	require.NoError(t, err)

	// Uninstall
	err = Uninstall(ctx, customDir)
	require.NoError(t, err)

	// Verify removed
	installed, checkErr := IsInstalled(customDir)
	require.NoError(t, checkErr)
	assert.False(t, installed)
}

// --- Helper function tests ---

func TestLinkOrCopy(t *testing.T) {
	t.Parallel()

	// Create a source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source-binary")
	require.NoError(t, os.WriteFile(srcFile, []byte("binary-content"), 0o755))

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "dest-binary")

	method, err := linkOrCopy(context.Background(), srcFile, dstFile)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		assert.Equal(t, "copy", method)
	} else {
		assert.Equal(t, "symlink", method)
	}

	// Verify destination exists
	_, statErr := os.Lstat(dstFile)
	require.NoError(t, statErr)
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source")
	content := []byte("test-content-for-copy")
	require.NoError(t, os.WriteFile(srcFile, content, 0o755))

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "dest")

	err := copyFile(srcFile, dstFile)
	require.NoError(t, err)

	// Verify content
	result, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, content, result)

	// Verify executable permission preserved
	fi, err := os.Stat(dstFile)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.True(t, fi.Mode()&0o111 != 0, "expected executable permissions")
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	t.Parallel()

	dstFile := filepath.Join(t.TempDir(), "dest")
	err := copyFile("/nonexistent/source", dstFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening source")
}

func TestCopyFile_DestinationCreationFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission test")
	}

	// Create a source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o755))

	// Create a read-only parent directory so destination creation fails
	parentDir := t.TempDir()
	readOnlyDir := filepath.Join(parentDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755)
	})

	dstFile := filepath.Join(readOnlyDir, "dest")
	err := copyFile(srcFile, dstFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating destination")
}

func TestRemoveAnalyzerDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create mixed directories
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.1.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.2.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "resource-aws-v4.0.0"), 0o755))

	err := removeAnalyzerDirs(dir)
	require.NoError(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "resource-aws-v4.0.0", entries[0].Name())
}
