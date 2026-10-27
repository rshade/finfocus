package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

type stackOutput struct {
	Finfocus struct {
		EmptyPlan bool `json:"emptyPlan"`
		Summary   struct {
			TotalMonthly float64 `json:"totalMonthly"`
		} `json:"summary"`
		Diff struct {
			TotalAfter float64 `json:"totalAfter"`
		} `json:"diff"`
		Stack struct {
			TotalMonthly     float64           `json:"totalMonthly"`
			Resources        int               `json:"resources"`
			Complete         bool              `json:"complete"`
			IncompleteReason string            `json:"incompleteReason"`
			OmittedUnchanged int               `json:"omittedUnchanged"`
			Unchanged        []json.RawMessage `json:"unchanged"`
		} `json:"stack"`
	} `json:"finfocus"`
}

type unavailableStateRunner struct{ scriptedPulumi }

func (r unavailableStateRunner) Run(
	ctx context.Context,
	dir, name string,
	env []string,
	args ...string,
) ([]byte, []byte, error) {
	if strings.Contains(strings.Join(args, " "), "stack export") {
		return nil, []byte(
				"network failure https://backend.invalid/private?token=secret",
			), errors.New(
				"unreachable",
			)
	}
	return r.scriptedPulumi.Run(ctx, dir, name, env, args...)
}

// previewOfStack copies a dev-stack preview fixture with its URNs moved to stack.
func previewOfStack(t *testing.T, preview, stack string) string {
	t.Helper()
	data, err := os.ReadFile(preview)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), stack+"-preview.json")
	moved := strings.ReplaceAll(string(data), "urn:pulumi:dev::", "urn:pulumi:"+stack+"::")
	require.NoError(t, os.WriteFile(path, []byte(moved), 0o600))
	return path
}

// Root commands and runner fixtures mutate process environment.
func TestProjectedStackTotalsAndAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, plan, stack             string
		project, failed               bool
		resources, unchanged, omitted int
		complete                      bool
		reason                        string
	}{
		{"default plus state", "default", "dev", true, false, 5, 4, 0, true, ""},
		{"show sames plus state", "show-sames", "dev", true, false, 5, 0, 0, true, ""},
		{"saved default", "default", "dev", false, false, 1, 0, 4, false, "not_requested"},
		{"saved show sames", "show-sames", "dev", false, false, 5, 0, 0, false, "not_requested"},
		{"unreachable state", "default", "dev", true, true, 1, 0, 4, false, "backend_unreachable"},
		// A preview of another stack is projected alone, as outside a project.
		{"other stack preview", "default", "prod", true, false, 1, 0, 4, false, "not_requested"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := os.ReadFile("testdata/pulumi/stack-export-dev.json")
			require.NoError(t, err)
			preview, err := filepath.Abs("testdata/pulumi/preview-" + tc.plan + ".json")
			require.NoError(t, err)
			preview = previewOfStack(t, preview, tc.stack)
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
			t.Setenv("FINFOCUS_LOG_LEVEL", "error")
			t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
			t.Setenv(config.HistoryEnvEnabled, "false")
			t.Cleanup(config.ResetGlobalConfigForTest)
			project := t.TempDir()
			if tc.project {
				writePulumiProject(t, project, false)
			}
			t.Chdir(project)
			installFakePulumi(t)
			runner := scriptedPulumi{export: state}
			if tc.failed {
				t.Cleanup(pulumidetect.SetRunnerForTest(unavailableStateRunner{runner}))
			} else {
				t.Cleanup(pulumidetect.SetRunnerForTest(runner))
			}
			result := axtest.Run(
				context.Background(),
				t,
				NewRootCmd("test"),
				[]string{
					"cost",
					"projected",
					"--pulumi-json",
					preview,
					"--stack",
					"dev",
					"--output",
					"json",
				},
			)
			require.Equal(t, 0, result.ExitCode, "%s", result.Stderr)
			var doc stackOutput
			require.NoError(t, json.Unmarshal(result.Stdout, &doc), "%s", result.Stdout)
			stack := doc.Finfocus.Stack
			assert.Equal(t, tc.complete, stack.Complete)
			assert.Equal(t, tc.resources, stack.Resources)
			assert.Len(t, stack.Unchanged, tc.unchanged)
			assert.Equal(t, tc.reason, stack.IncompleteReason)
			assert.Equal(t, tc.omitted, stack.OmittedUnchanged)
			assert.InDelta(t, doc.Finfocus.Summary.TotalMonthly, doc.Finfocus.Diff.TotalAfter, 1e-9)
			assert.NotContains(t, string(result.Stderr), "backend.invalid")
			assert.NotContains(t, string(result.Stderr), "token=secret")
			if tc.failed {
				assert.Contains(
					t,
					string(result.Stderr),
					"stack state unavailable (backend_unreachable)",
				)
			}
			if !tc.project && tc.omitted > 0 {
				assert.Contains(t, string(result.Stderr), "preview omits 4 unchanged resources")
			}
			if tc.omitted == 0 {
				assert.NotContains(t, string(result.Stderr), "preview omits")
			}
		})
	}
}

// Root commands mutate process environment.
func TestProjectedExplicitStateAndPreviewPair(t *testing.T) {
	preview, err := filepath.Abs("testdata/pulumi/preview-default.json")
	require.NoError(t, err)
	state, err := filepath.Abs("testdata/pulumi/stack-export-dev.json")
	require.NoError(t, err)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	t.Chdir(t.TempDir())
	run := func(statePath string) axtest.Result {
		return axtest.Run(context.Background(), t, NewRootCmd("test"), []string{
			"cost", "projected", "--pulumi-json", preview, "--pulumi-state", statePath, "--output", "json",
		})
	}

	result := run(state)
	require.Equal(t, 0, result.ExitCode, "%s", result.Stderr)
	var doc stackOutput
	require.NoError(t, json.Unmarshal(result.Stdout, &doc), "%s", result.Stdout)
	assert.True(t, doc.Finfocus.Stack.Complete)
	assert.Equal(t, 5, doc.Finfocus.Stack.Resources)
	assert.Len(t, doc.Finfocus.Stack.Unchanged, 4)

	missing := run(filepath.Join(t.TempDir(), "missing.json"))
	assert.Equal(t, 2, missing.ExitCode, "an explicit state file that cannot be read is input error")
	assert.Contains(t, string(missing.Stderr)+string(missing.Stdout), "no such file")
}
