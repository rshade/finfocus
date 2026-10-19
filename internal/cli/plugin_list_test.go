package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/cli"
)

// createMockPluginBinary writes a mock plugin executable named name into dir,
// appending .exe on Windows so the registry's executable detection (which
// requires the .exe extension there) discovers it.
func createMockPluginBinary(t *testing.T, dir, name string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho test"), 0o755))
}

//nolint:paralleltest // t.Setenv changes the process-wide environment
func TestNewPluginListCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_HOME", t.TempDir())
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

	allFlag := cmd.Flags().Lookup("all")
	require.NotNil(t, allFlag)
	assert.Equal(t, "bool", allFlag.Value.Type())
	assert.Equal(t, "false", allFlag.DefValue)
	assert.Contains(t, allFlag.Usage, "every installed version")
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
	assert.Contains(t, output, "highest semver")
	assert.Contains(t, output, "--all")
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
	assert.Contains(t, cmd.Example, "finfocus plugin list --all")
	assert.Contains(t, cmd.Example, "finfocus plugin list --verbose")
	assert.Contains(t, cmd.Example, "List plugins with detailed information")
}

func TestPluginListCmdOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_HOME", t.TempDir())
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
	assert.Contains(t, output, "opencost")
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
	t.Parallel()

	t.Run("full entry with all fields", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
		entries := []cli.PluginJSONEntry{}
		data, err := json.Marshal(entries)
		require.NoError(t, err)
		assert.Equal(t, "[]", string(data))
	})
}

// T013: Test that table output is unchanged with --output table.
func TestPluginListCmd_TableOutputUnchanged(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	for _, version := range []string{"0.1.5", "0.1.9", "0.2.0"} {
		dir := filepath.Join(home, "plugins", "aws-public", version)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		createMockPluginBinary(t, dir, "finfocus-plugin-aws-public")
	}

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

	// Both should produce identical output, and only the highest semver.
	assert.Equal(t, defaultBuf.String(), tableBuf.String())
	assert.Contains(t, defaultBuf.String(), "0.2.0")
	assert.NotContains(t, defaultBuf.String(), "0.1.9")
	assert.NotContains(t, defaultBuf.String(), "0.1.5")
}

// TestPluginListCmd_VersionOrder catches completion-order-dependent rows for
// multiple installed versions of the same plugin.
func TestPluginListCmd_VersionOrder(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	for _, version := range []string{"0.1.5", "0.1.9", "0.2.0"} {
		dir := filepath.Join(home, "plugins", "aws-public", version)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		createMockPluginBinary(t, dir, "finfocus-plugin-aws-public")
	}

	// Cancel metadata lookups so ordering is tested independently of plugin startup.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 20 {
		var buf bytes.Buffer
		cmd := cli.NewPluginListCmd()
		cmd.SetContext(ctx)
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--all", "--output", "json"})
		require.NoError(t, cmd.Execute())

		var entries []cli.PluginJSONEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		versions := make([]string, 0, len(entries))
		for _, entry := range entries {
			versions = append(versions, entry.Version)
		}
		assert.Equal(t, []string{"0.2.0", "0.1.9", "0.1.5"}, versions)
	}
}

func TestPluginListCmd_SemverDescending(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	for _, version := range []string{"0.9.0", "0.10.0", "0.2.0", "beta", "alpha"} {
		dir := filepath.Join(home, "plugins", "aws-public", version)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		createMockPluginBinary(t, dir, "finfocus-plugin-aws-public")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	want := []string{"0.10.0", "0.9.0", "0.2.0", "alpha", "beta"}

	var firstTable string
	var firstJSON string
	for run := range 2 {
		var tableBuf bytes.Buffer
		tableCmd := cli.NewPluginListCmd()
		tableCmd.SetContext(ctx)
		tableCmd.SetOut(&tableBuf)
		tableCmd.SetErr(&tableBuf)
		tableCmd.SetErr(&bytes.Buffer{})
		tableCmd.SetArgs([]string{"--all", "--output", "table"})
		require.NoError(t, tableCmd.Execute())

		var jsonBuf bytes.Buffer
		jsonCmd := cli.NewPluginListCmd()
		jsonCmd.SetContext(ctx)
		jsonCmd.SetOut(&jsonBuf)
		jsonCmd.SetErr(&bytes.Buffer{})
		jsonCmd.SetArgs([]string{"--all", "--output", "json"})
		require.NoError(t, jsonCmd.Execute())

		if run == 0 {
			firstTable = tableBuf.String()
			firstJSON = jsonBuf.String()
		} else {
			assert.Equal(t, firstTable, tableBuf.String())
			assert.JSONEq(t, firstJSON, jsonBuf.String())
		}

		var entries []cli.PluginJSONEntry
		require.NoError(t, json.Unmarshal(jsonBuf.Bytes(), &entries))
		versions := make([]string, 0, len(entries))
		for _, entry := range entries {
			versions = append(versions, entry.Version)
		}
		assert.Equal(t, want, versions)

		order := make([]string, 0, len(want))
		for _, line := range strings.Split(tableBuf.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "aws-public" {
				order = append(order, fields[1])
			}
		}
		assert.Equal(t, want, order)
	}
}

// T038: Test that batch_cost capability appears in JSON output.
func TestPluginJSONEntry_BatchCostCapability(t *testing.T) {
	t.Parallel()

	t.Run("batch_cost in capabilities appears in JSON output", func(t *testing.T) {
		t.Parallel()
		entry := cli.PluginJSONEntry{
			Name:               "aws-public",
			Version:            "1.0.0",
			Path:               "/home/user/.finfocus/plugins/aws-public/1.0.0/finfocus-plugin-aws-public",
			SpecVersion:        "v0.6.0",
			RuntimeVersion:     "1.0.0",
			SupportedProviders: []string{"aws"},
			Capabilities:       []string{"projected_costs", "actual_costs", "batch_cost"},
		}

		data, err := json.Marshal(entry)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &parsed))

		caps, ok := parsed["capabilities"].([]interface{})
		require.True(t, ok)
		assert.Len(t, caps, 3)

		// Verify batch_cost is present
		found := false
		for _, c := range caps {
			if c == "batch_cost" {
				found = true
				break
			}
		}
		assert.True(t, found, "batch_cost should appear in capabilities JSON output")
	})

	t.Run("batch_cost in JSON serialization", func(t *testing.T) {
		t.Parallel()
		// Create a PluginJSONEntry with batch_cost and verify it round-trips
		entry := cli.PluginJSONEntry{
			Name:         "test-plugin",
			Version:      "0.1.0",
			Capabilities: []string{"projected_costs", "batch_cost"},
		}

		data, err := json.Marshal(entry)
		require.NoError(t, err)

		output := string(data)
		assert.Contains(t, output, "batch_cost",
			"batch_cost capability should be present in serialized output")
		assert.Contains(t, output, "projected_costs")
	})
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

// TestPluginListCmd_NoPlugins tests listing with no plugins installed.
func TestPluginListCmd_NoPlugins(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set FINFOCUS_HOME to temp directory to ensure no plugins are found
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	cmd := cli.NewPluginListCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	// Should succeed even with no plugins
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "No plugins")
}

// TestPluginListCmd_WithPlugins tests listing with mock plugins.
func TestPluginListCmd_WithPlugins(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Create temporary plugin directory structure
	tempDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tempDir)
	pluginDir := filepath.Join(tempDir, "plugins")

	// Create mock plugin directories
	kubecostDir := filepath.Join(pluginDir, "kubecost", "v0.1.0")
	err := os.MkdirAll(kubecostDir, 0755)
	require.NoError(t, err)

	// Create mock plugin binary
	createMockPluginBinary(t, kubecostDir, "finfocus-plugin-kubecost")

	cmd := cli.NewPluginListCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "kubecost")
	assert.Contains(t, output, "v0.1.0")
}

// TestPluginListCmd_VerboseOutput tests verbose output for plugin list.
func TestPluginListCmd_VerboseOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Create temporary plugin directory
	tempDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tempDir)
	pluginDir := filepath.Join(tempDir, "plugins")
	kubecostDir := filepath.Join(pluginDir, "kubecost", "v0.1.0")
	err := os.MkdirAll(kubecostDir, 0755)
	require.NoError(t, err)

	// Create plugin binary
	createMockPluginBinary(t, kubecostDir, "finfocus-plugin-kubecost")

	cmd := cli.NewPluginListCmd()
	cmd.SetArgs([]string{"--verbose"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	// Verbose output should have extra column
	output := out.String()
	assert.Contains(t, output, "Path")
	assert.Contains(t, output, "kubecost")
}

type listedPlugin struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Active    bool   `json:"active"`
	Providers any    `json:"supportedProviders"`
}

type recordingLauncher struct {
	mu    sync.Mutex
	paths []string
}

func (r *recordingLauncher) Start(
	_ context.Context,
	path string,
	_ ...string,
) (*grpc.ClientConn, func() error, error) {
	r.mu.Lock()
	r.paths = append(r.paths, path)
	r.mu.Unlock()
	return nil, nil, errors.New("spy declined launch")
}

func (r *recordingLauncher) started() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

func installMockPlugin(t *testing.T, home, name, version string) string {
	t.Helper()
	dir := filepath.Join(home, "plugins", name, version)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	binary := "finfocus-plugin-" + name
	createMockPluginBinary(t, dir, binary)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	return filepath.Join(dir, binary)
}

func runPluginList(t *testing.T, args ...string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := cli.NewPluginListCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return stdout.String(), stderr.String()
}

func decodePluginList(t *testing.T, stdout string) []listedPlugin {
	t.Helper()
	var entries []listedPlugin
	require.NoError(t, json.Unmarshal([]byte(stdout), &entries))
	return entries
}

type tablePlugin struct {
	Name      string
	Version   string
	Active    string
	Providers string
}

func pluginTableRows(output string) []tablePlugin {
	var header []string
	var rows []tablePlugin
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "----") {
			continue
		}
		if fields[0] == "Name" {
			header = fields
			continue
		}
		if header == nil {
			continue
		}
		row := tablePlugin{Name: fields[0]}
		for i, column := range header {
			if i >= len(fields) {
				break
			}
			switch column {
			case "Version":
				row.Version = fields[i]
			case "Active":
				row.Active = fields[i]
			case "Providers":
				row.Providers = fields[i]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func TestPluginListCmd_LatestByDefault(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	installMockPlugin(t, home, "aws-public", "0.1.5")
	installMockPlugin(t, home, "aws-public", "0.1.9")
	installMockPlugin(t, home, "aws-public", "0.2.0")
	installMockPlugin(t, home, "recorder", "0.1.0")

	stdout, stderr := runPluginList(t, "--output", "json")
	assert.Empty(t, stderr)
	assert.Equal(t, []listedPlugin{
		{Name: "aws-public", Version: "0.2.0", Active: true, Providers: []any{}},
		{Name: "recorder", Version: "0.1.0", Active: true, Providers: []any{}},
	}, decodePluginList(t, stdout))

	table, tableErr := runPluginList(t, "--output", "table")
	assert.Empty(t, tableErr)
	assert.Equal(t, []tablePlugin{
		{Name: "aws-public", Version: "0.2.0", Providers: "*"},
		{Name: "recorder", Version: "0.1.0", Providers: "*"},
	}, pluginTableRows(table))
	assert.NotContains(t, table, "Active")
}

func TestPluginListCmd_AllMarksActiveVersion(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	installMockPlugin(t, home, "aws-public", "0.1.5")
	installMockPlugin(t, home, "aws-public", "0.1.9")
	installMockPlugin(t, home, "aws-public", "0.2.0")
	installMockPlugin(t, home, "recorder", "0.1.0")
	spy := &recordingLauncher{}
	cli.SetPluginListLauncherForTest(t, spy)

	stdout, stderr := runPluginList(t, "--all", "--output", "json")
	assert.Empty(t, stderr)
	assert.Equal(t, []listedPlugin{
		{Name: "aws-public", Version: "0.2.0", Active: true, Providers: []any{}},
		{Name: "aws-public", Version: "0.1.9", Active: false},
		{Name: "aws-public", Version: "0.1.5", Active: false},
		{Name: "recorder", Version: "0.1.0", Active: true, Providers: []any{}},
	}, decodePluginList(t, stdout))

	table, _ := runPluginList(t, "--all", "--output", "table")
	assert.Equal(t, []tablePlugin{
		{Name: "aws-public", Version: "0.2.0", Active: "yes", Providers: "*"},
		{Name: "aws-public", Version: "0.1.9", Active: "no", Providers: "-"},
		{Name: "aws-public", Version: "0.1.5", Active: "no", Providers: "-"},
		{Name: "recorder", Version: "0.1.0", Active: "yes", Providers: "*"},
	}, pluginTableRows(table))
}

func TestPluginListCmd_DoesNotLaunchOlderVersions(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	oldA := installMockPlugin(t, home, "aws-public", "0.1.5")
	oldB := installMockPlugin(t, home, "aws-public", "0.1.9")
	latest := installMockPlugin(t, home, "aws-public", "0.2.0")
	recorder := installMockPlugin(t, home, "recorder", "0.1.0")

	for _, args := range [][]string{{}, {"--all"}} {
		spy := &recordingLauncher{}
		cli.SetPluginListLauncherForTest(t, spy)
		_, stderr := runPluginList(t, args...)
		assert.Empty(t, stderr)
		assert.ElementsMatch(t, []string{latest, recorder}, spy.started())
		assert.NotContains(t, spy.started(), oldA)
		assert.NotContains(t, spy.started(), oldB)
	}
}

func TestPluginListCmd_VerboseLaunchesEveryVersion(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	oldA := installMockPlugin(t, home, "aws-public", "0.1.5")
	oldB := installMockPlugin(t, home, "aws-public", "0.1.9")
	latest := installMockPlugin(t, home, "aws-public", "0.2.0")
	recorder := installMockPlugin(t, home, "recorder", "0.1.0")
	spy := &recordingLauncher{}
	cli.SetPluginListLauncherForTest(t, spy)

	_, stderr := runPluginList(t, "--all", "--verbose")
	assert.Empty(t, stderr)
	assert.ElementsMatch(t, []string{oldA, oldB, latest, recorder}, spy.started())
}

func TestPluginListCmd_InvalidSemverWarns(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	installMockPlugin(t, home, "aws-public", "not-a-version")

	stdout, stderr := runPluginList(t)
	assert.Contains(t, stderr, "invalid semver")
	assert.Contains(t, stderr, "not-a-version")
	assert.Contains(t, stdout, "No plugins found.")
	assert.NotContains(t, stdout, "not-a-version")

	installMockPlugin(t, home, "aws-public", "1.0.0")
	stdout, stderr = runPluginList(t, "--output", "json")
	assert.Contains(t, stderr, "not-a-version")
	assert.Equal(t, []listedPlugin{
		{Name: "aws-public", Version: "1.0.0", Active: true, Providers: []any{}},
	}, decodePluginList(t, stdout))

	stdout, stderr = runPluginList(t, "--all", "--output", "json")
	assert.Contains(t, stderr, "invalid semver")
	assert.Equal(t, []listedPlugin{
		{Name: "aws-public", Version: "1.0.0", Active: true, Providers: []any{}},
		{Name: "aws-public", Version: "not-a-version", Active: false},
	}, decodePluginList(t, stdout))
}
