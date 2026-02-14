package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/spec"
)

// getAnalyzerLogLevel reads the FINFOCUS_LOG_LEVEL environment variable and returns
// the corresponding zerolog level. If the environment variable is unset or cannot be
// parsed, it returns zerolog.InfoLevel.
func getAnalyzerLogLevel() zerolog.Level {
	if envLevel := os.Getenv(pluginsdk.EnvLogLevel); envLevel != "" {
		if parsed, err := zerolog.ParseLevel(envLevel); err == nil {
			return parsed
		}
	}
	return zerolog.InfoLevel
}

// NewAnalyzerServeCmd creates the analyzer serve command.
//
// This command starts the gRPC server for the Pulumi Analyzer plugin.
// It binds to a random TCP port and prints ONLY the port number to stdout
// (this is the handshake protocol with Pulumi engine).
//
// to stderr.
func NewAnalyzerServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Pulumi Analyzer gRPC server",
		Long: `Starts the FinFocus Analyzer as a gRPC server for Pulumi integration.

This command is called automatically by the Pulumi engine when the analyzer
is configured in a project's Pulumi.yaml file. It:

  1. Binds to a random available TCP port
  2. Prints ONLY the port number to stdout (Pulumi handshake)
  3. Starts the gRPC server and waits for requests
  4. Handles SIGINT/SIGTERM for graceful shutdown

IMPORTANT: stdout is reserved exclusively for the port handshake.
All logging output goes to stderr.`,
		Example: `  # Normal usage (called by Pulumi engine)
  finfocus analyzer serve

  # With debug logging
  finfocus analyzer serve --debug`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunAnalyzerServe(cmd)
		},
	}

	return cmd
}

// RunAnalyzerServe starts a Pulumi Analyzer gRPC server, binds to a random TCP port, writes only the chosen port number to stdout for the Pulumi plugin handshake, and serves analyzer requests until a shutdown signal or context cancellation occurs.
//
// The cmd parameter is the Cobra command whose context and root version are used to control lifecycle and to populate the server version string.
//
// It returns an error if the server fails to bind to a port or if the gRPC server returns a runtime error while serving.
func RunAnalyzerServe(cmd *cobra.Command) error {
	ctx := cmd.Context()

	// CRITICAL: Create a logger that writes ONLY to stderr
	// stdout must be reserved for the port handshake
	// Default to info level to reduce noise in Pulumi output
	stderrLogger := zerolog.New(os.Stderr).
		Level(getAnalyzerLogLevel()).
		With().
		Str("component", "analyzer").
		Timestamp().
		Logger()

	// Set environment variable to indicate analyzer mode for plugin suppression
	// This prevents plugins from outputting verbose logs that clutter Pulumi preview
	if err := os.Setenv(constants.EnvAnalyzerMode, "true"); err != nil {
		// os.Setenv rarely fails, but if it does, log and continue
		stderrLogger.Warn().Err(err).Msg("failed to set analyzer mode environment variable")
	}

	stderrLogger.Debug().Msg("starting analyzer server")

	// Load configuration
	cfg := config.New()

	// Create spec loader for fallback pricing
	specLoader := spec.NewLoader(cfg.SpecDir)

	// Create registry for plugin discovery
	reg := registry.NewDefault()

	// Open plugin clients (empty adapter means all available plugins)
	clients, cleanup, err := reg.Open(ctx, "")
	if err != nil {
		stderrLogger.Warn().Err(err).Msg("failed to open plugins, continuing with spec-only mode")
		clients = nil
	}
	if cleanup != nil {
		defer cleanup()
	}

	stderrLogger.Debug().Int("plugin_count", len(clients)).Msg("plugins loaded")

	// Create the cost calculation engine
	eng := engine.New(clients, specLoader).
		WithRouter(createRouterForEngine(ctx, clients))

	// Create the analyzer server
	// Use the version from the command's root if available
	version := cmd.Root().Version
	if version == "" {
		version = "0.0.0-dev"
	}
	server := analyzer.NewServer(eng, version)

	// Listen on random port
	//nolint:gosec,noctx // G102: Intentionally binds to all interfaces for Pulumi plugin protocol
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		stderrLogger.Error().Err(err).Msg("failed to bind to port")
		return fmt.Errorf("binding to port: %w", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			stderrLogger.Debug().Err(closeErr).Msg("listener close error (already closed)")
		}
	}()

	// Get the actual port
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		stderrLogger.Error().Msg("failed to get TCP address")
		return errors.New("getting TCP address")
	}
	port := tcpAddr.Port

	stderrLogger.Info().Int("port", port).Msg("analyzer server listening")

	// CRITICAL: Print ONLY the port number to stdout
	// This is the Pulumi plugin handshake protocol
	// Any other output to stdout will break the handshake
	//nolint:forbidigo // Required by Pulumi plugin protocol - port handshake must use stdout
	fmt.Println(port)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(grpcServer, server)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create error channel for server goroutine
	errChan := make(chan error, 1)

	// Start server in goroutine
	go func() {
		stderrLogger.Debug().Msg("starting gRPC serve")
		if serveErr := grpcServer.Serve(listener); serveErr != nil {
			errChan <- serveErr
		}
		close(errChan)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		stderrLogger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		grpcServer.GracefulStop()
	case serveErr := <-errChan:
		if serveErr != nil {
			stderrLogger.Error().Err(serveErr).Msg("server error")
			return fmt.Errorf("serving gRPC: %w", serveErr)
		}
	case <-cmd.Context().Done():
		stderrLogger.Info().Msg("context canceled")
		grpcServer.GracefulStop()
	}

	stderrLogger.Info().Msg("analyzer server stopped")
	return nil
}
