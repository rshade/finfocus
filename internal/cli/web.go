package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/webui"
)

// webFlag is the root-local flag that serves the finfocus web UI on
// localhost. Like --mcp, it is defined with cmd.Flags() on the root and is
// never persistent, so no subcommand inherits it.
const webFlag = "web"

// webShutdownTimeout bounds the graceful server drain after interrupt.
const webShutdownTimeout = 5 * time.Second

// goosWindows is the [runtime.GOOS] value for Windows, shared by the browser
// opener and the test helpers that fake platform binaries.
const goosWindows = "windows"

// webCompanionFlags are the root-local flags that are only meaningful with
// --web; used without it they are rejected, because a root-local flag would
// otherwise be parsed silently.
//
//nolint:gochecknoglobals // Immutable flag policy table.
var webCompanionFlags = []string{
	"port", "no-browser", "pulumi-json", "pulumi-state",
	"stack", "from", "to", "adapter", "filter",
}

// webPipelineHook observes the pipeline a web session builds, just before it
// starts. It is the seam the web handlers (T021/T022) use to subscribe SSE
// and REST handlers to pipeline events, and tests use it to inspect the
// session's pipeline.
//
//nolint:gochecknoglobals // Test/wiring seam for the --web entry point.
var webPipelineHook = func(*OverviewPipeline) {}

// webFlagValues holds the parsed values of the --web root-local flags.
type webFlagValues struct {
	web         bool
	port        int
	noBrowser   bool
	pulumiJSON  string
	pulumiState string
	stack       string
	from        string
	to          string
	adapter     string
	filter      []string
}

// registerWebFlags defines --web and its companions as root-local flags on
// cmd, writing parsed values into flags. No short forms, to avoid colliding
// with ax-go's mounted flags.
func registerWebFlags(cmd *cobra.Command, flags *webFlagValues) {
	cmd.Flags().BoolVar(&flags.web, webFlag, false,
		"serve the finfocus web UI on localhost and open it in a browser (Pulumi projects only)")
	cmd.Flags().IntVar(&flags.port, "port", 0,
		"pin the web UI port (0 = kernel-assigned; only valid with --web)")
	cmd.Flags().BoolVar(&flags.noBrowser, "no-browser", false,
		"do not open a browser; print the web UI URL only (only valid with --web)")
	cmd.Flags().StringVar(&flags.pulumiJSON, "pulumi-json", "",
		"path to Pulumi preview JSON (only valid with --web)")
	cmd.Flags().StringVar(&flags.pulumiState, "pulumi-state", "",
		"path to Pulumi state JSON (only valid with --web)")
	cmd.Flags().StringVar(&flags.stack, "stack", "",
		"Pulumi stack name for auto-detection (only valid with --web)")
	cmd.Flags().StringVar(&flags.from, "from", "",
		"start date (YYYY-MM-DD or RFC3339) (only valid with --web)")
	cmd.Flags().StringVar(&flags.to, "to", "",
		"end date (YYYY-MM-DD or RFC3339, defaults to now) (only valid with --web)")
	cmd.Flags().StringVar(&flags.adapter, "adapter", "",
		"restrict to a specific adapter plugin (only valid with --web)")
	cmd.Flags().StringSliceVar(&flags.filter, "filter", nil,
		"resource filters (only valid with --web)")
}

// validate enforces the --web flag combination rules: --web and --mcp are
// mutually exclusive, and every companion flag requires --web.
func (f *webFlagValues) validate(cmd *cobra.Command) error {
	if f.web {
		if mcp, _ := cmd.Flags().GetBool(mcpFlag); mcp {
			return fmt.Errorf("--%s and --%s are mutually exclusive", webFlag, mcpFlag)
		}
		return nil
	}
	for _, name := range webCompanionFlags {
		if cmd.Flags().Changed(name) {
			return fmt.Errorf("--%s is only valid with --%s", name, webFlag)
		}
	}
	return nil
}

// runWebSession is the --web entry point: it validates the source inputs the
// same way `finfocus overview` does, starts the localhost web UI server,
// prints the session URL, opens the browser unless --no-browser, and loads
// overview data through the shared OverviewPipeline until interrupted.
func runWebSession(cmd *cobra.Command, flags *webFlagValues) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	log := logging.FromContext(ctx)
	audit := newAuditContext(ctx, "web", map[string]string{
		"pulumi_state":     flags.pulumiState,
		auditKeyPulumiJSON: flags.pulumiJSON,
	})

	params := overviewParams{
		pulumiJSON:  flags.pulumiJSON,
		pulumiState: flags.pulumiState,
		stack:       flags.stack,
		fromStr:     flags.from,
		toStr:       flags.to,
		adapter:     flags.adapter,
		filter:      flags.filter,
		cfg:         config.New(),
	}
	dateRange, err := resolveOverviewDateRange(flags.from, flags.to, time.Now())
	if err != nil {
		return fmt.Errorf("invalid date range: %w", err)
	}

	// With no source flags, fail the same way `finfocus overview` does —
	// before any server starts.
	if params.pulumiState == "" && params.pulumiJSON == "" {
		if _, _, detectErr := detectPulumiProject(ctx, params.stack); detectErr != nil {
			return fmt.Errorf("auto-detecting Pulumi project: %w", detectErr)
		}
	}

	pipeCtx, pipeCancel := context.WithCancel(ctx)
	defer pipeCancel()
	passphrases := make(chan string)
	data := newWebData(cmd, params, dateRange, audit)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd, params: data.overviewParams(), dateRange: dateRange, audit: audit, loader: autoDetectOverviewLoader,
		passphrases: passphrases, wantBudget: true, budgetLive: true, budgetFallback: true, dismissalRows: true,
		retryPassphrase: true, transientPassphrase: true, newEngine: data.newEngine,
	})
	pipeDone := make(chan struct{})
	session := webui.NewSession(pipeCtx, webSessionOptions(pipeCtx, data, pipe, pipeDone, passphrases))
	defer session.Close()
	bindWebPipeline(ctx, pipe, session, params, dateRange)
	webPipelineHook(pipe)
	srv, err := webui.New(webui.Options{Port: flags.port, Session: session})
	if err != nil {
		return err
	}
	if listenErr := srv.Listen(); listenErr != nil {
		audit.logFailure(ctx, listenErr)
		return listenErr
	}
	if _, printErr := fmt.Fprintln(cmd.OutOrStdout(), srv.URL()); printErr != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), webShutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return fmt.Errorf("printing web UI URL: %w", printErr)
	}
	go func() { defer close(pipeDone); _ = pipe.Run(pipeCtx) }()

	if !flags.noBrowser {
		if browserErr := openWebBrowser(ctx, srv.URL()); browserErr != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "cli").
				Str("operation", "web_browser_open").
				Err(browserErr).
				Msg("could not open a browser; use the printed URL")
		}
	}

	serveErr := serveWebUntilInterrupt(ctx, srv)
	session.Close()

	// Stop the pipeline before releasing plugins so enrichment gRPC calls are
	// not torn down mid-flight (same race as the TUI cleanup, issue #716).
	pipeCancel()
	<-pipeDone
	if cleanup := pipe.Cleanup(); cleanup != nil {
		cleanup()
	}

	if serveErr != nil {
		audit.logFailure(ctx, serveErr)
		return fmt.Errorf("web server: %w", serveErr)
	}
	audit.logSuccess(ctx, pipe.Result().RowsEnriched, 0)
	return nil
}

// serveWebUntilInterrupt serves until ctx is cancelled or an interrupt
// arrives, then shuts the server down gracefully.
func serveWebUntilInterrupt(ctx context.Context, srv *webui.Server) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), webShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}

// openWebBrowser opens url in the system default browser via the platform
// opener. It is best-effort: the caller logs and continues on failure.
func openWebBrowser(ctx context.Context, url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case goosWindows:
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	// Reap the child in the background; the opener returns immediately.
	go func() { _ = cmd.Wait() }()
	return nil
}

// sessionRowsForPreview clones the final pipeline rows before shared engine
// helpers apply the same preview changes the TUI applies.
func sessionRowsForPreview(pipe *OverviewPipeline) []engine.OverviewRow {
	rows := pipe.Result().Rows
	return append([]engine.OverviewRow(nil), rows...)
}

func previewWebRows(ctx context.Context, pipe *OverviewPipeline, done <-chan struct{}) ([]engine.OverviewRow, error) {
	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if pipe.Engine() == nil {
		return nil, errors.New("overview data unavailable")
	}
	msg, err := pipe.RunPreviewWithError(ctx)
	if err != nil {
		return nil, err
	}
	rows := sessionRowsForPreview(pipe)
	engine.ApplyChangesToRows(rows, msg.StatusByURN)
	engine.ApplyPropertyDiffsToRows(rows, msg.PropertyDiffsByURN)
	engine.ApplyProjectedPropertiesToRows(rows, msg.ProjectedPropsByURN)
	return rows, nil
}

func bindWebPipeline(
	ctx context.Context,
	pipe *OverviewPipeline,
	session *webui.Session,
	params overviewParams,
	dateRange engine.DateRange,
) {
	log := logging.FromContext(ctx)
	pipe.OnPhase = session.Phase
	pipe.OnDataReady = func(rows []engine.OverviewRow, _ int, stack string) {
		session.SetData(pipe.Engine(), rows, dateRange, stack)
	}
	pipe.OnRow = session.Row
	pipe.OnProgress = session.Progress
	pipe.OnBudget = session.Budget
	pipe.OnExpansion = session.Expansion
	pipe.OnReady = func(result OverviewPipelineResult) { session.Ready(result.Rows) }
	pipe.OnPassphraseRequired = func() { session.RequirePassphrase(params.stack) }
	pipe.OnError = func(phase int, runErr error) {
		// Pulumi errors can include subprocess stderr. Publish and log safe labels.
		message := "Overview loading failed"
		if rejectedPassphrase(runErr) {
			message = "Unable to decrypt stack. Try the passphrase again."
		}
		session.Error(phase, message)
		log.Warn().Ctx(ctx).Str("component", "cli").Str("operation", "web_pipeline").Int("phase", phase).Msg(message)
	}
}

func webSessionOptions(
	pipeCtx context.Context,
	data *webData,
	pipe *OverviewPipeline,
	pipeDone <-chan struct{},
	passphrases chan<- string,
) webui.SessionOptions {
	return webui.SessionOptions{
		EstimateResources: data.estimateResources,
		ActualCosts:       data.actualCosts, Recommendations: data.recommendations, Trends: data.tableTrends,
		Preview: func(previewCtx context.Context) ([]engine.OverviewRow, error) {
			return previewWebRows(previewCtx, pipe, pipeDone)
		},
		SubmitPassphrase: func(requestCtx context.Context, pw string) error {
			select {
			case passphrases <- pw:
				return nil
			case <-requestCtx.Done():
				return requestCtx.Err()
			case <-pipeCtx.Done():
				return pipeCtx.Err()
			}
		},
	}
}
