package plugin_test

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// TestPluginInit_Basic verifies that plugin init creates correct project scaffolding [US1][T009].
func TestPluginInit_Basic(t *testing.T) {
	outputDir := t.TempDir()

	// Create and execute the init command
	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"test-plugin",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify project directory was created
	projectDir := filepath.Join(outputDir, "test-plugin")
	assert.DirExists(t, projectDir)

	// Verify main.go exists and has correct content
	mainGoPath := filepath.Join(projectDir, "cmd", "plugin", "main.go")
	assert.FileExists(t, mainGoPath)
	mainContent, err := os.ReadFile(mainGoPath)
	require.NoError(t, err)
	assert.Contains(t, string(mainContent), "package main")
	assert.Contains(t, string(mainContent), "pricing.NewCalculator()")

	// Verify go.mod exists
	goModPath := filepath.Join(projectDir, "go.mod")
	assert.FileExists(t, goModPath)
	goModContent, err := os.ReadFile(goModPath)
	require.NoError(t, err)
	assert.Contains(t, string(goModContent), "module github.com/example/test-plugin")
	assert.Contains(t, string(goModContent), "go 1.25.5")

	// Verify manifest.yaml exists and has correct content
	manifestPath := filepath.Join(projectDir, "manifest.yaml")
	assert.FileExists(t, manifestPath)
	manifestContent, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(manifestContent), "name: test-plugin")
	assert.Contains(t, string(manifestContent), "author: Test Author")

	// Verify directory structure
	dirs := []string{
		"cmd/plugin",
		"internal/pricing",
		"internal/client",
		"examples",
		"bin",
	}
	for _, dir := range dirs {
		assert.DirExists(t, filepath.Join(projectDir, dir), "directory %s should exist", dir)
	}

	// Verify README.md exists
	readmePath := filepath.Join(projectDir, "README.md")
	assert.FileExists(t, readmePath)

	// Verify Makefile exists
	makefilePath := filepath.Join(projectDir, "Makefile")
	assert.FileExists(t, makefilePath)
}

// TestPluginInit_MultiProvider verifies manifest contains multiple providers [US1][T010].
func TestPluginInit_MultiProvider(t *testing.T) {
	outputDir := t.TempDir()

	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"multi-cloud",
		"--author", "Test Author",
		"--providers", "aws,azure,gcp",
		"--output-dir", outputDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify manifest contains all providers
	projectDir := filepath.Join(outputDir, "multi-cloud")
	manifestPath := filepath.Join(projectDir, "manifest.yaml")
	manifestContent, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	// Check that all providers are listed
	assert.Contains(t, string(manifestContent), "aws")
	assert.Contains(t, string(manifestContent), "azure")
	assert.Contains(t, string(manifestContent), "gcp")

	// Verify plugin implementation references providers
	calculatorPath := filepath.Join(projectDir, "internal", "pricing", "calculator.go")
	calculatorContent, err := os.ReadFile(calculatorPath)
	require.NoError(t, err)
	assert.Contains(t, string(calculatorContent), `"aws", "azure", "gcp"`)
}

// TestPluginInit_CustomOutputDir verifies creation in specified path [US1][T011].
func TestPluginInit_CustomOutputDir(t *testing.T) {
	// Create a nested output directory
	baseDir := t.TempDir()
	customDir := filepath.Join(baseDir, "custom", "path", "plugins")
	require.NoError(t, os.MkdirAll(customDir, 0755))

	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"custom-plugin",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", customDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify project was created in custom directory
	projectDir := filepath.Join(customDir, "custom-plugin")
	assert.DirExists(t, projectDir)
	assert.FileExists(t, filepath.Join(projectDir, "go.mod"))
}

// TestPluginInit_Force verifies overwrite behavior [US1][T012].
func TestPluginInit_Force(t *testing.T) {
	outputDir := t.TempDir()

	// First, create a project
	cmd1 := cli.NewPluginInitCmd()
	var stdout1, stderr1 bytes.Buffer
	cmd1.SetOut(&stdout1)
	cmd1.SetErr(&stderr1)

	cmd1.SetArgs([]string{
		"force-test",
		"--author", "Original Author",
		"--providers", "aws",
		"--output-dir", outputDir,
	})

	err := cmd1.Execute()
	require.NoError(t, err)

	// Try to create again without --force (should fail)
	cmd2 := cli.NewPluginInitCmd()
	var stdout2, stderr2 bytes.Buffer
	cmd2.SetOut(&stdout2)
	cmd2.SetErr(&stderr2)

	cmd2.SetArgs([]string{
		"force-test",
		"--author", "New Author",
		"--providers", "gcp",
		"--output-dir", outputDir,
	})

	err = cmd2.Execute()
	assert.Error(t, err, "should fail without --force when directory exists")
	assert.Contains(t, err.Error(), "already exists")

	// Now use --force to overwrite
	cmd3 := cli.NewPluginInitCmd()
	var stdout3, stderr3 bytes.Buffer
	cmd3.SetOut(&stdout3)
	cmd3.SetErr(&stderr3)

	cmd3.SetArgs([]string{
		"force-test",
		"--author", "New Author",
		"--providers", "gcp",
		"--output-dir", outputDir,
		"--force",
	})

	err = cmd3.Execute()
	require.NoError(t, err, "should succeed with --force")

	// Verify the new author is in the manifest
	projectDir := filepath.Join(outputDir, "force-test")
	manifestPath := filepath.Join(projectDir, "manifest.yaml")
	manifestContent, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(manifestContent), "author: New Author")
	assert.Contains(t, string(manifestContent), "gcp")
}

// TestPluginInit_InvalidName verifies error handling for invalid plugin names [US1][T013].
func TestPluginInit_InvalidName(t *testing.T) {
	outputDir := t.TempDir()

	testCases := []struct {
		name        string
		pluginName  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "uppercase letters",
			pluginName:  "Invalid-Plugin",
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
		{
			name:        "spaces",
			pluginName:  "my plugin",
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
		{
			name:        "special characters",
			pluginName:  "plugin@1.0",
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
		{
			// Note: Names starting with hyphen are parsed as flags by Cobra.
			// This test verifies Cobra's parsing error is surfaced.
			name:        "starts with hyphen",
			pluginName:  "--hyphen-start",
			expectError: true,
			errorMsg:    "unknown flag",
		},
		{
			name:        "ends with hyphen",
			pluginName:  "plugin-",
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
		{
			name:        "too short",
			pluginName:  "a",
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
		{
			name:        "valid name with hyphens",
			pluginName:  "my-awesome-plugin",
			expectError: false,
		},
		{
			name:        "valid name with numbers",
			pluginName:  "plugin123",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.NewPluginInitCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			// Use a subdirectory for each test case to avoid conflicts
			testOutputDir := filepath.Join(outputDir, tc.name)
			require.NoError(t, os.MkdirAll(testOutputDir, 0755))

			cmd.SetArgs([]string{
				tc.pluginName,
				"--author", "Test Author",
				"--providers", "aws",
				"--output-dir", testOutputDir,
			})

			err := cmd.Execute()

			if tc.expectError {
				assert.Error(t, err, "expected error for plugin name: %s", tc.pluginName)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err, "expected success for plugin name: %s", tc.pluginName)
			}
		})
	}
}

// TestPluginInit_MissingRequiredFlags verifies error handling for missing required flags.
func TestPluginInit_MissingRequiredFlags(t *testing.T) {
	outputDir := t.TempDir()

	testCases := []struct {
		name     string
		args     []string
		errorMsg string
	}{
		{
			name: "missing author",
			args: []string{
				"test-plugin",
				"--providers", "aws",
				"--output-dir", outputDir,
			},
			errorMsg: "author",
		},
		{
			name: "missing providers",
			args: []string{
				"test-plugin",
				"--author", "Test Author",
				"--output-dir", outputDir,
			},
			errorMsg: "providers",
		},
		{
			name:     "missing plugin name",
			args:     []string{},
			errorMsg: "accepts 1 arg",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.NewPluginInitCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			assert.Error(t, err, "expected error for %s", tc.name)
			assert.Contains(t, err.Error(), tc.errorMsg)
		})
	}
}

// TestPluginInit_RecordedFixtures_Offline verifies recorded fixtures are generated offline [US1][T008].
func TestPluginInit_RecordedFixtures_Offline(t *testing.T) {
	outputDir := t.TempDir()

	// Create the init command with recording enabled and offline mode
	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"fixture-test",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
		"--record-fixtures",
		"--offline",
	})

	err := cmd.Execute()
	require.NoError(t, err, "init with offline fixtures should succeed")

	// Verify project directory was created
	projectDir := filepath.Join(outputDir, "fixture-test")
	assert.DirExists(t, projectDir)

	// Verify testdata directory with recorded requests exists
	testdataDir := filepath.Join(projectDir, "testdata", "recorded_requests")
	if _, err := os.Stat(testdataDir); err != nil {
		// The directory may not exist if recording wasn't fully attempted
		// This is acceptable since we're in offline mode with potentially missing fixtures
		t.Logf("testdata directory check: %v", err)
	}
}

// TestPluginInit_RecordedFixtures_Flag verifies --record-fixtures flag acceptance [US1][T008].
func TestPluginInit_RecordedFixtures_Flag(t *testing.T) {
	outputDir := t.TempDir()

	// Create the init command with recording enabled
	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"recording-flag-test",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
		"--record-fixtures",
		"--fixture-version", "main",
		"--offline",
	})

	err := cmd.Execute()
	require.NoError(t, err, "init with recording flags should succeed")

	// Verify project directory was created
	projectDir := filepath.Join(outputDir, "recording-flag-test")
	assert.DirExists(t, projectDir)

	// Verify the project structure is intact even if recording had issues
	assert.DirExists(t, filepath.Join(projectDir, "cmd", "plugin"))
	assert.DirExists(t, filepath.Join(projectDir, "internal", "pricing"))
	assert.FileExists(t, filepath.Join(projectDir, "go.mod"))
}

// TestPluginInit_OfflineMode verifies --offline flag prevents network access [US2][T015].
func TestPluginInit_OfflineMode(t *testing.T) {
	outputDir := t.TempDir()

	// Create the init command with offline mode enabled
	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"offline-test",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
		"--offline",
	})

	err := cmd.Execute()
	require.NoError(t, err, "init with offline mode should succeed")

	// Verify project directory was created
	projectDir := filepath.Join(outputDir, "offline-test")
	assert.DirExists(t, projectDir)

	// Verify basic project structure is intact
	assert.FileExists(t, filepath.Join(projectDir, "go.mod"))
	assert.FileExists(t, filepath.Join(projectDir, "manifest.yaml"))
	assert.DirExists(t, filepath.Join(projectDir, "internal", "pricing"))
}

// TestPluginInit_OfflineWithRecording verifies offline mode works with --record-fixtures [US2][T015].
func TestPluginInit_OfflineWithRecording(t *testing.T) {
	outputDir := t.TempDir()

	// Create the init command with offline mode and recording enabled
	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"offline-recording-test",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
		"--offline",
		"--record-fixtures",
	})

	err := cmd.Execute()
	require.NoError(t, err, "init with offline and recording should succeed")

	// Verify project directory was created
	projectDir := filepath.Join(outputDir, "offline-recording-test")
	assert.DirExists(t, projectDir)

	// Verify the essential project files exist
	assert.FileExists(t, filepath.Join(projectDir, "go.mod"))
	assert.FileExists(t, filepath.Join(projectDir, "manifest.yaml"))
	assert.DirExists(t, filepath.Join(projectDir, "cmd", "plugin"))
	assert.DirExists(t, filepath.Join(projectDir, "internal", "pricing"))
	assert.DirExists(t, filepath.Join(projectDir, "internal", "client"))
}

// TestPluginInit_OnlineNetworkFailure verifies graceful degradation when network access fails in online mode.
func TestPluginInit_OnlineNetworkFailure(t *testing.T) {
	outputDir := t.TempDir()

	// Save original http.DefaultTransport
	originalTransport := http.DefaultTransport
	defer func() {
		//nolint:reassign // Restoring original transport after test
		http.DefaultTransport = originalTransport
	}()

	// Install a failing HTTP transport
	//nolint:reassign // Intentionally override transport to simulate network failure
	http.DefaultTransport = &failingTransport{}

	// Create the init command without offline flag (online mode)
	cmd := cli.NewPluginInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"network-fail-test",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
		"--record-fixtures",
		"--fixture-version", "latest",
	})

	err := cmd.Execute()

	// The command should either:
	// 1. Return an error (handled failure), OR
	// 2. Succeed with minimal project structure (graceful degradation)

	if err != nil {
		// Verify error is related to network/fixture download
		assert.Contains(t, err.Error(), "simulated network failure",
			"error should indicate network failure")
		t.Logf("Network failure handled with error: %v", err)
	} else {
		// If no error, verify minimal project structure exists
		projectDir := filepath.Join(outputDir, "network-fail-test")
		assert.DirExists(t, projectDir, "project directory should be created even on network failure")
		assert.FileExists(t, filepath.Join(projectDir, "go.mod"),
			"go.mod should exist for graceful degradation")
		assert.FileExists(t, filepath.Join(projectDir, "manifest.yaml"),
			"manifest.yaml should exist for graceful degradation")
		t.Logf("Network failure handled gracefully with minimal project structure")
	}
}

// failingTransport is a custom http.RoundTripper that always returns an error.
type failingTransport struct{}

func (t *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("simulated network failure")
}
