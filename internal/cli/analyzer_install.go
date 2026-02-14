package cli

import (
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/analyzer"
)

// NewAnalyzerInstallCmd creates the analyzer install command.
//
// This command installs the finfocus binary as a Pulumi analyzer plugin by
// creating a symlink (Unix) or copy (Windows) in the Pulumi plugin directory.
func NewAnalyzerInstallCmd() *cobra.Command {
	var force bool
	var targetDir string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Pulumi analyzer plugin",
		Long: `Installs the finfocus binary as a Pulumi Analyzer plugin.

This replaces the manual 4-step process of finding the binary, determining
the version, creating the plugin directory, and creating a symlink.

The analyzer is installed to the Pulumi plugin directory with the correct
naming convention so that Pulumi can discover it automatically.`,
		Example: `  # Install the analyzer
  finfocus analyzer install

  # Force reinstall (after upgrading finfocus)
  finfocus analyzer install --force

  # Install to a custom directory
  finfocus analyzer install --target-dir /opt/pulumi/plugins`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			opts := analyzer.InstallOptions{
				Force:     force,
				TargetDir: targetDir,
			}

			result, err := analyzer.Install(ctx, opts)
			if err != nil {
				return err
			}

			switch {
			case result.Method != "":
				// Fresh install or force reinstall
				cmd.Printf("Analyzer installed successfully\n")
				cmd.Printf("  Version: %s\n", result.Version)
				cmd.Printf("  Path: %s\n", result.Path)
				cmd.Printf("  Method: %s\n", result.Method)
			case result.NeedsUpdate:
				// Installed but different version
				cmd.Printf("Analyzer already installed at v%s\n", result.Version)
				cmd.Printf("  Path: %s\n", result.Path)
				cmd.Printf("  Current finfocus version: v%s\n", result.CurrentVersion)
				cmd.Printf("  Use --force to upgrade\n")
			default:
				// Already installed, same version
				cmd.Printf("Analyzer already installed at v%s\n", result.Version)
				cmd.Printf("  Path: %s\n", result.Path)
				cmd.Printf("  Use --force to reinstall\n")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing installation")
	cmd.Flags().StringVar(&targetDir, "target-dir", "", "Override Pulumi plugin directory")

	return cmd
}
