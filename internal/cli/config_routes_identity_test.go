package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

func runRoutesTestJSON(t *testing.T, plugins ...config.PluginRouting) (cli.RoutesTestOutput, string) {
	t.Helper()
	cfg := config.New()
	cfg.Routing = &config.RoutingConfig{Plugins: plugins}
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2/instance:Instance", "--output", "json"})
	require.NoError(t, routesCmd.Execute())

	raw := strings.TrimSpace(buf.String())
	var output cli.RoutesTestOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &output))
	return output, raw
}

func runRoutesTestTable(t *testing.T, plugins ...config.PluginRouting) string {
	t.Helper()
	cfg := config.New()
	cfg.Routing = &config.RoutingConfig{Plugins: plugins}
	require.NoError(t, cfg.Save())

	routesCmd, buf := newTestConfigRoutesCmd(t)
	routesCmd.SetArgs([]string{"test", "aws:ec2/instance:Instance"})
	require.NoError(t, routesCmd.Execute())
	return buf.String()
}

func matchPlugins(output cli.RoutesTestOutput) []string {
	names := make([]string, 0, len(output.Matches))
	for _, m := range output.Matches {
		names = append(names, m.Plugin)
	}
	return names
}

func warningFor(output cli.RoutesTestOutput, plugin string) string {
	for _, w := range output.Warnings {
		if w.Plugin == plugin {
			return w.Message
		}
	}
	return ""
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via setupConfigRoutesTest)
func TestConfigRoutesTest_InstalledNameIsSimulated(t *testing.T) {
	setupConfigRoutesTest(t, "aws-public", "cloudzero")

	output, raw := runRoutesTestJSON(t,
		config.PluginRouting{Name: "aws-public", Features: []string{"ProjectedCosts"}},
		config.PluginRouting{Name: "cloudzero", Features: []string{"ActualCosts"}},
	)

	assert.Equal(t, []string{"aws-public", "cloudzero"}, matchPlugins(output))
	assert.Equal(t, "aws-public", output.Features["ProjectedCosts"])
	assert.Equal(t, "cloudzero", output.Features["ActualCosts"])
	assert.Empty(t, output.Warnings)
	assert.NotContains(t, raw, `"warnings"`, "a clean config keeps the JSON shape it had before")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via setupConfigRoutesTest)
func TestConfigRoutesTest_UnknownNameWarns(t *testing.T) {
	setupConfigRoutesTest(t, "aws-public", "acme")
	entries := []config.PluginRouting{
		{Name: "aws-public"},
		{Name: "aws-publik", Features: []string{"ActualCosts"}},
		{Name: "finfocus-plugin-acme"},
	}

	output, _ := runRoutesTestJSON(t, entries...)
	assert.Equal(t, []string{"aws-public"}, matchPlugins(output))
	assert.Contains(t, warningFor(output, "aws-publik"), "does not match an installed plugin; not simulated")
	assert.Contains(t, warningFor(output, "aws-publik"), "finfocus config validate")
	assert.Contains(t, warningFor(output, "finfocus-plugin-acme"), `did you mean "acme"?`)

	table := runRoutesTestTable(t, entries...)
	assert.Contains(t, table, "Warnings:\n")
	assert.Contains(t, table, "  aws-publik: does not match an installed plugin; not simulated")
	assert.Contains(
		t,
		table,
		`  finfocus-plugin-acme: does not match an installed plugin; not simulated (did you mean "acme"?)`,
	)
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via setupConfigRoutesTest)
func TestConfigRoutesTest_CatalogAliasIsSimulated(t *testing.T) {
	setupConfigRoutesTest(t, "aws-public")
	entry := config.PluginRouting{Name: "finfocus-plugin-aws-public", Features: []string{"ProjectedCosts"}}

	output, _ := runRoutesTestJSON(t, entry)
	assert.Equal(t, []string{"aws-public"}, matchPlugins(output))
	assert.Equal(t, "aws-public", output.Features["ProjectedCosts"])
	assert.Equal(t, `"finfocus-plugin-aws-public" will be retired; use "aws-public"`,
		warningFor(output, "finfocus-plugin-aws-public"))

	table := runRoutesTestTable(t, entry)
	assert.Contains(t, table,
		`  finfocus-plugin-aws-public: "finfocus-plugin-aws-public" will be retired; use "aws-public"`)
}
