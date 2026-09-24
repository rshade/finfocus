package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// mcpToolsGoldenPath returns internal/cli/testdata/mcp/tools.golden. The
// integration suite compares the live tools/list of both MCP entry points
// against the same file, so the static and live lists are pinned together.
func mcpToolsGoldenPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	return filepath.Join(filepath.Dir(filename), "testdata", "mcp", "tools.golden")
}

type mcpSchemaOutput struct {
	Tools []struct {
		Name string `json:"name"`
	} `json:"tools"`
}

// TestMCPSchemaToolAllowList pins the exact MCP tool allow-list. Any new
// command must make an explicit expose-or-exclude decision: regenerate with
// UPDATE_GOLDEN=1 only when the change to the tool surface is intended.
//
// The static __schema --as=mcp path applies the same interim exclusions as the
// live mcp-server/--mcp path (and drops positional-argument commands, which the
// live server cannot call), so this list is also the live tools/list.
func TestMCPSchemaToolAllowList(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"__schema", "--as=mcp"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)

	var schema mcpSchemaOutput
	require.NoError(t, json.Unmarshal(result.Stdout, &schema))
	require.NotEmpty(t, schema.Tools)
	names := make([]string, 0, len(schema.Tools))
	for _, tool := range schema.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)

	for _, excluded := range []string{
		"finfocus-analyzer-serve", "finfocus-setup", "finfocus-plugin-init",
	} {
		assert.NotContains(t, names, excluded)
	}

	assertGoldenFile(t, mcpToolsGoldenPath(t), strings.Join(names, "\n")+"\n")
}

// TestMCPExclusionsDoNotAffectHelpOrAXSchema verifies the interim exclusions
// only apply while an MCP tool list is built: the commands stay documented in
// --help and in the AX schema.
func TestMCPExclusionsDoNotAffectHelpOrAXSchema(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	root := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, root, []string{"__schema", "--as=mcp"})
	require.Equal(t, 0, result.ExitCode)

	help := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"--help"})
	require.Equal(t, 0, help.ExitCode)
	assert.Contains(t, string(help.Stdout), "setup")

	analyzerHelp := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"analyzer", "--help"})
	require.Equal(t, 0, analyzerHelp.ExitCode)
	assert.Contains(t, string(analyzerHelp.Stdout), "serve")

	for _, path := range [][]string{{"analyzer", "serve"}, {"setup"}, {"plugin", "init"}} {
		cmd, _, err := root.Find(path)
		require.NoError(t, err)
		assert.False(t, cmd.Hidden, "%v must be visible again after __schema --as=mcp", path)
	}

	axSchema := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"__schema"})
	require.Equal(t, 0, axSchema.ExitCode)
	assert.Regexp(t, `"use":\s*"serve"`, string(axSchema.Stdout))
	assert.Regexp(t, `"use":\s*"setup"`, string(axSchema.Stdout))
}

func TestMCPServerHelpShowsFinfocusExamples(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"mcp-server", "--help"})
	require.Equal(t, 0, result.ExitCode)

	out := string(result.Stdout)
	assert.Contains(t, out, "finfocus mcp-server")
	assert.Contains(t, out, "finfocus --mcp")
	assert.NotContains(t, out, "mycli")
}

func TestRootHelpListsMCPFlag(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"--help"})
	require.Equal(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stdout), "--mcp")
}
