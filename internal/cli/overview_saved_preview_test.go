package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/stackload"
	"github.com/rshade/finfocus/internal/webui"
)

func TestOverviewPreviewOnlyWarningReachesPlainOutput(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	const warning = "Stack state unavailable (backend_unreachable); 4 unchanged resources omitted."
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd:    cmd,
		params: overviewParams{cfg: &config.Config{}},
		loader: func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
			return &OverviewPipelineData{
				Rows: fakePipelineRows(), HasChanges: true, SourceWarning: warning,
			}, nil
		},
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	var events pipelineEvents
	events.attach(pipe)
	require.NoError(t, pipe.Run(t.Context()))
	t.Cleanup(pipe.Cleanup())
	assert.Contains(t, stderr.String(), warning)
	assert.Empty(t, events.stateOnly, "preview-only warnings must not enable a state-only preview action")
	require.Len(t, events.ready, 1)
	assert.True(t, events.ready[0].HasChanges)
	assert.NotEmpty(t, events.ready[0].Rows)
}

func TestOverviewPreviewOnlyWarningPersistsInWebSession(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	const warning = "Stack state unavailable (backend_unreachable); 4 unchanged resources omitted."
	params := overviewParams{cfg: &config.Config{}}
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: &cobra.Command{}, params: params,
		loader: func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
			return &OverviewPipelineData{
				Rows: fakePipelineRows(), HasChanges: true, SourceWarning: warning,
			}, nil
		},
		openPlugins: fakeOpenPlugins, newEngine: fakeNewEngine, enrichRows: passThroughEnrichment,
	})
	session := webui.NewSession(t.Context(), webui.SessionOptions{})
	t.Cleanup(session.Close)
	bindWebPipeline(t.Context(), pipe, session, params, engine.DateRange{})
	require.NoError(t, pipe.Run(t.Context()))
	t.Cleanup(pipe.Cleanup())
	server, err := webui.New(webui.Options{Session: session})
	require.NoError(t, err)
	require.NoError(t, server.Listen())
	served := make(chan error, 1)
	go func() { served <- server.Serve() }()
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown(context.Background()))
		require.ErrorIs(t, <-served, http.ErrServerClosed)
	})
	origin, cookie := webHTTP(t, server.URL())
	response := webAPI(t, origin, cookie, "/api/overview/stream", "")
	event, snapshot := webEvent(t, bufio.NewReader(response.Body))
	assert.Equal(t, "snapshot", event)
	assert.Equal(t, true, snapshot["ready"], "incomplete inventory must remain usable")
	assert.Equal(t, warning, snapshot["sourceWarning"])
	assert.Equal(t, false, snapshot["stateOnly"])
}

type savedPreviewUnavailableRunner struct {
	calls     []string
	exportEnv []string
}

func TestOverviewDoesNotPublishDetectionErrorsAsSourceWarnings(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd, params: overviewParams{cfg: &config.Config{}},
		loader: func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
			return &OverviewPipelineData{
				Rows: fakePipelineRows(), HasChanges: true,
				DetectErrMsg: "read /private/project/secret.yaml: permission denied",
			}, nil
		},
		openPlugins: fakeOpenPlugins, newEngine: fakeNewEngine, enrichRows: passThroughEnrichment,
	})
	var warnings []string
	pipe.OnSourceWarning = func(message string) { warnings = append(warnings, message) }
	require.NoError(t, pipe.Run(t.Context()))
	t.Cleanup(pipe.Cleanup())
	assert.Empty(t, warnings, "raw detection errors must not reach a web source-warning callback")
	assert.Empty(t, stderr.String())
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
			preview, err := os.ReadFile(plan)
			require.NoError(t, err)
			plan = filepath.Join(t.TempDir(), "preview.json")
			require.NoError(t, os.WriteFile(plan,
				bytes.ReplaceAll(preview, []byte(`"same": 1`), []byte(`"same": 5`)), 0600))
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
					assert.False(t, data.IsStateOnly, "a preview-only fallback must retain its planned changes")
					assert.Contains(t, data.SourceWarning, "state unavailable")
					assert.Contains(t, data.SourceWarning, reason)
					assert.Contains(t, data.SourceWarning, "unchanged resources")
					assert.Contains(t, data.SourceWarning, "4 omitted by preview")
					assert.NotContains(t, data.SourceWarning, "private.invalid")
					assert.NotContains(t, data.SourceWarning, "secret")
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
