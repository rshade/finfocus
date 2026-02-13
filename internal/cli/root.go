package cli

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/config"
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

// CostFlags holds the budget exit flags for the cost command group.
// These are persistent flags that apply to all cost subcommands.
type CostFlags struct {
	ExitOnThreshold bool
	ExitCode        int
	BudgetScope     string // Filter which budget scopes to display (T025)
}

// newCostCmd creates the cost command group with projected, actual, and recommendations subcommands.
// It also adds persistent flags for budget exit code configuration (Issue #219).
//
//nolint:gocognit // Function is logically cohesive; complexity comes from config setup.
func newCostCmd() *cobra.Command {
	var flags CostFlags

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Cost calculation commands",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Call root command's PersistentPreRunE to ensure logging/tracing is set up.
			// Cobra child commands override parent's PersistentPreRunE, so we must call explicitly.
			// Navigate to the root command to avoid recursion. We pass root itself as the command
			// to prevent Cobra from traversing back through the parent chain.
			root := cmd.Root()
			if root != nil && root.PersistentPreRunE != nil && root != cmd {
				if err := root.PersistentPreRunE(root, args); err != nil {
					return err
				}
			}

			// Apply CLI flag overrides to the global config if flags were explicitly set
			cfg := config.GetGlobalConfig()
			if cfg == nil {
				return nil
			}

			// Ensure budgets config structure exists for CLI flag overrides
			if cfg.Cost.Budgets == nil {
				cfg.Cost.Budgets = &config.BudgetsConfig{}
			}
			if cfg.Cost.Budgets.Global == nil {
				cfg.Cost.Budgets.Global = &config.ScopedBudget{}
			}

			// CLI flags override environment variables and config file
			if cmd.Flags().Changed("exit-on-threshold") {
				cfg.Cost.Budgets.Global.ExitOnThreshold = &flags.ExitOnThreshold
			}
			if cmd.Flags().Changed("exit-code") {
				cfg.Cost.Budgets.Global.ExitCode = &flags.ExitCode
			}

			// Validate budget configuration if ExitOnThreshold is enabled (T048)
			if cfg.Cost.Budgets.Global.ExitOnThreshold != nil && *cfg.Cost.Budgets.Global.ExitOnThreshold {
				if err := cfg.Cost.Budgets.Global.Validate(""); err != nil {
					return fmt.Errorf("invalid budget configuration: %w", err)
				}
			}

			return nil
		},
	}

	// Add persistent flags for budget exit behavior (T046, T047)
	cmd.PersistentFlags().BoolVar(&flags.ExitOnThreshold, "exit-on-threshold", false,
		"Exit with non-zero code when budget thresholds are exceeded")
	cmd.PersistentFlags().IntVar(&flags.ExitCode, "exit-code", 1,
		"Exit code to use when budget thresholds are exceeded (0-255)")

	// Add persistent flag for budget scope filtering (T025)
	cmd.PersistentFlags().StringVar(&flags.BudgetScope, "budget-scope", "",
		"Filter budget scopes to display: global, provider, provider=aws, tag, type (comma-separated)")

	// Add persistent flag for Pulumi stack selection during auto-detection
	cmd.PersistentFlags().String("stack", "",
		"Pulumi stack name for auto-detection (ignored with --pulumi-json/--pulumi-state)")

	cmd.AddCommand(NewCostProjectedCmd(), NewCostActualCmd(), NewCostRecommendationsCmd(), NewCostEstimateCmd())
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
