package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	osWindows = "windows"
	extExe    = ".exe"
	extZip    = ".zip"
	extTarGz  = ".tar.gz"

	// downloadTimeout is the timeout for large plugin downloads (15MB+).
	downloadTimeout = 5 * time.Minute

	// githubMaxPerPage is the maximum items per page for GitHub API pagination.
	githubMaxPerPage = 100
)

// GitHubRelease represents release metadata from GitHub API.
type GitHubRelease struct {
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []ReleaseAsset `json:"assets"`
}

// ReleaseAsset represents a downloadable asset.
type ReleaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// FallbackInfo contains metadata about release selection, including whether
// a fallback occurred during version resolution.
type FallbackInfo struct {
	// Release is the GitHub release that was selected
	Release *GitHubRelease
	// Asset is the platform-compatible asset from the release
	Asset *ReleaseAsset
	// WasFallback is true if the selected version differs from the requested version
	WasFallback bool
	// RequestedVersion is the original version that was requested (empty if @latest)
	RequestedVersion string
	// FallbackReason explains why fallback occurred (e.g., "no compatible assets")
	FallbackReason string
}

// GitHubClient provides GitHub API access for releases.
type GitHubClient struct {
	HTTPClient *http.Client
	BaseURL    string
	token      string
}

// NewGitHubClient creates and returns a GitHubClient configured to access the GitHub API.
// The client has an HTTP client with a 5-minute timeout (for large plugin downloads) and
// BaseURL set to https://api.github.com. It initializes the authentication token by reading
// NewGitHubClient creates and returns a GitHubClient configured for the GitHub API.
// It sets BaseURL to "https://api.github.com", constructs an HTTP client using the
// package downloadTimeout for request timeouts, and initializes the client's token
// from the GITHUB_TOKEN environment variable or, if unset, by attempting to obtain
// it from the `gh` CLI.
func NewGitHubClient() *GitHubClient {
	token := getGitHubToken()
	return &GitHubClient{
		HTTPClient: &http.Client{
			Timeout: downloadTimeout,
		},
		BaseURL: "https://api.github.com",
		token:   token,
	}
}

// getGitHubToken returns a GitHub authentication token from the GITHUB_TOKEN
// environment variable. Returns empty string if not set. The token is optional
// - anonymous requests will work for public repositories but may be rate-limited.
func getGitHubToken() string {
	return os.Getenv("GITHUB_TOKEN")
}

// GetLatestRelease returns the latest release for a repository.
func (c *GitHubClient) GetLatestRelease(owner, repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.BaseURL, owner, repo)
	return c.fetchRelease(url)
}

// GetReleaseByTag returns a specific release by tag name.
func (c *GitHubClient) GetReleaseByTag(owner, repo, tag string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.BaseURL, owner, repo, tag)
	return c.fetchRelease(url)
}

// ListStableReleases fetches all releases and returns only stable (non-draft, non-prerelease)
// releases sorted by creation order (newest first, as returned by GitHub API).
//
//nolint:noctx // context not needed for simple HTTP
func (c *GitHubClient) ListStableReleases(owner, repo string, limit int) ([]GitHubRelease, error) {
	// Fetch paginated releases (up to limit, max githubMaxPerPage per page)
	url := fmt.Sprintf(
		"%s/repos/%s/%s/releases?per_page=%d",
		c.BaseURL, owner, repo, min(limit, githubMaxPerPage),
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("repository not found")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, errors.New(
			"GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits",
		)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var allReleases []GitHubRelease
	if decodeErr := json.NewDecoder(resp.Body).Decode(&allReleases); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	// Filter to stable releases only (non-draft, non-prerelease)
	var stableReleases []GitHubRelease
	for _, release := range allReleases {
		if !release.Draft && !release.Prerelease {
			stableReleases = append(stableReleases, release)
			if len(stableReleases) >= limit {
				break
			}
		}
	}

	return stableReleases, nil
}

// FindReleaseWithAsset attempts to find a release with a matching platform asset.
// If version is specified, it first tries that exact version. If no asset is found
// for the requested version, or if version is empty, it falls back to searching
// through recent stable releases to find one with a compatible asset.
//
// Parameters:
//   - owner, repo: GitHub repository owner and name
//   - version: specific version to try first (empty string for latest)
//   - projectName: project name used in asset filenames
//   - hints: optional naming hints for asset matching
//
// Returns the release and asset that matched, or an error if none found.
func (c *GitHubClient) FindReleaseWithAsset(
	owner, repo, version, projectName string,
	hints *AssetNamingHints,
) (*GitHubRelease, *ReleaseAsset, error) {
	info, err := c.FindReleaseWithFallbackInfo(owner, repo, version, projectName, hints)
	if err != nil {
		return nil, nil, err
	}
	return info.Release, info.Asset, nil
}

// FindReleaseWithFallbackInfo attempts to find a release with a matching platform asset,
// returning detailed information about whether a fallback occurred.
//
// If version is specified, it first tries that exact version. If no asset is found
// for the requested version, or if version is empty, it falls back to searching
// through recent stable releases to find one with a compatible asset.
//
// Parameters:
//   - owner, repo: GitHub repository owner and name
//   - version: specific version to try first (empty string for latest)
//   - projectName: project name used in asset filenames
//   - hints: optional naming hints for asset matching
//
// Returns FallbackInfo containing:
//   - Release and Asset that matched
//   - WasFallback: true if the returned version differs from the requested version
//   - RequestedVersion: the original version that was requested
//   - FallbackReason: explanation if fallback occurred
//
//nolint:gocognit // Complexity is necessary to handle version lookup, fallback search, and asset matching scenarios
func (c *GitHubClient) FindReleaseWithFallbackInfo(
	owner, repo, version, projectName string,
	hints *AssetNamingHints,
) (*FallbackInfo, error) {
	const maxFallbackReleases = 10

	info := &FallbackInfo{
		RequestedVersion: version,
	}

	// If specific version requested, try it first
	if version != "" {
		release, err := c.GetReleaseByTag(owner, repo, version)
		if err == nil {
			asset, assetErr := FindPlatformAssetWithHints(release, projectName, hints)
			if assetErr == nil {
				// Found exact match - no fallback needed
				info.Release = release
				info.Asset = asset
				info.WasFallback = false
				return info, nil
			}
			// Asset not found for requested version, will try fallback
			info.FallbackReason = "no compatible assets"
		} else {
			// Release not found
			info.FallbackReason = "release not found"
		}
	}

	// Try stable releases as fallback
	stableReleases, err := c.ListStableReleases(owner, repo, maxFallbackReleases)
	if err != nil {
		if version != "" {
			return nil, fmt.Errorf(
				"version %s not available and failed to list fallback releases: %w",
				version, err,
			)
		}
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	if len(stableReleases) == 0 {
		return nil, errors.New("no stable releases found")
	}

	// Try each stable release until we find one with a matching asset
	var lastAssetErr error
	for i := range stableReleases {
		release := &stableReleases[i]
		asset, assetErr := FindPlatformAssetWithHints(release, projectName, hints)
		if assetErr == nil {
			info.Release = release
			info.Asset = asset
			// It's a fallback if we had a specific version requested and got a different one
			info.WasFallback = version != "" && release.TagName != version
			if !info.WasFallback && version == "" {
				// When no version specified, check if this was the "latest" release
				// If we're iterating through stable releases, first one is latest
				info.WasFallback = i > 0
				if info.WasFallback {
					info.FallbackReason = "latest version lacks compatible assets"
				}
			}
			return info, nil
		}
		lastAssetErr = assetErr
	}

	if version != "" {
		return nil, fmt.Errorf(
			"no compatible asset found for version %s or any of %d fallback releases: %w",
			version, len(stableReleases), lastAssetErr,
		)
	}
	return nil, fmt.Errorf(
		"no compatible asset found in %d stable releases: %w",
		len(stableReleases), lastAssetErr,
	)
}

// fetchRelease fetches release data with retry logic.
//
//nolint:noctx // context not needed for simple HTTP
func (c *GitHubClient) fetchRelease(url string) (*GitHubRelease, error) {
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s
			backoffDivisor := 2
			time.Sleep(time.Duration(1<<attempt) * time.Second / time.Duration(backoffDivisor))
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if c.token != "" {
			req.Header.Set("Authorization", "token "+c.token)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			return nil, errors.New("release not found")
		}
		if resp.StatusCode == http.StatusForbidden {
			_ = resp.Body.Close()
			return nil, errors.New(
				"GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits",
			)
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}

		var release GitHubRelease
		if decodeErr := json.NewDecoder(resp.Body).Decode(&release); decodeErr != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
		}
		_ = resp.Body.Close()
		return &release, nil
	}
	return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// FindPlatformAsset locates the release asset matching the current OS and architecture for the given project.
// It tries multiple naming conventions to handle different GoReleaser configurations:
//   - Standard: {project}_{version}_{os}_{arch}.{ext}
//   - Capitalized OS: {project}_{version}_{Os}_{arch}.{ext}
//   - x86_64 arch: {project}_{version}_{os}_x86_64.{ext}
//   - Region-specific: {project}_{version}_{Os}_{arch}_{region}.{ext}
//
// release is the GitHubRelease to search; projectName is the project name used in asset filenames.
// It returns the matching ReleaseAsset or an error that includes the available asset names when no match is found.
func FindPlatformAsset(release *GitHubRelease, projectName string) (*ReleaseAsset, error) {
	return FindPlatformAssetWithHints(release, projectName, nil)
}

// AssetNamingHints provides hints for matching assets with non-standard naming conventions.
type AssetNamingHints struct {
	// AssetPrefix is the project name prefix used in asset filenames
	// (e.g., "finfocus-plugin-aws-public" instead of just "aws-public")
	AssetPrefix string
	// Region specifies a region suffix to match (e.g., "us-east-1")
	Region string
	// VersionPrefix if false, version in asset name has no "v" prefix
	VersionPrefix bool
}

// FindPlatformAssetWithHints locates the release asset with custom naming hints.
func FindPlatformAssetWithHints(
	release *GitHubRelease,
	projectName string,
	hints *AssetNamingHints,
) (*ReleaseAsset, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Determine expected extension
	ext := extTarGz
	if goos == osWindows {
		ext = extZip
	}

	version := release.TagName

	// Build list of patterns to try (in order of preference)
	patterns := buildAssetPatterns(projectName, version, goos, goarch, ext, hints)

	// Try each pattern
	for _, pattern := range patterns {
		for i := range release.Assets {
			if matchesAssetPattern(release.Assets[i].Name, pattern) {
				return &release.Assets[i], nil
			}
		}
	}

	// List available assets for error message
	var available []string
	for _, asset := range release.Assets {
		available = append(available, asset.Name)
	}

	return nil, fmt.Errorf("no asset found for %s/%s. Available: %v", goos, goarch, available)
}

// buildAssetPatterns generates a list of asset name patterns to try.
//
//nolint:gocognit // Complexity is necessary to handle all naming convention variations
func buildAssetPatterns(
	projectName, version, goos, goarch, ext string,
	hints *AssetNamingHints,
) []string {
	// Project name variations (use asset prefix if provided)
	projectNames := []string{projectName}
	if hints != nil && hints.AssetPrefix != "" && hints.AssetPrefix != projectName {
		// Try asset prefix first (more specific)
		projectNames = []string{hints.AssetPrefix, projectName}
	}

	// OS name variations
	titleCaser := cases.Title(language.Und)
	osNames := []string{
		goos,                                 // linux, darwin, windows
		titleCaser.String(goos),              // Linux, Darwin, Windows
		strings.ToUpper(goos[:1]) + goos[1:], // Linux, Darwin, Windows (manual title case)
	}
	if goos == "darwin" {
		osNames = append(osNames, "Darwin", "macos", "macOS", "MacOS")
	}

	// Architecture variations
	archNames := []string{goarch} // amd64, arm64
	if goarch == "amd64" {
		archNames = append(archNames, "x86_64", "X86_64", "AMD64")
	}
	if goarch == "arm64" {
		archNames = append(archNames, "ARM64", "aarch64", "AARCH64")
	}

	// Version variations (with and without v prefix)
	versions := []string{version}
	if strings.HasPrefix(version, "v") {
		versions = append(versions, strings.TrimPrefix(version, "v"))
	} else {
		versions = append(versions, "v"+version)
	}

	// Region suffixes
	regions := []string{""}
	if hints != nil && hints.Region != "" {
		regions = []string{"_" + hints.Region, ""} // Try with region first
	}

	// Generate all combinations
	var patterns []string
	for _, pName := range projectNames {
		for _, ver := range versions {
			for _, osName := range osNames {
				for _, arch := range archNames {
					for _, region := range regions {
						pattern := fmt.Sprintf(
							"%s_%s_%s_%s%s%s",
							pName,
							ver,
							osName,
							arch,
							region,
							ext,
						)
						patterns = append(patterns, pattern)
					}
				}
			}
		}
	}

	return patterns
}

// matchesAssetPattern checks if an asset name matches a pattern.
// Currently uses exact match but could be extended for glob/regex support.
func matchesAssetPattern(assetName, pattern string) bool {
	return assetName == pattern
}

// DownloadAsset downloads a release asset to a local file.
//
//nolint:mnd,noctx // magic numbers for buffer size, context not needed for downloads
func (c *GitHubClient) DownloadAsset(
	url, destPath string,
	progress func(downloaded, total int64),
) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write: %w", writeErr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read error: %w", readErr)
		}
	}

	return nil
}
