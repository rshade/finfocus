package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/registry"
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

// NewPluginInstallCmd creates the install command for installing plugins from registry or URL.
//
//	--plugin-dir    Custom plugin directory (default: ~/.finfocus/plugins)
//	--clean         Remove other versions after successful install
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
