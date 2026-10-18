package cli

import (
	"os"

	"github.com/spf13/cobra"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

const usageProfileFlagHelp = "Workload hint for projected cost and recommendations: dev, prod, burst, or default"

// resolveUsageProfile chooses the workload hint for this command.
// An explicit --profile wins, including --profile=default. Otherwise a set
// FINFOCUS_PROFILE wins, then a non-empty cost.profile. An omitted flag, an
// unset variable, and an omitted config field are no preference. A present
// empty value, including a saved profile of "", is an error.
func resolveUsageProfile(cmd *cobra.Command, cfg *config.Config) (pbc.UsageProfile, error) {
	if cmd != nil {
		if flag := cmd.Flags().Lookup("profile"); flag != nil && flag.Changed {
			return config.ParseRequiredUsageProfile(flag.Value.String())
		}
	}
	if value, ok := os.LookupEnv(config.UsageProfileEnv); ok {
		return config.ParseRequiredUsageProfile(value)
	}
	if cfg != nil && (cfg.Cost.HasProfile() || cfg.Cost.Profile != "") {
		return config.ParseRequiredUsageProfile(cfg.Cost.Profile)
	}
	return pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, nil
}

// usageProfileSource is the config whose cost.profile applies.
// The merged singleton includes the project overlay. config.New does not,
// so a caller that loaded New falls back to it only before initialization.
func usageProfileSource(fallback *config.Config) *config.Config {
	if merged := config.InitializedGlobalConfig(); merged != nil {
		return merged
	}
	return fallback
}

// applyUsageProfile sets the resolved hint on an engine that prices projected
// cost or recommendations. Commands that must not send a hint do not call it.
func applyUsageProfile(cmd *cobra.Command, eng *engine.Engine, cfg *config.Config) error {
	if eng == nil {
		return nil
	}
	profile, err := resolveUsageProfile(cmd, cfg)
	if err != nil {
		return err
	}
	eng.WithUsageProfile(profile)
	return nil
}

func displayedUsageProfile(profile string) string {
	if profile == "" {
		return "default"
	}
	return profile
}
