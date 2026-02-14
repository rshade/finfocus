package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/pkg/version"
)

// newTestSetupCmd creates a testable setup command with captured output.
// It returns the command and a buffer that receives all output.
func newTestSetupCmd() (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := NewSetupCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Silence usage on error to keep test output clean
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd, buf
}

// runTestSetup executes the setup command with the given flags in a temporary directory.
// It sets FINFOCUS_HOME to the temp dir and returns the command output.
func runTestSetup(t *testing.T, flags ...string) (string, error) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	cmd, buf := newTestSetupCmd()
	args := append([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"}, flags...)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// TestFormatStatus verifies TTY and non-TTY status markers.
func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         StepStatus
		nonInteractive bool
		expected       string
	}{
		{"success_tty", StepSuccess, false, "\u2713"},
		{"warning_tty", StepWarning, false, "!"},
		{"skipped_tty", StepSkipped, false, "-"},
		{"error_tty", StepError, false, "\u2717"},
		{"success_non_interactive", StepSuccess, true, "[OK]"},
		{"warning_non_interactive", StepWarning, true, "[WARN]"},
		{"skipped_non_interactive", StepSkipped, true, "[SKIP]"},
		{"error_non_interactive", StepError, true, "[ERR]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStatus(tt.status, tt.nonInteractive)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- US1 Tests ---

// TestStepDisplayVersion verifies the version step outputs version and Go runtime.
func TestStepDisplayVersion(t *testing.T) {
	step := stepDisplayVersion()

	assert.Equal(t, StepSuccess, step.Status)
	assert.Contains(t, step.Message, version.GetVersion())
	assert.Contains(t, step.Message, runtime.Version())
	assert.Equal(t, "Version display", step.Name)
}

// TestStepDetectPulumi tests Pulumi detection for both found and not-found cases.
func TestStepDetectPulumi(t *testing.T) {
	t.Run("pulumi_not_found", func(t *testing.T) {
		// Ensure pulumi is not on a contrived PATH
		t.Setenv("PATH", t.TempDir())
		step := stepDetectPulumi(t.Context())

		assert.Equal(t, StepWarning, step.Status)
		assert.Contains(t, step.Message, "Pulumi CLI not found")
		assert.Contains(t, step.Message, "pulumi.com")
	})

	// Note: testing the "found" case requires pulumi on PATH, which is
	// environment-dependent. The integration test in TestSetupFullRun
	// covers this path when pulumi is available.
}

// TestStepCreateDirectories verifies directory creation on a clean system.
func TestStepCreateDirectories(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "finfocus")
	t.Setenv("FINFOCUS_HOME", tmpDir)

	steps := stepCreateDirectories()

	require.Len(t, steps, 4, "expected 4 directory steps (base, plugins, cache, logs)")

	for _, step := range steps {
		assert.Equal(t, StepSuccess, step.Status, "step %q should succeed", step.Name)
		assert.True(t, step.Critical, "directory steps should be critical")
		assert.Contains(t, step.Message, "Created")
	}

	// Verify directories actually exist
	assert.DirExists(t, tmpDir)
	assert.DirExists(t, filepath.Join(tmpDir, "plugins"))
	assert.DirExists(t, filepath.Join(tmpDir, "cache"))
	assert.DirExists(t, filepath.Join(tmpDir, "logs"))

	// Verify permissions on Unix
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(tmpDir, "plugins"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(dirPermPlugins), info.Mode().Perm(), "plugins dir should be 0750")
	}
}

// TestStepCreateDirectories_AlreadyExist verifies idempotent directory handling.
func TestStepCreateDirectories_AlreadyExist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Pre-create directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "plugins"), dirPermPlugins))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cache"), dirPermBase))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "logs"), dirPermBase))

	steps := stepCreateDirectories()

	require.Len(t, steps, 4)
	for _, step := range steps {
		assert.Equal(t, StepSuccess, step.Status, "existing dirs should report success, not error")
		assert.Contains(t, step.Message, "exists", "should report directory already exists")
	}
}

// TestStepInitConfig verifies config creation when no config exists.
func TestStepInitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Ensure the config directory exists (setup would have created it)
	require.NoError(t, os.MkdirAll(tmpDir, dirPermBase))

	step := stepInitConfig()

	assert.Equal(t, StepSuccess, step.Status)
	assert.True(t, step.Critical)
	assert.Contains(t, step.Message, "Initialized config")

	// Verify the config file was created
	configPath := filepath.Join(tmpDir, "config.yaml")
	assert.FileExists(t, configPath)
}

// TestStepInitConfig_AlreadyExists verifies config is not overwritten.
func TestStepInitConfig_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Create a custom config
	configPath := filepath.Join(tmpDir, "config.yaml")
	customContent := []byte("custom: true\n")
	require.NoError(t, os.WriteFile(configPath, customContent, 0o600))

	step := stepInitConfig()

	assert.Equal(t, StepSuccess, step.Status)
	assert.Contains(t, step.Message, "already exists")

	// Verify the original content is preserved
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, customContent, data, "existing config should not be overwritten")
}

// TestStepInstallAnalyzer tests the analyzer installation step.
// Note: This test may produce warnings if the Pulumi plugin directory
// cannot be resolved, which is expected in test environments.
func TestStepInstallAnalyzer(t *testing.T) {
	step := stepInstallAnalyzer(t.Context())

	// In test environments, the result depends on whether Pulumi is installed.
	// We just verify the step doesn't panic and returns a valid result.
	assert.NotEmpty(t, step.Name)
	assert.NotEmpty(t, step.Message)
	assert.False(t, step.Critical, "analyzer install should not be critical")
}

// TestStepInstallPlugins tests the plugin installation step.
// In test environments, this will likely warn about network issues.
func TestStepInstallPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "plugins"), dirPermPlugins))

	cmd, _ := newTestSetupCmd()
	steps := stepInstallPlugins(cmd)

	require.NotEmpty(t, steps, "should have at least one plugin result")
	for _, step := range steps {
		assert.NotEmpty(t, step.Name)
		assert.NotEmpty(t, step.Message)
		assert.False(t, step.Critical, "plugin install should not be critical")
	}
}

// TestStepInstallPlugins_AlreadyInstalled tests when plugins are already present.
func TestStepInstallPlugins_AlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Pre-create the plugin directory structure
	pluginDir := filepath.Join(tmpDir, "plugins", "aws-public", "v0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, dirPermPlugins))

	cmd, _ := newTestSetupCmd()
	steps := stepInstallPlugins(cmd)

	require.Len(t, steps, 1)
	assert.Equal(t, StepSuccess, steps[0].Status)
	assert.Contains(t, steps[0].Message, "already installed")
}

// --- US2 Idempotency Tests ---

// TestSetupIdempotency runs setup twice and verifies no errors on re-run.
func TestSetupIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// First run — creates everything
	cmd1, buf1 := newTestSetupCmd()
	cmd1.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd1.Execute()
	require.NoError(t, err, "first setup run should succeed")
	output1 := buf1.String()
	assert.Contains(t, output1, "Setup complete!")

	// Capture config content after first run
	configPath := filepath.Join(tmpDir, "config.yaml")
	configData1, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Second run — should succeed without errors
	cmd2, buf2 := newTestSetupCmd()
	cmd2.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err = cmd2.Execute()
	require.NoError(t, err, "second setup run should succeed (idempotent)")
	output2 := buf2.String()
	assert.Contains(t, output2, "Setup complete!")
	assert.Contains(t, output2, "exists", "second run should detect existing directories")
	assert.Contains(t, output2, "already exists", "second run should detect existing config")

	// Verify config was not modified
	configData2, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configData1, configData2, "config should not be modified on re-run")
}

// --- US3 Non-Interactive Tests ---

// TestSetupNonInteractive verifies ASCII output markers in non-interactive mode.
func TestSetupNonInteractive(t *testing.T) {
	output, err := runTestSetup(t)
	require.NoError(t, err)

	assert.Contains(t, output, "[OK]", "non-interactive mode should use [OK] markers")
	assert.NotContains(t, output, "\u2713", "non-interactive mode should not use unicode checkmarks")
}

// --- US4 Skip Flag Tests ---

// TestSetupSkipAnalyzer verifies the --skip-analyzer flag.
func TestSetupSkipAnalyzer(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[SKIP]", "should show skip marker for analyzer")
	assert.Contains(t, output, "Skipped analyzer installation")
}

// TestSetupSkipPlugins verifies the --skip-plugins flag.
func TestSetupSkipPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-plugins", "--skip-analyzer"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Skipped plugin installation")
}

// TestSetupCombinedSkipFlags verifies both skip flags together.
func TestSetupCombinedSkipFlags(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should have directory and config steps but skip analyzer and plugins
	assert.Contains(t, output, "Skipped analyzer installation")
	assert.Contains(t, output, "Skipped plugin installation")
	// Directories and config should still be created
	assert.DirExists(t, tmpDir)
	assert.DirExists(t, filepath.Join(tmpDir, "plugins"))
	assert.FileExists(t, filepath.Join(tmpDir, "config.yaml"))
}

// --- US5 Custom Home Directory Tests ---

// TestSetupFinfocusHome verifies FINFOCUS_HOME override.
func TestSetupFinfocusHome(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", customDir)
	// Ensure PULUMI_HOME doesn't interfere
	t.Setenv("PULUMI_HOME", "")

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, customDir, "output should reference the custom directory")

	// Verify directories created under custom path
	assert.DirExists(t, filepath.Join(customDir, "plugins"))
	assert.DirExists(t, filepath.Join(customDir, "cache"))
	assert.DirExists(t, filepath.Join(customDir, "logs"))
	assert.FileExists(t, filepath.Join(customDir, "config.yaml"))
}

// TestSetupPulumiHome verifies PULUMI_HOME fallback (when FINFOCUS_HOME is not set).
func TestSetupPulumiHome(t *testing.T) {
	pulumiDir := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiDir)
	t.Setenv("FINFOCUS_HOME", "")

	expectedDir := filepath.Join(pulumiDir, "finfocus")

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, expectedDir, "output should reference PULUMI_HOME/finfocus")

	// Verify directories created under PULUMI_HOME/finfocus
	assert.DirExists(t, filepath.Join(expectedDir, "plugins"))
	assert.DirExists(t, filepath.Join(expectedDir, "cache"))
}

// --- Edge Case Tests ---

// TestSetupPartialExisting tests setup when some dirs exist but config is missing.
func TestSetupPartialExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Pre-create only the plugins directory
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "plugins"), dirPermPlugins))

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Some dirs exist, some are created, config is new
	assert.Contains(t, output, "Setup complete!")

	// All expected resources should exist
	assert.DirExists(t, filepath.Join(tmpDir, "cache"))
	assert.DirExists(t, filepath.Join(tmpDir, "logs"))
	assert.FileExists(t, filepath.Join(tmpDir, "config.yaml"))
}

// TestSetupExitCodeWithWarnings verifies exit 0 when only warnings occur.
func TestSetupExitCodeWithWarnings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	// Force Pulumi to not be found to generate a warning
	t.Setenv("PATH", tmpDir)

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err, "warnings should not cause non-zero exit")

	output := buf.String()
	assert.Contains(t, output, "[WARN]", "should show warning for missing Pulumi")
	assert.Contains(t, output, "Setup complete!", "should still show success message")
}

// TestSetupExitCodeWithCriticalFailure verifies exit 1 when a critical step fails.
func TestSetupExitCodeWithCriticalFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}

	// Create a read-only directory that prevents subdirectory creation
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o700)
	})

	t.Setenv("FINFOCUS_HOME", filepath.Join(readOnlyDir, "finfocus"))

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.Error(t, err, "critical failure should produce error")

	output := buf.String()
	assert.Contains(t, output, "[ERR]", "should show error marker")
	assert.Contains(t, output, "errors", "should report errors in summary")
}

// TestSetupFullRun tests the complete setup flow with skip flags.
func TestSetupFullRun(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	cmd, buf := newTestSetupCmd()
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should contain version info
	assert.Contains(t, output, "FinFocus v")
	assert.Contains(t, output, runtime.Version())

	// Should contain directory creation
	assert.True(t,
		strings.Contains(output, "Created") || strings.Contains(output, "exists"),
		"output should mention directory creation or existence")

	// Should contain config
	assert.True(t,
		strings.Contains(output, "Initialized config") || strings.Contains(output, "already exists"),
		"output should mention config initialization")

	// Should contain skip markers
	assert.Contains(t, output, "Skipped analyzer installation")
	assert.Contains(t, output, "Skipped plugin installation")

	// Should end with summary
	assert.Contains(t, output, "Setup complete!")
}
