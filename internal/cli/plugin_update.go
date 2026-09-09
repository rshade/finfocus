package cli

import (
	"context"
	"fmt"

	"github.com/rshade/ax-go"
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
		version      string
		pluginDir    string
		skipChecksum bool
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

  # Preview update without making changes
  finfocus plugin update kubecost --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginUpdate(cmd, args[0], version, pluginDir, skipChecksum)
		},
	}

	cmd.Flags().
		StringVar(&version, "version", "", "Specific version to update to (default: latest)")
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "Custom plugin directory")
	cmd.Flags().BoolVar(
		&skipChecksum,
		"skip-checksum",
		false,
		"Skip SHA256 checksum verification during update",
	)

	return cmd
}

// runPluginUpdate performs the "plugin update" command: it resolves the
// target version and updates the plugin, honoring --dry-run via ax.Perform
// (rehearse reports what would change; commit performs the real update).
func runPluginUpdate(cmd *cobra.Command, name, version, pluginDir string, skipChecksum bool) error {
	ctx := cmd.Context()
	dryRun := ax.DryRunFromContext(ctx)

	installer := registry.NewInstaller(pluginDir)

	opts := registry.UpdateOptions{
		DryRun:       dryRun,
		Version:      version,
		PluginDir:    pluginDir,
		SkipChecksum: skipChecksum,
	}

	progress := func(msg string) {
		cmd.Printf("%s\n", msg)
	}

	// Rehearse: resolve the version and check compatibility without updating
	rehearse := func(ctx2 context.Context) error {
		result, err := installer.Update(ctx2, name, opts, progress)
		if err != nil {
			return fmt.Errorf("updating plugin %q: %w", name, err)
		}

		if result.WasUpToDate {
			cmd.Printf("\n✓ Plugin %s is already up to date (%s)\n", result.Name, result.OldVersion)
			return nil
		}

		cmd.Printf("\n→ Would update %s from %s to %s\n", result.Name, result.OldVersion, result.NewVersion)
		return nil
	}

	// Commit: actually update the plugin
	commit := func(ctx2 context.Context) error {
		result, err := installer.Update(ctx2, name, opts, progress)
		if err != nil {
			return fmt.Errorf("updating plugin %q: %w", name, err)
		}

		if result.WasUpToDate {
			cmd.Printf("\n✓ Plugin %s is already up to date (%s)\n", result.Name, result.OldVersion)
			return nil
		}

		cmd.Printf("\n✓ Plugin updated successfully\n")
		cmd.Printf("  Name:        %s\n", result.Name)
		cmd.Printf("  Old version: %s\n", result.OldVersion)
		cmd.Printf("  New version: %s\n", result.NewVersion)
		cmd.Printf("  Path:        %s\n", result.Path)

		return nil
	}

	return ax.Perform(ctx, rehearse, commit)
}
