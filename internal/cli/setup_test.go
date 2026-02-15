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
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/pkg/version"
)

// newTestSetupCmd creates a testable setup command with captured output.
// It returns the command and a buffer that receives all output.
func newTestSetupCmd() (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := cli.NewSetupCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Silence usage on error to keep test output clean
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd, buf
}

// newTestSetupCmdWithRunner creates a testable setup command using a custom runner.
func newTestSetupCmdWithRunner(runner *cli.SetupRunner) (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := cli.NewSetupCmdWithRunner(runner)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
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
		status         cli.StepStatus
		nonInteractive bool
		expected       string
	}{
		{"success_tty", cli.StepSuccess, false, "\u2713"},
		{"warning_tty", cli.StepWarning, false, "!"},
		{"skipped_tty", cli.StepSkipped, false, "-"},
		{"error_tty", cli.StepError, false, "\u2717"},
		{"success_non_interactive", cli.StepSuccess, true, "[OK]"},
		{"warning_non_interactive", cli.StepWarning, true, "[WARN]"},
		{"skipped_non_interactive", cli.StepSkipped, true, "[SKIP]"},
		{"error_non_interactive", cli.StepError, true, "[ERR]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cli.FormatStatus(tt.status, tt.nonInteractive)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- StepStatus Tests ---

// TestStepStatus_String verifies human-readable labels for all StepStatus values.
func TestStepStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   cli.StepStatus
		expected string
	}{
		{"success", cli.StepSuccess, "success"},
		{"warning", cli.StepWarning, "warning"},
		{"skipped", cli.StepSkipped, "skipped"},
		{"error", cli.StepError, "error"},
		{"unknown", cli.StepStatus(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

// TestStepStatus_MarshalJSON verifies JSON string output for StepStatus.
func TestStepStatus_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		status   cli.StepStatus
		expected string
	}{
		{"success", cli.StepSuccess, `"success"`},
		{"warning", cli.StepWarning, `"warning"`},
		{"skipped", cli.StepSkipped, `"skipped"`},
		{"error", cli.StepError, `"error"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

// TestStepStatus_UnmarshalJSON verifies round-trip and error handling.
func TestStepStatus_UnmarshalJSON(t *testing.T) {
	t.Run("round_trip", func(t *testing.T) {
		statuses := []cli.StepStatus{
			cli.StepSuccess,
			cli.StepWarning,
			cli.StepSkipped,
			cli.StepError,
		}
		for _, original := range statuses {
			data, err := json.Marshal(original)
			require.NoError(t, err)

			var parsed cli.StepStatus
			err = json.Unmarshal(data, &parsed)
			require.NoError(t, err)
			assert.Equal(t, original, parsed)
		}
	})

	t.Run("invalid_value", func(t *testing.T) {
		var s cli.StepStatus
		err := json.Unmarshal([]byte(`"bogus"`), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown step status")
	})

	t.Run("invalid_json", func(t *testing.T) {
		var s cli.StepStatus
		err := json.Unmarshal([]byte(`123`), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing step status")
	})
}

// TestStepStatus_StepResult_JSON verifies StepResult serializes status as string.
func TestStepStatus_StepResult_JSON(t *testing.T) {
	sr := cli.StepResult{
		Name:    "test step",
		Status:  cli.StepSuccess,
		Message: "all good",
	}
	data, err := json.Marshal(sr)
	require.NoError(t, err)

	// Status should be a string, not an integer
	assert.Contains(t, string(data), `"status":"success"`)
	assert.NotContains(t, string(data), `"status":0`)
}

// --- US1 Tests ---

// TestStepDisplayVersion verifies the version step outputs version and Go runtime.
func TestStepDisplayVersion(t *testing.T) {
	step := cli.StepDisplayVersion()

	assert.Equal(t, cli.StepSuccess, step.Status)
	assert.Contains(t, step.Message, version.GetVersion())
	assert.Contains(t, step.Message, runtime.Version())
	assert.Equal(t, "Version display", step.Name)
}

// TestStepDetectPulumi tests Pulumi detection for both found and not-found cases.
func TestStepDetectPulumi(t *testing.T) {
	t.Run("pulumi_not_found", func(t *testing.T) {
		// Ensure pulumi is not on a contrived PATH
		t.Setenv("PATH", t.TempDir())
		step := cli.StepDetectPulumi(t.Context())

		assert.Equal(t, cli.StepWarning, step.Status)
		assert.Contains(t, step.Message, "Pulumi CLI not found")
		assert.Contains(t, step.Message, "pulumi.com")
	})

	t.Run("pulumi_found", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a fake pulumi script that prints a version string
		var script string
		var name string
		if runtime.GOOS == "windows" {
			name = "pulumi.bat"
			script = "@echo v3.100.0\n"
		} else {
			name = "pulumi"
			script = "#!/bin/sh\necho v3.100.0\n"
		}
		fakePulumi := filepath.Join(tmpDir, name)
		require.NoError(t, os.WriteFile(fakePulumi, []byte(script), 0o755))

		t.Setenv("PATH", tmpDir)
		step := cli.StepDetectPulumi(t.Context())

		assert.Equal(t, cli.StepSuccess, step.Status)
		assert.Contains(t, step.Message, "v3.100.0")
	})
}

// TestStepCreateDirectories verifies directory creation on a clean system.
func TestStepCreateDirectories(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "finfocus")
	t.Setenv("FINFOCUS_HOME", tmpDir)

	steps := cli.StepCreateDirectories(config.ResolveConfigDir())

	require.Len(t, steps, 4, "expected 4 directory steps (base, plugins, cache, logs)")

	for _, step := range steps {
		assert.Equal(t, cli.StepSuccess, step.Status, "step %q should succeed", step.Name)
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
		assert.Equal(t, os.FileMode(cli.DirPermPlugins), info.Mode().Perm(), "plugins dir should be 0750")
	}
}

// TestStepCreateDirectories_AlreadyExist verifies idempotent directory handling.
func TestStepCreateDirectories_AlreadyExist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Pre-create directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "plugins"), cli.DirPermPlugins))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cache"), cli.DirPermBase))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "logs"), cli.DirPermBase))

	steps := cli.StepCreateDirectories(config.ResolveConfigDir())

	require.Len(t, steps, 4)
	for _, step := range steps {
		assert.Equal(t, cli.StepSuccess, step.Status, "existing dirs should report success, not error")
		assert.Contains(t, step.Message, "exists", "should report directory already exists")
	}
}

// TestStepInitConfig verifies config creation when no config exists.
func TestStepInitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Ensure the config directory exists (setup would have created it)
	require.NoError(t, os.MkdirAll(tmpDir, cli.DirPermBase))

	step := cli.StepInitConfig(config.ResolveConfigDir())

	assert.Equal(t, cli.StepSuccess, step.Status)
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

	step := cli.StepInitConfig(config.ResolveConfigDir())

	assert.Equal(t, cli.StepSuccess, step.Status)
	assert.Contains(t, step.Message, "already exists")

	// Verify the original content is preserved
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, customContent, data, "existing config should not be overwritten")
}

// TestStepInstallAnalyzer tests the analyzer installation step with mock installer.
func TestStepInstallAnalyzer(t *testing.T) {
	t.Run("success_installed", func(t *testing.T) {
		runner := &cli.SetupRunner{
			AnalyzerInstaller: cli.AnalyzerInstallerFunc(
				func(_ context.Context, _ analyzer.InstallOptions) (*analyzer.InstallResult, error) {
					return &analyzer.InstallResult{
						Installed: true,
						Version:   "1.0.0",
						Method:    "symlink",
						Action:    analyzer.ActionInstalled,
					}, nil
				},
			),
		}

		step := runner.StepInstallAnalyzer(t.Context())

		assert.Equal(t, cli.StepSuccess, step.Status)
		assert.Contains(t, step.Message, "Installed Pulumi analyzer")
		assert.Contains(t, step.Message, "v1.0.0")
		assert.Contains(t, step.Message, "symlink")
		assert.False(t, step.Critical)
	})

	t.Run("already_current", func(t *testing.T) {
		runner := &cli.SetupRunner{
			AnalyzerInstaller: cli.AnalyzerInstallerFunc(
				func(_ context.Context, _ analyzer.InstallOptions) (*analyzer.InstallResult, error) {
					return &analyzer.InstallResult{
						Installed: true,
						Version:   "1.0.0",
						Action:    analyzer.ActionAlreadyCurrent,
					}, nil
				},
			),
		}

		step := runner.StepInstallAnalyzer(t.Context())

		assert.Equal(t, cli.StepSuccess, step.Status)
		assert.Contains(t, step.Message, "already current")
		assert.Contains(t, step.Message, "v1.0.0")
	})

	t.Run("update_available", func(t *testing.T) {
		runner := &cli.SetupRunner{
			AnalyzerInstaller: cli.AnalyzerInstallerFunc(
				func(_ context.Context, _ analyzer.InstallOptions) (*analyzer.InstallResult, error) {
					return &analyzer.InstallResult{
						Installed:      true,
						Version:        "0.9.0",
						CurrentVersion: "1.0.0",
						Action:         analyzer.ActionUpdateAvailable,
					}, nil
				},
			),
		}

		step := runner.StepInstallAnalyzer(t.Context())

		assert.Equal(t, cli.StepWarning, step.Status)
		assert.Contains(t, step.Message, "v0.9.0")
		assert.Contains(t, step.Message, "v1.0.0")
		assert.Contains(t, step.Message, "update available")
	})

	t.Run("error", func(t *testing.T) {
		runner := &cli.SetupRunner{
			AnalyzerInstaller: cli.AnalyzerInstallerFunc(
				func(_ context.Context, _ analyzer.InstallOptions) (*analyzer.InstallResult, error) {
					return nil, errors.New("plugin dir not found")
				},
			),
		}

		step := runner.StepInstallAnalyzer(t.Context())

		assert.Equal(t, cli.StepWarning, step.Status)
		assert.Contains(t, step.Message, "Failed to install analyzer")
		assert.Contains(t, step.Message, "plugin dir not found")
		assert.NotNil(t, step.Err)
	})
}

// TestStepInstallPlugins tests the plugin installation step with a mock installer.
func TestStepInstallPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "plugins"), cli.DirPermPlugins))

	runner := &cli.SetupRunner{
		PluginInstaller: cli.PluginInstallerFunc(
			func(_ context.Context, _ string, _ registry.InstallOptions, _ func(string)) (*registry.InstallResult, error) {
				return &registry.InstallResult{Name: "aws-public", Version: "v0.1.0"}, nil
			},
		),
	}

	steps := runner.StepInstallPlugins(t.Context(), tmpDir)

	require.NotEmpty(t, steps, "should have at least one plugin result")
	for _, step := range steps {
		assert.Equal(t, cli.StepSuccess, step.Status)
		assert.NotEmpty(t, step.Name)
		assert.NotEmpty(t, step.Message)
		assert.False(t, step.Critical, "plugin install should not be critical")
	}
	assert.Contains(t, steps[0].Message, "aws-public")
}

// TestStepInstallPlugins_AlreadyInstalled tests when plugins are already present.
func TestStepInstallPlugins_AlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Pre-create the plugin directory structure
	pluginDir := filepath.Join(tmpDir, "plugins", "aws-public", "v0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, cli.DirPermPlugins))

	runner := &cli.SetupRunner{}
	steps := runner.StepInstallPlugins(t.Context(), tmpDir)

	require.Len(t, steps, len(cli.DefaultPlugins))
	assert.Equal(t, cli.StepSuccess, steps[0].Status)
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

// TestSetupSkipAnalyzer verifies the --skip-analyzer flag in isolation.
func TestSetupSkipAnalyzer(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Mock both installers via SetupRunner
	runner := &cli.SetupRunner{
		AnalyzerInstaller: cli.AnalyzerInstallerFunc(
			func(_ context.Context, _ analyzer.InstallOptions) (*analyzer.InstallResult, error) {
				return &analyzer.InstallResult{Action: analyzer.ActionAlreadyCurrent, Version: "test"}, nil
			},
		),
		PluginInstaller: cli.PluginInstallerFunc(
			func(_ context.Context, specifier string, _ registry.InstallOptions, _ func(string)) (*registry.InstallResult, error) {
				return &registry.InstallResult{Name: specifier, Version: "v0.1.0"}, nil
			},
		),
	}

	cmd, buf := newTestSetupCmdWithRunner(runner)
	// Only skip analyzer — plugins still run (both mocked)
	cmd.SetArgs([]string{"--non-interactive", "--skip-analyzer"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[SKIP]", "should show skip marker for analyzer")
	assert.Contains(t, output, "Skipped analyzer installation")
	assert.NotContains(t, output, "Skipped plugin installation", "plugins should not be skipped")
	// Directories and config should still be created
	assert.DirExists(t, filepath.Join(tmpDir, "plugins"))
	assert.FileExists(t, filepath.Join(tmpDir, "config.yaml"))
}

// TestSetupSkipPlugins verifies the --skip-plugins flag in isolation.
func TestSetupSkipPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)

	// Mock analyzer so it doesn't depend on real Pulumi
	runner := &cli.SetupRunner{
		AnalyzerInstaller: cli.AnalyzerInstallerFunc(
			func(_ context.Context, _ analyzer.InstallOptions) (*analyzer.InstallResult, error) {
				return &analyzer.InstallResult{Action: analyzer.ActionAlreadyCurrent, Version: "test"}, nil
			},
		),
	}

	cmd, buf := newTestSetupCmdWithRunner(runner)
	// Only skip plugins — analyzer still runs (mocked)
	cmd.SetArgs([]string{"--non-interactive", "--skip-plugins"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Skipped plugin installation")
	assert.NotContains(t, output, "Skipped analyzer installation", "analyzer should not be skipped")
	assert.Contains(t, output, "Pulumi analyzer already current", "mocked analyzer should run and report status")
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
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "plugins"), cli.DirPermPlugins))

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
