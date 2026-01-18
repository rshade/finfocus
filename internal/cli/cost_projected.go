package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/spec"
	"github.com/spf13/cobra"
)

// displayErrorSummary prints an error summary to the command output.
// It only displays for table format since JSON/NDJSON formats include errors in their structure.
func displayErrorSummary(
	cmd *cobra.Command,
	resultWithErrors *engine.CostResultWithErrors,
	outputFormat engine.OutputFormat,
) {
	if resultWithErrors.HasErrors() && outputFormat == engine.OutputTable {
		cmd.Println() // Add blank line before error summary
		cmd.Println("ERRORS")
		cmd.Println("======")
		cmd.Print(resultWithErrors.ErrorSummary())
	}
}

// costProjectedParams holds the parameters for the projected cost command execution.
type costProjectedParams struct {
	planPath    string
	specDir     string
	adapter     string
	output      string
	filter      []string
	utilization float64
}

// NewCostProjectedCmd creates the "projected" subcommand that calculates estimated costs from a Pulumi preview JSON.
//
// The returned command registers these flags: --pulumi-json (required), --spec-dir, --adapter, --output, --filter (can be provided multiple times), and --utilization.
// When executed the command collects the flag values and calls executeCostProjected with the assembled parameters.
func NewCostProjectedCmd() *cobra.Command {
	var params costProjectedParams

	cmd := &cobra.Command{
		Use:     "projected",
		Short:   "Calculate projected costs from a Pulumi plan",
		Long:    "Calculate projected costs by analyzing a Pulumi preview JSON output",
		Example: costProjectedExample,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeCostProjected(cmd, params)
		},
	}

	cmd.Flags().StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON output (required)")
	cmd.Flags().StringVar(&params.specDir, "spec-dir", "", "Directory containing pricing spec files")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use only the specified adapter plugin")
	cmd.Flags().StringVar(
		&params.output, "output", config.GetDefaultOutputFormat(), "Output format: table, json, or ndjson")
	cmd.Flags().StringArrayVar(&params.filter, "filter", []string{},
		"Resource filter expressions (e.g., 'type=aws:ec2/instance')")
	cmd.Flags().Float64Var(
		&params.utilization, "utilization", 1.0, "Utilization rate for sustainability calculations (0.0 to 1.0)")
	_ = cmd.MarkFlagRequired("pulumi-json")

	return cmd
}

const costProjectedExample = `  # Basic usage
  finfocus cost projected --pulumi-json plan.json

  # Filter resources by type
  finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2/instance"

  # Output as JSON
  finfocus cost projected --pulumi-json plan.json --output json

  # Use a specific adapter plugin
  finfocus cost projected --pulumi-json plan.json --adapter aws-plugin

  # Use custom spec directory
  finfocus cost projected --pulumi-json plan.json --spec-dir ./custom-specs`

// executeCostProjected runs the projected cost workflow for a Pulumi plan.
// It validates and injects the utilization into the context, loads and maps resources
// from the provided Pulumi preview JSON, applies any resource filter expressions,
// opens adapter plugins, computes projected costs, renders results to the command output,
// and records audit information.
//
// Parameters:
//   - cmd: the Cobra command whose context and output stream are used.
//   - params: configuration for the operation (plan path, spec directory, adapter, output format,
//     filter expressions, and utilization).
//
// Returns an error if validation fails (e.g., utilization out of range or invalid filter),
// resource loading/mapping or plugin initialization fails, cost calculation fails, or
// result rendering fails.
func executeCostProjected(cmd *cobra.Command, params costProjectedParams) error {
	ctx := cmd.Context()

	if params.utilization < 0.0 || params.utilization > 1.0 {
		return fmt.Errorf("utilization must be between 0.0 and 1.0, got %f", params.utilization)
	}
	ctx = context.WithValue(ctx, engine.ContextKeyUtilization, params.utilization)

	log := logging.FromContext(ctx)
	log.Debug().Ctx(ctx).Str("operation", "cost_projected").Str("plan_path", params.planPath).
		Msg("starting projected cost calculation")

	auditParams := map[string]string{"pulumi_json": params.planPath, "output": params.output}
	if len(params.filter) > 0 {
		auditParams["filter"] = strings.Join(params.filter, ",")
	}
	audit := newAuditContext(ctx, "cost projected", auditParams)

	resources, err := loadAndMapResources(ctx, params.planPath, audit)
	if err != nil {
		return err
	}

	resources, err = ApplyFilters(ctx, resources, params.filter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("invalid filter expression")
		audit.logFailure(ctx, err)
		return fmt.Errorf("applying filters: %w", err)
	}

	specDir := params.specDir
	if specDir == "" {
		specDir = config.New().SpecDir
	}

	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	resultWithErrors, err := engine.New(clients, spec.NewLoader(specDir)).GetProjectedCostWithErrors(ctx, resources)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to calculate projected costs")
		audit.logFailure(ctx, err)
		return fmt.Errorf("calculating projected costs: %w", err)
	}

	if renderErr := RenderCostOutput(ctx, cmd, params.output, resultWithErrors); renderErr != nil {
		return renderErr
	}

	log.Info().Ctx(ctx).Str("operation", "cost_projected").Int("result_count", len(resultWithErrors.Results)).
		Dur("duration_ms", time.Since(audit.start)).Msg("projected cost calculation complete")

	totalCost := 0.0
	for _, r := range resultWithErrors.Results {
		totalCost += r.Monthly
	}
	audit.logSuccess(ctx, len(resultWithErrors.Results), totalCost)
	return nil
}
