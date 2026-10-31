package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigSetCmd creates the config set command for setting configuration values.
func NewConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Sets a configuration value using dot notation. The configuration will be saved to ~/.finfocus/config.hujson.

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
			path := filepath.Join(config.ResolveConfigDir(), "config.hujson")
			change := config.Change{Key: key, Value: &value}
			shown := value
			if config.IsSecretSetting(config.CanonicalKey(key)) {
				shown = config.RedactedValue
			}

			// Rehearse validates without writing. Commit patches the file in place
			// so environment values folded by config.New are not persisted.
			rehearse := func(_ context.Context) error {
				if _, err := config.PatchFile(path, config.TierGlobal, "", []config.Change{change}, true); err != nil {
					return err
				}
				cmd.Printf("Would set: %s = %s\n", key, shown)
				return nil
			}
			commit := func(_ context.Context) error {
				if _, err := config.PatchFile(path, config.TierGlobal, "", []config.Change{change}, false); err != nil {
					return err
				}
				cmd.Printf("Configuration updated: %s = %s\n", key, shown)
				warnIfProjectShadows(cmd, path, key)
				return nil
			}
			if err := ax.Perform(ctx, rehearse, commit); err != nil {
				wrapped := fmt.Errorf("failed to set config value: %w", err)
				var invalid *config.InvalidChangeError
				var rejected *config.TierValidationError
				if errors.As(err, &invalid) || errors.As(err, &rejected) {
					return toValidationError(ctx, wrapped)
				}
				return wrapped
			}
			return nil
		},
	}

	return cmd
}

// warnIfProjectShadows tells the user when a value written to the global file
// will not apply in the current project: the project config defines the key's
// section and replaces the whole global section.
func warnIfProjectShadows(cmd *cobra.Command, globalPath, key string) {
	if path, shadowed := config.ProjectShadowsKey(globalPath, config.GetResolvedProjectDir(), key); shadowed {
		cmd.PrintErrf(
			"Warning: %s is defined by the project config %s, which replaces the global %s section; "+
				"the value just written does not apply in this project.\n",
			key, path, config.SectionOf(key),
		)
	}
}
