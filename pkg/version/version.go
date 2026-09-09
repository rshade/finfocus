// Package version provides build version and metadata information for finfocus.
// Version information is embedded at build time via ldflags from the CI/CD pipeline.
package version

import "github.com/rshade/ax-go"

// Build information set during compilation.
var (
	version   = "0.1.0"
	gitCommit = "unknown" //nolint:gochecknoglobals // Build system sets this via ldflags
	buildDate = "unknown" //nolint:gochecknoglobals // Build system sets this via ldflags
)

// GetVersion returns the version string. It is resolved through ax.ResolveVersion
// so that if version is ever left empty (no ldflags injection), it falls back
// through Go build metadata instead of surfacing a blank version to agents.
func GetVersion() string {
	return ax.ResolveVersion(version)
}

// GetGitCommit returns the git commit hash.
func GetGitCommit() string {
	return gitCommit
}

// GetBuildDate returns the build date.
func GetBuildDate() string {
	return buildDate
}
