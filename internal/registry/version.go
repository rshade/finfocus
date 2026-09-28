package registry

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// VersionConstraint represents a semantic version constraint for dependencies.
type VersionConstraint struct {
	Raw        string
	Constraint *semver.Constraints
}

// ParseVersionConstraint parses a version constraint string.
// Supported formats:
//   - ">=1.0.0" - Greater than or equal
//   - "<2.0.0" - Less than
//   - ">=1.0.0,<2.0.0" - Range (AND)
//   - "~1.2.3" - Patch-level changes (>=1.2.3,<1.3.0)
//
// ParseVersionConstraint parses a semantic version constraint string and returns a VersionConstraint
// containing the original raw string and the parsed semver.Constraints.
//
// If s is empty, an error is returned. If parsing fails, an error describing the invalid constraint
// and wrapping the underlying semver parse error is returned.
func ParseVersionConstraint(s string) (*VersionConstraint, error) {
	if s == "" {
		return nil, errors.New("empty version constraint")
	}

	constraint, err := semver.NewConstraint(s)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint %q: %w", s, err)
	}

	return &VersionConstraint{
		Raw:        s,
		Constraint: constraint,
	}, nil
}

// SatisfiesConstraint reports whether the provided semantic version satisfies the given VersionConstraint.
// It accepts a version string (a leading "v" is ignored) and evaluates it against the parsed constraint.
// If the constraint or its parsed representation is nil, an error is returned.
// If the version cannot be parsed as a semantic version, an error describing the invalid version is returned.
// The boolean result is true when the version satisfies the constraint, false otherwise.
func SatisfiesConstraint(version string, constraint *VersionConstraint) (bool, error) {
	if constraint == nil || constraint.Constraint == nil {
		return false, errors.New("nil version constraint")
	}

	// Strip 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	v, err := semver.NewVersion(version)
	if err != nil {
		return false, fmt.Errorf("invalid version %q: %w", version, err)
	}

	return constraint.Constraint.Check(v), nil
}

// CompareVersions compares two versions.
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//
// CompareVersions compares two semantic version strings and returns -1, 0, or 1.
// It ignores a leading 'v' prefix on each version before parsing.
// v1 and v2 are the version strings to compare.
// The returned int is -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
// An error is returned if either input is not a valid semantic version.
func CompareVersions(v1, v2 string) (int, error) {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	ver1, err := semver.NewVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v1, err)
	}

	ver2, err := semver.NewVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v2, err)
	}

	return ver1.Compare(ver2), nil
}

// IsValidVersion reports whether the provided string is a valid semantic version.
// A leading "v" is ignored; the function returns true if the remainder parses as a semantic version, false otherwise.
func IsValidVersion(version string) bool {
	version = strings.TrimPrefix(version, "v")
	_, err := semver.NewVersion(version)
	return err == nil
}

// CanonicalVersion strips a monorepo tag prefix, turning "kubernetes-v0.1.0"
// into "v0.1.0". Tags without the prefix are returned unchanged.
func CanonicalVersion(tag, prefix string) string {
	if prefix == "" {
		return tag
	}
	return strings.TrimPrefix(tag, prefix)
}

// ReleaseTag converts a user-supplied version into the release tag to fetch.
// With a prefix, "0.1.0" and "v0.1.0" both become "<prefix>v0.1.0".
func ReleaseTag(version, prefix string) string {
	if prefix == "" || strings.HasPrefix(version, prefix) {
		return version
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return prefix + version
}

// tagPrefixOf returns the tag prefix carried by h, or "" when h is nil.
func tagPrefixOf(h *AssetNamingHints) string {
	if h == nil {
		return ""
	}
	return h.TagPrefix
}
