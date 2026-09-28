package registry

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// RegistryEntry represents a plugin in the embedded registry.
//
//nolint:revive // type stuttering is acceptable for clarity
type RegistryEntry struct {
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Repository         string              `json:"repository"`
	Author             string              `json:"author"`
	License            string              `json:"license"`
	Homepage           string              `json:"homepage"`
	SupportedProviders []string            `json:"supported_providers"`
	Capabilities       []string            `json:"capabilities"`
	SecurityLevel      string              `json:"security_level"`
	MinSpecVersion     string              `json:"min_spec_version"`
	AssetHints         *RegistryAssetHints `json:"asset_hints,omitempty"`
	// TagPrefix marks a plugin released from a monorepo under prefixed tags,
	// e.g. "kubernetes-" for tag "kubernetes-v0.1.0". Empty for single-plugin repos.
	TagPrefix string `json:"tag_prefix,omitempty"`
}

// RegistryAssetHints provides hints for asset naming conventions specific to a plugin.
//
//nolint:revive // name matches JSON field convention in registry.json
type RegistryAssetHints struct {
	// AssetPrefix is the project name prefix used in asset filenames
	// (e.g., "finfocus-plugin-aws-public" for assets named
	// "finfocus-plugin-aws-public_0.0.6_Linux_x86_64.tar.gz")
	AssetPrefix string `json:"asset_prefix,omitempty"`
	// DefaultRegion is the region suffix to use when downloading (e.g., "us-east-1")
	DefaultRegion string `json:"default_region,omitempty"`
	// VersionPrefix indicates if the version in asset names has a "v" prefix
	VersionPrefix bool `json:"version_prefix,omitempty"`
}

// PluginSpecifier represents a parsed plugin specifier (name or URL with optional version).
type PluginSpecifier struct {
	Name    string
	Version string
	IsURL   bool
	Owner   string
	Repo    string
}

var (
	repoPattern   = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)
	githubPattern = regexp.MustCompile(`^github\.com/([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)$`)
	// tagPrefixPattern excludes prefixes starting with "v" so a monorepo
	// plugin tag (e.g. "vantage-v0.1.0") can never be mistaken for a
	// bare CLI tag by the `startsWith(tag, 'v')` guards in the release
	// workflows.
	tagPrefixPattern = regexp.MustCompile(`^[a-uw-z0-9][a-z0-9-]*-$`)
)

// ValidateRegistryEntry checks that the required fields of a RegistryEntry are present and well-formed.
// It returns an error describing the first validation failure encountered or nil if the entry is valid.
//
// The following validations are performed:
//   - `Name` must not be empty.
//   - `Repository` must not be empty and must match the "owner/repo" format.
//   - If `SecurityLevel` is set, it must be one of "official", "community", or "experimental".
//
// ValidateRegistryEntry validates required fields and formats of a RegistryEntry.
// It checks that Name and Repository are present, that Repository matches the
// "owner/repo" pattern, and that SecurityLevel (if set) is one of "official",
// "community", or "experimental".
// On failure it returns an error describing which field is invalid and includes
// the entry name when available. On success it returns nil.
func ValidateRegistryEntry(entry RegistryEntry) error {
	if entry.Name == "" {
		return errors.New("registry entry missing required field: name")
	}
	if entry.Repository == "" {
		return fmt.Errorf("registry entry %q missing required field: repository", entry.Name)
	}
	if !repoPattern.MatchString(entry.Repository) {
		return fmt.Errorf(
			"registry entry %q has invalid repository format: %s (expected owner/repo)",
			entry.Name,
			entry.Repository,
		)
	}
	validLevels := map[string]bool{"official": true, "community": true, "experimental": true}
	if entry.SecurityLevel != "" && !validLevels[entry.SecurityLevel] {
		return fmt.Errorf(
			"registry entry %q has invalid security_level: %s",
			entry.Name,
			entry.SecurityLevel,
		)
	}
	if entry.TagPrefix != "" && !tagPrefixPattern.MatchString(entry.TagPrefix) {
		return fmt.Errorf("invalid tag_prefix %q for %s: must match %s",
			entry.TagPrefix, entry.Name, tagPrefixPattern.String())
	}
	return nil
}

// ParsePluginSpecifier parses a plugin specifier string.
// Formats:
//   - "kubecost" - registry plugin, latest version
//   - "kubecost@v1.0.0" - registry plugin, specific version
//   - "github.com/owner/repo" - GitHub URL, latest version
//
// ParsePluginSpecifier parses a plugin specifier string into a PluginSpecifier.
// It accepts either a registry name or a GitHub URL with an optional version suffix
// separated by `@` (for example: `kubecost`, `kubecost@v1.0.0`, `github.com/owner/repo`, or
// `github.com/owner/repo@v1.0.0`).
//
// The `spec` parameter is the plugin specifier to parse. If `spec` is empty, the function
// returns an error. When the input is a GitHub URL, the returned PluginSpecifier has
// IsURL set to true, Owner and Repo populated from the URL, and Name derived from the
// repository name with a leading `finfocus-plugin-` prefix removed (if present).
// When the input is a registry name, IsURL is false and Name is set to the given name.
// The Version field is set if a `@version` suffix is provided; otherwise it is empty.
//
// The function returns an error if `spec` is empty or if a GitHub URL does not match
// the expected `github.com/owner/repo` format.
func ParsePluginSpecifier(spec string) (*PluginSpecifier, error) {
	if spec == "" {
		return nil, errors.New("empty plugin specifier")
	}

	// Split by @ to extract version
	parts := strings.SplitN(spec, "@", 2) //nolint:mnd // split into 2 parts
	nameOrURL := parts[0]
	version := ""
	if len(parts) == 2 { //nolint:mnd // check for version part
		version = parts[1]
	}

	// Check if it's a GitHub URL
	if strings.HasPrefix(nameOrURL, "github.com/") {
		matches := githubPattern.FindStringSubmatch(nameOrURL)
		if matches == nil {
			return nil, fmt.Errorf("invalid GitHub URL format: %s", nameOrURL)
		}
		// Derive plugin name from repo name
		repoName := matches[2]
		pluginName := strings.TrimPrefix(repoName, "finfocus-plugin-")

		return &PluginSpecifier{
			Name:    pluginName,
			Version: version,
			IsURL:   true,
			Owner:   matches[1],
			Repo:    matches[2],
		}, nil
	}

	// It's a registry name
	return &PluginSpecifier{
		Name:    nameOrURL,
		Version: version,
		IsURL:   false,
	}, nil
}

// ParseGitHubURL extracts owner and repo from a GitHub URL.
//
// ParseGitHubURL extracts the owner and repository name from a GitHub URL of the form "github.com/owner/repo".
// It returns the captured owner and repo strings.
// If the input does not match the expected GitHub URL format, it returns a non-nil error.
func ParseGitHubURL(url string) (string, string, error) {
	matches := githubPattern.FindStringSubmatch(url)
	if matches == nil {
		return "", "", fmt.Errorf("invalid GitHub URL format: %s", url)
	}
	return matches[1], matches[2], nil
}
