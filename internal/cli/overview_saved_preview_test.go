package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/stackload"
)

type savedPreviewUnavailableRunner struct {
	calls     []string
	exportEnv []string
}

func (r *savedPreviewUnavailableRunner) Run(
	_ context.Context,
	_, _ string,
	env []string,
	args ...string,
) ([]byte, []byte, error) {
	command := strings.Join(args, " ")
	r.calls = append(r.calls, command)
	if strings.Contains(command, "stack export") {
		r.exportEnv = append([]string(nil), env...)
	}
	return nil, []byte("network unreachable https://private.invalid?token=secret"), errors.New("unreachable")
}

// Process environment and Pulumi runners are isolated for each loader invocation.
func TestOverviewSavedPreviewSurvivesUnavailableProjectState(t *testing.T) {
	for _, failure := range []string{"missing binary", "backend unavailable"} {
		t.Run(failure, func(t *testing.T) {
			plan, err := filepath.Abs("testdata/pulumi/preview-create-only.json")
			require.NoError(t, err)
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			t.Setenv(config.HistoryEnvEnabled, "false")
			project := t.TempDir()
			require.NoError(
				t,
				os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: test\nruntime: yaml\n"), 0600),
			)
			t.Chdir(project)
			runner := &savedPreviewUnavailableRunner{}
			reason := stackload.ReasonReadFailed
			if failure == "missing binary" {
				t.Setenv("PATH", t.TempDir())
			} else {
				installFakePulumi(t)
				t.Cleanup(pulumidetect.SetRunnerForTest(runner))
				reason = stackload.ReasonBackendUnreachable
			}
			for _, loader := range []struct {
				name string
				load OverviewPipelineLoader
			}{
				{"plain", plainOverviewLoader}, {"terminal", tuiOverviewLoader}, {"web", autoDetectOverviewLoader},
			} {
				t.Run(loader.name, func(t *testing.T) {
					params := overviewParams{pulumiJSON: plan, cfg: &config.Config{}}
					pipe := newOverviewPipeline(overviewPipelineConfig{cmd: &cobra.Command{}, params: params})
					passphrase := "operation-secret"
					pipe.passphrase = &passphrase
					data, loadErr := loader.load(context.Background(), pipe)
					require.NoError(t, loadErr)
					require.NotNil(t, data.Snapshot)
					assert.True(t, data.Snapshot.PlanStatus.Loaded)
					assert.False(t, data.Snapshot.StateStatus.Loaded)
					assert.Equal(t, reason, data.Snapshot.StateStatus.Reason)
					assert.Equal(t, project, data.ProjectDir)
					assert.NotEmpty(t, data.Rows)
					assert.NotContains(t, data.DetectErrMsg, "private.invalid")
					if failure == "backend unavailable" {
						assert.Contains(t, runner.exportEnv, "PULUMI_CONFIG_PASSPHRASE="+passphrase)
					}
				})
			}
			for _, call := range runner.calls {
				assert.Contains(t, call, "stack export")
				assert.NotContains(t, call, "stack ls")
			}
		})
	}
}
