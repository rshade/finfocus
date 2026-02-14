package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// writePulumiYAML creates a minimal Pulumi.yaml in the given directory.
func writePulumiYAML(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: test\n"), 0644))
}

func TestResolveProjectDir_FlagOverride(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "") // ensure env is clear

	flagDir := t.TempDir()

	got := config.ResolveProjectDir(flagDir, "/does/not/matter")

	assert.Equal(t, filepath.Join(flagDir, ".finfocus"), got)
	assert.True(t, filepath.IsAbs(got), "returned path must be absolute")
}

func TestResolveProjectDir_FlagOverridesEnv(t *testing.T) {
	envDir := t.TempDir()
	flagDir := t.TempDir()
	t.Setenv("FINFOCUS_PROJECT_DIR", envDir)

	got := config.ResolveProjectDir(flagDir, "/does/not/matter")

	assert.Equal(t, filepath.Join(flagDir, ".finfocus"), got)
}

func TestResolveProjectDir_EnvVarOverride(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("FINFOCUS_PROJECT_DIR", envDir)

	got := config.ResolveProjectDir("", "/does/not/matter")

	assert.Equal(t, filepath.Join(envDir, ".finfocus"), got)
	assert.True(t, filepath.IsAbs(got), "returned path must be absolute")
}

func TestResolveProjectDir_PulumiWalkUp(t *testing.T) {
	root := t.TempDir()
	writePulumiYAML(t, root)

	subDir := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	got := config.ResolveProjectDir("", subDir)

	assert.Equal(t, filepath.Join(root, ".finfocus"), got)
	assert.True(t, filepath.IsAbs(got), "returned path must be absolute")
}

func TestResolveProjectDir_NoProjectFallback(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Use a temp dir with no Pulumi.yaml anywhere in its ancestry.
	emptyDir := t.TempDir()

	got := config.ResolveProjectDir("", emptyDir)

	assert.Empty(t, got, "should return empty string when no project found")
}

func TestResolveProjectDir_DeepNesting(t *testing.T) {
	root := t.TempDir()
	writePulumiYAML(t, root)

	// Build a 25-level-deep directory tree.
	deepDir := root
	for i := range 25 {
		deepDir = filepath.Join(deepDir, "d"+string(rune('a'+i%26)))
	}
	require.NoError(t, os.MkdirAll(deepDir, 0755))

	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	got := config.ResolveProjectDir("", deepDir)

	assert.Equal(t, filepath.Join(root, ".finfocus"), got)
}

func TestResolveProjectDir_FilesystemRootBoundary(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Starting from filesystem root should find no project and return "".
	got := config.ResolveProjectDir("", "/")

	assert.Empty(t, got, "should return empty string when starting from filesystem root")
}

func TestResolveProjectDir_RelativeFlagValue(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	got := config.ResolveProjectDir("relative/path", "/does/not/matter")

	assert.True(t, filepath.IsAbs(got), "returned path must be absolute even for relative flag input")
	assert.Contains(t, got, ".finfocus")
}

func TestResolveProjectDir_FlagWithFinfocusSuffix(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// User passes a path that already ends with .finfocus —
	// should NOT double-append.
	got := config.ResolveProjectDir("/my/project/.finfocus", "")

	assert.Equal(t, "/my/project/.finfocus", got)
	assert.True(t, filepath.IsAbs(got))
}

func TestResolveProjectDir_EnvWithFinfocusSuffix(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "/other/project/.finfocus")

	got := config.ResolveProjectDir("", "")

	assert.Equal(t, "/other/project/.finfocus", got)
	assert.True(t, filepath.IsAbs(got))
}

func TestResolveProjectDir_InvalidFlagPath(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Even a non-existent path should be returned (ResolveProjectDir is read-only,
	// it does not check existence).
	got := config.ResolveProjectDir("/nonexistent/path/to/project", "")

	assert.Equal(t, filepath.Join("/nonexistent/path/to/project", ".finfocus"), got)
	assert.True(t, filepath.IsAbs(got))
}

func TestResolveProjectDir_NestedPulumiProjects(t *testing.T) {
	// Setup: Pulumi.yaml at both /a/ and /a/b/
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "a", "b")
	dirC := filepath.Join(root, "a", "b", "c")

	require.NoError(t, os.MkdirAll(dirC, 0755))
	writePulumiYAML(t, dirA)
	writePulumiYAML(t, dirB)

	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Walk-up from /a/b/c/ should find /a/b/ first (nearest ancestor).
	got := config.ResolveProjectDir("", dirC)

	assert.Equal(t, filepath.Join(dirB, ".finfocus"), got,
		"should find nearest Pulumi.yaml, not the one further up")
}

func TestResolveProjectDir_EmptyInputs(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Both flag and startDir empty: startDir="" will be resolved to cwd by
	// filepath.Abs inside pulumi.FindProject. This should not panic.
	got := config.ResolveProjectDir("", "")

	// The result depends on whether cwd has a Pulumi.yaml ancestor.
	// We just verify it does not panic and returns a valid result.
	if got != "" {
		assert.True(t, filepath.IsAbs(got))
	}
}

func TestSetResolvedProjectDir_RoundTrip(t *testing.T) {
	// Save and restore original value.
	orig := config.GetResolvedProjectDir()
	t.Cleanup(func() { config.SetResolvedProjectDir(orig) })

	config.SetResolvedProjectDir("/some/project/.finfocus")
	assert.Equal(t, "/some/project/.finfocus", config.GetResolvedProjectDir())

	config.SetResolvedProjectDir("")
	assert.Empty(t, config.GetResolvedProjectDir())
}

func TestSetResolvedProjectDir_EmptyString(t *testing.T) {
	orig := config.GetResolvedProjectDir()
	t.Cleanup(func() { config.SetResolvedProjectDir(orig) })

	config.SetResolvedProjectDir("")
	assert.Empty(t, config.GetResolvedProjectDir())
}

func TestNewWithProjectDir_BackwardCompatibility(t *testing.T) {
	t.Run("with_FINFOCUS_HOME", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("FINFOCUS_HOME", tmpHome)
		t.Setenv("PULUMI_HOME", "")

		cfgNew := config.New()
		cfgProject := config.NewWithProjectDir("")

		assert.Equal(t, cfgNew.Output, cfgProject.Output)
		assert.Equal(t, cfgNew.Plugins, cfgProject.Plugins)
		assert.Equal(t, cfgNew.Logging.Level, cfgProject.Logging.Level)
		assert.Equal(t, cfgNew.Logging.Format, cfgProject.Logging.Format)
		assert.Equal(t, cfgNew.Logging.File, cfgProject.Logging.File)
		assert.Equal(t, cfgNew.Analyzer, cfgProject.Analyzer)
		assert.Equal(t, cfgNew.PluginHostConfig, cfgProject.PluginHostConfig)
		assert.Equal(t, cfgNew.Cost, cfgProject.Cost)
		assert.Equal(t, cfgNew.Routing, cfgProject.Routing)
		assert.Equal(t, cfgNew.PluginDir, cfgProject.PluginDir)
		assert.Equal(t, cfgNew.SpecDir, cfgProject.SpecDir)
	})

	t.Run("with_PULUMI_HOME", func(t *testing.T) {
		tmpPulumi := t.TempDir()
		t.Setenv("FINFOCUS_HOME", "")
		t.Setenv("PULUMI_HOME", tmpPulumi)

		cfgNew := config.New()
		cfgProject := config.NewWithProjectDir("")

		assert.Equal(t, cfgNew.Output, cfgProject.Output)
		assert.Equal(t, cfgNew.Plugins, cfgProject.Plugins)
		assert.Equal(t, cfgNew.Logging.Level, cfgProject.Logging.Level)
		assert.Equal(t, cfgNew.Logging.Format, cfgProject.Logging.Format)
		assert.Equal(t, cfgNew.Logging.File, cfgProject.Logging.File)
		assert.Equal(t, cfgNew.Analyzer, cfgProject.Analyzer)
		assert.Equal(t, cfgNew.PluginHostConfig, cfgProject.PluginHostConfig)
		assert.Equal(t, cfgNew.Cost, cfgProject.Cost)
		assert.Equal(t, cfgNew.Routing, cfgProject.Routing)
		assert.Equal(t, cfgNew.PluginDir, cfgProject.PluginDir)
		assert.Equal(t, cfgNew.SpecDir, cfgProject.SpecDir)
	})

	t.Run("with_neither_set", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("FINFOCUS_HOME", "")
		t.Setenv("PULUMI_HOME", "")
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		cfgNew := config.New()
		cfgProject := config.NewWithProjectDir("")

		assert.Equal(t, cfgNew.Output, cfgProject.Output)
		assert.Equal(t, cfgNew.Plugins, cfgProject.Plugins)
		assert.Equal(t, cfgNew.Logging.Level, cfgProject.Logging.Level)
		assert.Equal(t, cfgNew.Logging.Format, cfgProject.Logging.Format)
		assert.Equal(t, cfgNew.Logging.File, cfgProject.Logging.File)
		assert.Equal(t, cfgNew.Analyzer, cfgProject.Analyzer)
		assert.Equal(t, cfgNew.PluginHostConfig, cfgProject.PluginHostConfig)
		assert.Equal(t, cfgNew.Cost, cfgProject.Cost)
		assert.Equal(t, cfgNew.Routing, cfgProject.Routing)
		assert.Equal(t, cfgNew.PluginDir, cfgProject.PluginDir)
		assert.Equal(t, cfgNew.SpecDir, cfgProject.SpecDir)
	})
}

func TestNewWithProjectDir_PreservesGlobalPaths(t *testing.T) {
	// After merging a project config, PluginDir and SpecDir must still
	// point to the global directory, not the project directory.
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)
	t.Setenv("PULUMI_HOME", "")

	// Create a project directory with a config override.
	projectDir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	projectCfg := `cost:
  budgets:
    global:
      amount: 2000
      currency: USD
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.yaml"), []byte(projectCfg), 0644))

	cfg := config.NewWithProjectDir(projectDir)

	require.NotNil(t, cfg)

	// PluginDir and SpecDir should point to the global FINFOCUS_HOME paths.
	assert.Equal(t, filepath.Join(globalDir, "plugins"), cfg.PluginDir,
		"PluginDir must use global config directory, not project directory")
	assert.Equal(t, filepath.Join(globalDir, "specs"), cfg.SpecDir,
		"SpecDir must use global config directory, not project directory")

	// Verify the project config was actually merged.
	require.NotNil(t, cfg.Cost.Budgets)
	require.NotNil(t, cfg.Cost.Budgets.Global)
	assert.Equal(t, float64(2000), cfg.Cost.Budgets.Global.Amount)

	// Also verify via GetPluginDir/GetSpecDir which read from FINFOCUS_HOME.
	pluginDir, err := config.GetPluginDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(globalDir, "plugins"), pluginDir)

	specDir, err := config.GetSpecDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(globalDir, "specs"), specDir)
}

func TestNewWithProjectDir_DualPathScenario(t *testing.T) {
	// FINFOCUS_HOME=/custom AND project dir active.
	// Verify PluginDir uses /custom/plugins while config uses project overlay.
	customHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", customHome)
	t.Setenv("PULUMI_HOME", "")

	// Create global config with output format.
	globalCfg := `output:
  default_format: ndjson
`
	require.NoError(t, os.WriteFile(filepath.Join(customHome, "config.yaml"), []byte(globalCfg), 0644))

	// Create project directory with logging override.
	projectDir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	projectCfg := `logging:
  level: debug
  format: json
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.yaml"), []byte(projectCfg), 0644))

	cfg := config.NewWithProjectDir(projectDir)

	require.NotNil(t, cfg)

	// PluginDir must point to custom home, not project dir.
	assert.Equal(t, filepath.Join(customHome, "plugins"), cfg.PluginDir,
		"PluginDir must use FINFOCUS_HOME-based path")
	assert.Equal(t, filepath.Join(customHome, "specs"), cfg.SpecDir,
		"SpecDir must use FINFOCUS_HOME-based path")

	// Output format from global config.
	assert.Equal(t, "ndjson", cfg.Output.DefaultFormat,
		"output format should come from global config")

	// Logging from project overlay.
	assert.Equal(t, "debug", cfg.Logging.Level,
		"logging level should come from project overlay")
	assert.Equal(t, "json", cfg.Logging.Format,
		"logging format should come from project overlay")
}

// BenchmarkResolveProjectDir_DeepTree verifies that project discovery completes
// in under 100ms for a 50-level-deep directory tree (SC-004).
func BenchmarkResolveProjectDir_DeepTree(b *testing.B) {
	root := b.TempDir()
	require.NoError(b, os.WriteFile(
		filepath.Join(root, "Pulumi.yaml"),
		[]byte("name: bench\n"),
		0644,
	))

	// Build a 50-level-deep directory tree.
	deepDir := root
	for i := range 50 {
		deepDir = filepath.Join(deepDir, "d"+string(rune('a'+i%26)))
	}
	require.NoError(b, os.MkdirAll(deepDir, 0755))

	b.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Warm-up to ensure filesystem caches are populated.
	config.ResolveProjectDir("", deepDir)

	b.ResetTimer()

	start := time.Now()
	for b.Loop() {
		result := config.ResolveProjectDir("", deepDir)
		if result == "" {
			b.Fatal("expected non-empty result")
		}
	}
	elapsed := time.Since(start)

	avgPerOp := elapsed / time.Duration(b.N)
	if avgPerOp > 100*time.Millisecond {
		b.Fatalf("SC-004 violation: average %v per operation exceeds 100ms threshold", avgPerOp)
	}
}

func TestNewWithProjectDir_CorruptedYAML(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project", ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "config.yaml"),
		[]byte("{{{invalid yaml"),
		0o644,
	))

	// Corrupted YAML logs warning and returns global defaults.
	cfg := config.NewWithProjectDir(projectDir)
	assert.NotNil(t, cfg)
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
}

func TestNewWithProjectDir_MissingConfigYAML(t *testing.T) {
	// Project dir exists but has no config.yaml.
	projectDir := filepath.Join(t.TempDir(), "project", ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	cfg := config.NewWithProjectDir(projectDir)
	assert.NotNil(t, cfg)
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
}
