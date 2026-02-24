//go:build integration
// +build integration

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// TestConfig_WalkUpDiscovery verifies that ResolveProjectDir walks up from
// a deeply nested subdirectory to find the nearest Pulumi.yaml ancestor and
// returns the corresponding project/.finfocus/ directory path.
func TestConfig_WalkUpDiscovery(t *testing.T) {
	// Clear env vars that would short-circuit discovery
	t.Setenv("FINFOCUS_PROJECT_DIR", "")

	// Create directory tree:
	//   project/
	//     Pulumi.yaml
	//     .finfocus/
	//       config.yaml  (with budget limit)
	//     subdir/
	//       nested/      ← start directory for walk-up
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	finfocusDir := filepath.Join(projectDir, ".finfocus")
	nestedDir := filepath.Join(projectDir, "subdir", "nested")

	require.NoError(t, os.MkdirAll(finfocusDir, 0o755))
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	// Create Pulumi.yaml project marker
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "Pulumi.yaml"),
		[]byte("name: walk-up-test\nruntime: go\n"),
		0o644,
	))

	// Create project config with budget limit
	configYAML := `cost:
  budgets:
    global:
      amount: 100
      currency: USD
`
	require.NoError(t, os.WriteFile(
		filepath.Join(finfocusDir, "config.yaml"),
		[]byte(configYAML),
		0o644,
	))

	// Resolve project dir starting from the nested subdirectory
	ctx := context.Background()
	got := config.ResolveProjectDir(ctx, "", nestedDir)

	// Should walk up from subdir/nested → subdir → project (finds Pulumi.yaml)
	// and return project/.finfocus/
	require.NotEmpty(t, got, "ResolveProjectDir should find the project")
	assert.Equal(t, finfocusDir, got,
		"should resolve to the .finfocus directory next to Pulumi.yaml")
	assert.True(t, filepath.IsAbs(got), "returned path must be absolute")

	// Verify the resolved directory actually contains the config file
	configPath := filepath.Join(got, "config.yaml")
	_, err := os.Stat(configPath)
	require.NoError(t, err, "resolved directory should contain config.yaml")

	// Verify the config can be loaded from the resolved directory
	cfg := config.NewWithProjectDir(ctx, got)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Cost.Budgets, "budget config should be loaded from project config")
	require.NotNil(t, cfg.Cost.Budgets.Global)
	assert.Equal(t, float64(100), cfg.Cost.Budgets.Global.Amount,
		"budget limit should be 100 from project config")
}

// TestConfig_ProjectOverridesGlobal verifies that project-local config values
// override global config at the top-level key level (shallow merge) while
// global keys not present in the project config are inherited.
func TestConfig_ProjectOverridesGlobal(t *testing.T) {
	// Set up isolated global config directory
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)
	t.Setenv("PULUMI_HOME", "")

	// Reset global config singleton so it picks up our test env
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	// Create global config: output=table, logging=info, plugins with a key
	globalCfg := `output:
  default_format: table
  precision: 4
logging:
  level: info
  format: text
plugins:
  mock-plugin:
    api_key: global-key
`
	require.NoError(t, os.WriteFile(
		filepath.Join(globalDir, "config.yaml"),
		[]byte(globalCfg),
		0o644,
	))

	// Create project directory with config that overrides output format only
	projectFinfocusDir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(projectFinfocusDir, 0o755))

	projectCfg := `output:
  default_format: json
  precision: 2
`
	require.NoError(t, os.WriteFile(
		filepath.Join(projectFinfocusDir, "config.yaml"),
		[]byte(projectCfg),
		0o644,
	))

	// Load config with project overlay
	ctx := context.Background()
	cfg := config.NewWithProjectDir(ctx, projectFinfocusDir)
	require.NotNil(t, cfg)

	// output section should be fully replaced by project config (shallow merge)
	assert.Equal(t, "json", cfg.Output.DefaultFormat,
		"output format should be 'json' from project config (project wins)")
	assert.Equal(t, 2, cfg.Output.Precision,
		"precision should be 2 from project config (entire output section replaced)")

	// logging section should be inherited from global (not in project config)
	assert.Equal(t, "info", cfg.Logging.Level,
		"logging level should be 'info' from global config (inherited)")
	assert.Equal(t, "text", cfg.Logging.Format,
		"logging format should be 'text' from global config (inherited)")

	// plugins section should be inherited from global (not in project config)
	pluginCfg, err := cfg.GetPluginConfig("mock-plugin")
	require.NoError(t, err)
	assert.Equal(t, "global-key", pluginCfg["api_key"],
		"plugin config should be inherited from global config")
}

// TestConfig_FlagOverridesEnv verifies that the --project-dir flag value
// takes precedence over the FINFOCUS_PROJECT_DIR environment variable
// when resolving the project configuration directory.
func TestConfig_FlagOverridesEnv(t *testing.T) {
	// Create two separate project trees with distinguishable configs
	flagRoot := filepath.Join(t.TempDir(), "flag-project")
	flagFinfocus := filepath.Join(flagRoot, ".finfocus")
	require.NoError(t, os.MkdirAll(flagFinfocus, 0o755))

	flagCfg := `output:
  default_format: json
  precision: 6
`
	require.NoError(t, os.WriteFile(
		filepath.Join(flagFinfocus, "config.yaml"),
		[]byte(flagCfg),
		0o644,
	))

	envRoot := filepath.Join(t.TempDir(), "env-project")
	envFinfocus := filepath.Join(envRoot, ".finfocus")
	require.NoError(t, os.MkdirAll(envFinfocus, 0o755))

	envCfg := `output:
  default_format: table
  precision: 2
`
	require.NoError(t, os.WriteFile(
		filepath.Join(envFinfocus, "config.yaml"),
		[]byte(envCfg),
		0o644,
	))

	// Set env var to env project path
	t.Setenv("FINFOCUS_PROJECT_DIR", envRoot)

	ctx := context.Background()

	// Call ResolveProjectDir with flag path — flag should win over env
	got := config.ResolveProjectDir(ctx, flagRoot, "")

	// toAbsFinfocusDir appends .finfocus when base is not ".finfocus"
	assert.Equal(t, flagFinfocus, got,
		"flag path should take precedence over FINFOCUS_PROJECT_DIR env var")

	// Verify loading config from the resolved path yields the flag config values
	cfg := config.NewWithProjectDir(ctx, got)
	require.NotNil(t, cfg)
	assert.Equal(t, "json", cfg.Output.DefaultFormat,
		"should load config from flag path (json), not env path (table)")
	assert.Equal(t, 6, cfg.Output.Precision,
		"should load precision from flag path (6), not env path (2)")
}

// TestConfig_MalformedYAMLReturnsDescriptiveError verifies that malformed YAML
// config files produce descriptive errors (not panics) and that
// NewWithProjectDir degrades gracefully to defaults.
func TestConfig_MalformedYAMLReturnsDescriptiveError(t *testing.T) {
	t.Run("MalformedYAML", func(t *testing.T) {
		// Create a project dir with syntactically invalid YAML
		projectDir := filepath.Join(t.TempDir(), ".finfocus")
		require.NoError(t, os.MkdirAll(projectDir, 0o755))

		malformedYAML := `"{{invalid yaml: [`
		require.NoError(t, os.WriteFile(
			filepath.Join(projectDir, "config.yaml"),
			[]byte(malformedYAML),
			0o644,
		))

		// ShallowMergeYAML should return a descriptive error
		target := config.New()
		err := config.ShallowMergeYAML(target, filepath.Join(projectDir, "config.yaml"))
		require.Error(t, err, "malformed YAML should produce an error")

		errMsg := err.Error()
		yamlRelated := containsAny(errMsg, "yaml", "unmarshal", "parse")
		assert.True(t, yamlRelated,
			"error should mention yaml/unmarshal/parse, got: %s", errMsg)

		// NewWithProjectDir should degrade gracefully (no panic, returns defaults)
		ctx := context.Background()
		cfg := config.NewWithProjectDir(ctx, projectDir)
		require.NotNil(t, cfg, "should return a valid config even on malformed YAML")
		assert.Equal(t, "table", cfg.Output.DefaultFormat,
			"should fall back to default format when project config is malformed")
	})

	t.Run("SemanticallyInvalidValues", func(t *testing.T) {
		// Valid YAML syntax but semantically wrong types for config fields
		projectDir := filepath.Join(t.TempDir(), ".finfocus")
		require.NoError(t, os.MkdirAll(projectDir, 0o755))

		// output.precision expects an int; providing a string should fail unmarshal
		badSemantics := `output:
  default_format: nonexistent
  precision: not-a-number
`
		require.NoError(t, os.WriteFile(
			filepath.Join(projectDir, "config.yaml"),
			[]byte(badSemantics),
			0o644,
		))

		// ShallowMergeYAML should return an error for type mismatch
		target := config.New()
		err := config.ShallowMergeYAML(target, filepath.Join(projectDir, "config.yaml"))
		require.Error(t, err,
			"semantically invalid values (string for int field) should produce an error")

		// NewWithProjectDir should degrade gracefully
		ctx := context.Background()
		cfg := config.NewWithProjectDir(ctx, projectDir)
		require.NotNil(t, cfg, "should return a valid config on semantic errors")
		// Should fall back to defaults since the merge failed
		assert.Equal(t, "table", cfg.Output.DefaultFormat,
			"should fall back to default format on semantic errors")
	})
}

// TestConfig_EnsureGitignoreIdempotent verifies that EnsureGitignore creates a
// .gitignore file on the first call and is a no-op on subsequent calls,
// preserving the original content without modification.
func TestConfig_EnsureGitignoreIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	gitignorePath := filepath.Join(dir, ".gitignore")

	// First call should create the file
	created, err := config.EnsureGitignore(dir)
	require.NoError(t, err)
	assert.True(t, created, "first call should create the .gitignore file")

	// Verify the file exists with expected content
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, config.GitignoreContent(), string(content),
		".gitignore content should match GitignoreContent()")

	// Record modification time
	info1, err := os.Stat(gitignorePath)
	require.NoError(t, err)
	modTime1 := info1.ModTime()

	// Small delay to ensure filesystem timestamp would differ if file were rewritten
	time.Sleep(50 * time.Millisecond)

	// Second call should be a no-op
	created2, err := config.EnsureGitignore(dir)
	require.NoError(t, err)
	assert.False(t, created2, "second call should not create a new file")

	// Verify content is unchanged
	content2, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, string(content), string(content2),
		"content should not change on second call")

	// Verify modification time is unchanged (file was not rewritten)
	info2, err := os.Stat(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, modTime1, info2.ModTime(),
		"modification time should not change on second call (no-op)")
}

// TestConfig_ShallowMergeReplacement verifies that ShallowMergeYAML replaces
// entire top-level sections present in the overlay, inherits sections absent
// from the overlay, and adds new keys from the overlay.
func TestConfig_ShallowMergeReplacement(t *testing.T) {
	// Set up isolated global config so New() loads known defaults
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)
	t.Setenv("PULUMI_HOME", "")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	// Create global config with output, plugins, and logging sections
	globalCfg := `output:
  default_format: table
  precision: 4
plugins:
  test-plugin:
    api_key: test-key-123
logging:
  level: info
  format: text
`
	require.NoError(t, os.WriteFile(
		filepath.Join(globalDir, "config.yaml"),
		[]byte(globalCfg),
		0o644,
	))

	// Load the target config (picks up global values)
	target := config.New()
	require.Equal(t, "table", target.Output.DefaultFormat)
	require.Equal(t, 4, target.Output.Precision)
	require.Equal(t, "info", target.Logging.Level)

	// Create overlay file with output (replace) and analyzer (new key)
	overlayDir := t.TempDir()
	overlayPath := filepath.Join(overlayDir, "overlay.yaml")
	overlayYAML := `output:
  default_format: ndjson
  precision: 1
analyzer:
  enforcement: advisory
  timeout:
    per_resource: 3s
`
	require.NoError(t, os.WriteFile(overlayPath, []byte(overlayYAML), 0o644))

	// Apply shallow merge
	err := config.ShallowMergeYAML(target, overlayPath)
	require.NoError(t, err)

	// output should be fully replaced by overlay
	assert.Equal(t, "ndjson", target.Output.DefaultFormat,
		"output.default_format should be replaced by overlay")
	assert.Equal(t, 1, target.Output.Precision,
		"output.precision should be replaced by overlay")

	// plugins should be inherited from target (not in overlay)
	pluginCfg, err := target.GetPluginConfig("test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "test-key-123", pluginCfg["api_key"],
		"plugins should be inherited from target (not in overlay)")

	// logging should be inherited from target (not in overlay)
	assert.Equal(t, "info", target.Logging.Level,
		"logging.level should be inherited from target")
	assert.Equal(t, "text", target.Logging.Format,
		"logging.format should be inherited from target")

	// analyzer should be added from overlay (new key)
	assert.Equal(t, "advisory", target.Analyzer.Enforcement,
		"analyzer should be added from overlay")
	assert.Equal(t, 3*time.Second, time.Duration(target.Analyzer.Timeout.PerResource),
		"analyzer.timeout.per_resource should be set from overlay")
}

// TestConfig_DismissedJsonProjectLocalPrecedence verifies that project-local
// dismissed.json takes precedence over global dismissed.json when loading
// dismissal state, and that recommendations present only in the global file
// are still accessible via a separate store instance.
func TestConfig_DismissedJsonProjectLocalPrecedence(t *testing.T) {
	// Set up isolated global config dir
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)
	t.Setenv("PULUMI_HOME", "")

	// Create project-local dir
	projectDir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Write project-local dismissed.json: rec-001 = dismissed
	projectDismissals := map[string]interface{}{
		"version": 1,
		"dismissals": map[string]interface{}{
			"rec-001": map[string]interface{}{
				"recommendation_id": "rec-001",
				"status":            "dismissed",
				"reason":            "cost_acceptable",
				"dismissed_at":      time.Now().Format(time.RFC3339),
				"expires_at":        nil,
				"history":           []interface{}{},
			},
		},
	}
	projectJSON, err := json.MarshalIndent(projectDismissals, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "dismissed.json"),
		projectJSON,
		0o644,
	))

	// Write global dismissed.json: rec-001 = active, rec-002 = dismissed
	globalDismissals := map[string]interface{}{
		"version": 1,
		"dismissals": map[string]interface{}{
			"rec-001": map[string]interface{}{
				"recommendation_id": "rec-001",
				"status":            "active",
				"reason":            "re-enabled",
				"dismissed_at":      time.Now().Format(time.RFC3339),
				"expires_at":        nil,
				"history":           []interface{}{},
			},
			"rec-002": map[string]interface{}{
				"recommendation_id": "rec-002",
				"status":            "dismissed",
				"reason":            "not_applicable",
				"dismissed_at":      time.Now().Format(time.RFC3339),
				"expires_at":        nil,
				"history":           []interface{}{},
			},
		},
	}
	globalJSON, err := json.MarshalIndent(globalDismissals, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(globalDir, "dismissed.json"),
		globalJSON,
		0o644,
	))

	// Set the resolved project dir so NewDismissalStore("") uses project-local
	config.SetResolvedProjectDir(projectDir)
	t.Cleanup(func() { config.SetResolvedProjectDir("") })

	// Load project-local dismissal store (empty path triggers project resolution)
	projectStore, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, projectStore.Load())

	// rec-001 should be "dismissed" from project-local (not "active" from global)
	rec001, found := projectStore.Get("rec-001")
	require.True(t, found, "rec-001 should exist in project-local store")
	assert.Equal(t, config.StatusDismissed, rec001.Status,
		"rec-001 should be 'dismissed' from project-local, not 'active' from global")

	// rec-002 should NOT be in project-local store (it's only in global)
	_, found = projectStore.Get("rec-002")
	assert.False(t, found, "rec-002 should not exist in project-local store")

	// Load global store directly via explicit path to verify rec-002 is accessible
	globalStore, err := config.NewDismissalStore(filepath.Join(globalDir, "dismissed.json"))
	require.NoError(t, err)
	require.NoError(t, globalStore.Load())

	rec002, found := globalStore.Get("rec-002")
	require.True(t, found, "rec-002 should exist in global store")
	assert.Equal(t, config.StatusDismissed, rec002.Status,
		"rec-002 should be 'dismissed' in global store")

	// Verify global store has rec-001 as "active" (different from project-local)
	globalRec001, found := globalStore.Get("rec-001")
	require.True(t, found, "rec-001 should exist in global store")
	assert.Equal(t, config.StatusActive, globalRec001.Status,
		"rec-001 should be 'active' in global store (project-local overrode it)")
}

// containsAny checks if s contains any of the given substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
