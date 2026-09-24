package cli

import (
	"context"
	"fmt"

	"github.com/rshade/ax-go"
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
				return fmt.Errorf("resolve pulumi plugin dir: %w", err)
			}

			installed, err := analyzer.IsInstalled(pluginDir)
			if err != nil {
				return fmt.Errorf("check analyzer installed: %w", err)
			}

			if !installed {
				if machineOutputRequested(cmd) {
					return writeJSON(cmd, actionResult{Action: "not_installed", DryRun: ax.DryRunFromContext(ctx)})
				}
				cmd.Printf("Analyzer is not installed\n")
				return nil
			}

			// Get version before removing for output
			ver, _ := analyzer.InstalledVersion(pluginDir)

			// Rehearse: report what would be removed
			rehearse := func(_ context.Context) error {
				return reportAnalyzerUninstall(cmd, pluginDir, ver, true)
			}

			// Commit: actually uninstall
			commit := func(ctx2 context.Context) error {
				if uninstallErr := analyzer.Uninstall(ctx2, pluginDir); uninstallErr != nil {
					return fmt.Errorf("uninstall analyzer: %w", uninstallErr)
				}

				return reportAnalyzerUninstall(cmd, pluginDir, ver, false)
			}

			return ax.Perform(ctx, rehearse, commit)
		},
	}

	cmd.Flags().StringVar(&targetDir, "target-dir", "", "Override Pulumi plugin directory")

	return cmd
}

// reportAnalyzerUninstall reports a rehearsed (dryRun) or completed uninstall of
// analyzer version ver from pluginDir, as JSON in machine mode.
func reportAnalyzerUninstall(cmd *cobra.Command, pluginDir, ver string, dryRun bool) error {
	if machineOutputRequested(cmd) {
		action := "uninstalled"
		if dryRun {
			action = "would_uninstall"
		}
		return writeJSON(cmd, actionResult{Action: action, DryRun: dryRun, Path: pluginDir, Version: ver})
	}
	if dryRun {
		cmd.Printf("Would uninstall analyzer")
		if ver != "" {
			cmd.Printf(" v%s", ver)
		}
		cmd.Printf("\n")
		return nil
	}
	cmd.Printf("Analyzer uninstalled successfully\n")
	if ver != "" {
		cmd.Printf("  Removed: v%s\n", ver)
	}
	return nil
}
