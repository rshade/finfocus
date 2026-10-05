package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/tui"
)

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// nWriter succeeds for left writes, then returns err.
type nWriter struct {
	left int
	err  error
}

func (w *nWriter) Write(p []byte) (int, error) {
	if w.left == 0 {
		return 0, w.err
	}
	w.left--
	return len(p), nil
}

type scriptedPulumi struct {
	export  []byte
	preview []byte
}

func (s scriptedPulumi) Run(
	_ context.Context, _, name string, _ []string, args ...string,
) ([]byte, []byte, error) {
	if name != "pulumi" {
		return nil, []byte(name), errors.New("unexpected command")
	}
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "stack ls"):
		return []byte(`[{"name":"dev","current":true}]`), nil, nil
	case strings.Contains(joined, "stack export"):
		return s.export, nil, nil
	case strings.Contains(joined, "preview"):
		return s.preview, nil, nil
	default:
		return nil, []byte(joined), errors.New("unexpected pulumi args")
	}
}

func overviewFixture(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "overview", name)
}

func stoppedProgram(t *testing.T) *tea.Program {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return tea.NewProgram(
		testNoopModel{},
		tea.WithContext(ctx),
		tea.WithoutRenderer(),
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)
}

func overviewCmd(in io.Reader) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(in)
	cmd.SetContext(context.Background())
	return cmd, out
}

func TestShortErrMsg(t *testing.T) {
	t.Parallel()

	assert.Empty(t, shortErrMsg(nil))
	assert.Equal(t, "boom", shortErrMsg(errors.New("boom")))
	long := strings.Repeat("á", 61)
	assert.Equal(t, strings.Repeat("á", 60), shortErrMsg(errors.New(long)))
}

func TestSumOverviewProjectedCost_SkipsNil(t *testing.T) {
	t.Parallel()

	got := sumOverviewProjectedCost([]engine.OverviewRow{
		{ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 1.5}},
		{},
		{ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 2}},
	})
	assert.InDelta(t, 3.5, got, 0.001)
	assert.Zero(t, sumOverviewProjectedCost(nil))
}

func TestDeepCopyAny_SliceAndScalar(t *testing.T) {
	t.Parallel()

	copied := deepCopyAny([]any{map[string]any{"n": 1}, "x"})
	slice, ok := copied.([]any)
	require.True(t, ok)
	require.Len(t, slice, 2)
	nested, ok := slice[0].(map[string]any)
	require.True(t, ok)
	nested["n"] = 9
	assert.Equal(t, "x", slice[1])
	assert.Equal(t, "plain", deepCopyAny("plain"))
}

func TestPromptForPreview_Branches(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	readErr := errors.New("read failed")

	tests := []struct {
		name       string
		w          io.Writer
		r          io.Reader
		signal     pulumidetect.ChangeSignal
		skip       bool
		want       bool
		wantErr    string
		wantOutput string
	}{
		{name: "yes skips the prompt", w: &bytes.Buffer{}, r: strings.NewReader(""), skip: true, want: true},
		{
			name: "first deploy runs preview", w: &bytes.Buffer{}, r: strings.NewReader(""),
			signal: pulumidetect.ChangeSignal{IsFirstDeploy: true}, want: true,
			wantOutput: "not been deployed",
		},
		{
			name: "first deploy write error", w: errWriter{err: writeErr},
			signal: pulumidetect.ChangeSignal{IsFirstDeploy: true}, wantErr: "writing to output",
		},
		{
			name: "no likely changes skips preview", w: &bytes.Buffer{}, r: strings.NewReader("n\n"),
			signal: pulumidetect.ChangeSignal{}, want: false,
		},
		{
			name: "empty answer accepts", w: &bytes.Buffer{}, r: strings.NewReader(""),
			signal: pulumidetect.ChangeSignal{HasLikelyChanges: true, ModifiedFiles: []string{"index.ts"}},
			want:   true, wantOutput: "index.ts",
		},
		{
			name: "explicit no declines", w: &bytes.Buffer{}, r: strings.NewReader("no\n"),
			signal: pulumidetect.ChangeSignal{HasLikelyChanges: true}, want: false,
		},
		{
			name: "yes word accepts", w: &bytes.Buffer{}, r: strings.NewReader("yes\n"),
			signal: pulumidetect.ChangeSignal{HasLikelyChanges: true}, want: true,
		},
		{
			name: "read error", w: &bytes.Buffer{}, r: errReader{err: readErr},
			signal: pulumidetect.ChangeSignal{HasLikelyChanges: true}, wantErr: "reading user input",
		},
		{
			name: "prompt write error", w: errWriter{err: writeErr},
			signal: pulumidetect.ChangeSignal{HasLikelyChanges: true}, wantErr: "writing to output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := promptForPreview(tt.w, tt.r, tt.signal, tt.skip)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.wantOutput != "" {
				buf, ok := tt.w.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, buf.String(), tt.wantOutput)
			}
		})
	}
}

func TestRenderOverviewOutput_Formats(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{{
		URN: "urn:pulumi:prod::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket",
		Status: engine.StatusActive,
	}}
	stack := engine.StackContext{StackName: "prod", TotalResources: 1}
	for _, format := range []string{outputFormatTable, outputFormatJSON, "ndjson"} {
		cmd, out := overviewCmd(strings.NewReader(""))
		err := renderOverviewOutput(cmd, format, rows, stack, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, out.String())
	}
	cmd, _ := overviewCmd(strings.NewReader(""))
	err := renderOverviewOutput(cmd, "yaml", nil, stack, nil)
	require.ErrorContains(t, err, "unsupported output format")
}

func TestValidateAndApplyOverviewFilters_Edges(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{URN: "a", Type: "aws:ec2/instance:Instance", Status: engine.StatusActive},
		{URN: "b", Type: "aws:s3/bucket:Bucket", Status: engine.StatusDeleting},
	}
	kept, err := validateAndApplyOverviewFilters(rows, nil)
	require.NoError(t, err)
	assert.Len(t, kept, 2)

	_, err = validateAndApplyOverviewFilters(rows, []string{"=value"})
	require.ErrorContains(t, err, "non-empty")
	_, err = validateAndApplyOverviewFilters(rows, []string{"type="})
	require.ErrorContains(t, err, "non-empty")
	_, err = validateAndApplyOverviewFilters(rows, []string{"nope"})
	require.ErrorContains(t, err, "expected key=value")
	_, err = validateAndApplyOverviewFilters(rows, []string{"region=us-east-1"})
	require.ErrorContains(t, err, "unknown filter key")

	filtered, err := validateAndApplyOverviewFilters(rows, []string{
		"provider=aws", "type=aws:s3/bucket:Bucket", "status=deleting",
	})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "b", filtered[0].URN)

	assert.False(t, matchesOverviewFilters(rows[0], []string{"type=other"}))
	assert.False(t, matchesOverviewFilters(rows[0], []string{"status=deleting"}))
	assert.False(t, matchesOverviewFilters(rows[0], []string{"provider=gcp"}))
	assert.True(t, matchesOverviewFilters(rows[0], []string{"not-a-filter"}))
	assert.Equal(t, []string{"plain"}, splitFilter("plain"))
}

func TestBuildOverviewRowsAndPlanHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := []engine.StateResource{{
		URN: "urn:pulumi:prod::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket",
		Custom: true,
	}}
	rows, steps, hasChanges, count, err := buildOverviewRows(ctx, true, state, overviewParams{}, "", "", nil)
	require.NoError(t, err)
	assert.Nil(t, steps)
	assert.False(t, hasChanges)
	assert.Zero(t, count)
	require.Len(t, rows, 1)

	missing := overviewParams{pulumiJSON: filepath.Join(t.TempDir(), "missing.json")}
	_, _, _, _, err = buildOverviewRows(ctx, false, state, missing, t.TempDir(), "dev", nil)
	require.Error(t, err)

	planPath := overviewFixture(t, "plan-no-changes.json")
	params := overviewParams{pulumiJSON: planPath}
	loaded, err := loadPlanForOverview(ctx, params, t.TempDir(), "dev", nil)
	require.NoError(t, err)
	assert.Empty(t, loaded)

	_, err = resolveOverviewPlan(ctx, filepath.Join(t.TempDir(), "bad.json"), t.TempDir(), "dev", nil)
	require.ErrorContains(t, err, "loading Pulumi plan")

	cmd := buildPreviewCmd(ctx, missing, t.TempDir(), "dev", nil)
	msg := cmd()
	ready, ok := msg.(tui.OverviewChangesReadyMsg)
	require.True(t, ok)
	assert.False(t, ready.HasChanges)
}

func TestBuildPreviewCmd_FailedPreview(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	params := overviewParams{pulumiJSON: filepath.Join(t.TempDir(), "missing.json"), cfg: &config.Config{}}
	msg := runBackgroundPreview(ctx, params, t.TempDir(), "dev", nil)
	assert.False(t, msg.HasChanges)
	assert.Empty(t, msg.StatusByURN)
}

func TestLoadPlainOverviewData_FileAndStateOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cmd, _ := overviewCmd(strings.NewReader(""))
	statePath := overviewFixture(t, "state-no-changes.json")
	planPath := overviewFixture(t, "plan-no-changes.json")

	state, steps, stack, stateOnly, err := loadPlainOverviewData(ctx, cmd, overviewParams{
		pulumiState: statePath, pulumiJSON: planPath,
	})
	require.NoError(t, err)
	assert.False(t, stateOnly)
	assert.NotEmpty(t, stack)
	assert.NotEmpty(t, state)
	assert.Empty(t, steps)

	state, steps, _, stateOnly, err = loadPlainOverviewData(ctx, cmd, overviewParams{
		pulumiState: statePath, stateOnly: true,
	})
	require.NoError(t, err)
	assert.True(t, stateOnly)
	assert.Nil(t, steps)
	assert.NotEmpty(t, state)

	_, _, _, _, err = loadPlainOverviewData(ctx, cmd, overviewParams{
		pulumiState: filepath.Join(t.TempDir(), "missing.json"), stateOnly: true,
	})
	require.ErrorContains(t, err, "loading Pulumi state")
}

func TestExtractStackNameAndFormatDiff(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "unknown", extractStackName(""))
	assert.Equal(t, "unknown", extractStackName("."))
	assert.Equal(t, "stack", extractStackName("dir/stack.json"))
	assert.Empty(t, formatDiffValue(nil))
	ch := make(chan int)
	assert.Equal(t, fmt.Sprintf("%v", ch), formatDiffValue(ch))
}

func TestLaunchBudgetFetch_EmptyEngine(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := launchBudgetFetch(ctx, engine.New(nil, nil))
	select {
	case got := <-ch:
		require.NoError(t, got.err)
		require.NotNil(t, got.result)
		assert.Empty(t, got.result.Budgets)
	case <-ctx.Done():
		t.Fatal("budget fetch did not return")
	}
}

func TestSendBudgetResultToTUI_PluginAndEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	prog := stoppedProgram(t)
	pluginCh := make(chan budgetFetchResult, 1)
	pluginCh <- budgetFetchResult{result: &engine.BudgetResult{Budgets: []*pbc.Budget{{}}}}
	sendBudgetResultToTUI(ctx, prog, pluginCh, nil)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	sendBudgetResultToTUI(cancelled, prog, make(chan budgetFetchResult), nil)
}

func TestBridgeEnrichmentToTUI_OneRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var count atomic.Int64
	rows := []engine.OverviewRow{{
		URN: "urn:pulumi:prod::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket",
		Status: engine.StatusActive,
	}}
	bridgeEnrichmentToTUI(ctx, stoppedProgram(t), rows, engine.New(nil, nil), engine.DateRange{
		Start: time.Now().Add(-48 * time.Hour), End: time.Now(),
	}, &count)
	assert.Equal(t, int64(1), count.Load())
}

func TestBridgeEnrichmentToTUI_CancelledBeforeProgress(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var count atomic.Int64
	rows := []engine.OverviewRow{{
		URN: "urn:pulumi:prod::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket",
		Status: engine.StatusActive,
	}}
	bridgeEnrichmentToTUI(ctx, stoppedProgram(t), rows, engine.New(nil, nil), engine.DateRange{
		Start: time.Now().Add(-48 * time.Hour), End: time.Now(),
	}, &count)
	assert.Zero(t, count.Load())
}

func unsetPassphraseEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PULUMI_CONFIG_PASSPHRASE", "PULUMI_CONFIG_PASSPHRASE_FILE"} {
		if orig, ok := os.LookupEnv(key); ok {
			t.Setenv(key, orig)
		}
		require.NoError(t, os.Unsetenv(key))
	}
}

func installFakePulumi(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	name := "pulumi"
	if runtime.GOOS == goosWindows {
		name += ".exe"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExportAndPreview_MockedPulumi(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	require.NoError(t, os.WriteFile(
		filepath.Join(project, "Pulumi.yaml"),
		[]byte("name: test\nruntime: yaml\n"),
		0o600,
	))

	export, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	preview, err := os.ReadFile(overviewFixture(t, "plan-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: export, preview: preview}))

	ctx := context.Background()
	resources, manifest, projectDir, stack, err := exportStateFromProject(ctx, "dev", nil)
	require.NoError(t, err)
	assert.Equal(t, project, projectDir)
	assert.Equal(t, "dev", stack)
	assert.NotEmpty(t, manifest)
	assert.NotEmpty(t, resources)

	state, steps, name, err := loadOverviewFromAutoDetect(ctx, overviewParams{stack: "dev"})
	require.NoError(t, err)
	assert.Equal(t, "dev", name)
	assert.NotEmpty(t, state)
	assert.Empty(t, steps)

	parsed, err := resolveOverviewPlan(ctx, "", project, "dev", nil)
	require.NoError(t, err)
	assert.Empty(t, parsed)

	pw := "secret"
	withPass, err := resolveOverviewPlan(ctx, "", project, "dev", &pw)
	require.NoError(t, err)
	assert.Empty(t, withPass)
}

//nolint:paralleltest // changes the working directory
func TestExportStateFromProject_NoProject(t *testing.T) {
	t.Chdir(t.TempDir())
	installFakePulumi(t)
	_, _, _, _, err := exportStateFromProject(context.Background(), "dev", nil)
	require.ErrorContains(t, err, "auto-detecting Pulumi project")
}

//nolint:paralleltest // reads and clears passphrase environment variables
func TestCheckAndPromptPassphrase_SkipBranches(t *testing.T) {
	prog := stoppedProgram(t)
	ctx := context.Background()

	t.Run("env passphrase", func(t *testing.T) {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "secret")
		pw, err := checkAndPromptPassphrase(ctx, prog, overviewParams{}, make(chan string, 1))
		require.NoError(t, err)
		require.NotNil(t, pw)
		assert.Equal(t, "secret", *pw)
	})

	t.Run("passphrase file", func(t *testing.T) {
		unsetPassphraseEnv(t)
		t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", filepath.Join(t.TempDir(), "pw"))
		pw, err := checkAndPromptPassphrase(ctx, prog, overviewParams{}, make(chan string, 1))
		require.NoError(t, err)
		assert.Nil(t, pw)
	})

	t.Run("explicit state file", func(t *testing.T) {
		unsetPassphraseEnv(t)
		pw, err := checkAndPromptPassphrase(ctx, prog, overviewParams{pulumiState: "state.json"}, make(chan string, 1))
		require.NoError(t, err)
		assert.Nil(t, pw)
	})

	t.Run("no project", func(t *testing.T) {
		unsetPassphraseEnv(t)
		t.Chdir(t.TempDir())
		pw, err := checkAndPromptPassphrase(ctx, prog, overviewParams{}, make(chan string, 1))
		require.NoError(t, err)
		assert.Nil(t, pw)
	})
}

func TestSendBudgetResultToTUI_ConfigFallback(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Cleanup(config.ResetGlobalConfigForTest)
	amount := 10.0
	config.SetGlobalConfig(&config.Config{
		Cost: config.CostConfig{
			Budgets: &config.BudgetsConfig{
				Global: &config.ScopedBudget{Amount: amount, Currency: "USD"},
			},
		},
	})
	prog := stoppedProgram(t)
	ch := make(chan budgetFetchResult, 1)
	ch <- budgetFetchResult{result: &engine.BudgetResult{}, err: errors.New("no plugin budgets")}
	rows := []engine.OverviewRow{{ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 3}}}
	sendBudgetResultToTUI(context.Background(), prog, ch, rows)

	config.SetGlobalConfig(&config.Config{})
	empty := make(chan budgetFetchResult, 1)
	empty <- budgetFetchResult{result: &engine.BudgetResult{}}
	sendBudgetResultToTUI(context.Background(), prog, empty, rows)
}

func TestRunBackgroundPreview_PlanFile(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	msg := runBackgroundPreview(context.Background(), overviewParams{
		pulumiJSON: overviewFixture(t, "plan-no-changes.json"),
		cfg:        &config.Config{},
	}, t.TempDir(), "dev", nil)
	assert.False(t, msg.HasChanges)
	assert.Empty(t, msg.StatusByURN)
}

func TestLoadAndProcessPlainOverview_StateFile(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()
	cmd, _ := overviewCmd(strings.NewReader(""))
	audit := newAuditContext(ctx, "overview", nil)
	params := overviewParams{
		pulumiState: overviewFixture(t, "state-no-changes.json"),
		stateOnly:   true,
		cfg:         &config.Config{},
	}
	rows, stack, hasChanges, count, stateOnly, err := loadAndProcessPlainOverview(ctx, cmd, params, audit)
	require.NoError(t, err)
	assert.True(t, stateOnly)
	assert.False(t, hasChanges)
	assert.Zero(t, count)
	assert.NotEmpty(t, stack)
	assert.NotEmpty(t, rows)

	params.filter = []string{"region=us-east-1"}
	_, _, _, _, _, err = loadAndProcessPlainOverview(ctx, cmd, params, audit)
	require.ErrorContains(t, err, "unknown filter key")

	params.filter = nil
	params.pulumiState = filepath.Join(t.TempDir(), "missing.json")
	_, _, _, _, _, err = loadAndProcessPlainOverview(ctx, cmd, params, audit)
	require.ErrorContains(t, err, "resolve overview data")
}

func TestExecuteOverview_PlainStateOnly(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd, out := overviewCmd(strings.NewReader(""))
	cmd.SetContext(ctx)
	err := executeOverview(cmd, overviewParams{
		pulumiState: overviewFixture(t, "state-no-changes.json"),
		stateOnly:   true,
		plain:       true,
		yes:         true,
		output:      outputFormatTable,
		cfg:         &config.Config{},
	})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "RESOURCE")

	bad, _ := overviewCmd(strings.NewReader(""))
	err = executeOverview(bad, overviewParams{output: "yaml", plain: true, exitCode: 1, cfg: &config.Config{}})
	require.ErrorContains(t, err, "unsupported output format")

	badCode, _ := overviewCmd(strings.NewReader(""))
	err = executeOverview(badCode, overviewParams{
		output: outputFormatTable, plain: true, exitCode: 999, cfg: &config.Config{},
	})
	require.ErrorContains(t, err, "exit-code")
}

func TestOverviewInitAndEnrich_StateOnlyFile(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd, _ := overviewCmd(strings.NewReader(""))
	cleanupChan := make(chan func(), 1)
	var rowCount atomic.Int64
	overviewInitAndEnrich(
		ctx, cmd, stoppedProgram(t),
		overviewParams{
			pulumiState: overviewFixture(t, "state-no-changes.json"),
			stateOnly:   true,
			output:      outputFormatTable,
			cfg:         &config.Config{},
		},
		engine.DateRange{Start: time.Now().Add(-72 * time.Hour), End: time.Now()},
		newAuditContext(ctx, "overview", nil),
		cleanupChan, &rowCount, make(chan string, 1),
	)
	select {
	case cleanup := <-cleanupChan:
		if cleanup != nil {
			cleanup()
		}
	default:
	}
	assert.Positive(t, rowCount.Load())
}

func TestReadStackSettingsFile_EmptyAndMissing(t *testing.T) {
	t.Parallel()

	assert.Nil(t, stackSettingsNameCandidates("  "))
	_, err := readStackSettingsFile(t.TempDir(), "  ")
	require.ErrorContains(t, err, "stack name is required")

	_, err = readStackSettingsFile(t.TempDir(), "dev")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestProjectedPropertiesFromStep_LegacyInputs(t *testing.T) {
	t.Parallel()

	got := projectedPropertiesFromStep(ingest.PulumiStep{
		Inputs:   map[string]any{"instanceType": "t3.micro"},
		NewState: &ingest.PulumiState{},
	})
	require.NotNil(t, got)
	assert.Equal(t, "t3.micro", got["instanceType"])
}

func TestOverviewSummaryAndConfirm_WriteErrors(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	_, err := printOverviewSummaryLine(errWriter{err: writeErr}, strings.NewReader(""), false, true, 1, false, 0, 0)
	require.ErrorContains(t, err, "writing overview summary")

	_, err = printOverviewSummaryLine(&bytes.Buffer{}, errReader{err: writeErr}, false, true, 1, true, 1, 1)
	require.ErrorContains(t, err, "reading confirmation")

	_, err = printOverviewSummaryLine(
		&nWriter{left: 2, err: writeErr}, strings.NewReader("n\n"), false, true, 1, false, 0, 0,
	)
	require.ErrorContains(t, err, "writing cancellation")

	cmd, _ := overviewCmd(strings.NewReader(""))
	cmd.SetOut(errWriter{err: writeErr})
	stop, err := confirmPlainOverview(cmd, overviewParams{output: outputFormatTable}, 1, false, 0, 1, false)
	require.ErrorContains(t, err, "pre-flight prompt")
	assert.False(t, stop)
}

func TestFinalizeOverviewOutput_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cmd, _ := overviewCmd(strings.NewReader(""))
	err := finalizeOverviewOutput(
		ctx, cmd, overviewParams{output: "yaml"}, nil, engine.New(nil, nil),
		engine.DateRange{}, "dev", false, 0, false, nil, newAuditContext(ctx, "overview", nil),
	)
	require.ErrorContains(t, err, "unsupported output format")
}

func TestApplyOverviewFilters_EmptyKeepsRows(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{{URN: "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs"}}
	assert.Equal(t, rows, applyOverviewFilters(rows, nil))
}

func TestMatchesOverviewFilters_UnexpectedKeyPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		matchesOverviewFilters(engine.OverviewRow{Type: "aws:s3/bucket:Bucket"}, []string{"region=us-east-1"})
	})
}

func TestPromptForPreview_SecondWriteFails(t *testing.T) {
	t.Parallel()

	ok, err := promptForPreview(
		&nWriter{left: 1, err: errors.New("write failed")},
		strings.NewReader("y\n"),
		pulumidetect.ChangeSignal{HasLikelyChanges: true, ModifiedFiles: []string{"index.ts"}},
		false,
	)
	require.ErrorContains(t, err, "writing to output")
	assert.False(t, ok)
}
