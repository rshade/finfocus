package cli

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/tui"
)

const (
	// repoPartCount is the expected number of parts when splitting owner/repo.
	repoPartCount = 2

	// pluginInstallLong is the long description for the install command.
	pluginInstallLong = `Install a plugin from the registry or directly from a GitHub URL.

Plugins can be specified in several formats:
  - Registry name: kubecost
  - Registry name with version: kubecost@v1.0.0
  - GitHub URL: github.com/owner/repo
  - GitHub URL with version: github.com/owner/repo@v1.0.0

Fallback Behavior:
  When a requested version exists but lacks compatible assets for your platform,
  the command can fall back to the latest stable version with compatible assets.

  - Interactive mode: You will be prompted to accept the fallback (default: No)
  - Non-interactive mode: Use --fallback-to-latest for automatic fallback
  - Strict mode: Use --no-fallback to disable fallback entirely`

	// pluginInstallExample contains usage examples for the install command.
	pluginInstallExample = `  # Install latest version from registry
  finfocus plugin install kubecost

  # Install specific version from registry
  finfocus plugin install kubecost@v1.0.0

  # Install from GitHub URL
  finfocus plugin install github.com/rshade/finfocus-plugin-aws-public

  # Install specific version from URL
  finfocus plugin install github.com/rshade/finfocus-plugin-aws-public@v0.1.0

  # Force reinstall even if already installed
  finfocus plugin install kubecost --force

  # Install without saving to config
  finfocus plugin install kubecost --no-save

  # Install and remove all other versions (cleanup disk space)
  finfocus plugin install kubecost --clean

  # Auto-fallback to latest stable if requested version lacks assets (CI mode)
  finfocus plugin install kubecost@v1.0.0 --fallback-to-latest

  # Fail immediately if requested version lacks assets (strict mode)
  finfocus plugin install kubecost@v1.0.0 --no-fallback

  # Install with region metadata (selects region-specific binary)
  finfocus plugin install aws-public --metadata="region=us-west-2"`
)

// formatBytes formats a byte count into a human-readable string (KB, MB, GB).
func formatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// displaySecurityWarning shows security warnings for plugin installations.
func displaySecurityWarning(cmd *cobra.Command, spec *registry.PluginSpecifier) {
	if spec.IsURL {
		cmd.Printf("⚠️  Installing from URL: %s/%s\n", spec.Owner, spec.Repo)
		cmd.Printf("   URL-based plugins are not verified by the FinFocus team.\n")
		cmd.Printf("   Only install from sources you trust.\n\n")
		return
	}

	// Check security level for registry plugins
	entry, err := registry.GetPlugin(spec.Name)
	if err == nil && entry.SecurityLevel == "experimental" {
		cmd.Printf("⚠️  Plugin %q has security level: experimental\n", spec.Name)
		cmd.Printf("   This plugin is not yet fully reviewed.\n\n")
	}
}

// handleCleanup removes other versions if --clean flag is set.
func handleCleanup(
	cmd *cobra.Command,
	installer *registry.Installer,
	result *registry.InstallResult,
	pluginDir string,
	progress func(string),
) {
	cleanupResult, cleanErr := installer.RemoveOtherVersions(
		result.Name,
		result.Version,
		pluginDir,
		progress,
	)
	if cleanErr != nil {
		cmd.Printf("\nWarning: cleanup failed: %v\n", cleanErr)
		return
	}

	if len(cleanupResult.RemovedVersions) > 0 {
		cmd.Printf("\n✓ Cleaned up %d old version(s)\n", len(cleanupResult.RemovedVersions))
		for _, v := range cleanupResult.RemovedVersions {
			cmd.Printf("  Removed: %s\n", v)
		}
		if cleanupResult.BytesFreed > 0 {
			cmd.Printf("  Freed: %s\n", formatBytes(cleanupResult.BytesFreed))
		}
	}
}

// displayInstallResult shows the installation result with fallback information if applicable.
func displayInstallResult(cmd *cobra.Command, result *registry.InstallResult) {
	cmd.Printf("\n✓ Plugin installed successfully\n")
	cmd.Printf("  Name:    %s\n", result.Name)
	if result.WasFallback && result.RequestedVersion != "" {
		cmd.Printf("  Version: %s (requested: %s)\n", result.Version, result.RequestedVersion)
	} else {
		cmd.Printf("  Version: %s\n", result.Version)
	}
	cmd.Printf("  Path:    %s\n", result.Path)
}

// isNoAssetError checks if an error indicates missing platform assets.
func isNoAssetError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no asset found for") ||
		strings.Contains(errStr, "no compatible asset found")
}

// getPlatformString returns the current platform in os/arch format.
func getPlatformString() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

// handleInstallError processes installation errors and attempts fallback if appropriate.
// Returns nil if fallback succeeds, or an appropriate error otherwise.
func handleInstallError(
	cmd *cobra.Command,
	installer *registry.Installer,
	spec *registry.PluginSpecifier,
	opts registry.InstallOptions,
	progress func(string),
	specifier string,
	installErr error,
	noFallback bool,
	fallbackToLatest bool,
	clean bool,
	pluginDir string,
) error {
	// Check if we can attempt fallback
	if !isNoAssetError(installErr) || spec.Version == "" || noFallback {
		return fmt.Errorf("installing plugin %q: %w", specifier, installErr)
	}

	// Try to find a fallback version
	fallbackResult, fallbackErr := handleFallback(cmd, installer, spec, opts, progress, fallbackToLatest)
	if fallbackErr != nil {
		if errors.Is(fallbackErr, errFallbackDeclined) {
			cmd.Printf("Installation aborted.\n")
			return errors.New("installation aborted")
		}
		return fmt.Errorf("installing plugin %q: %w", specifier, installErr)
	}

	// Fallback succeeded
	displayInstallResult(cmd, fallbackResult)
	if clean {
		handleCleanup(cmd, installer, fallbackResult, pluginDir, progress)
	}
	return nil
}

// NewPluginInstallCmd creates the install command for installing plugins from registry or URL.
//
//	--plugin-dir         Custom plugin directory (default: ~/.finfocus/plugins)
//	--clean              Remove other versions after successful install
//	--fallback-to-latest Automatically install latest stable version if requested version lacks assets
//
// NewPluginInstallCmd creates the "install" CLI command that installs a plugin from either the registry or a direct URL.
//
// The returned *cobra.Command parses a single plugin specifier argument and supports options to control installation
// behavior, including reinstallation, saving to config, cleanup of other versions, custom plugin directory, and
// controlling fallback behavior when platform-specific assets are missing. It also accepts repeatable metadata
// key=value pairs that are passed through to the installer.
//
// Notable flags:
//   - --force, -f: reinstall even if the requested version already exists.
//   - --no-save: do not add the plugin to the user's config file.
//   - --clean: remove other installed versions after a successful install.
//   - --plugin-dir: specify a custom plugin installation directory.
//   - --fallback-to-latest: automatically attempt a compatible latest stable release if the requested version lacks assets.
//   - --no-fallback: disable fallback behavior entirely (mutually exclusive with --fallback-to-latest).
//   - --metadata: repeatable key=value pairs attached to the plugin install.
//
// The command prints progress, shows a security warning for URL-based installs, and displays a concise install result.
// It returns the configured *cobra.Command.
func NewPluginInstallCmd() *cobra.Command {
	var (
		force            bool
		noSave           bool
		pluginDir        string
		clean            bool
		fallbackToLatest bool
		noFallback       bool
		metadata         []string
	)

	cmd := &cobra.Command{
		Use:     "install <plugin>",
		Short:   "Install a plugin from registry or URL",
		Long:    pluginInstallLong,
		Example: pluginInstallExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specifier := args[0]

			// Show security warning for URL installs
			spec, err := registry.ParsePluginSpecifier(specifier)
			if err != nil {
				return fmt.Errorf("parsing plugin specifier %q: %w", specifier, err)
			}

			displaySecurityWarning(cmd, spec)

			// Parse --metadata flags into map
			metadataMap, metadataWarnings := parseMetadataFlags(metadata)
			for _, w := range metadataWarnings {
				cmd.Printf("Warning: %s\n", w)
			}

			// Create installer and install
			installer := registry.NewInstaller(pluginDir)
			opts := registry.InstallOptions{
				Force:            force,
				NoSave:           noSave,
				PluginDir:        pluginDir,
				FallbackToLatest: fallbackToLatest,
				NoFallback:       noFallback,
				Metadata:         metadataMap,
			}

			progress := func(msg string) {
				cmd.Printf("%s\n", msg)
			}

			// Try the initial installation
			result, err := installer.Install(specifier, opts, progress)
			if err != nil {
				return handleInstallError(
					cmd, installer, spec, opts, progress,
					specifier, err, noFallback, fallbackToLatest, clean, pluginDir,
				)
			}

			displayInstallResult(cmd, result)

			if clean {
				handleCleanup(cmd, installer, result, pluginDir, progress)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Reinstall even if version already exists")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Don't add plugin to config file")
	cmd.Flags().BoolVar(&clean, "clean", false, "Remove other versions after successful install")
	cmd.Flags().
		StringVar(&pluginDir, "plugin-dir", "", "Custom plugin directory (default: ~/.finfocus/plugins)")
	cmd.Flags().BoolVar(
		&fallbackToLatest,
		"fallback-to-latest",
		false,
		"Automatically install latest stable version if requested version lacks assets",
	)
	cmd.Flags().BoolVar(
		&noFallback,
		"no-fallback",
		false,
		"Disable fallback behavior entirely (fail if requested version lacks assets)",
	)

	cmd.Flags().StringArrayVar(
		&metadata,
		"metadata",
		nil,
		"Plugin metadata as key=value pairs (e.g., --metadata=\"region=us-west-2\"); repeatable",
	)

	// Mark flags as mutually exclusive
	cmd.MarkFlagsMutuallyExclusive("fallback-to-latest", "no-fallback")

	return cmd
}

// errFallbackDeclined is returned when the user declines fallback in interactive mode.
var errFallbackDeclined = errors.New("fallback declined")

// handleFallback attempts to find a compatible release and install a fallback version
// when the requested plugin version has no assets for the current platform.
//
// It looks up repository and asset hints from the registry (or uses spec.Owner/spec.Repo
// for URL specs), queries GitHub for a release that provides compatible assets, and then
// either automatically proceeds (when autoFallback is true), prompts the user (when running
// in a TTY), or returns an error (when not in a TTY and autoFallback is false).
//
// On success the returned InstallResult is marked as a fallback (WasFallback = true) and
// RequestedVersion is set to the originally requested version.
//
// Parameters:
//   - cmd: the Cobra command used for printing prompts and messages.
//   - installer: installer used to perform the fallback installation.
//   - spec: original plugin specifier (name or URL and requested version).
//   - opts: original install options; select fields (Force, NoSave, PluginDir, Metadata)
//     are propagated to the fallback install. Fallback-specific options (FallbackToLatest,
//     NoFallback) are set to prevent recursion.
//   - progress: progress callback used during installation.
//   - autoFallback: when true, proceed without prompting when a compatible fallback is found.
//
// Returns:
//   - *registry.InstallResult containing details of the installed fallback release when successful.
//   - error if repository lookup fails, the repository format is invalid, no compatible fallback
//     is found, the user declines the fallback, or the fallback installation fails.
func handleFallback(
	cmd *cobra.Command,
	installer *registry.Installer,
	spec *registry.PluginSpecifier,
	opts registry.InstallOptions,
	progress func(string),
	autoFallback bool,
) (*registry.InstallResult, error) {
	// Get GitHub client to find fallback version
	client := registry.NewGitHubClient()

	var owner, repo string
	var assetHints *registry.AssetNamingHints

	if spec.IsURL {
		owner = spec.Owner
		repo = spec.Repo
	} else {
		// Look up plugin in registry to get repository info
		entry, err := registry.GetPlugin(spec.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to look up plugin: %w", err)
		}
		parts := strings.SplitN(entry.Repository, "/", repoPartCount)
		if len(parts) != repoPartCount {
			return nil, fmt.Errorf("invalid repository format: %s", entry.Repository)
		}
		owner, repo = parts[0], parts[1]

		// Get asset hints from registry entry
		if entry.AssetHints != nil {
			assetHints = &registry.AssetNamingHints{
				AssetPrefix:   entry.AssetHints.AssetPrefix,
				Region:        entry.AssetHints.DefaultRegion,
				VersionPrefix: entry.AssetHints.VersionPrefix,
			}
		}
	}

	// Find a release with compatible assets using fallback search
	info, err := client.FindReleaseWithFallbackInfo(owner, repo, spec.Version, spec.Name, assetHints)
	if err != nil {
		return nil, fmt.Errorf("no compatible version found: %w", err)
	}

	// If no fallback was needed (shouldn't happen if we got here), just return
	if !info.WasFallback {
		return nil, errors.New("unexpected: no fallback needed")
	}

	fallbackVersion := info.Release.TagName
	platform := getPlatformString()

	// Handle based on mode: auto-fallback or interactive
	switch {
	case autoFallback:
		// Automatic fallback mode (--fallback-to-latest)
		cmd.Printf("\nWarning: No compatible assets found for %s@%s (%s).\n",
			spec.Name, spec.Version, platform)
		cmd.Printf("Installing %s@%s (fallback from %s)...\n",
			spec.Name, fallbackVersion, spec.Version)

	case tui.IsTTY():
		// Interactive mode - prompt user
		promptResult := ConfirmFallback(
			cmd.OutOrStdout(),
			os.Stdin,
			spec.Name,
			spec.Version,
			fallbackVersion,
			platform,
		)

		if !promptResult.Accepted {
			return nil, errFallbackDeclined
		}

		cmd.Printf("\nInstalling %s@%s (fallback from %s)...\n",
			spec.Name, fallbackVersion, spec.Version)

	default:
		// Non-TTY without --fallback-to-latest - fail per existing behavior
		return nil, fmt.Errorf("no compatible assets found for %s@%s (%s) and fallback not enabled",
			spec.Name, spec.Version, platform)
	}

	// Install the fallback version
	fallbackSpecifier := fmt.Sprintf("%s@%s", spec.Name, fallbackVersion)
	if spec.IsURL {
		fallbackSpecifier = fmt.Sprintf("github.com/%s/%s@%s", owner, repo, fallbackVersion)
	}

	fallbackOpts := registry.InstallOptions{
		Force:            opts.Force,
		NoSave:           opts.NoSave,
		PluginDir:        opts.PluginDir,
		FallbackToLatest: false, // Don't recurse
		NoFallback:       true,  // Don't recurse
		Metadata:         opts.Metadata,
	}

	result, err := installer.Install(fallbackSpecifier, fallbackOpts, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to install fallback version: %w", err)
	}

	// Mark result as fallback
	result.WasFallback = true
	result.RequestedVersion = spec.Version

	return result, nil
}

// parseMetadataFlags converts --metadata flag values ("key=value") into a map.
// Entries that do not contain an '=' are collected as warnings. Leading and
// trailing whitespace is trimmed from both keys and values. If the input
// contains no valid pairs, the function returns nil and any accumulated
// warnings. When a key appears multiple times, the last occurrence wins.
func parseMetadataFlags(flags []string) (map[string]string, []string) {
	if len(flags) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(flags))
	var warnings []string
	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", 2) //nolint:mnd // split into key=value
		if len(parts) != 2 {                  //nolint:mnd // expect key and value
			warnings = append(warnings, fmt.Sprintf("ignored metadata entry %q: missing '='", flag))
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			warnings = append(warnings, fmt.Sprintf("ignored metadata entry %q: empty key", flag))
			continue
		}
		m[key] = strings.TrimSpace(parts[1])
	}
	if len(m) == 0 {
		return nil, warnings
	}
	return m, warnings
}
