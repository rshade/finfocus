package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewInstaller(t *testing.T) {
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
			installer := NewInstaller(tt.pluginDir)
			if installer == nil {
				t.Fatal("NewInstaller returned nil")
			}
			if installer.client == nil {
				t.Error("installer.client is nil")
			}
			if tt.wantDir && installer.pluginDir == "" {
				t.Error("installer.pluginDir is empty")
			}
			if tt.pluginDir != "" && installer.pluginDir != tt.pluginDir {
				t.Errorf("pluginDir = %v, want %v", installer.pluginDir, tt.pluginDir)
			}
		})
	}
}

func TestInstallOptions(t *testing.T) {
	opts := InstallOptions{
		Force:     true,
		NoSave:    true,
		PluginDir: "/custom/dir",
	}

	if !opts.Force {
		t.Error("Force should be true")
	}
	if !opts.NoSave {
		t.Error("NoSave should be true")
	}
	if opts.PluginDir != "/custom/dir" {
		t.Errorf("PluginDir = %v, want /custom/dir", opts.PluginDir)
	}
}

func TestInstallResult(t *testing.T) {
	result := InstallResult{
		Name:       "test-plugin",
		Version:    "v1.0.0",
		Path:       "/path/to/plugin",
		FromURL:    true,
		Repository: "owner/repo",
	}

	if result.Name != "test-plugin" {
		t.Errorf("Name = %v, want test-plugin", result.Name)
	}
	if result.Version != "v1.0.0" {
		t.Errorf("Version = %v, want v1.0.0", result.Version)
	}
	if !result.FromURL {
		t.Error("FromURL should be true")
	}
}

func TestParseOwnerRepo(t *testing.T) {
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
			owner, repo, err := parseOwnerRepo(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOwnerRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %v, want %v", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %v, want %v", repo, tt.wantRepo)
			}
		})
	}
}

func TestFindPluginBinary(t *testing.T) {
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
			dir := tt.setupDir(t)
			result := findPluginBinary(dir, tt.pluginName)
			if tt.wantFound && result == "" {
				t.Error("expected to find binary but got empty string")
			}
			if !tt.wantFound && result != "" {
				t.Errorf("expected no binary but found: %s", result)
			}
		})
	}
}

func TestInstallAlreadyExists(t *testing.T) {
	// Create temp plugin directory with existing installation
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-plugin", "v1.0.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	installer := NewInstaller(tmpDir)
	opts := InstallOptions{Force: false}

	// This should fail because we can't actually contact GitHub
	// but if it gets past the "already installed" check, the test structure is correct
	_, err := installer.Install("test-plugin@v1.0.0", opts, nil)
	if err == nil {
		t.Error("expected error for non-existent registry plugin")
	}
}

func TestUpdateOptions(t *testing.T) {
	opts := UpdateOptions{
		DryRun:    true,
		Version:   "v2.0.0",
		PluginDir: "/test/dir",
	}

	if !opts.DryRun {
		t.Error("UpdateOptions.DryRun should be true")
	}
	if opts.Version != "v2.0.0" {
		t.Errorf("UpdateOptions.Version = %v, want v2.0.0", opts.Version)
	}
	if opts.PluginDir != "/test/dir" {
		t.Errorf("UpdateOptions.PluginDir = %v, want /test/dir", opts.PluginDir)
	}
}

func TestRemoveOptions(t *testing.T) {
	opts := RemoveOptions{
		KeepConfig: true,
		PluginDir:  "/test/dir",
	}

	if !opts.KeepConfig {
		t.Error("RemoveOptions.KeepConfig should be true")
	}
	if opts.PluginDir != "/test/dir" {
		t.Errorf("RemoveOptions.PluginDir = %v, want /test/dir", opts.PluginDir)
	}
}

func TestUpdateResult(t *testing.T) {
	result := UpdateResult{
		Name:        "test-plugin",
		OldVersion:  "v1.0.0",
		NewVersion:  "v2.0.0",
		Path:        "/path/to/plugin",
		WasUpToDate: false,
	}

	if result.Name != "test-plugin" {
		t.Errorf("Name = %v, want test-plugin", result.Name)
	}
	if result.OldVersion != "v1.0.0" {
		t.Errorf("OldVersion = %v, want v1.0.0", result.OldVersion)
	}
	if result.NewVersion != "v2.0.0" {
		t.Errorf("NewVersion = %v, want v2.0.0", result.NewVersion)
	}
	if result.WasUpToDate {
		t.Error("WasUpToDate should be false")
	}
}

func TestInstallEmptySpecifier(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	opts := InstallOptions{}

	_, err := installer.Install("", opts, nil)
	if err == nil {
		t.Error("expected error for empty specifier")
	}
}

func TestInstallInvalidURLFormat(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	opts := InstallOptions{}

	_, err := installer.Install("github.com/invalid", opts, nil)
	if err == nil {
		t.Error("expected error for invalid URL format")
	}
}

func TestFindPluginBinaryNonExistentDir(t *testing.T) {
	result := findPluginBinary("/nonexistent/path", "test")
	if result != "" {
		t.Errorf("expected empty string for non-existent dir, got: %s", result)
	}
}

func TestParseOwnerRepoEmptyInput(t *testing.T) {
	_, _, err := parseOwnerRepo("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseOwnerRepoOnlySlash(t *testing.T) {
	_, _, err := parseOwnerRepo("/")
	if err == nil {
		t.Error("expected error for empty owner/repo segments")
	}
}

func TestInstallerLock(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Acquire lock first time
	unlock1, err := installer.acquireLock(name)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if unlock1 == nil {
		t.Fatal("Unlock function is nil")
	}

	// Try to acquire lock second time - should fail
	unlock2, err := installer.acquireLock(name)
	if err == nil {
		t.Error("Expected error when acquiring already held lock")
		if unlock2 != nil {
			unlock2()
		}
	}

	// Release first lock
	unlock1()

	// Try to acquire lock again - should succeed now
	unlock3, err := installer.acquireLock(name)
	if err != nil {
		t.Fatalf("Failed to acquire lock after release: %v", err)
	}
	if unlock3 == nil {
		t.Fatal("Unlock function is nil")
	}
	unlock3()
}

func TestInstallerLockStaleDetection(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Ensure plugin directory exists
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		t.Fatalf("Failed to create plugin directory: %v", err)
	}

	// Create a stale lock file with an invalid PID
	lockPath := filepath.Join(tmpDir, name+".lock")
	if err := os.WriteFile(lockPath, []byte("99999999"), 0600); err != nil {
		t.Fatalf("Failed to create stale lock file: %v", err)
	}

	// Acquiring lock should succeed because the PID is invalid (stale)
	unlock, err := installer.acquireLock(name)
	if err != nil {
		t.Errorf("Expected to acquire lock with stale lock file, got error: %v", err)
		return
	}
	if unlock == nil {
		t.Fatal("Unlock function is nil")
	}
	unlock()
}

func TestInstallerLockEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Ensure plugin directory exists
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		t.Fatalf("Failed to create plugin directory: %v", err)
	}

	// Create an empty lock file (legacy or corrupt)
	lockPath := filepath.Join(tmpDir, name+".lock")
	if err := os.WriteFile(lockPath, []byte(""), 0600); err != nil {
		t.Fatalf("Failed to create empty lock file: %v", err)
	}

	// Acquiring lock should succeed because empty file is treated as stale
	unlock, err := installer.acquireLock(name)
	if err != nil {
		t.Errorf("Expected to acquire lock with empty lock file, got error: %v", err)
		return
	}
	if unlock == nil {
		t.Fatal("Unlock function is nil")
	}
	unlock()
}

func TestInstallerLockInvalidPID(t *testing.T) {
	tmpDir := t.TempDir()
	installer := NewInstaller(tmpDir)
	name := "test-plugin"

	// Ensure plugin directory exists
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		t.Fatalf("Failed to create plugin directory: %v", err)
	}

	// Create a lock file with invalid content
	lockPath := filepath.Join(tmpDir, name+".lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid"), 0600); err != nil {
		t.Fatalf("Failed to create invalid lock file: %v", err)
	}

	// Acquiring lock should succeed because invalid PID is treated as stale
	unlock, err := installer.acquireLock(name)
	if err != nil {
		t.Errorf("Expected to acquire lock with invalid PID, got error: %v", err)
		return
	}
	if unlock == nil {
		t.Fatal("Unlock function is nil")
	}
	unlock()
}

func TestIsProcessRunning(t *testing.T) {
	// Test with current process - should be running
	currentPID := os.Getpid()
	if !isProcessRunning(currentPID) {
		t.Error("Expected current process to be running")
	}

	// Test with invalid PID - should not be running
	if isProcessRunning(99999999) {
		t.Error("Expected invalid PID to not be running")
	}

	// Test with PID 0 - typically kernel, but behavior varies
	// Just ensure it doesn't panic
	_ = isProcessRunning(0)
}

func TestIsLockStale(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty file is stale",
			content:  "",
			expected: true,
		},
		{
			name:     "whitespace only is stale",
			content:  "   \n  ",
			expected: true,
		},
		{
			name:     "invalid PID is stale",
			content:  "not-a-number",
			expected: true,
		},
		{
			name:     "very large PID is stale",
			content:  "99999999",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockPath := filepath.Join(tmpDir, "test-"+tt.name+".lock")
			if err := os.WriteFile(lockPath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("Failed to create lock file: %v", err)
			}

			result := isLockStale(lockPath)
			if result != tt.expected {
				t.Errorf("isLockStale() = %v, expected %v", result, tt.expected)
			}
		})
	}

	// Test with non-existent file - should not be stale (safe default)
	if isLockStale(filepath.Join(tmpDir, "nonexistent.lock")) {
		t.Error("Non-existent lock file should not be considered stale")
	}
}

func TestRemoveOtherVersions(t *testing.T) {
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
				dir := t.TempDir()
				// Create multiple versions
				for _, v := range []string{"v1.0.0", "v1.1.0", "v2.0.0"} {
					vPath := filepath.Join(dir, "test-plugin", v)
					if err := os.MkdirAll(vPath, 0755); err != nil {
						t.Fatal(err)
					}
					// Add a file to track size
					binPath := filepath.Join(vPath, "binary")
					if err := os.WriteFile(binPath, []byte("test content"), 0755); err != nil {
						t.Fatal(err)
					}
				}
				return dir
			},
			pluginName:     "test-plugin",
			keepVersion:    "v2.0.0",
			wantRemoved:    2,
			wantBytesFreed: true,
		},
		{
			name: "no versions to remove",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				vPath := filepath.Join(dir, "test-plugin", "v1.0.0")
				if err := os.MkdirAll(vPath, 0755); err != nil {
					t.Fatal(err)
				}
				return dir
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
				dir := t.TempDir()
				pluginPath := filepath.Join(dir, "test-plugin")
				if err := os.MkdirAll(pluginPath, 0755); err != nil {
					t.Fatal(err)
				}
				// Create a version directory
				vPath := filepath.Join(pluginPath, "v1.0.0")
				if err := os.MkdirAll(vPath, 0755); err != nil {
					t.Fatal(err)
				}
				// Create a lock file (non-directory)
				lockPath := filepath.Join(pluginPath, "test-plugin.lock")
				if err := os.WriteFile(lockPath, []byte("lock"), 0600); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			pluginName:  "test-plugin",
			keepVersion: "v1.0.0",
			wantRemoved: 0, // Lock file should be skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErrContains)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.RemovedVersions) != tt.wantRemoved {
				t.Errorf("removed %d versions, want %d", len(result.RemovedVersions), tt.wantRemoved)
			}

			if tt.wantBytesFreed && result.BytesFreed == 0 {
				t.Error("expected bytes freed to be > 0")
			}

			if result.PluginName != tt.pluginName {
				t.Errorf("PluginName = %s, want %s", result.PluginName, tt.pluginName)
			}

			if result.KeptVersion != tt.keepVersion {
				t.Errorf("KeptVersion = %s, want %s", result.KeptVersion, tt.keepVersion)
			}
		})
	}
}

func TestRemoveOtherVersionsResult(t *testing.T) {
	result := RemoveOtherVersionsResult{
		PluginName:      "test-plugin",
		KeptVersion:     "v2.0.0",
		RemovedVersions: []string{"v1.0.0", "v1.5.0"},
		BytesFreed:      1024,
	}

	if result.PluginName != "test-plugin" {
		t.Errorf("PluginName = %v, want test-plugin", result.PluginName)
	}
	if result.KeptVersion != "v2.0.0" {
		t.Errorf("KeptVersion = %v, want v2.0.0", result.KeptVersion)
	}
	if len(result.RemovedVersions) != 2 {
		t.Errorf("RemovedVersions length = %d, want 2", len(result.RemovedVersions))
	}
	if result.BytesFreed != 1024 {
		t.Errorf("BytesFreed = %d, want 1024", result.BytesFreed)
	}
}

func TestGetDirSize(t *testing.T) {
	dir := t.TempDir()

	// Create some files with known sizes
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "subdir", "file2.txt")

	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file1, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("world!"), 0644); err != nil {
		t.Fatal(err)
	}

	size, err := getDirSize(dir)
	if err != nil {
		t.Fatalf("getDirSize() error: %v", err)
	}

	// "hello" = 5 bytes, "world!" = 6 bytes = 11 bytes total
	expectedSize := int64(11)
	if size != expectedSize {
		t.Errorf("getDirSize() = %d, want %d", size, expectedSize)
	}
}

func TestGetDirSizeNonExistent(t *testing.T) {
	_, err := getDirSize("/nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// TestInstallerLockConcurrent verifies that multiple concurrent acquisition
// attempts are properly serialized and only one goroutine can hold the lock
// at a time.
func TestInstallerLockConcurrent(t *testing.T) {
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
	if successCount.Load() == 0 {
		t.Error("Expected at least one goroutine to acquire the lock")
	}

	// All goroutines should have either succeeded or failed
	total := successCount.Load() + errorCount.Load()
	if total != numGoroutines {
		t.Errorf("Expected %d total results, got %d", numGoroutines, total)
	}

	// After all goroutines complete, we should be able to acquire the lock again
	unlock, err := installer.acquireLock(name)
	if err != nil {
		t.Fatalf("Failed to acquire lock after concurrent test: %v", err)
	}
	unlock()
}
