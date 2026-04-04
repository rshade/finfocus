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
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
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
// exclusively for the port handshake.
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

// analyzerInfra holds the analyzer server infrastructure components.
type analyzerInfra struct {
	server  *analyzer.Server
	cleanup func()
}

// setupAnalyzerInfra creates and returns analyzerInfra configured for the given command and logger.
// It resolves the project directory, constructs a project-aware config, and builds a spec loader and registry.
// The function attempts to open plugin clients; if that fails it continues in spec-only mode with nil clients.
// It then creates an engine (with a router), determines a server version fallback ("0.0.0-dev"), and computes a
// summary directory (falling back to the global config dir when no project dir is set). The returned analyzerInfra
// contains the configured analyzer server and an optional cleanup callback for any opened plugin clients.
func setupAnalyzerInfra(cmd *cobra.Command, logger zerolog.Logger) analyzerInfra {
	ctx := cmd.Context()

	// Load project-aware configuration
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		logger.Warn().Err(cwdErr).Msg("failed to get CWD, using global config only")
	}
	projectDirFlag, _ := cmd.Flags().GetString("project-dir")
	projectDir := config.ResolveProjectDir(ctx, projectDirFlag, cwd)
	cfg := config.NewWithProjectDir(ctx, projectDir)

	// Create spec loader and registry
	specLoader := spec.NewLoader(cfg.SpecDir)
	reg := registry.NewDefault()

	// Open plugin clients (empty adapter means all available plugins)
	clients, cleanup, err := reg.Open(ctx, "")
	if err != nil {
		logger.Warn().Err(err).Msg("failed to open plugins, continuing with spec-only mode")
		clients = nil
	}

	// Filter out plugins that are explicitly disabled in analyzer.plugins config (#751).
	clients = filterDisabledClients(clients, cfg.Analyzer.Plugins)

	logger.Debug().Int("plugin_count", len(clients)).Msg("plugins loaded")

	// Create engine with router
	eng := engine.New(clients, specLoader).
		WithRouter(createRouterForEngine(ctx, cfg, clients))

	// Determine version and summary directory
	version := cmd.Root().Version
	if version == "" {
		version = "0.0.0-dev"
	}
	summaryDir := projectDir
	if summaryDir == "" {
		summaryDir = config.ResolveConfigDir()
	}

	// Initialize history store for recording analyzer events (cloud IDs).
	historyStore, historyCleanup := initHistoryFromConfig(ctx, cfg)
	var historyWriter *history.Writer
	if historyStore != nil && historyStore.IsEnabled() {
		historyWriter = history.NewWriter(historyStore, logger)
	}

	combinedCleanup := func() {
		historyCleanup()
		if cleanup != nil {
			cleanup()
		}
	}

	return analyzerInfra{
		server: analyzer.NewServer(eng, version).
			WithConfig(cfg).
			WithSummaryDir(summaryDir).
			WithHistoryWriter(historyWriter),
		cleanup: combinedCleanup,
	}
}

// RunAnalyzerServe starts the Pulumi Analyzer gRPC server, writes the selected
// listening port to stdout for the Pulumi plugin handshake, and runs until the
// RunAnalyzerServe starts a Pulumi gRPC analyzer server, prints the assigned port
// number to stdout for the Pulumi plugin handshake, and blocks until shutdown.
//
// The provided Cobra command's context is used for cancellation. The function
// installs signal handlers for SIGINT and SIGTERM and performs a graceful
// shutdown when a termination signal is received or when the command context is
// canceled. It also sets the analyzer mode environment variable to indicate
// analyzer-only operation and may load project-specific configuration and
// plugins as part of server setup.
//
// Parameters:
//   - cmd: the Cobra command whose context controls lifecycle and cancellation.
//
// Returns:
//   - an error if the server fails to bind to a TCP port, if the listener's
//     address cannot be determined, or if the gRPC server fails while serving;
//     returns nil on normal graceful shutdown.
func RunAnalyzerServe(cmd *cobra.Command) error {
	// CRITICAL: stdout must be reserved for the port handshake.
	// Use the project's logging framework which writes to stderr by default.
	stderrLogger := logging.FromContext(cmd.Context()).
		With().
		Str("component", "analyzer").
		Str("operation", "serve").
		Logger()

	// Set environment variable to indicate analyzer mode for plugin suppression
	if err := os.Setenv(constants.EnvAnalyzerMode, "true"); err != nil {
		stderrLogger.Warn().Err(err).Msg("failed to set analyzer mode environment variable")
	}

	stderrLogger.Debug().Msg("starting analyzer server")

	infra := setupAnalyzerInfra(cmd, stderrLogger)
	if infra.cleanup != nil {
		defer infra.cleanup()
	}

	// Listen on random port (localhost only)
	//nolint:noctx // net.Listen does not accept a context
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	//nolint:forbidigo // Required by Pulumi plugin protocol - port handshake must use stdout
	fmt.Println(port)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(grpcServer, infra.server)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
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

// filterDisabledClients removes clients whose names are explicitly configured as
// Enabled: false in the analyzer plugins map. Plugins absent from the map default
// to enabled for backward compatibility. A nil or empty map passes all clients through.
func filterDisabledClients(
	clients []*pluginhost.Client,
	analyzerPlugins map[string]config.AnalyzerPlugin,
) []*pluginhost.Client {
	if len(analyzerPlugins) == 0 {
		return clients
	}
	result := make([]*pluginhost.Client, 0, len(clients))
	for _, client := range clients {
		if plugin, ok := analyzerPlugins[client.Name]; ok && !plugin.Enabled {
			continue // explicitly disabled
		}
		result = append(result, client)
	}
	return result
}
