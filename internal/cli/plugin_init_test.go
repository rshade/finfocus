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
	"golang.org/x/mod/semver"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/pluginupgrade"
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

//nolint:paralleltest // t.Setenv changes the process-wide environment
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
		".golangci-lint.yml",
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

//nolint:paralleltest // t.Setenv changes the process-wide environment
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

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
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

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
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

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
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
	_, err = os.Stat(filepath.Join(projectDir, ".golangci-lint.yml"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(projectDir, "cmd", "plugin", "main.go"))
	assert.True(t, os.IsNotExist(err))
}

func TestPluginInitShouldGenerateFlags(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			assert.Equal(t, tc.wantDocker, tc.opts.ShouldGenerateDocker())
			assert.Equal(t, tc.wantDocs, tc.opts.ShouldGenerateDocs())
			assert.Equal(t, tc.wantHealth, tc.opts.ShouldGenerateHealth())
		})
	}
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
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

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
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
	assert.Contains(t, string(calculator), "SpecVersion = pluginsdk.SpecVersion",
		"SpecVersion must follow the SDK constant, which is v-prefixed")
	assert.Contains(t, string(calculator), `case "aws:ec2/instance:Instance":`,
		"the example must match the Pulumi type token finfocus sends")
	assert.NotContains(t, string(calculator), `case "aws:ec2:Instance":`)
	assert.NotContains(t, string(calculator), `instanceType = "t3.micro"`,
		"a missing instance type must not default to a priced one")
	assert.NotContains(t, string(calculator), "// fallback",
		"an unknown instance type must not get a default price")

	calculatorTest, err := os.ReadFile(filepath.Join(projectDir, "internal", "pricing", "calculator_test.go"))
	require.NoError(t, err)
	assert.Contains(t, string(calculatorTest), "func TestGetPluginInfo(t *testing.T)")
	assert.Contains(t, string(calculatorTest), "func TestSupports(t *testing.T)")
	assert.Contains(t, string(calculatorTest), "assert.Contains(t, resp.Providers, \"aws\")")
	assert.Contains(t, string(calculatorTest), "{\"aws supported\", \"aws\", true}")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitHealthEndpoint(t *testing.T) {
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
	}
	runPluginInitForTest(t, opts)

	mainGo, err := os.ReadFile(filepath.Join(tmpDir, "test-plugin", "cmd", "plugin", "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(mainGo), "func startHealthServer(")
	assert.Contains(t, string(mainGo), `mux.HandleFunc("/health"`)
	assert.Contains(t, string(mainGo), `mux.HandleFunc("/ready"`)
	assert.Contains(t, string(mainGo), "FINFOCUS_PLUGIN_HEALTH_ENDPOINT")
	assert.Contains(t, string(mainGo), "FINFOCUS_PLUGIN_HEALTH_PORT")
	assert.Contains(t, string(mainGo), "pricing.PluginVersion")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitNoHealth(t *testing.T) {
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
		NoHealth:   true,
	}
	runPluginInitForTest(t, opts)

	mainGo, err := os.ReadFile(filepath.Join(tmpDir, "test-plugin", "cmd", "plugin", "main.go"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainGo), "startHealthServer")
	assert.NotContains(t, string(mainGo), "/health")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitDocsGenerated(t *testing.T) {
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
	}
	runPluginInitForTest(t, opts)

	projectDir := filepath.Join(tmpDir, "test-plugin")

	for _, file := range []string{"docs/api.md", "docs/configuration.md", "docs/deployment.md"} {
		_, err := os.Stat(filepath.Join(projectDir, file))
		require.NoError(t, err, "expected docs file %s", file)
	}

	apiDoc, err := os.ReadFile(filepath.Join(projectDir, "docs", "api.md"))
	require.NoError(t, err)
	assert.Contains(t, string(apiDoc), "# test-plugin Plugin API Reference")
	assert.Contains(t, string(apiDoc), "FinFocus plugin for test-plugin")
	assert.Contains(t, string(apiDoc), "`GetPluginInfoRequest`")
	assert.NotContains(t, string(apiDoc), "{{PLUGIN_NAME}}")
	assert.NotContains(t, string(apiDoc), "{{BACKTICK}}")

	configDoc, err := os.ReadFile(filepath.Join(projectDir, "docs", "configuration.md"))
	require.NoError(t, err)
	assert.Contains(t, string(configDoc), "FINFOCUS_PLUGIN_HEALTH_PORT")

	deployDoc, err := os.ReadFile(filepath.Join(projectDir, "docs", "deployment.md"))
	require.NoError(t, err)
	assert.Contains(t, string(deployDoc), "docker run -p 8080:8080 -p 8081:8081 test-plugin:local")
	assert.Contains(t, string(deployDoc), "```yaml")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitNoDocs(t *testing.T) {
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
		NoDocs:     true,
	}
	runPluginInitForTest(t, opts)

	_, err := os.Stat(filepath.Join(tmpDir, "test-plugin", "docs"))
	assert.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitGolangciConfig(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:      "my-plugin",
		Author:    "Test Author",
		Providers: []string{"aws"},
		OutputDir: tmpDir,
		Force:     true,
	}
	runPluginInitForTest(t, opts)

	projectDir := filepath.Join(tmpDir, "my-plugin")
	cfg, err := os.ReadFile(filepath.Join(projectDir, ".golangci-lint.yml"))
	require.NoError(t, err)
	text := string(cfg)
	assert.Contains(t, text, "version: \"2\"")
	assert.Contains(t, text, "local-prefixes:")
	assert.Contains(t, text, "github.com/example/my-plugin")
	assert.Contains(t, text, "- errcheck")
	assert.Contains(t, text, "- staticcheck")
	assert.Contains(t, text, "- gosec")

	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(goMod), "module github.com/example/my-plugin")

	makefile, err := os.ReadFile(filepath.Join(projectDir, "Makefile"))
	require.NoError(t, err)
	assert.Contains(t, string(makefile), "golangci-lint run --config .golangci-lint.yml")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitEnhancedMakefile(t *testing.T) {
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
	}
	runPluginInitForTest(t, opts)

	makefile, err := os.ReadFile(filepath.Join(tmpDir, "test-plugin", "Makefile"))
	require.NoError(t, err)
	content := string(makefile)

	for _, target := range []string{
		"build:", "test:", "test-integration:", "test-all:", "clean:", "lint:",
		"install:", "develop:", "build-debug:", "docker-build:", "docker-run:",
		"cover:", "fmt:", "deps:", "ensure:", "security:", "help:",
	} {
		assert.Contains(t, content, target)
	}
	assert.Contains(t, content, "VERSION = 0.1.0")
	assert.Contains(t, content, "golangci-lint run --config .golangci-lint.yml --allow-parallel-runners")
	assert.Contains(t, content, "~/.finfocus/plugins/$(PLUGIN_NAME)/$(VERSION)/")
	assert.Contains(t, content,
		`> ~/.finfocus/plugins/$(PLUGIN_NAME)/$(VERSION)/plugin.manifest.json`,
		"install must write the flat JSON manifest that plugin validate reads")
	assert.NotContains(t, content, "cp manifest.yaml")
	assert.Contains(t, content, "docker build -t $(PLUGIN_NAME):local -f docker/Dockerfile .")
	assert.NotContains(t, content, "{{NAME}}")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitMakefileNoDocker(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:       "test-plugin",
		Author:     "Test Author",
		Providers:  []string{"aws"},
		OutputDir:  tmpDir,
		Force:      true,
		WithDocs:   true,
		WithHealth: true,
	}
	runPluginInitForTest(t, opts)

	makefile, err := os.ReadFile(filepath.Join(tmpDir, "test-plugin", "Makefile"))
	require.NoError(t, err)
	content := string(makefile)

	assert.NotContains(t, content, "docker-build:")
	assert.NotContains(t, content, "docker-run:")
	assert.Contains(t, content, "build:")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitWorkflows(t *testing.T) {
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
	}
	runPluginInitForTest(t, opts)

	workflowsDir := filepath.Join(tmpDir, "test-plugin", ".github", "workflows")

	for _, file := range []string{"ci.yml", "release.yml", "release-please.yml", "docker.yml"} {
		_, err := os.Stat(filepath.Join(workflowsDir, file))
		require.NoError(t, err, "expected workflow %s", file)
	}

	// Claude review workflow not generated without --with-claude-review
	_, err := os.Stat(filepath.Join(workflowsDir, "claude-code-review.yml"))
	assert.True(t, os.IsNotExist(err))

	ci, err := os.ReadFile(filepath.Join(workflowsDir, "ci.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(ci), "go-version: '1.27.1'")
	assert.NotContains(t, string(ci), "{{GO_VERSION}}")
	assert.Contains(t, string(ci), "golangci/golangci-lint-action@v7")
	assert.Contains(t, string(ci), "args: --config .golangci-lint.yml --allow-parallel-runners")

	docker, err := os.ReadFile(filepath.Join(workflowsDir, "docker.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(docker), "IMAGE_NAME: ${{ github.repository }}")
	assert.Contains(t, string(docker), "type=semver,pattern={{version}}")
	assert.Contains(t, string(docker), "file: docker/Dockerfile")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitWorkflowsNoDocker(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:       "test-plugin",
		Author:     "Test Author",
		Providers:  []string{"aws"},
		OutputDir:  tmpDir,
		Force:      true,
		WithDocs:   true,
		WithHealth: true,
	}
	runPluginInitForTest(t, opts)

	workflowsDir := filepath.Join(tmpDir, "test-plugin", ".github", "workflows")

	for _, file := range []string{"ci.yml", "release.yml", "release-please.yml"} {
		_, err := os.Stat(filepath.Join(workflowsDir, file))
		require.NoError(t, err, "expected workflow %s", file)
	}

	_, err := os.Stat(filepath.Join(workflowsDir, "docker.yml"))
	assert.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitClaudeReviewWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &cli.PluginInitOptions{
		Name:             "test-plugin",
		Author:           "Test Author",
		Providers:        []string{"aws"},
		OutputDir:        tmpDir,
		Force:            true,
		WithDocker:       true,
		WithDocs:         true,
		WithHealth:       true,
		WithClaudeReview: true,
	}
	runPluginInitForTest(t, opts)

	claude, err := os.ReadFile(
		filepath.Join(tmpDir, "test-plugin", ".github", "workflows", "claude-code-review.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(claude), "anthropics/claude-code-action@v1")
}

// minScaffoldSpecVersion is the oldest finfocus-spec a new plugin may start on
// (#248).
const minScaffoldSpecVersion = "v0.7.1"

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitTracksCoreSpecVersion(t *testing.T) {
	tmpDir := t.TempDir()
	runPluginInitForTest(t, &cli.PluginInitOptions{
		Name: "spec-plugin", Author: "Test Author", Providers: []string{"aws"},
		OutputDir: tmpDir, Force: true,
	})
	projectDir := filepath.Join(tmpDir, "spec-plugin")

	gomod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(gomod), "github.com/rshade/finfocus-spec "+pluginsdk.SpecVersion+"\n")
	assert.GreaterOrEqual(t, semver.Compare(pluginsdk.SpecVersion, minScaffoldSpecVersion), 0,
		"a new plugin must start on finfocus-spec %s or newer", minScaffoldSpecVersion)

	project, err := pluginupgrade.Detect(projectDir)
	require.NoError(t, err)
	plan, err := pluginupgrade.NewPlan(project, pluginsdk.SpecVersion, pluginsdk.SpecVersion)
	require.NoError(t, err)
	assert.True(t, plan.UpToDate, "a fresh scaffold needs no plugin upgrade")
	assert.Empty(t, plan.Warnings, "the scaffold declares SpecVersion through the SDK constant")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via runPluginInitForTest)
func TestPluginInitCalculatorExtractsAndValidates(t *testing.T) {
	tmpDir := t.TempDir()
	runPluginInitForTest(t, &cli.PluginInitOptions{
		Name: "extract-plugin", Author: "Test Author", Providers: []string{"aws"},
		OutputDir: tmpDir, Force: true,
	})
	projectDir := filepath.Join(tmpDir, "extract-plugin")

	calculator, err := os.ReadFile(filepath.Join(projectDir, "internal", "pricing", "calculator.go"))
	require.NoError(t, err)
	content := string(calculator)
	assert.Contains(t, content, "func fillFromInputs(resource *pbc.ResourceDescriptor)")
	assert.Contains(t, content, "mapping.ExtractAWSSKU(resource.GetTags())")
	assert.Contains(t, content, "mapping.ExtractAWSRegion(resource.GetTags())")
	assert.Contains(t, content, "pluginsdk.ValidateProjectedCostRequest(req)")
	assert.Contains(t, content, "status.Error(codes.InvalidArgument, err.Error())")
	assert.Contains(t, content, `if resource.GetRegion() != "us-east-1"`,
		"us-east-1 example rates must not price other regions")

	readme, err := os.ReadFile(filepath.Join(projectDir, "README.md"))
	require.NoError(t, err)
	for _, want := range []string{
		"https://github.com/rshade/finfocus-spec",
		"https://pkg.go.dev/github.com/rshade/finfocus-spec/sdk/go/pluginsdk",
		"finfocus plugin conformance ./bin/finfocus-plugin-extract-plugin",
		"`.agents/skills/`",
		"Go 1.27.1+",
	} {
		assert.Contains(t, string(readme), want)
	}
	assert.NotContains(t, string(readme), "{{")
}
