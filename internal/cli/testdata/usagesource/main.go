// Command usagesource is the usage-stats plugin installed by
// TestCostCluster_UsageSourceSelection. Its name is the plugin directory it
// is copied into. GetStats appends the requested mode to <marker dir>/<name>
// so the test can see which plugins were asked. FINFOCUS_TEST_RUN_RATE_ONLY
// and FINFOCUS_TEST_HISTORICAL_ONLY name the plugin that rejects the other
// mode with InvalidArgument, and FINFOCUS_TEST_ALLOCATOR names the plugin
// that also advertises allocation.
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

const (
	markerDirEnv      = "FINFOCUS_TEST_GETSTATS_MARKER_DIR"
	runRateOnlyEnv    = "FINFOCUS_TEST_RUN_RATE_ONLY"
	historicalOnlyEnv = "FINFOCUS_TEST_HISTORICAL_ONLY"
	allocatorEnv      = "FINFOCUS_TEST_ALLOCATOR"
)

func main() {
	name := pluginName()
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", name).Logger()
	caps := []pbc.PluginCapability{pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS}
	if os.Getenv(allocatorEnv) == name {
		caps = append(caps, pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION)
	}
	if err := pluginsdk.Serve(context.Background(), pluginsdk.ServeConfig{
		Plugin:     &source{BasePlugin: pluginsdk.NewBasePlugin(name), name: name},
		PluginInfo: pluginsdk.NewPluginInfo(name, "v0.0.0-test", pluginsdk.WithCapabilities(caps...)),
		Logger:     &logger,
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

	name string
}

func (s *source) GetStats(_ context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error) {
	window := req.GetStart() != nil || req.GetEnd() != nil
	mode, label := pbc.StatsMode_STATS_MODE_RUN_RATE, "run-rate"
	if window {
		mode, label = pbc.StatsMode_STATS_MODE_HISTORICAL, "historical"
	}
	if err := s.mark(label); err != nil {
		return nil, status.Errorf(codes.Internal, "GetStats marker: %v", err)
	}
	if window && os.Getenv(runRateOnlyEnv) == s.name {
		return nil, status.Error(codes.InvalidArgument, s.name+" usage source is run-rate only")
	}
	if !window && os.Getenv(historicalOnlyEnv) == s.name {
		return nil, status.Error(codes.InvalidArgument, s.name+" usage source is historical only")
	}
	return &pbc.GetStatsResponse{Mode: mode}, nil
}

func (s *source) mark(label string) error {
	dir := os.Getenv(markerDirEnv)
	if dir == "" {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, s.name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.WriteString(label + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
