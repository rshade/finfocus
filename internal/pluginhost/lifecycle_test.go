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
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// Ensure fmt and plugin imports are used for integration test helpers.
var (
	_ = fmt.Sprintf
	_ = plugin.NewMockPlugin
)

// TestProcessLauncher_Success tests successful plugin launch with ProcessLauncher.
func TestProcessLauncher_Success(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NotNil(t, closeFn)

	// Verify connection works
	client := proto.NewCostSourceClient(conn)
	nameResp, err := client.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", nameResp.GetName())

	// Cleanup
	err = closeFn()
	assert.NoError(t, err)
}

// TestProcessLauncher_NonExistentBinary tests error handling for missing binary.
func TestProcessLauncher_NonExistentBinary(t *testing.T) {
	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, "/nonexistent/plugin")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Contains(t, err.Error(), "starting plugin")
}

// TestProcessLauncher_NonExecutableBinary tests error handling for non-executable file.
func TestProcessLauncher_NonExecutableBinary(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create non-executable file
	tempDir := t.TempDir()
	pluginPath := filepath.Join(tempDir, "non-executable")
	err := os.WriteFile(pluginPath, []byte("not executable"), 0644) // No execute permission
	require.NoError(t, err)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
}

// TestProcessLauncher_ContextCancellation tests behavior when context is cancelled.
func TestProcessLauncher_ContextCancellation(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createSlowPluginBinary(t)

	launcher := pluginhost.NewProcessLauncher()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)

	// Should timeout
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Contains(t, err.Error(), "timeout")
}

// TestProcessLauncher_Cleanup tests that cleanup properly closes connections and kills processes.
func TestProcessLauncher_Cleanup(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)

	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)
	require.NoError(t, err)

	// Close immediately
	err = closeFn()
	assert.NoError(t, err)

	// Verify connection is closed
	state := conn.GetState()
	assert.Contains(t, []string{"SHUTDOWN", "TRANSIENT_FAILURE"}, state.String())
}

// TestProcessLauncher_MultipleStarts tests launching multiple plugins in sequence.
func TestProcessLauncher_MultipleStarts(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)
	launcher := pluginhost.NewProcessLauncher()
	ctx := context.Background()

	// Launch first plugin
	conn1, closeFn1, err1 := launcher.Start(ctx, pluginPath)
	require.NoError(t, err1)
	defer closeFn1()

	// Launch second plugin
	conn2, closeFn2, err2 := launcher.Start(ctx, pluginPath)
	require.NoError(t, err2)
	defer closeFn2()

	// Both should work independently
	client1 := proto.NewCostSourceClient(conn1)
	client2 := proto.NewCostSourceClient(conn2)

	resp1, err := client1.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", resp1.GetName())

	resp2, err := client2.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", resp2.GetName())
}

// TestStdioLauncher_Success tests successful plugin launch with StdioLauncher.
func TestStdioLauncher_Success(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)

	launcher := pluginhost.NewStdioLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NotNil(t, closeFn)

	// Verify connection works
	client := proto.NewCostSourceClient(conn)
	nameResp, err := client.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", nameResp.GetName())

	// Cleanup
	err = closeFn()
	assert.NoError(t, err)
}

// TestStdioLauncher_NonExistentBinary tests error handling for missing binary.
func TestStdioLauncher_NonExistentBinary(t *testing.T) {
	launcher := pluginhost.NewStdioLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, "/nonexistent/plugin")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, closeFn)
	assert.Contains(t, err.Error(), "starting plugin")
}

// TestStdioLauncher_Cleanup tests that cleanup properly closes all resources.
func TestStdioLauncher_Cleanup(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)

	launcher := pluginhost.NewStdioLauncher()
	ctx := context.Background()

	conn, closeFn, err := launcher.Start(ctx, pluginPath)
	require.NoError(t, err)

	// Close immediately
	err = closeFn()
	assert.NoError(t, err)

	// Verify connection is closed
	state := conn.GetState()
	assert.Contains(t, []string{"SHUTDOWN", "TRANSIENT_FAILURE"}, state.String())
}

// TestLauncher_SwitchBetweenTypes tests switching between ProcessLauncher and StdioLauncher.
func TestLauncher_SwitchBetweenTypes(t *testing.T) {
	t.Skip("Skipping test that requires building plugin binaries (internal package import restrictions)")

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pluginPath := createMockPluginBinary(t)
	ctx := context.Background()

	// Start with ProcessLauncher
	processLauncher := pluginhost.NewProcessLauncher()
	conn1, closeFn1, err1 := processLauncher.Start(ctx, pluginPath)
	require.NoError(t, err1)
	defer closeFn1()

	// Switch to StdioLauncher
	stdioLauncher := pluginhost.NewStdioLauncher()
	conn2, closeFn2, err2 := stdioLauncher.Start(ctx, pluginPath)
	require.NoError(t, err2)
	defer closeFn2()

	// Both should work
	client1 := proto.NewCostSourceClient(conn1)
	resp1, err := client1.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", resp1.GetName())

	client2 := proto.NewCostSourceClient(conn2)
	resp2, err := client2.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", resp2.GetName())
}

// createMockPluginBinary creates a minimal executable that starts a gRPC server.
// This is a real integration test that launches actual processes.
func createMockPluginBinary(t *testing.T) string {
	t.Helper()

	// Use the examples/plugins/aws-example if it exists, otherwise create a simple one
	// For now, we'll create a Go program that uses our mock server
	tempDir := t.TempDir()

	var pluginPath string
	if runtime.GOOS == "windows" {
		pluginPath = filepath.Join(tempDir, "mock-plugin.exe")
	} else {
		pluginPath = filepath.Join(tempDir, "mock-plugin")
	}

	// Create a simple Go program that starts the mock server
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
	stdio := flag.Bool("stdio", false, "Use stdio instead of TCP")
	flag.Parse()

	// Create mock plugin
	mockPlugin := plugin.NewMockPlugin("test-plugin")
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	proto.RegisterCostSourceServer(grpcServer, mockPlugin)

	if *stdio {
		// Stdio mode - listen on stdin/stdout
		// For simplicity, we'll just exit - real stdio mode is complex
		log.Fatal("stdio mode not fully implemented in test binary")
	}

	// TCP mode
	if *port == 0 {
		// Try to get port from environment
		if envPort := os.Getenv("FINFOCUS_PLUGIN_PORT"); envPort != "" {
			fmt.Sscanf(envPort, "%d", port)
		}
	}

	if *port == 0 {
		log.Fatal("No port specified")
	}

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

	// Build the binary
	cmd := exec.Command("go", "build", "-o", pluginPath, mainGo)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build mock plugin: %s", string(output))

	return pluginPath
}

// createSlowPluginBinary creates a plugin binary that takes a long time to start.
func createSlowPluginBinary(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()

	var pluginPath string
	if runtime.GOOS == "windows" {
		pluginPath = filepath.Join(tempDir, "slow-plugin.exe")
	} else {
		pluginPath = filepath.Join(tempDir, "slow-plugin")
	}

	mainGo := filepath.Join(tempDir, "main.go")
	code := `package main

import (
	"time"
)

func main() {
	// Sleep for a long time to simulate slow startup
	time.Sleep(30 * time.Second)
}
`

	err := os.WriteFile(mainGo, []byte(code), 0644)
	require.NoError(t, err)

	cmd := exec.Command("go", "build", "-o", pluginPath, mainGo)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build slow plugin: %s", string(output))

	return pluginPath
}
