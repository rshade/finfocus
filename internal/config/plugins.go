package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// InstalledPlugin represents an installed plugin entry in config.yaml.
type InstalledPlugin struct {
	Name    string `yaml:"name"    json:"name"`
	URL     string `yaml:"url"     json:"url"`
	Version string `yaml:"version" json:"version"`
}

// InstalledPluginsConfig holds the installed plugins list.
type InstalledPluginsConfig struct {
	InstalledPlugins []InstalledPlugin `yaml:"installed_plugins" json:"installed_plugins"`
}

// pluginsConfigPath returns the full path to the plugins configuration file at
// $HOME/.finfocus/config.yaml. It returns an error if the current user's home
// directory cannot be determined.
func pluginsConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".finfocus", "config.yaml"), nil
}

// LoadInstalledPlugins loads the list of installed plugins from the config file.
// It returns an empty list if the file does not exist, or an error if the YAML cannot be parsed.
func LoadInstalledPlugins() ([]InstalledPlugin, error) {
	configPath, err := pluginsConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []InstalledPlugin{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg InstalledPluginsConfig
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse config: %w", unmarshalErr)
	}

	return cfg.InstalledPlugins, nil
}

// SaveInstalledPlugins saves the provided list of installed plugins into the user's FinFocus config.
// It ensures the config directory exists, preserves other top-level config keys, updates the
// `installed_plugins` entry, and performs an atomic write to the config file.
// The `plugins` parameter is the full list of plugins to persist.
// It returns an error if the config path cannot be determined, if marshaling or file operations fail.
func SaveInstalledPlugins(plugins []InstalledPlugin) error {
	configPath, err := pluginsConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	err = os.MkdirAll(filepath.Dir(configPath), 0700)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load existing config to preserve other settings
	var fullConfig map[string]interface{}
	if existingData, readErr := os.ReadFile(configPath); readErr == nil {
		if unmarshalErr := yaml.Unmarshal(existingData, &fullConfig); unmarshalErr != nil {
			fullConfig = make(map[string]interface{})
		}
	}
	if fullConfig == nil {
		fullConfig = make(map[string]interface{})
	}

	// Update installed_plugins
	fullConfig["installed_plugins"] = plugins

	// Marshal and save
	data, err := yaml.Marshal(fullConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temp file first, then rename (atomic write)
	tmpPath := configPath + ".tmp"
	if writeErr := os.WriteFile(tmpPath, data, 0600); writeErr != nil {
		return fmt.Errorf("failed to write config: %w", writeErr)
	}

	if renameErr := os.Rename(tmpPath, configPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to save config: %w", renameErr)
	}

	return nil
}

// AddInstalledPlugin adds or updates the given plugin in the installed plugins configuration.
// If a plugin with the same Name already exists it is replaced; otherwise the plugin is appended.
// It persists the updated list and returns an error if loading or saving the configuration fails.
func AddInstalledPlugin(plugin InstalledPlugin) error {
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		return err
	}

	// Check if already exists and update
	found := false
	for i, p := range plugins {
		if p.Name == plugin.Name {
			plugins[i] = plugin
			found = true
			break
		}
	}

	if !found {
		plugins = append(plugins, plugin)
	}

	return SaveInstalledPlugins(plugins)
}

// RemoveInstalledPlugin removes the installed plugin with the given name from the configuration.
// It returns an error if loading the current configuration or saving the updated configuration fails.
func RemoveInstalledPlugin(name string) error {
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		return err
	}

	// Find and remove
	var newPlugins []InstalledPlugin
	for _, p := range plugins {
		if p.Name != name {
			newPlugins = append(newPlugins, p)
		}
	}

	return SaveInstalledPlugins(newPlugins)
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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	pluginsDir := filepath.Join(homeDir, ".finfocus", "plugins")

	var missing []InstalledPlugin
	for _, p := range plugins {
		pluginDir := filepath.Join(pluginsDir, p.Name, p.Version)
		if _, statErr := os.Stat(pluginDir); os.IsNotExist(statErr) {
			missing = append(missing, p)
		}
	}

	return missing, nil
}
