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
	planPath            string
	terraformState      string
	specDir             string
	adapter             string
	output              string
	filter              []string
	utilization         float64
	jobs                int
	showBreakdown       bool
	pricingSpecFallback bool
}

// NewCostProjectedCmd returns a Cobra command configured to calculate projected costs
// from a Pulumi preview JSON and render the results in table, JSON, or NDJSON formats.
// The command accepts either an explicit Pulumi preview JSON file or will auto-detect
// and run a Pulumi preview in the current project when the --pulumi-json flag is omitted.
//
// The command registers the following flags:
//   - --pulumi-json: optional path to a Pulumi preview JSON output (auto-detected if omitted).
//   - --terraform-state: optional path to a Terraform state file (mutually exclusive with --pulumi-json).
//   - --spec-dir: directory containing pricing specification files.
//   - --adapter: restricts execution to a single adapter plugin.
//   - --output: output format ("table", "json", or "ndjson").
//   - --filter: repeatable resource filter expressions (e.g., "type=aws:ec2/instance").
//   - --utilization: utilization rate for sustainability calculations (0.0 to 1.0).
//   - --jobs, -j: number of parallel workers (0 = auto based on CPU count).
//   - --show-breakdown: per-component sub-rows in table output. JSON and NDJSON ignore it.
//   - --pricing-spec-fallback: price from plugin GetPricingSpec before local YAML
//     when GetProjectedCost misses. Default off. Overrides cost.pricing_spec_fallback
//     when the flag is set.
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
	cmd.Flags().StringVar(&params.terraformState, "terraform-state", "",
		"Path to a Terraform state file (terraform.tfstate); mutually exclusive with --pulumi-json")
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
	cmd.Flags().BoolVar(&params.showBreakdown, "show-breakdown", false,
		"Show per-component cost sub-rows in table output")
	cmd.Flags().BoolVar(&params.pricingSpecFallback, "pricing-spec-fallback", false,
		"Price from plugin GetPricingSpec before local YAML when projected cost is missing")
	addAccessibilityFlags(cmd)

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
  finfocus cost projected --pulumi-json plan.json --spec-dir ./custom-specs

  # Price from plugin GetPricingSpec before local YAML
  finfocus cost projected --pulumi-json plan.json --pricing-spec-fallback`

// validateCostProjectedParams validates the projected cost command parameters
// before any expensive work begins (plan loading, plugin startup) so invalid
// input fails fast.
func validateCostProjectedParams(params costProjectedParams) error {
	if params.jobs < 0 {
		return fmt.Errorf("--jobs must be non-negative, got %d", params.jobs)
	}

	if params.terraformState != "" && params.planPath != "" {
		return errors.New("--terraform-state and --pulumi-json are mutually exclusive; use only one")
	}

	if params.utilization < 0.0 || params.utilization > 1.0 {
		return fmt.Errorf("utilization must be between 0.0 and 1.0, got %f", params.utilization)
	}

	if !isValidOutputFormat(engine.OutputFormat(config.GetOutputFormat(params.output))) {
		return fmt.Errorf("unsupported output format: %s (supported: table, json, ndjson)", params.output)
	}

	return nil
}

// pricingSpecFallbackEnabled reports whether projected cost should call
// GetPricingSpec before local YAML. An explicitly set CLI flag wins.
// Otherwise cost.pricing_spec_fallback is used. Both default to off.
func pricingSpecFallbackEnabled(changed, flagValue bool, cfg *config.Config) bool {
	if changed {
		return flagValue
	}
	if cfg == nil {
		return false
	}
	return cfg.Cost.PricingSpecFallback
}

// executeCostProjected runs the projected cost calculation pipeline and renders output.
// It returns an error if any step (validation, loading, calculation, rendering) fails.
func executeCostProjected(cmd *cobra.Command, params costProjectedParams) error {
	ctx := cmd.Context()
	params.output = resolveOutputFormat(cmd, "output", params.output)

	if err := validateCostProjectedParams(params); err != nil {
		return toValidationError(ctx, err)
	}
	ctx = context.WithValue(ctx, engine.ContextKeyUtilization, params.utilization)

	log := logging.FromContext(ctx)
	log.Debug().Ctx(ctx).Str("operation", "cost_projected").Str("plan_path", params.planPath).
		Msg("starting projected cost calculation")

	audit := newCostProjectedAudit(ctx, params)

	resources, err := loadProjectedResources(ctx, cmd, params, audit)
	if err != nil {
		audit.logFailure(ctx, err)
		if params.terraformState != "" || params.planPath != "" {
			return toValidationError(ctx, err)
		}
		return err
	}

	resources, err = ApplyFilters(ctx, resources, params.filter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("invalid filter expression")
		audit.logFailure(ctx, err)
		return toValidationError(ctx, fmt.Errorf("applying filters: %w", err))
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

	warnNoTypeResolvingPlugin(cmd, params.terraformState, clients)

	eng, cacheStore, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(specDir), cfg)
	defer cacheCleanup()
	eng = eng.WithJobs(params.jobs).WithPricingSpecFallback(pricingSpecFallbackEnabled(
		cmd.Flags().Changed("pricing-spec-fallback"),
		params.pricingSpecFallback,
		cfg,
	))
	// No-op for Pulumi-sourced resources; see resolveResourceTypes.
	resources = resolveResourceTypes(ctx, clients, cacheStore, resources)
	start := time.Now()
	diff, err := eng.GetProjectedCostDiff(ctx, resources)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to calculate projected costs")
		audit.logFailure(ctx, err)
		return fmt.Errorf("calculating projected costs: %w", err)
	}

	mergeProjectedDiffRecommendations(ctx, eng, resources, diff)

	if renderErr := renderProjectedDiff(cmd, params.output, diff, params.showBreakdown); renderErr != nil {
		return renderErr
	}

	printTimingOutput(cmd, start, len(resources), params.output)

	log.Info().Ctx(ctx).Str("operation", "cost_projected").Int("result_count", len(diff.Entries)).
		Dur("duration_ms", time.Since(audit.start)).Msg("projected cost calculation complete")

	totalCost := diff.Summary.TotalAfter
	if budgetErr := evaluateBudgetStatusForOutput(
		cmd, diff.AfterCosts(), totalCost, params.output, storedBudgetFlagOverrides(cmd),
	); budgetErr != nil {
		audit.logFailure(ctx, budgetErr)
		return toAxExitError(ctx, budgetErr)
	}
	audit.logSuccess(ctx, len(diff.Entries), totalCost)
	return nil
}

// newCostProjectedAudit creates the audit context for the projected-cost
// command, recording the input source selected by params: the Terraform state
// path, the Pulumi plan path, or "auto-detect" when neither flag is set.
func newCostProjectedAudit(ctx context.Context, params costProjectedParams) *auditContext {
	auditParams := map[string]string{auditKeyOutput: params.output}
	switch {
	case params.terraformState != "":
		auditParams["terraform_state"] = params.terraformState
	case params.planPath != "":
		auditParams[auditKeyPulumiJSON] = params.planPath
	default:
		auditParams[auditKeyPulumiJSON] = "auto-detect"
	}
	if len(params.filter) > 0 {
		auditParams["filter"] = strings.Join(params.filter, ",")
	}
	return newAuditContext(ctx, "cost projected", auditParams)
}

// loadProjectedResources loads resource descriptors for the projected-cost
// command from the source selected by params: a Terraform state file
// (--terraform-state), a Pulumi preview JSON (--pulumi-json), or an
// auto-detected Pulumi preview in the current project.
func loadProjectedResources(
	ctx context.Context,
	cmd *cobra.Command,
	params costProjectedParams,
	audit *auditContext,
) ([]engine.ResourceDescriptor, error) {
	switch {
	case params.terraformState != "":
		return loadAndMapTerraformResources(ctx, params.terraformState, audit)
	case params.planPath != "":
		return loadAndMapResources(ctx, params.planPath, audit)
	default:
		stackFlag, err := cmd.Flags().GetString("stack")
		if err != nil {
			return nil, fmt.Errorf("reading --stack flag: %w", err)
		}
		return resolveResourcesFromPulumi(ctx, stackFlag, modePulumiPreview)
	}
}
