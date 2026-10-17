package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rshade/ax-go"
	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"
)

// InstalledPlugin represents an installed plugin entry in config file.
type InstalledPlugin struct {
	Name    string `yaml:"name"    json:"name"`
	URL     string `yaml:"url"     json:"url"`
	Version string `yaml:"version" json:"version"`
}

// InstalledPluginsConfig holds the installed plugins list.
type InstalledPluginsConfig struct {
	InstalledPlugins []InstalledPlugin `yaml:"installed_plugins" json:"installed_plugins"`
}

// pluginsConfigPath returns the full path to the plugins configuration file
// following the same precedence as ResolveConfigDir(): FINFOCUS_HOME, PULUMI_HOME/finfocus, or ~/.finfocus.
func pluginsConfigPath() string {
	configDir := ResolveConfigDir()
	return filepath.Join(configDir, "config.hujson")
}

// LoadInstalledPlugins loads the list of installed plugins from the config file.
// It returns an empty list if the file does not exist, or an error if the JSON/Hujson cannot be parsed.
//
//nolint:gocognit,nestif // Function necessarily complex due to handling new format with legacy format fallback
func LoadInstalledPlugins() ([]InstalledPlugin, error) {
	configPath := pluginsConfigPath()

	// Check if new format exists, and if not, try legacy format
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Check for legacy .yaml file
		legacyPath := filepath.Join(filepath.Dir(configPath), "config.yaml")
		if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {
			// Legacy file exists, try to read it
			data, readErr := os.ReadFile(legacyPath)
			if readErr != nil {
				if os.IsNotExist(readErr) {
					return []InstalledPlugin{}, nil
				}
				return nil, fmt.Errorf("failed to read legacy config: %w", readErr)
			}

			// Try to parse as JSON first, then YAML as fallback
			var cfg InstalledPluginsConfig
			if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
				// JSON failed, try YAML
				if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr != nil {
					// Both JSON and YAML failed - this is an error
					return nil, fmt.Errorf("failed to parse legacy config: %w", yamlErr)
				}
			}
			return cfg.InstalledPlugins, nil
		}

		// No file found at all
		return []InstalledPlugin{}, nil
	}

	// Read the new format file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []InstalledPlugin{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg InstalledPluginsConfig
	if unmarshalErr := ax.ParseConfig(context.Background(), bytes.NewReader(data), &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse config: %w", unmarshalErr)
	}

	return cfg.InstalledPlugins, nil
}

// SaveInstalledPlugins saves the provided list of installed plugins into the user's FinFocus config.
// It ensures the config directory exists, preserves other top-level config keys, updates the
// `installed_plugins` entry, and performs an atomic write to the config file.
// The `plugins` parameter is the full list of plugins to persist.
// It returns an error if marshaling or file operations fail.
func SaveInstalledPlugins(plugins []InstalledPlugin) error {
	return updateInstalledPlugins(func([]InstalledPlugin) ([]InstalledPlugin, error) {
		return plugins, nil
	})
}

// AddInstalledPlugin adds or updates the given plugin in the installed plugins configuration.
// If a plugin with the same Name already exists it is replaced; otherwise the plugin is appended.
// It persists the updated list and returns an error if loading or saving the configuration fails.
func AddInstalledPlugin(plugin InstalledPlugin) error {
	return updateInstalledPlugins(func(plugins []InstalledPlugin) ([]InstalledPlugin, error) {
		for i, existing := range plugins {
			if existing.Name == plugin.Name {
				plugins[i] = plugin
				return plugins, nil
			}
		}
		return append(plugins, plugin), nil
	})
}

// RemoveInstalledPlugin removes the installed plugin with the given name from the configuration.
// It returns an error if loading the current configuration or saving the updated configuration fails.
func RemoveInstalledPlugin(name string) error {
	return updateInstalledPlugins(func(plugins []InstalledPlugin) ([]InstalledPlugin, error) {
		kept := make([]InstalledPlugin, 0, len(plugins))
		for _, existing := range plugins {
			if existing.Name != name {
				kept = append(kept, existing)
			}
		}
		return kept, nil
	})
}

// updateInstalledPlugins holds the config file lock across the read and the
// comment-preserving write so a settings save cannot land between them.
func updateInstalledPlugins(mutate func([]InstalledPlugin) ([]InstalledPlugin, error)) error {
	path := pluginsConfigPath()
	unlock, err := acquirePathLock(path)
	if err != nil {
		return err
	}
	defer unlock()
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		return err
	}
	next, err := mutate(plugins)
	if err != nil {
		return err
	}
	return writeInstalledPlugins(path, next)
}

func writeInstalledPlugins(path string, plugins []InstalledPlugin) error {
	if plugins == nil {
		plugins = []InstalledPlugin{}
	}
	payload, err := json.Marshal(plugins)
	if err != nil {
		return fmt.Errorf("failed to marshal installed plugins: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if len(bytes.TrimSpace(original)) == 0 {
		original = []byte("{}\n")
	} else if isYAMLDocument(original) {
		converted, convErr := yamlToJSON(original)
		if convErr != nil {
			return fmt.Errorf("converting legacy configuration: %w", convErr)
		}
		original = converted
	}
	value, err := hujson.Parse(original)
	if err != nil {
		return fmt.Errorf("parsing configuration: %w", err)
	}
	if err = setAtPointer(&value, "/"+keyInstalledPlugin, payload); err != nil {
		return err
	}
	_, err = writeAtomic(path, value.Pack())
	return err
}

// GetInstalledPlugin retrieves the installed plugin with the given name.
// It returns a pointer to the InstalledPlugin if found, or an error if the plugin is not present in the config or if the installed plugins cannot be loaded.
func GetInstalledPlugin(name string) (*InstalledPlugin, error) {
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	for _, p := range plugins {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("plugin %q not found in config", name)
}

// UpdateInstalledPluginVersion updates the version of the installed plugin with the given name to the provided version.
// It returns an error if the installed-plugins configuration cannot be loaded, if no plugin with the given name exists, or if saving the updated configuration fails.
func UpdateInstalledPluginVersion(name, version string) error {
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		return err
	}

	found := false
	for i, p := range plugins {
		if p.Name == name {
			plugins[i].Version = version
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("plugin %q not found in config", name)
	}

	return SaveInstalledPlugins(plugins)
}

// GetMissingPlugins returns plugins that are in config but not installed on disk.
func GetMissingPlugins() ([]InstalledPlugin, error) {
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		return nil, err
	}

	// Use the resolved plugin directory, including config and environment overrides.
	pluginsDir := New().PluginDir

	var missing []InstalledPlugin
	for _, p := range plugins {
		pluginDir := filepath.Join(pluginsDir, p.Name, p.Version)
		if _, statErr := os.Stat(pluginDir); os.IsNotExist(statErr) {
			missing = append(missing, p)
		}
	}

	return missing, nil
}
