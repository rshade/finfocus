package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/router"
)

// NewConfigValidateCmd returns a Cobra command that validates the application's configuration.
// The command checks general configuration syntax and routing semantics, including plugin existence,
// pattern and feature validation, priority values, and duplicate plugin detection.
// The returned command accepts a --verbose / -v flag to emit detailed validation information.
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

// runConfigValidate validates the application's configuration and reports results to cmd.
// It validates both general and routing configuration. Human output prints any routing
// warnings, a blank separator line, and the success message; with --format json it writes
// a single JSON object ({"valid": true, "warnings": [...]}) instead.
// cmd is used for CLI output. If verbose is true, detailed configuration information is printed.
// It returns an error when validation fails.
func runConfigValidate(cmd *cobra.Command, verbose bool) error {
	cfg := config.New()

	// Validate general configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Validate routing configuration if present
	warnings, err := validateRoutingConfig(cmd, cfg)
	if err != nil {
		return err
	}

	if machineOutputRequested(cmd) {
		return renderConfigValidateJSON(cmd, warnings)
	}

	if len(warnings) > 0 {
		cmd.Println("Routing configuration warnings:")
		for _, w := range warnings {
			cmd.Printf("  - %s\n", w)
		}
		cmd.Println()
	}
	cmd.Printf("✅ Configuration is valid\n")

	if verbose {
		printVerboseDetails(cmd, cfg)
	}

	return nil
}

// configValidateJSON is the machine-readable result of a successful config validate.
type configValidateJSON struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings"`
}

// renderConfigValidateJSON writes the successful validation result as JSON.
func renderConfigValidateJSON(cmd *cobra.Command, warnings []string) error {
	if warnings == nil {
		warnings = []string{}
	}
	return writeJSON(cmd, configValidateJSON{Valid: true, Warnings: warnings})
}

// validateRoutingConfig validates routing-related configuration against the available plugins.
// Errors are reported to the command's stderr and returned as a non-nil error.
//
// cmd is the Cobra command used for printing validation errors and for deriving a context when loading plugins.
// cfg is the loaded configuration to validate; if cfg.Routing is nil the function performs no validation.
//
// It returns the routing warning messages (nil when there are none). When plugin loading fails the function
// emits a warning to stderr and proceeds with validation without plugin clients.
func validateRoutingConfig(cmd *cobra.Command, cfg *config.Config) ([]string, error) {
	if cfg.Routing == nil {
		return nil, nil
	}

	// Get plugin clients for validation
	reg := registry.NewDefault()
	clients, cleanup, err := reg.Open(cmd.Context(), "")
	if err != nil {
		cmd.PrintErrln("Warning: Could not load plugins for validation:", err)
		// Create synthetic clients from config names so pattern/feature validation still runs
		clients = make([]*pluginhost.Client, 0, len(cfg.Routing.Plugins))
		for _, p := range cfg.Routing.Plugins {
			clients = append(clients, &pluginhost.Client{Name: p.Name})
		}
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
		return nil, fmt.Errorf("routing configuration has %d error(s)", len(result.Errors))
	}

	if result.HasWarnings() {
		return result.WarningMessages(), nil
	}

	return nil, nil
}

// printVerboseDetails prints detailed configuration information to the command's output.
// It writes the output format, output precision, logging level, and log file, then
// prints plugin and routing summaries via printPluginDetails and printRoutingDetails.
// cmd is the Cobra command used for printing output.
// cfg is the configuration whose details are displayed.
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

// printPluginDetails writes a summary of configured plugins to the command output.
// It prints the number of plugins and each plugin name from cfg.Plugins, or "No plugins configured" if none.
func printPluginDetails(cmd *cobra.Command, cfg *config.Config) {
	if len(cfg.Plugins) > 0 {
		cmd.Printf("  Configured plugins: %d\n", len(cfg.Plugins))
		names := make([]string, 0, len(cfg.Plugins))
		for name := range cfg.Plugins {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, pluginName := range names {
			cmd.Printf("    - %s\n", pluginName)
		}
	} else {
		cmd.Println("  No plugins configured")
	}
}

// printRoutingDetails writes a summary of the routing configuration to the command's output.
// It reports when automatic routing is in use, or lists each configured routing plugin with its name,
// priority, features (if any), and the number of patterns (if any).
// cmd is the Cobra command used for printing output.
// cfg is the configuration whose Routing field provides the routing rules to summarize.
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
