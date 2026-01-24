package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
)

// ErrPluginValidationFailed is returned when one or more plugins fail validation.
// This replaces os.Exit(1) to allow proper testing and error handling (SC-002 fix).
var ErrPluginValidationFailed = errors.New("one or more plugins failed validation")

// NewPluginValidateCmd creates the plugin validate command for validating plugin installations.
func NewPluginValidateCmd() *cobra.Command {
	var targetPlugin string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate installed plugins",
		Long:  "Validate that installed plugins can be loaded and respond to basic API calls",
		Example: `  # Validate all installed plugins
  finfocus plugin validate

  # Validate a specific plugin
  finfocus plugin validate --plugin aws-plugin

  # Validate kubecost plugin specifically
  finfocus plugin validate --plugin kubecost`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginValidateCmd(cmd, targetPlugin)
		},
	}

	cmd.Flags().StringVar(&targetPlugin, "plugin", "", "Validate a specific plugin by name")

	return cmd
}

// ValidatePlugin validates a plugin's binary and its optional manifest.
// It ensures the binary exists at plugin.Path, is not a directory, and has the executable bit set.
// If a manifest file named "plugin.manifest.json" exists in the same directory, it is loaded
// and its Name and Version fields are compared to plugin.Name and plugin.Version.
// The function returns nil when all checks pass; otherwise it returns an error describing the
// first failing check (missing binary, stat error, path is a directory, not executable,
// invalid manifest, or manifest name/version mismatch).
//
// ctx is reserved for future use and is not inspected.
// plugin provides the plugin's Path, Name, and Version used for validation.
func ValidatePlugin(_ context.Context, plugin registry.PluginInfo) error {
	if _, err := os.Stat(plugin.Path); err != nil {
		return fmt.Errorf("plugin binary not found: %s", plugin.Path)
	}

	info, err := os.Stat(plugin.Path)
	if err != nil {
		return fmt.Errorf("cannot stat plugin binary: %w", err)
	}

	if info.IsDir() {
		return errors.New("plugin path is a directory, not a binary")
	}

	if info.Mode()&0111 == 0 {
		return errors.New("plugin binary is not executable")
	}

	manifestPath := filepath.Join(filepath.Dir(plugin.Path), "plugin.manifest.json")
	if _, statErr := os.Stat(manifestPath); statErr == nil {
		manifest, loadErr := registry.LoadManifest(manifestPath)
		if loadErr != nil {
			return fmt.Errorf("invalid manifest: %w", loadErr)
		}
		if manifest.Name != plugin.Name {
			return fmt.Errorf(
				"manifest name mismatch: expected %s, got %s",
				plugin.Name,
				manifest.Name,
			)
		}
		if manifest.Version != plugin.Version {
			return fmt.Errorf(
				"manifest version mismatch: expected %s, got %s",
				plugin.Version,
				manifest.Version,
			)
		}
	}

	return nil
}

// runPluginValidateCmd validates installed plugins or a specific plugin and prints status to the provided command output.
// It loads configuration to locate the plugin directory, lists installed plugins from the default registry,
// filters by the optional targetPlugin, and executes the validation run for the selected plugins.
//
// Parameters:
//   - cmd: the Cobra command used for printing progress and messages.
//   - targetPlugin: optional plugin name to validate; when empty all installed plugins are validated.
//
// Returns:
//   - an error if the requested plugin cannot be found (when the plugin directory is missing or no plugins are installed),
//     if listing plugins fails (wrapped), or if filtering plugins fails. Returns nil when validation is performed or when
//     there are no plugins to validate and no specific target was requested.
func runPluginValidateCmd(cmd *cobra.Command, targetPlugin string) error {
	ctx := context.Background()

	cfg := config.New()
	if _, err := os.Stat(cfg.PluginDir); os.IsNotExist(err) {
		if targetPlugin != "" {
			return fmt.Errorf(
				"plugin '%s' not found: plugin directory does not exist",
				targetPlugin,
			)
		}
		cmd.Printf("Plugin directory does not exist: %s\n", cfg.PluginDir)
		cmd.Println("No plugins to validate.")
		return nil
	}

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()
	if err != nil {
		return fmt.Errorf("listing plugins: %w", err)
	}

	if len(plugins) == 0 {
		if targetPlugin != "" {
			return fmt.Errorf("plugin '%s' not found: no plugins installed", targetPlugin)
		}
		cmd.Println("No plugins found to validate.")
		return nil
	}

	plugins, err = filterPlugins(plugins, targetPlugin)
	if err != nil {
		return err
	}

	return runValidation(ctx, cmd, plugins)
}

// filterPlugins returns a slice containing only plugins whose Name equals targetPlugin.
// If targetPlugin is empty, the original plugins slice is returned unchanged.
// If targetPlugin is non-empty and no plugins match, an error is returned.
//
// plugins is the list of available plugins to filter.
// targetPlugin is the plugin name to select.
//
// The returned slice contains all matching PluginInfo entries when matches exist,
// otherwise returns an error indicating the plugin was not found.
func filterPlugins(
	plugins []registry.PluginInfo,
	targetPlugin string,
) ([]registry.PluginInfo, error) {
	if targetPlugin == "" {
		return plugins, nil
	}

	var filtered []registry.PluginInfo
	for _, p := range plugins {
		if p.Name == targetPlugin {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("plugin '%s' not found", targetPlugin)
	}
	return filtered, nil
}

// runValidation validates a set of plugins, prints per-plugin progress and a final summary.
//
// It validates each plugin in the provided slice and prints progress messages to the supplied
// Cobra command's output. Returns ErrPluginValidationFailed if any plugin fails validation,
// allowing callers to handle the error appropriately.
//
// Parameters:
//   - ctx: the context for validation operations.
//   - cmd: the Cobra command used for printing progress and summary messages.
//   - plugins: the list of plugins to validate.
//
// Returns:
//   - error: returns ErrPluginValidationFailed if any plugin fails validation,
//     nil otherwise.
func runValidation(ctx context.Context, cmd *cobra.Command, plugins []registry.PluginInfo) error {
	cmd.Printf("Validating %d plugin(s)...\n\n", len(plugins))

	validCount := 0
	for _, plugin := range plugins {
		if validateSinglePlugin(ctx, cmd, plugin) {
			validCount++
		}
	}

	cmd.Printf("\nValidation complete: %d/%d plugins valid\n", validCount, len(plugins))

	if validCount < len(plugins) {
		return ErrPluginValidationFailed
	}
	return nil
}

// validateSinglePlugin validates the given plugin and reports progress to the provided command output.
// It prints a per-plugin status line, prints any validation error, and a success message on success.
//
// Parameters:
//
//	ctx: context for cancellation and deadlines affecting validation operations.
//	cmd: Cobra command used to write progress and result messages to the command's output.
//	plugin: plugin information (name, version, path) identifying the plugin to validate.
//
// Returns:
//
//	true if the plugin passed validation and a success message was printed, false if validation failed
//	and an error message was printed.
func validateSinglePlugin(
	ctx context.Context,
	cmd *cobra.Command,
	plugin registry.PluginInfo,
) bool {
	cmd.Printf("Validating %s v%s... ", plugin.Name, plugin.Version)

	if err := ValidatePlugin(ctx, plugin); err != nil {
		cmd.Printf("FAILED: %v\n", err)
		return false
	}

	cmd.Println("OK")
	return true
}
