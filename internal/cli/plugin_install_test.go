package cli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPluginInstallCmd_Help(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "install", "--help"})

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	output := string(result.Stdout)

	// Check for expected content
	expectedStrings := []string{
		"Install a plugin from",
		"--force",
		"--no-save",
		"--plugin-dir",
		"kubecost",
		"github.com",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("help output missing expected string: %q", expected)
		}
	}
}

func TestPluginInstallCmd_NoArgs(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "install"})

	if result.ExitCode == 0 {
		t.Error("expected error when no plugin specified")
	}

	errOutput := string(result.Stderr)
	if !strings.Contains(errOutput, "accepts 1 arg") {
		t.Errorf("expected 'accepts 1 arg' error, got: %s", errOutput)
	}
}

func TestPluginInstallCmd_InvalidPlugin(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "install", "nonexistent-plugin-xyz"})

	if result.ExitCode == 0 {
		t.Error("expected error for non-existent plugin")
	}
}

func TestPluginInstallCmd_InvalidGitHubURL(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "install", "github.com/invalid"})

	if result.ExitCode == 0 {
		t.Error("expected error for invalid GitHub URL")
	}
}

func TestPluginInstallCmd_Flags(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	// Get the install command to check flags
	pluginCmd, _, err := rootCmd.Find([]string{"plugin", "install"})
	if err != nil {
		t.Fatalf("failed to find install command: %v", err)
	}

	// Check that expected flags exist
	expectedFlags := []string{"force", "no-save", "plugin-dir", "clean", "skip-checksum"}
	for _, flag := range expectedFlags {
		if pluginCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s not found", flag)
		}
	}

	// Check short flag for force
	if pluginCmd.Flags().ShorthandLookup("f") == nil {
		t.Error("expected short flag -f for --force not found")
	}
}

func TestPluginInstallCmd_Examples(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "install", "--help"})

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	output := string(result.Stdout)

	// Check for example commands
	examples := []string{
		"finfocus plugin install kubecost",
		"kubecost@v1.0.0",
		"--force",
		"--no-save",
		"--clean",
	}

	for _, example := range examples {
		if !strings.Contains(output, example) {
			t.Errorf("help output missing example: %q", example)
		}
	}
}

func TestPluginInstallCmd_URLSecurityWarning(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Install from URL - this will fail but we can test it reaches the security warning code
	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "install", "github.com/owner/repo", "--plugin-dir", tmpDir},
	)

	output := string(result.Stdout)
	// Check for security warning for URL-based installs
	if !strings.Contains(output, "Installing from URL") {
		t.Errorf("expected URL security warning, got: %s", output)
	}
}

func TestPluginInstallCmd_RegistryPluginNotFound(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	result := axtest.Run(context.Background(), t, rootCmd,
		[]string{"plugin", "install", "nonexistent-registry-plugin", "--plugin-dir", tmpDir},
	)

	if result.ExitCode == 0 {
		t.Error("expected error for non-existent registry plugin")
	}

	errOutput := string(result.Stderr)
	if !strings.Contains(errOutput, "not found") {
		t.Errorf("expected 'not found' error, got: %s", errOutput)
	}
}

func TestPluginInstallCmd_VersionSpecified(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Try to install with version - will fail but tests the code path
	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "install", "kubecost@v999.0.0", "--plugin-dir", tmpDir},
	)

	// Error expected (version doesn't exist) but we exercised the code path
	if result.ExitCode == 0 {
		t.Error("expected error for non-existent version")
	}
}
