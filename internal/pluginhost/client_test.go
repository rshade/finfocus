package pluginhost_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

const bufSize = 1024 * 1024

type mockCostSourceServer struct {
	pbc.UnimplementedCostSourceServiceServer

	name           string
	pluginInfo     *pbc.GetPluginInfoResponse
	pluginInfoErr  error
	pluginInfoWait time.Duration
}

func (s *mockCostSourceServer) Name(
	ctx context.Context,
	req *pbc.NameRequest,
) (*pbc.NameResponse, error) {
	return &pbc.NameResponse{Name: s.name}, nil
}

func (s *mockCostSourceServer) GetPluginInfo(
	ctx context.Context,
	req *pbc.GetPluginInfoRequest,
) (*pbc.GetPluginInfoResponse, error) {
	if s.pluginInfoWait > 0 {
		select {
		case <-time.After(s.pluginInfoWait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.pluginInfoErr != nil {
		return nil, s.pluginInfoErr
	}
	return s.pluginInfo, nil
}

type grpcMockLauncher struct {
	listener *bufconn.Listener
	server   *grpc.Server
}

func (m *grpcMockLauncher) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return m.listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	return conn, func() error { return conn.Close() }, err
}

func setupMockServer(_ *testing.T, srv *mockCostSourceServer) (*grpcMockLauncher, func()) {
	listener := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pbc.RegisterCostSourceServiceServer(s, srv)
	go func() {
		if err := s.Serve(listener); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				// Unexpected error during serve
				panic(err)
			}
			// Server stopped gracefully
		}
	}()

	launcher := &grpcMockLauncher{
		listener: listener,
		server:   s,
	}

	return launcher, func() {
		s.Stop()
		listener.Close()
	}
}

func TestGetPluginInfo(t *testing.T) {
	tests := []struct {
		name          string
		srv           *mockCostSourceServer
		strictMode    *bool // nil = don't set env var
		wantErr       bool
		wantErrIs     error
		wantName      string
		wantNilClient bool
		wantVersion   string
		wantNilMeta   bool
	}{
		{
			name: "success",
			srv: &mockCostSourceServer{
				name: "test-plugin",
				pluginInfo: &pbc.GetPluginInfoResponse{
					Version:     "1.0.0",
					SpecVersion: "0.4.14",
				},
			},
			wantName:    "test-plugin",
			wantVersion: "1.0.0",
		},
		{
			name: "unimplemented_graceful_degradation",
			srv: &mockCostSourceServer{
				name:          "legacy-plugin",
				pluginInfoErr: status.Error(codes.Unimplemented, "method not implemented"),
			},
			wantName: "legacy-plugin",
		},
		{
			name: "timeout_returns_nil_metadata",
			srv: &mockCostSourceServer{
				name:           "slow-plugin",
				pluginInfoWait: 6 * time.Second,
				pluginInfo:     &pbc.GetPluginInfoResponse{Version: "1.0.0"},
			},
			wantName:    "slow-plugin",
			wantNilMeta: true,
		},
		{
			name: "strict_mode_blocks_incompatible",
			srv: &mockCostSourceServer{
				name: "incompatible-plugin",
				pluginInfo: &pbc.GetPluginInfoResponse{
					Version:     "1.0.0",
					SpecVersion: "99.0.0",
				},
			},
			strictMode:    boolPtr(true),
			wantErr:       true,
			wantErrIs:     pluginhost.ErrPluginIncompatible,
			wantNilClient: true,
		},
		{
			name: "permissive_mode_allows_incompatible",
			srv: &mockCostSourceServer{
				name: "incompatible-plugin",
				pluginInfo: &pbc.GetPluginInfoResponse{
					Version:     "1.0.0",
					SpecVersion: "99.0.0",
				},
			},
			strictMode: boolPtr(false),
			wantName:   "incompatible-plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.strictMode != nil {
				if *tt.strictMode {
					t.Setenv("FINFOCUS_STRICT_COMPATIBILITY", "true")
				} else {
					t.Setenv("FINFOCUS_STRICT_COMPATIBILITY", "false")
				}
			}

			launcher, cleanup := setupMockServer(t, tt.srv)
			defer cleanup()

			ctx := context.Background()
			client, err := pluginhost.NewClient(ctx, launcher, "dummy")

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			defer client.Close()

			assert.Equal(t, tt.wantName, client.Name)

			if tt.wantNilMeta {
				assert.Nil(t, client.Metadata)
			} else if tt.wantVersion != "" {
				require.NotNil(t, client.Metadata)
				assert.Equal(t, tt.wantVersion, client.Metadata.Version)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

// TestNewClient_Success tests successful client creation with mock plugin.
func TestNewClient_Success(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")

	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()
	assert.Equal(t, "mock-plugin", client.Name) // Mock plugin returns "mock-plugin"
	assert.NotNil(t, client.Conn)
	assert.NotNil(t, client.API)
	assert.NotNil(t, client.Close)
}

// TestNewClient_LauncherErrorMock tests error handling when launcher fails using mock.
func TestNewClient_LauncherErrorMock(t *testing.T) {
	expectedErr := errors.New("launcher failed")
	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			return nil, nil, expectedErr
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")

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
	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				closeCalled = true
				return nil
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")

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
	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				return closeErr
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "getting plugin name")
	assert.Contains(t, err.Error(), "close error")
}

// TestClient_Fields tests that all client fields are properly populated.
func TestClient_Fields(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")

	require.NoError(t, err)

	// Verify Name field
	assert.Equal(t, "mock-plugin", client.Name) // Mock plugin returns "mock-plugin"

	// Verify Conn field
	assert.NotNil(t, client.Conn)

	// Verify API field
	assert.NotNil(t, client.API)

	// Verify Close field
	assert.NotNil(t, client.Close)
	closeErr := client.Close()
	assert.NoError(t, closeErr)
}

// TestClient_APIUsage tests that the client API can be used for RPC calls.
func TestClient_APIUsage(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")
	require.NoError(t, err)
	defer client.Close()

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
	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				closeCalled = true
				return nil
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")
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
	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				return expectedErr
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")
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
	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			conn := helper.Dial()
			return conn, func() error {
				closeCount++
				return nil
			}, nil
		},
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")
	require.NoError(t, err)

	// Call Close() multiple times — should be idempotent (sync.Once)
	err1 := client.Close()
	err2 := client.Close()
	err3 := client.Close()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, 1, closeCount, "Close function should be called only once (idempotent)")
}

// TestClient_ContextCancellation tests behavior when context is cancelled.
func TestClient_ContextCancellation(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockLauncherInst := &mockLauncherBasic{
		startFunc: func(startCtx context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error) {
			// Launcher should check the context parameter passed to Start
			if startCtx.Err() != nil {
				return nil, nil, startCtx.Err()
			}
			conn := helper.Dial()
			return conn, func() error { return nil }, nil
		},
	}

	client, err := pluginhost.NewClient(ctx, mockLauncherInst, "/fake/path")

	// Should fail due to cancelled context
	assert.Error(t, err)
	assert.Nil(t, client)
}

// mockLauncherBasic is a mock implementation of the Launcher interface for testing.
type mockLauncherBasic struct {
	startFunc func(_ context.Context, _ string, _ ...string) (*grpc.ClientConn, func() error, error)
}

func (m *mockLauncherBasic) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	if m.startFunc != nil {
		return m.startFunc(ctx, path, args...)
	}
	return nil, nil, errors.New("mockLauncherBasic.Start not implemented")
}
