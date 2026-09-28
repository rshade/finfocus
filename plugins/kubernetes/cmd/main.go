// Command finfocus-plugin-kubernetes serves the kubernetes usage source and allocator.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/plugins/kubernetes"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v0.0.0-dev"

func main() {
	os.Exit(run())
}

func run() int {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", kubernetes.PluginName).Logger()
	if os.Getenv("FINFOCUS_LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := pluginsdk.ServeConfig{
		Plugin:     kubernetes.New(kubernetes.KubeconfigClusters),
		PluginInfo: kubernetes.Info(version),
		Logger:     &logger,
	}
	if err := pluginsdk.Serve(ctx, cfg); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}
