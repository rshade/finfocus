package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

// setupConfigRoutesTest sets up an isolated test environment for config routes tests.
// It creates a temporary HOME directory, suppresses log output, and resets the
// resolved project directory to ensure "global" source detection.
func setupConfigRoutesTest(t *testing.T) string {
	t.Helper()
	testHome := t.TempDir()
	t.Setenv("HOME", testHome)
	t.Setenv("USERPROFILE", testHome)
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	origProjectDir := config.GetResolvedProjectDir()
	config.SetResolvedProjectDir("")
	t.Cleanup(func() { config.SetResolvedProjectDir(origProjectDir) })

	return testHome
}

// newTestConfigRoutesCmd creates a routes command tree (routes list/test) for executing in tests.
// It returns the routes parent command and a buffer capturing stdout/stderr.
func newTestConfigRoutesCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	routesCmd := cli.NewConfigRoutesCmd()

	var buf bytes.Buffer
	routesCmd.SetOut(&buf)
	routesCmd.SetErr(&buf)

	return routesCmd, &buf
}

func buildStandardRoutingConfig() *config.RoutingConfig {
	falseVal := false
	trueVal := true
	return &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Priority: 10,
				Features: []string{"ProjectedCosts"},
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:ec2:*"},
				},
				Fallback: &falseVal,
			},
			{
				Name:     "aws-ce",
				Priority: 5,
				Features: []string{"ActualCosts", "Recommendations"},
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:*"},
				},
				Fallback: &trueVal,
			},
			{
				Name:     "recorder",
				Priority: 1,
				Fallback: &trueVal,
			},
		},
	}
}

func TestConfigRoutesListTable(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Assert headers
	assert.Contains(t, output, "PRIORITY")
	assert.Contains(t, output, "PLUGIN")
	assert.Contains(t, output, "FEATURES")
	assert.Contains(t, output, "PATTERNS")
	assert.Contains(t, output, "FALLBACK")

	// Assert data values
	assert.Contains(t, output, "aws-public")
	assert.Contains(t, output, "aws-ce")
	assert.Contains(t, output, "recorder")
	assert.Contains(t, output, "ProjectedCosts")
	assert.Contains(t, output, "ActualCosts,Recommendations")
	assert.Contains(t, output, "glob:aws:ec2:*")
	assert.Contains(t, output, "glob:aws:*")
	assert.Contains(t, output, "(all)")

	// Assert priority sort order (descending: 10 before 5 before 1)
	awsPublicIdx := strings.Index(output, "aws-public")
	awsCeIdx := strings.Index(output, "aws-ce")
	recorderIdx := strings.Index(output, "recorder")
	assert.Greater(t, awsCeIdx, awsPublicIdx, "aws-public should appear before aws-ce")
	assert.Greater(t, recorderIdx, awsCeIdx, "aws-ce should appear before recorder")

	// Assert fallback values
	assert.Contains(t, output, "no")
	assert.Contains(t, output, "yes")

	// Assert source line
	assert.Contains(t, output, "Source:")
	assert.Contains(t, output, "(global)")
}

func TestConfigRoutesListAutomatic(t *testing.T) {
	setupConfigRoutesTest(t)

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, strings.ToLower(output), "automatic")
	assert.Contains(t, output, "No routing configured")
}

func TestConfigRoutesListJSON(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list", "--output", "json"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	var output cli.RoutesListOutput
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &output))

	assert.Equal(t, "configured", output.Mode)
	assert.NotEmpty(t, output.ConfigPath)
	assert.Equal(t, "global", output.Source)
	require.Len(t, output.Rules, 3)

	// Assert priority-descending order in JSON output (consistent with table rendering).
	require.Equal(t, "aws-public", output.Rules[0].Plugin, "highest priority plugin first")
	require.Equal(t, 10, output.Rules[0].Priority)
	require.Equal(t, "aws-ce", output.Rules[1].Plugin, "second priority plugin")
	require.Equal(t, 5, output.Rules[1].Priority)
	require.Equal(t, "recorder", output.Rules[2].Plugin, "lowest priority plugin last")
	require.Equal(t, 1, output.Rules[2].Priority)

	assert.Equal(t, []string{"ProjectedCosts"}, output.Rules[0].Features)
	assert.Equal(t, []string{"glob:aws:ec2:*"}, output.Rules[0].Patterns)
}

func TestConfigRoutesListAutomaticJSON(t *testing.T) {
	setupConfigRoutesTest(t)

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list", "--output", "json"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	var output cli.RoutesListOutput
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &output))

	assert.Equal(t, "automatic", output.Mode)
	assert.NotEmpty(t, output.ConfigPath)
	assert.Equal(t, "global", output.Source)
	require.NotNil(t, output.Rules)
	assert.Len(t, output.Rules, 0)
}

func TestConfigRoutesListEmptyPlugins(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = &config.RoutingConfig{Plugins: []config.PluginRouting{}}
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "PRIORITY")
	assert.Contains(t, output, "PLUGIN")
	assert.Contains(t, output, "No plugins configured")
}

func TestConfigRoutesListProjectLocal(t *testing.T) {
	testHome := setupConfigRoutesTest(t)

	// Save a global config with no routing section.
	globalCfg := config.New()
	globalCfg.Routing = nil
	require.NoError(t, globalCfg.Save())

	projectFinfocusDir := filepath.Join(t.TempDir(), ".finfocus")
	projectCfg := config.New()
	projectCfg.SetConfigPath(filepath.Join(projectFinfocusDir, "config.yaml"))
	projectCfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, projectCfg.Save())

	origProjectDir := config.GetResolvedProjectDir()
	config.SetResolvedProjectDir(projectFinfocusDir)
	t.Cleanup(func() { config.SetResolvedProjectDir(origProjectDir) })

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "(project)")
	assert.Contains(t, output, filepath.Join(projectFinfocusDir, "config.yaml"))
	assert.NotContains(t, output, filepath.Join(testHome, ".finfocus", "config.yaml"))
}

func TestConfigRoutesTestTable(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Plugin selection for aws:ec2:Instance")
	assert.Contains(t, output, "provider: aws")
	assert.Contains(t, output, "#")
	assert.Contains(t, output, "PLUGIN")
	assert.Contains(t, output, "PRIORITY")
	assert.Contains(t, output, "MATCH REASON")
	assert.Contains(t, output, "SOURCE")
	assert.Contains(t, output, "aws-public")
	assert.Contains(t, output, "aws-ce")
	assert.Contains(t, output, "recorder")
	assert.Contains(t, output, "Feature availability:")
	assert.Contains(t, output, "ProjectedCosts:")
	assert.Contains(t, output, "ActualCosts:")
	assert.Contains(t, output, "Recommendations:")
	assert.Contains(t, output, "Carbon:")
	assert.Contains(t, output, "DryRun:")
	assert.Contains(t, output, "Budgets:")
}

func TestConfigRoutesTestWithRegion(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance", "us-east-1"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Plugin selection for aws:ec2:Instance")
	assert.Contains(t, output, "region: us-east-1")
	assert.Contains(t, output, "aws-public")
	assert.Contains(t, output, "aws-ce")
}

func TestConfigRoutesTestAutomatic(t *testing.T) {
	setupConfigRoutesTest(t)

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, strings.ToLower(output), "automatic")
	assert.Contains(t, output, "provider")
	assert.Contains(t, output, "would be queried")
}

func TestConfigRoutesTestJSON(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance", "--output", "json"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	var output cli.RoutesTestOutput
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &output))

	assert.Equal(t, "aws:ec2:Instance", output.ResourceType)
	assert.Equal(t, "aws", output.Provider)
	assert.Equal(t, "configured", output.Mode)
	require.NotEmpty(t, output.Matches)
	assert.Equal(t, 1, output.Matches[0].Rank)
	require.NotNil(t, output.Features)
	for _, feature := range []string{
		"ProjectedCosts",
		"ActualCosts",
		"Recommendations",
		"Carbon",
		"DryRun",
		"Budgets",
	} {
		assert.Contains(t, output.Features, feature)
	}
}

func TestConfigRoutesTestNoMatches(t *testing.T) {
	setupConfigRoutesTest(t)

	falseVal := false
	cfg := config.New()
	cfg.Routing = &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Priority: 10,
				Features: []string{"ProjectedCosts"},
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:*"},
				},
				Fallback: &falseVal,
			},
		},
	}
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "gcp:compute:Instance"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No plugins match")
}

func TestConfigRoutesTestMissingArg(t *testing.T) {
	setupConfigRoutesTest(t)

	routesCmd, _ := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test"})

	err := routesCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "resource-type")
}

func TestConfigRoutesTestTooManyArgs(t *testing.T) {
	setupConfigRoutesTest(t)

	routesCmd, _ := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance", "us-east-1", "extra"})

	err := routesCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 2 args")
}

func TestConfigRoutesListJSONContract(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list", "--output", "json"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &payload))

	assert.Contains(t, payload, "mode")
	assert.Contains(t, payload, "config_path")
	assert.Contains(t, payload, "source")
	assert.Contains(t, payload, "rules")

	rules, ok := payload["rules"].([]interface{})
	require.True(t, ok, "rules must be an array")
	require.NotEmpty(t, rules)

	firstRule, ok := rules[0].(map[string]interface{})
	require.True(t, ok, "rule elements must be objects")
	assert.Contains(t, firstRule, "plugin")
	assert.Contains(t, firstRule, "priority")
	assert.Contains(t, firstRule, "features")
	assert.Contains(t, firstRule, "patterns")
	assert.Contains(t, firstRule, "fallback")
	_, featuresIsArray := firstRule["features"].([]interface{})
	assert.True(t, featuresIsArray, "features must be an array")
	_, patternsIsArray := firstRule["patterns"].([]interface{})
	assert.True(t, patternsIsArray, "patterns must be an array")
}

func TestConfigRoutesTestJSONContract(t *testing.T) {
	setupConfigRoutesTest(t)

	cfg := config.New()
	cfg.Routing = buildStandardRoutingConfig()
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance", "--output", "json"})

	err := routesCmd.Execute()
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &payload))

	assert.Contains(t, payload, "resource_type")
	assert.Contains(t, payload, "provider")
	assert.Contains(t, payload, "mode")
	assert.Contains(t, payload, "matches")
	assert.Contains(t, payload, "features")

	matches, ok := payload["matches"].([]interface{})
	require.True(t, ok, "matches must be an array")
	require.NotEmpty(t, matches)
	firstMatch, ok := matches[0].(map[string]interface{})
	require.True(t, ok, "match elements must be objects")
	assert.Contains(t, firstMatch, "rank")
	assert.Contains(t, firstMatch, "plugin")
	assert.Contains(t, firstMatch, "priority")
	assert.Contains(t, firstMatch, "match_reason")
	assert.Contains(t, firstMatch, "source")
	assert.Contains(t, firstMatch, "fallback")

	features, ok := payload["features"].(map[string]interface{})
	require.True(t, ok, "features must be an object")
	for _, feature := range []string{
		"ProjectedCosts",
		"ActualCosts",
		"Recommendations",
		"Carbon",
		"DryRun",
		"Budgets",
	} {
		assert.Contains(t, features, feature)
	}
}

func TestConfigRoutesOutputFormatValidation(t *testing.T) {
	setupConfigRoutesTest(t)

	routesCmd, _ := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"list", "--output", "xml"})
	err := routesCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "supported: table, json")

	routesCmd, _ = newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2:Instance", "--output", "xml"})
	err = routesCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "supported: table, json")
}
