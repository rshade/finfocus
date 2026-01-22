package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigInitCmd creates the config init command for initializing configuration.
func NewConfigInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file with default values",
		Long:  "Creates a new configuration file at ~/.finfocus/config.yaml with default values.",
		Example: `  # Create default configuration
  finfocus config init
  
  # Create default configuration, overwriting existing
  finfocus config init --force`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.New()

			// Check if config already exists and force isn't set
			if !force {
				if err := cfg.Load(); err == nil {
					return errors.New("configuration file already exists, use --force to overwrite")
				}
			}

			// Save the default configuration
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			cmd.Printf("Configuration initialized successfully\n")
			cmd.Printf("Configuration file: ~/.finfocus/config.yaml\n")

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration file")

	return cmd
}
