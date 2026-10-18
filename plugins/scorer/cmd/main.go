// Command finfocus-plugin-scorer serves the backend-neutral recommendation scorer.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/plugins/scorer"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v0.0.0-dev"

func main() {
	os.Exit(run())
}

func run() int {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", scorer.PluginName).Logger()
	if os.Getenv("FINFOCUS_LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	cfg, err := scorer.ConfigFromEnv(os.Getenv)
	if err != nil {
		logger.Error().Err(err).Msg("invalid configuration")
		return 1
	}
	cfg.Logger = logger
	if cfg.APIKey == "" {
		logger.Warn().
			Str("env", scorer.EnvAPIKey).
			Msg("no API key configured; scoring calls will return UNAUTHENTICATED")
	}

	plugin, err := scorer.New(cfg)
	if err != nil {
		logger.Error().Err(err).Msg("cannot start plugin")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serve := pluginsdk.ServeConfig{
		Plugin:     plugin,
		PluginInfo: scorer.Info(version),
		Logger:     &logger,
	}
	if err = pluginsdk.Serve(ctx, serve); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}
