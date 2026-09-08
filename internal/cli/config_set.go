package cli

import (
	"context"
	"fmt"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigSetCmd creates the config set command for setting configuration values.
func NewConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Sets a configuration value using dot notation. The configuration will be saved to ~/.finfocus/config.yaml.

For sensitive values like API keys or credentials, use environment variables instead:
  export FINFOCUS_PLUGIN_AWS_SECRET_KEY="mysecret"
  export FINFOCUS_PLUGIN_AZURE_CLIENT_SECRET="secret"`,
		Example: `  # Set output format
  finfocus config set output.default_format json

  # Set output precision
  finfocus config set output.precision 4

  # Set plugin configuration
  finfocus config set plugins.aws.region us-west-2
  finfocus config set plugins.aws.profile production

  # Set logging level
  finfocus config set logging.level debug

  # For sensitive values, use environment variables instead
  export FINFOCUS_PLUGIN_AWS_SECRET_KEY="mysecret"`,
		Args: cobra.ExactArgs(2), //nolint:mnd // Exactly 2 args: key and value
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key := args[0]
			value := args[1]

			cfg := config.New()

			// Set the value in memory
			if err := cfg.Set(key, value); err != nil {
				return fmt.Errorf("failed to set config value: %w", err)
			}

			// Validate the configuration
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			// Rehearse: report what would be set
			rehearse := func(_ context.Context) error {
				cmd.Printf("Would set: %s = %s\n", key, value)
				return nil
			}

			// Commit: save the configuration
			commit := func(_ context.Context) error {
				if err := cfg.Save(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				cmd.Printf("Configuration updated: %s = %s\n", key, value)

				return nil
			}

			return ax.Perform(ctx, rehearse, commit)
		},
	}

	return cmd
}
