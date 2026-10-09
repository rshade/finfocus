package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/tui"
)

// pipelineEvents records every callback a pipeline emits, in emission order.
type pipelineEvents struct {
	phases        []int
	phaseNames    []string
	dataReady     []engine.OverviewRow
	dataReadyName string
	stateOnly     []string
	rows          map[int]engine.OverviewRow
	rowOrder      []int
	progress      [][2]int
	allLoaded     int
	expansions    [][]engine.OverviewRow
	expansionNote [][]string
	budgets       []*engine.BudgetResult
	budgetErrs    []error
	errors        []int
	errs          []error
	ready         []OverviewPipelineResult
	passphraseReq int
	previewReady  int
}

func (e *pipelineEvents) attach(p *OverviewPipeline) {
	p.OnPhase = func(phase int, name string) {
		e.phases = append(e.phases, phase)
		e.phaseNames = append(e.phaseNames, name)
	}
	p.OnDataReady = func(rows []engine.OverviewRow, _ int, stackName string) {
		e.dataReady = rows
		e.dataReadyName = stackName
	}
	p.OnStateOnly = func(detectErrMsg string) { e.stateOnly = append(e.stateOnly, detectErrMsg) }
	p.OnRow = func(index int, row engine.OverviewRow) {
		if e.rows == nil {
			e.rows = map[int]engine.OverviewRow{}
		}
		e.rows[index] = row
		e.rowOrder = append(e.rowOrder, index)
	}
	p.OnProgress = func(loaded, total int) { e.progress = append(e.progress, [2]int{loaded, total}) }
	p.OnAllRowsLoaded = func() { e.allLoaded++ }
	p.OnExpansion = func(rows []engine.OverviewRow, notes []string) {
		e.expansions = append(e.expansions, rows)
		e.expansionNote = append(e.expansionNote, notes)
	}
	p.OnBudget = func(result *engine.BudgetResult, err error) {
		e.budgets = append(e.budgets, result)
		e.budgetErrs = append(e.budgetErrs, err)
	}
	p.OnError = func(phase int, err error) {
		e.errors = append(e.errors, phase)
		e.errs = append(e.errs, err)
	}
	p.OnReady = func(result OverviewPipelineResult) { e.ready = append(e.ready, result) }
	p.OnPassphraseRequired = func() { e.passphraseReq++ }
	p.OnPreviewReady = func(_ tui.OverviewChangesReadyMsg) { e.previewReady++ }
}

// writePulumiProject writes a minimal Pulumi project (and, when encrypted is
// true, a dev stack settings file carrying an encryptionsalt) into dir.
func writePulumiProject(t *testing.T, dir string, encrypted bool) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.yaml"), []byte("name: test\nruntime: yaml\n"), 0o600,
	))
	if encrypted {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "Pulumi.dev.yaml"), []byte("encryptionsalt: v1:abc123\n"), 0o600,
		))
	}
}

// fakePipelineLoader emits phases 1-3 through the pipeline and returns the
// given rows, mimicking a completed stack load.
func fakePipelineLoader(rows []engine.OverviewRow) OverviewPipelineLoader {
	return func(_ context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
		p.emitPhase(OverviewPhaseLoadStackState, overviewPhaseLoadStackStateName)
		p.emitPhase(OverviewPhaseDetectChanges, overviewPhaseDetectChangesName)
		p.emitPhase(OverviewPhaseMergeResources, overviewPhaseMergeResourcesName)
		return &OverviewPipelineData{
			Rows:       rows,
			StackName:  "dev",
			ProjectDir: ".",
		}, nil
	}
}

func fakePipelineRows() []engine.OverviewRow {
	return []engine.OverviewRow{
		{URN: "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket",
			Status: engine.StatusActive},
		{URN: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", Type: "aws:ec2/instance:Instance",
			Status: engine.StatusActive},
	}
}

// passThroughEnrichment mimics the engine.EnrichOverviewRows contract: one
// update per row on the channel, closed before returning.
func passThroughEnrichment(
	_ context.Context,
	rows []engine.OverviewRow,
	_ *engine.Engine,
	_ engine.DateRange,
	ch chan<- engine.OverviewRowUpdate,
) []engine.OverviewRow {
	for i := range rows {
		ch <- engine.OverviewRowUpdate{Index: i, Row: rows[i]}
	}
	close(ch)
	return rows
}

func fakeNewEngine(
	_ context.Context, _ *cobra.Command, _ []*pluginhost.Client,
) (*engine.Engine, func()) {
	return engine.New(nil, nil), func() {}
}

func fakeOpenPlugins(
	_ context.Context, _ string, _ *auditContext,
) ([]*pluginhost.Client, func(), error) {
	return nil, func() {}, nil
}

func TestOverviewPipeline_EngineAvailableBeforeDataReady(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:      overviewParams{cfg: &config.Config{}},
		loader:      fakePipelineLoader(fakePipelineRows()),
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	var readyEngine *engine.Engine
	pipe.OnDataReady = func(_ []engine.OverviewRow, _ int, _ string) {
		readyEngine = pipe.Engine()
	}
	require.NoError(t, pipe.Run(context.Background()))
	require.NotNil(t, readyEngine, "data-ready consumers need the constructed engine")
	assert.Same(t, pipe.Result().Engine, readyEngine)
}

// TestOverviewPipeline_PhasesInOrder runs the real TUI loader against a
// state-only fixture and asserts phase events 1..6 arrive in order.
func TestOverviewPipeline_PhasesInOrder(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd, _ := overviewCmd(nil)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd,
		params: overviewParams{
			pulumiState: overviewFixture(t, "state-no-changes.json"),
			stateOnly:   true,
			cfg:         &config.Config{},
		},
		dateRange: engine.DateRange{Start: time.Now().Add(-72 * time.Hour), End: time.Now()},
		audit:     newAuditContext(ctx, "overview", nil),
		loader:    tuiOverviewLoader,
	})
	events := &pipelineEvents{}
	events.attach(pipe)

	require.NoError(t, pipe.Run(ctx))
	require.Empty(t, events.errors, "no error events expected: %v", events.errs)
	assert.Equal(t,
		[]int{
			OverviewPhaseLoadStackState, OverviewPhaseDetectChanges, OverviewPhaseMergeResources,
			OverviewPhaseStartPlugins, OverviewPhasePrepareEngine, OverviewPhaseEnrichResources,
		},
		events.phases,
		"phase events must be emitted 1..6 in order",
	)
	assert.Equal(t, 1, events.allLoaded)
	require.Len(t, events.ready, 1)
	assert.NotEmpty(t, events.ready[0].Rows)
	assert.Equal(t, "state-no-changes", events.dataReadyName)
	// State-only mode must surface the state-only event exactly once.
	assert.Len(t, events.stateOnly, 1)
	if cleanup := pipe.Cleanup(); cleanup != nil {
		cleanup()
	}
}

// TestOverviewPipeline_RowEventsMatchEnrichment proves each OnRow event
// carries exactly what engine.EnrichOverviewRows produced for that index
// (plus the pipeline's pre-computed delta).
func TestOverviewPipeline_RowEventsMatchEnrichment(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dateRange := engine.DateRange{Start: time.Now().Add(-72 * time.Hour), End: time.Now()}
	cmd, _ := overviewCmd(nil)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd,
		params: overviewParams{
			pulumiState: overviewFixture(t, "state-no-changes.json"),
			stateOnly:   true,
			cfg:         &config.Config{},
		},
		dateRange: dateRange,
		audit:     newAuditContext(ctx, "overview", nil),
		loader:    tuiOverviewLoader,
	})
	events := &pipelineEvents{}
	events.attach(pipe)

	require.NoError(t, pipe.Run(ctx))
	require.NotEmpty(t, events.dataReady, "OnDataReady must deliver the pre-enrichment rows")
	if cleanup := pipe.Cleanup(); cleanup != nil {
		cleanup()
	}

	// Independently enrich the same rows the pipeline started from.
	want := make([]engine.OverviewRow, len(events.dataReady))
	copy(want, events.dataReady)
	want = engine.EnrichOverviewRows(ctx, want, engine.New(nil, nil), dateRange, nil)

	require.Len(t, events.rows, len(want), "one OnRow event per row")
	indices := make([]int, len(want))
	for i := range want {
		indices[i] = i
	}
	assert.ElementsMatch(t, indices, events.rowOrder)
	for i := range want {
		got, ok := events.rows[i]
		require.True(t, ok, "missing OnRow for index %d", i)
		gotDelta := got.ComputedDelta
		got.ComputedDelta = nil
		assert.Equal(t, want[i], got, "row %d must match engine.EnrichOverviewRows output", i)
		if d, dok := engine.CalculateRowDelta(want[i], time.Now().Day()); dok {
			require.NotNil(t, gotDelta, "row %d must carry the pre-computed delta", i)
			assert.InDelta(t, d, *gotDelta, 1e-9)
		}
	}
	// Progress events end at the row total.
	if assert.NotEmpty(t, events.progress) {
		last := events.progress[len(events.progress)-1]
		assert.Equal(t, [2]int{len(want), len(want)}, last)
	}
}

// TestOverviewPipeline_ExpansionEvent verifies the expansion event carries
// exactly the rows expandOverviewClusters returned, and that no event fires
// when expansion changes nothing.
//
//nolint:paralleltest // Subtests share the parent-scoped process-wide environment.
func TestOverviewPipeline_ExpansionEvent(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()

	expanded := append(fakePipelineRows(), engine.OverviewRow{
		URN: "urn:pulumi:dev::app::kubernetes:core/v1:Pod::web-0", Type: "kubernetes:core/v1:Pod",
		Status: engine.StatusActive, ParentURN: "urn:pulumi:dev::app::aws:eks/cluster:Cluster::eks",
		ExpansionSource: engine.ExpansionSourceProjected,
	})
	notes := []string{"live cluster data preferred for eks"}

	tests := []struct {
		name          string
		expandRows    []engine.OverviewRow
		expandNotes   []string
		wantExpansion bool
	}{
		{name: "changed row set emits expansion", expandRows: expanded, expandNotes: notes, wantExpansion: true},
		{name: "unchanged row set emits nothing", expandRows: fakePipelineRows()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipe := newOverviewPipeline(overviewPipelineConfig{
				params:      overviewParams{cfg: &config.Config{}},
				loader:      fakePipelineLoader(fakePipelineRows()),
				audit:       newAuditContext(ctx, "overview", nil),
				openPlugins: fakeOpenPlugins,
				newEngine:   fakeNewEngine,
				enrichRows:  passThroughEnrichment,
				expandClusters: func(
					_ context.Context, _ []engine.OverviewRow, _ []*pluginhost.Client,
					_ engine.ResourcePricer, _ *config.Config,
				) ([]engine.OverviewRow, []string) {
					return tt.expandRows, tt.expandNotes
				},
			})
			events := &pipelineEvents{}
			events.attach(pipe)

			require.NoError(t, pipe.Run(ctx))
			if tt.wantExpansion {
				require.Len(t, events.expansions, 1)
				assert.Equal(t, tt.expandRows, events.expansions[0],
					"expansion event must carry the rows expandOverviewClusters returned")
				assert.Equal(t, tt.expandNotes, events.expansionNote[0])
				require.Len(t, events.ready, 1)
				assert.Equal(t, tt.expandRows, events.ready[0].Rows)
			} else {
				assert.Empty(t, events.expansions)
				require.Len(t, events.ready, 1)
				assert.Len(t, events.ready[0].Rows, len(fakePipelineRows()))
			}
		})
	}
}

// TestOverviewPipeline_ErrorCarriesPhase verifies fatal failures emit exactly
// one error event tagged with the phase that failed, and stop the pipeline.
//
//nolint:paralleltest // Subtests share the parent-scoped process-wide environment.
func TestOverviewPipeline_ErrorCarriesPhase(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()
	boom := errors.New("boom")

	tests := []struct {
		name      string
		mutate    func(*overviewPipelineConfig)
		wantPhase int
	}{
		{
			name: "loader failure at merge phase",
			mutate: func(cfg *overviewPipelineConfig) {
				cfg.loader = func(_ context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
					p.emitPhase(OverviewPhaseLoadStackState, overviewPhaseLoadStackStateName)
					p.emitPhase(OverviewPhaseDetectChanges, overviewPhaseDetectChangesName)
					return nil, &PhaseError{Phase: OverviewPhaseMergeResources, Err: boom}
				}
			},
			wantPhase: OverviewPhaseMergeResources,
		},
		{
			name: "plugin open failure",
			mutate: func(cfg *overviewPipelineConfig) {
				cfg.openPlugins = func(
					_ context.Context, _ string, _ *auditContext,
				) ([]*pluginhost.Client, func(), error) {
					return nil, nil, boom
				}
			},
			wantPhase: OverviewPhaseStartPlugins,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := overviewPipelineConfig{
				params:      overviewParams{cfg: &config.Config{}},
				loader:      fakePipelineLoader(fakePipelineRows()),
				audit:       newAuditContext(ctx, "overview", nil),
				openPlugins: fakeOpenPlugins,
				newEngine:   fakeNewEngine,
				enrichRows:  passThroughEnrichment,
			}
			tt.mutate(&cfg)
			pipe := newOverviewPipeline(cfg)
			events := &pipelineEvents{}
			events.attach(pipe)

			err := pipe.Run(ctx)
			require.ErrorIs(t, err, boom)
			require.Len(t, events.errors, 1, "exactly one error event")
			assert.Equal(t, tt.wantPhase, events.errors[0], "error event must carry the failed phase")
			require.ErrorIs(t, events.errs[0], boom)
			assert.Empty(t, events.ready, "a failed pipeline must not emit ready")
		})
	}
}

// TestOverviewPipeline_BudgetAndReady verifies the budget event fires with
// the plugin fetch result and the ready event carries the final row set.
func TestOverviewPipeline_BudgetAndReady(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	ctx := context.Background()

	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:         overviewParams{cfg: &config.Config{}},
		loader:         fakePipelineLoader(fakePipelineRows()),
		audit:          newAuditContext(ctx, "overview", nil),
		openPlugins:    fakeOpenPlugins,
		newEngine:      fakeNewEngine,
		enrichRows:     passThroughEnrichment,
		wantBudget:     true,
		budgetLive:     true,
		budgetFallback: false,
	})
	events := &pipelineEvents{}
	events.attach(pipe)

	require.NoError(t, pipe.Run(ctx))
	require.Len(t, events.budgets, 1, "budget event must fire once")
	require.NotNil(t, events.budgets[0])
	require.NoError(t, events.budgetErrs[0])
	require.Len(t, events.ready, 1)
	assert.Equal(t, len(fakePipelineRows()), events.ready[0].RowsEnriched)
	assert.Equal(t, len(fakePipelineRows()), events.ready[0].ResourceCount)
}

// TestOverviewPipeline_PassphraseRequired drives an encrypted-stack project
// and asserts the pipeline asks for the passphrase once and threads the
// submitted value to the loader.
//
// Not parallel: changes the working directory and the Pulumi runner.
func TestOverviewPipeline_PassphraseRequired(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")

	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, true)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{}))

	passphrases := make(chan string, 1)
	passphrases <- "stack-secret"
	var gotPassphrase *string
	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:      overviewParams{cfg: &config.Config{}},
		passphrases: passphrases,
		audit:       newAuditContext(context.Background(), "overview", nil),
		loader: func(_ context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
			gotPassphrase = p.Passphrase()
			return &OverviewPipelineData{Rows: fakePipelineRows(), StackName: "dev"}, nil
		},
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	events := &pipelineEvents{}
	events.attach(pipe)

	require.NoError(t, pipe.Run(context.Background()))
	assert.Equal(t, 1, events.passphraseReq, "passphrase_required must fire exactly once")
	require.NotNil(t, gotPassphrase)
	assert.Equal(t, "stack-secret", *gotPassphrase)
}

// TestOverviewPipeline_RunPreview verifies a state-only session exposes the
// on-demand preview and reports the result on the preview-ready callback.
func TestOverviewPipeline_RunPreview(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()

	pipe := newOverviewPipeline(overviewPipelineConfig{
		params: overviewParams{
			pulumiJSON: overviewFixture(t, "plan-no-changes.json"),
			cfg:        &config.Config{},
		},
		audit: newAuditContext(ctx, "overview", nil),
		loader: func(_ context.Context, _ *OverviewPipeline) (*OverviewPipelineData, error) {
			return &OverviewPipelineData{
				Rows: fakePipelineRows(), StackName: "dev", ProjectDir: t.TempDir(), IsStateOnly: true,
			}, nil
		},
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	events := &pipelineEvents{}
	events.attach(pipe)

	require.NoError(t, pipe.Run(ctx))
	require.Len(t, events.stateOnly, 1, "state-only event must fire")

	msg := pipe.RunPreview(ctx)
	assert.False(t, msg.HasChanges)
	assert.Empty(t, msg.StatusByURN)
	assert.Equal(t, 1, events.previewReady, "preview-ready callback must fire once")
}

// TestOverviewPipeline_ConfirmDeclined verifies a declined pre-flight
// confirmation stops the pipeline quietly: no error, no ready, no enrichment.
func TestOverviewPipeline_ConfirmDeclined(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()
	var enriched atomic.Int64

	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:      overviewParams{cfg: &config.Config{}},
		loader:      fakePipelineLoader(fakePipelineRows()),
		audit:       newAuditContext(ctx, "overview", nil),
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows: func(
			ctx context.Context, rows []engine.OverviewRow, eng *engine.Engine,
			dr engine.DateRange, ch chan<- engine.OverviewRowUpdate,
		) []engine.OverviewRow {
			enriched.Add(1)
			return passThroughEnrichment(ctx, rows, eng, dr, ch)
		},
	})
	events := &pipelineEvents{}
	events.attach(pipe)
	pipe.ConfirmEnrichment = func(_ context.Context, _ OverviewPipelineData, clientCount int) (bool, error) {
		assert.Zero(t, clientCount)
		return false, nil
	}

	require.NoError(t, pipe.Run(ctx))
	assert.Zero(t, enriched.Load(), "declined confirmation must skip enrichment")
	assert.Empty(t, events.ready)
	assert.Empty(t, events.errors)
	// Plugins were opened, so a cleanup must be available and safe to call.
	if cleanup := pipe.Cleanup(); cleanup != nil {
		cleanup()
	}
}

// TestOverviewPipeline_AutoDetectLoader runs the web/auto-detect loader
// against a scripted Pulumi project and checks it resolves project, stack,
// and state through the same seam as finfocus overview.
//
// Not parallel: changes the working directory and the Pulumi runner.
func TestOverviewPipeline_AutoDetectLoader(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, false)

	export, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	preview, err := os.ReadFile(overviewFixture(t, "plan-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: export, preview: preview}))

	ctx := context.Background()
	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:      overviewParams{stack: "dev", cfg: &config.Config{}},
		passphrases: make(chan string, 1),
		audit:       newAuditContext(ctx, "overview", nil),
		loader:      autoDetectOverviewLoader,
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	events := &pipelineEvents{}
	events.attach(pipe)

	require.NoError(t, pipe.Run(ctx))
	assert.Equal(t, "dev", events.dataReadyName)
	assert.NotEmpty(t, events.dataReady)
	require.Len(t, events.ready, 1)
	assert.Equal(t, "dev", events.ready[0].StackName)
	assert.Equal(t, project, events.ready[0].ProjectDir)
}

func TestOverviewPipeline_CancellationWaitsForEnrichment(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerStarted := make(chan struct{})
	releaseWorker := make(chan struct{})
	expanded := false
	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:      overviewParams{cfg: &config.Config{}},
		loader:      fakePipelineLoader(fakePipelineRows()),
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows: func(_ context.Context, rows []engine.OverviewRow, _ *engine.Engine,
			_ engine.DateRange, ch chan<- engine.OverviewRowUpdate) []engine.OverviewRow {
			cancel()
			ch <- engine.OverviewRowUpdate{Index: 0, Row: rows[0]}
			close(ch)
			close(workerStarted)
			<-releaseWorker
			return rows
		},
		expandClusters: func(_ context.Context, rows []engine.OverviewRow, _ []*pluginhost.Client,
			_ engine.ResourcePricer, _ *config.Config) ([]engine.OverviewRow, []string) {
			expanded = true
			return rows, nil
		},
	})
	events := &pipelineEvents{}
	events.attach(pipe)
	done := make(chan error, 1)
	go func() { done <- pipe.Run(ctx) }()
	<-workerStarted
	select {
	case err := <-done:
		assert.Fail(t, "pipeline returned before enrichment finished", "error: %v", err)
		done <- err
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseWorker)
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not finish after enrichment stopped")
	}
	assert.False(t, expanded, "cancelled enrichment must not expand clusters")
	assert.Empty(t, events.ready, "cancelled enrichment must not report success")
	assert.Equal(t, []int{OverviewPhaseEnrichResources}, events.errors)
	assert.Zero(t, events.allLoaded)
}

func TestOverviewPipeline_ExplicitStateWithPassphrase(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test-passphrase")
	t.Chdir(t.TempDir())
	pipe := newOverviewPipeline(overviewPipelineConfig{
		params:      overviewParams{pulumiState: overviewFixture(t, "state-no-changes.json"), cfg: &config.Config{}},
		loader:      autoDetectOverviewLoader,
		passphrases: make(chan string, 1),
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	require.NoError(t, pipe.Run(context.Background()))
	assert.NotEmpty(t, pipe.Result().Rows)
	assert.Empty(t, pipe.Result().ProjectDir)
}

func TestOverviewPipeline_PreviewFileKeepsProjectIdentity(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test-passphrase")
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, false)
	export, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: export}))
	pipe := newOverviewPipeline(overviewPipelineConfig{
		params: overviewParams{
			pulumiJSON: overviewFixture(t, "plan-no-changes.json"), stack: "dev", cfg: &config.Config{},
		},
		loader:      autoDetectOverviewLoader,
		passphrases: make(chan string, 1),
		openPlugins: fakeOpenPlugins,
		newEngine:   fakeNewEngine,
		enrichRows:  passThroughEnrichment,
	})
	require.NoError(t, pipe.Run(context.Background()))
	assert.Equal(t, project, pipe.Result().ProjectDir)
	assert.Equal(t, "dev", pipe.Result().StackName)
	assert.NotEmpty(t, pipe.Result().Rows)
}

// Changes cwd and passphrase environment for encrypted stack detection.
func TestOverviewPipelineRetriesRejectedPassphrase(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	unsetPassphraseEnv(t)
	project := t.TempDir()
	t.Chdir(project)
	writePulumiProject(t, project, true)
	installFakePulumi(t)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{}))
	passphrases := make(chan string, 2)
	passphrases <- "bad-secret"
	passphrases <- "correct-secret"
	calls := 0
	prompts := 0
	var events []string
	p := newOverviewPipeline(
		overviewPipelineConfig{
			params:              overviewParams{stack: "dev", cfg: &config.Config{}},
			passphrases:         passphrases,
			retryPassphrase:     true,
			transientPassphrase: true,
			openPlugins:         fakeOpenPlugins,
			newEngine:           fakeNewEngine,
			enrichRows:          passThroughEnrichment,
			loader: func(_ context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
				calls++
				if *p.Passphrase() == "bad-secret" {
					return nil, &PhaseError{
						Phase: 1,
						Err:   errors.New("decrypting secrets: incorrect passphrase bad-secret"),
					}
				}
				return &OverviewPipelineData{Rows: []engine.OverviewRow{}, StackName: "dev"}, nil
			},
		},
	)
	p.OnPassphraseRequired = func() { prompts++ }
	p.OnError = func(_ int, err error) { events = append(events, err.Error()) }
	require.NoError(t, p.Run(context.Background()))
	assert.Equal(t, 2, calls)
	assert.Equal(t, 2, prompts)
	require.Len(t, events, 1)
	assert.NotContains(t, events[0], "bad-secret")
	assert.Nil(t, p.Passphrase())
	if cleanup := p.Cleanup(); cleanup != nil {
		cleanup()
	}
}
func TestRunOverviewPipelineReleasesResources(t *testing.T) {
	t.Parallel()
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "cancelled"}[cancelled], func(t *testing.T) {
			t.Parallel()
			pluginClosed := false
			cacheClosed := false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p := newOverviewPipeline(
				overviewPipelineConfig{
					loader: fakePipelineLoader([]engine.OverviewRow{{URN: "urn::test", Type: "aws:ec2:Instance"}}),
					params: overviewParams{cfg: &config.Config{}},
					openPlugins: func(context.Context, string, *auditContext) ([]*pluginhost.Client, func(), error) {
						return nil, func() { pluginClosed = true }, nil
					},
					newEngine: func(context.Context, *cobra.Command, []*pluginhost.Client) (*engine.Engine, func()) {
						return engine.New(nil, nil), func() { cacheClosed = true }
					},
					enrichRows: func(_ context.Context, rows []engine.OverviewRow, _ *engine.Engine, _ engine.DateRange, progress chan<- engine.OverviewRowUpdate) []engine.OverviewRow {
						close(progress)
						if cancelled {
							cancel()
						}
						return rows
					},
				},
			)
			err := runOverviewPipeline(ctx, p)
			if cancelled {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
			}
			assert.True(t, pluginClosed)
			assert.True(t, cacheClosed)
		})
	}
}

// Uses isolated cwd, config, and fake Pulumi discovery.
func TestExplicitPlanWithoutProjectLoadsPlainAndWeb(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Chdir(t.TempDir())
	installFakePulumi(t)
	path := overviewFixture(t, "plan-mixed-changes.json")
	params := overviewParams{pulumiJSON: path, cfg: &config.Config{}}
	_, steps, _, err := resolveOverviewData(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, steps)
	p := newOverviewPipeline(
		overviewPipelineConfig{
			params:      params,
			loader:      autoDetectOverviewLoader,
			openPlugins: fakeOpenPlugins,
			newEngine:   fakeNewEngine,
			enrichRows:  passThroughEnrichment,
		},
	)
	require.NoError(t, runOverviewPipeline(context.Background(), p))
	assert.NotEmpty(t, p.Result().Rows)
}

// Changes cwd and Pulumi command runner.
func TestWebAutoDetectUsesStateWhenPreviewUnavailable(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, false)
	state, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: state}))
	p := newOverviewPipeline(
		overviewPipelineConfig{
			params:      overviewParams{stack: "dev", cfg: &config.Config{}},
			loader:      autoDetectOverviewLoader,
			openPlugins: fakeOpenPlugins,
			newEngine:   fakeNewEngine,
			enrichRows:  passThroughEnrichment,
		},
	)
	require.NoError(t, runOverviewPipeline(context.Background(), p))
	assert.NotEmpty(t, p.Result().Rows)
	assert.True(t, p.Result().IsStateOnly)
}

// Changes cwd, passphrase environment and Pulumi command runner.
func TestWebPreviewRequestsFreshScopedPassphrase(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, true)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{}))
	passphrases := make(chan string, 2)
	passphrases <- "load-secret"
	passphrases <- "preview-secret"
	prompts := 0
	p := newOverviewPipeline(
		overviewPipelineConfig{
			params: overviewParams{
				stack:      "dev",
				pulumiJSON: overviewFixture(t, "plan-mixed-changes.json"),
				cfg:        &config.Config{},
			},
			passphrases:         passphrases,
			transientPassphrase: true,
			openPlugins:         fakeOpenPlugins,
			newEngine:           fakeNewEngine,
			enrichRows:          passThroughEnrichment,
			loader: func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
				return &OverviewPipelineData{ProjectDir: project, StackName: "dev", Rows: fakePipelineRows()}, nil
			},
		},
	)
	p.OnPassphraseRequired = func() { prompts++ }
	p.OnDataReady = func([]engine.OverviewRow, int, string) {
		assert.Nil(t, p.Passphrase(), "unlock secret must be released before enrichment")
	}
	require.NoError(t, runOverviewPipeline(context.Background(), p))
	_, err := p.RunPreviewWithError(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, prompts)
	assert.Nil(t, p.Passphrase())
}

type previewPassphraseRunner struct {
	preview []byte
	calls   int
}

func (r *previewPassphraseRunner) Run(
	ctx context.Context,
	dir, name string,
	env []string,
	args ...string,
) ([]byte, []byte, error) {
	if strings.Contains(strings.Join(args, " "), "preview") {
		r.calls++
		if slices.Contains(env, "PULUMI_CONFIG_PASSPHRASE=bad-preview-secret") {
			return nil, []byte("incorrect passphrase bad-preview-secret"), errors.New("exit status 1")
		}
	}
	return (scriptedPulumi{preview: r.preview}).Run(ctx, dir, name, env, args...)
}

// Changes cwd, passphrase environment and Pulumi command runner.
func TestWebPreviewRetriesRejectedPassphrase(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, true)
	preview, err := os.ReadFile(overviewFixture(t, "plan-mixed-changes.json"))
	require.NoError(t, err)
	runner := &previewPassphraseRunner{preview: preview}
	t.Cleanup(pulumidetect.SetRunnerForTest(runner))
	passphrases := make(chan string, 3)
	passphrases <- "load-secret"
	passphrases <- "bad-preview-secret"
	passphrases <- "correct-preview-secret"
	p := newOverviewPipeline(
		overviewPipelineConfig{
			params:              overviewParams{stack: "dev", cfg: &config.Config{}},
			passphrases:         passphrases,
			transientPassphrase: true,
			retryPassphrase:     true,
			openPlugins:         fakeOpenPlugins,
			newEngine:           fakeNewEngine,
			enrichRows:          passThroughEnrichment,
			loader: func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
				return &OverviewPipelineData{ProjectDir: project, StackName: "dev", Rows: fakePipelineRows()}, nil
			},
		},
	)
	require.NoError(t, runOverviewPipeline(context.Background(), p))
	var events []string
	p.OnPassphraseRequired = func() { events = append(events, "passphrase_required") }
	p.OnError = func(phase int, err error) {
		assert.Equal(t, OverviewPhaseDetectChanges, phase)
		assert.NotContains(t, err.Error(), "bad-preview-secret")
		events = append(events, "error")
	}
	_, err = p.RunPreviewWithError(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"passphrase_required", "error", "passphrase_required"}, events)
	assert.Equal(t, 2, runner.calls)
	assert.Nil(t, p.Passphrase())
}
