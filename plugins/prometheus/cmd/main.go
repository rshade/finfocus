// Command finfocus-plugin-prometheus serves the Prometheus historical usage source.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/plugins/prometheus"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v0.0.0-dev"

func main() {
	os.Exit(run())
}

func run() int {
	// Stdout is the port handshake. Every log line goes to stderr.
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", prometheus.PluginName).Logger()
	if os.Getenv("FINFOCUS_LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// A missing address must not stop the server: core skips a plugin that
	// fails to launch, so the user would never see the variable named.
	// GetStats returns the same error instead.
	cfg, err := prometheus.LoadConfig()
	if err != nil {
		logger.Warn().Err(err).Msg("no Prometheus address; GetStats will fail until it is set")
	}
	logger.Info().Str("prometheus_url", cfg.URL).Bool("bearer_token_set", cfg.Token != "").
		Msg("starting prometheus usage source")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serve := pluginsdk.ServeConfig{
		Plugin:     prometheus.New(cfg),
		PluginInfo: prometheus.Info(version),
		Logger:     &logger,
	}
	if err = pluginsdk.Serve(ctx, serve); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}
