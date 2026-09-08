package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

// NewConfigInitCmd creates the config init command for initializing configuration.
// When run inside a Pulumi project (without --global), it creates a project-local
// .finfocus/ directory with config.hujson and .gitignore. Otherwise, it creates the
// global ~/.finfocus/config.hujson.
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
$PROJECT/.finfocus/config.hujson with a .gitignore to protect user-specific data.
Use --global to force global configuration initialization even inside a project.`,
		Example: `  # Create project-local configuration (inside a Pulumi project)
  finfocus config init

  # Create global configuration
  finfocus config init --global

  # Create configuration, overwriting existing
  finfocus config init --force`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			projectDir := config.GetResolvedProjectDir()

			if projectDir != "" && !global {
				return initProjectConfig(ctx, cmd, projectDir, force)
			}

			return initGlobalConfig(ctx, cmd, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration file")
	cmd.Flags().BoolVar(&global, "global", false, "force global configuration init even inside a Pulumi project")

	return cmd
}

// initProjectConfig creates project-local config at projectDir/config.hujson with .gitignore.
func initProjectConfig(ctx context.Context, cmd *cobra.Command, projectDir string, force bool) error {
	configPath := filepath.Join(projectDir, "config.hujson")

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

	// Rehearse: report what would be written
	rehearse := func(_ context.Context) error {
		cmd.Printf("Would create configuration at %s\n", configPath)
		cmd.Printf("  (with .gitignore to protect user-specific data)\n")
		return nil
	}

	// Commit: actually create the config
	commit := func(_ context.Context) error {
		// Ensure the project .finfocus/ directory exists
		if err := os.MkdirAll(projectDir, 0o750); err != nil {
			return fmt.Errorf("failed to create project config directory: %w", err)
		}

		// Write a minimal project skeleton with only comments showing override keys.
		// Unlike the global config (which writes all defaults), the project config
		// should contain only overrides — absent keys inherit from ~/.finfocus/config.hujson.
		if err := config.SaveProjectSkeleton(configPath); err != nil {
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

	return ax.Perform(ctx, rehearse, commit)
}

// initGlobalConfig creates global config at ~/.finfocus/config.hujson.
func initGlobalConfig(ctx context.Context, cmd *cobra.Command, force bool) error {
	cfg := config.New()

	// Check if config already exists and force isn't set
	if !force {
		if _, err := os.Stat(cfg.ConfigPath()); err == nil {
			return errors.New("configuration file already exists, use --force to overwrite")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access config path %s: %w", cfg.ConfigPath(), err)
		}
	}

	// Rehearse: report what would be created
	rehearse := func(_ context.Context) error {
		cmd.Printf("Would create configuration at %s\n", cfg.ConfigPath())
		return nil
	}

	// Commit: actually create the config
	commit := func(_ context.Context) error {
		// Save the default configuration
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		cmd.Printf("Configuration initialized successfully\n")
		cmd.Printf("Configuration file: %s\n", cfg.ConfigPath())

		return nil
	}

	return ax.Perform(ctx, rehearse, commit)
}
