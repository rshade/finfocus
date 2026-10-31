package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// anomaliesDeps replaces the overview pipeline stages in tests. A nil field
// selects the production stage.
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
}

// NewCostAnomaliesCmd returns `cost anomalies`. It runs the overview pipeline
// and lists the rows whose run-rate drift crossed the overview warning
// threshold. The selection is engine.SelectAnomalies, shared with the web
// dashboard.
func NewCostAnomaliesCmd() *cobra.Command {
	return newCostAnomaliesCmd(anomaliesDeps{})
}

func newCostAnomaliesCmd(deps anomaliesDeps) *cobra.Command {
	var params overviewParams

	cmd := &cobra.Command{
		Use:   "anomalies",
		Short: "List resources whose month-to-date run-rate diverged from the projection",
		Long: `List the overview drift warnings as rows.

Each row is a resource whose extrapolated monthly run-rate differs from its
projected monthly cost by more than 10%. The thresholds and the skips for the
first two days, new resources, and deleted resources are the overview's own;
this command only selects the rows that carry drift data.

baseline is the projected monthly cost normalized to the calendar month.
observed is the extrapolated monthly run-rate, not the raw month-to-date spend.
A row whose cost lookup failed is not an anomaly.

Without --pulumi-json the command reads the deployed state and does not run
pulumi preview. JSON output includes considered, the number of rows the
overview produced, so an empty list differs from a failed load. Table, JSON,
and NDJSON are supported. The command prices through the overview pipeline and
is as slow as finfocus overview --output json.`,
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
		report engine.AnomalyReport
		ready  bool
	)
	pipe.OnReady = func(result OverviewPipelineResult) {
		report = engine.SelectAnomalies(result.Rows)
		ready = true
	}
	if runErr := runOverviewPipeline(ctx, pipe); runErr != nil {
		return runErr
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
