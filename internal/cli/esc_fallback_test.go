package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/esc/esctest"
	"github.com/rshade/finfocus/internal/pluginhost"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

const escFixtureSecret = "fixture-secret-9f3c"

type escFallbackCase struct {
	name  string
	stack string
	env   func(t *testing.T, api *esctest.Server)
	want  string
}

//nolint:paralleltest // sets the process environment and the config singleton
func TestESCFallbackWarnings(t *testing.T) {
	cases := []escFallbackCase{
		{
			name:  "not found",
			stack: "environment:\n  - acme/billing/missing\n",
			want:  "not found or no access",
			env: func(t *testing.T, api *esctest.Server) {
				t.Helper()
				t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
				t.Setenv("PULUMI_BACKEND_URL", api.URL)
			},
		},
		{
			name:  "access denied",
			stack: "environment:\n  - acme/billing/denied\n",
			want:  "access_denied",
			env: func(t *testing.T, api *esctest.Server) {
				t.Helper()
				t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
				t.Setenv("PULUMI_BACKEND_URL", api.URL)
			},
		},
		{
			name:  "not logged in",
			stack: "environment:\n  - acme/billing/prod\n",
			want:  "set PULUMI_ACCESS_TOKEN or run `pulumi login`",
			env: func(t *testing.T, _ *esctest.Server) {
				t.Helper()
				t.Setenv("PULUMI_ACCESS_TOKEN", "")
				t.Setenv("PULUMI_CREDENTIALS_PATH", t.TempDir())
			},
		},
		{
			name:  "file backend",
			stack: "environment:\n  - acme/billing/prod\n",
			want:  "unsupported_backend",
			env: func(t *testing.T, _ *esctest.Server) {
				t.Helper()
				t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
				t.Setenv("PULUMI_BACKEND_URL", "file://"+t.TempDir())
			},
		},
		{
			name:  "inline environment",
			stack: "environment:\n  values:\n    TOKEN: " + escFixtureSecret + "\n",
			want:  "unsupported_definition",
			env: func(t *testing.T, api *esctest.Server) {
				t.Helper()
				t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
				t.Setenv("PULUMI_BACKEND_URL", api.URL)
			},
		},
	}
	for _, tc := range cases {
		//nolint:paralleltest // sets the process environment and the config singleton
		t.Run(tc.name, func(t *testing.T) {
			assertESCFallback(t, tc)
		})
	}
}

func assertESCFallback(t *testing.T, tc escFallbackCase) {
	t.Helper()
	t.Cleanup(pulumidetect.SetRunnerForTest(escCLIRunner{t: t}))
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/missing": {
			Status: 404, Message: escFixtureSecret,
			Variables: map[string]esctest.Variable{"TOKEN": {Value: escFixtureSecret}},
		},
		"acme/billing/denied": {
			Status: 403, Message: escFixtureSecret,
			Variables: map[string]esctest.Variable{"TOKEN": {Value: escFixtureSecret}},
		},
		"acme/billing/prod": {
			ETag:      "e1",
			Variables: map[string]esctest.Variable{"TOKEN": {Value: escFixtureSecret}},
		},
	})
	t.Cleanup(api.Close)

	home := t.TempDir()
	logPath := filepath.Join(home, "finfocus.log")
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"),
		[]byte(`{"logging":{"level":"debug","format":"json","file":"`+logPath+`"}}`), 0o600))
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_LOG_LEVEL", "debug")
	t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", t.TempDir())
	installRecorderPlugin(t, home)
	tc.env(t, api)

	plan := escAbsPlan(t)
	baselineDir := escWriteProject(t, "config: {}\n")
	failedDir := escWriteProject(t, tc.stack)
	baseline := runProjectedIn(t, baselineDir, plan)
	require.Equal(t, 0, baseline.ExitCode, "baseline stderr: %s", baseline.Stderr)
	got := runProjectedIn(t, failedDir, plan)
	require.Equal(t, 0, got.ExitCode, "stderr: %s", got.Stderr)
	assert.Equal(t, string(baseline.Stdout), string(got.Stdout))
	assert.Equal(t, 1, strings.Count(string(got.Stderr), "Warning: ESC"), "stderr: %s", got.Stderr)
	assert.Contains(t, string(got.Stderr), tc.want)
	logBody, err := os.ReadFile(logPath)
	require.NoError(t, err)
	combined := string(got.Stdout) + string(got.Stderr) + string(logBody)
	assert.NotContains(t, combined, escFixtureSecret)
}

func TestESCNoEnvironmentMakesNoRequests(t *testing.T) {
	t.Cleanup(pulumidetect.SetRunnerForTest(escCLIRunner{t: t}))
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/prod": {
			ETag:      "e1",
			Variables: map[string]esctest.Variable{"TOKEN": {Value: escFixtureSecret}},
		},
	})
	t.Cleanup(api.Close)
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
	t.Setenv("PULUMI_BACKEND_URL", api.URL)
	t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", t.TempDir())
	installRecorderPlugin(t, home)
	project := escWriteProject(t, "config: {}\n")
	result := runProjectedIn(t, project, escAbsPlan(t))
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Equal(t, 0, api.Requests())
	assert.NotContains(t, string(result.Stderr), "Warning: ESC")
	assert.NotContains(t, string(result.Stdout), escFixtureSecret)
}

func TestESCNoPluginMakesNoRequests(t *testing.T) {
	t.Cleanup(pulumidetect.SetRunnerForTest(escCLIRunner{t: t}))
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/prod": {
			ETag:      "e1",
			Variables: map[string]esctest.Variable{"TOKEN": {Value: escFixtureSecret}},
		},
	})
	t.Cleanup(api.Close)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
	t.Setenv("PULUMI_BACKEND_URL", api.URL)
	project := escWriteProject(t, "environment:\n  - acme/billing/prod\n")
	result := runProjectedIn(t, project, escAbsPlan(t))
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Equal(t, 0, api.Requests(), "no plugin launches, so nothing needs the values")
}

func TestEmptyESCDeliveryMatchesNoSource(t *testing.T) {
	t.Parallel()
	settings := []pluginhost.SettingDesc{{
		Key: "output_dir", Env: "FINFOCUS_RECORDER_OUTPUT_DIR", HasDefault: true, Default: "./recorded_data",
	}}
	parent := []string{"FINFOCUS_RECORDER_OUTPUT_DIR=/tmp/shell"}
	ctx := esc.WithSource(context.Background(), esc.NewSource(esc.Options{
		Budget: 1, Getenv: func(string) string { return "" },
	}))
	with := pluginhost.ResolveDeliveredEnv(
		ctx,
		"recorder",
		parent,
		nil,
		nil,
		settings,
		esc.Delivery{},
	)
	without := pluginhost.ResolveDeliveredEnv(
		context.Background(),
		"recorder",
		parent,
		nil,
		nil,
		settings,
		esc.Delivery{},
	)
	assert.Equal(t, without.Pairs, with.Pairs)
	assert.Equal(t, without.Digest, with.Digest)
	assert.NotContains(t, strings.Join(with.Pairs, "\n"), escFixtureSecret)
}

func runProjectedIn(t *testing.T, dir, plan string) axtest.Result {
	t.Helper()
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	t.Chdir(dir)
	return axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{
		"cost", "projected", "--pulumi-json", plan, "--stack", "dev", "--output", "json",
	})
}

func escWriteProject(t *testing.T, stackYAML string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(dir, "Pulumi.yaml"),
			[]byte("name: demo\nruntime: yaml\n"),
			0o600,
		),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(dir, "Pulumi.dev.yaml"), []byte(stackYAML), 0o600),
	)
	return dir
}

func escAbsPlan(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(simplePlanPath)
	require.NoError(t, err)
	return abs
}

type escCLIRunner struct {
	t *testing.T
}

func (r escCLIRunner) Run(
	_ context.Context,
	_, _ string,
	_ []string,
	args ...string,
) ([]byte, []byte, error) {
	if strings.Contains(strings.Join(args, " "), "stack export") {
		return []byte(`{"version":3,"deployment":{"resources":[]}}`), nil, nil
	}
	r.t.Fatal("pulumi CLI was invoked")
	return nil, nil, nil
}

func installRecorderPlugin(t *testing.T, home string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	dir := filepath.Join(home, "plugins", "recorder", "0.1.0")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	name := "finfocus-plugin-recorder"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	build := exec.CommandContext(
		context.Background(),
		"go",
		"build",
		"-o",
		filepath.Join(dir, name),
		"./plugins/recorder/cmd",
	)
	build.Dir = root
	output, err := build.CombinedOutput()
	require.NoError(t, err, "building the recorder plugin: %s", output)
	manifest, err := os.ReadFile(filepath.Join(root, "plugins", "recorder", "plugin.manifest.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), manifest, 0o600))
}
