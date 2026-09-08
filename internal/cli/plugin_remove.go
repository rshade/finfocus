package cli

import (
	"context"
	"fmt"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/registry"
)

// NewPluginRemoveCmd returns a Cobra command configured to remove an installed plugin.
// The command accepts a single plugin name argument and provides the flags
// `--keep-config` to retain the plugin entry in the configuration and
// `--plugin-dir` to specify a custom plugin directory. On execution it removes
// the plugin files and, unless `--keep-config` is set, removes the plugin entry
// from the configuration. The command's execution returns an error if removal fails.
func NewPluginRemoveCmd() *cobra.Command {
	var (
		keepConfig bool
		pluginDir  string
	)

	cmd := &cobra.Command{
		Use:     "remove <plugin>",
		Aliases: []string{"uninstall", "rm"},
		Short:   "Remove an installed plugin",
		Long: `Remove an installed plugin from the system.

This will delete the plugin files and remove it from the configuration.`,
		Example: `  # Remove a plugin
  finfocus plugin remove kubecost

  # Remove but keep in config (for reinstalling later)
  finfocus plugin remove kubecost --keep-config

  # Using alias
  finfocus plugin uninstall kubecost`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			// Create installer
			installer := registry.NewInstaller(pluginDir)

			opts := registry.RemoveOptions{
				KeepConfig: keepConfig,
				PluginDir:  pluginDir,
			}

			// Progress callback
			progress := func(msg string) {
				cmd.Printf("%s\n", msg)
			}

			// Rehearse: report which files would be deleted
			rehearse := func(_ context.Context) error {
				cmd.Printf("Would remove plugin %q\n", name)
				if !keepConfig {
					cmd.Printf("  (plugin entry will also be removed from config)\n")
				}
				return nil
			}

			// Commit: actually remove the plugin
			commit := func(_ context.Context) error {
				if err := installer.Remove(name, opts, progress); err != nil {
					return fmt.Errorf("removing plugin %q: %w", name, err)
				}

				cmd.Printf("\n✓ Plugin %s removed successfully\n", name)

				return nil
			}

			return ax.Perform(ctx, rehearse, commit)
		},
	}

	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep plugin entry in config file")
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "Custom plugin directory")

	return cmd
}
