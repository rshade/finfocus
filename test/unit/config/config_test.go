// Package config_test provides unit tests for the configuration management system.
package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
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

// TestGetConfigDir tests retrieval of the configuration directory path.
func TestGetConfigDir(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		expected string
	}{
		{
			name:     "default config directory",
			envVar:   "",
			envValue: "",
			expected: ".finfocus",
		},
		{
			name:     "custom config directory via HOME",
			envVar:   "HOME",
			envValue: "/custom/home",
			expected: "/custom/home/.finfocus",
		},
		{
			name:     "custom config directory via USERPROFILE",
			envVar:   "USERPROFILE",
			envValue: "/custom/user",
			expected: "/custom/user/.finfocus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip USERPROFILE test on non-Windows platforms
			if tt.envVar == "USERPROFILE" && runtime.GOOS != "windows" {
				t.Skip("USERPROFILE is Windows-specific")
			}

			// Set test env var if specified
			if tt.envVar != "" && tt.envValue != "" {
				t.Setenv(tt.envVar, tt.envValue)
			}

			result, err := config.GetConfigDir()
			require.NoError(t, err)

			assert.Contains(t, result, tt.expected)
		})
	}
}

// TestGetPluginDir tests retrieval of the plugin directory path.
func TestGetPluginDir(t *testing.T) {
	t.Run("returns plugin directory path", func(t *testing.T) {
		pluginDir, err := config.GetPluginDir()
		require.NoError(t, err)

		assert.NotEmpty(t, pluginDir)
		assert.Contains(t, pluginDir, ".finfocus")
		assert.Contains(t, pluginDir, "plugins")
	})
}

// TestGetSpecDir tests retrieval of the spec directory path.
func TestGetSpecDir(t *testing.T) {
	t.Run("returns spec directory path", func(t *testing.T) {
		specDir, err := config.GetSpecDir()
		require.NoError(t, err)

		assert.NotEmpty(t, specDir)
		assert.Contains(t, specDir, ".finfocus")
		assert.Contains(t, specDir, "specs")
	})
}

// TestEnsureConfigDir tests creation of the configuration directory.
func TestEnsureConfigDir(t *testing.T) {
	t.Run("creates config directory if it doesn't exist", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".finfocus")

		// Override the config dir for testing
		setHomeDir(t, tempDir)

		// Directory should not exist initially
		_, err := os.Stat(configDir)
		assert.True(t, os.IsNotExist(err))

		// This should create the directory
		err = config.EnsureConfigDir()
		require.NoError(t, err)

		// Directory should now exist
		info, err := os.Stat(configDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("handles existing config directory", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".finfocus")

		// Create the directory first
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		// Override the config dir for testing
		setHomeDir(t, tempDir)

		// This should not error on existing directory
		err = config.EnsureConfigDir()
		require.NoError(t, err)

		// Directory should still exist
		info, err := os.Stat(configDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

// TestEnsureSubDirs tests creation of plugin and spec subdirectories.
func TestEnsureSubDirs(t *testing.T) {
	t.Run("creates plugin and spec subdirectories", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir := t.TempDir()

		// Override the config dir for testing
		setHomeDir(t, tempDir)

		err := config.EnsureSubDirs()
		require.NoError(t, err)

		// Check that subdirectories were created
		pluginDir := filepath.Join(tempDir, ".finfocus", "plugins")
		specDir := filepath.Join(tempDir, ".finfocus", "specs")

		pluginInfo, err := os.Stat(pluginDir)
		require.NoError(t, err)
		assert.True(t, pluginInfo.IsDir())

		specInfo, err := os.Stat(specDir)
		require.NoError(t, err)
		assert.True(t, specInfo.IsDir())
	})
}

// TestConfigPaths tests all configuration path retrieval functions.
func TestConfigPaths(t *testing.T) {
	t.Run("all config paths are accessible", func(t *testing.T) {
		configDir, err := config.GetConfigDir()
		require.NoError(t, err)
		pluginDir, err := config.GetPluginDir()
		require.NoError(t, err)
		specDir, err := config.GetSpecDir()
		require.NoError(t, err)

		// All paths should be non-empty
		assert.NotEmpty(t, configDir)
		assert.NotEmpty(t, pluginDir)
		assert.NotEmpty(t, specDir)

		// Plugin and spec dirs should be under config dir
		assert.Contains(t, pluginDir, ".finfocus")
		assert.Contains(t, specDir, ".finfocus")

		// Paths should be absolute or relative
		assert.True(t, filepath.IsAbs(configDir) || !filepath.IsAbs(configDir))
	})
}

// TestConfigPermissions tests that created directories have correct permissions.
func TestConfigPermissions(t *testing.T) {
	t.Run("config directories have correct permissions", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir := t.TempDir()

		// Override the config dir for testing
		setHomeDir(t, tempDir)

		err := config.EnsureConfigDir()
		require.NoError(t, err)

		err = config.EnsureSubDirs()
		require.NoError(t, err)

		// Check permissions on created directories
		configDir := filepath.Join(tempDir, ".finfocus")
		pluginDir := filepath.Join(tempDir, ".finfocus", "plugins")
		specDir := filepath.Join(tempDir, ".finfocus", "specs")

		for _, dir := range []string{configDir, pluginDir, specDir} {
			info, statErr := os.Stat(dir)
			require.NoError(t, statErr)

			// Should be a directory
			assert.True(t, info.IsDir())

			// Should have reasonable permissions (readable and writable by owner)
			mode := info.Mode().Perm()
			assert.NotEqual(t, 0, mode&0700, "Directory should be readable/writable by owner: %s", dir)
		}
	})
}
