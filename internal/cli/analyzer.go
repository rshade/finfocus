package cli

import (
	"github.com/spf13/cobra"
)

// NewAnalyzerCmd creates the analyzer command group for Pulumi Analyzer plugin functionality.
//
// The analyzer command provides subcommands for running FinFocus as a Pulumi Analyzer plugin.
// This enables zero-click cost estimation during `pulumi preview` operations.
func NewAnalyzerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyzer",
		Short: "Pulumi Analyzer plugin commands",
		Long: `Commands for running FinFocus as a Pulumi Analyzer plugin.

The analyzer plugin provides cost estimation during pulumi preview operations.
It communicates with the Pulumi engine via gRPC and returns cost diagnostics
that appear in the CLI output.`,
		Example: `  # Install analyzer plugin and policy pack
  finfocus analyzer install

  # Verify analyzer setup
  finfocus analyzer check

  # Start the analyzer server (used by Pulumi engine)
  finfocus analyzer serve --debug`,
	}

	cmd.AddCommand(NewAnalyzerServeCmd())
	cmd.AddCommand(NewAnalyzerInstallCmd())
	cmd.AddCommand(NewAnalyzerCheckCmd())
	cmd.AddCommand(NewAnalyzerUninstallCmd())

	return cmd
}
