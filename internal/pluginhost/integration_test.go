package pluginhost_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rshade/finfocus/internal/pluginhost"
)

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_ProcessLauncherWithClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test integration between ProcessLauncher and NewClient
	launcher := pluginhost.NewProcessLauncher()

	// Create a mock plugin that will fail but test the integration
	mockPlugin := createFailingMockPlugin(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// This should fail but test the full integration path
	client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)

	if err == nil {
		t.Log("Note: Mock plugin unexpectedly succeeded")
		if client != nil {
			client.Close()
		}
	} else {
		t.Logf("Expected integration failure (handled) for ProcessLauncher (plugin: %s): %v", mockPlugin, err)
	}

	// Should not panic or leave hanging processes
}

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_StdioLauncherWithClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test integration between StdioLauncher and NewClient
	launcher := pluginhost.NewStdioLauncher()

	// Create a mock plugin that will fail but test the integration
	mockPlugin := createFailingMockPlugin(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// This should fail but test the full integration path
	client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)

	if err == nil {
		t.Log("Note: Mock plugin unexpectedly succeeded")
		if client != nil {
			client.Close()
		}
	} else {
		t.Logf("Expected integration failure (handled) for StdioLauncher (plugin: %s): %v", mockPlugin, err)
	}

	// Should not panic or leave hanging processes
}

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_LauncherSwitching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test that the same binary path works with different launchers
	mockPlugin := createFailingMockPlugin(t)

	launchers := []struct {
		name     string
		launcher pluginhost.Launcher
	}{
		{"ProcessLauncher", pluginhost.NewProcessLauncher()},
		{"StdioLauncher", pluginhost.NewStdioLauncher()},
	}

	for _, launcher := range launchers {
		t.Run(launcher.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			client, err := pluginhost.NewClient(ctx, launcher.launcher, mockPlugin)

			if client != nil {
				client.Close()
			}

			// We expect errors with our mock plugin
			if err != nil {
				t.Logf("%s integration failure (expected/handled): %v", launcher.name, err)
			}
		})
	}
}

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_ConcurrentClients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test creating multiple clients concurrently
	launcher := pluginhost.NewProcessLauncher()

	const numClients = 5
	results := make(chan error, numClients)

	// Start multiple client creation attempts concurrently
	// Each goroutine gets its own mock plugin to avoid conflicts
	for range numClients {
		go func() {
			// Create a unique mock plugin for this client to avoid conflicts
			mockPlugin := createFailingMockPlugin(t)

			// Use shorter timeout for concurrent stress test
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)
			if client != nil {
				client.Close()
			}

			results <- err
		}()
	}

	// Collect results
	for i := range numClients {
		err := <-results
		if err != nil {
			t.Logf("Concurrent client %d failure (expected with mock plugin): %v", i, err)
		} else {
			t.Logf("Concurrent client %d unexpectedly succeeded", i)
		}
	}

	// Should not panic or deadlock
}

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_RapidCreateDestroy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test rapid creation and destruction of clients
	launcher := pluginhost.NewProcessLauncher()

	for i := range 10 {
		// Create a unique mock plugin for each iteration to avoid conflicts
		mockPlugin := createFailingMockPlugin(t)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)
		if client != nil {
			client.Close()
		}
		cancel()

		if err != nil {
			t.Logf(
				"Rapid create/destroy iteration %d failure (expected with mock plugin): %v",
				i,
				err,
			)
		} else {
			t.Logf("Rapid create/destroy iteration %d unexpectedly succeeded", i)
		}

		// Brief pause between iterations to allow cleanup
		time.Sleep(50 * time.Millisecond)
	}
}

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test that context cancellation works properly throughout the integration
	launcher := pluginhost.NewProcessLauncher()
	mockPlugin := createWorkingMockPlugin(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a brief delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)
	if client != nil {
		client.Close()
	}

	// Should handle cancellation gracefully by surfacing an error
	if err == nil {
		t.Fatalf("expected error from NewClient after context cancellation, got nil")
	}
}

//nolint:paralleltest // executes a file it just wrote; concurrent forks cause ETXTBSY (golang/go#22315)
func TestIntegration_PluginDirectoryStructure(t *testing.T) {
	// Test the plugin directory structure requirements
	tempDir := t.TempDir()

	// Create plugin directory structure: ~/.finfocus/plugins/<name>/<version>/
	pluginDir := filepath.Join(tempDir, "test-plugin", "v1.0.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin directory: %v", err)
	}

	// Create plugin binary
	binPath := filepath.Join(pluginDir, "test-plugin")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if err := os.WriteFile(binPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("failed to create plugin binary: %v", err)
	}

	// Test that plugin is detected as executable
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("plugin binary not found: %v", err)
	}

	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			t.Error("plugin binary is not executable")
		}
	} else {
		if filepath.Ext(binPath) != ".exe" {
			t.Error("Windows plugin doesn't have .exe extension")
		}
	}

	// Test with launcher (will fail but shouldn't panic)
	launcher := pluginhost.NewProcessLauncher()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := pluginhost.NewClient(ctx, launcher, binPath)
	if client != nil {
		client.Close()
	}

	if err != nil {
		t.Logf("Expected failure with test plugin binary (handled) (%s): %v", binPath, err)
	} else {
		t.Logf("Test plugin binary unexpectedly succeeded: %s", binPath)
	}
}

//nolint:paralleltest // subtests share the parent-scoped fixture launcher = pluginhost.NewProcessLauncher(...)
func TestIntegration_ErrorRecovery(t *testing.T) {
	// Test that the system recovers gracefully from various error conditions
	launcher := pluginhost.NewProcessLauncher()

	testCases := []struct {
		name     string
		binPath  string
		expected string
	}{
		{
			name:     "nonexistent_binary",
			binPath:  "/nonexistent/plugin/binary",
			expected: "starting plugin",
		},
		{
			name:     "invalid_path",
			binPath:  "",
			expected: "starting plugin",
		},
		{
			name:     "directory_instead_of_binary",
			binPath:  t.TempDir(),
			expected: "starting plugin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			client, err := pluginhost.NewClient(ctx, launcher, tc.binPath)
			if client != nil {
				client.Close()
			}

			if err == nil {
				t.Fatalf("expected error for %s (binPath: %s), got nil", tc.name, tc.binPath)
			} else {
				t.Logf("Expected failure for (handled) %s (binPath: %s): %v", tc.name, tc.binPath, err)
			}
		})
	}
}

// Helper functions for integration tests

func createFailingMockPlugin(t *testing.T) string {
	// Create a plugin that will start but fail to serve gRPC
	// For ProcessLauncher: try to bind to port but exit immediately
	// For StdioLauncher: just exit
	script := `#!/bin/bash
if [ "$1" = "--stdio" ]; then
    # Stdio mode - just exit
    exit 1
else
    # Process mode - try to bind to port briefly then exit
    PORT="${PORT:-${FINFOCUS_PLUGIN_PORT}}"
    if [ -n "$PORT" ]; then
        # Try to bind to port for a moment (will fail to serve gRPC)
        timeout 0.1 nc -l 127.0.0.1 "$PORT" 2>/dev/null || true
    fi
    exit 1
fi`
	if runtime.GOOS == "windows" {
		script = `if "%1"=="--stdio" (
    exit 1
) else (
    set PORT=%PORT%
    if "%PORT%"=="" set PORT=%FINFOCUS_PLUGIN_PORT%
    if defined PORT (
        timeout 1 >nul 2>nul
    )
    exit 1
)`
	}

	return createTestScript(t, script, ".sh")
}

func createWorkingMockPlugin(t *testing.T) string {
	// Create a plugin that will run but not serve gRPC
	// For ProcessLauncher: bind to port and keep running briefly
	// For StdioLauncher: keep stdin/stdout open briefly
	script := `#!/bin/bash
if [ "$1" = "--stdio" ]; then
    # Stdio mode - keep pipes open briefly then exit
    sleep 2
    exit 0
else
    # Process mode - bind to port and keep listening briefly
    PORT="${PORT:-${FINFOCUS_PLUGIN_PORT}}"
    if [ -n "$PORT" ]; then
        # Bind to port and keep listening for a short time
        timeout 2 nc -l 127.0.0.1 "$PORT" 2>/dev/null || sleep 2
    else
        sleep 2
    fi
    exit 0
fi`
	if runtime.GOOS == "windows" {
		script = `if "%1"=="--stdio" (
    timeout 2 >nul
    exit 0
) else (
    set PORT=%PORT%
    if "%PORT%"=="" set PORT=%FINFOCUS_PLUGIN_PORT%
    if defined PORT (
        timeout 2 >nul 2>nul
    ) else (
        timeout 2 >nul
    )
    exit 0
)`
	}

	return createTestScript(t, script, ".sh")
}

func createTestScript(t *testing.T, content, ext string) string {
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}

	tmpfile, err := os.CreateTemp(t.TempDir(), "integration-test-*"+ext)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpfile.Close()

	if _, writeErr := tmpfile.WriteString(content); writeErr != nil {
		t.Fatalf("failed to write script: %v", writeErr)
	}

	if chmodErr := tmpfile.Chmod(0755); chmodErr != nil {
		t.Fatalf("failed to make script executable: %v", chmodErr)
	}

	fileName := tmpfile.Name()

	// Clean up after test - ensure file is removed even if test fails
	t.Cleanup(func() {
		if removeErr := os.Remove(fileName); removeErr != nil && !os.IsNotExist(removeErr) {
			t.Logf("Warning: failed to cleanup test script %s: %v", fileName, removeErr)
		}
	})

	return fileName
}
