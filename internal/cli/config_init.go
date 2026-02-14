package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigInitCmd creates the config init command for initializing configuration.
// When run inside a Pulumi project (without --global), it creates a project-local
// .finfocus/ directory with config.yaml and .gitignore. Otherwise, it creates the
// global ~/.finfocus/config.yaml.
func NewConfigInitCmd() *cobra.Command {
	var (
		force  bool
		global bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file with default values",
		Long: `Creates a new configuration file with default values.

When run inside a Pulumi project, creates project-local configuration at
$PROJECT/.finfocus/config.yaml with a .gitignore to protect user-specific data.
Use --global to force global configuration initialization even inside a project.`,
		Example: `  # Create project-local configuration (inside a Pulumi project)
  finfocus config init

  # Create global configuration
  finfocus config init --global

  # Create configuration, overwriting existing
  finfocus config init --force`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectDir := config.GetResolvedProjectDir()

			if projectDir != "" && !global {
				return initProjectConfig(cmd, projectDir, force)
			}

			return initGlobalConfig(cmd, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration file")
	cmd.Flags().BoolVar(&global, "global", false, "force global configuration init even inside a Pulumi project")

	return cmd
}

// initProjectConfig creates project-local config at projectDir/config.yaml with .gitignore.
func initProjectConfig(cmd *cobra.Command, projectDir string, force bool) error {
	configPath := filepath.Join(projectDir, "config.yaml")

	// Check if config already exists and force isn't set
	if !force {
		_, err := os.Stat(configPath)
		if err == nil {
			return errors.New("configuration file already exists, use --force to overwrite")
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access config path %s: %w", configPath, err)
		}
	}

	// Ensure the project .finfocus/ directory exists
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		return fmt.Errorf("failed to create project config directory: %w", err)
	}

	// Save default configuration to project directory
	cfg := config.New()
	cfg.SetConfigPath(configPath)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Create .gitignore (never overwrites existing)
	created, err := config.EnsureGitignore(projectDir)
	if err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	cmd.Printf("Configuration initialized at %s\n", configPath)
	if created {
		cmd.Printf("Created .gitignore to protect user-specific data\n")
	}

	return nil
}

// initGlobalConfig creates global config at ~/.finfocus/config.yaml.
func initGlobalConfig(cmd *cobra.Command, force bool) error {
	cfg := config.New()

	// Check if config already exists and force isn't set
	if !force {
		if _, err := os.Stat(cfg.ConfigPath()); err == nil {
			return errors.New("configuration file already exists, use --force to overwrite")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access config path %s: %w", cfg.ConfigPath(), err)
		}
	}

	// Save the default configuration
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	cmd.Printf("Configuration initialized successfully\n")
	cmd.Printf("Configuration file: %s\n", cfg.ConfigPath())

	return nil
}
