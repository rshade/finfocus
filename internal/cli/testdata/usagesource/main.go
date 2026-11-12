// Command usagesource is the usage-stats plugin installed by the cost cluster
// tests. Its name is the plugin directory it is copied into. GetStats appends
// the requested mode, and Allocate appends "allocate", to <marker dir>/<name>
// so a test can see which plugins were asked, in order.
// FINFOCUS_TEST_RUN_RATE_ONLY and FINFOCUS_TEST_HISTORICAL_ONLY name the
// plugin that rejects the other mode with InvalidArgument, and any scope is
// rejected as an unknown context. FINFOCUS_TEST_GETSTATS_UNAVAILABLE names
// the plugin whose GetStats fails with Unavailable. FINFOCUS_TEST_ALLOCATOR
// names the plugin that also advertises allocation. FINFOCUS_TEST_STALL, as
// <plugin>:<call> (GetStats or Allocate), makes that call never answer, even
// after the caller gives up; the plugin first writes its PID to
// <FINFOCUS_TEST_STALL_PID_DIR>/<name> when that is set.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	markerDirEnv        = "FINFOCUS_TEST_GETSTATS_MARKER_DIR"
	runRateOnlyEnv      = "FINFOCUS_TEST_RUN_RATE_ONLY"
	historicalOnlyEnv   = "FINFOCUS_TEST_HISTORICAL_ONLY"
	allocatorEnv        = "FINFOCUS_TEST_ALLOCATOR"
	allocateInternalEnv = "FINFOCUS_TEST_ALLOCATE_INTERNAL"
	unavailableEnv      = "FINFOCUS_TEST_GETSTATS_UNAVAILABLE"
	stallEnv            = "FINFOCUS_TEST_STALL"
	stallPIDDirEnv      = "FINFOCUS_TEST_STALL_PID_DIR"
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
	s.stallIfAsked("GetStats")
	if os.Getenv(unavailableEnv) == s.name {
		return nil, status.Error(codes.Unavailable, "cluster unreachable")
	}
	if req.GetScope() != "" {
		return nil, status.Errorf(codes.InvalidArgument, "context %q not found", req.GetScope())
	}
	if window && os.Getenv(runRateOnlyEnv) == s.name {
		return nil, status.Error(codes.InvalidArgument, s.name+" usage source is run-rate only")
	}
	if !window && os.Getenv(historicalOnlyEnv) == s.name {
		return nil, status.Error(codes.InvalidArgument, s.name+" usage source is historical only")
	}
	return &pbc.GetStatsResponse{Mode: mode}, nil
}

// Allocate validates the policy the way the allocation contract requires
// (INVALID_ARGUMENT for an unknown field or value) and echoes it as the
// effective policy. It is served only by the plugin FINFOCUS_TEST_ALLOCATOR
// names. FINFOCUS_TEST_ALLOCATE_INTERNAL makes it fail with Internal.
func (s *source) Allocate(_ context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
	if err := s.mark("allocate"); err != nil {
		return nil, status.Errorf(codes.Internal, "Allocate marker: %v", err)
	}
	s.stallIfAsked("Allocate")
	if os.Getenv(allocateInternalEnv) == s.name {
		return nil, status.Error(codes.Internal, "allocator crashed")
	}
	policy := req.GetPolicyJson()
	if len(policy) == 0 {
		policy = []byte(`{"idle":"share"}`)
	}
	var fields map[string]any
	if err := json.Unmarshal(policy, &fields); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "allocation policy: %v", err)
	}
	for key, value := range fields {
		switch key {
		case "idle":
			if value != "share" && value != "separate" {
				return nil, status.Errorf(codes.InvalidArgument, "idle: unsupported value %q", value)
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "allocation policy: unknown field %q", key)
		}
	}
	return &pbc.AllocateResponse{EffectivePolicyJson: policy, PolicyDigest: "test"}, nil
}

// stallIfAsked blocks for good when FINFOCUS_TEST_STALL names this plugin and
// call. It ignores the request context, as a hung plugin would, so only the
// host killing the process ends it.
func (s *source) stallIfAsked(call string) {
	if os.Getenv(stallEnv) != s.name+":"+call {
		return
	}
	if dir := os.Getenv(stallPIDDirEnv); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, s.name), []byte(strconv.Itoa(os.Getpid())), 0o600)
	}
	select {}
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
