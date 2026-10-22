package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

// ErrCatalogSpecifier is returned when a catalog install is asked for a URL
// or an owner/repo slug. Catalog installs accept a registry name only.
var ErrCatalogSpecifier = errors.New("catalog installs accept a registry name only")

const (
	executablePermissionMask = 0111 // File permission mask for executable files
)

// InstallOptions configures plugin installation behavior.
type InstallOptions struct {
	Force            bool              // Reinstall even if version exists
	NoSave           bool              // Don't add to config file
	PluginDir        string            // Custom plugin directory (default: DefaultPluginDir)
	FallbackToLatest bool              // Automatically install latest stable version if requested version lacks assets
	NoFallback       bool              // Disable fallback behavior entirely (fail if requested version lacks assets)
	Metadata         map[string]string // User-supplied metadata (e.g., region=us-west-2), stored as plugin.metadata.json
	SkipChecksum     bool              // SkipChecksum bypasses SHA256 checksum verification during installation
	// OnEvent receives a typed progress event beside the string callback.
	// A nil OnEvent leaves the string callback as the only progress channel.
	OnEvent func(ProgressEvent)
	// CatalogOnly rejects URL and owner/repo specifiers and ignores SkipChecksum.
	CatalogOnly bool
	// RequireChecksumFile fails the install when checksums.txt is absent,
	// unless AcceptMissingChecksum is set. A hash mismatch stays fatal.
	RequireChecksumFile bool
	// AcceptMissingChecksum continues when checksums.txt is absent.
	AcceptMissingChecksum bool
	// AcceptFallbackVersion is the canonical version the caller confirmed
	// when the latest release has no assets. Any other value keeps the
	// no-asset error instead of installing a different release.
	AcceptFallbackVersion string
}

// InstallResult contains the result of a plugin installation.
type InstallResult struct {
	Name             string
	Version          string
	Path             string
	FromURL          bool
	Repository       string
	WasFallback      bool   // True if installed version differs from requested
	RequestedVersion string // Original version requested (empty if @latest)
}

// Installer handles plugin installation from registry or URLs.
type Installer struct {
	client    *GitHubClient
	pluginDir string
}

// HintsForEntry builds asset naming hints from a registry entry.
// It returns nil when the entry carries no hints and no tag prefix.
func HintsForEntry(entry *RegistryEntry) *AssetNamingHints {
	if entry == nil || (entry.AssetHints == nil && entry.TagPrefix == "") {
		return nil
	}
	h := &AssetNamingHints{TagPrefix: entry.TagPrefix}
	if entry.AssetHints != nil {
		h.AssetPrefix = entry.AssetHints.AssetPrefix
		h.Region = entry.AssetHints.DefaultRegion
		h.VersionPrefix = entry.AssetHints.VersionPrefix
	}
	return h
}

// hintsForRepository builds asset naming hints for a plugin installed by URL.
// Such a plugin has no catalog entry to supply an asset prefix, and release
// assets are conventionally prefixed with the repository name.
func hintsForRepository(repo string) *AssetNamingHints {
	if repo == "" {
		return nil
	}
	return &AssetNamingHints{AssetPrefix: repo}
}

// NewInstaller creates an Installer with a default GitHub client that installs
// plugins into pluginDir. An empty pluginDir resolves to DefaultPluginDir.
func NewInstaller(pluginDir string) *Installer {
	return NewInstallerWithClient(NewGitHubClient(), pluginDir)
}

// NewInstallerWithClient creates an Installer that uses client and installs
// plugins into pluginDir. An empty pluginDir resolves to DefaultPluginDir.
func NewInstallerWithClient(client *GitHubClient, pluginDir string) *Installer {
	if pluginDir == "" {
		pluginDir = DefaultPluginDir()
	}
	return &Installer{
		client:    client,
		pluginDir: pluginDir,
	}
}

// DefaultPluginDir returns the directory plugin discovery reads: FINFOCUS_PLUGIN_DIR,
// then the plugin_dir config key, then the plugins directory under the resolved
// FinFocus home (FINFOCUS_HOME, then PULUMI_HOME/finfocus, then ~/.finfocus).
// NewDefault uses the same value, so installs land where discovery looks.
func DefaultPluginDir() string {
	return config.New().PluginDir
}

// Install installs a plugin from a specifier (name or URL with optional version).
func (i *Installer) Install(
	ctx context.Context,
	specifier string,
	opts InstallOptions,
	progress func(msg string),
) (*InstallResult, error) {
	spec, err := ParsePluginSpecifier(specifier)
	if err != nil {
		return nil, err
	}
	if opts.CatalogOnly {
		opts.SkipChecksum = false
		if spec.IsURL || strings.Contains(spec.Name, "/") {
			return nil, fmt.Errorf("%w: %q", ErrCatalogSpecifier, specifier)
		}
	}

	report := newProgressReport(progress, opts.OnEvent)

	// Acquire lock for this plugin
	unlock, err := i.acquireLock(spec.Name)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if spec.IsURL {
		return i.installFromURL(ctx, spec, opts, report)
	}
	return i.installFromRegistry(ctx, spec, opts, report)
}

// installFromRegistry installs a plugin from the embedded registry.
func (i *Installer) installFromRegistry(
	ctx context.Context,
	spec *PluginSpecifier,
	opts InstallOptions,
	report progressReport,
) (*InstallResult, error) {
	// Look up plugin in registry
	entry, err := GetPlugin(spec.Name)
	if err != nil {
		// Check if this is a "not found" error vs other registry errors
		if strings.Contains(err.Error(), "not found in registry") {
			return nil, fmt.Errorf("plugin %q not found in registry", spec.Name)
		}
		return nil, fmt.Errorf("failed to access registry: %w", err)
	}

	// Validate entry
	if validateErr := ValidateRegistryEntry(*entry); validateErr != nil {
		return nil, validateErr
	}

	// Parse repository
	owner, repo, err := parseOwnerRepo(entry.Repository)
	if err != nil {
		return nil, err
	}

	// Convert registry hints to asset hints
	assetHints := HintsForEntry(entry)

	// Get release
	if spec.Version != "" {
		report.msg(fmt.Sprintf("Fetching release %s for %s...", spec.Version, spec.Name))
	} else {
		report.msg(fmt.Sprintf("Fetching latest release for %s...", spec.Name))
	}
	release, err := i.fetchRelease(ctx, owner, repo, spec.Version, assetHints, opts, spec.Name, report)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	// Install the release
	result, err := i.installRelease(
		ctx,
		spec.Name,
		release,
		entry.Repository,
		opts,
		report,
		assetHints,
	)
	if err != nil {
		return nil, err
	}

	result.Repository = entry.Repository
	return result, nil
}

// fetchRelease resolves the release to install: an explicit version (prefixed
// when the plugin ships from a monorepo), the newest prefixed release, or the
// repository's latest release. When no version was requested and the latest
// release has no assets yet, the newest earlier stable release that has assets
// is used instead and a warning naming both versions goes to progress. An
// explicit version is returned as is.
func (i *Installer) fetchRelease(
	ctx context.Context,
	owner, repo, version string,
	hints *AssetNamingHints,
	opts InstallOptions,
	pluginName string,
	report progressReport,
) (*GitHubRelease, error) {
	prefix := tagPrefixOf(hints)
	if version != "" {
		return i.client.GetReleaseByTag(ctx, owner, repo, ReleaseTag(version, prefix))
	}

	var releases []GitHubRelease
	var latest *GitHubRelease
	var err error
	if prefix != "" {
		releases, err = i.client.ListStableReleases(ctx, owner, repo, prefixedReleaseScan)
		if err != nil {
			return nil, err
		}
		latest, err = selectLatestByPrefix(releases, prefix)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", owner, repo, err)
		}
	} else {
		latest, err = i.client.GetLatestRelease(ctx, owner, repo)
		if err != nil {
			return nil, err
		}
	}
	if len(latest.Assets) > 0 {
		return latest, nil
	}

	if releases == nil {
		releases, err = i.client.ListStableReleases(ctx, owner, repo, githubMaxPerPage)
		if err != nil {
			// installRelease reports the asset-less latest release clearly.
			return latest, nil //nolint:nilerr // the missing assets are the error the user needs
		}
	}
	earlier := selectEarlierWithAssets(releases, prefix, latest)
	if earlier == nil {
		return latest, nil
	}
	fallbackVersion := CanonicalVersion(earlier.TagName, prefix)
	if opts.CatalogOnly && opts.AcceptFallbackVersion != fallbackVersion {
		return nil, &NoAssetsError{
			Plugin:   pluginName,
			Version:  CanonicalVersion(latest.TagName, prefix),
			Platform: runtime.GOOS + "/" + runtime.GOARCH,
		}
	}
	report.msg(fmt.Sprintf(
		"Warning: latest release %s has no assets yet (the upload may still be in progress); installing %s instead",
		CanonicalVersion(latest.TagName, prefix), fallbackVersion,
	))
	return earlier, nil
}

// installFromURL installs a plugin directly from a GitHub URL.
func (i *Installer) installFromURL(
	ctx context.Context,
	spec *PluginSpecifier,
	opts InstallOptions,
	report progressReport,
) (*InstallResult, error) {
	if spec.Version != "" {
		report.msg(fmt.Sprintf("Fetching release %s from %s/%s...", spec.Version, spec.Owner, spec.Repo))
	} else {
		report.msg(fmt.Sprintf("Fetching latest release from %s/%s...", spec.Owner, spec.Repo))
	}
	hints := hintsForRepository(spec.Repo)
	release, err := i.fetchRelease(ctx, spec.Owner, spec.Repo, spec.Version, hints, opts, spec.Name, report)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	repository := fmt.Sprintf("%s/%s", spec.Owner, spec.Repo)
	result, err := i.installRelease(ctx, spec.Name, release, repository, opts, report, hints)
	if err != nil {
		return nil, err
	}

	result.FromURL = true
	result.Repository = fmt.Sprintf("%s/%s", spec.Owner, spec.Repo)
	return result, nil
}

// installRelease downloads and installs a specific release.
//
//nolint:gocognit // download, checksum, and extract stay in one install transaction
func (i *Installer) installRelease(
	ctx context.Context,
	name string,
	release *GitHubRelease,
	repository string,
	opts InstallOptions,
	report progressReport,
	hints *AssetNamingHints,
) (*InstallResult, error) {
	version := CanonicalVersion(release.TagName, tagPrefixOf(hints))

	// Determine plugin directory
	pluginDir := i.pluginDir
	if opts.PluginDir != "" {
		pluginDir = opts.PluginDir
	}

	// Check if already installed
	installDir := filepath.Join(pluginDir, name, version)
	if _, err := os.Stat(installDir); err == nil && !opts.Force {
		return nil, fmt.Errorf(
			"plugin %s@%s already installed. Use --force to reinstall",
			name,
			version,
		)
	}

	// Override region in asset hints if user supplied region via --metadata
	if opts.Metadata != nil {
		if region, ok := opts.Metadata["region"]; ok && region != "" {
			if hints == nil {
				hints = &AssetNamingHints{}
			}
			hints.Region = region
		}
	}

	// Find platform-specific asset (use hints if provided)
	asset, err := FindPlatformAssetWithHints(release, name, hints)
	if err != nil {
		return nil, err
	}

	report.msg(fmt.Sprintf("Downloading %s (%d bytes)...", asset.Name, asset.Size))

	// Download asset and verify checksum
	tmpPath, cleanupTmp, err := i.downloadAndVerify(ctx, release, asset, opts, report)
	if err != nil {
		return nil, err
	}
	defer cleanupTmp()

	// Create install directory
	if mkdirErr := os.MkdirAll(installDir, 0750); mkdirErr != nil {
		return nil, fmt.Errorf("failed to create directory: %w", mkdirErr)
	}

	report.msg("Extracting archive...")

	// Extract archive
	if extractErr := ExtractArchive(tmpPath, installDir); extractErr != nil {
		_ = os.RemoveAll(installDir)
		return nil, fmt.Errorf("failed to extract: %w", extractErr)
	}

	// Find and validate binary
	binaryPath := findPluginBinary(installDir, name)
	if binaryPath == "" {
		_ = os.RemoveAll(installDir)
		return nil, errors.New("no executable found in archive")
	}

	if validateErr := ValidateBinary(binaryPath); validateErr != nil {
		_ = os.RemoveAll(installDir)
		return nil, validateErr
	}

	// Write plugin.metadata.json if metadata was provided
	if len(opts.Metadata) > 0 {
		if metaErr := WritePluginMetadata(installDir, opts.Metadata); metaErr != nil {
			report.msg(fmt.Sprintf("Warning: failed to write plugin metadata: %v", metaErr))
		}
	}

	// Save to config unless --no-save
	if !opts.NoSave {
		plugin := config.InstalledPlugin{
			Name:    name,
			URL:     fmt.Sprintf("github.com/%s", repository),
			Version: version,
		}
		if addErr := config.AddInstalledPlugin(plugin); addErr != nil {
			// Non-fatal, just warn
			report.msg(fmt.Sprintf("Warning: failed to save to config: %v", addErr))
		}
	}

	report.msg(fmt.Sprintf("Successfully installed %s@%s", name, version))

	return &InstallResult{
		Name:    name,
		Version: version,
		Path:    installDir,
	}, nil
}

// downloadAndVerify downloads the release asset to a temp file, optionally verifies
// its checksum against the release's checksums.txt, and returns the temp file path
// along with a cleanup function that removes it. The caller must defer the cleanup.
func (i *Installer) downloadAndVerify(
	ctx context.Context,
	release *GitHubRelease,
	asset *ReleaseAsset,
	opts InstallOptions,
	report progressReport,
) (string, func(), error) {
	// Determine extension for temp file
	pattern := "finfocus-plugin-*"
	switch {
	case strings.HasSuffix(asset.Name, extZip):
		pattern += extZip
	case strings.HasSuffix(asset.Name, extTarGz):
		pattern += extTarGz
	case strings.HasSuffix(asset.Name, ".tgz"):
		pattern += ".tgz"
	default:
		pattern += filepath.Ext(asset.Name)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	cleanup := func() { _ = os.Remove(tmpPath) }

	// Download asset. The percent sentence goes to the string callback once.
	// The typed event carries the same sentence plus the byte counts.
	downloadProgress := func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		pct := float64(downloaded) / float64(total) * 100 //nolint:mnd // percentage calculation
		message := fmt.Sprintf("Downloading... %.0f%%", pct)
		if report.text != nil {
			report.text(message)
		}
		if report.event != nil {
			report.event(ProgressEvent{
				Phase:   PhaseDownload,
				Message: message,
				Bytes:   downloaded,
				Total:   total,
			})
		}
	}
	downloadErr := i.client.DownloadAsset(ctx, asset.BrowserDownloadURL, tmpPath, downloadProgress)
	if downloadErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("failed to download: %w", downloadErr)
	}

	// Checksum verification (after download, before extraction)
	if !opts.SkipChecksum {
		if checksumErr := i.verifyChecksumForRelease(ctx, release, asset, tmpPath, opts, report); checksumErr != nil {
			cleanup()
			return "", func() {}, checksumErr
		}
	}

	return tmpPath, cleanup, nil
}

// checksumWarn emits a progress warning when a callback is set.
func checksumWarn(report progressReport, msg string) {
	report.msg("Warning: " + msg)
}

// verifyChecksumForRelease performs checksum verification of a downloaded file against
// the checksums.txt asset in the release. Only a confirmed hash mismatch returns an
// error; all other issues (missing checksums.txt, download failures, parse errors,
// asset not listed) produce warnings and allow installation to proceed.
func (i *Installer) verifyChecksumForRelease(
	ctx context.Context,
	release *GitHubRelease,
	asset *ReleaseAsset,
	downloadedFilePath string,
	opts InstallOptions,
	report progressReport,
) error {
	expectedHash, err := i.fetchExpectedHash(ctx, release, asset.Name)
	if err != nil {
		// The dashboard accepts only an explicit missing-file confirmation.
		// A download, parse, or missing-entry failure still fails closed.
		// The CLI warns and continues unless the caller required the file.
		missingAccepted := errors.Is(err, ErrChecksumUnavailable) && opts.AcceptMissingChecksum
		if opts.RequireChecksumFile && !missingAccepted {
			return err
		}
		checksumWarn(report, err.Error()+", skipping verification")
		return nil
	}

	// This is the only fatal path: a confirmed hash mismatch
	if verifyErr := VerifyChecksum(ctx, downloadedFilePath, expectedHash); verifyErr != nil {
		return fmt.Errorf("integrity check failed: %w", verifyErr)
	}

	report.msg("Checksum verified (SHA256)")
	return nil
}

// fetchExpectedHash downloads the checksums.txt from the release and parses it to
// find the expected hash for the named asset. Returns the hash on success, or a
// factual error describing what went wrong. The caller decides the policy (e.g.
// whether to skip verification or abort).
func (i *Installer) fetchExpectedHash(
	ctx context.Context,
	release *GitHubRelease,
	assetName string,
) (string, error) {
	checksumAsset := FindChecksumAsset(release)
	if checksumAsset == nil {
		return "", ErrChecksumUnavailable
	}

	checksumData, err := i.downloadChecksumAsset(ctx, checksumAsset)
	if err != nil {
		return "", err
	}

	expectedHash, err := ParseChecksumsFile(checksumData, assetName)
	if err == nil {
		return expectedHash, nil
	}

	switch {
	case errors.Is(err, ErrAssetNotInChecksums):
		return "", fmt.Errorf("%s not found in checksums file", assetName)
	case errors.Is(err, ErrMalformedChecksums):
		return "", errors.New("checksums.txt is malformed")
	default:
		return "", fmt.Errorf("failed to parse checksums.txt: %w", err)
	}
}

// downloadChecksumAsset downloads the checksums asset to a temp file and returns
// its content. Returns a descriptive error on failure.
func (i *Installer) downloadChecksumAsset(
	ctx context.Context,
	checksumAsset *ReleaseAsset,
) ([]byte, error) {
	checksumTmp, err := os.CreateTemp("", "finfocus-checksums-*")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create temp file for checksums: %w",
			err,
		)
	}
	checksumTmpPath := checksumTmp.Name()
	_ = checksumTmp.Close()
	defer func() { _ = os.Remove(checksumTmpPath) }()

	downloadErr := i.client.DownloadAsset(
		ctx,
		checksumAsset.BrowserDownloadURL,
		checksumTmpPath,
		nil,
	)
	if downloadErr != nil {
		return nil, fmt.Errorf(
			"failed to download checksums.txt: %w",
			downloadErr,
		)
	}

	data, err := os.ReadFile(checksumTmpPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read checksums.txt: %w",
			err,
		)
	}

	return data, nil
}

// parseOwnerRepo parses a repository string in the "owner/repo" format and returns
// the owner and repository name. It returns an error if the input does not contain
// exactly one '/' separator or if either the owner or repo component is empty.
func parseOwnerRepo(repo string) (string, string, error) {
	parts := strings.SplitN(repo, "/", 2) //nolint:mnd // split into 2 parts
	if len(parts) != 2 {                  //nolint:mnd // expect 2 parts
		return "", "", fmt.Errorf("invalid repository format: %s", repo)
	}
	owner, repoName := parts[0], parts[1]
	if owner == "" || repoName == "" {
		return "", "", errors.New("invalid repository format: owner or repo empty")
	}
	return owner, repoName, nil
}

// findPluginBinary searches dir for an executable plugin binary matching name.
// It first checks common filename patterns (e.g. name, name.exe,
// finfocus-plugin-name, finfocus-plugin-name.exe). If none match, it
// scans the directory for any executable file (on Windows files must end with
// .exe; on other systems the file must have an executable bit). It returns the
// full path to the first matching file, or an empty string if no binary is
// found.
func findPluginBinary(dir, name string) string {
	// Check common patterns
	patterns := []string{
		name,
		name + ".exe",
		"finfocus-plugin-" + name,
		"finfocus-plugin-" + name + ".exe",
	}

	for _, pattern := range patterns {
		path := filepath.Join(dir, pattern)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	// Fall back to scanning directory
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			// Check executability based on OS
			var isExecutable bool
			if runtime.GOOS == osWindows {
				isExecutable = strings.HasSuffix(path, ".exe")
			} else {
				isExecutable = info.Mode()&executablePermissionMask != 0
			}
			if isExecutable {
				return path
			}
		}
	}

	return ""
}

// UpdateOptions configures plugin update behavior.
type UpdateOptions struct {
	DryRun       bool   // Show what would be updated without changes
	Version      string // Specific version to update to (empty = latest)
	PluginDir    string // Custom plugin directory
	SkipChecksum bool   // SkipChecksum bypasses SHA256 checksum verification during update
	// OnEvent receives a typed progress event beside the string callback.
	OnEvent func(ProgressEvent)
	// CatalogOnly, RequireChecksumFile, AcceptMissingChecksum, and
	// AcceptFallbackVersion match InstallOptions and apply to the install
	// performed by a non-dry-run update.
	CatalogOnly           bool
	RequireChecksumFile   bool
	AcceptMissingChecksum bool
	AcceptFallbackVersion string
}

// UpdateResult contains the result of a plugin update.
type UpdateResult struct {
	Name        string
	OldVersion  string
	NewVersion  string
	Path        string
	WasUpToDate bool
}

// Update updates an installed plugin to the latest or specified version.
//
//nolint:funlen // version comparison and reinstall stay together
func (i *Installer) Update(
	ctx context.Context,
	name string,
	opts UpdateOptions,
	progress func(msg string),
) (*UpdateResult, error) {
	report := newProgressReport(progress, opts.OnEvent)
	policy := InstallOptions{
		CatalogOnly:           opts.CatalogOnly,
		RequireChecksumFile:   opts.RequireChecksumFile,
		AcceptMissingChecksum: opts.AcceptMissingChecksum,
		AcceptFallbackVersion: opts.AcceptFallbackVersion,
	}
	if opts.CatalogOnly {
		opts.SkipChecksum = false
		policy.SkipChecksum = false
	}

	// Acquire lock for this plugin
	unlock, err := i.acquireLock(name)
	if err != nil {
		return nil, err
	}
	defer unlock()

	// Get installed plugin info
	installed, err := config.GetInstalledPlugin(name)
	if err != nil {
		return nil, fmt.Errorf("plugin %q is not installed", name)
	}

	// Look up in registry first, then try as URL
	owner, repo, assetHints, err := i.resolvePluginSource(name, installed.URL)
	if err != nil {
		return nil, err
	}

	// Get target version
	if opts.Version != "" {
		report.msg(fmt.Sprintf("Fetching release %s for %s...", opts.Version, name))
	} else {
		report.msg(fmt.Sprintf("Checking for updates to %s...", name))
	}
	release, err := i.fetchRelease(ctx, owner, repo, opts.Version, assetHints, policy, name, report)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	newVersion := CanonicalVersion(release.TagName, tagPrefixOf(assetHints))
	oldVersion := installed.Version

	// Compare versions
	cmp, err := CompareVersions(newVersion, oldVersion)
	if err != nil {
		// If version comparison fails, proceed anyway
		cmp = 1
	}

	if cmp == 0 {
		return &UpdateResult{
			Name:        name,
			OldVersion:  oldVersion,
			NewVersion:  newVersion,
			WasUpToDate: true,
		}, nil
	}

	if cmp < 0 && opts.Version == "" {
		// Latest is older than installed (shouldn't happen normally)
		return &UpdateResult{
			Name:        name,
			OldVersion:  oldVersion,
			NewVersion:  newVersion,
			WasUpToDate: true,
		}, nil
	}

	if opts.DryRun {
		report.msg(fmt.Sprintf("Would update %s from %s to %s", name, oldVersion, newVersion))
		return &UpdateResult{
			Name:       name,
			OldVersion: oldVersion,
			NewVersion: newVersion,
		}, nil
	}

	// Install new version
	pluginDir := i.pluginDir
	if opts.PluginDir != "" {
		pluginDir = opts.PluginDir
	}

	installOpts := InstallOptions{
		Force:                 true, // Allow overwriting
		NoSave:                true, // We'll update config ourselves
		PluginDir:             pluginDir,
		SkipChecksum:          opts.SkipChecksum,
		CatalogOnly:           opts.CatalogOnly,
		RequireChecksumFile:   opts.RequireChecksumFile,
		AcceptMissingChecksum: opts.AcceptMissingChecksum,
		AcceptFallbackVersion: opts.AcceptFallbackVersion,
	}

	repository := fmt.Sprintf("%s/%s", owner, repo)
	result, err := i.installRelease(ctx, name, release, repository, installOpts, report, assetHints)
	if err != nil {
		return nil, err
	}

	// Remove old version directory
	oldDir := filepath.Join(pluginDir, name, oldVersion)
	if oldVersion != newVersion {
		_ = os.RemoveAll(oldDir)
	}

	// Update config
	if updateErr := config.UpdateInstalledPluginVersion(name, newVersion); updateErr != nil {
		report.msg(fmt.Sprintf("Warning: failed to update config: %v", updateErr))
	}

	return &UpdateResult{
		Name:       name,
		OldVersion: oldVersion,
		NewVersion: newVersion,
		Path:       result.Path,
	}, nil
}

// resolvePluginSource resolves the GitHub owner, repo, and asset hints for a plugin.
// It first looks up the plugin in the embedded registry. If not found, it parses
// the installed URL to extract owner and repo. Returns an error if neither lookup
// succeeds.
func (i *Installer) resolvePluginSource(
	name, installedURL string,
) (string, string, *AssetNamingHints, error) {
	// Try registry first
	entry, err := GetPlugin(name)
	if err == nil {
		// Found in registry - parse owner/repo and get hints
		owner, repo, parseErr := parseOwnerRepo(entry.Repository)
		if parseErr != nil {
			return "", "", nil, parseErr
		}
		hints := HintsForEntry(entry)
		return owner, repo, hints, nil
	}

	// Not in registry - try parsing installed URL
	if installedURL == "" {
		return "", "", nil, fmt.Errorf("plugin %q not found in registry and no URL available", name)
	}

	// Remove github.com/ prefix if present
	urlPath := strings.TrimPrefix(installedURL, "github.com/")
	owner, repo, parseErr := parseOwnerRepo(urlPath)
	if parseErr != nil {
		return "", "", nil, fmt.Errorf("failed to parse plugin URL %q: %w", installedURL, parseErr)
	}

	return owner, repo, hintsForRepository(repo), nil
}

// RemoveOtherVersionsResult contains the result of removing other plugin versions.
type RemoveOtherVersionsResult struct {
	PluginName      string
	KeptVersion     string
	RemovedVersions []string
	BytesFreed      int64
}

// RemoveOtherVersions removes all versions of a plugin except the specified one.
// This is used for cleanup after upgrade/install operations.
// It scans the plugin directory for version subdirectories and removes any
// that don't match the keepVersion parameter.
func (i *Installer) RemoveOtherVersions(
	name string,
	keepVersion string,
	pluginDir string,
	progress func(msg string),
) (*RemoveOtherVersionsResult, error) {
	report := newProgressReport(progress, nil)

	// Acquire lock for concurrent safety during version removal
	unlock, err := i.acquireLock(name)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for %s: %w", name, err)
	}
	defer unlock()

	if pluginDir == "" {
		pluginDir = i.pluginDir
	}

	pluginPath := filepath.Join(pluginDir, name)

	// Check if plugin directory exists
	if _, statErr := os.Stat(pluginPath); os.IsNotExist(statErr) {
		return &RemoveOtherVersionsResult{
			PluginName:  name,
			KeptVersion: keepVersion,
		}, nil
	}

	// List all version directories
	entries, err := os.ReadDir(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	result := &RemoveOtherVersionsResult{
		PluginName:      name,
		KeptVersion:     keepVersion,
		RemovedVersions: []string{},
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip non-directories (e.g., lock files)
		}

		version := entry.Name()
		if version == keepVersion {
			continue // Keep the specified version
		}

		versionPath := filepath.Join(pluginPath, version)

		// Calculate directory size before removal
		size, sizeErr := getDirSize(versionPath)
		if sizeErr != nil {
			report.msg(fmt.Sprintf("Debug: failed to calculate size for %s: %v", versionPath, sizeErr))
		}

		report.msg(fmt.Sprintf("Removing old version %s...", version))

		if removeErr := os.RemoveAll(versionPath); removeErr != nil {
			// Log warning but continue with other versions
			report.msg(fmt.Sprintf("Warning: failed to remove %s: %v", version, removeErr))
			continue
		}

		result.RemovedVersions = append(result.RemovedVersions, version)
		result.BytesFreed += size
	}

	return result, nil
}

// getDirSize calculates the total size of all files in a directory recursively.
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// LockAvailable reports whether name can be modified right now.
// A held lock returns an error that matches ErrPluginBusy.
func (i *Installer) LockAvailable(name string) error {
	unlock, err := i.acquireLock(name)
	if err != nil {
		return err
	}
	unlock()
	return nil
}

// RemoveOptions configures plugin removal behavior.
type RemoveOptions struct {
	KeepConfig bool   // Don't remove from config file
	PluginDir  string // Custom plugin directory
	// Version removes that installed version. Empty removes the configured version.
	Version string
	// OnEvent receives a typed progress event beside the string callback.
	OnEvent func(ProgressEvent)
}

// Remove removes an installed plugin.
func (i *Installer) Remove(name string, opts RemoveOptions, progress func(msg string)) error {
	report := newProgressReport(progress, opts.OnEvent)

	// Acquire lock for this plugin
	unlock, err := i.acquireLock(name)
	if err != nil {
		return err
	}
	defer unlock()

	// Resolve pluginDir before config check (needed for filesystem fallback)
	pluginDir := i.pluginDir
	if opts.PluginDir != "" {
		pluginDir = opts.PluginDir
	}

	// Try config first, fall back to filesystem discovery
	inConfig := true
	installed, err := config.GetInstalledPlugin(name)
	if err != nil {
		discovered, discoverErr := i.discoverPluginFromFilesystem(name, pluginDir)
		if discoverErr != nil && opts.Version == "" {
			return fmt.Errorf("plugin %q is not installed", name)
		}
		installed = discovered
		inConfig = false
	}
	version := ""
	if installed != nil {
		version = installed.Version
	}
	if opts.Version != "" {
		version = opts.Version
	}
	if version == "" {
		return fmt.Errorf("plugin %q is not installed", name)
	}
	pluginPath := filepath.Join(pluginDir, name, version)
	if _, statErr := os.Stat(pluginPath); statErr != nil {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	// Remove plugin directory
	report.msg(fmt.Sprintf("Removing %s@%s...", name, version))

	if removeErr := os.RemoveAll(pluginPath); removeErr != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", removeErr)
	}

	// Remove parent directory if empty
	parentDir := filepath.Join(pluginDir, name)
	entries, err := os.ReadDir(parentDir)
	if err == nil && len(entries) == 0 {
		if rmErr := os.Remove(parentDir); rmErr != nil {
			report.msg(fmt.Sprintf("Warning: failed to remove parent directory %s: %v", parentDir, rmErr))
		}
	}

	// Remove from config unless --keep-config, the plugin was not in config,
	// or a different installed version was removed.
	configured := installed != nil && installed.Version == version
	if !opts.KeepConfig && inConfig && configured {
		if removeErr := config.RemoveInstalledPlugin(name); removeErr != nil {
			report.msg(fmt.Sprintf("Warning: failed to remove from config: %v", removeErr))
		}
	}

	report.msg(fmt.Sprintf("Successfully removed %s", name))

	return nil
}

// discoverPluginFromFilesystem scans the plugin directory for a plugin not tracked in config.
func (i *Installer) discoverPluginFromFilesystem(
	name, pluginDir string,
) (*config.InstalledPlugin, error) {
	pluginPath := filepath.Join(pluginDir, name)
	versions, err := os.ReadDir(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("plugin directory not found: %w", err)
	}
	for _, v := range versions {
		if !v.IsDir() {
			continue
		}
		return &config.InstalledPlugin{
			Name:    name,
			Version: v.Name(),
		}, nil
	}
	return nil, fmt.Errorf("no version found for plugin %q", name)
}
