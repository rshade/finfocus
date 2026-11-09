package cli

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/webui"
)

// webStack is the runtime of one stack in the web dashboard: its query data,
// overview pipeline, session, passphrase channel, and ESC source. The process
// shares only the stores, log hub, audit context, and dismissals; the config
// editor and plugin catalog callbacks are stateless, but a plugin install or
// removal reloads only this stack's plugin set. Plugin sets, the query gate, and
// snapshots stay here because ESC-derived plugin environment differs per stack.
type webStack struct {
	data     *webData
	pipe     *OverviewPipeline
	session  *webui.Session
	pipeCtx  context.Context
	cancel   context.CancelFunc
	pipeDone chan struct{}

	started  atomic.Bool
	stopOnce sync.Once
}

// newWebStack builds one stack's runtime from its params. The passphrase
// channel is created here, so a prompt and its answer can only meet inside the
// same stack. stores are injected and never closed by the stack.
func newWebStack(
	ctx context.Context,
	cmd *cobra.Command,
	params overviewParams,
	dateRange engine.DateRange,
	audit *auditContext,
	stores *sharedStores,
) *webStack {
	pipeCtx, cancel := context.WithCancel(ctx)
	passphrases := make(chan string)
	data := newWebData(cmd, params, dateRange, audit).
		withMergedConfig(config.InitializedGlobalConfig()).
		withStores(stores)
	pipeCtx = data.bindStackESC(pipeCtx)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd, params: data.params, dateRange: dateRange, audit: audit, loader: autoDetectOverviewLoader,
		passphrases: passphrases, wantBudget: true, budgetLive: true, budgetFallback: true, dismissalRows: true,
		retryPassphrase: true, transientPassphrase: true, newEngine: data.newEngine, stores: stores,
	})
	pipeDone := make(chan struct{})
	options := webSessionOptions(pipeCtx, data, pipe, pipeDone, passphrases)
	wireConfigEditor(ctx, &options, cmd, audit)
	wirePlugins(ctx, &options, cmd, data)
	session := webui.NewSession(pipeCtx, options)
	bindWebPipeline(ctx, pipe, session, params, dateRange)
	data.publishGraph = session.SetDependencyGraph
	data.bindPipelineSource(pipeCtx, pipe)
	webPipelineHook(pipe)
	return &webStack{data: data, pipe: pipe, session: session, pipeCtx: pipeCtx, cancel: cancel, pipeDone: pipeDone}
}

// start runs the pipeline in the background; pipeDone closes when it returns.
func (s *webStack) start() {
	s.started.Store(true)
	go func() { defer close(s.pipeDone); _ = s.pipe.Run(s.pipeCtx) }()
}

// stop ends the session, then the pipeline, and only then releases plugins so
// enrichment gRPC calls are not torn down mid-flight (same race as the TUI
// cleanup, issue #716). It is safe to call on a stack that never started (a lazily
// loaded one) and more than once; only the first call does the work.
func (s *webStack) stop() {
	s.stopOnce.Do(func() {
		s.session.Close()
		s.cancel()
		if s.started.Load() {
			<-s.pipeDone
		}
		s.data.closeReloaded()
		if cleanup := s.pipe.Cleanup(); cleanup != nil {
			cleanup()
		}
	})
}
