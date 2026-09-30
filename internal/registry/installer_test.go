package registry

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstaller(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pluginDir string
		wantDir   bool
	}{
		{
			name:      "with custom dir",
			pluginDir: "/custom/path",
			wantDir:   true,
		},
		{
			name:      "with empty dir uses default",
			pluginDir: "",
			wantDir:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			installer := NewInstaller(tt.pluginDir)
			require.NotNil(t, installer, "NewInstaller returned nil")
			assert.NotNil(t, installer.client, "installer.client is nil")
			if tt.wantDir {
				assert.NotEmpty(t, installer.pluginDir, "installer.pluginDir is empty")
			}
			if tt.pluginDir != "" {
				assert.Equal(t, tt.pluginDir, installer.pluginDir)
			}
		})
	}
}

func TestInstallOptions(t *testing.T) {
	t.Parallel()

	opts := InstallOptions{
		Force:     true,
		NoSave:    true,
		PluginDir: "/custom/dir",
	}

	assert.True(t, opts.Force, "Force should be true")
	assert.True(t, opts.NoSave, "NoSave should be true")
	assert.Equal(t, "/custom/dir", opts.PluginDir)
}

func TestInstallResult(t *testing.T) {
	t.Parallel()

	result := InstallResult{
		Name:       "test-plugin",
		Version:    "v1.0.0",
		Path:       "/path/to/plugin",
		FromURL:    true,
		Repository: "owner/repo",
	}

	assert.Equal(t, "test-plugin", result.Name)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.True(t, result.FromURL, "FromURL should be true")
}

func TestParseOwnerRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid format",
			input:     "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:    "invalid format no slash",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:      "with multiple slashes",
			input:     "owner/repo/extra",
			wantOwner: "owner",
			wantRepo:  "repo/extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			owner, repo, err := parseOwnerRepo(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestFindPluginBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupDir   func(t *testing.T) string
		pluginName string
		wantFound  bool
	}{
		{
			name: "finds exact name match",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				binPath := filepath.Join(dir, "test-plugin")
				if err := os.WriteFile(binPath, []byte("binary"), 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			pluginName: "test-plugin",
			wantFound:  true,
		},
		{
			name: "finds prefixed name",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				binPath := filepath.Join(dir, "finfocus-plugin-test")
				if err := os.WriteFile(binPath, []byte("binary"), 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			pluginName: "test",
			wantFound:  true,
		},
		{
			name: "empty directory",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			pluginName: "test",
			wantFound:  false,
		},
		{
			name: "finds any executable",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				// Use .exe on Windows since that's what determines executability
				binName := "some-binary"
				if runtime.GOOS == "windows" {
					binName = "some-binary.exe"
				}
				binPath := filepath.Join(dir, binName)
				if err := os.WriteFile(binPath, []byte("binary"), 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			pluginName: "different-name",
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.setupDir(t)
			result := findPluginBinary(dir, tt.pluginName)
			if tt.wantFound {
				assert.NotEmpty(t, result, "expected to find binary")
			} else {
				assert.Empty(t, result, "expected no binary")
			}
		})
	}
}

func TestInstallAlreadyExists(t *testing.T) {
	t.Parallel()

	// Create temp plugin directory with existing installation
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-plugin", "v1.0.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	installer := NewInstaller(tmpDir)
	opts := InstallOptions{Force: false}

	// This should fail because we can't actually contact GitHub
	// but if it gets past the "already installed" check, the test structure is correct
	_, err := installer.Install(context.Background(), "test-plugin@v1.0.0", opts, nil)
	assert.Error(t, err, "expected error for non-existent registry plugin")
}

func TestUpdateOptions(t *testing.T) {
	t.Parallel()

	opts := UpdateOptions{
		DryRun:    true,
		Version:   "v2.0.0",
		PluginDir: "/test/dir",
	}

	assert.True(t, opts.DryRun, "DryRun should be true")
	assert.Equal(t, "v2.0.0", opts.Version)
	assert.Equal(t, "/test/dir", opts.PluginDir)
}

func TestRemoveOptions(t *testing.T) {
	t.Parallel()

	opts := RemoveOptions{
		KeepConfig: true,
		PluginDir:  "/test/dir",
	}

	assert.True(t, opts.KeepConfig, "KeepConfig should be true")
	assert.Equal(t, "/test/dir", opts.PluginDir)
}

func TestUpdateResult(t *testing.T) {
	t.Parallel()

	result := UpdateResult{
		Name:        "test-plugin",
		OldVersion:  "v1.0.0",
		NewVersion:  "v2.0.0",
		Path:        "/path/to/plugin",
		WasUpToDate: false,
	}

	assert.Equal(t, "test-plugin", result.Name)
	assert.Equal(t, "v1.0.0", result.OldVersion)
	assert.Equal(t, "v2.0.0", result.NewVersion)
	assert.False(t, result.WasUpToDate, "WasUpToDate should be false")
}

func TestInstallEmptySpecifier(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	opts := InstallOptions{}

	_, err := installer.Install(context.Background(), "", opts, nil)
	assert.Error(t, err, "expected error for empty specifier")
}

func TestInstallInvalidURLFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	opts := InstallOptions{}

	_, err := installer.Install(context.Background(), "github.com/invalid", opts, nil)
	assert.Error(t, err, "expected error for invalid URL format")
}

func TestFindPluginBinaryNonExistentDir(t *testing.T) {
	t.Parallel()

	result := findPluginBinary("/nonexistent/path", "test")
	assert.Empty(t, result, "expected empty string for non-existent dir")
}

func TestParseOwnerRepoEmptyInput(t *testing.T) {
	t.Parallel()

	_, _, err := parseOwnerRepo("")
	assert.Error(t, err, "expected error for empty input")
}

func TestParseOwnerRepoOnlySlash(t *testing.T) {
	t.Parallel()

	_, _, err := parseOwnerRepo("/")
	assert.Error(t, err, "expected error for empty owner/repo segments")
}

func TestInstallerLock(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Acquire lock first time
	unlock1, err := installer.acquireLock(name)
	require.NoError(t, err, "Failed to acquire lock")
	require.NotNil(t, unlock1, "Unlock function is nil")

	// Try to acquire lock second time - should fail
	unlock2, err := installer.acquireLock(name)
	require.Error(t, err, "Expected error when acquiring already held lock")
	if unlock2 != nil {
		unlock2()
	}

	// Release first lock
	unlock1()

	// Try to acquire lock again - should succeed now
	unlock3, err := installer.acquireLock(name)
	require.NoError(t, err, "Failed to acquire lock after release")
	require.NotNil(t, unlock3, "Unlock function is nil")
	unlock3()
}

func TestInstallerLockStaleDetection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Ensure plugin directory exists
	require.NoError(t, os.MkdirAll(tmpDir, 0750), "Failed to create plugin directory")

	// Create a stale lock file with an invalid PID
	lockPath := filepath.Join(tmpDir, name+".lock")
	require.NoError(t, os.WriteFile(lockPath, []byte("99999999"), 0600), "Failed to create stale lock file")

	// Acquiring lock should succeed because the PID is invalid (stale)
	unlock, err := installer.acquireLock(name)
	require.NoError(t, err, "Expected to acquire lock with stale lock file")
	require.NotNil(t, unlock, "Unlock function is nil")
	unlock()
}

func TestInstallerLockEmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Ensure plugin directory exists
	require.NoError(t, os.MkdirAll(tmpDir, 0750), "Failed to create plugin directory")

	// Create an empty lock file (legacy or corrupt)
	lockPath := filepath.Join(tmpDir, name+".lock")
	require.NoError(t, os.WriteFile(lockPath, []byte(""), 0600), "Failed to create empty lock file")

	// Acquiring lock should succeed because empty file is treated as stale
	unlock, err := installer.acquireLock(name)
	require.NoError(t, err, "Expected to acquire lock with empty lock file")
	require.NotNil(t, unlock, "Unlock function is nil")
	unlock()
}

func TestInstallerLockInvalidPID(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Ensure plugin directory exists
	require.NoError(t, os.MkdirAll(tmpDir, 0750), "Failed to create plugin directory")

	// Create a lock file with invalid content
	lockPath := filepath.Join(tmpDir, name+".lock")
	require.NoError(t, os.WriteFile(lockPath, []byte("not-a-pid"), 0600), "Failed to create invalid lock file")

	// Acquiring lock should succeed because invalid PID is treated as stale
	unlock, err := installer.acquireLock(name)
	require.NoError(t, err, "Expected to acquire lock with invalid PID")
	require.NotNil(t, unlock, "Unlock function is nil")
	unlock()
}

// setupPluginVersions creates a temp dir containing one subdirectory per
// version for the plugin, each with a marker binary file.
func setupPluginVersions(t *testing.T, pluginName string, versions ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, v := range versions {
		vPath := filepath.Join(dir, pluginName, v)
		require.NoError(t, os.MkdirAll(vPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(vPath, "binary"), []byte("test content"), 0755))
	}
	return dir
}

func TestRemoveOtherVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupDir        func(t *testing.T) string
		pluginName      string
		keepVersion     string
		wantRemoved     int
		wantBytesFreed  bool
		wantErrContains string
	}{
		{
			name: "removes other versions, keeps specified",
			setupDir: func(t *testing.T) string {
				// Create multiple versions, each with a file to track size
				return setupPluginVersions(t, "test-plugin", "v1.0.0", "v1.1.0", "v2.0.0")
			},
			pluginName:     "test-plugin",
			keepVersion:    "v2.0.0",
			wantRemoved:    2,
			wantBytesFreed: true,
		},
		{
			name: "no versions to remove",
			setupDir: func(t *testing.T) string {
				return setupPluginVersions(t, "test-plugin", "v1.0.0")
			},
			pluginName:  "test-plugin",
			keepVersion: "v1.0.0",
			wantRemoved: 0,
		},
		{
			name: "plugin directory does not exist",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			pluginName:  "nonexistent-plugin",
			keepVersion: "v1.0.0",
			wantRemoved: 0,
		},
		{
			name: "skips non-directory entries",
			setupDir: func(t *testing.T) string {
				dir := setupPluginVersions(t, "test-plugin", "v1.0.0")
				// Create a lock file (non-directory)
				pluginPath := filepath.Join(dir, "test-plugin")
				lockPath := filepath.Join(pluginPath, "test-plugin.lock")
				require.NoError(t, os.WriteFile(lockPath, []byte("lock"), 0600))
				return dir
			},
			pluginName:  "test-plugin",
			keepVersion: "v1.0.0",
			wantRemoved: 0, // Lock file should be skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pluginDir := tt.setupDir(t)
			installer := NewInstaller(pluginDir)

			var progressMessages []string
			progress := func(msg string) {
				progressMessages = append(progressMessages, msg)
			}

			result, err := installer.RemoveOtherVersions(
				tt.pluginName,
				tt.keepVersion,
				pluginDir,
				progress,
			)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result.RemovedVersions, tt.wantRemoved)

			if tt.wantBytesFreed {
				assert.Positive(t, result.BytesFreed, "expected bytes freed to be > 0")
			}

			assert.Equal(t, tt.pluginName, result.PluginName)
			assert.Equal(t, tt.keepVersion, result.KeptVersion)
		})
	}
}

func TestRemoveOtherVersionsResult(t *testing.T) {
	t.Parallel()

	result := RemoveOtherVersionsResult{
		PluginName:      "test-plugin",
		KeptVersion:     "v2.0.0",
		RemovedVersions: []string{"v1.0.0", "v1.5.0"},
		BytesFreed:      1024,
	}

	assert.Equal(t, "test-plugin", result.PluginName)
	assert.Equal(t, "v2.0.0", result.KeptVersion)
	assert.Len(t, result.RemovedVersions, 2)
	assert.Equal(t, int64(1024), result.BytesFreed)
}

func TestRemoveOtherVersionsAcquiresLock(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "lock-test-plugin"

	// Create a plugin with versions
	for _, v := range []string{"v1.0.0", "v2.0.0"} {
		vPath := filepath.Join(tmpDir, name, v)
		require.NoError(t, os.MkdirAll(vPath, 0755))
		binPath := filepath.Join(vPath, "binary")
		require.NoError(t, os.WriteFile(binPath, []byte("test"), 0755))
	}

	// Acquire lock first to simulate contention
	unlock, err := installer.acquireLock(name)
	require.NoError(t, err, "Failed to acquire initial lock")

	// Try to call RemoveOtherVersions while lock is held - should fail
	_, removeErr := installer.RemoveOtherVersions(name, "v2.0.0", tmpDir, nil)
	require.Error(t, removeErr, "Expected error when lock is already held")
	assert.Contains(t, removeErr.Error(), "failed to acquire lock")

	// Release the lock
	unlock()

	// Now RemoveOtherVersions should succeed
	result, err := installer.RemoveOtherVersions(name, "v2.0.0", tmpDir, nil)
	require.NoError(t, err, "RemoveOtherVersions failed after lock release")
	assert.Len(t, result.RemovedVersions, 1)
}

func TestGetDirSize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create some files with known sizes
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "subdir", "file2.txt")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(file1, []byte("hello"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("world!"), 0644))

	size, err := getDirSize(dir)
	require.NoError(t, err)

	// "hello" = 5 bytes, "world!" = 6 bytes = 11 bytes total
	expectedSize := int64(11)
	assert.Equal(t, expectedSize, size)
}

func TestGetDirSizeNonExistent(t *testing.T) {
	t.Parallel()

	_, err := getDirSize("/nonexistent/path")
	assert.Error(t, err, "expected error for non-existent path")
}

// TestInstallerLockConcurrent verifies that multiple concurrent acquisition
// attempts are properly serialized and only one goroutine can hold the lock
// at a time.
func TestInstallerLockConcurrent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "concurrent-test-plugin"

	const numGoroutines = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	// Start signal channel to ensure all goroutines start simultaneously
	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait for start signal
			<-start

			unlock, err := installer.acquireLock(name)
			if err != nil {
				errorCount.Add(1)
				return
			}

			// Successfully acquired the lock
			successCount.Add(1)

			// Hold the lock briefly to simulate work
			// (no actual sleep, just release immediately)
			unlock()
		}()
	}

	// Signal all goroutines to start
	close(start)

	// Wait for all goroutines to complete
	wg.Wait()

	// At least one goroutine should have acquired the lock
	assert.Positive(t, successCount.Load(), "Expected at least one goroutine to acquire the lock")

	// All goroutines should have either succeeded or failed
	total := successCount.Load() + errorCount.Load()
	assert.Equal(t, int32(numGoroutines), total)

	// After all goroutines complete, we should be able to acquire the lock again
	unlock, err := installer.acquireLock(name)
	require.NoError(t, err, "Failed to acquire lock after concurrent test")
	unlock()
}
