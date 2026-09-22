package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestPluginInitCommand(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cmd := cli.NewPluginInitCmd()

	if cmd.Use != "init <plugin-name>" {
		t.Errorf("Expected Use 'init <plugin-name>', got %s", cmd.Use)
	}

	if cmd.Short != "Initialize a new plugin development project" {
		t.Errorf("Expected short description about initializing plugin project, got %s", cmd.Short)
	}
}

func TestPluginInitValidation(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	testCases := []struct {
		name        string
		args        []string
		opts        cli.PluginInitOptions
		wantErr     bool
		errContains string
	}{
		{
			name: "valid plugin name",
			args: []string{"aws-plugin"},
			opts: cli.PluginInitOptions{
				Author:    "Test Author",
				Providers: []string{"aws"},
			},
			wantErr: false,
		},
		{
			name: "invalid plugin name with uppercase",
			args: []string{"AWS-Plugin"},
			opts: cli.PluginInitOptions{
				Author:    "Test Author",
				Providers: []string{"aws"},
			},
			wantErr:     true,
			errContains: "invalid plugin name",
		},
		{
			name: "invalid plugin name with underscore",
			args: []string{"aws_plugin"},
			opts: cli.PluginInitOptions{
				Author:    "Test Author",
				Providers: []string{"aws"},
			},
			wantErr:     true,
			errContains: "invalid plugin name",
		},
		{
			name: "empty providers",
			args: []string{"aws-plugin"},
			opts: cli.PluginInitOptions{
				Author:    "Test Author",
				Providers: []string{},
			},
			wantErr:     true,
			errContains: "providers",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tc.opts.OutputDir = tmpDir
			tc.opts.Name = tc.args[0]
			tc.opts.Force = true

			// Create command via full root tree
			root := cli.NewRootCmd("test")
			cmdArgs := []string{
				"plugin", "init",
				tc.args[0],
				"--author", tc.opts.Author,
				"--output-dir", tmpDir,
				"--force",
			}
			if len(tc.opts.Providers) > 0 {
				cmdArgs = append(cmdArgs, "--providers", tc.opts.Providers[0])
			}

			result := axtest.Run(context.Background(), t, root, cmdArgs)

			if tc.wantErr {
				assert.NotZero(t, result.ExitCode)
				assert.Contains(t, string(result.Stderr), tc.errContains)
			} else {
				assert.Zero(t, result.ExitCode, string(result.Stderr))
			}
		})
	}
}

func TestPluginInitProjectGeneration(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:      "test-plugin",
		Author:    "Test Author",
		Providers: []string{"aws", "azure"},
		OutputDir: tmpDir,
		Force:     true,
	}

	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cli.RunPluginInit(cmd.Context(), cmd, opts)
		},
	}
	cmd.SetContext(context.Background())

	err := cmd.RunE(cmd, []string{"test-plugin"})
	if err != nil {
		t.Fatalf("Plugin init failed: %v", err)
	}

	// Check that project directory was created
	projectDir := filepath.Join(tmpDir, "test-plugin")
	if _, statErr := os.Stat(projectDir); os.IsNotExist(statErr) {
		t.Errorf("Project directory was not created: %s", projectDir)
	}

	// Check key files exist
	expectedFiles := []string{
		"go.mod",
		"manifest.yaml",
		"cmd/plugin/main.go",
		"internal/pricing/calculator.go",
		"internal/pricing/data.go",
		"internal/client/client.go",
		"Makefile",
		"README.md",
		"internal/pricing/calculator_test.go",
	}

	for _, file := range expectedFiles {
		fullPath := filepath.Join(projectDir, file)
		if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
			t.Errorf("Expected file was not created: %s", file)
		}
	}

	// Check directories exist
	expectedDirs := []string{
		"cmd/plugin",
		"internal/pricing",
		"internal/client",
		"examples",
		"bin",
	}

	for _, dir := range expectedDirs {
		fullPath := filepath.Join(projectDir, dir)
		if info, statErr := os.Stat(fullPath); os.IsNotExist(statErr) || !info.IsDir() {
			t.Errorf("Expected directory was not created: %s", dir)
		}
	}
}

func TestPluginInitForceOverwrite(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-plugin")

	// Create existing directory
	err := os.MkdirAll(projectDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	opts := &cli.PluginInitOptions{
		Name:      "test-plugin",
		Author:    "Test Author",
		Providers: []string{"aws"},
		OutputDir: tmpDir,
		Force:     false, // Don't force overwrite
	}

	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cli.RunPluginInit(cmd.Context(), cmd, opts)
		},
	}
	cmd.SetContext(context.Background())

	// Should fail without force
	err = cmd.RunE(cmd, []string{"test-plugin"})
	if err == nil {
		t.Errorf("Expected error when directory exists and force=false, got none")
	}

	// Should succeed with force
	opts.Force = true
	err = cmd.RunE(cmd, []string{"test-plugin"})
	if err != nil {
		t.Errorf("Expected success with force=true, got error: %v", err)
	}
}

func TestIsValidPluginName(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple name", "aws", true},
		{"valid hyphenated name", "aws-plugin", true},
		{"valid with numbers", "aws-plugin-v2", true},
		{"too short", "a", false},
		{"starts with hyphen", "-aws", false},
		{"ends with hyphen", "aws-", false},
		{"contains uppercase", "AWS", false},
		{"contains underscore", "aws_plugin", false},
		{"contains space", "aws plugin", false},
		{"contains dot", "aws.plugin", false},
		{
			"too long",
			"this-is-a-very-long-plugin-name-that-exceeds-the-maximum-allowed-length",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cli.IsValidPluginName(tc.input)
			if result != tc.expected {
				t.Errorf("isValidPluginName(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func runPluginInitForTest(t *testing.T, opts *cli.PluginInitOptions) {
	t.Helper()
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cli.RunPluginInit(cmd.Context(), cmd, opts)
		},
	}
	cmd.SetContext(context.Background())

	err := cmd.RunE(cmd, []string{opts.Name})
	require.NoError(t, err)
}

func TestPluginInitDockerFilesGenerated(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:       "test-plugin",
		Author:     "Test Author",
		Providers:  []string{"aws"},
		OutputDir:  tmpDir,
		Force:      true,
		WithDocker: true,
	}
	runPluginInitForTest(t, opts)

	projectDir := filepath.Join(tmpDir, "test-plugin")

	dockerfile, err := os.ReadFile(filepath.Join(projectDir, "docker", "Dockerfile"))
	require.NoError(t, err)
	assert.Contains(t, string(dockerfile), "FROM golang:")
	assert.Contains(t, string(dockerfile), "AS builder")
	assert.Contains(t, string(dockerfile), "adduser -D -u 65532")
	assert.Contains(t, string(dockerfile), "HEALTHCHECK")
	assert.NotContains(t, string(dockerfile), "{{GO_VERSION}}")

	dockerignore, err := os.ReadFile(filepath.Join(projectDir, ".dockerignore"))
	require.NoError(t, err)
	assert.Contains(t, string(dockerignore), "bin/")
	assert.Contains(t, string(dockerignore), ".git")
}

func TestPluginInitNoDocker(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:      "test-plugin",
		Author:    "Test Author",
		Providers: []string{"aws"},
		OutputDir: tmpDir,
		Force:     true,
		NoDocker:  true,
	}
	runPluginInitForTest(t, opts)

	projectDir := filepath.Join(tmpDir, "test-plugin")

	// Standard files still generated
	_, err := os.Stat(filepath.Join(projectDir, "go.mod"))
	require.NoError(t, err)

	// Docker files skipped
	_, err = os.Stat(filepath.Join(projectDir, "docker", "Dockerfile"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(projectDir, ".dockerignore"))
	assert.True(t, os.IsNotExist(err))
}

func TestPluginInitDockerOnly(t *testing.T) {
	tmpDir := t.TempDir()
	// Pre-existing project directory (docker-only targets existing projects)
	projectDir := filepath.Join(tmpDir, "test-plugin")
	require.NoError(t, os.MkdirAll(projectDir, 0o750))

	opts := &cli.PluginInitOptions{
		Name:       "test-plugin",
		Author:     "Test Author",
		Providers:  []string{"aws"},
		OutputDir:  tmpDir,
		DockerOnly: true,
	}
	runPluginInitForTest(t, opts)

	// Docker files generated
	_, err := os.Stat(filepath.Join(projectDir, "docker", "Dockerfile"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(projectDir, ".dockerignore"))
	require.NoError(t, err)

	// Standard scaffolding skipped
	_, err = os.Stat(filepath.Join(projectDir, "go.mod"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(projectDir, "cmd", "plugin", "main.go"))
	assert.True(t, os.IsNotExist(err))
}

func TestPluginInitShouldGenerateFlags(t *testing.T) {
	testCases := []struct {
		name       string
		opts       cli.PluginInitOptions
		wantDocker bool
		wantDocs   bool
		wantHealth bool
	}{
		{
			name:       "full generation by default",
			opts:       cli.PluginInitOptions{WithDocker: true, WithDocs: true, WithHealth: true},
			wantDocker: true,
			wantDocs:   true,
			wantHealth: true,
		},
		{
			name:       "with-docker=false skips docker",
			opts:       cli.PluginInitOptions{WithDocs: true, WithHealth: true},
			wantDocker: false,
			wantDocs:   true,
			wantHealth: true,
		},
		{
			name:       "no-docker alias skips docker",
			opts:       cli.PluginInitOptions{WithDocker: true, WithDocs: true, WithHealth: true, NoDocker: true},
			wantDocker: false,
			wantDocs:   true,
			wantHealth: true,
		},
		{
			name:       "no-docs alias skips docs",
			opts:       cli.PluginInitOptions{WithDocker: true, WithDocs: true, WithHealth: true, NoDocs: true},
			wantDocker: true,
			wantDocs:   false,
			wantHealth: true,
		},
		{
			name:       "no-health alias skips health",
			opts:       cli.PluginInitOptions{WithDocker: true, WithDocs: true, WithHealth: true, NoHealth: true},
			wantDocker: true,
			wantDocs:   true,
			wantHealth: false,
		},
		{
			name: "minimal overrides with-* flags",
			opts: cli.PluginInitOptions{
				WithDocker: true,
				WithDocs:   true,
				WithHealth: true,
				Minimal:    true,
			},
			wantDocker: false,
			wantDocs:   false,
			wantHealth: false,
		},
		{
			name:       "docker-only always generates docker",
			opts:       cli.PluginInitOptions{Minimal: true, DockerOnly: true},
			wantDocker: true,
			wantDocs:   false,
			wantHealth: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantDocker, tc.opts.ShouldGenerateDocker())
			assert.Equal(t, tc.wantDocs, tc.opts.ShouldGenerateDocs())
			assert.Equal(t, tc.wantHealth, tc.opts.ShouldGenerateHealth())
		})
	}
}

func TestPluginInitMinimal(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:       "test-plugin",
		Author:     "Test Author",
		Providers:  []string{"aws"},
		OutputDir:  tmpDir,
		Force:      true,
		WithDocker: true,
		WithDocs:   true,
		WithHealth: true,
		Minimal:    true,
	}
	runPluginInitForTest(t, opts)

	projectDir := filepath.Join(tmpDir, "test-plugin")

	// Standard scaffolding still generated
	_, err := os.Stat(filepath.Join(projectDir, "go.mod"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(projectDir, "cmd", "plugin", "main.go"))
	require.NoError(t, err)

	// Docker files skipped despite --with-docker=true
	_, err = os.Stat(filepath.Join(projectDir, "docker", "Dockerfile"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(projectDir, ".dockerignore"))
	assert.True(t, os.IsNotExist(err))
}

func TestPluginInitCalculatorRPCMethods(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:       "test-plugin",
		Author:     "Test Author",
		Providers:  []string{"aws", "azure"},
		OutputDir:  tmpDir,
		Force:      true,
		WithDocker: true,
		WithDocs:   true,
		WithHealth: true,
	}
	runPluginInitForTest(t, opts)

	projectDir := filepath.Join(tmpDir, "test-plugin")

	calculator, err := os.ReadFile(filepath.Join(projectDir, "internal", "pricing", "calculator.go"))
	require.NoError(t, err)
	assert.Contains(t, string(calculator), "PluginVersion = \"0.1.0\"")
	assert.Contains(t, string(calculator), "SpecVersion")
	assert.Contains(t, string(calculator), "func (c *Calculator) GetPluginInfo(")
	assert.Contains(t, string(calculator), "func (c *Calculator) Supports(")
	assert.Contains(t, string(calculator), "pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS")
	assert.Contains(t, string(calculator), "pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS")
	assert.Contains(t, string(calculator), "\"aws\", \"azure\"")

	calculatorTest, err := os.ReadFile(filepath.Join(projectDir, "internal", "pricing", "calculator_test.go"))
	require.NoError(t, err)
	assert.Contains(t, string(calculatorTest), "func TestGetPluginInfo(t *testing.T)")
	assert.Contains(t, string(calculatorTest), "func TestSupports(t *testing.T)")
	assert.Contains(t, string(calculatorTest), "assert.Contains(t, resp.Providers, \"aws\")")
	assert.Contains(t, string(calculatorTest), "{\"aws supported\", \"aws\", true}")
}
