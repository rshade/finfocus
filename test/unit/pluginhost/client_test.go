package pluginhost_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TestNewClient_Success tests successful client creation with mock plugin.
func TestNewClient_Success(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "mock-plugin", client.Name) // Mock plugin returns "mock-plugin"
	assert.NotNil(t, client.Conn)
	assert.NotNil(t, client.API)
	assert.NotNil(t, client.Close)
}

// TestNewClient_LauncherError tests error handling when launcher fails.
func TestNewClient_LauncherError(t *testing.T) {
	expectedErr := errors.New("launcher failed")
	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			return nil, nil, expectedErr
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")

	require.Error(t, err)
	assert.Nil(t, client)
	assert.ErrorIs(t, err, expectedErr)
}

// TestNewClient_NameRPCError tests error handling when Name() RPC fails.
func TestNewClient_NameRPCError(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	// Configure mock to fail on Name() call
	helper.SetError("Name", plugin.ErrorProtocol)

	closeCalled := false
	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				closeCalled = true
				return nil
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "getting plugin name")
	assert.True(t, closeCalled, "Close function should be called on error")
}

// TestNewClient_NameRPCErrorWithCloseFail tests error handling when both Name() and Close() fail.
func TestNewClient_NameRPCErrorWithCloseFail(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.SetError("Name", plugin.ErrorProtocol)

	closeErr := errors.New("close failed")
	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				return closeErr
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "getting plugin name")
	assert.Contains(t, err.Error(), "close error")
}

// TestClient_Fields tests that all client fields are properly populated.
func TestClient_Fields(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")

	require.NoError(t, err)

	// Verify Name field
	assert.Equal(t, "mock-plugin", client.Name) // Mock plugin returns "mock-plugin"

	// Verify Conn field
	assert.NotNil(t, client.Conn)

	// Verify API field
	assert.NotNil(t, client.API)
	_, ok := client.API.(proto.CostSourceClient)
	assert.True(t, ok, "API should implement CostSourceClient")

	// Verify Close field
	assert.NotNil(t, client.Close)
	closeErr := client.Close()
	assert.NoError(t, closeErr)
}

// TestClient_APIUsage tests that the client API can be used for RPC calls.
func TestClient_APIUsage(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")
	require.NoError(t, err)

	// Test Name() call
	nameResp, err := client.API.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "mock-plugin", nameResp.GetName()) // Mock plugin returns "mock-plugin"

	// Test GetProjectedCost() call
	costReq := &proto.GetProjectedCostRequest{
		Resources: []*proto.ResourceDescriptor{
			{
				Type:       "aws:ec2/instance:Instance",
				Properties: map[string]string{"instanceType": "t3.micro"},
			},
		},
	}
	costResp, err := client.API.GetProjectedCost(ctx, costReq)
	require.NoError(t, err)
	assert.NotNil(t, costResp)
	assert.NotEmpty(t, costResp.Results)
	assert.Equal(t, "USD", costResp.Results[0].Currency)
}

// TestClient_Close tests the Close() functionality.
func TestClient_Close(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	closeCalled := false
	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				closeCalled = true
				return nil
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")
	require.NoError(t, err)

	// Close the client
	closeErr := client.Close()

	assert.NoError(t, closeErr)
	assert.True(t, closeCalled, "Close function should be called")
}

// TestClient_CloseError tests error handling in Close().
func TestClient_CloseError(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	expectedErr := errors.New("close failed")
	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				return expectedErr
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")
	require.NoError(t, err)

	// Close the client
	closeErr := client.Close()

	assert.Error(t, closeErr)
	assert.ErrorIs(t, closeErr, expectedErr)
}

// TestClient_MultipleCloses tests that Close() can be called multiple times.
func TestClient_MultipleCloses(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	closeCount := 0
	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				closeCount++
				return nil
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")
	require.NoError(t, err)

	// Call Close() multiple times
	err1 := client.Close()
	err2 := client.Close()
	err3 := client.Close()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, 3, closeCount, "Close function should be called each time")
}

// TestClient_ContextCancellation tests behavior when context is cancelled.
func TestClient_ContextCancellation(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockLauncher := &mockLauncher{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			// Launcher should check context
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	client, err := pluginhost.NewClient(ctx, mockLauncher, "/fake/path")

	// Should fail due to cancelled context
	assert.Error(t, err)
	assert.Nil(t, client)
}

// mockLauncher is a mock implementation of the Launcher interface for testing.
type mockLauncher struct {
	startFunc func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error)
}

func (m *mockLauncher) Start(ctx context.Context, path string, args ...string) (*grpc.ClientConn, func() error, error) {
	if m.startFunc != nil {
		return m.startFunc(ctx, path, args...)
	}
	return nil, nil, errors.New("mockLauncher.Start not implemented")
}
