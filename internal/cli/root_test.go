package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewRootCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tests := []struct {
		name        string
		args        []string
		expectError bool
		checkOutput func(t *testing.T, output string)
	}{
		{
			name:        "help flag",
			args:        []string{"--help"},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(
					t,
					output,
					"FinFocus: Calculate projected and actual cloud costs via plugins",
				)
				assert.Contains(t, output, "Available Commands:")
				assert.Contains(t, output, "cost")
				assert.Contains(t, output, "plugin")
			},
		},
		{
			name:        "version flag",
			args:        []string{"--version"},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "finfocus version")
			},
		},
		{
			name:        "debug flag",
			args:        []string{"--debug", "--help"},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "FinFocus")
			},
		},
		{
			name:        "invalid command",
			args:        []string{"invalid"},
			expectError: true,
		},
		{
			name:        "cost subcommand help",
			args:        []string{"cost", "--help"},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Cost calculation commands")
				assert.Contains(t, output, "projected")
				assert.Contains(t, output, "actual")
			},
		},
		{
			name:        "plugin subcommand help",
			args:        []string{"plugin", "--help"},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Plugin management commands")
				assert.Contains(t, output, "list")
				assert.Contains(t, output, "validate")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewRootCmd("test-version")
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkOutput != nil {
					tt.checkOutput(t, buf.String())
				}
			}
		})
	}
}

func TestRootCmdExamples(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewRootCmd("test-version")

	// Check that examples are present
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "finfocus cost projected")
	assert.Contains(t, cmd.Example, "finfocus cost actual")
	assert.Contains(t, cmd.Example, "finfocus plugin list")
	assert.Contains(t, cmd.Example, "finfocus plugin validate")
}

func TestRootCmdStructure(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewRootCmd("test-version")

	// Check that main subcommands exist
	costCmd, _, err := cmd.Find([]string{"cost"})
	require.NoError(t, err)
	assert.NotNil(t, costCmd)

	pluginCmd, _, err := cmd.Find([]string{"plugin"})
	require.NoError(t, err)
	assert.NotNil(t, pluginCmd)

	// Check that cost subcommands exist
	projectedCmd, _, err := cmd.Find([]string{"cost", "projected"})
	require.NoError(t, err)
	assert.NotNil(t, projectedCmd)

	actualCmd, _, err := cmd.Find([]string{"cost", "actual"})
	require.NoError(t, err)
	assert.NotNil(t, actualCmd)

	// Check that plugin subcommands exist
	listCmd, _, err := cmd.Find([]string{"plugin", "list"})
	require.NoError(t, err)
	assert.NotNil(t, listCmd)

	validateCmd, _, err := cmd.Find([]string{"plugin", "validate"})
	require.NoError(t, err)
	assert.NotNil(t, validateCmd)
}

func TestRootCmdFlags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewRootCmd("test-version")

	// Check persistent flags
	debugFlag := cmd.PersistentFlags().Lookup("debug")
	assert.NotNil(t, debugFlag)
	assert.Equal(t, "bool", debugFlag.Value.Type())
	assert.Equal(t, "false", debugFlag.DefValue)

	// Check version flag is available
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "test-version")
}

// TestRootCmdPluginMode tests that the root command correctly detects plugin mode
// and adjusts its Use and Example strings accordingly.
func TestRootCmdPluginMode(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name           string
		args           []string
		env            map[string]string
		expectedUse    string
		exampleContain string
		exampleNotHave string
	}{
		{
			name:           "standard mode - regular binary name",
			args:           []string{"/usr/bin/finfocus"},
			env:            map[string]string{},
			expectedUse:    "finfocus",
			exampleContain: "finfocus cost projected",
			exampleNotHave: "pulumi plugin run tool cost",
		},
		{
			name:           "plugin mode - plugin binary name",
			args:           []string{"/usr/bin/pulumi-tool-cost"},
			env:            map[string]string{},
			expectedUse:    "pulumi plugin run tool cost",
			exampleContain: "pulumi plugin run tool cost",
			exampleNotHave: "",
		},
		{
			name:           "plugin mode - Windows binary name with .exe",
			args:           []string{"C:\\pulumi\\plugins\\pulumi-tool-cost.exe"},
			env:            map[string]string{},
			expectedUse:    "pulumi plugin run tool cost",
			exampleContain: "pulumi plugin run tool cost",
			exampleNotHave: "",
		},
		{
			name:           "plugin mode - env var override true",
			args:           []string{"/usr/bin/finfocus"},
			env:            map[string]string{"FINFOCUS_PLUGIN_MODE": "true"},
			expectedUse:    "pulumi plugin run tool cost",
			exampleContain: "pulumi plugin run tool cost",
			exampleNotHave: "",
		},
		{
			name:           "plugin mode - env var override 1",
			args:           []string{"/usr/bin/finfocus"},
			env:            map[string]string{"FINFOCUS_PLUGIN_MODE": "1"},
			expectedUse:    "pulumi plugin run tool cost",
			exampleContain: "pulumi plugin run tool cost",
			exampleNotHave: "",
		},
		{
			name:           "standard mode - env var set to false",
			args:           []string{"/usr/bin/finfocus"},
			env:            map[string]string{"FINFOCUS_PLUGIN_MODE": "false"},
			expectedUse:    "finfocus",
			exampleContain: "finfocus cost projected",
			exampleNotHave: "pulumi plugin run tool cost",
		},
		{
			name:           "plugin mode - binary name takes precedence over false env",
			args:           []string{"/usr/bin/pulumi-tool-cost"},
			env:            map[string]string{"FINFOCUS_PLUGIN_MODE": "false"},
			expectedUse:    "pulumi plugin run tool cost",
			exampleContain: "pulumi plugin run tool cost",
			exampleNotHave: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupEnv := func(key string) (string, bool) {
				val, ok := tt.env[key]
				return val, ok
			}

			cmd := cli.NewRootCmdWithArgs("test-version", tt.args, lookupEnv)

			assert.Equal(t, tt.expectedUse, cmd.Use, "Use string mismatch")
			assert.Contains(t, cmd.Example, tt.exampleContain, "Example should contain expected string")
			if tt.exampleNotHave != "" {
				assert.NotContains(t, cmd.Example, tt.exampleNotHave, "Example should not contain unexpected string")
			}
		})
	}
}

// TestRootCmdPluginModeHelpOutput verifies the help output in plugin mode.
func TestRootCmdPluginModeHelpOutput(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	lookupEnv := func(key string) (string, bool) {
		if key == "FINFOCUS_PLUGIN_MODE" {
			return "true", true
		}
		return "", false
	}

	var buf bytes.Buffer
	cmd := cli.NewRootCmdWithArgs("test-version", []string{"/usr/bin/finfocus"}, lookupEnv)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "pulumi plugin run tool cost", "Help should show plugin usage")
	assert.Contains(t, output, "pulumi plugin run tool cost -- cost projected", "Examples should use plugin syntax")
}

// TestRootCmdProjectDirFlag verifies the --project-dir persistent flag:
// 1. Exists as a persistent string flag with empty default
// 2. Is available on subcommands (inherited)
// 3. Appears in help output.
func TestRootCmdProjectDirFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")

	t.Run("flag exists with correct type and default", func(t *testing.T) {
		cmd := cli.NewRootCmd("test-version")

		flag := cmd.PersistentFlags().Lookup("project-dir")
		require.NotNil(t, flag, "--project-dir persistent flag should exist")
		assert.Equal(t, "string", flag.Value.Type(), "flag should be a string type")
		assert.Equal(t, "", flag.DefValue, "flag should have empty default value")
		assert.Contains(t, flag.Usage, "project directory",
			"flag usage should describe project directory")
	})

	t.Run("flag available on cost subcommand via help", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := cli.NewRootCmd("test-version")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"cost", "--project-dir", "/some/path", "--help"})

		err := cmd.Execute()
		require.NoError(t, err, "cost --project-dir /some/path --help should succeed")

		output := buf.String()
		assert.Contains(t, output, "Cost calculation commands",
			"cost help output should be displayed")
	})

	t.Run("flag available on config subcommand via help", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := cli.NewRootCmd("test-version")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"config", "--project-dir", "/another/path", "--help"})

		err := cmd.Execute()
		require.NoError(t, err, "config --project-dir /another/path --help should succeed")

		output := buf.String()
		assert.Contains(t, output, "Configuration management commands",
			"config help output should be displayed")
	})

	t.Run("flag appears in root help output", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := cli.NewRootCmd("test-version")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--help"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "--project-dir",
			"root help should mention --project-dir flag")
	})
}

// TestExitCodeBehavior verifies that the CLI returns proper exit codes:
// - nil (exit 0) for successful commands
// - error (exit 1) for failed commands
// Note: This tests the Execute() error return, not os.Exit() directly.
// The main() function converts non-nil errors to os.Exit(1).
func TestExitCodeBehavior(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "success - help command",
			args:        []string{"--help"},
			expectError: false,
		},
		{
			name:        "success - version command",
			args:        []string{"--version"},
			expectError: false,
		},
		{
			name:        "failure - unknown command",
			args:        []string{"unknown-command"},
			expectError: true,
		},
		{
			name:        "failure - unknown flag",
			args:        []string{"--unknown-flag"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewRootCmd("test-version")
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err, "Command should return error for exit code 1")
			} else {
				require.NoError(t, err, "Command should return nil for exit code 0")
			}
		})
	}
}
