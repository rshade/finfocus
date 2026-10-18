package cli

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// blockingBudgetAPI ignores cancellation until release is closed.
type blockingBudgetAPI struct {
	proto.CostSourceClient

	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
	once     sync.Once
}

func (b *blockingBudgetAPI) GetBudgets(
	context.Context,
	*pbc.GetBudgetsRequest,
	...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	close(b.finished)
	return &pbc.GetBudgetsResponse{}, nil
}

func TestBudgetFetchDoneClosesAfterGetBudgets(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	api := &blockingBudgetAPI{
		started:  make(chan struct{}),
		release:  release,
		finished: make(chan struct{}),
	}
	eng := engine.New([]*pluginhost.Client{{Name: "slow", API: api}}, nil)
	_, done := launchBudgetFetch(context.Background(), eng)

	select {
	case <-api.started:
	case <-time.After(time.Second):
		require.Fail(t, "GetBudgets did not start")
	}
	select {
	case <-done:
		require.Fail(t, "budget goroutine finished while GetBudgets was blocked")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "budget goroutine did not finish after GetBudgets returned")
	}
}

func TestOverviewPipelineCleansUpAfterBudgetFetch(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	api := &blockingBudgetAPI{
		started:  make(chan struct{}),
		release:  release,
		finished: make(chan struct{}),
	}
	eng := engine.New([]*pluginhost.Client{{Name: "slow", API: api}}, nil)
	cleaned := make(chan struct{})
	pipe := newOverviewPipeline(overviewPipelineConfig{
		wantBudget: true,
		budgetLive: true,
		loader: func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
			return &OverviewPipelineData{StackName: "dev"}, nil
		},
		openPlugins: func(context.Context, string, *auditContext) ([]*pluginhost.Client, func(), error) {
			return nil, func() { close(cleaned) }, nil
		},
		newEngine: func(context.Context, *cobra.Command, []*pluginhost.Client) (*engine.Engine, func()) {
			return eng, func() {}
		},
		enrichRows: func(
			_ context.Context,
			_ []engine.OverviewRow,
			_ *engine.Engine,
			_ engine.DateRange,
			progressChan chan<- engine.OverviewRowUpdate,
		) []engine.OverviewRow {
			close(progressChan)
			return nil
		},
		expandClusters: func(
			context.Context,
			[]engine.OverviewRow,
			[]*pluginhost.Client,
			engine.ResourcePricer,
			*config.Config,
		) ([]engine.OverviewRow, []string) {
			return nil, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- pipe.Run(ctx) }()

	select {
	case <-api.started:
	case <-time.After(time.Second):
		require.Fail(t, "GetBudgets did not start")
	}
	cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(release)
	}()
	select {
	case err := <-runDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		require.Fail(t, "pipeline did not return after GetBudgets")
	}
	select {
	case <-api.finished:
	default:
		require.Fail(t, "pipeline returned before GetBudgets finished")
	}
	select {
	case <-pipe.budgetDone:
	default:
		require.Fail(t, "pipeline returned before the budget goroutine finished")
	}
	cleanup := pipe.Cleanup()
	require.NotNil(t, cleanup)
	cleanup()
	select {
	case <-cleaned:
	default:
		require.Fail(t, "plugin cleanup was not invoked")
	}
}

func TestFinishInteractiveOverview_TakesCleanupSentAfterEnrichDone(t *testing.T) {
	t.Parallel()

	enrichDone := make(chan struct{})
	cleanupChan := make(chan func(), 1)
	close(enrichDone)
	called := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		cleanupChan <- func() { close(called) }
	}()
	finishInteractiveOverview(context.Background(), enrichDone, cleanupChan)
	select {
	case <-called:
	default:
		require.Fail(t, "cleanup sent after enrichDone was dropped")
	}
}

func TestFinishInteractiveOverview_StuckEnrichmentQuitsWithinGrace(t *testing.T) {
	t.Parallel()

	start := time.Now()
	finishInteractiveOverview(context.Background(), make(chan struct{}), make(chan func()))
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, overviewShutdownGrace)
	assert.Less(t, elapsed, overviewShutdownGrace+overviewCleanupHandoff+500*time.Millisecond)
}

func TestFinishInteractiveOverview_RunsCleanupOnceAfterLateEnrichment(t *testing.T) {
	t.Parallel()

	enrichDone := make(chan struct{})
	cleanupChan := make(chan func(), 1)
	var calls atomic.Int32
	called := make(chan struct{}, 2)

	start := time.Now()
	finishInteractiveOverview(context.Background(), enrichDone, cleanupChan)
	assert.Less(t, time.Since(start), overviewShutdownGrace+overviewCleanupHandoff+500*time.Millisecond)
	assert.Zero(t, calls.Load())

	cleanupChan <- func() {
		calls.Add(1)
		called <- struct{}{}
	}
	close(enrichDone)

	require.Eventually(t, func() bool { return calls.Load() == 1 }, 2*time.Second, 5*time.Millisecond)
	select {
	case <-called:
	case <-time.After(time.Second):
		require.Fail(t, "late cleanup did not run")
	}
	assert.Never(t, func() bool { return calls.Load() > 1 }, 100*time.Millisecond, 10*time.Millisecond)
}

func TestCleanupPublisher_PublishesOnceBeforeEnrichmentEnds(t *testing.T) {
	t.Parallel()

	pipe := &OverviewPipeline{}
	cleanupChan := make(chan func(), 1)
	publish := newCleanupPublisher(pipe, cleanupChan)

	publish()
	assert.Empty(t, cleanupChan, "nothing is published before plugins open")

	var calls atomic.Int32
	pipe.mu.Lock()
	pipe.cleanup = func() { calls.Add(1) }
	pipe.mu.Unlock()

	publish()
	publish()
	require.Len(t, cleanupChan, 1)

	enrichDone := make(chan struct{})
	finishInteractiveOverview(context.Background(), enrichDone, cleanupChan)
	assert.Equal(t, int32(1), calls.Load(), "cleanup must run even though enrichment never finished")
}
