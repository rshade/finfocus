package cli

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/esc/esctest"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

type escFailRunner struct {
	t *testing.T
}

func (r escFailRunner) Run(context.Context, string, string, []string, ...string) ([]byte, []byte, error) {
	r.t.Fatal("pulumi CLI was invoked")
	return nil, nil, nil
}

func TestPersistentPreRunAttachesESCSource(t *testing.T) {
	t.Cleanup(pulumidetect.SetRunnerForTest(escFailRunner{t: t}))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: demo\n"), 0o600))
	t.Chdir(project)

	var out, errBuf bytes.Buffer
	cmd := NewRootCmdWithArgs("test", []string{"finfocus", "config", "validate"}, func(key string) (string, bool) {
		if key == "FINFOCUS_SKIP_MIGRATION_CHECK" {
			return "1", true
		}
		return "", false
	})
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"config", "validate"})
	_ = cmd.Execute()

	require.NotNil(t, findESCSource(cmd))
}

func findESCSource(cmd *cobra.Command) *esc.Source {
	if src, ok := esc.SourceFromContext(cmd.Context()); ok {
		return src
	}
	for _, child := range cmd.Commands() {
		if src := findESCSource(child); src != nil {
			return src
		}
	}
	return nil
}

func TestESCStackFlagBeatsWorkspace(t *testing.T) {
	t.Cleanup(pulumidetect.SetRunnerForTest(escFailRunner{t: t}))
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", home)
	t.Setenv("PULUMI_BACKEND_URL", "")
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	project := escProject(t, map[string]string{
		"dev":  "environment:\n  - proj/dev\n",
		"prod": "environment:\n  - proj/prod\n",
	})
	writeESCWorkspace(t, home, project, `{"stacks":{"https://api.pulumi.com":"dev"}}`)
	plan := escPlanPath(t)
	t.Chdir(project)
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	workspace := executeESC(t, "config", "validate")
	require.Equal(t, 0, workspace.exit, "stderr: %s", workspace.stderr)
	got := findESCSource(workspace.cmd)
	require.NotNil(t, got)
	assert.Equal(t, []string{"proj/dev"}, got.Options().Stack.Environment.Imports)

	config.ResetGlobalConfigForTest()
	flagged := executeESC(t, "cost", "projected", "--pulumi-json", plan, "--stack", "prod", "--output", "json")
	require.Equal(t, 0, flagged.exit, "stderr: %s", flagged.stderr)
	flaggedSource := findESCSource(flagged.cmd)
	require.NotNil(t, flaggedSource)
	assert.Equal(t, "prod", flaggedSource.Options().Stack.Stack)
	assert.Equal(t, []string{"proj/prod"}, flaggedSource.Options().Stack.Environment.Imports)
}

func TestMCPCallsReceiveDistinctESCSources(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("PULUMI_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	var sources []*esc.Source
	stubMCPServe(t, func(ctx context.Context, root *cobra.Command, _ string) error {
		for _, args := range [][]string{
			{"config", "list", "--format=json"},
			{"config", "validate"},
		} {
			call := executeNested(ctx, root, args)
			require.NoError(t, call.err, call.stderr)
			cmd, _, err := root.Find(args[:2])
			require.NoError(t, err)
			src, ok := esc.SourceFromContext(cmd.Context())
			require.True(t, ok)
			sources = append(sources, src)
		}
		return nil
	})

	result := axtest.Run(context.Background(), t, NewRootCmd("v1.2.3"), []string{"--mcp"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	require.Len(t, sources, 2)
	assert.NotSame(t, sources[0], sources[1])
}

func TestAnalyzerESCIgnoresStackImports(t *testing.T) {
	t.Parallel()
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/prod": {ETag: "e1", Variables: map[string]esctest.Variable{"FOO": {Value: "bar"}}},
	})
	t.Cleanup(api.Close)
	src := esc.NewSource(esc.Options{
		UseStack: true,
		Stack: pulumidetect.StackContext{
			Stack: "acme/demo/dev",
			Environment: pulumidetect.StackEnvironment{
				Form:    pulumidetect.StackEnvImports,
				Imports: []string{"acme/billing/prod"},
			},
		},
		Getenv: func(key string) string {
			switch key {
			case "PULUMI_ACCESS_TOKEN":
				return "tok"
			case "PULUMI_BACKEND_URL":
				return api.URL
			default:
				return ""
			}
		},
		Budget: time.Second,
	})
	ctx := analyzerESCContext(esc.WithSource(context.Background(), src), &cobra.Command{})
	got, ok := esc.SourceFromContext(ctx)
	require.True(t, ok)
	assert.NotSame(t, src, got)
	assert.False(t, got.Options().UseStack)
	assert.Empty(t, got.Options().Stack.Environment.Imports)
	assert.Empty(t, got.Delivery(context.Background(), "recorder").Values)
	assert.Equal(t, 0, api.Requests())
}

func TestESCConfigRejectsBadReferencesExceptConfig(t *testing.T) {
	t.Cleanup(pulumidetect.SetRunnerForTest(escFailRunner{t: t}))
	home := t.TempDir()
	plan := escPlanPath(t)
	project := escProject(t, nil)
	bad := []byte(`{"esc":{"environments":["a//b"]}}`)
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), bad, 0o600))
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	api := esctest.New("tok", nil)
	t.Cleanup(api.Close)
	t.Setenv("PULUMI_BACKEND_URL", api.URL)
	t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
	t.Chdir(project)

	for _, args := range [][]string{
		{"config", "validate"},
		{"cost", "projected", "--pulumi-json", plan, "--output", "json"},
		{"overview", "--plain", "--pulumi-json", plan},
		{"--web", "--no-browser", "--port", "18766"},
	} {
		config.ResetGlobalConfigForTest()
		result := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
		combined := string(result.Stdout) + string(result.Stderr)
		assert.Equal(t, 2, result.ExitCode, "%v\n%s", args, combined)
		assert.Contains(t, combined, "esc.environments[0]")
	}
	assert.Equal(t, 0, api.Requests())

	config.ResetGlobalConfigForTest()
	repaired := axtest.Run(context.Background(), t, NewRootCmd("test"),
		[]string{"config", "set", "esc.environments", "billing/aws"})
	require.Equal(t, 0, repaired.ExitCode, "stderr: %s", repaired.Stderr)
	body, err := os.ReadFile(filepath.Join(home, "config.hujson"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "billing/aws")
	assert.NotContains(t, string(body), "a//b")

	config.ResetGlobalConfigForTest()
	again := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"config", "validate"})
	require.Equal(t, 0, again.ExitCode, "stderr: %s", again.Stderr)
}

func TestESCSourceUsesProjectTier(t *testing.T) {
	t.Cleanup(pulumidetect.SetRunnerForTest(escFailRunner{t: t}))
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"),
		[]byte(`{"esc":{"environments":["global/env"]}}`), 0o600))
	project := escProject(t, nil)
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".finfocus"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(project, ".finfocus", "config.hujson"), []byte(`{
		"esc": {
			"environments": ["proj/only"],
			"plugins": {"recorder": {"environments": ["billing/recorder"], "variables": ["FOO"]}}
		}
	}`), 0o600))
	t.Chdir(project)
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	ran := executeESC(t, "config", "validate")
	require.Equal(t, 0, ran.exit, "stderr: %s", ran.stderr)
	src := findESCSource(ran.cmd)
	require.NotNil(t, src)
	opts := src.Options()
	require.NotNil(t, opts.Global)
	assert.Equal(t, []string{"proj/only"}, *opts.Global)
	require.Contains(t, opts.PerPlugin, "recorder")
	require.NotNil(t, opts.PerPlugin["recorder"].Environments)
	assert.Equal(t, []string{"billing/recorder"}, *opts.PerPlugin["recorder"].Environments)
	assert.Equal(t, []string{"FOO"}, opts.PerPlugin["recorder"].Variables)
}

type escRun struct {
	cmd    *cobra.Command
	exit   int
	stderr string
}

func executeESC(t *testing.T, args ...string) escRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := NewRootCmd("test")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	exit := 0
	if err != nil {
		exit = 1
	}
	return escRun{cmd: cmd, exit: exit, stderr: stderr.String()}
}

func escProject(t *testing.T, stacks map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: demo\nruntime: yaml\n"), 0o600))
	for name, body := range stacks {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi."+name+".yaml"), []byte(body), 0o600))
	}
	return dir
}

func writeESCWorkspace(t *testing.T, home, project, body string) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(project, "Pulumi.yaml"))
	require.NoError(t, err)
	sum := sha1.Sum([]byte(abs))
	path := filepath.Join(home, "workspaces", fmt.Sprintf("demo-%x-workspace.json", sum))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func escPlanPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "examples", "plans", "aws-simple-plan.json"))
	require.NoError(t, err)
	return abs
}
