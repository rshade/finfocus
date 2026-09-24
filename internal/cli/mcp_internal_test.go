package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubMCPServe replaces mcpServeFunc for the duration of the test.
func stubMCPServe(t *testing.T, serve func(ctx context.Context, root *cobra.Command, ver string) error) {
	t.Helper()
	original := mcpServeFunc
	mcpServeFunc = serve
	t.Cleanup(func() { mcpServeFunc = original })
}

// TestMCPFlagServesInsteadOfOverview verifies that root --mcp routes to the MCP
// serve function with the exclusions applied, even inside a Pulumi project where
// the bare root command would otherwise delegate to the overview dashboard.
func TestMCPFlagServesInsteadOfOverview(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "Pulumi.yaml"),
		[]byte("name: test\nruntime: go\n"), 0o600))
	t.Chdir(projectDir)

	var (
		calls         int
		servedVersion string
		hiddenWhile   map[string]bool
	)
	stubMCPServe(t, func(_ context.Context, root *cobra.Command, ver string) error {
		calls++
		servedVersion = ver
		hiddenWhile = map[string]bool{}
		for _, exclusion := range mcpExcludedCommands {
			cmd := findSubcommand(root, exclusion.path)
			require.NotNil(t, cmd, "excluded command %v must exist", exclusion.path)
			hiddenWhile[cmd.CommandPath()] = cmd.Hidden
		}
		return nil
	})

	root := NewRootCmd("v1.2.3")
	result := axtest.Run(context.Background(), t, root, []string{"--mcp"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)

	assert.Equal(t, 1, calls, "--mcp must call the serve function exactly once")
	assert.Equal(t, "v1.2.3", servedVersion)
	assert.Empty(t, result.Stdout, "--mcp must not render the overview")
	for path, hidden := range hiddenWhile {
		assert.True(t, hidden, "%s must be hidden while serving", path)
	}
	for _, exclusion := range mcpExcludedCommands {
		assert.False(t, findSubcommand(root, exclusion.path).Hidden,
			"%v must be visible again after serving", exclusion.path)
	}
}

// TestMCPDispatchedCallsShareLoggingSession simulates the MCP dispatcher, which
// re-executes the shared root once per tools/call while the server runs. Calls
// must reuse the server's logging session (no per-call "Logging to:" line), the
// root command must refuse to run as a tool (so --mcp can never recurse), and
// the server must shut down without a double-close error.
func TestMCPDispatchedCallsShareLoggingSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	logDir := filepath.Join(home, "logs")
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(`{"logging": {
		"level": "info",
		"file": "`+filepath.Join(logDir, "finfocus.log")+`",
		"audit": {"enabled": true, "file": "`+filepath.Join(logDir, "audit.log")+`"}
	}}`), 0o600))

	var dispatched []dispatchedCall
	stubMCPServe(t, func(ctx context.Context, root *cobra.Command, _ string) error {
		for _, args := range [][]string{
			{"config", "list", "--format=json"},
			{"config", "validate", "--format=json"},
			{"--mcp"},
		} {
			dispatched = append(dispatched, executeNested(ctx, root, args))
		}
		return nil
	})

	result := axtest.Run(context.Background(), t, NewRootCmd("v1.2.3"), []string{"--mcp"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.NotContains(t, string(result.Stderr), "already closed")
	assert.NotContains(t, string(result.Stderr), "internal_error")

	require.Len(t, dispatched, 3)
	for _, call := range dispatched[:2] {
		require.NoError(t, call.err)
		assert.True(t, json.Valid([]byte(call.stdout)), "payload must be JSON: %s", call.stdout)
		assert.NotContains(t, call.stderr, "Logging to:")
	}
	require.Error(t, dispatched[2].err, "the root command must not run inside a tool call")
	assert.Contains(t, dispatched[2].err.Error(), "not callable as an MCP tool")
}

type dispatchedCall struct {
	stdout string
	stderr string
	err    error
}

// executeNested runs args against root the way ax-go's dispatcher does for a
// tools/call: a nested ExecuteContext on the shared tree with per-call buffers.
func executeNested(ctx context.Context, root *cobra.Command, args []string) dispatchedCall {
	var stdout, stderr bytes.Buffer
	root.SetArgs(args)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.ExecuteContext(ctx)
	root.SetArgs(nil)
	return dispatchedCall{stdout: stdout.String(), stderr: stderr.String(), err: err}
}
