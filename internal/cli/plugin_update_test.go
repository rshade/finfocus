package cli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPluginUpdateCmd_Help(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "update", "--help"})

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	output := string(result.Stdout)

	// Check for expected content
	expectedStrings := []string{
		"Update",
		"--dry-run",
		"--version",
		"--plugin-dir",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("help output missing expected string: %q", expected)
		}
	}
}

func TestPluginUpdateCmd_NoArgs(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "update"})

	if result.ExitCode == 0 {
		t.Error("expected error when no plugin specified")
	}

	errOutput := string(result.Stderr)
	if !strings.Contains(errOutput, "accepts 1 arg") {
		t.Errorf("expected 'accepts 1 arg' error, got: %s", errOutput)
	}
}

func TestPluginUpdateCmd_NotInstalled(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	// Set HOME to temp directory
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "update", "nonexistent-plugin"})

	if result.ExitCode == 0 {
		t.Error("expected error for non-installed plugin")
	}
}

func TestPluginUpdateCmd_Flags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	// Get the update command to check flags
	pluginCmd, _, err := rootCmd.Find([]string{"plugin", "update"})
	if err != nil {
		t.Fatalf("failed to find update command: %v", err)
	}

	// Check that expected flags exist
	// Note: --dry-run is now a global flag, not a local command flag
	expectedFlags := []string{"version", "plugin-dir", "skip-checksum"}
	for _, flag := range expectedFlags {
		if pluginCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s not found", flag)
		}
	}
}

func TestPluginUpdateCmd_DryRun(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "update", "test-plugin", "--dry-run"})

	// Should still error because plugin not installed
	if result.ExitCode == 0 {
		t.Error("expected error for non-installed plugin even with dry-run")
	}
}

func TestPluginUpdateCmd_VersionFlag(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "update", "test-plugin", "--version", "v2.0.0"},
	)

	// Should error because plugin not installed
	if result.ExitCode == 0 {
		t.Error("expected error for non-installed plugin")
	}
}

func TestPluginUpdateCmd_PluginDirFlag(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "update", "test-plugin", "--plugin-dir", tmpDir},
	)

	// Should error because plugin not installed
	if result.ExitCode == 0 {
		t.Error("expected error for non-installed plugin")
	}
}

func TestPluginUpdateCmd_AllFlagsCombined(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(context.Background(), t, rootCmd,
		[]string{
			"plugin",
			"update",
			"test-plugin",
			"--dry-run",
			"--version",
			"v1.0.0",
			"--plugin-dir",
			tmpDir,
		},
	)

	// Should error because plugin not installed
	if result.ExitCode == 0 {
		t.Error("expected error for non-installed plugin")
	}
}
