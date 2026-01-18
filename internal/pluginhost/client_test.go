package pluginhost_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
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
		time.Sleep(s.pluginInfoWait)
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
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return m.listener.Dial()
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

func TestGetPluginInfo_Success(t *testing.T) {
	// Setup mock server returning valid info
	srv := &mockCostSourceServer{
		name: "test-plugin",
		pluginInfo: &pbc.GetPluginInfoResponse{
			Version:     "1.0.0",
			SpecVersion: "0.4.14",
		},
	}
	launcher, cleanup := setupMockServer(t, srv)
	defer cleanup()

	// Call NewClient (which triggers GetPluginInfo)
	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, launcher, "dummy")
	require.NoError(t, err)
	defer client.Close()

	// Verify client has stored metadata
	assert.Equal(t, "test-plugin", client.Name)
	assert.Equal(t, "1.0.0", client.Metadata.Version)
}

func TestGetPluginInfo_Unimplemented(t *testing.T) {
	// Setup mock server returning Unimplemented for GetPluginInfo
	srv := &mockCostSourceServer{
		name:          "legacy-plugin",
		pluginInfoErr: status.Error(codes.Unimplemented, "method not implemented"),
	}
	launcher, cleanup := setupMockServer(t, srv)
	defer cleanup()

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, launcher, "dummy")

	// Should NOT fail, but log warning (impl detail)
	require.NoError(t, err)
	assert.Equal(t, "legacy-plugin", client.Name)
}

func TestGetPluginInfo_Timeout(t *testing.T) {
	// Setup mock server that sleeps longer than timeout
	srv := &mockCostSourceServer{
		name:           "slow-plugin",
		pluginInfoWait: 6 * time.Second, // Timeout is 5s
		pluginInfo: &pbc.GetPluginInfoResponse{
			Version: "1.0.0",
		},
	}
	launcher, cleanup := setupMockServer(t, srv)
	defer cleanup()

	ctx := context.Background()
	// On timeout, NewClient logs a warning and returns client with nil Metadata
	client, err := pluginhost.NewClient(ctx, launcher, "dummy")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "slow-plugin", client.Name)
	assert.Nil(t, client.Metadata)
}

func TestGetPluginInfo_StrictMode_BlocksIncompatible(t *testing.T) {
	// Enable strict compatibility mode
	t.Setenv("FINFOCUS_STRICT_COMPATIBILITY", "true")

	// Setup mock server returning incompatible major version
	srv := &mockCostSourceServer{
		name: "incompatible-plugin",
		pluginInfo: &pbc.GetPluginInfoResponse{
			Version:     "1.0.0",
			SpecVersion: "99.0.0", // Major version mismatch
		},
	}
	launcher, cleanup := setupMockServer(t, srv)
	defer cleanup()

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, launcher, "dummy")

	// In strict mode, incompatible plugins should fail to load
	require.Error(t, err)
	assert.ErrorIs(t, err, pluginhost.ErrPluginIncompatible)
	assert.Nil(t, client)
}

func TestGetPluginInfo_PermissiveMode_AllowsIncompatible(t *testing.T) {
	// Ensure strict mode is OFF (default)
	t.Setenv("FINFOCUS_STRICT_COMPATIBILITY", "false")

	// Setup mock server returning incompatible major version
	srv := &mockCostSourceServer{
		name: "incompatible-plugin",
		pluginInfo: &pbc.GetPluginInfoResponse{
			Version:     "1.0.0",
			SpecVersion: "99.0.0", // Major version mismatch
		},
	}
	launcher, cleanup := setupMockServer(t, srv)
	defer cleanup()

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, launcher, "dummy")

	// In permissive mode (default), incompatible plugins load with warning
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()
	assert.Equal(t, "incompatible-plugin", client.Name)
}
