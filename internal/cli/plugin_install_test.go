package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPluginInstallCmd_Help(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"plugin", "install", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

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

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"plugin", "install"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no plugin specified")
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "accepts 1 arg") {
		t.Errorf("expected 'accepts 1 arg' error, got: %s", errOutput)
	}
}

func TestPluginInstallCmd_InvalidPlugin(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"plugin", "install", "nonexistent-plugin-xyz"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestPluginInstallCmd_InvalidGitHubURL(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	rootCmd := cli.NewRootCmd("test")

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"plugin", "install", "github.com/invalid"})

	err := rootCmd.Execute()
	if err == nil {
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
	expectedFlags := []string{"force", "no-save", "plugin-dir", "clean"}
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

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"plugin", "install", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	// Install from URL - this will fail but we can test it reaches the security warning code
	rootCmd.SetArgs([]string{"plugin", "install", "github.com/owner/repo", "--plugin-dir", tmpDir})

	_ = rootCmd.Execute() // Error expected - we just want to exercise the code path

	output := stdout.String()
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

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(
		[]string{"plugin", "install", "nonexistent-registry-plugin", "--plugin-dir", tmpDir},
	)

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent registry plugin")
	}

	errOutput := err.Error()
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

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	// Try to install with version - will fail but tests the code path
	rootCmd.SetArgs([]string{"plugin", "install", "kubecost@v999.0.0", "--plugin-dir", tmpDir})

	err := rootCmd.Execute()
	// Error expected (version doesn't exist) but we exercised the code path
	if err == nil {
		t.Error("expected error for non-existent version")
	}
}
