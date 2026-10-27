package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

// The command fixtures change process configuration and the working directory.
func TestProjectedUnchangedStackFormats(t *testing.T) {
	for _, format := range []string{"table", "json", "ndjson"} {
		t.Run(format, func(t *testing.T) {
			state, err := os.ReadFile("testdata/pulumi/stack-export-dev.json")
			require.NoError(t, err)
			plan, err := filepath.Abs("testdata/pulumi/unchanged-preview-dev.json")
			require.NoError(t, err)
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
			t.Setenv("FINFOCUS_LOG_LEVEL", "error")
			t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
			t.Setenv(config.HistoryEnvEnabled, "false")
			t.Cleanup(config.ResetGlobalConfigForTest)
			project := t.TempDir()
			writePulumiProject(t, project, false)
			t.Chdir(project)
			installFakePulumi(t)
			t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: state}))
			result := axtest.Run(
				context.Background(),
				t,
				NewRootCmd("test"),
				[]string{
					"cost",
					"projected",
					"--pulumi-json",
					plan,
					"--stack",
					"dev",
					"--output",
					format,
				},
			)
			require.Equal(t, 0, result.ExitCode, "%s", result.Stderr)
			switch format {
			case "table":
				assert.Contains(
					t,
					string(result.Stdout),
					"No changes. Projected monthly cost is zero.",
				)
				assert.Contains(t, string(result.Stdout), "across 5 unchanged resources.")
			case "json":
				var doc stackOutput
				require.NoError(t, json.Unmarshal(result.Stdout, &doc))
				assert.True(t, doc.Finfocus.EmptyPlan)
				assert.True(t, doc.Finfocus.Stack.Complete)
				assert.Equal(t, 5, doc.Finfocus.Stack.Resources)
			case "ndjson":
				assert.Empty(t, result.Stdout, "state-only rows must not add NDJSON lines")
				assert.Contains(t, string(result.Stderr), "No changes. Projected monthly cost is zero.")
				assert.Contains(t, string(result.Stderr), "across 5 unchanged resources.")
			}
			if format != "ndjson" {
				assert.NotContains(t, string(result.Stderr), "No changes.", "stdout already explains it")
			}
			assert.NotContains(t, string(result.Stderr), "$0 total is not an estimate")
		})
	}
}
