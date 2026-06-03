package plugin

import (
	"context"
	"fmt"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// MockServer represents a running mock plugin gRPC server for testing.
type MockServer struct {
	Plugin   *MockPlugin
	server   *grpc.Server
	listener *bufconn.Listener
	address  string
}

// StartMockServer creates and starts a new mock plugin server on a random available port.
// The server runs in a separate goroutine and can be stopped with StopServer.
// Returns the server instance and any error encountered during startup.
func StartMockServer() (*MockServer, error) {
	return StartMockServerWithPlugin(NewMockPlugin())
}

// StartMockServerWithPlugin creates and starts a mock server using the provided plugin instance.
// This allows pre-configuring the plugin before starting the server.
func StartMockServerWithPlugin(plugin *MockPlugin) (*MockServer, error) {
	// Use bufconn for in-memory gRPC testing (faster than TCP)
	listener := bufconn.Listen(bufSize)

	grpcServer := grpc.NewServer()
	mockSrv := newMockServer(plugin)
	mockSrv.RegisterServer(grpcServer)

	mockServer := &MockServer{
		Plugin:   plugin,
		server:   grpcServer,
		listener: listener,
		address:  "bufnet", // Special address for bufconn
	}

	// Start server in background
	go func() {
		_ = grpcServer.Serve(listener) // Server stopped is expected during cleanup
	}()

	return mockServer, nil
}

// StartMockServerTCP creates and starts a mock server on a TCP port for integration testing.
// Returns the server instance with the actual TCP address and port.
func StartMockServerTCP() (*MockServer, error) {
	return StartMockServerTCPWithPlugin(NewMockPlugin())
}

// StartMockServerTCPWithPlugin creates a TCP mock server with a pre-configured plugin.
func StartMockServerTCPWithPlugin(plugin *MockPlugin) (*MockServer, error) {
	// Listen on random available port
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	mockSrv := newMockServer(plugin)
	mockSrv.RegisterServer(grpcServer)

	mockServer := &MockServer{
		Plugin:   plugin,
		server:   grpcServer,
		listener: nil, // TCP listener, not bufconn
		address:  listener.Addr().String(),
	}

	// Start server in background
	go func() {
		_ = grpcServer.Serve(listener) // Server stopped is expected during cleanup
	}()

	return mockServer, nil
}

// Address returns the server's address.
// For bufconn servers, returns "bufnet".
// For TCP servers, returns the actual address (e.g., "127.0.0.1:12345").
func (s *MockServer) Address() string {
	return s.address
}

// Dial creates a gRPC client connection to the mock server.
// The connection should be closed when no longer needed.
//
//nolint:staticcheck // SA1019: migrate deprecated grpc dial helpers in https://github.com/rshade/finfocus/issues/1213.
func (s *MockServer) Dial(ctx context.Context) (*grpc.ClientConn, error) {
	if s.address == "bufnet" {
		// Use bufconn dialer for in-memory testing
		bufDialer := func(context.Context, string) (net.Conn, error) {
			return s.listener.Dial()
		}

		return grpc.DialContext(
			ctx,
			"bufnet",
			grpc.WithContextDialer(bufDialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	}

	// Use normal TCP dial for TCP servers
	return grpc.DialContext(
		ctx,
		s.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

// Stop gracefully stops the mock server and cleans up resources.
func (s *MockServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

// ForceStop immediately stops the server without waiting for active connections.
// Use this if GracefulStop hangs or in cleanup after test timeout.
func (s *MockServer) ForceStop() {
	if s.server != nil {
		s.server.Stop()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

// TestHelper provides convenience methods for testing with mock plugins.
type TestHelper struct {
	t      *testing.T
	server *MockServer
}

// NewTestHelper creates a new test helper with an automatically started mock server.
// The server will be automatically stopped when the test completes.
func NewTestHelper(t *testing.T) *TestHelper {
	t.Helper()

	server, err := StartMockServer()
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}

	t.Cleanup(func() {
		server.Stop()
	})

	return &TestHelper{
		t:      t,
		server: server,
	}
}

// NewTestHelperWithPlugin creates a test helper with a pre-configured plugin.
func NewTestHelperWithPlugin(t *testing.T, plugin *MockPlugin) *TestHelper {
	t.Helper()

	server, err := StartMockServerWithPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}

	t.Cleanup(func() {
		server.Stop()
	})

	return &TestHelper{
		t:      t,
		server: server,
	}
}

// Server returns the underlying mock server instance.
func (h *TestHelper) Server() *MockServer {
	return h.server
}

// Plugin returns the mock plugin for configuration.
func (h *TestHelper) Plugin() *MockPlugin {
	return h.server.Plugin
}

// Dial creates a client connection to the mock server.
// The connection is automatically closed when the test completes.
func (h *TestHelper) Dial() *grpc.ClientConn {
	h.t.Helper()

	ctx := context.Background()
	conn, err := h.server.Dial(ctx)
	if err != nil {
		h.t.Fatalf("Failed to dial mock server: %v", err)
	}

	h.t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

// ConfigureScenario is a convenience method to configure a test scenario.
func (h *TestHelper) ConfigureScenario(scenario ResponseScenario) {
	h.server.Plugin.ConfigureScenario(scenario)
}

// SetProjectedCost is a convenience method to set a projected cost response.
func (h *TestHelper) SetProjectedCost(resourceType string, monthly, hourly float64) {
	h.server.Plugin.SetProjectedCostResponse(resourceType, QuickResponse("USD", monthly, hourly))
}

// SetError is a convenience method to configure error injection.
func (h *TestHelper) SetError(method string, errorType ErrorType) {
	h.server.Plugin.SetError(method, errorType)
}

// Reset resets the mock plugin to its default state.
func (h *TestHelper) Reset() {
	h.server.Plugin.Reset()
}
