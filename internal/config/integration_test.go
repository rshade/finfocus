package config

import (
	"context"
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

func TestInitGlobalConfigWithProject(t *testing.T) {
	ctx := context.Background()

	t.Run("project_config_overrides_global_budget", func(t *testing.T) {
		ResetGlobalConfigForTest()
		t.Cleanup(func() { ResetGlobalConfigForTest() })

		// Set up isolated global config directory via FINFOCUS_HOME.
		globalDir := t.TempDir()
		t.Setenv("FINFOCUS_HOME", globalDir)
		t.Setenv("PULUMI_HOME", "")

		globalCfg := `cost:
  budgets:
    global:
      amount: 10000
      currency: USD
`
		require.NoError(t, os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0o644))

		// Set up project directory with a budget override.
		projectDir := filepath.Join(t.TempDir(), ".finfocus")
		require.NoError(t, os.MkdirAll(projectDir, 0o755))
		projectCfg := `cost:
  budgets:
    global:
      amount: 5000
      currency: USD
`
		require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.yaml"), []byte(projectCfg), 0o644))

		InitGlobalConfigWithProject(ctx, projectDir)
		cfg := GetGlobalConfig()

		require.NotNil(t, cfg)
		require.NotNil(t, cfg.Cost.Budgets)
		require.NotNil(t, cfg.Cost.Budgets.Global)
		assert.Equal(t, float64(5000), cfg.Cost.Budgets.Global.Amount,
			"project budget should override global budget")
	})

	t.Run("project_config_inherits_output_format_from_global", func(t *testing.T) {
		ResetGlobalConfigForTest()
		t.Cleanup(func() { ResetGlobalConfigForTest() })

		globalDir := t.TempDir()
		t.Setenv("FINFOCUS_HOME", globalDir)
		t.Setenv("PULUMI_HOME", "")

		// Global sets output format to json.
		globalCfg := `output:
  default_format: json
  precision: 4
`
		require.NoError(t, os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0o644))

		// Project only overrides budget, not output.
		projectDir := filepath.Join(t.TempDir(), ".finfocus")
		require.NoError(t, os.MkdirAll(projectDir, 0o755))
		projectCfg := `cost:
  budgets:
    global:
      amount: 5000
      currency: USD
`
		require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.yaml"), []byte(projectCfg), 0o644))

		InitGlobalConfigWithProject(ctx, projectDir)
		cfg := GetGlobalConfig()

		require.NotNil(t, cfg)
		assert.Equal(t, "json", cfg.Output.DefaultFormat,
			"output format should be inherited from global config")
		assert.Equal(t, 4, cfg.Output.Precision,
			"output precision should be inherited from global config")
		require.NotNil(t, cfg.Cost.Budgets)
		require.NotNil(t, cfg.Cost.Budgets.Global)
		assert.Equal(t, float64(5000), cfg.Cost.Budgets.Global.Amount,
			"budget should come from project config")
	})

	t.Run("empty_project_dir_produces_same_as_InitGlobalConfig", func(t *testing.T) {
		ResetGlobalConfigForTest()
		t.Cleanup(func() { ResetGlobalConfigForTest() })

		tmpHome := t.TempDir()
		t.Setenv("FINFOCUS_HOME", tmpHome)
		t.Setenv("PULUMI_HOME", "")

		// Initialize with empty project dir.
		InitGlobalConfigWithProject(ctx, "")
		cfgWithEmpty := GetGlobalConfig()
		require.NotNil(t, cfgWithEmpty)

		// Create a fresh New() for comparison.
		cfgNew := New()
		require.NotNil(t, cfgNew)

		assert.Equal(t, cfgNew.Output, cfgWithEmpty.Output)
		assert.Equal(t, cfgNew.Logging.Level, cfgWithEmpty.Logging.Level)
		assert.Equal(t, cfgNew.Logging.Format, cfgWithEmpty.Logging.Format)
		assert.Equal(t, cfgNew.Cost, cfgWithEmpty.Cost)
		assert.Equal(t, cfgNew.PluginHostConfig, cfgWithEmpty.PluginHostConfig)
		assert.Equal(t, cfgNew.PluginDir, cfgWithEmpty.PluginDir)
		assert.Equal(t, cfgNew.SpecDir, cfgWithEmpty.SpecDir)
	})

	t.Run("two_projects_load_independent_configs", func(t *testing.T) {
		// Project A: budget 3000.
		projectDirA := filepath.Join(t.TempDir(), ".finfocus")
		require.NoError(t, os.MkdirAll(projectDirA, 0o755))
		projectCfgA := `cost:
  budgets:
    global:
      amount: 3000
      currency: USD
`
		require.NoError(t, os.WriteFile(filepath.Join(projectDirA, "config.yaml"), []byte(projectCfgA), 0o644))

		// Project B: budget 7000.
		projectDirB := filepath.Join(t.TempDir(), ".finfocus")
		require.NoError(t, os.MkdirAll(projectDirB, 0o755))
		projectCfgB := `cost:
  budgets:
    global:
      amount: 7000
      currency: USD
`
		require.NoError(t, os.WriteFile(filepath.Join(projectDirB, "config.yaml"), []byte(projectCfgB), 0o644))

		globalDir := t.TempDir()
		t.Setenv("FINFOCUS_HOME", globalDir)
		t.Setenv("PULUMI_HOME", "")

		// Init with project A and verify.
		ResetGlobalConfigForTest()
		InitGlobalConfigWithProject(ctx, projectDirA)
		cfgA := GetGlobalConfig()
		require.NotNil(t, cfgA)
		require.NotNil(t, cfgA.Cost.Budgets)
		require.NotNil(t, cfgA.Cost.Budgets.Global)
		assert.Equal(t, float64(3000), cfgA.Cost.Budgets.Global.Amount,
			"project A budget should be 3000")

		// Reset and init with project B and verify.
		ResetGlobalConfigForTest()
		InitGlobalConfigWithProject(ctx, projectDirB)
		cfgB := GetGlobalConfig()
		require.NotNil(t, cfgB)
		require.NotNil(t, cfgB.Cost.Budgets)
		require.NotNil(t, cfgB.Cost.Budgets.Global)
		assert.Equal(t, float64(7000), cfgB.Cost.Budgets.Global.Amount,
			"project B budget should be 7000")

		// Cleanup.
		ResetGlobalConfigForTest()
	})
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
