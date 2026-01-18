package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/registry"
)

// formatBytes converts a size in bytes to a human-readable string using GB, MB, KB,
// or bytes. bytes is interpreted as a count of bytes; values at or above KB, MB,
// or GB are formatted with two decimal places (e.g., "1.23 MB"), otherwise the
// integer byte count is returned as "<n> bytes".
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

// displaySecurityWarning prints security warnings related to a plugin install request.
// It warns when the spec refers to a URL-based plugin that URL-based plugins are not
// verified by the FinFocus team and should only be installed from trusted sources.
// For registry plugins, it warns if the plugin's SecurityLevel is "experimental" to
// indicate the plugin has not been fully reviewed.
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

// handleCleanup removes other installed versions of the plugin described by
// result and prints progress and a summary to cmd.
// 
// It uses installer to remove versions from pluginDir, invoking progress with
// progress messages. On error it prints a warning to cmd. If versions were
// removed, it prints each removed version and, when available, the amount of
// space freed formatted by formatBytes.
//
// Parameters:
//  - cmd: Cobra command used for printing output.
//  - installer: Installer used to perform removal of other versions.
//  - result: Installation result containing the installed plugin name and version.
//  - pluginDir: Directory where plugins are stored.
//  - progress: Callback invoked with progress messages.
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

// NewPluginInstallCmd creates the install command for installing plugins from registry or URL.
//
//	--plugin-dir    Custom plugin directory (default: ~/.finfocus/plugins)
// NewPluginInstallCmd creates the "install" Cobra command used to install plugins from the registry or a GitHub URL.
// 
// The command accepts a single plugin specifier (registry name, registry with version, GitHub URL, or URL with version),
// and exposes flags to force reinstall, skip saving to config, clean up other installed versions, and override the plugin directory.
// On successful installation the command prints the installed plugin's name, version, and path; if the --clean flag is provided it
// additionally removes other versions of the installed plugin to free disk space.
// 
// It returns the configured *cobra.Command.
func NewPluginInstallCmd() *cobra.Command {
	var (
		force     bool
		noSave    bool
		pluginDir string
		clean     bool
	)

	cmd := &cobra.Command{
		Use:   "install <plugin>",
		Short: "Install a plugin from registry or URL",
		Long: `Install a plugin from the registry or directly from a GitHub URL.

Plugins can be specified in several formats:
  - Registry name: kubecost
  - Registry name with version: kubecost@v1.0.0
  - GitHub URL: github.com/owner/repo
  - GitHub URL with version: github.com/owner/repo@v1.0.0`,
		Example: `  # Install latest version from registry
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
  finfocus plugin install kubecost --clean`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specifier := args[0]

			// Show security warning for URL installs
			spec, err := registry.ParsePluginSpecifier(specifier)
			if err != nil {
				return fmt.Errorf("parsing plugin specifier %q: %w", specifier, err)
			}

			displaySecurityWarning(cmd, spec)

			// Create installer and install
			installer := registry.NewInstaller(pluginDir)
			opts := registry.InstallOptions{
				Force:     force,
				NoSave:    noSave,
				PluginDir: pluginDir,
			}

			progress := func(msg string) {
				cmd.Printf("%s\n", msg)
			}

			result, err := installer.Install(specifier, opts, progress)
			if err != nil {
				return fmt.Errorf("installing plugin %q: %w", specifier, err)
			}

			cmd.Printf("\n✓ Plugin installed successfully\n")
			cmd.Printf("  Name:    %s\n", result.Name)
			cmd.Printf("  Version: %s\n", result.Version)
			cmd.Printf("  Path:    %s\n", result.Path)

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

	return cmd
}