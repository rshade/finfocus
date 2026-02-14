package cli

import (
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/analyzer"
)

// NewAnalyzerUninstallCmd creates the analyzer uninstall command.
//
// This command removes all installed analyzer-finfocus-v* directories from
// the Pulumi plugin directory.
func NewAnalyzerUninstallCmd() *cobra.Command {
	var targetDir string

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the Pulumi analyzer plugin",
		Long: `Removes the finfocus Pulumi Analyzer plugin from the plugin directory.

All analyzer-finfocus-v* directories are removed, cleaning up any installed
versions of the analyzer.`,
		Example: `  # Uninstall the analyzer
  finfocus analyzer uninstall

  # Uninstall from a custom directory
  finfocus analyzer uninstall --target-dir /opt/pulumi/plugins`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Check if installed before uninstalling (for user messaging)
			pluginDir, err := analyzer.ResolvePulumiPluginDir(targetDir)
			if err != nil {
				return err
			}

			installed, err := analyzer.IsInstalled(pluginDir)
			if err != nil {
				return err
			}

			if !installed {
				cmd.Printf("Analyzer is not installed\n")
				return nil
			}

			// Get version before removing for output
			ver, _ := analyzer.InstalledVersion(pluginDir)

			if uninstallErr := analyzer.Uninstall(ctx, targetDir); uninstallErr != nil {
				return uninstallErr
			}

			cmd.Printf("Analyzer uninstalled successfully\n")
			if ver != "" {
				cmd.Printf("  Removed: v%s\n", ver)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&targetDir, "target-dir", "", "Override Pulumi plugin directory")

	return cmd
}
