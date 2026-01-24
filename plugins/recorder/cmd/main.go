// Package main provides the entry point for the recorder plugin.
//
// The recorder plugin is a reference implementation that:
//   - Records all gRPC requests to JSON files for inspection
//   - Optionally returns mock cost responses for testing
//   - Demonstrates pluginsdk v0.4.6 patterns and best practices
//
// Configuration via environment variables:
//   - FINFOCUS_RECORDER_OUTPUT_DIR: Directory for recorded files (default: ./recorded_data)
//   - FINFOCUS_RECORDER_MOCK_RESPONSE: Enable mock responses (default: false)
//
// Usage:
//
//	# Start with TCP mode (default)
//	./finfocus-plugin-recorder
//
//	# Start with specific port
//	FINFOCUS_PLUGIN_PORT=50051 ./finfocus-plugin-recorder
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/plugins/recorder"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Initialize logger
	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("plugin", "recorder").
		Logger()

	// Set log level based on environment
	if os.Getenv("FINFOCUS_LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	logger.Info().Msg("starting recorder plugin")

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		signal.Stop(sigCh)
		cancel()
	}()

	// Load configuration from environment
	cfg := recorder.LoadConfig()

	// Create the plugin instance
	plugin := recorder.NewRecorderPlugin(cfg, logger)
	defer plugin.Shutdown()

	// Configure and start the gRPC server
	serveConfig := pluginsdk.ServeConfig{
		Plugin: plugin,
		Port:   0, // Use FINFOCUS_PLUGIN_PORT env var or random port
		Logger: &logger,
	}

	// Serve until context is cancelled
	if err := pluginsdk.Serve(ctx, serveConfig); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}

	logger.Info().Msg("recorder plugin stopped")
	return 0
}
