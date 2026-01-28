package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/router"
)

// NewConfigValidateCmd creates the config validate command for validating configuration.
func NewConfigValidateCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long: `Validates the configuration file at ~/.finfocus/config.yaml for syntax and semantic correctness.

This includes:
- General configuration syntax validation
- Routing configuration validation (if present):
  - Plugin existence verification
  - Pattern syntax validation (glob and regex)
  - Feature name validation
  - Priority value validation
  - Duplicate plugin detection`,
		Example: `  # Validate current configuration
  finfocus config validate

  # Validate and show detailed information
  finfocus config validate --verbose`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigValidate(cmd, verbose)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed validation information")

	return cmd
}

// runConfigValidate executes the configuration validation logic.
func runConfigValidate(cmd *cobra.Command, verbose bool) error {
	cfg := config.New()

	// Validate general configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Validate routing configuration if present
	hasRoutingWarnings, err := validateRoutingConfig(cmd, cfg)
	if err != nil {
		return err
	}

	if hasRoutingWarnings {
		cmd.Println()
	}
	cmd.Printf("✅ Configuration is valid\n")

	if verbose {
		printVerboseDetails(cmd, cfg)
	}

	return nil
}

// validateRoutingConfig validates the routing configuration against available plugins.
// Returns true if there are warnings, and an error if validation failed.
func validateRoutingConfig(cmd *cobra.Command, cfg *config.Config) (bool, error) {
	if cfg.Routing == nil {
		return false, nil
	}

	// Get plugin clients for validation
	reg := registry.NewDefault()
	clients, cleanup, err := reg.Open(cmd.Context(), "")
	if err != nil {
		cmd.PrintErrln("Warning: Could not load plugins for validation:", err)
		clients = nil
	} else {
		defer cleanup()
	}

	// Validate routing config against available plugins
	result := router.ValidateRoutingConfig(cfg.Routing, clients)

	if result.HasErrors() {
		cmd.PrintErrln("Routing configuration errors:")
		for _, e := range result.Errors {
			cmd.PrintErrf("  - %s\n", e.Error())
		}
		return false, fmt.Errorf("routing configuration has %d error(s)", len(result.Errors))
	}

	if result.HasWarnings() {
		cmd.Println("Routing configuration warnings:")
		for _, w := range result.WarningMessages() {
			cmd.Printf("  - %s\n", w)
		}
		return true, nil
	}

	return false, nil
}

// printVerboseDetails prints detailed configuration information.
func printVerboseDetails(cmd *cobra.Command, cfg *config.Config) {
	cmd.Println()
	cmd.Println("Configuration details:")
	cmd.Printf("  Output format: %s\n", cfg.Output.DefaultFormat)
	cmd.Printf("  Output precision: %d\n", cfg.Output.Precision)
	cmd.Printf("  Logging level: %s\n", cfg.Logging.Level)
	cmd.Printf("  Log file: %s\n", cfg.Logging.File)

	printPluginDetails(cmd, cfg)
	printRoutingDetails(cmd, cfg)
}

// printPluginDetails prints configured plugins information.
func printPluginDetails(cmd *cobra.Command, cfg *config.Config) {
	if len(cfg.Plugins) > 0 {
		cmd.Printf("  Configured plugins: %d\n", len(cfg.Plugins))
		for pluginName := range cfg.Plugins {
			cmd.Printf("    - %s\n", pluginName)
		}
	} else {
		cmd.Println("  No plugins configured")
	}
}

// printRoutingDetails prints routing configuration summary.
func printRoutingDetails(cmd *cobra.Command, cfg *config.Config) {
	if cfg.Routing == nil || len(cfg.Routing.Plugins) == 0 {
		cmd.Println("  No routing rules configured (automatic routing)")
		return
	}

	cmd.Printf("  Routing rules: %d\n", len(cfg.Routing.Plugins))
	for _, p := range cfg.Routing.Plugins {
		cmd.Printf("    - %s (priority: %d", p.Name, p.Priority)
		if len(p.Features) > 0 {
			cmd.Printf(", features: %v", p.Features)
		}
		if len(p.Patterns) > 0 {
			cmd.Printf(", patterns: %d", len(p.Patterns))
		}
		cmd.Println(")")
	}
}
