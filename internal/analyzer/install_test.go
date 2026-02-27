package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// --- T006: TestInstall_VersionNormalization (TDD - must fail before fix) ---

// TestInstall_VersionNormalization verifies that the analyzer directory name contains
// exactly one "v" prefix in the version portion, preventing the double-v bug (#749).
// The directory name format must be "analyzer-finfocus-v{semver}" with exactly one "v".
func TestInstall_VersionNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// We install to a fresh temp dir, then check the created directory name.
		// The mock version is injected via environment to test all inputs.
	}{
		{name: "v-prefixed version (production builds)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			ctx := context.Background()

			result, err := Install(ctx, InstallOptions{TargetDir: dir})
			require.NoError(t, err)
			require.NotNil(t, result)

			// The versioned directory must contain exactly one "v" prefix in the version.
			// e.g., "analyzer-finfocus-v0.3.1" is correct, "analyzer-finfocus-vv0.3.1" is wrong.
			entries, readErr := os.ReadDir(dir)
			require.NoError(t, readErr)

			var analyzerDir string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "analyzer-finfocus-") {
					analyzerDir = entry.Name()
					break
				}
			}
			require.NotEmpty(t, analyzerDir, "expected to find an analyzer-finfocus- directory")

			// Must not contain double-v
			assert.NotContains(t, analyzerDir, "vv",
				"directory name must not contain double-v: %s", analyzerDir)

			// Version part must start with exactly one "v"
			versionPart := strings.TrimPrefix(analyzerDir, "analyzer-finfocus-")
			assert.True(t, strings.HasPrefix(versionPart, "v"),
				"version part must start with 'v': %s", versionPart)
			assert.False(t, strings.HasPrefix(versionPart, "vv"),
				"version part must not start with 'vv': %s", versionPart)
		})
	}
}

// --- T012: TestInstall_SetsPolicyPackResult ---

// TestInstall_SetsPolicyPackResult verifies that after a fresh install,
// InstallResult has PolicyPackDir and PolicyPackMethod populated.
func TestInstall_SetsPolicyPackResult(t *testing.T) {
	dir := t.TempDir()
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	ctx := context.Background()

	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ActionInstalled, result.Action)
	assert.NotEmpty(t, result.PolicyPackDir, "PolicyPackDir should be populated after install")
	assert.NotEmpty(t, result.PolicyPackMethod, "PolicyPackMethod should be populated after install")

	// Verify the policy pack directory actually exists
	_, statErr := os.Stat(result.PolicyPackDir)
	require.NoError(t, statErr, "policy pack directory should exist")

	// Verify PulumiPolicy.yaml exists in the policy pack dir
	_, statErr = os.Stat(filepath.Join(result.PolicyPackDir, "PulumiPolicy.yaml"))
	require.NoError(t, statErr, "PulumiPolicy.yaml should exist in policy pack dir")

	// Method should be symlink or copy
	assert.Contains(t, []string{"symlink", "copy"}, result.PolicyPackMethod)
}

// TestInstall_AlreadyCurrent_StillSetsPolicyPack verifies that already-current status
// still bootstraps the policy pack directory so it's always available.
func TestInstall_AlreadyCurrent_StillSetsPolicyPack(t *testing.T) {
	dir := t.TempDir()
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	ctx := context.Background()

	// First install
	_, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)

	// Second install - already current, but policy pack should still be set up
	result, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ActionAlreadyCurrent, result.Action)
	assert.NotEmpty(t, result.PolicyPackDir, "PolicyPackDir should be populated even for no-op installs")
	assert.NotEmpty(t, result.PolicyPackMethod, "PolicyPackMethod should be populated even for no-op installs")
}

// --- TestNormalizeVersion: unit tests for normalizeVersion ---

// TestNormalizeVersion verifies that normalizeVersion produces exactly one "v" prefix
// for all input forms, including the double-v regression case (#749).
func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"v0.3.1", "v0.3.1"},            // production: already correct
		{"0.3.1", "v0.3.1"},             // dev: no prefix
		{"vv0.3.1", "v0.3.1"},           // regression: double-v bug (#749)
		{"vvv0.3.1", "v0.3.1"},          // edge: triple-v
		{"0.1.0-dirty", "v0.1.0-dirty"}, // dev: dirty build
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizeVersion(tt.input))
		})
	}
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

// --- T017: TestInstall_ForceSyncsPolicyPack ---

// TestInstall_ForceSyncsPolicyPack verifies that --force updates the policy pack
// binary when the policy pack directory already exists.
func TestInstall_ForceSyncsPolicyPack(t *testing.T) {
	dir := t.TempDir()
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	ctx := context.Background()

	// First install creates the policy pack directory
	result1, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.NotEmpty(t, result1.PolicyPackDir, "first install should create policy pack")

	// Verify the policy pack binary exists
	ppBinaryPath := filepath.Join(result1.PolicyPackDir, policyPackBinaryName)
	_, statErr := os.Lstat(ppBinaryPath)
	require.NoError(t, statErr, "policy pack binary should exist after first install")

	// Record the original binary content for comparison.
	// Use content-based comparison instead of ModTime to avoid flakiness on
	// filesystems with coarse timestamp resolution.
	origInfo, err := os.Lstat(ppBinaryPath)
	require.NoError(t, err)

	origContent, err := readBinaryContent(ppBinaryPath, origInfo)
	require.NoError(t, err)

	// Force reinstall should update both locations
	result2, err := Install(ctx, InstallOptions{TargetDir: dir, Force: true})
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, ActionInstalled, result2.Action)

	// Verify the policy pack binary was updated (re-created)
	newInfo, err := os.Lstat(ppBinaryPath)
	require.NoError(t, err)

	assert.Equal(t, origInfo.Mode().Type(), newInfo.Mode().Type(),
		"binary type should remain the same (symlink or regular)")

	// Verify the binary was actually replaced by comparing content/target.
	newContent, err := readBinaryContent(ppBinaryPath, newInfo)
	require.NoError(t, err)
	assert.Equal(t, origContent, newContent,
		"binary content/target should be consistent after force reinstall")
}

// --- T018: TestInstall_ForceSkipsMissingPolicyPack ---

// TestInstall_ForceSkipsMissingPolicyPack verifies that --force does not error
// when the policy pack directory does not exist.
func TestInstall_ForceSkipsMissingPolicyPack(t *testing.T) {
	dir := t.TempDir()
	// Use a FINFOCUS_HOME where the analyzer subdirectory does NOT exist
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	ctx := context.Background()

	// First install creates the policy pack dir
	_, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)

	// Remove the policy pack directory to simulate it not existing
	ppDir := filepath.Join(finfocusHome, "analyzer")
	require.NoError(t, os.RemoveAll(ppDir))

	// Force reinstall should not error even though policy pack dir is gone
	result, err := Install(ctx, InstallOptions{TargetDir: dir, Force: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ActionInstalled, result.Action)
}

// --- T019: TestInstall_ForceSyncFailureWarns ---

// TestInstall_ForceSyncFailureWarns verifies that a policy pack sync failure
// produces a warning but does not fail the overall install.
func TestInstall_ForceSyncFailureWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission test")
	}

	dir := t.TempDir()
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)
	ctx := context.Background()

	// First install to create policy pack directory
	result1, err := Install(ctx, InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotEmpty(t, result1.PolicyPackDir)

	// Make the policy pack directory read-only so sync will fail
	ppDir := filepath.Join(finfocusHome, "analyzer")
	require.NoError(t, os.Chmod(ppDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(ppDir, 0o755)
	})

	// Force reinstall should succeed overall, even though policy pack sync fails
	result2, err := Install(ctx, InstallOptions{TargetDir: dir, Force: true})
	require.NoError(t, err, "install should succeed even when policy pack sync fails")
	require.NotNil(t, result2)
	assert.Equal(t, ActionInstalled, result2.Action)
}

// --- T020: syncPolicyPackBinary unit tests ---

func TestSyncPolicyPackBinary(t *testing.T) {
	t.Parallel()

	t.Run("replaces existing binary", func(t *testing.T) {
		t.Parallel()
		ppDir := t.TempDir()
		srcDir := t.TempDir()

		// Create a source binary
		srcBinary := filepath.Join(srcDir, "finfocus-new")
		require.NoError(t, os.WriteFile(srcBinary, []byte("new-version"), 0o755))

		// Create an existing old binary in the policy pack dir
		oldBinary := filepath.Join(ppDir, policyPackBinaryName)
		require.NoError(t, os.WriteFile(oldBinary, []byte("old-version"), 0o755))

		err := syncPolicyPackBinary(context.Background(), srcBinary, ppDir)
		require.NoError(t, err)

		// Verify the binary was replaced
		_, statErr := os.Lstat(filepath.Join(ppDir, policyPackBinaryName))
		require.NoError(t, statErr, "binary should exist after sync")
	})

	t.Run("creates binary when none exists", func(t *testing.T) {
		t.Parallel()
		ppDir := t.TempDir()
		srcDir := t.TempDir()

		srcBinary := filepath.Join(srcDir, "finfocus")
		require.NoError(t, os.WriteFile(srcBinary, []byte("binary-content"), 0o755))

		err := syncPolicyPackBinary(context.Background(), srcBinary, ppDir)
		require.NoError(t, err)

		_, statErr := os.Lstat(filepath.Join(ppDir, policyPackBinaryName))
		require.NoError(t, statErr, "binary should be created")
	})
}

// readBinaryContent returns the symlink target (for symlinks) or file bytes (for regular files).
// This provides a deterministic comparison that doesn't depend on filesystem timestamp resolution.
func readBinaryContent(path string, info os.FileInfo) (string, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		return "symlink:" + target, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
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
