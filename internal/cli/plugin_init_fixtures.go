package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

const (
	defaultFixtureBaseURL = "https://raw.githubusercontent.com/rshade/finfocus"
	githubAPIURL          = "https://api.github.com/repos/rshade/finfocus/releases"
	fixtureTimeout        = 30 * time.Second
)

type FixtureSource struct {
	ID       string
	Type     string
	Provider string
	Version  string
	Origin   string
	Checksum string
}

type FixtureResolver struct {
	logger            zerolog.Logger
	offlineMode       bool
	fixtureVersion    string
	localBasePath     string
	releaseTagFetcher func(context.Context) (string, error)
}

// NewFixtureResolver creates a FixtureResolver configured to resolve fixtures.
//
// logger is used for structured logging. offline toggles local-only resolution when true.
// version selects the fixture version to resolve (e.g., "latest", a tag, "main", or "local").
// localBase sets the base filesystem path to search for local fixtures.
// opts are optional configuration functions that can customize the resolver behavior.
//
// The returned FixtureResolver is ready to resolve and (when not offline) download fixture sources.
func NewFixtureResolver(
	logger zerolog.Logger,
	offline bool,
	version, localBase string,
	opts ...func(*FixtureResolver),
) *FixtureResolver {
	resolver := &FixtureResolver{
		logger:            logger,
		offlineMode:       offline,
		fixtureVersion:    version,
		localBasePath:     localBase,
		releaseTagFetcher: fetchLatestReleaseTag, // Default implementation
	}

	// Apply optional configuration
	for _, opt := range opts {
		opt(resolver)
	}

	return resolver
}

// WithReleaseTagFetcher configures a custom release tag fetcher function.
func WithReleaseTagFetcher(
	fetcher func(context.Context) (string, error),
) func(*FixtureResolver) {
	return func(r *FixtureResolver) {
		r.releaseTagFetcher = fetcher
	}
}

// findFirstExistingPath iterates through the provided paths, converts each to an absolute path,
// and returns the first one that exists on the filesystem.
// Returns empty string and error if no paths exist or all conversions fail.
func (r *FixtureResolver) findFirstExistingPath(paths []string) (string, error) {
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, statErr := os.Stat(absPath); statErr == nil {
			return absPath, nil
		}
	}
	return "", errors.New("no existing path found in provided list")
}

func (r *FixtureResolver) ResolvePlanFixture(ctx context.Context, provider string) (*FixtureSource, error) {
	if r.offlineMode {
		return r.resolveLocalPlanFixture(provider)
	}
	return r.resolveRemotePlanFixture(ctx, provider)
}

func (r *FixtureResolver) ResolveStateFixture(ctx context.Context) (*FixtureSource, error) {
	if r.offlineMode {
		return r.resolveLocalStateFixture()
	}
	return r.resolveRemoteStateFixture(ctx)
}

func (r *FixtureResolver) resolveRemotePlanFixture(ctx context.Context, provider string) (*FixtureSource, error) {
	version := r.fixtureVersion
	if version == "latest" {
		var err error
		version, err = r.releaseTagFetcher(ctx)
		if err != nil {
			r.logger.Warn().Err(err).Msg("failed to fetch latest release, using main branch")
			version = "main"
		}
	}

	origin := fmt.Sprintf("%s/%s/test/fixtures/plans/%s/simple.json", defaultFixtureBaseURL, version, provider)

	source := &FixtureSource{
		ID:       fmt.Sprintf("plan-%s-%s", provider, version),
		Type:     "plan",
		Provider: provider,
		Version:  version,
		Origin:   origin,
	}

	r.logger.Info().
		Str("provider", provider).
		Str("version", version).
		Str("url", origin).
		Msg("resolved remote plan fixture")

	return source, nil
}

func (r *FixtureResolver) resolveLocalPlanFixture(provider string) (*FixtureSource, error) {
	paths := []string{
		filepath.Join(r.localBasePath, "plans", provider, "simple.json"),
		filepath.Join(r.localBasePath, "fixtures", "plans", provider, "simple.json"),
		filepath.Join(r.localBasePath, "..", "fixtures", "plans", provider, "simple.json"),
	}

	absPath, err := r.findFirstExistingPath(paths)
	if err != nil {
		return nil, fmt.Errorf("local plan fixture not found for provider %s", provider)
	}

	source := &FixtureSource{
		ID:       fmt.Sprintf("plan-%s-local", provider),
		Type:     "plan",
		Provider: provider,
		Version:  "local",
		Origin:   absPath,
	}
	r.logger.Info().
		Str("provider", provider).
		Str("path", absPath).
		Msg("resolved local plan fixture")
	return source, nil
}

func (r *FixtureResolver) resolveRemoteStateFixture(ctx context.Context) (*FixtureSource, error) {
	version := r.fixtureVersion
	if version == "latest" {
		var err error
		version, err = r.releaseTagFetcher(ctx)
		if err != nil {
			r.logger.Warn().Err(err).Msg("failed to fetch latest release, using main branch")
			version = "main"
		}
	}

	origin := fmt.Sprintf("%s/%s/test/fixtures/state/valid-state.json", defaultFixtureBaseURL, version)

	source := &FixtureSource{
		ID:      fmt.Sprintf("state-%s", version),
		Type:    "state",
		Version: version,
		Origin:  origin,
	}

	r.logger.Info().
		Str("version", version).
		Str("url", origin).
		Msg("resolved remote state fixture")

	return source, nil
}

func (r *FixtureResolver) resolveLocalStateFixture() (*FixtureSource, error) {
	paths := []string{
		filepath.Join(r.localBasePath, "state", "valid-state.json"),
		filepath.Join(r.localBasePath, "fixtures", "state", "valid-state.json"),
		filepath.Join(r.localBasePath, "..", "fixtures", "state", "valid-state.json"),
	}

	absPath, err := r.findFirstExistingPath(paths)
	if err != nil {
		return nil, errors.New("local state fixture not found")
	}

	source := &FixtureSource{
		ID:      "state-local",
		Type:    "state",
		Version: "local",
		Origin:  absPath,
	}
	r.logger.Info().
		Str("path", absPath).
		Msg("resolved local state fixture")
	return source, nil
}

func (r *FixtureResolver) DownloadFixture(ctx context.Context, source *FixtureSource) (string, error) {
	if r.offlineMode {
		return source.Origin, nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, fixtureTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxWithTimeout, http.MethodGet, source.Origin, nil)
	if err != nil {
		return "", fmt.Errorf("fetching fixture %s: %w", source.Origin, err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching fixture %s: %w", source.Origin, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching fixture %s: HTTP %d", source.Origin, resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading fixture %s: %w", source.Origin, err)
	}

	tempFile, err := os.CreateTemp("", "finfocus-fixture-*.json")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	if _, writeErr := tempFile.Write(content); writeErr != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("writing temp file: %w", writeErr)
	}

	if syncErr := tempFile.Sync(); syncErr != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("syncing temp file: %w", syncErr)
	}

	if closeErr := tempFile.Close(); closeErr != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("closing temp file: %w", closeErr)
	}

	r.logger.Debug().
		Str("source", source.Origin).
		Str("temp_file", tempFile.Name()).
		Msg("downloaded fixture")

	return tempFile.Name(), nil
}

// fetchLatestReleaseTag queries the GitHub Releases API for the repository's latest release and returns its tag name.
// The provided context is used for cancellation and the request is bounded by fixtureTimeout.
// It returns the release tag on success or an error if the HTTP request fails, the response status is not 200, or the JSON response cannot be decoded.
func fetchLatestReleaseTag(ctx context.Context) (string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, fixtureTimeout)
	defer cancel()

	client := &http.Client{Timeout: fixtureTimeout}

	req, err := http.NewRequestWithContext(ctxWithTimeout, http.MethodGet, githubAPIURL+"/latest", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&release); decodeErr != nil {
		return "", decodeErr
	}

	return release.TagName, nil
}
