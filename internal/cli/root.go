package cli

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/migration"
)

// isTerminal checks if the given file is a terminal.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// logger is the package-level logger for CLI operations.
var logger zerolog.Logger //nolint:gochecknoglobals // Required for zerolog context integration

// NewRootCmd creates the root Cobra command for the finfocus CLI.
// It wires up logging, tracing, audit logging, and subcommands (cost, plugin, config, analyzer).
// The command dynamically adjusts its Use and Example strings based on whether it's running
// as a Pulumi tool plugin (detected via binary name or FINFOCUS_PLUGIN_MODE env var).
func NewRootCmd(ver string) *cobra.Command {
	return NewRootCmdWithArgs(ver, os.Args, os.LookupEnv)
}

// NewRootCmdWithArgs creates the root command with explicit args and env lookup for testability.
// This allows tests to inject custom args and environment variables.
func NewRootCmdWithArgs(
	ver string,
	args []string,
	lookupEnv func(string) (string, bool),
) *cobra.Command {
	var logResult *logging.LogPathResult

	// Detect plugin mode from binary name or environment variable
	pluginMode := DetectPluginMode(args, lookupEnv)

	// Select the appropriate Use and Example strings based on mode
	useName := "finfocus"
	example := rootCmdExample
	if pluginMode {
		useName = "pulumi plugin run tool cost"
		example = pluginCmdExample
	}

	cmd := &cobra.Command{
		Use:     useName,
		Short:   "FinFocus CLI and plugin host",
		Long:    "FinFocus: Calculate projected and actual cloud costs via plugins",
		Version: ver,
		Example: example,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Validate cache-ttl is non-negative (negative values cause undefined cache expiry behavior)
			cacheTTL, _ := cmd.Flags().GetInt("cache-ttl")
			if cacheTTL < 0 {
				return fmt.Errorf("cache-ttl must be >= 0, got %d", cacheTTL)
			}

			// Check for migration if in interactive terminal
			_, skipMigration := lookupEnv("FINFOCUS_SKIP_MIGRATION_CHECK")
			if isTerminal(os.Stdin) && !skipMigration {
				if err := migration.RunMigration(cmd.OutOrStdout(), cmd.InOrStdin()); err != nil {
					// We log the error but don't fail the command as migration is best-effort
					cmd.PrintErrf("Warning: migration check failed: %v\n", err)
				}

				// Alias reminder - use precomputed pluginMode for consistency with tests
				if os.Getenv("FINFOCUS_HIDE_ALIAS_HINT") == "" && !pluginMode {
					msg := "Tip: Add 'alias fin=finfocus' to your shell profile for a shorter command!"
					cmd.PrintErrln(msg)
				}
			}

			result := setupLogging(cmd)
			logResult = &result
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, _ []string) error {
			return cleanupLogging(cmd, logResult)
		},
	}

	cmd.PersistentFlags().Bool("debug", false, "enable debug logging")
	cmd.PersistentFlags().
		Bool("skip-version-check", false, "skip plugin spec version compatibility check")
	cmd.PersistentFlags().
		Int("cache-ttl", 0, "cache TTL in seconds (0 = use config default, overrides config file and env var)")
	cmd.AddCommand(newCostCmd(), newPluginCmd(), newConfigCmd(), NewAnalyzerCmd())

	return cmd
}

const rootCmdExample = `  # Calculate projected costs from a Pulumi plan
  finfocus cost projected --pulumi-json plan.json

  # Get actual costs for the last 7 days
  finfocus cost actual --pulumi-json plan.json --from 2025-01-07

  # Get cost recommendations with custom cache TTL (5 minutes)
  finfocus cost recommendations --pulumi-json plan.json --cache-ttl 300

  # Install a plugin from registry
  finfocus plugin install kubecost

  # List installed plugins
  finfocus plugin list

  # Initialize a new plugin project
  finfocus plugin init aws-plugin --author "Your Name" --providers aws

  # Validate all plugins
  finfocus plugin validate

  # Initialize configuration
  finfocus config init

  # Set configuration values
  finfocus config set output.default_format json`

// pluginCmdExample is the example text shown when running as a Pulumi tool plugin.
const pluginCmdExample = `  # Calculate projected costs from a Pulumi plan
  pulumi plugin run tool cost -- cost projected --pulumi-json plan.json

  # Get actual costs for the last 7 days
  pulumi plugin run tool cost -- cost actual --pulumi-json plan.json --from 2025-01-07

  # List installed plugins
  pulumi plugin run tool cost -- plugin list

  # Validate all plugins
  pulumi plugin run tool cost -- plugin validate

  # Initialize configuration
  pulumi plugin run tool cost -- config init

  # Set configuration values
  pulumi plugin run tool cost -- config set output.default_format json`

// newCostCmd creates the cost command group with projected, actual, and recommendations subcommands.
func newCostCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "cost", Short: "Cost calculation commands"}
	cmd.AddCommand(NewCostProjectedCmd(), NewCostActualCmd(), NewCostRecommendationsCmd())
	return cmd
}

// newPluginCmd creates the plugin command group with management subcommands.
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plugin", Short: "Plugin management commands"}
	cmd.AddCommand(
		NewPluginValidateCmd(), NewPluginListCmd(), NewPluginInitCmd(),
		NewPluginInstallCmd(), NewPluginUpdateCmd(), NewPluginRemoveCmd(),
		NewPluginConformanceCmd(), NewPluginCertifyCmd(), NewPluginInspectCmd(),
	)
	return cmd
}

// newConfigCmd creates the config command group with configuration subcommands.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration management commands"}
	cmd.AddCommand(
		NewConfigInitCmd(), NewConfigSetCmd(), NewConfigGetCmd(),
		NewConfigListCmd(), NewConfigValidateCmd(),
	)
	return cmd
}
