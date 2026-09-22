package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// setHomeDir sets the appropriate home directory environment variable
// for both Unix (HOME) and Windows (USERPROFILE) systems.
// This ensures os.UserHomeDir() works correctly in tests on all platforms.
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}
}

func TestInstalledPlugin(t *testing.T) {
	plugin := InstalledPlugin{
		Name:    "test-plugin",
		URL:     "github.com/owner/repo",
		Version: "v1.0.0",
	}

	if plugin.Name != "test-plugin" {
		t.Errorf("Name = %v, want test-plugin", plugin.Name)
	}
	if plugin.URL != "github.com/owner/repo" {
		t.Errorf("URL = %v, want github.com/owner/repo", plugin.URL)
	}
	if plugin.Version != "v1.0.0" {
		t.Errorf("Version = %v, want v1.0.0", plugin.Version)
	}
}

func TestInstalledPluginsConfig(t *testing.T) {
	cfg := InstalledPluginsConfig{
		InstalledPlugins: []InstalledPlugin{
			{Name: "plugin1", URL: "url1", Version: "v1.0.0"},
			{Name: "plugin2", URL: "url2", Version: "v2.0.0"},
		},
	}

	if len(cfg.InstalledPlugins) != 2 {
		t.Errorf("len(InstalledPlugins) = %d, want 2", len(cfg.InstalledPlugins))
	}
}

func TestLoadInstalledPluginsNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	plugins, err := LoadInstalledPlugins()
	if err != nil {
		t.Errorf("LoadInstalledPlugins() error = %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("LoadInstalledPlugins() = %v, want empty slice", plugins)
	}
}

func TestLoadInstalledPluginsWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".finfocus")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}

	cfg := InstalledPluginsConfig{
		InstalledPlugins: []InstalledPlugin{
			{Name: "test-plugin", URL: "github.com/test", Version: "v1.0.0"},
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadInstalledPlugins()
	if err != nil {
		t.Errorf("LoadInstalledPlugins() error = %v", err)
	}
	// Use require to stop test on failure and prevent panic
	require.Len(t, plugins, 1, "LoadInstalledPlugins() returned wrong number of plugins")
	require.Equal(t, "test-plugin", plugins[0].Name)
}

func TestSaveInstalledPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	plugins := []InstalledPlugin{
		{Name: "plugin1", URL: "url1", Version: "v1.0.0"},
		{Name: "plugin2", URL: "url2", Version: "v2.0.0"},
	}

	err := SaveInstalledPlugins(plugins)
	if err != nil {
		t.Errorf("SaveInstalledPlugins() error = %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ".finfocus", "config.hujson")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load and verify
	loaded, err := LoadInstalledPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d plugins, want 2", len(loaded))
	}
}

func TestAddInstalledPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Add first plugin
	plugin1 := InstalledPlugin{Name: "plugin1", URL: "url1", Version: "v1.0.0"}
	if err := AddInstalledPlugin(plugin1); err != nil {
		t.Errorf("AddInstalledPlugin() error = %v", err)
	}

	// Add second plugin
	plugin2 := InstalledPlugin{Name: "plugin2", URL: "url2", Version: "v2.0.0"}
	if err := AddInstalledPlugin(plugin2); err != nil {
		t.Errorf("AddInstalledPlugin() error = %v", err)
	}

	// Verify both are saved
	plugins, err := LoadInstalledPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 {
		t.Errorf("len(plugins) = %d, want 2", len(plugins))
	}

	// Update existing plugin
	plugin1Updated := InstalledPlugin{Name: "plugin1", URL: "url1", Version: "v1.1.0"}
	if err := AddInstalledPlugin(plugin1Updated); err != nil {
		t.Errorf("AddInstalledPlugin() update error = %v", err)
	}

	// Verify update
	plugins, _ = LoadInstalledPlugins()
	if len(plugins) != 2 {
		t.Errorf("len(plugins) after update = %d, want 2", len(plugins))
	}
	for _, p := range plugins {
		if p.Name == "plugin1" && p.Version != "v1.1.0" {
			t.Errorf("plugin1 version = %v, want v1.1.0", p.Version)
		}
	}
}

func TestRemoveInstalledPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Add plugins
	plugins := []InstalledPlugin{
		{Name: "plugin1", URL: "url1", Version: "v1.0.0"},
		{Name: "plugin2", URL: "url2", Version: "v2.0.0"},
	}
	if err := SaveInstalledPlugins(plugins); err != nil {
		t.Fatal(err)
	}

	// Remove plugin1
	if err := RemoveInstalledPlugin("plugin1"); err != nil {
		t.Errorf("RemoveInstalledPlugin() error = %v", err)
	}

	// Verify only plugin2 remains
	loaded, err := LoadInstalledPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Errorf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].Name != "plugin2" {
		t.Errorf("loaded[0].Name = %v, want plugin2", loaded[0].Name)
	}
}

func TestGetInstalledPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Add plugin
	plugin := InstalledPlugin{Name: "test-plugin", URL: "url", Version: "v1.0.0"}
	if err := AddInstalledPlugin(plugin); err != nil {
		t.Fatal(err)
	}

	// Get existing plugin
	got, err := GetInstalledPlugin("test-plugin")
	if err != nil {
		t.Errorf("GetInstalledPlugin() error = %v", err)
	}
	if got.Name != "test-plugin" {
		t.Errorf("got.Name = %v, want test-plugin", got.Name)
	}

	// Get non-existent plugin
	_, err = GetInstalledPlugin("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestUpdateInstalledPluginVersion(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Add plugin
	plugin := InstalledPlugin{Name: "test-plugin", URL: "url", Version: "v1.0.0"}
	if err := AddInstalledPlugin(plugin); err != nil {
		t.Fatal(err)
	}

	// Update version
	if err := UpdateInstalledPluginVersion("test-plugin", "v2.0.0"); err != nil {
		t.Errorf("UpdateInstalledPluginVersion() error = %v", err)
	}

	// Verify update
	got, err := GetInstalledPlugin("test-plugin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "v2.0.0" {
		t.Errorf("got.Version = %v, want v2.0.0", got.Version)
	}

	// Update non-existent plugin
	err = UpdateInstalledPluginVersion("nonexistent", "v3.0.0")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestGetMissingPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Create plugins directory
	pluginsDir := filepath.Join(tmpDir, ".finfocus", "plugins")
	if err := os.MkdirAll(pluginsDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Add plugins to config
	plugins := []InstalledPlugin{
		{Name: "installed", URL: "url1", Version: "v1.0.0"},
		{Name: "missing", URL: "url2", Version: "v2.0.0"},
	}
	if err := SaveInstalledPlugins(plugins); err != nil {
		t.Fatal(err)
	}

	// Create directory for "installed" plugin
	installedDir := filepath.Join(pluginsDir, "installed", "v1.0.0")
	if err := os.MkdirAll(installedDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Get missing plugins
	missing, err := GetMissingPlugins()
	if err != nil {
		t.Errorf("GetMissingPlugins() error = %v", err)
	}
	if len(missing) != 1 {
		t.Errorf("len(missing) = %d, want 1", len(missing))
	}
	if missing[0].Name != "missing" {
		t.Errorf("missing[0].Name = %v, want missing", missing[0].Name)
	}
}

func TestLoadInstalledPluginsInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	setHomeDir(t, tmpDir)

	// Create config with invalid YAML
	configDir := filepath.Join(tmpDir, ".finfocus")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstalledPlugins()
	require.Error(t, err, "expected error for invalid YAML")
}

func TestSaveInstalledPluginsPreservesConfig(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/config/preserved.hujson")
	require.NoError(t, err)
	tests := []struct {
		name        string
		fileName    string
		data        []byte
		wantErr     bool
		errContains string
	}{
		{
			name: "legacy", fileName: "config.yaml",
			data: []byte("output:\n  default_format: json\n  precision: 4\ncustom:\n  enabled: true\n"),
		},
		{name: "hujson", fileName: "config.hujson", data: fixture},
		{
			name: "corrupted", fileName: "config.hujson", data: []byte("{invalid"),
			wantErr: true, errContains: "failed to load config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("FINFOCUS_HOME", dir)
			path := filepath.Join(dir, tt.fileName)
			data := tt.data
			require.NoError(t, os.WriteFile(path, data, 0600))
			plugins := []InstalledPlugin{{Name: "updated", Version: "v2.0.0"}}
			err = SaveInstalledPlugins(plugins)
			if tt.wantErr {
				require.ErrorContains(t, err, tt.errContains)
				got, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				assert.Equal(t, data, got)
				return
			}
			require.NoError(t, err)
			loaded, err := LoadInstalledPlugins()
			require.NoError(t, err)
			assert.Equal(t, plugins, loaded)
			saved, err := os.ReadFile(filepath.Join(dir, "config.hujson"))
			require.NoError(t, err)
			var values map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(saved, &values))
			assert.JSONEq(t, `{"enabled":true}`, string(values["custom"]))
			assert.JSONEq(t, `{"default_format":"json","precision":4}`, string(values["output"]))
		})
	}
}

func TestGetMissingPluginsDirectoryOverrides(t *testing.T) {
	for _, source := range []string{"default", "config", "environment"} {
		t.Run(source, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("FINFOCUS_HOME", dir)
			t.Setenv("FINFOCUS_PLUGIN_DIR", "")
			cfg := New()
			pluginsDir := cfg.PluginDir
			if source != "default" {
				pluginsDir = t.TempDir()
				cfg.PluginDirOverride = pluginsDir
			}
			if source == "environment" {
				cfg.PluginDirOverride = t.TempDir()
				t.Setenv("FINFOCUS_PLUGIN_DIR", pluginsDir)
			}
			cfg.InstalledPlugins = []InstalledPlugin{{Name: "present", Version: "v1"}, {Name: "missing", Version: "v1"}}
			require.NoError(t, cfg.Save())
			require.NoError(t, os.MkdirAll(filepath.Join(pluginsDir, "present", "v1"), 0700))
			missing, err := GetMissingPlugins()
			require.NoError(t, err)
			assert.Equal(t, cfg.InstalledPlugins[1:], missing)
		})
	}
}
