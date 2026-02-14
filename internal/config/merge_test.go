package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// newDefaultTarget returns a Config with known non-zero defaults so tests can
// verify that absent overlay keys leave the original values intact.
func newDefaultTarget() *config.Config {
	return &config.Config{
		Output: config.OutputConfig{
			DefaultFormat: "table",
			Precision:     2,
		},
		Plugins: map[string]config.PluginConfig{
			"existing": {Config: map[string]interface{}{"key": "val"}},
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Analyzer: config.AnalyzerConfig{
			Plugins: map[string]config.AnalyzerPlugin{
				"default-plugin": {Path: "/usr/bin/plugin", Enabled: true},
			},
		},
		PluginHostConfig: config.PluginHostConfig{
			StrictCompatibility: false,
		},
		Cost: config.CostConfig{
			Cache: config.CacheConfig{
				Enabled:    true,
				TTLSeconds: 3600,
				MaxSizeMB:  100,
			},
		},
	}
}

// writeOverlay is a test helper that writes YAML content to a temp file
// and returns its path.
func writeOverlay(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "overlay.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestShallowMergeYAML_SingleKeyOverride(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
output:
  default_format: json
  precision: 4
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	// Output should be replaced.
	assert.Equal(t, "json", target.Output.DefaultFormat)
	assert.Equal(t, 4, target.Output.Precision)

	// Other sections should be unchanged.
	assert.Equal(t, "info", target.Logging.Level)
	assert.Equal(t, "text", target.Logging.Format)
	assert.True(t, target.Cost.Cache.Enabled)
	assert.Equal(t, 3600, target.Cost.Cache.TTLSeconds)
}

func TestShallowMergeYAML_MultipleKeyOverride(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
output:
  default_format: ndjson
  precision: 6
cost:
  cache:
    enabled: false
    ttl_seconds: 600
    max_size_mb: 50
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.Equal(t, "ndjson", target.Output.DefaultFormat)
	assert.Equal(t, 6, target.Output.Precision)
	assert.False(t, target.Cost.Cache.Enabled)
	assert.Equal(t, 600, target.Cost.Cache.TTLSeconds)
	assert.Equal(t, 50, target.Cost.Cache.MaxSizeMB)
}

func TestShallowMergeYAML_AbsentKeysPreserved(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
output:
  default_format: json
  precision: 8
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	// Logging, Analyzer, PluginHostConfig, Cost, Plugins should all remain at defaults.
	assert.Equal(t, "info", target.Logging.Level)
	assert.Equal(t, "text", target.Logging.Format)
	assert.False(t, target.PluginHostConfig.StrictCompatibility)
	assert.True(t, target.Cost.Cache.Enabled)
	assert.Equal(t, 3600, target.Cost.Cache.TTLSeconds)
	require.Contains(t, target.Plugins, "existing")
	assert.Equal(t, "val", target.Plugins["existing"].Config["key"])
	require.Contains(t, target.Analyzer.Plugins, "default-plugin")
	assert.Equal(t, "/usr/bin/plugin", target.Analyzer.Plugins["default-plugin"].Path)
}

func TestShallowMergeYAML_EmptyOverlayFile(t *testing.T) {
	target := newDefaultTarget()
	original := *target
	overlay := writeOverlay(t, "")

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	// Everything should be unchanged.
	assert.Equal(t, original.Output, target.Output)
	assert.Equal(t, original.Logging, target.Logging)
	assert.Equal(t, original.Cost, target.Cost)
	assert.Equal(t, original.PluginHostConfig, target.PluginHostConfig)
}

func TestShallowMergeYAML_CommentOnlyFile(t *testing.T) {
	target := newDefaultTarget()
	original := *target
	overlay := writeOverlay(t, "# this file is intentionally empty\n# just comments\n")

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.Equal(t, original.Output, target.Output)
	assert.Equal(t, original.Logging, target.Logging)
}

func TestShallowMergeYAML_CorruptedYAMLReturnsError(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, "{{{{not valid yaml at all")

	err := config.ShallowMergeYAML(target, overlay)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing overlay YAML")
}

func TestShallowMergeYAML_MissingFileReturnsError(t *testing.T) {
	target := newDefaultTarget()

	err := config.ShallowMergeYAML(target, "/nonexistent/path/overlay.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading overlay file")
}

func TestShallowMergeYAML_OverrideOutput(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
output:
  default_format: json
  precision: 10
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.Equal(t, "json", target.Output.DefaultFormat)
	assert.Equal(t, 10, target.Output.Precision)
}

func TestShallowMergeYAML_OverridePlugins(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
plugins:
  aws-public:
    region: us-west-2
  vantage:
    api_key: test-key
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	require.Contains(t, target.Plugins, "aws-public")
	assert.Equal(t, "us-west-2", target.Plugins["aws-public"].Config["region"])
	require.Contains(t, target.Plugins, "vantage")
	assert.Equal(t, "test-key", target.Plugins["vantage"].Config["api_key"])
	// The "existing" plugin should be gone because plugins section was fully replaced.
	assert.NotContains(t, target.Plugins, "existing")
}

func TestShallowMergeYAML_OverrideLogging(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
logging:
  level: debug
  format: json
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.Equal(t, "debug", target.Logging.Level)
	assert.Equal(t, "json", target.Logging.Format)
}

func TestShallowMergeYAML_OverrideAnalyzer(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
analyzer:
  timeout:
    per_resource: 10s
    total: 120s
    warn_threshold: 60s
  plugins:
    custom:
      path: /opt/bin/custom
      enabled: true
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	require.Contains(t, target.Analyzer.Plugins, "custom")
	assert.Equal(t, "/opt/bin/custom", target.Analyzer.Plugins["custom"].Path)
	assert.True(t, target.Analyzer.Plugins["custom"].Enabled)
	// The "default-plugin" should be gone since analyzer was fully replaced.
	assert.NotContains(t, target.Analyzer.Plugins, "default-plugin")
}

func TestShallowMergeYAML_OverridePluginHost(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
plugin_host:
  strict_compatibility: true
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.True(t, target.PluginHostConfig.StrictCompatibility)
}

func TestShallowMergeYAML_OverrideCost(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
cost:
  cache:
    enabled: false
    ttl_seconds: 1800
    max_size_mb: 200
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.False(t, target.Cost.Cache.Enabled)
	assert.Equal(t, 1800, target.Cost.Cache.TTLSeconds)
	assert.Equal(t, 200, target.Cost.Cache.MaxSizeMB)
}

func TestShallowMergeYAML_OverrideRouting(t *testing.T) {
	target := newDefaultTarget()
	require.Nil(t, target.Routing, "default target should have nil routing")

	overlay := writeOverlay(t, `
routing:
  plugins:
    - name: aws-public
      priority: 10
    - name: vantage
      priority: 5
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	require.NotNil(t, target.Routing)
	require.Len(t, target.Routing.Plugins, 2)
	assert.Equal(t, "aws-public", target.Routing.Plugins[0].Name)
	assert.Equal(t, 10, target.Routing.Plugins[0].Priority)
	assert.Equal(t, "vantage", target.Routing.Plugins[1].Name)
	assert.Equal(t, 5, target.Routing.Plugins[1].Priority)
}

func TestShallowMergeYAML_PartialCostWithBudgets(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
cost:
  budgets:
    global:
      amount: 5000.00
      currency: USD
      period: monthly
      alerts:
        - threshold: 80
          type: actual
        - threshold: 100
          type: forecasted
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	require.NotNil(t, target.Cost.Budgets)
	require.NotNil(t, target.Cost.Budgets.Global)
	assert.Equal(t, 5000.0, target.Cost.Budgets.Global.Amount)
	assert.Equal(t, "USD", target.Cost.Budgets.Global.Currency)
	assert.Equal(t, "monthly", target.Cost.Budgets.Global.Period)
	require.Len(t, target.Cost.Budgets.Global.Alerts, 2)
	assert.Equal(t, 80.0, target.Cost.Budgets.Global.Alerts[0].Threshold)
	assert.Equal(t, config.AlertTypeActual, target.Cost.Budgets.Global.Alerts[0].Type)
	assert.Equal(t, 100.0, target.Cost.Budgets.Global.Alerts[1].Threshold)
	assert.Equal(t, config.AlertTypeForecasted, target.Cost.Budgets.Global.Alerts[1].Type)
}

func TestShallowMergeYAML_ZeroValueFieldsReplaceDefaults(t *testing.T) {
	target := newDefaultTarget()

	// Verify target has non-zero defaults before merge.
	require.Equal(t, 2, target.Output.Precision)
	require.True(t, target.Cost.Cache.Enabled)
	require.Equal(t, 3600, target.Cost.Cache.TTLSeconds)
	require.Equal(t, 100, target.Cost.Cache.MaxSizeMB)

	overlay := writeOverlay(t, `
output:
  default_format: table
  precision: 0
cost:
  cache:
    enabled: false
    ttl_seconds: 0
    max_size_mb: 0
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	// Zero values from overlay should replace the non-zero defaults.
	assert.Equal(t, 0, target.Output.Precision)
	assert.False(t, target.Cost.Cache.Enabled)
	assert.Equal(t, 0, target.Cost.Cache.TTLSeconds)
	assert.Equal(t, 0, target.Cost.Cache.MaxSizeMB)
}

func TestShallowMergeYAML_UnknownKeysIgnored(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
output:
  default_format: json
  precision: 3
unknown_section:
  foo: bar
extra_key: 42
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	// The known key should be applied.
	assert.Equal(t, "json", target.Output.DefaultFormat)
	assert.Equal(t, 3, target.Output.Precision)

	// Unknown keys should be silently ignored, no error.
	assert.Equal(t, "info", target.Logging.Level)
}

func TestShallowMergeYAML_RoutingWithPatterns(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
routing:
  plugins:
    - name: aws-public
      priority: 10
      patterns:
        - type: glob
          pattern: "aws:*"
      fallback: true
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	require.NotNil(t, target.Routing)
	require.Len(t, target.Routing.Plugins, 1)
	plugin := target.Routing.Plugins[0]
	assert.Equal(t, "aws-public", plugin.Name)
	assert.Equal(t, 10, plugin.Priority)
	require.Len(t, plugin.Patterns, 1)
	assert.Equal(t, "glob", plugin.Patterns[0].Type)
	assert.Equal(t, "aws:*", plugin.Patterns[0].Pattern)
	require.NotNil(t, plugin.Fallback)
	assert.True(t, *plugin.Fallback)
}

func TestShallowMergeYAML_LoggingWithOutputs(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
logging:
  level: warn
  format: json
  outputs:
    - type: console
      level: error
      format: text
  audit:
    enabled: true
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	assert.Equal(t, "warn", target.Logging.Level)
	assert.Equal(t, "json", target.Logging.Format)
	require.Len(t, target.Logging.Outputs, 1)
	assert.Equal(t, "console", target.Logging.Outputs[0].Type)
	assert.Equal(t, "error", target.Logging.Outputs[0].Level)
	assert.True(t, target.Logging.Audit.Enabled)
}

func TestShallowMergeYAML_AnalyzerWithEnv(t *testing.T) {
	target := newDefaultTarget()
	overlay := writeOverlay(t, `
analyzer:
  plugins:
    aws-plugin:
      path: /opt/aws-plugin
      enabled: true
      env:
        AWS_REGION: us-east-1
        API_KEY: secret
`)

	err := config.ShallowMergeYAML(target, overlay)
	require.NoError(t, err)

	require.Contains(t, target.Analyzer.Plugins, "aws-plugin")
	p := target.Analyzer.Plugins["aws-plugin"]
	assert.Equal(t, "/opt/aws-plugin", p.Path)
	assert.True(t, p.Enabled)
	require.Len(t, p.Env, 2)
	assert.Equal(t, "us-east-1", p.Env["AWS_REGION"])
	assert.Equal(t, "secret", p.Env["API_KEY"])
}
