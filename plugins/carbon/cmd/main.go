// Command finfocus-plugin-carbon serves offline EC2 operational carbon estimates.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/plugins/carbon"
)

// version is set by the build and monorepo release workflows.
var version = "v0.0.0-dev"

func main() { os.Exit(run()) }

func run() int {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", carbon.PluginName).Logger()
	cfg, err := carbon.ConfigFromEnv(os.Getenv)
	if err != nil {
		logger.Error().Err(err).Msg("invalid carbon configuration")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err = pluginsdk.Serve(
		ctx,
		pluginsdk.ServeConfig{Plugin: carbon.New(cfg), PluginInfo: carbon.Info(version), Logger: &logger},
	); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}
