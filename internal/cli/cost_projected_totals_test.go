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

// Root commands and runner fixtures mutate process environment.
func TestProjectedStackTotalsAndAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, plan                    string
		project, failed               bool
		resources, unchanged, omitted int
		complete                      bool
		reason                        string
	}{
		{"default plus state", "default", true, false, 5, 4, 0, true, ""},
		{"show sames plus state", "show-sames", true, false, 5, 0, 0, true, ""},
		{"saved default", "default", false, false, 1, 0, 4, false, "not_requested"},
		{"saved show sames", "show-sames", false, false, 5, 0, 0, false, "not_requested"},
		{"unreachable state", "default", true, true, 1, 0, 4, false, "backend_unreachable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := os.ReadFile("testdata/pulumi/stack-export-dev.json")
			require.NoError(t, err)
			preview, err := filepath.Abs("testdata/pulumi/preview-" + tc.plan + ".json")
			require.NoError(t, err)
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
