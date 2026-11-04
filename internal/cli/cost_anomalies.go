package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// anomaliesDeps replaces the overview pipeline stages and the daily-series
// reader in tests. A nil field selects the production stage.
type anomaliesDeps struct {
	loader      OverviewPipelineLoader
	openPlugins func(ctx context.Context, adapter string, audit *auditContext) ([]*pluginhost.Client, func(), error)
	newEngine   func(ctx context.Context, cmd *cobra.Command, clients []*pluginhost.Client) (*engine.Engine, func())
	enrichRows  func(
		ctx context.Context,
		rows []engine.OverviewRow,
		eng *engine.Engine,
		dateRange engine.DateRange,
		progressChan chan<- engine.OverviewRowUpdate,
	) []engine.OverviewRow
	// dailySeries reads the stack's stored daily actuals for the window,
	// dropping the current partial day. nil selects the production reader
	// over the history directory, and a nil result keeps the drift rows.
	dailySeries func(
		ctx context.Context,
		stack, project string,
		window engine.DateRange,
		now time.Time,
	) ([]history.DailyPoint, error)
}

// NewCostAnomaliesCmd returns `cost anomalies`. It runs the overview pipeline
// and lists the rows whose run-rate drift crossed the overview warning
// threshold, replacing a resource's drift row with a robust-median row when
// the stack's stored daily actuals qualify. The selection is
// engine.SelectAnomalies, shared with the web dashboard.
func NewCostAnomaliesCmd() *cobra.Command {
	return newCostAnomaliesCmd(anomaliesDeps{})
}

func newCostAnomaliesCmd(deps anomaliesDeps) *cobra.Command {
	var params overviewParams

	cmd := &cobra.Command{
		Use:   "anomalies",
		Short: "List resources whose month-to-date run-rate diverged from the projection",
		Long: `List the overview drift warnings as rows, plus robust-median rows when the
stack's stored daily actuals mark the latest complete day as a spike.

Drift rows are resources whose extrapolated monthly run-rate differs from
their projected monthly cost by more than 10%. The thresholds and the skips
for the first two days, new resources, and deleted resources are the
overview's own; this command only selects the rows that carry drift data.

When the daily series stored by "cost history collect-daily" has at least 14
earlier days of one currency, a resource's latest complete day is compared
with the median of the days before it instead. A robust z-score of 3.5 or
more in either direction reports that day with method "robust-median" in
place of the resource's drift row: baseline is the median, observed is the
day's amount, and baselineWindow names the first and last day inside the
median. A missing day is a gap, never a zero, and never part of the count.
A flat series (median absolute deviation 0) hides a spike from the score, so
no statistical row replaces the drift row for it. When the median is 0 the
percentDrift field is left out and deviation still reports the gap.
Currencies are never pooled into one median.

The daily series follows the dates you ask for: --from and --to bound the
days it reads, and without them the whole stored history of the stack is
used, because the daily store is already a rolling history of that stack.
The default cost window (the current month, which the drift comparison
needs) never truncates it, so the method works as soon as the store holds
fifteen days of one currency.

Drift baseline is the projected monthly cost normalized to the calendar
month, and drift observed is the extrapolated monthly run-rate, not the raw
month-to-date spend. A row whose cost lookup failed is not an anomaly.

Without --pulumi-json the command reads the deployed state and does not run
pulumi preview. The daily series follows the stack the overview resolved, so
--stack is not needed to use the method; a stack that has never been collected
keeps every resource's drift row. JSON output includes considered, the number
of rows the overview produced, so an empty list differs from a failed load.
Table, JSON, and NDJSON are supported. The command prices through the overview
pipeline and is as slow as finfocus overview --output json.`,
		Example: `  finfocus cost anomalies
  finfocus cost anomalies --stack prod --output json
  finfocus cost anomalies --pulumi-state state.json --filter provider=aws
  finfocus cost anomalies --from 2026-10-01 --to 2026-10-15 --output ndjson`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCostAnomalies(cmd, params, deps)
		},
	}

	cmd.Flags().StringVar(&params.pulumiJSON, "pulumi-json", "", "path to Pulumi preview JSON")
	cmd.Flags().StringVar(&params.pulumiState, "pulumi-state", "", "path to Pulumi state JSON")
	cmd.Flags().StringVar(&params.fromStr, "from", "", "start date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&params.toStr, "to", "", "end date (YYYY-MM-DD or RFC3339, defaults to now)")
	cmd.Flags().StringVarP(&params.adapter, "adapter", "a", "", "restrict to a specific adapter plugin")
	cmd.Flags().StringSliceVarP(&params.filter, "filter", "f", nil,
		"resource filters (key=value; keys: type, status, provider)")
	cmd.Flags().StringVar(&params.output, "output", "", "output format (table, json, ndjson)")

	return cmd
}

// runCostAnomalies validates every input before any state is loaded, runs the
// overview pipeline, and renders the anomaly list.
func runCostAnomalies(cmd *cobra.Command, params overviewParams, deps anomaliesDeps) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	output := resolveOutputFormatWithConfig(cmd, params.output)
	switch output {
	case outputFormatTable, outputFormatJSON, outputFormatNDJSON:
	default:
		return toValidationError(ctx, fmt.Errorf(
			"unsupported output format: %s (supported: table, json, ndjson)", output))
	}
	params.output = output

	if err := validateInfrastructureInputs(cmd); err != nil {
		return toValidationError(ctx, err)
	}
	if _, err := validateAndApplyOverviewFilters(nil, params.filter); err != nil {
		return toValidationError(ctx, err)
	}
	dateRange, err := resolveOverviewDateRange(params.fromStr, params.toStr, time.Now())
	if err != nil {
		return toValidationError(ctx, fmt.Errorf("invalid date range: %w", err))
	}

	if params.cfg == nil {
		params.cfg = config.New()
	}
	if _, err = resolveUsageProfile(cmd, usageProfileSource(params.cfg)); err != nil {
		return toValidationError(ctx, err)
	}
	if stack := cmd.Flags().Lookup("stack"); stack != nil {
		params.stack = stack.Value.String()
	}
	params.stateOnly = params.pulumiJSON == ""

	audit := newAuditContext(ctx, "cost anomalies", map[string]string{
		auditKeyPulumiState: params.pulumiState,
		auditKeyPulumiJSON:  params.pulumiJSON,
		auditKeyOutput:      params.output,
	})

	loader := deps.loader
	if loader == nil {
		loader = plainOverviewLoader
	}
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd:         cmd,
		params:      params,
		dateRange:   dateRange,
		audit:       audit,
		loader:      loader,
		openPlugins: deps.openPlugins,
		newEngine:   deps.newEngine,
		enrichRows:  deps.enrichRows,
	})

	var (
		report    engine.AnomalyReport
		ready     bool
		seriesErr error
	)
	// The series loads once the pipeline has resolved the stack, so OnReady
	// applies both methods at once and the method also works without
	// --stack. A series read failure is a real failure: the stored daily
	// costs are half the evidence this command reports.
	pipe.OnReady = func(result OverviewPipelineResult) {
		report, seriesErr = anomalyReport(ctx, deps, params, dateRange, result, cmd)
		ready = seriesErr == nil
	}
	if runErr := runOverviewPipeline(ctx, pipe); runErr != nil {
		return runErr
	}
	if seriesErr != nil {
		audit.logFailure(ctx, seriesErr)
		return seriesErr
	}
	if !ready {
		return errors.New("overview finished without producing rows")
	}

	if renderErr := renderAnomalies(cmd, params.output, report); renderErr != nil {
		audit.logFailure(ctx, renderErr)
		return renderErr
	}
	audit.logSuccess(ctx, report.Considered, 0)
	return nil
}

// anomalyReport applies both methods to the overview rows: the drift rows of
// #1590 and the robust-median rows of the stack's stored daily actuals. The
// series belongs to the stack the overview priced, and the loaded rows name
// its project, so --stack is not needed to use the method.
func anomalyReport(
	ctx context.Context,
	deps anomaliesDeps,
	params overviewParams,
	dateRange engine.DateRange,
	result OverviewPipelineResult,
	cmd *cobra.Command,
) (engine.AnomalyReport, error) {
	series, err := loadAnomalySeries(
		ctx,
		deps,
		anomalySeriesStack(params.stack, result.StackName),
		anomalySeriesProject(result.Rows),
		dailySeriesWindow(params, dateRange),
		explicitPulumiInput(cmd),
	)
	if err != nil {
		return engine.AnomalyReport{}, err
	}
	return engine.SelectAnomalies(result.Rows, series), nil
}

// anomalySeriesStack names the stack whose stored daily actuals the command
// reads. The --stack flag wins; without one the stack the overview resolved
// does, so a bare `finfocus cost anomalies` scores the series of the stack it
// just priced. An empty name means no series, and the drift rows stand.
func anomalySeriesStack(flagStack, resolvedStack string) string {
	if stack := strings.TrimSpace(flagStack); stack != "" {
		return stack
	}
	return strings.TrimSpace(resolvedStack)
}

// anomalySeriesProject names the project whose daily actuals the command
// reads. The loaded rows' Pulumi URNs carry the project the pipeline priced,
// so a bare --stack (or no --stack at all) still finds the file the collector
// wrote as `<project>@<stack>.daily.db`. Non-Pulumi URNs are ignored, and a
// stack value that names its own project wins over this fallback.
func anomalySeriesProject(rows []engine.OverviewRow) string {
	for i := range rows {
		if project := history.ProjectFromURN(rows[i].URN); project != "" {
			return project
		}
	}
	return ""
}

// dailySeriesWindow returns the window the daily reader scores inside. The
// stored daily actuals are a rolling history of the stack, so an explicit
// --from or --to bounds them and the implicit default window (the current
// month, which the drift comparison needs) does not. A zero bound filters
// nothing on that side.
func dailySeriesWindow(params overviewParams, resolved engine.DateRange) engine.DateRange {
	window := engine.DateRange{}
	if params.fromStr != "" {
		window.Start = resolved.Start
	}
	if params.toStr != "" {
		window.End = resolved.End
	}
	return window
}

// loadAnomalySeries reads the stored daily actuals through the deps seam.
// The production reader looks up the history directory's daily file for the
// named stack, and any seam replaces it wholesale in tests.
func loadAnomalySeries(
	ctx context.Context,
	deps anomaliesDeps,
	stack, project string,
	window engine.DateRange,
	explicitPulumi bool,
) ([]history.DailyPoint, error) {
	loader := deps.dailySeries
	if loader == nil {
		loader = func(
			listCtx context.Context,
			listStack, listProject string,
			listWindow engine.DateRange,
			now time.Time,
		) ([]history.DailyPoint, error) {
			return loadDailyAnomalySeries(listCtx, "", listStack, listProject, listWindow, now, explicitPulumi)
		}
	}
	return loader(ctx, stack, project, window, time.Now())
}

func renderAnomalies(cmd *cobra.Command, output string, report engine.AnomalyReport) error {
	w := cmd.OutOrStdout()
	var err error
	switch output {
	case outputFormatJSON:
		err = engine.RenderAnomaliesAsJSON(w, report)
	case outputFormatNDJSON:
		err = engine.RenderAnomaliesAsNDJSON(w, report)
	default:
		err = engine.RenderAnomaliesAsTable(w, report)
	}
	if err != nil {
		return fmt.Errorf("rendering anomalies: %w", err)
	}
	return nil
}
