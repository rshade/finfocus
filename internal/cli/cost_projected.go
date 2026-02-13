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
}

// NewCostProjectedCmd creates the "projected" subcommand for calculating projected costs.
//
// The command analyzes a Pulumi preview to produce projected monthly costs and supports
// auto-detection of a Pulumi project: when --pulumi-json is omitted, the current Pulumi
// project is used and a preview is executed to obtain JSON input (use --stack to target
// a specific stack during auto-detection).
//
// Registered flags:
//   - --pulumi-json: optional path to a Pulumi preview JSON file (auto-detected if omitted)
//   - --spec-dir: directory containing pricing specification files
//   - --adapter: restrict processing to a single adapter plugin
//   - --output: output format, one of table, json, or ndjson (default from configuration)
//   - --filter: repeatable resource filter expression(s)
//   - --utilization: utilization rate for sustainability calculations (0.0 to 1.0)
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

// executeCostProjected runs the projected cost calculation for the "projected" command.
// It validates the utilization value, obtains resource descriptors either from an explicit
// Pulumi JSON plan or by running a Pulumi preview for the current/selected stack, applies
// resource filters, loads pricing specs and adapter plugins, computes projected costs
// (including any per-resource errors), renders the chosen output format, and evaluates
// budget status when results use a single currency.
//
// Parameters:
//   - cmd: the Cobra command whose context and output stream are used.
//   - params: configuration for the operation (plan path, spec directory, adapter, output format,
//     filter expressions, and utilization).
//
// Returns an error when validation fails, resources cannot be loaded or filtered, plugins cannot
// be opened, cost computation fails, rendering fails, or when a budget-related exit condition is
// triggered.
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

	var resources []engine.ResourceDescriptor
	var err error

	if params.planPath != "" {
		resources, err = loadAndMapResources(ctx, params.planPath, audit)
	} else {
		auditParams["pulumi_json"] = "auto-detect"
		stackFlag, _ := cmd.Flags().GetString("stack")
		resources, err = resolveResourcesFromPulumi(ctx, stackFlag, modePulumiPreview)
	}
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

	eng := engine.New(clients, spec.NewLoader(specDir))
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

	log.Info().Ctx(ctx).Str("operation", "cost_projected").Int("result_count", len(resultWithErrors.Results)).
		Dur("duration_ms", time.Since(audit.start)).Msg("projected cost calculation complete")

	totalCost := 0.0
	for _, r := range resultWithErrors.Results {
		totalCost += r.Monthly
	}

	currency, mixedCurrencies := extractCurrencyFromResults(resultWithErrors.Results)
	audit.logSuccess(ctx, len(resultWithErrors.Results), totalCost)

	// Evaluate and render budget status (T025: Call checkBudgetExit after renderBudgetIfConfigured)
	// Render budget status only when currencies are consistent
	if !mixedCurrencies {
		scopeFilter := getBudgetScopeFilter(cmd)

		budgetResult, budgetErr := renderBudgetWithScope(
			cmd, resultWithErrors.Results, totalCost, currency, scopeFilter)
		if exitErr := checkBudgetExitFromResult(cmd, budgetResult, budgetErr); exitErr != nil {
			return exitErr
		}
	}

	return nil
}
