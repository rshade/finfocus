package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// UsageProfileEnv is the environment variable that overrides cost.profile.
const UsageProfileEnv = "FINFOCUS_PROFILE"

// ParseUsageProfile parses a profile label. An empty string is no preference.
// "default" and "unspecified" are no preference. Surrounding space is ignored.
// A non-empty unknown label returns an error.
func ParseUsageProfile(value string) (pbc.UsageProfile, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if value == "" {
			return pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, nil
		}
		return pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, errUsageProfileEmpty
	}
	if strings.EqualFold(trimmed, "default") {
		return pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, nil
	}
	profile, err := pluginsdk.ParseUsageProfile(trimmed)
	if err != nil {
		return pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, fmt.Errorf("usage profile: %w", err)
	}
	return profile, nil
}

// ParseRequiredUsageProfile parses a profile that was explicitly set.
// An empty or whitespace-only value is an error.
func ParseRequiredUsageProfile(value string) (pbc.UsageProfile, error) {
	if strings.TrimSpace(value) == "" {
		return pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, errUsageProfileEmpty
	}
	return ParseUsageProfile(value)
}

// UsageProfileLabel is the name printed for a profile. No preference is "default".
func UsageProfileLabel(profile pbc.UsageProfile) string {
	switch profile {
	case pbc.UsageProfile_USAGE_PROFILE_DEV:
		return "dev"
	case pbc.UsageProfile_USAGE_PROFILE_PROD:
		return "prod"
	case pbc.UsageProfile_USAGE_PROFILE_BURST:
		return "burst"
	case pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED:
		return "default"
	default:
		return "default"
	}
}

var errUsageProfileEmpty = errors.New("usage profile is empty (valid: dev, prod, burst, default)")
