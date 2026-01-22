package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigListCmd creates the config list command for listing all configuration values.
func NewConfigListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long:  "Lists all configuration values from ~/.finfocus/config.yaml in the specified format.",
		Example: `  # List all configuration (default YAML format)
  finfocus config list
  
  # List configuration in JSON format
  finfocus config list --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// config.New() already loads from disk and applies env overrides
			cfg := config.New()

			// Get all configuration
			allConfig := cfg.List()

			// Format and output based on requested format
			switch format {
			case "json":
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
				return fmt.Errorf("unsupported format: %s (supported: json, yaml, yml)", format)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "yaml", "output format (yaml, json)")

	return cmd
}
