package e2e

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// Config holds configuration for E2E tests.
type Config struct {
	AWSRegion       string
	Regions         []string // List of regions to test
	Tolerance       float64
	Timeout         time.Duration
	SkipActualCosts bool // Skip actual cost tests if true
}

// LoadConfig returns a Config populated from environment variables with sensible defaults.
//
// It initializes defaults (AWSRegion "us-east-1", Regions ["us-east-1"], Tolerance 0.05, Timeout 10 minutes, SkipActualCosts false) and
// overrides them when the following environment variables are set and valid:
// FINFOCUS_E2E_AWS_REGION (string), FINFOCUS_E2E_REGIONS (comma-separated), FINFOCUS_E2E_TOLERANCE (float64),
// FINFOCUS_E2E_TIMEOUT (duration parseable by time.ParseDuration), FINFOCUS_E2E_SKIP_ACTUAL_COSTS (bool).
// Invalid values are silently ignored and defaults are retained.
func LoadConfig() *Config {
	cfg := &Config{
		AWSRegion:       "us-east-1",
		Regions:         []string{"us-east-1"},
		Tolerance:       0.05,
		Timeout:         10 * time.Minute,
		SkipActualCosts: false,
	}

	if region := os.Getenv("FINFOCUS_E2E_AWS_REGION"); region != "" {
		cfg.AWSRegion = region
	}

	if regionsStr := os.Getenv("FINFOCUS_E2E_REGIONS"); regionsStr != "" {
		regions := strings.Split(regionsStr, ",")
		var validRegions []string
		for _, r := range regions {
			r = strings.TrimSpace(r)
			if r != "" {
				validRegions = append(validRegions, r)
			}
		}
		if len(validRegions) > 0 {
			cfg.Regions = validRegions
		}
	}

	if tolStr := os.Getenv("FINFOCUS_E2E_TOLERANCE"); tolStr != "" {
		if tol, err := strconv.ParseFloat(tolStr, 64); err == nil {
			cfg.Tolerance = tol
		}
	}

	if timeoutStr := os.Getenv("FINFOCUS_E2E_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.Timeout = timeout
		}
	}

	if skipStr := os.Getenv("FINFOCUS_E2E_SKIP_ACTUAL_COSTS"); skipStr != "" {
		if skip, err := strconv.ParseBool(skipStr); err == nil && skip {
			cfg.SkipActualCosts = true
		}
	}

	return cfg
}

// CheckPulumiAutomationAPISupport checks if Pulumi Automation API support is available (issue #177).
// Returns true if the feature is implemented and available for testing.
func CheckPulumiAutomationAPISupport() bool {
	// Check if Pulumi CLI is available (required for Automation API)
	// For now, assume it's available if running in CI or if explicitly enabled
	return os.Getenv("PULUMI_AUTOMATION_API_ENABLED") == "true" || os.Getenv("CI") == "true"
}

// CheckGetActualCostSupport checks if GetActualCost fallback implementation is available (issue #24).
// Returns true if the feature is implemented and available for testing.
func CheckGetActualCostSupport() bool {
	// Check if AWS plugin supports GetActualCost
	versionStr := os.Getenv("FINFOCUS_PLUGIN_AWS_VERSION")
	if versionStr != "" {
		// Parse version using semantic versioning
		v, err := semver.NewVersion(versionStr)
		if err == nil {
			// Check if version >= 0.1.0
			constraint, constraintErr := semver.NewConstraint(">= 0.1.0")
			if constraintErr != nil {
				// Constraint parsing failed, fall back to CI check
				return os.Getenv("CI") == "true"
			}
			if constraint.Check(v) {
				return true
			}
		}
	}
	// Fallback to CI check
	return os.Getenv("CI") == "true"
}
