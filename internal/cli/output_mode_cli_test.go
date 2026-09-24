package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

const simplePlanPath = "../../examples/plans/aws-simple-plan.json"

// TestFormatJSONSelectsMachineOutput verifies that the global --format json
// flag (which the MCP dispatcher injects on every tools/call) makes every
// command print a parseable JSON payload on stdout when --output is not set.
func TestFormatJSONSelectsMachineOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "cost projected", args: []string{"cost", "projected", "--pulumi-json", simplePlanPath}},
		{name: "cost recommendations", args: []string{"cost", "recommendations", "--pulumi-json", simplePlanPath}},
		{name: "plugin list without plugin dir", args: []string{"plugin", "list"}},
		{name: "plugin list available", args: []string{"plugin", "list", "--available"}},
		{name: "plugin validate without plugin dir", args: []string{"plugin", "validate"}},
		{name: "config list", args: []string{"config", "list"}},
		{name: "config validate", args: []string{"config", "validate"}},
		{name: "config routes list", args: []string{"config", "routes", "list"}},
		{name: "config init dry run", args: []string{"config", "init", "--global", "--dry-run"}},
		{name: "analyzer uninstall dry run", args: []string{"analyzer", "uninstall", "--dry-run"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("FINFOCUS_HOME", home)
			t.Setenv("PULUMI_HOME", t.TempDir())

			args := append(append([]string{}, tc.args...), "--format", "json")
			result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
			require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
			assert.True(t, json.Valid(result.Stdout), "stdout must be JSON, got: %s", result.Stdout)
		})
	}
}

func TestFormatJSONPluginListEmptyIsEmptyArray(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"plugin", "list", "--format", "json"})
	require.Equal(t, 0, result.ExitCode)
	assert.JSONEq(t, "[]", string(result.Stdout))
	assert.NotContains(t, string(result.Stdout), "Plugin directory does not exist")
}

func TestExplicitOutputWinsOverFormatJSON(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "projected", "--pulumi-json", simplePlanPath, "--output", "table", "--format", "json"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.False(t, json.Valid(result.Stdout))
	assert.Contains(t, string(result.Stdout), "COST SUMMARY")
}

func TestFormatJSONDryRunConfigInitWritesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "init", "--global", "--dry-run", "--format", "json"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)

	var action struct {
		Action string `json:"action"`
		DryRun bool   `json:"dry_run"`
		Path   string `json:"path"`
	}
	require.NoError(t, json.Unmarshal(result.Stdout, &action))
	assert.Equal(t, "would_create", action.Action)
	assert.True(t, action.DryRun)
	assert.NoFileExists(t, filepath.Join(home, "config.hujson"))
}

// TestInvalidOutputRejectedBeforeLoadingState verifies an invalid --output is
// reported as such even when the input it would load does not exist, so the
// format error is never masked by (or skipped because of) state loading.
func TestInvalidOutputRejectedBeforeLoadingState(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	tests := []struct {
		name string
		args []string
	}{
		{name: "cost actual", args: []string{"cost", "actual", "--pulumi-json", missing, "--from", "2025-01-01"}},
		{name: "cost recommendations", args: []string{"cost", "recommendations", "--pulumi-json", missing}},
		{name: "cost estimate", args: []string{"cost", "estimate", "--pulumi-json", missing}},
		{name: "overview", args: []string{"overview", "--pulumi-state", missing}},
		{name: "analyzer check", args: []string{"analyzer", "check"}},
		{name: "config routes list", args: []string{"config", "routes", "list"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FINFOCUS_HOME", t.TempDir())

			args := append(append([]string{}, tc.args...), "--output", "xml", "--format", "json")
			result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
			require.NotEqual(t, 0, result.ExitCode)
			assert.Contains(t, string(result.Stderr), "xml")
			assert.Contains(t, string(result.Stderr), "output format")
		})
	}
}

// TestPipedOutputRequiresExplicitMachineSignal verifies that a piped (non-TTY)
// invocation keeps its human default unless --format json or AGENT_MODE asks for
// JSON. config list therefore prints YAML when piped, as it did before ax-go.
func TestPipedOutputRequiresExplicitMachineSignal(t *testing.T) {
	tests := []struct {
		name      string
		agentMode string
		args      []string
		wantJSON  bool
	}{
		{name: "no signal stays yaml", args: []string{"config", "list"}},
		{name: "format json", args: []string{"config", "list", "--format", "json"}, wantJSON: true},
		{name: "AGENT_MODE=1", agentMode: "1", args: []string{"config", "list"}, wantJSON: true},
		{name: "format human beats AGENT_MODE", agentMode: "1", args: []string{"config", "list", "--format", "human"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			t.Setenv("AGENT_MODE", tc.agentMode)

			result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), tc.args)
			require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
			assert.Equal(t, tc.wantJSON, json.Valid(result.Stdout), "stdout: %.200s", result.Stdout)
		})
	}
}
