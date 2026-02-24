//go:build integration
// +build integration

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// makeTestResources creates a slice of ResourceDescriptor for use in integration tests.
func makeTestResources(resourceType string, count int) []*proto.ResourceDescriptor {
	resources := make([]*proto.ResourceDescriptor, count)
	for i := range count {
		resources[i] = &proto.ResourceDescriptor{
			Type:     resourceType,
			ID:       "test-resource",
			Provider: "aws",
			Properties: map[string]string{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		}
	}
	return resources
}

// TestPluginResilience_CrashMidRPC verifies that when a plugin's gRPC server is
// force-stopped while processing an RPC, the caller receives a structured error
// (not a panic) with an actionable message.
func TestPluginResilience_CrashMidRPC(t *testing.T) {
	// Start a TCP mock server with a long sleep duration so the handler
	// is busy when we crash the server.
	mockPlugin := plugin.NewMockPlugin()
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)
	mockPlugin.SetSleepDuration(30 * time.Second) // Handler will block for 30s

	server, err := plugin.StartMockServerTCPWithPlugin(mockPlugin)
	require.NoError(t, err, "mock server should start")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a gRPC client connection to the mock server
	conn, err := server.Dial(ctx)
	require.NoError(t, err, "should connect to mock server")
	defer func() { _ = conn.Close() }()

	client := proto.NewCostSourceClient(conn)

	resources := makeTestResources("aws:ec2/instance:Instance", 1)

	// Launch the RPC call in a goroutine
	var result *proto.CostResultWithErrors
	var callErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result = proto.GetProjectedCostWithErrors(ctx, client, "mock-plugin", resources)
		// If GetProjectedCostWithErrors returns (which it should even on error),
		// callErr remains nil. The error is captured in result.Errors.
		_ = callErr
	}()

	// Give the RPC call time to reach the server handler (which is now sleeping)
	time.Sleep(500 * time.Millisecond)

	// Crash the server mid-RPC by force-stopping it
	server.ForceStop()

	// Wait for the RPC call to complete (should return with error, not hang forever)
	wg.Wait()

	// The result should exist and contain an error, not be nil or panic
	require.NotNil(t, result, "result should not be nil even after server crash")
	assert.True(t, result.HasErrors(), "should have errors after server crash")
	require.NotEmpty(t, result.Results, "should have placeholder results")

	// Verify the error is a PLUGIN_ERROR (gRPC transport failure)
	foundPluginError := false
	for _, r := range result.Results {
		if r.StructuredError != nil && r.StructuredError.Code == proto.ErrCodePluginError {
			foundPluginError = true
			assert.NotEmpty(t, r.StructuredError.Message, "error message should be actionable")
			assert.Equal(t, "aws:ec2/instance:Instance", r.StructuredError.ResourceType)
		}
	}
	assert.True(t, foundPluginError, "should contain PLUGIN_ERROR structured error")

	// Verify error details are populated
	require.NotEmpty(t, result.Errors)
	assert.Equal(t, "aws:ec2/instance:Instance", result.Errors[0].ResourceType)
	assert.Equal(t, "mock-plugin", result.Errors[0].PluginName)
}

// TestPluginResilience_TimeoutExceedsDeadline verifies that when a plugin's RPC
// handler exceeds the context deadline, the caller receives a TIMEOUT_ERROR
// structured error and no goroutines are leaked.
func TestPluginResilience_TimeoutExceedsDeadline(t *testing.T) {
	// Start a TCP mock server with SleepDuration exceeding the context deadline.
	mockPlugin := plugin.NewMockPlugin()
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)
	mockPlugin.SetSleepDuration(10 * time.Second) // Handler sleeps 10s

	server, err := plugin.StartMockServerTCPWithPlugin(mockPlugin)
	require.NoError(t, err, "mock server should start")
	// Use ForceStop rather than Stop (GracefulStop) because the handler is
	// intentionally sleeping beyond the context deadline. GracefulStop would
	// block until the handler completes its full sleep, making the test slow.
	defer server.ForceStop()

	// Use a context with a short deadline (2s) — much shorter than the 10s sleep
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create a gRPC client connection
	conn, err := server.Dial(context.Background()) // Use background for connection
	require.NoError(t, err, "should connect to mock server")
	defer func() { _ = conn.Close() }()

	client := proto.NewCostSourceClient(conn)

	resources := makeTestResources("aws:ec2/instance:Instance", 1)

	// Capture goroutine count before the call
	goroutinesBefore := runtime.NumGoroutine()

	// Make the RPC call — it should timeout due to context deadline
	result := proto.GetProjectedCostWithErrors(ctx, client, "mock-plugin", resources)

	// Verify the result contains a timeout error
	require.NotNil(t, result, "result should not be nil on timeout")
	assert.True(t, result.HasErrors(), "should have errors on timeout")
	require.NotEmpty(t, result.Results, "should have placeholder results")

	// Verify the structured error code is TIMEOUT_ERROR
	foundTimeoutError := false
	for _, r := range result.Results {
		if r.StructuredError != nil && r.StructuredError.Code == proto.ErrCodeTimeoutError {
			foundTimeoutError = true
			assert.NotEmpty(t, r.StructuredError.Message, "timeout error should have message")
			assert.Equal(t, "aws:ec2/instance:Instance", r.StructuredError.ResourceType)
		}
	}
	assert.True(t, foundTimeoutError, "should contain TIMEOUT_ERROR structured error")

	// Check for goroutine leaks — allow a small delta for runtime variance
	// Give goroutines time to clean up
	time.Sleep(500 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	goroutineDelta := goroutinesAfter - goroutinesBefore
	assert.LessOrEqual(t, goroutineDelta, 2,
		"goroutine leak detected: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, goroutineDelta)
}

// TestPluginResilience_MissingBinary verifies that attempting to launch a plugin
// from a nonexistent binary path produces a clear error containing the missing path.
func TestPluginResilience_MissingBinary(t *testing.T) {
	const missingPath = "/nonexistent/path/finfocus-plugin-fake"

	launcher := pluginhost.NewProcessLauncher()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, closeFn, err := launcher.Start(ctx, missingPath)

	// Clean up if connection was unexpectedly established
	if conn != nil {
		_ = conn.Close()
	}
	if closeFn != nil {
		_ = closeFn()
	}

	// Must return an error for a nonexistent binary
	require.Error(t, err, "should fail when plugin binary does not exist")
	assert.Contains(t, err.Error(), missingPath,
		"error should contain the exact missing path for debugging")
}

// TestPluginResilience_ZombieProcessPrevention verifies that a killed subprocess
// is properly reaped (no zombie) when cmd.Wait() is called after cmd.Process.Kill().
func TestPluginResilience_ZombieProcessPrevention(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("zombie process detection via signal 0 is Unix-specific")
	}

	// Start a real subprocess that will run until killed
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start(), "should start sleep subprocess")

	pid := cmd.Process.Pid
	require.Greater(t, pid, 0, "PID should be positive")

	// Verify the process exists before killing
	proc, err := os.FindProcess(pid)
	require.NoError(t, err, "should find the running process")
	require.NoError(t, proc.Signal(syscall.Signal(0)),
		"signal 0 should succeed for a running process")

	// Kill the process (mirrors pluginhost.killProcess pattern)
	require.NoError(t, cmd.Process.Kill(), "should kill the subprocess")

	// Wait to reap the zombie — this is the critical step that prevents zombies
	err = cmd.Wait()
	// cmd.Wait() returns an error for killed processes (signal: killed), which is expected
	require.Error(t, err, "Wait should return error for killed process")

	// Give the OS a moment to fully remove the process entry
	time.Sleep(100 * time.Millisecond)

	// Verify the process is no longer running — signal 0 should fail
	proc, err = os.FindProcess(pid)
	require.NoError(t, err, "FindProcess always succeeds on Unix")
	err = proc.Signal(syscall.Signal(0))
	assert.Error(t, err,
		"signal 0 should fail after kill+wait: process should no longer exist (pid=%d)", pid)
}

// TestPluginResilience_RecoveryAfterCrash verifies that after a plugin returns an
// error on the first request, subsequent requests either succeed or return clean
// errors (not stale connection failures).
func TestPluginResilience_RecoveryAfterCrash(t *testing.T) {
	mockPlugin := plugin.NewMockPlugin()
	mockPlugin.ConfigureScenario(plugin.ScenarioSuccess)

	server, err := plugin.StartMockServerTCPWithPlugin(mockPlugin)
	require.NoError(t, err, "mock server should start")
	defer server.ForceStop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := server.Dial(ctx)
	require.NoError(t, err, "should connect to mock server")
	defer func() { _ = conn.Close() }()

	client := proto.NewCostSourceClient(conn)

	// First request: configure FailForTypes so the plugin returns an error
	mockPlugin.SetFailForTypes([]string{"aws:ec2/instance:Instance"})
	resources := makeTestResources("aws:ec2/instance:Instance", 1)
	result1 := proto.GetProjectedCostWithErrors(ctx, client, "mock-plugin", resources)
	require.NotNil(t, result1, "first result should not be nil")
	assert.True(t, result1.HasErrors(), "first request should have errors (FailForTypes)")

	// Second request: clear FailForTypes so the plugin succeeds
	mockPlugin.SetFailForTypes(nil)
	result2 := proto.GetProjectedCostWithErrors(ctx, client, "mock-plugin", resources)
	require.NotNil(t, result2, "second result should not be nil")
	assert.False(t, result2.HasErrors(),
		"second request should succeed after clearing FailForTypes")
	require.NotEmpty(t, result2.Results, "second result should have cost data")
	assert.Greater(t, result2.Results[0].MonthlyCost, 0.0,
		"recovered request should return a real cost value")

	// Sub-test: rapid successive failures and recovery
	t.Run("RapidSuccessiveCrashes", func(t *testing.T) {
		goroutinesBefore := runtime.NumGoroutine()

		// Crash 3 times rapidly within 1 second
		for i := range 3 {
			mockPlugin.SetFailForTypes([]string{"aws:ec2/instance:Instance"})
			res := proto.GetProjectedCostWithErrors(ctx, client, "mock-plugin", resources)
			require.NotNil(t, res, "result should not be nil on crash iteration %d", i)
			assert.True(t, res.HasErrors(), "should have error on crash iteration %d", i)
		}

		// Recover: clear failures and verify clean response
		mockPlugin.SetFailForTypes(nil)
		recoveredResult := proto.GetProjectedCostWithErrors(ctx, client, "mock-plugin", resources)
		require.NotNil(t, recoveredResult, "recovered result should not be nil")
		assert.False(t, recoveredResult.HasErrors(),
			"should recover cleanly after 3 rapid crashes")

		// Verify no goroutine leaks after rapid crash/recovery cycle
		time.Sleep(500 * time.Millisecond)
		goroutinesAfter := runtime.NumGoroutine()
		goroutineDelta := goroutinesAfter - goroutinesBefore
		assert.LessOrEqual(t, goroutineDelta, 2,
			"goroutine leak after rapid crashes: before=%d, after=%d, delta=%d",
			goroutinesBefore, goroutinesAfter, goroutineDelta)
	})
}
