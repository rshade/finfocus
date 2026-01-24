package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigValidateCmd creates the config validate command for validating configuration.
func NewConfigValidateCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long:  "Validates the configuration file at ~/.finfocus/config.yaml for syntax and semantic correctness.",
		Example: `  # Validate current configuration
  finfocus config validate

  # Validate and show detailed information
  finfocus config validate --verbose`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.New()

			// Validate configuration
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			cmd.Printf("✅ Configuration is valid\n")

			if verbose {
				cmd.Printf("\nConfiguration details:\n")
				cmd.Printf("- Output format: %s\n", cfg.Output.DefaultFormat)
				cmd.Printf("- Output precision: %d\n", cfg.Output.Precision)
				cmd.Printf("- Logging level: %s\n", cfg.Logging.Level)
				cmd.Printf("- Log file: %s\n", cfg.Logging.File)

				if len(cfg.Plugins) > 0 {
					cmd.Printf("- Configured plugins: %d\n", len(cfg.Plugins))
					for pluginName := range cfg.Plugins {
						cmd.Printf("  - %s\n", pluginName)
					}
				} else {
					cmd.Printf("- No plugins configured\n")
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed validation information")

	return cmd
}
