package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rshade/ax-go"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigListCmd creates the config list command for listing all configuration values.
//
// The output style is chosen via --as (yaml, json, or yml) rather than
// --format: --format is a persistent flag ax.Execute mounts on the root
// command for agent/human mode selection (json|human), and a local flag of
// the same name on this command would shadow it, breaking --format for every
// invocation of "config list". When --as is not explicitly set, ax's
// resolved mode (if json) selects JSON output, otherwise the yaml default.
func NewConfigListCmd() *cobra.Command {
	var as string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long:  "Lists all configuration values from the config file in the specified style.",
		Example: `  # List all configuration (default YAML style)
  finfocus config list

  # List configuration in JSON style
  finfocus config list --as json

  # --format json (the global agent-mode flag) also selects JSON style
  finfocus config list --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			style := as
			if !cmd.Flags().Changed("as") {
				if mode, ok := ax.ModeFromContext(ctx); ok && mode == ax.ModeJSON {
					style = outputFormatJSON
				}
			}

			// config.New() already loads from disk and applies env overrides
			cfg := config.New()

			// Get all configuration
			allConfig := cfg.List()

			// Format and output based on requested style
			switch style {
			case outputFormatJSON:
				jsonData, err := json.MarshalIndent(allConfig, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal config to JSON: %w", err)
				}
				cmd.Printf("%s\n", jsonData)

			case "yaml", "yml":
				yamlData, err := yaml.Marshal(allConfig)
				if err != nil {
					return fmt.Errorf("failed to marshal config to YAML: %w", err)
				}
				cmd.Printf("%s\n", yamlData)

			default:
				return fmt.Errorf("unsupported style: %s (supported: json, yaml, yml)", style)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&as, "as", "f", "yaml", "output style (yaml, json)")

	return cmd
}
