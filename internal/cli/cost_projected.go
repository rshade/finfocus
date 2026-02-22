package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/spec"
)

// getBudgetScopeFilter returns the budget-scope flag value or empty string if not set.
func getBudgetScopeFilter(cmd *cobra.Command) string {
	if flag := cmd.Flag("budget-scope"); flag != nil {
		return flag.Value.String()
	}
	return ""
}

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
	jobs        int
}

// NewCostProjectedCmd returns a Cobra command configured to calculate projected costs
// from a Pulumi preview JSON and render the results in table, JSON, or NDJSON formats.
// The command accepts either an explicit Pulumi preview JSON file or will auto-detect
// and run a Pulumi preview in the current project when the --pulumi-json flag is omitted.
//
// The command registers the following flags:
//   - --pulumi-json: optional path to a Pulumi preview JSON output (auto-detected if omitted).
//   - --spec-dir: directory containing pricing specification files.
//   - --adapter: restricts execution to a single adapter plugin.
//   - --output: output format ("table", "json", or "ndjson").
//   - --filter: repeatable resource filter expressions (e.g., "type=aws:ec2/instance").
//   - --utilization: utilization rate for sustainability calculations (0.0 to 1.0).
//   - --jobs, -j: number of parallel workers (0 = auto based on CPU count).
//
// The returned command is ready to be added to the application's command tree.
func NewCostProjectedCmd() *cobra.Command {
	var params costProjectedParams

	cmd := &cobra.Command{
		Use:   "projected",
		Short: "Calculate projected costs from a Pulumi plan",
		Long: `Calculate projected costs by analyzing a Pulumi preview JSON output.

When --pulumi-json is omitted, finfocus automatically detects the Pulumi project
in the current directory and runs 'pulumi preview --json' to generate the input.
Use --stack to target a specific stack during auto-detection.`,
		Example: costProjectedExample,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeCostProjected(cmd, params)
		},
	}

	// --pulumi-json is intentionally optional (not MarkFlagRequired) to support
	// automatic Pulumi project detection via FindProject + preview.
	cmd.Flags().StringVar(&params.planPath, "pulumi-json", "",
		"Path to Pulumi preview JSON output (optional; auto-detected from Pulumi project if omitted)")
	cmd.Flags().StringVar(&params.specDir, "spec-dir", "", "Directory containing pricing spec files")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use only the specified adapter plugin")
	cmd.Flags().StringVar(
		&params.output, "output", config.GetDefaultOutputFormat(), "Output format: table, json, or ndjson")
	cmd.Flags().StringArrayVar(&params.filter, "filter", []string{},
		"Resource filter expressions (e.g., 'type=aws:ec2/instance')")
	cmd.Flags().Float64Var(
		&params.utilization, "utilization", 1.0, "Utilization rate for sustainability calculations (0.0 to 1.0)")
	cmd.Flags().IntVarP(&params.jobs, "jobs", "j", 0,
		"Number of parallel workers (0 = auto based on CPU count)")

	return cmd
}

const costProjectedExample = `  # Auto-detect from Pulumi project
  finfocus cost projected

  # Specific stack
  finfocus cost projected --stack production

  # Explicit file (existing behavior)
  finfocus cost projected --pulumi-json plan.json

  # Filter resources by type
  finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2/instance"

  # Output as JSON
  finfocus cost projected --pulumi-json plan.json --output json

  # Use a specific adapter plugin
  finfocus cost projected --pulumi-json plan.json --adapter aws-plugin

  # Use custom spec directory
  finfocus cost projected --pulumi-json plan.json --spec-dir ./custom-specs`

// executeCostProjected runs the projected cost calculation pipeline and renders output.
// It returns an error if any step (validation, loading, calculation, rendering) fails.
func executeCostProjected(cmd *cobra.Command, params costProjectedParams) error {
	ctx := cmd.Context()

	if params.jobs < 0 {
		return fmt.Errorf("--jobs must be non-negative, got %d", params.jobs)
	}

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

	var (
		resources []engine.ResourceDescriptor
		err       error
	)

	if params.planPath != "" {
		resources, err = loadAndMapResources(ctx, params.planPath, audit)
	} else {
		auditParams["pulumi_json"] = "auto-detect"
		stackFlag, flagErr := cmd.Flags().GetString("stack")
		if flagErr != nil {
			return fmt.Errorf("reading --stack flag: %w", flagErr)
		}
		resources, err = resolveResourcesFromPulumi(ctx, stackFlag, modePulumiPreview)
	}
	if err != nil {
		audit.logFailure(ctx, err)
		return err
	}

	resources, err = ApplyFilters(ctx, resources, params.filter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("invalid filter expression")
		audit.logFailure(ctx, err)
		return fmt.Errorf("applying filters: %w", err)
	}

	cfg, specDir := config.GetGlobalConfig(), params.specDir
	if specDir == "" {
		specDir = cfg.SpecDir
	}

	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	eng, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(specDir), cfg)
	defer cacheCleanup()
	eng = eng.WithJobs(params.jobs)
	start := time.Now()
	resultWithErrors, err := eng.GetProjectedCostWithErrors(ctx, resources)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to calculate projected costs")
		audit.logFailure(ctx, err)
		return fmt.Errorf("calculating projected costs: %w", err)
	}

	fetchAndMergeRecommendations(ctx, eng, resources, resultWithErrors.Results)

	if renderErr := RenderCostOutput(ctx, cmd, params.output, resultWithErrors); renderErr != nil {
		return renderErr
	}

	printTimingOutput(cmd, start, len(resources), params.output)

	log.Info().Ctx(ctx).Str("operation", "cost_projected").Int("result_count", len(resultWithErrors.Results)).
		Dur("duration_ms", time.Since(audit.start)).Msg("projected cost calculation complete")

	totalCost := sumMonthlyCosts(resultWithErrors.Results)
	if budgetErr := evaluateBudgetStatus(cmd, resultWithErrors.Results, totalCost); budgetErr != nil {
		audit.logFailure(ctx, budgetErr)
		return budgetErr
	}
	audit.logSuccess(ctx, len(resultWithErrors.Results), totalCost)
	return nil
}
