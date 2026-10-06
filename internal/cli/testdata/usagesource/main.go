// Command usagesource is the usage-stats plugin installed by
// TestCostCluster_WindowKeepsUsageSourceSelection. Its name is the plugin
// directory it is copied into. GetStats writes a marker so that test can
// prove the command never called it.
package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// getStatsMarkerEnv is the file GetStats creates when the command reaches it.
const getStatsMarkerEnv = "FINFOCUS_TEST_GETSTATS_MARKER"

func main() {
	name := pluginName()
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", name).Logger()
	if err := pluginsdk.Serve(context.Background(), pluginsdk.ServeConfig{
		Plugin: &source{BasePlugin: pluginsdk.NewBasePlugin(name)},
		PluginInfo: pluginsdk.NewPluginInfo(name, "v0.0.0-test",
			pluginsdk.WithCapabilities(pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS),
		),
		Logger: &logger,
	}); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		os.Exit(1)
	}
}

// pluginName is the install directory: <root>/plugins/<name>/<version>/<binary>.
func pluginName() string {
	exe, err := os.Executable()
	if err != nil {
		return "usage"
	}
	return filepath.Base(filepath.Dir(filepath.Dir(exe)))
}

type source struct {
	*pluginsdk.BasePlugin
}

func (s *source) GetStats(context.Context, *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error) {
	if path := os.Getenv(getStatsMarkerEnv); path != "" {
		if err := os.WriteFile(path, []byte("called"), 0o600); err != nil {
			return nil, status.Errorf(codes.Internal, "GetStats marker: %v", err)
		}
	}
	return nil, status.Error(codes.Internal, "GetStats must not be called")
}
