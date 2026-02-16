package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/registry"
)

// NewPluginUpdateCmd returns a Cobra command that updates an installed plugin to the latest or a specified version.
// The command requires a single positional argument (plugin name) and provides the following flags:
//
//	--dry-run   Show what would be updated without making changes.
//	--version   Specific version to update to (default: latest).
//	--plugin-dir Custom plugin directory.
//
// NewPluginUpdateCmd returns a cobra command that updates an installed plugin to the latest
// version or to a specified version. The command accepts a single plugin name argument and
// supports the flags --dry-run (show changes without applying), --version (target version),
// and --plugin-dir (custom plugin directory). It prints progress messages and a summary of the
// result (including name, old/new versions, and path).
//
// The command's execution returns an error if the underlying installer fails to perform the
// NewPluginUpdateCmd returns a *cobra.Command configured to update an installed plugin.
// The command expects a single positional argument (plugin name) and updates that plugin
// to the latest version or to a specific version supplied via the --version flag.
// It supports --dry-run to preview changes and --plugin-dir to override the plugin directory.
// The command prints progress and summary information and returns an error if the update operation fails.
func NewPluginUpdateCmd() *cobra.Command {
	var (
		dryRun    bool
		version   string
		pluginDir string
	)

	cmd := &cobra.Command{
		Use:   "update <plugin>",
		Short: "Update an installed plugin to the latest version",
		Long: `Update an installed plugin to the latest version or a specific version.

The plugin must already be installed. Use 'plugin install' to install new plugins.`,
		Example: `  # Update to latest version
  finfocus plugin update kubecost

  # Update to specific version
  finfocus plugin update kubecost --version v2.0.0

  # Check what would be updated without making changes
  finfocus plugin update kubecost --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Create installer
			installer := registry.NewInstaller(pluginDir)

			opts := registry.UpdateOptions{
				DryRun:    dryRun,
				Version:   version,
				PluginDir: pluginDir,
			}

			// Progress callback
			progress := func(msg string) {
				cmd.Printf("%s\n", msg)
			}

			// Update
			result, err := installer.Update(cmd.Context(), name, opts, progress)
			if err != nil {
				return fmt.Errorf("updating plugin %q: %w", name, err)
			}

			if result.WasUpToDate {
				cmd.Printf(
					"\n✓ Plugin %s is already up to date (%s)\n",
					result.Name,
					result.OldVersion,
				)
				return nil
			}

			if dryRun {
				cmd.Printf(
					"\n→ Would update %s from %s to %s\n",
					result.Name,
					result.OldVersion,
					result.NewVersion,
				)
				return nil
			}

			cmd.Printf("\n✓ Plugin updated successfully\n")
			cmd.Printf("  Name:        %s\n", result.Name)
			cmd.Printf("  Old version: %s\n", result.OldVersion)
			cmd.Printf("  New version: %s\n", result.NewVersion)
			cmd.Printf("  Path:        %s\n", result.Path)

			return nil
		},
	}

	cmd.Flags().
		BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")
	cmd.Flags().
		StringVar(&version, "version", "", "Specific version to update to (default: latest)")
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "Custom plugin directory")

	return cmd
}
