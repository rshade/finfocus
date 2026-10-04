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
	"github.com/rshade/finfocus/plugins/kubernetes/workload"
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

	pricing := kubernetes.LoadConfig(os.Getenv)
	logPricingConfig(logger, pricing)

	cfg := pluginsdk.ServeConfig{
		Plugin:     kubernetes.New(kubernetes.KubeconfigClusters, pricing),
		PluginInfo: kubernetes.Info(version),
		Logger:     &logger,
	}
	if err := pluginsdk.Serve(ctx, cfg); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}

// logPricingConfig records which projected-cost settings are present. Rates and
// hints are not secrets, so accepted values are logged; a rejected value is
// logged only through its validation error.
func logPricingConfig(logger zerolog.Logger, cfg kubernetes.Config) {
	logger.Debug().Func(func(event *zerolog.Event) {
		logFloat := func(name string, setting workload.Setting[float64]) {
			event.Bool(name+"_set", setting.Set)
			if setting.Usable() {
				event.Float64(name, setting.Value)
			}
		}
		logFloat(workload.EnvCPUHourlyRate, cfg.CPUHourlyRate)
		logFloat(workload.EnvMemoryGiBHourlyRate, cfg.MemoryGiBHourlyRate)
		logFloat(workload.EnvJobHoursPerMonth, cfg.JobHoursPerMonth)
		event.Bool(workload.EnvDaemonSetNodeCount+"_set", cfg.DaemonSetNodeCount.Set)
		if cfg.DaemonSetNodeCount.Usable() {
			event.Int(workload.EnvDaemonSetNodeCount, cfg.DaemonSetNodeCount.Value)
		}
	}).Msg("projected pricing configuration loaded")
	for _, err := range []error{
		cfg.CPUHourlyRate.Err, cfg.MemoryGiBHourlyRate.Err, cfg.DaemonSetNodeCount.Err, cfg.JobHoursPerMonth.Err,
	} {
		if err != nil {
			logger.Warn().Err(err).Msg("projected pricing setting rejected; affected workloads will be declined")
		}
	}
}
