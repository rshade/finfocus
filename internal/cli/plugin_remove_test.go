package cli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPluginRemoveCmd_Help(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "remove", "--help"})

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	output := string(result.Stdout)

	// Check for expected content
	expectedStrings := []string{
		"Remove",
		"--keep-config",
		"--plugin-dir",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("help output missing expected string: %q", expected)
		}
	}
}

func TestPluginRemoveCmd_NoArgs(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "remove"})

	if result.ExitCode == 0 {
		t.Error("expected error when no plugin specified")
	}

	errOutput := string(result.Stderr)
	if !strings.Contains(errOutput, "accepts 1 arg") {
		t.Errorf("expected 'accepts 1 arg' error, got: %s", errOutput)
	}
}

func TestPluginRemoveCmd_NotInstalled(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	// Set HOME to temp directory
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "remove", "nonexistent-plugin"})

	if result.ExitCode == 0 {
		t.Error("expected error for non-installed plugin")
	}
}

func TestPluginRemoveCmd_Flags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	// Get the remove command to check flags
	pluginCmd, _, err := rootCmd.Find([]string{"plugin", "remove"})
	if err != nil {
		t.Fatalf("failed to find remove command: %v", err)
	}

	// Check that expected flags exist
	expectedFlags := []string{"keep-config", "plugin-dir"}
	for _, flag := range expectedFlags {
		if pluginCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s not found", flag)
		}
	}
}

func TestPluginRemoveCmd_Aliases(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	// Test that "uninstall" alias works
	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "uninstall", "--help"})

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code with uninstall alias: %d", result.ExitCode)
	}
}
