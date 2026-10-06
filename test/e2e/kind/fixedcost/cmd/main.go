// Command finfocus-plugin-fixedcost serves the e2e fixed actual-cost plugin.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/test/e2e/kind/fixedcost"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	// Stdout is the port handshake. Every log line goes to stderr.
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", fixedcost.PluginName).Logger()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := pluginsdk.Serve(ctx, pluginsdk.ServeConfig{
		Plugin:     fixedcost.New(),
		PluginInfo: fixedcost.Info(version),
		Logger:     &logger,
	}); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}
