package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalConfig(t *testing.T) {
	// Reset global config
	ResetGlobalConfigForTest()

	// Test GetGlobalConfig initializes if needed
	cfg := GetGlobalConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "table", cfg.Output.DefaultFormat)

	// Test that subsequent calls return the same instance
	cfg2 := GetGlobalConfig()
	assert.Same(t, cfg, cfg2)

	// Test ResetGlobalConfigForTest resets the instance
	ResetGlobalConfigForTest()
	cfg3 := GetGlobalConfig()
	assert.NotSame(t, cfg, cfg3)
}

func TestConfigGetters(t *testing.T) {
	// Reset and initialize with test values
	ResetGlobalConfigForTest()
	cfg := GetGlobalConfig()
	cfg.Output.DefaultFormat = "json"
	cfg.Output.Precision = 4
	cfg.Logging.Level = "debug"
	cfg.Logging.File = "/tmp/test.log"
	cfg.SetPluginConfig("test", map[string]interface{}{"key": "value"})

	// Test getter functions
	assert.Equal(t, "json", GetDefaultOutputFormat())
	assert.Equal(t, 4, GetOutputPrecision())
	assert.Equal(t, "debug", GetLogLevel())
	assert.Equal(t, "/tmp/test.log", GetLogFile())

	pluginConfig, err := GetPluginConfiguration("test")
	require.NoError(t, err)
	assert.Equal(t, "value", pluginConfig["key"])

	// Test non-existent plugin
	pluginConfig, err = GetPluginConfiguration("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, pluginConfig)
}

func TestEnsureConfigDir(t *testing.T) {
	// Create a temporary home directory
	tmpHome := t.TempDir()

	// Mock home directory for both Unix and Windows
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

	// Test ensuring config directory
	err := EnsureConfigDir()
	require.NoError(t, err)

	configDir := filepath.Join(tmpHome, ".finfocus")
	stat, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
}

func TestEnsureLogDir(t *testing.T) {
	// Create a temporary directory for logs
	tmpDir := t.TempDir()

	// Reset global config and set custom log file
	ResetGlobalConfigForTest()
	cfg := GetGlobalConfig()
	cfg.Logging.File = filepath.Join(tmpDir, "logs", "subdir", "test.log")

	// Test ensuring log directory
	err := EnsureLogDir()
	require.NoError(t, err)

	logDir := filepath.Join(tmpDir, "logs", "subdir")
	stat, err := os.Stat(logDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
}

func TestEnsureLogDirError(t *testing.T) {
	// Reset global config and set invalid log file path
	ResetGlobalConfigForTest()
	cfg := GetGlobalConfig()

	// Try to create a log directory in a place we don't have permission
	// Use a path that's likely to fail (existing file as directory)
	tmpFile, err := os.CreateTemp("", "test-file")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg.Logging.File = filepath.Join(tmpFile.Name(), "subdir", "test.log")

	// This should fail because tmpFile.Name() is a file, not a directory
	err = EnsureLogDir()
	assert.Error(t, err)
}

func TestGetConfigDir(t *testing.T) {
	stubHome(t)

	dir, err := GetConfigDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)

	// Should be under home directory
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Contains(t, dir, homeDir)
	assert.Contains(t, dir, ".finfocus")
}

func TestGetPluginDir(t *testing.T) {
	stubHome(t)

	dir, err := GetPluginDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)

	// Should be under config directory
	configDir, err := GetConfigDir()
	require.NoError(t, err)
	assert.Contains(t, dir, configDir)
	assert.Contains(t, dir, "plugins")
}

func TestGetSpecDir(t *testing.T) {
	stubHome(t)

	dir, err := GetSpecDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)

	// Should be under config directory
	configDir, err := GetConfigDir()
	require.NoError(t, err)
	assert.Contains(t, dir, configDir)
	assert.Contains(t, dir, "specs")
}

func TestEnsureSubDirs(t *testing.T) {
	stubHome(t)

	// Ensure subdirs should create the necessary directories
	err := EnsureSubDirs()
	require.NoError(t, err)

	// Check that config directory exists
	configDir, err := GetConfigDir()
	require.NoError(t, err)
	stat, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())

	// Check that plugin directory exists
	pluginDir, err := GetPluginDir()
	require.NoError(t, err)
	stat, err = os.Stat(pluginDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())

	// Check that spec directory exists
	specDir, err := GetSpecDir()
	require.NoError(t, err)
	stat, err = os.Stat(specDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
}
