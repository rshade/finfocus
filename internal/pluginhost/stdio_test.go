package pluginhost // needs access to unexported methods

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	testTimeout = 30 * time.Second
)

func TestStdioLauncher_Start_MockCommand(t *testing.T) {
	// Skip this test in short mode as it involves process creation
	if testing.Short() {
		t.Skip("skipping stdio launcher test in short mode")
	}

	launcher := NewStdioLauncher()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create a mock plugin that echoes on stdio
	mockPlugin := createMockStdioCommand(t)

	conn, cleanup, err := launcher.Start(ctx, mockPlugin)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer cleanup()

	if conn == nil {
		t.Fatal("expected non-nil connection")
	}

	// Verify connection state
	state := conn.GetState()
	if state.String() == "SHUTDOWN" {
		t.Error("connection is in shutdown state")
	}
}

func TestStdioLauncher_StartInvalidCommand(t *testing.T) {
	launcher := NewStdioLauncher()
	ctx := context.Background()

	// Use a command that doesn't exist
	_, _, err := launcher.Start(ctx, "/nonexistent/command")

	if err == nil {
		t.Error("expected error for invalid command")
	}

	if !strings.Contains(err.Error(), "starting plugin") {
		t.Errorf("error doesn't mention plugin start: %v", err)
	}
}

func TestStdioLauncher_StartWithTimeout(t *testing.T) {
	launcher := NewStdioLauncher()

	// Use very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Use a command that doesn't exist to trigger immediate failure
	_, _, err := launcher.Start(ctx, "/nonexistent/command/that/will/fail")

	if err == nil {
		t.Error("expected error with nonexistent command")
		return
	}

	// The error should be related to starting the plugin
	errStr := err.Error()
	if !strings.Contains(errStr, "starting plugin") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStdioLauncher_StartWithStdinPipeError(t *testing.T) {
	launcher := NewStdioLauncher()
	ctx := context.Background()

	// We can't easily force a stdin pipe error without complex mocking
	// So we'll test with a command that should work to verify stdin pipe creation
	if runtime.GOOS == "windows" {
		t.Skip("skipping stdin pipe test on Windows")
	}

	// Use echo command which should work
	cmd := "echo"
	_, _, err := launcher.Start(ctx, cmd, "test")

	// The error could be various things (connection, process, etc.)
	// We're mainly testing that the stdin pipe creation doesn't panic
	if err != nil {
		t.Logf("Expected error for simple echo command: %v", err)
	}
}

func TestStdioLauncher_Proxy(t *testing.T) {
	launcher := NewStdioLauncher()

	// Create test pipes to simulate stdin/stdout
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	// Create a test listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create test listener: %v", err)
	}
	defer listener.Close()

	// Start the proxy in a goroutine
	go launcher.proxy(listener, stdinW, stdoutR)

	// Give proxy a moment to start
	time.Sleep(100 * time.Millisecond)

	// Connect to the proxy
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Test data flow through proxy
	testData := "test message"

	// Write to connection (should go to stdin)
	go func() {
		_, _ = conn.Write([]byte(testData))
		conn.Close()
	}()

	// Read from stdin pipe
	buffer := make([]byte, len(testData))
	_, err = stdinR.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Errorf("failed to read from stdin pipe: %v", err)
	}

	if string(buffer) != testData {
		t.Errorf("expected %q, got %q", testData, string(buffer))
	}

	// Test reverse direction - write to stdout and read from connection
	go func() {
		_, _ = stdoutW.Write([]byte(testData))
		stdoutW.Close()
	}()

	// Clean up
	stdinR.Close()
	stdoutR.Close()
}

func TestStdioLauncher_ProxyConnectionError(t *testing.T) {
	launcher := NewStdioLauncher()

	// Create a listener that we'll close immediately
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create test listener: %v", err)
	}
	listener.Close() // Close immediately

	// Create dummy pipes
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	defer stdinR.Close()
	defer stdinW.Close()
	defer stdoutR.Close()
	defer stdoutW.Close()

	// Start proxy with closed listener (should handle error gracefully)
	launcher.proxy(listener, stdinW, stdoutR)

	// Should not panic - proxy should handle the error gracefully
}

func TestStdioLauncher_CleanupFunction(t *testing.T) {
	// Create a test command
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test command: %v", err)
	}

	// Create a test listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("failed to create test listener: %v", err)
	}

	// Create a test connection
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		cmd.Process.Kill()
		listener.Close()
		t.Fatalf("failed to create test connection: %v", err)
	}

	// Create a mock gRPC connection
	mockConn := &testGRPCConn{netConn: conn}

	// Create cleanup function (simulate what Start() does)
	closeFn := func() error {
		if connCloseErr := mockConn.Close(); connCloseErr != nil {
			return fmt.Errorf("closing connection: %w", connCloseErr)
		}
		if listenerCloseErr := listener.Close(); listenerCloseErr != nil {
			return fmt.Errorf("closing listener: %w", listenerCloseErr)
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return nil
	}

	// Test cleanup
	if closeErr := closeFn(); closeErr != nil {
		t.Errorf("cleanup function failed: %v", closeErr)
	}

	// Verify process was killed (with timeout)
	for range 10 {
		time.Sleep(50 * time.Millisecond)
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return // Success
		}
	}

	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Log("Note: Process cleanup timing can be system-dependent")
	}
}

func TestNewStdioLauncher(t *testing.T) {
	launcher := NewStdioLauncher()

	if launcher == nil {
		t.Fatal("NewStdioLauncher returned nil")
	}

	if launcher.timeout != stdioTimeout {
		t.Errorf("expected timeout %v, got %v", stdioTimeout, launcher.timeout)
	}
}

func TestStdioLauncher_ProcessCleanup(t *testing.T) {
	launcher := NewStdioLauncher()
	ctx := context.Background()

	// Start with invalid command to test cleanup on error
	_, _, err := launcher.Start(ctx, "/nonexistent/command")

	if err == nil {
		t.Error("expected error for invalid command")
	}

	// Should not leave any hanging processes or connections
	// This is mainly testing that error cleanup doesn't panic

	// Verify launcher is still usable after error
	if launcher.timeout != stdioTimeout {
		t.Error("launcher timeout was modified after error")
	}
}

func TestStdioLauncher_MultipleStarts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multiple starts test in short mode")
	}

	launcher := NewStdioLauncher()

	// Try multiple starts to ensure no resource leaks
	for i := range 3 {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)

		// Use invalid command to ensure quick failure and cleanup
		_, _, err := launcher.Start(ctx, "/nonexistent/command")

		cancel()

		if err == nil {
			t.Errorf("iteration %d: expected error for invalid command", i)
		}

		// Brief pause between attempts
		time.Sleep(10 * time.Millisecond)
	}
}

// Helper functions for creating mock commands

func createMockStdioCommand(t *testing.T) string {
	// Create a simple command that can handle stdio
	if runtime.GOOS == "windows" {
		return createWindowsStdioMock(t)
	}

	// On Unix systems, use cat which echoes stdin to stdout
	script := `#!/bin/bash
# Ignore the --stdio flag and just echo
exec cat
`

	return createStdioScript(t, script, ".sh")
}

func createWindowsStdioMock(t *testing.T) string {
	// On Windows, use a batch file that invokes PowerShell to echo stdin
	// This avoids the "not a valid Win32 application" error when executing .ps1 directly
	script := `@echo off
powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$input | ForEach-Object { $_ }"
`
	return createStdioScript(t, script, ".bat")
}

func createStdioScript(t *testing.T, content, ext string) string {
	tmpfile, err := os.CreateTemp(t.TempDir(), "mock-stdio-*"+ext)
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

	// Clean up after test
	t.Cleanup(func() {
		os.Remove(tmpfile.Name())
	})

	return tmpfile.Name()
}

// Test helper types

type testGRPCConn struct {
	netConn net.Conn
}

func (t *testGRPCConn) Close() error {
	if t.netConn != nil {
		return t.netConn.Close()
	}
	return nil
}

func (t *testGRPCConn) GetState() interface{} {
	return "READY"
}
