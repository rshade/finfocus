package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewPluginListCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no flags",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "verbose flag",
			args:        []string{"--verbose"},
			expectError: false,
		},
		{
			name:        "help flag",
			args:        []string{"--help"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewPluginListCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPluginListCmdFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginListCmd()

	// Check verbose flag
	verboseFlag := cmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
	assert.Equal(t, "bool", verboseFlag.Value.Type())
	assert.Equal(t, "false", verboseFlag.DefValue)
	assert.Contains(t, verboseFlag.Usage, "Show detailed plugin information")
}

func TestPluginListCmdHelp(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "List all installed plugins with their versions and paths")
	assert.Contains(t, output, "List all installed plugins with their versions and paths")
	assert.Contains(t, output, "--verbose")
	assert.Contains(t, output, "Show detailed plugin information")
}

func TestPluginListCmdExamples(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginListCmd()

	// Check that examples are present
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "finfocus plugin list")
	assert.Contains(t, cmd.Example, "finfocus plugin list --verbose")
	assert.Contains(t, cmd.Example, "List plugins with detailed information")
}

func TestPluginListCmdOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginListCmd()
	// Need to set args to empty to avoid using os.Args
	cmd.SetArgs([]string{})

	// The command should execute without error even when no plugins exist
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestPluginListCmdAvailable(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--available"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Check that registry plugins are listed
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Description")
	assert.Contains(t, output, "Repository")
	assert.Contains(t, output, "Security")
	// Check for known registry plugins
	assert.Contains(t, output, "kubecost")
	assert.Contains(t, output, "aws-public")
}

func TestPluginListCmdAvailableFlag(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginListCmd()

	// Check available flag
	availableFlag := cmd.Flags().Lookup("available")
	assert.NotNil(t, availableFlag)
	assert.Equal(t, "bool", availableFlag.Value.Type())
	assert.Equal(t, "false", availableFlag.DefValue)
	assert.Contains(t, availableFlag.Usage, "List available plugins from registry")
}

// T013: Test --output flag registration and defaults.
func TestPluginListCmd_OutputFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginListCmd()

	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "string", outputFlag.Value.Type())
	assert.Equal(t, "table", outputFlag.DefValue)
	assert.Contains(t, outputFlag.Usage, "Output format")
}

// T013: Test that --output json with no plugins produces [].
func TestPluginListCmd_JSONOutput_NoPlugins(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Point to a temporary home directory so no plugins are found
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	assert.Equal(t, "[]", output)
}

// T013: Test that --output json produces valid JSON array.
func TestPluginListCmd_JSONOutput_ValidJSON(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Use a controlled temp directory so the test is deterministic
	// regardless of the host's installed plugins.
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	var entries []cli.PluginJSONEntry
	require.NoError(t, json.Unmarshal([]byte(output), &entries), "output should be valid JSON array")

	// With an empty FINFOCUS_HOME, no plugins should be found.
	assert.Empty(t, entries, "empty FINFOCUS_HOME should produce no entries")
}

// T013: Test that invalid --output value returns an error.
func TestPluginListCmd_InvalidOutputFormat(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	var buf bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "xml"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

// T013: Test PluginJSONEntry serialization matches contract schema.
func TestPluginJSONEntry_Serialization(t *testing.T) {
	t.Run("full entry with all fields", func(t *testing.T) {
		entry := cli.PluginJSONEntry{
			Name:               "aws-public",
			Version:            "1.0.0",
			Path:               "/home/user/.finfocus/plugins/aws-public/1.0.0/finfocus-plugin-aws-public",
			SpecVersion:        "v0.5.6",
			RuntimeVersion:     "1.0.0",
			SupportedProviders: []string{"aws"},
			Capabilities:       []string{"projected_costs", "actual_costs", "recommendations"},
		}

		data, err := json.Marshal(entry)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &parsed))

		assert.Equal(t, "aws-public", parsed["name"])
		assert.Equal(t, "1.0.0", parsed["version"])
		assert.Equal(t, "/home/user/.finfocus/plugins/aws-public/1.0.0/finfocus-plugin-aws-public", parsed["path"])
		assert.Equal(t, "v0.5.6", parsed["specVersion"])
		assert.Equal(t, "1.0.0", parsed["runtimeVersion"])

		providers, ok := parsed["supportedProviders"].([]interface{})
		require.True(t, ok)
		assert.Len(t, providers, 1)
		assert.Equal(t, "aws", providers[0])

		caps, ok := parsed["capabilities"].([]interface{})
		require.True(t, ok)
		assert.Len(t, caps, 3)

		// Notes should be omitted when empty
		_, hasNotes := parsed["notes"]
		assert.False(t, hasNotes, "empty notes should be omitted from JSON")
	})

	t.Run("failure entry with null providers and capabilities", func(t *testing.T) {
		entry := cli.PluginJSONEntry{
			Name:           "broken-plugin",
			Version:        "0.0.1",
			Path:           "/home/user/.finfocus/plugins/broken/0.0.1/finfocus-plugin-broken",
			SpecVersion:    "N/A",
			RuntimeVersion: "N/A",
			Notes:          "failed to connect: connection refused",
		}

		data, err := json.Marshal(entry)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &parsed))

		assert.Equal(t, "broken-plugin", parsed["name"])
		assert.Equal(t, "N/A", parsed["specVersion"])
		assert.Nil(t, parsed["supportedProviders"])
		assert.Nil(t, parsed["capabilities"])
		assert.Equal(t, "failed to connect: connection refused", parsed["notes"])
	})

	t.Run("empty array serializes as []", func(t *testing.T) {
		entries := []cli.PluginJSONEntry{}
		data, err := json.Marshal(entries)
		require.NoError(t, err)
		assert.Equal(t, "[]", string(data))
	})
}

// T013: Test that table output is unchanged with --output table.
func TestPluginListCmd_TableOutputUnchanged(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// Run without --output flag (default table)
	var defaultBuf bytes.Buffer
	cmd1 := cli.NewPluginListCmd()
	cmd1.SetOut(&defaultBuf)
	cmd1.SetErr(&defaultBuf)
	cmd1.SetArgs([]string{})
	err := cmd1.Execute()
	require.NoError(t, err)

	// Run with explicit --output table
	var tableBuf bytes.Buffer
	cmd2 := cli.NewPluginListCmd()
	cmd2.SetOut(&tableBuf)
	cmd2.SetErr(&tableBuf)
	cmd2.SetArgs([]string{"--output", "table"})
	err = cmd2.Execute()
	require.NoError(t, err)

	// Both should produce identical output
	assert.Equal(t, defaultBuf.String(), tableBuf.String())
}

// BenchmarkPluginList measures plugin listing performance.
// With parallel fetching, execution time should scale O(1) relative to plugin count
// (bounded by the slowest plugin), not O(N) (sum of all plugin latencies).
//
// Run with: go test -bench=BenchmarkPluginList -benchtime=1x ./internal/cli/...
func BenchmarkPluginList(b *testing.B) {
	// Suppress log output during benchmarks
	b.Setenv("FINFOCUS_LOG_LEVEL", "error")

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		cmd := cli.NewPluginListCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			b.Fatalf("plugin list execute failed: %v", err)
		}
	}
}
