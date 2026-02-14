package cli

import (
	"github.com/spf13/cobra"
)

// NewAnalyzerCmd creates the analyzer command group for Pulumi Analyzer plugin functionality.
//
// The analyzer command provides subcommands for running FinFocus as a Pulumi Analyzer plugin.
// NewAnalyzerCmd creates the "analyzer" cobra command group for Pulumi Analyzer plugin functionality.
// 
// The command group includes subcommands to serve, install, and uninstall the FinFocus analyzer.
// The analyzer provides cost estimation during `pulumi preview` by communicating with the Pulumi engine
// via gRPC and returning cost diagnostics that appear in the CLI output.
// 
// It returns a configured *cobra.Command representing the analyzer command and its subcommands.
func NewAnalyzerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyzer",
		Short: "Pulumi Analyzer plugin commands",
		Long: `Commands for running FinFocus as a Pulumi Analyzer plugin.

The analyzer plugin provides cost estimation during pulumi preview operations.
It communicates with the Pulumi engine via gRPC and returns cost diagnostics
that appear in the CLI output.`,
		Example: `  # Start the analyzer server (used by Pulumi engine)
  finfocus analyzer serve

  # Start with debug logging
  finfocus analyzer serve --debug`,
	}

	cmd.AddCommand(NewAnalyzerServeCmd())
	cmd.AddCommand(NewAnalyzerInstallCmd())
	cmd.AddCommand(NewAnalyzerUninstallCmd())

	return cmd
}