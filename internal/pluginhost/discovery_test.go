package pluginhost_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// Ensure plugin import is used for integration test helpers.
var _ = plugin.NewMockPlugin

// TestLauncherInterface_ProcessLauncher tests that ProcessLauncher implements Launcher interface.
func TestLauncherInterface_ProcessLauncher(t *testing.T) {
	var _ pluginhost.Launcher = (*pluginhost.ProcessLauncher)(nil)
}

// TestLauncherInterface_StdioLauncher tests that StdioLauncher implements Launcher interface.
func TestLauncherInterface_StdioLauncher(t *testing.T) {
	var _ pluginhost.Launcher = (*pluginhost.StdioLauncher)(nil)
}

// TestPortAllocation_Uniqueness tests that ProcessLauncher allocates unique ports.
func TestPortAllocation_Uniqueness(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createPortReportingPlugin(t)
	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	// Launch multiple plugins and collect their ports
	var ports []int
	var cleanups []func() error

	for i := 0; i < 5; i++ {
		conn, closeFn, err := launcher.Start(ctx, pluginPath)
		require.NoError(t, err, "Failed to start plugin %d", i)

		cleanups = append(cleanups, closeFn)

		// Extract port from connection (this is a bit hacky but works for testing)
		addr := conn.Target()
		var port int
		_, err = fmt.Sscanf(addr, "127.0.0.1:%d", &port)
		if err == nil && port > 0 {
			ports = append(ports, port)
		}
	}

	// Cleanup all
	for _, closeFn := range cleanups {
		closeFn()
	}

	// Verify all ports are unique
	assert.GreaterOrEqual(t, len(ports), 3, "Should have captured at least 3 ports")
	portSet := make(map[int]bool)
	for _, port := range ports {
		assert.False(t, portSet[port], "Port %d was allocated twice", port)
		portSet[port] = true
	}
}

// TestConnectionRetry_Success tests that launcher retries connection attempts.
func TestConnectionRetry_Success(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a plugin that takes a moment to start listening
	pluginPath := createDelayedStartPlugin(t, 500*time.Millisecond)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	start := time.Now()
	conn, closeFn, err := launcher.Start(ctx, pluginPath)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, conn)
	defer closeFn()

	// Should have waited for the plugin to start
	assert.Greater(t, elapsed, 400*time.Millisecond, "Should have waited for delayed start")
	assert.Less(t, elapsed, 10*time.Second, "Should not have timed out")

	// Verify connection works
	assert.NotNil(t, conn, "Connection should be established")
}

// TestConnectionRetry_Timeout tests that launcher times out after max retries.
func TestConnectionRetry_Timeout(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a plugin that never starts listening
	pluginPath := createNonListeningPlugin(t)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	start := time.Now()
	conn, closeFn, err := launcher.Start(ctx, pluginPath)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Contains(t, err.Error(), "timeout")
	assert.GreaterOrEqual(t, elapsed, 9*time.Second, "Should have retried for ~10 seconds")
}

// TestEnvironmentVariables_PortPassing tests that port is passed via environment variable.
func TestEnvironmentVariables_PortPassing(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createEnvCheckingPlugin(t)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)

	require.NoError(t, err)
	require.NotNil(t, conn)
	defer closeFn()

	// If we get here, the plugin successfully read the environment variable
	// (createEnvCheckingPlugin creates a plugin that exits if FINFOCUS_PLUGIN_PORT is not set)
	assert.NotNil(t, conn, "Connection should be established")
}

// TestConnectionState_Ready tests that connection reaches Ready or Idle state.
func TestConnectionState_Ready(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)
	require.NoError(t, err)
	defer closeFn()

	// Check connection state
	state := conn.GetState()
	validStates := []string{"READY", "IDLE", "CONNECTING"}
	assert.Contains(t, validStates, state.String(),
		"Connection should be in valid state, got: %s", state.String())

	// Try to reach Ready state
	ctx2, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for state.String() != "READY" && ctx2.Err() == nil {
		if state.String() == "IDLE" {
			break // Idle is acceptable
		}
		conn.WaitForStateChange(ctx2, state)
		state = conn.GetState()
	}

	finalState := conn.GetState()
	acceptableStates := []string{"READY", "IDLE"}
	assert.Contains(t, acceptableStates, finalState.String(),
		"Connection should reach Ready or Idle state, got: %s", finalState.String())
}

// TestBinaryValidation_Permissions tests handling of non-executable binaries.
func TestBinaryValidation_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tempDir := t.TempDir()
	pluginPath := filepath.Join(tempDir, "non-executable")

	// Create file without execute permission
	err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0644)
	require.NoError(t, err)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Contains(t, err.Error(), "permission denied")
}

// TestLauncherCreation_ProcessLauncher tests ProcessLauncher constructor.
func TestLauncherCreation_ProcessLauncher(t *testing.T) {
	launcher := pluginhost.NewProcessLauncher()
	assert.NotNil(t, launcher)
}

// TestLauncherCreation_StdioLauncher tests StdioLauncher constructor.
func TestLauncherCreation_StdioLauncher(t *testing.T) {
	launcher := pluginhost.NewStdioLauncher()
	assert.NotNil(t, launcher)
}

// TestProcessCleanup_ZombieProcesses tests that processes are properly reaped.
func TestProcessCleanup_ZombieProcesses(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)
	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	// Start and immediately close multiple plugins
	for i := 0; i < 10; i++ {
		_, closeFn, err := launcher.Start(ctx, pluginPath)
		require.NoError(t, err, "Failed to start plugin %d", i)

		// Close immediately
		err = closeFn()
		assert.NoError(t, err, "Failed to close plugin %d", i)
	}

	// Wait a moment for processes to be reaped
	time.Sleep(500 * time.Millisecond)

	// Check for zombie processes (this is platform-specific)
	// On Unix, we can check ps output
	if runtime.GOOS != "windows" {
		cmd := exec.Command("sh", "-c", "ps aux | grep -c '[m]ock-plugin.*defunct' || true")
		output, err := cmd.Output()
		require.NoError(t, err)

		zombieCount := 0
		fmt.Sscanf(string(output), "%d", &zombieCount)
		assert.Equal(t, 0, zombieCount, "Should have no zombie processes")
	}
}

// TestConcurrentConnections tests multiple simultaneous plugin connections.
func TestConcurrentConnections(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)
	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	const concurrency = 5
	results := make(chan error, concurrency)

	// Launch plugins concurrently
	for i := 0; i < concurrency; i++ {
		go func(_ int) {
			_, closeFn, err := launcher.Start(ctx, pluginPath)
			if err != nil {
				results <- err
				return
			}
			defer closeFn()

			// Quick verification
			time.Sleep(100 * time.Millisecond)
			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < concurrency; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent connection %d should succeed", i)
	}
}

// Helper functions for creating specialized test plugins

func createPortReportingPlugin(t *testing.T) string {
	t.Helper()
	// For simplicity, use the same mock plugin - it will use whatever port is allocated
	return createMockPluginBinary(t)
}

func createDelayedStartPlugin(t *testing.T, delay time.Duration) string {
	t.Helper()

	tempDir := t.TempDir()
	var pluginPath string
	if runtime.GOOS == "windows" {
		pluginPath = filepath.Join(tempDir, "delayed-plugin.exe")
	} else {
		pluginPath = filepath.Join(tempDir, "delayed-plugin")
	}

	mainGo := filepath.Join(tempDir, "main.go")
	code := fmt.Sprintf(`package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
	"google.golang.org/grpc"
)

func main() {
	port := flag.Int("port", 0, "TCP port to listen on")
	flag.Parse()

	if *port == 0 {
		if envPort := os.Getenv("FINFOCUS_PLUGIN_PORT"); envPort != "" {
			fmt.Sscanf(envPort, "%%d", port)
		}
	}

	// Delay before starting to listen
	time.Sleep(%d * time.Millisecond)

	mockPlugin := plugin.NewMockPlugin("test-plugin")
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	grpcServer := grpc.NewServer()
	proto.RegisterCostSourceServer(grpcServer, mockPlugin)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %%v", err)
	}

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %%v", err)
	}
}
`, delay.Milliseconds())

	err := os.WriteFile(mainGo, []byte(code), 0644)
	require.NoError(t, err)

	cmd := exec.Command("go", "build", "-o", pluginPath, mainGo)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build delayed plugin: %s", string(output))

	return pluginPath
}

func createNonListeningPlugin(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	var pluginPath string
	if runtime.GOOS == "windows" {
		pluginPath = filepath.Join(tempDir, "non-listening.exe")
	} else {
		pluginPath = filepath.Join(tempDir, "non-listening")
	}

	mainGo := filepath.Join(tempDir, "main.go")
	code := `package main

import (
	"time"
)

func main() {
	// Just sleep without listening on the port
	time.Sleep(30 * time.Second)
}
`

	err := os.WriteFile(mainGo, []byte(code), 0644)
	require.NoError(t, err)

	cmd := exec.Command("go", "build", "-o", pluginPath, mainGo)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build non-listening plugin: %s", string(output))

	return pluginPath
}

func createEnvCheckingPlugin(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	var pluginPath string
	if runtime.GOOS == "windows" {
		pluginPath = filepath.Join(tempDir, "env-check.exe")
	} else {
		pluginPath = filepath.Join(tempDir, "env-check")
	}

	mainGo := filepath.Join(tempDir, "main.go")
	code := `package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
	"google.golang.org/grpc"
)

func main() {
	port := flag.Int("port", 0, "TCP port to listen on")
	flag.Parse()

	// Check if environment variable is set
	envPort := os.Getenv("FINFOCUS_PLUGIN_PORT")
	if envPort == "" {
		log.Fatal("FINFOCUS_PLUGIN_PORT environment variable not set")
	}

	if *port == 0 {
		fmt.Sscanf(envPort, "%d", port)
	}

	mockPlugin := plugin.NewMockPlugin("test-plugin")
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	grpcServer := grpc.NewServer()
	proto.RegisterCostSourceServer(grpcServer, mockPlugin)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
`

	err := os.WriteFile(mainGo, []byte(code), 0644)
	require.NoError(t, err)

	cmd := exec.Command("go", "build", "-o", pluginPath, mainGo)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build env-checking plugin: %s", string(output))

	return pluginPath
}
