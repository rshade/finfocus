package pluginhost

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// CompatibilityResult represents the outcome of a version compatibility check.
type CompatibilityResult int

const (
	// Compatible indicates the versions are compatible.
	Compatible CompatibilityResult = iota
	// MajorMismatch indicates a major version difference (likely incompatible).
	MajorMismatch
	// Invalid indicates one or both version strings are invalid.
	Invalid
)

// String returns the human-readable name of the CompatibilityResult.
// It implements the [fmt.Stringer] interface for use in logs and debug output.
// Returns "Compatible", "MajorMismatch", "Invalid", or "CompatibilityResult(n)" for unknown values.
func (r CompatibilityResult) String() string {
	switch r {
	case Compatible:
		return "Compatible"
	case MajorMismatch:
		return "MajorMismatch"
	case Invalid:
		return "Invalid"
	default:
		return fmt.Sprintf("CompatibilityResult(%d)", r)
	}
}

// CompareSpecVersions compares the core spec version with the plugin's spec version.
// It returns a compatibility result and an error if parsing fails.
func CompareSpecVersions(coreVersion, pluginVersion string) (CompatibilityResult, error) {
	cVer, err := semver.NewVersion(coreVersion)
	if err != nil {
		return Invalid, fmt.Errorf("invalid core version: %w", err)
	}

	pVer, err := semver.NewVersion(pluginVersion)
	if err != nil {
		return Invalid, fmt.Errorf("invalid plugin version: %w", err)
	}

	if cVer.Major() != pVer.Major() {
		return MajorMismatch, nil
	}

	return Compatible, nil
}
