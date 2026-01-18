package cli_test

import (
	"bytes"
	"testing"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
