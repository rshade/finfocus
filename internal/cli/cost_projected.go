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
	"github.com/rshade/finfocus/internal/stackload"
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
	pulumiJSONPath      string
	terraformPlan       string
	pulumiState         string
	terraformState      string
	specDir             string
	adapter             string
	output              string
	filter              []string
	utilization         float64
	jobs                int
	showBreakdown       bool
	pricingSpecFallback bool
	explain             bool
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
//   - --explain: show GetPricingSpec billing mode, unit, rate, source, assumptions,
//     and tiers beside each resource. Display only; the calculated cost is unchanged.
//
// The returned command is ready to be added to the application's command tree.
func NewCostProjectedCmd() *cobra.Command {
	var params costProjectedParams

	cmd := &cobra.Command{
		Use:   "projected",
		Short: "Calculate projected costs from a Pulumi or Terraform plan",
		Long: `Calculate projected costs by analyzing a Pulumi preview JSON output.

When --pulumi-json is omitted, finfocus automatically detects the Pulumi project
in the current directory and runs 'pulumi preview --json' to generate the input.
Use --stack to target a specific stack during auto-detection.

In a Terraform directory, runs OpenTofu plan and show without init.
Use --terraform-plan to read saved show -json output without planning, or
--terraform-state to price an existing state file. Inputs are mutually exclusive.
--include-workspace includes the workspace in Terraform history for this run;
otherwise cost.terraform.include_workspace applies. Pulumi ignores this setting.`,
		Example: costProjectedExample,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeCostProjected(cmd, params)
		},
	}

	// --pulumi-json is intentionally optional (not MarkFlagRequired) to support
	// automatic Pulumi project detection via FindProject + preview.
	cmd.Flags().StringVar(&params.pulumiJSONPath, "pulumi-json", "",
		"Path to Pulumi preview JSON output (optional; auto-detected from Pulumi project if omitted)")
	cmd.Flags().
		StringVar(&params.pulumiState, "pulumi-state", "", "Path to Pulumi stack export JSON")
	addIncludeWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&params.terraformPlan, "terraform-plan", "",
		"Path to tofu show -json output; does not run tofu")
	cmd.Flags().StringVar(&params.terraformState, "terraform-state", "",
		"Path to a Terraform state file (terraform.tfstate); mutually exclusive with --pulumi-json")
	cmd.Flags().
		StringVar(&params.specDir, "spec-dir", "", "Directory containing pricing spec files")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use only the specified adapter plugin")
	cmd.Flags().StringVar(
		&params.output, "output", "", "Output format: table, json, or ndjson (default from output.default_format, else table)")
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
	cmd.Flags().BoolVar(&params.explain, "explain", false,
		"Show plugin GetPricingSpec details for each resource without changing the cost")
	addAccessibilityFlags(cmd)
	scopeAccessibilityFlagsToBudget(cmd)

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
  finfocus cost projected --pulumi-json plan.json --pricing-spec-fallback

  # Show how each price was derived
  finfocus cost projected --pulumi-json plan.json --explain`

// validateCostProjectedParams validates the projected cost command parameters
// before any expensive work begins (plan loading, plugin startup) so invalid
// input fails fast.
func validateCostProjectedParams(params costProjectedParams) error {
	if params.jobs < 0 {
		return fmt.Errorf("--jobs must be non-negative, got %d", params.jobs)
	}

	// A Pulumi preview and state file combine, as they do for overview.
	pulumiOnly := params.terraformPlan == "" && params.terraformState == ""
	if err := validateInputPaths(
		params.terraformPlan, params.pulumiJSONPath, params.pulumiState, params.terraformState,
	); err != nil && !pulumiOnly {
		return err
	}

	if params.utilization < 0.0 || params.utilization > 1.0 {
		return fmt.Errorf("utilization must be between 0.0 and 1.0, got %f", params.utilization)
	}

	if !isValidOutputFormat(engine.OutputFormat(config.GetOutputFormat(params.output))) {
		return fmt.Errorf(
			"unsupported output format: %s (supported: table, json, ndjson)",
			params.output,
		)
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

// pricingSpecFallbackForCmd resolves the fallback setting for a parsed command.
// It is the single place that reads whether --pricing-spec-fallback was set.
func pricingSpecFallbackForCmd(cmd *cobra.Command, flagValue bool, cfg *config.Config) bool {
	return pricingSpecFallbackEnabled(cmd.Flags().Changed("pricing-spec-fallback"), flagValue, cfg)
}

const emptyPlanWarning = "Warning: the plan has no resources to price, so the $0 total is not an estimate. " +
	"A default `pulumi preview --json` omits unchanged resources; make the plan with `--show-sames` " +
	"or run `finfocus overview` to see the whole stack."

// warnIfPlanEmpty reports whether the plan holds nothing to price (no resources,
// or only Pulumi bookkeeping types such as the root stack) and says so on stderr,
// so a $0 total is not read as an estimate. It looks at the loaded plan, before
// any --filter or pricing, so neither a filter nor a missing plugin triggers it.
func warnIfPlanEmpty(cmd *cobra.Command, resources []engine.ResourceDescriptor) bool {
	if !planHasNothingToPrice(resources) {
		return false
	}
	fmt.Fprintln(cmd.ErrOrStderr(), emptyPlanWarning)
	return true
}

func planHasNothingToPrice(resources []engine.ResourceDescriptor) bool {
	for _, r := range resources {
		if r.Origin == engine.OriginPlan && !engine.IsPulumiInternalType(r.Type) {
			return false
		}
	}
	return true
}

// applyProjectedFilters applies --filter, recording an invalid expression in the
// log and audit trail and returning it as a validation error.
func applyProjectedFilters(
	ctx context.Context,
	params costProjectedParams,
	audit *auditContext,
	resources []engine.ResourceDescriptor,
) ([]engine.ResourceDescriptor, error) {
	filtered, err := ApplyFilters(ctx, resources, params.filter)
	if err != nil {
		logging.FromContext(ctx).Error().Ctx(ctx).Err(err).Msg("invalid filter expression")
		audit.logFailure(ctx, err)
		return nil, toValidationError(ctx, fmt.Errorf("applying filters: %w", err))
	}
	return filtered, nil
}

const noChangesMessage = "No changes. Projected monthly cost is zero."

// projectedUnchanged reports a Terraform or OpenTofu plan with no changes
// through writeUnchanged, then records the empty projection.
func projectedUnchanged(
	ctx context.Context,
	cmd *cobra.Command,
	audit *auditContext,
	format string,
	renderEmpty func() error,
) error {
	if err := writeUnchanged(cmd, format, renderEmpty); err != nil {
		audit.logFailure(ctx, err)
		return err
	}
	recordTerraformProjection(ctx, cmd, nil)
	audit.logSuccess(ctx, 0, 0)
	return nil
}

// writeUnchanged prints the no-change sentence for human output. JSON and
// NDJSON instead write the command's own empty document through renderEmpty,
// so stdout stays parseable, and the sentence moves to stderr.
func writeUnchanged(cmd *cobra.Command, format string, renderEmpty func() error) error {
	if format != outputFormatJSON && format != outputFormatNDJSON {
		cmd.Println(noChangesMessage)
		return nil
	}
	if err := renderEmpty(); err != nil {
		return err
	}
	cmd.PrintErrln(noChangesMessage)
	return nil
}

// unchangedProjectedDiff is the diff of a plan with no changes. Nothing is
// priced, so JSON sets emptyPlan as it does for an empty Pulumi preview.
func unchangedProjectedDiff() *engine.DiffResult {
	return &engine.DiffResult{
		EmptyPlan: true,
		Summary:   engine.DiffSummary{Currency: defaultCurrency},
	}
}

func projectedLoadError(
	ctx context.Context,
	params costProjectedParams,
	audit *auditContext,
	err error,
) error {
	audit.logFailure(ctx, err)
	userInput := errors.Is(err, ErrNoProjectMarker) ||
		params.terraformState != "" || params.pulumiJSONPath != "" || params.terraformPlan != "" || params.pulumiState != ""
	if userInput {
		return toValidationError(ctx, err)
	}
	return err
}

// executeCostProjected runs the projected cost calculation pipeline and renders output.
// It returns an error if any step (validation, loading, calculation, rendering) fails.
func executeCostProjected(cmd *cobra.Command, params costProjectedParams) error {
	ctx := cmd.Context()
	params.output = resolveOutputFormatWithConfig(cmd, params.output)

	if err := validateCostProjectedParams(params); err != nil {
		return toValidationError(ctx, err)
	}
	ctx = context.WithValue(ctx, engine.ContextKeyUtilization, params.utilization)

	log := logging.FromContext(ctx)
	log.Debug().Ctx(ctx).Str("operation", "cost_projected").Str("plan_path", params.pulumiJSONPath).
		Msg("starting projected cost calculation")

	audit := newCostProjectedAudit(ctx, params)

	resources, unchanged, snapshot, err := loadProjectedInventory(ctx, cmd, params, audit)
	if err != nil {
		return projectedLoadError(ctx, params, audit, err)
	}
	if unchanged {
		return projectedUnchanged(ctx, cmd, audit, params.output, func() error {
			return renderProjectedDiff(cmd, params.output, unchangedProjectedDiff(), false)
		})
	}

	emptyPlan := warnForProjectedPlan(cmd, resources, snapshot)
	resources, err = applyProjectedFilters(ctx, params, audit, resources)
	if err != nil {
		return err
	}

	cfg, specDir := projectedPricingConfig(params.specDir)

	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	warnNoTypeResolvingPlugin(cmd, params.terraformState, clients)

	eng, cacheStore, cacheCleanup := newEngineWithCache(
		ctx,
		cmd,
		clients,
		spec.NewLoader(specDir),
		cfg,
	)
	defer cacheCleanup()
	if profileErr := applyUsageProfile(cmd, eng, cfg); profileErr != nil {
		return toValidationError(ctx, profileErr)
	}
	eng = eng.WithJobs(params.jobs).WithPricingSpecFallback(
		pricingSpecFallbackForCmd(cmd, params.pricingSpecFallback, cfg),
	).WithExplainPricing(params.explain)
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
	diff.EmptyPlan = emptyPlan
	setProjectedStackSummary(diff, snapshot)

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
	recordTerraformProjection(ctx, cmd, diff.AfterCosts())
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
	case params.pulumiJSONPath != "":
		auditParams[auditKeyPulumiJSON] = params.pulumiJSONPath
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
) ([]engine.ResourceDescriptor, bool, error) {
	returnProjected, unchanged, _, err := loadProjectedInventory(ctx, cmd, params, audit)
	return returnProjected, unchanged, err
}

func loadProjectedInventory(
	ctx context.Context,
	cmd *cobra.Command,
	params costProjectedParams,
	audit *auditContext,
) ([]engine.ResourceDescriptor, bool, *stackload.Snapshot, error) {
	switch {
	case params.terraformState != "":
		resources, err := loadAndMapTerraformResources(ctx, params.terraformState, audit)
		return resources, false, nil, err
	case params.terraformPlan != "":
		resources, unchanged, err := loadTerraformPlanFile(ctx, cmd, params.terraformPlan)
		return resources, unchanged, nil, err
	case params.pulumiState != "" || params.pulumiJSONPath != "":
		resources, snapshot, err := loadPulumiProjected(ctx, cmd, params)
		return resources, false, snapshot, err
	default:
		source, err := DetectProjectSource(".")
		if err != nil {
			return nil, false, nil, err
		}
		if source.Kind == ProjectKindPulumi {
			resources, snapshot, loadErr := loadPulumiProjected(ctx, cmd, params)
			return resources, false, snapshot, loadErr
		}
		resources, err := loadTerraformDirectory(ctx, cmd, source.Dir)
		return resources, terraformPlanUnchanged(resources), nil, err
	}
}

func warnForProjectedPlan(
	cmd *cobra.Command,
	resources []engine.ResourceDescriptor,
	snapshot *stackload.Snapshot,
) bool {
	empty := planHasNothingToPrice(resources)
	if empty && (snapshot == nil || !snapshot.StateStatus.Loaded) {
		warnIfPlanEmpty(cmd, resources)
	}
	return empty
}

func projectedPricingConfig(specDir string) (*config.Config, string) {
	cfg := config.GetGlobalConfig()
	if specDir == "" {
		specDir = cfg.SpecDir
	}
	return cfg, specDir
}
