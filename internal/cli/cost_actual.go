package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
)

const (
	filterKeyValueParts = 2   // For "key=value" pairs
	maxDateRangeDays    = 366 // Maximum date range (1 year + 1 day for leap years)
	maxPastYears        = 5   // Maximum years in the past allowed
	hoursPerDay         = 24  // Hours in a day for date calculations
)

// costActualParams holds the parameters for the actual cost command execution.
type costActualParams struct {
	planPath           string // Path to Pulumi preview JSON (mutually exclusive with statePath)
	statePath          string // Path to Pulumi state JSON (mutually exclusive with planPath)
	estimateConfidence bool   // Show confidence level for cost estimates
	fallbackEstimate   bool   // Include $0 placeholders for resources with no plugin data
	adapter            string
	output             string
	fromStr            string
	toStr              string
	groupBy            string
	filter             []string
}

// defaultToNow returns s if non-empty, otherwise returns the current time in RFC3339 format.
func defaultToNow(s string) string {
	if s == "" {
		return time.Now().Format(time.RFC3339)
	}
	return s
}

// NewCostActualCmd creates the "actual" subcommand for fetching historical cloud costs
// or estimating costs from Pulumi state. The command accepts a Pulumi preview JSON
// (--pulumi-json), a Pulumi state export (--pulumi-state), or auto-detects the Pulumi
// project in the current directory when both are omitted. When using state or
// auto-detection, the start date (--from) may be auto-detected from the earliest
// resource Created timestamp; when using a preview JSON, --from must be supplied.
// The command exposes flags for output format, grouping, filtering, adapter selection,
// showing estimate confidence, and including fallback $0 placeholders for resources
// with no plugin cost data (--fallback-estimate).
func NewCostActualCmd() *cobra.Command {
	var params costActualParams

	cmd := &cobra.Command{
		Use:   "actual",
		Short: "Fetch actual historical costs",
		Long: `Fetch actual historical costs for resources from cloud provider billing APIs,
or estimate costs from Pulumi state file timestamps.

When --pulumi-json and --pulumi-state are both omitted, finfocus automatically
detects the Pulumi project in the current directory and runs 'pulumi stack export'
to generate the input. The --from date is auto-detected from the earliest Created
timestamp, and --stack can be used to target a specific stack.

When using --pulumi-state, costs are estimated based on resource runtime calculated
from the Created timestamp. The --from date is auto-detected from the earliest
timestamp if not provided.`,
		Example: `  # Auto-detect from Pulumi project (dates auto-detected from state)
  finfocus cost actual

  # Auto-detect with specific stack
  finfocus cost actual --stack production

  # Get costs for the last 7 days (to defaults to now)
  finfocus cost actual --pulumi-json plan.json --from 2025-01-07

  # Get costs for a specific date range
  finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --to 2025-01-31

  # Estimate costs from Pulumi state (--from auto-detected from timestamps)
  finfocus cost actual --pulumi-state state.json

  # Estimate costs from state with explicit date range
  finfocus cost actual --pulumi-state state.json --from 2025-01-01 --to 2025-01-31

  # Group costs by resource type
  finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --group-by type

  # Daily cross-provider aggregation table
  finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --to 2025-01-07 --group-by daily

  # Monthly cross-provider aggregation table
  finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --to 2025-03-31 --group-by monthly

  # Output as JSON with grouping by provider
  finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --output json --group-by provider

  # Show confidence levels for cost estimates (useful for imported resources)
  finfocus cost actual --pulumi-state state.json --estimate-confidence`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeCostActual(cmd, params)
		},
	}

	cmd.Flags().
		StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON output")
	cmd.Flags().
		StringVar(&params.statePath, "pulumi-state", "", "Path to Pulumi state JSON from 'pulumi stack export'")
	cmd.Flags().StringVar(
		&params.fromStr, "from", "", "Start date (YYYY-MM-DD or RFC3339, auto-detected with --pulumi-state)",
	)
	cmd.Flags().StringVar(&params.toStr, "to", "", "End date (YYYY-MM-DD or RFC3339) (defaults to now)")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use only the specified adapter plugin")

	// Use configuration default if no output format specified
	defaultFormat := config.GetDefaultOutputFormat()
	cmd.Flags().StringVar(&params.output, "output", defaultFormat, "Output format: table, json, or ndjson")
	cmd.Flags().
		StringVar(&params.groupBy, "group-by", "", "Group results by: resource, type, provider, date, daily, monthly, or filter by tag:key=value")
	cmd.Flags().BoolVar(
		&params.estimateConfidence,
		"estimate-confidence",
		false,
		"Show confidence level for cost estimates",
	)
	cmd.Flags().BoolVar(
		&params.fallbackEstimate,
		"fallback-estimate",
		false,
		"Include $0 placeholder results for resources with no plugin cost data",
	)
	cmd.Flags().StringArrayVar(&params.filter, "filter", []string{},
		"Resource filter expressions (e.g., 'type=aws:ec2/instance', 'tag:env=prod')")

	// Note: --pulumi-json and --from are no longer required - validation is done in executeCostActual

	return cmd
}

// executeCostActual orchestrates the "actual" cost workflow: it validates CLI flags,
// loads and filters resources, resolves the date range, opens adapter plugins, requests
// actual cost data from the engine, renders the output, evaluates budget status (when
// applicable), and records audit events.
//
// Parameters:
//   - cmd: the Cobra command providing context and CLI flag state.
//   - params: the parsed costActualParams carrying paths, date strings, grouping, filters,
//     adapter and output options, and flags such as estimate confidence and fallback-estimate.
//
// Returns an error if validation fails, resources cannot be loaded or filtered, the date range
// cannot be parsed or resolved, plugins cannot be opened, the engine fails to fetch costs, the
// output rendering fails, or any other non-recoverable step in the workflow encounters an error.
func executeCostActual(cmd *cobra.Command, params costActualParams) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	// Validate mutually exclusive flags
	if err := validateActualInputFlags(params); err != nil {
		return err
	}

	log.Debug().Ctx(ctx).Str("operation", "cost_actual").
		Str("plan_path", params.planPath).Str("state_path", params.statePath).
		Str("from", params.fromStr).Str("to", params.toStr).Str("group_by", params.groupBy).
		Msg("starting actual cost calculation")

	audit := newAuditContext(ctx, "cost actual", buildActualAuditParams(params))

	resources, err := loadActualResources(ctx, cmd, params, audit)
	if err != nil {
		return err
	}

	resources, err = ApplyFilters(ctx, resources, params.filter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("invalid filter expression")
		audit.logFailure(ctx, err)
		return fmt.Errorf("applying filters: %w", err)
	}

	fromStr, err := resolveFromDate(ctx, params, resources)
	if err != nil {
		return err
	}

	from, to, err := ParseTimeRange(fromStr, defaultToNow(params.toStr))
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to parse time range")
		audit.logFailure(ctx, err)
		return fmt.Errorf("parsing time range: %w", err)
	}

	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	tags, actualGroupBy := parseTagFilter(params.groupBy)
	request := engine.ActualCostRequest{
		Resources: resources, From: from, To: to,
		Adapter: params.adapter, GroupBy: actualGroupBy, Tags: tags,
		EstimateConfidence: params.estimateConfidence,
		FallbackEstimate:   params.fallbackEstimate,
	}

	eng := engine.New(clients, nil)
	resultWithErrors, err := eng.GetActualCostWithOptionsAndErrors(ctx, request)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to fetch actual costs")
		audit.logFailure(ctx, err)
		return fmt.Errorf("fetching actual costs: %w", err)
	}

	fetchAndMergeRecommendations(ctx, eng, resources, resultWithErrors.Results)

	if renderErr := RenderActualCostOutput(
		ctx, cmd, params.output, resultWithErrors, actualGroupBy, params.estimateConfidence,
	); renderErr != nil {
		return renderErr
	}

	log.Info().Ctx(ctx).Str("operation", "cost_actual").Int("result_count", len(resultWithErrors.Results)).
		Dur("duration_ms", time.Since(audit.start)).Msg("actual cost calculation complete")

	totalCost := 0.0
	for _, r := range resultWithErrors.Results {
		totalCost += r.TotalCost
	}

	currency, mixedCurrencies := extractCurrencyFromResults(resultWithErrors.Results)

	// Render budget status only for table format and when currencies are consistent
	// T026: Call checkBudgetExit after renderBudgetIfConfigured
	if !mixedCurrencies && params.output == outputFormatTable {
		scopeFilter := getBudgetScopeFilter(cmd)

		budgetResult, budgetErr := renderBudgetWithScope(
			cmd, resultWithErrors.Results, totalCost, currency, scopeFilter)
		if exitErr := checkBudgetExitFromResult(cmd, budgetResult, budgetErr); exitErr != nil {
			log.Warn().Ctx(ctx).Err(exitErr).Msg("budget exit check returned error (non-fatal)")
		}
	}

	audit.logSuccess(ctx, len(resultWithErrors.Results), totalCost)
	return nil
}

// ParseTimeRange parses the provided from and to date strings into time values and validates that the range is chronological.
//
// ParseTimeRange accepts two date strings, parses each into a time.Time, and ensures the 'to' time is after the 'from' time.
// It returns the parsed from and to times on success. If either date cannot be parsed or if the 'to' time is not after
// the 'from' time, an error is returned describing the failure.
// Additionally validates that the date range does not exceed maximum limits.
func ParseTimeRange(fromStr, toStr string) (time.Time, time.Time, error) {
	from, err := ParseTime(fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing 'from' date: %w", err)
	}

	to, err := ParseTime(toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing 'to' date: %w", err)
	}

	if !to.After(from) {
		return time.Time{}, time.Time{}, errors.New("'to' date must be after 'from' date")
	}

	// Validate date range is within acceptable limits
	if rangeErr := ValidateDateRange(from, to); rangeErr != nil {
		return time.Time{}, time.Time{}, rangeErr
	}

	return from, to, nil
}

// ParseTime parses str as a date in either "YYYY-MM-DD" or RFC3339 format.
// It validates that the parsed time is not in the future and is not more than maxPastYears years in the past.
func ParseTime(str string) (time.Time, error) {
	layouts := []string{
		"2006-01-02",
		time.RFC3339,
	}

	var parsedTime time.Time
	var parseErr error
	parsed := false

	for _, layout := range layouts {
		t, err := time.Parse(layout, str)
		if err == nil {
			parsedTime = t
			parsed = true
			break
		}
		parseErr = err
	}

	if !parsed {
		return time.Time{}, fmt.Errorf(
			"unable to parse date: %s (use YYYY-MM-DD or RFC3339): %w",
			str,
			parseErr,
		)
	}

	// Validate: date cannot be in the future
	now := time.Now()
	if parsedTime.After(now) {
		return time.Time{}, fmt.Errorf("date cannot be in the future: %s", str)
	}

	// Validate: date cannot be more than maxPastYears years in the past
	oldestAllowed := now.AddDate(-maxPastYears, 0, 0)
	if parsedTime.Before(oldestAllowed) {
		return time.Time{}, fmt.Errorf(
			"date too far in past: %s (max %d years ago)",
			str,
			maxPastYears,
		)
	}

	return parsedTime, nil
}

// ValidateDateRange validates that the date range is within acceptable limits.
// Returns an error if the range exceeds maxDateRangeDays (approximately 1 year).
func ValidateDateRange(from, to time.Time) error {
	days := int(to.Sub(from).Hours() / hoursPerDay)
	if days > maxDateRangeDays {
		return fmt.Errorf("date range too large: %d days (max %d days / ~1 year). "+
			"Tip: Use --group-by monthly to analyze longer periods efficiently", days, maxDateRangeDays)
	}
	return nil
}

// parseTagFilter parses a group-by specifier for a tag filter and returns the parsed tags and the resulting groupBy.
// If groupBy is of the form "tag:key=value", it returns a map containing {key: value} and an empty actualGroupBy (indicating tag-based filtering).
// string (empty when filtering by tag).
func parseTagFilter(groupBy string) (map[string]string, string) {
	tags := make(map[string]string)
	actualGroupBy := groupBy

	if strings.HasPrefix(groupBy, "tag:") && strings.Contains(groupBy, "=") {
		tagPart := strings.TrimPrefix(groupBy, "tag:")
		if parts := strings.Split(tagPart, "="); len(parts) == filterKeyValueParts {
			tags[parts[0]] = parts[1]
			actualGroupBy = "" // Clear groupBy since we're filtering by tag
		}
	}

	return tags, actualGroupBy
}

// renderActualCostOutput renders actual cost results to writer using the specified outputFormat.
// If actualGroupBy indicates a time-based grouping, it first creates a cross-provider aggregation
// and renders that aggregation; otherwise it renders the raw results. The estimateConfidence flag
// controls whether confidence values are included in non-aggregated output.
//
// Parameters:
//   - writer: destination for rendered output.
//   - outputFormat: format to render results in (table, json, ndjson, etc.).
//   - results: slice of cost results to render or aggregate.
//   - actualGroupBy: grouping spec; time-based values trigger cross-provider aggregation.
//   - estimateConfidence: include confidence levels in the rendered output when applicable.
//
// Returns an error if aggregation or rendering fails.
func renderActualCostOutput(
	writer io.Writer,
	outputFormat engine.OutputFormat,
	results []engine.CostResult,
	actualGroupBy string,
	estimateConfidence bool,
) error {
	// Check if we need cross-provider aggregation
	groupByType := engine.GroupBy(actualGroupBy)
	if groupByType.IsTimeBasedGrouping() {
		aggregations, err := engine.CreateCrossProviderAggregation(results, groupByType)
		if err != nil {
			return fmt.Errorf("creating cross-provider aggregation: %w", err)
		}
		return engine.RenderCrossProviderAggregation(
			writer,
			outputFormat,
			aggregations,
			groupByType,
		)
	}

	return engine.RenderActualCostResults(writer, outputFormat, results, estimateConfidence)
}

// validateActualInputFlags validates the combinations of CLI input flags used by the
// "actual" cost command, ensuring mutual exclusivity and required options.
//
// Returns an error if both --pulumi-json and --pulumi-state are provided at the same
// time, or if --pulumi-json is supplied without an explicit --from date. When neither
// is provided, auto-detection is permitted and --from is optional.
func validateActualInputFlags(params costActualParams) error {
	hasPlan := params.planPath != ""
	hasState := params.statePath != ""

	// Check mutual exclusivity
	if hasPlan && hasState {
		return errors.New("--pulumi-json and --pulumi-state are mutually exclusive; use only one")
	}

	// Neither provided is valid: auto-detection will be attempted
	// When using --pulumi-json, --from is required
	if hasPlan && params.fromStr == "" {
		return errors.New("--from is required when using --pulumi-json")
	}

	// When using --pulumi-state or auto-detection, --from is optional (auto-detected from timestamps)

	return nil
}

// loadResourcesFromState loads resources from a Pulumi state file (from `pulumi stack export`).
// It parses the state JSON and maps custom resources to ResourceDescriptors.
func loadResourcesFromState(
	ctx context.Context,
	statePath string,
	audit *auditContext,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	log.Debug().Ctx(ctx).Str("component", "cli").Str("state_path", statePath).
		Msg("loading resources from Pulumi state")

	state, err := ingest.LoadStackExportWithContext(ctx, statePath)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Str("state_path", statePath).
			Msg("failed to load state file")
		audit.logFailure(ctx, err)
		return nil, fmt.Errorf("loading Pulumi state: %w", err)
	}

	customResources := state.GetCustomResourcesWithContext(ctx)
	if len(customResources) == 0 {
		log.Warn().Ctx(ctx).Msg("no custom resources found in state")
		return []engine.ResourceDescriptor{}, nil
	}

	resources, mapErr := ingest.MapStateResources(customResources)
	if mapErr != nil {
		log.Error().Ctx(ctx).Err(mapErr).Msg("failed to map state resources")
		audit.logFailure(ctx, mapErr)
		return nil, fmt.Errorf("mapping state resources: %w", mapErr)
	}

	log.Debug().Ctx(ctx).Int("resource_count", len(resources)).
		Msg("loaded resources from state")

	return resources, nil
}

// buildActualAuditParams constructs a map of audit parameters from the provided costActualParams.
// The returned map always contains the keys "from", "to", "adapter", "output", "group_by",
// "estimate_confidence", and "fallback_estimate". If present in the params, "plan_path" and
// "state_path" are added to the map. The values are stringified suitable for audit logging.
func buildActualAuditParams(params costActualParams) map[string]string {
	auditParams := map[string]string{
		"from":                params.fromStr,
		"to":                  params.toStr,
		"adapter":             params.adapter,
		"output":              params.output,
		"group_by":            params.groupBy,
		"estimate_confidence": strconv.FormatBool(params.estimateConfidence),
		"fallback_estimate":   strconv.FormatBool(params.fallbackEstimate),
	}
	if params.planPath != "" {
		auditParams["plan_path"] = params.planPath
	}
	if params.statePath != "" {
		auditParams["state_path"] = params.statePath
	}
	return auditParams
}

// loadActualResources loads resource descriptors from one of three sources:
// a Pulumi state file, a Pulumi preview plan, or by auto-detecting from the
// current Pulumi project when neither path is provided.
//
// The selection is driven by params: if params.statePath is set the state file
// is used; if params.planPath is set the plan is used; otherwise the function
// attempts auto-detection and uses the `stack` flag from cmd for project
// resolution. The audit context is used to record failures encountered while
// loading or mapping resources.
//
// cmd is consulted only for CLI flags necessary during auto-detection.
// params provides the plan/state paths and other flags relevant to loading.
// audit is used to log and record load or mapping failures.
//
// The function returns a slice of engine.ResourceDescriptor on success.
// It returns a non-nil error if loading the state or plan fails, if mapping
// resources fails, or if auto-detection cannot resolve resources.
func loadActualResources(
	ctx context.Context,
	cmd *cobra.Command,
	params costActualParams,
	audit *auditContext,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	if params.statePath != "" {
		return loadResourcesFromState(ctx, params.statePath, audit)
	}

	if params.planPath != "" {
		// Load from Pulumi plan
		plan, err := ingest.LoadPulumiPlanWithContext(ctx, params.planPath)
		if err != nil {
			log.Error().Ctx(ctx).Err(err).Str("plan_path", params.planPath).
				Msg("failed to load Pulumi plan")
			audit.logFailure(ctx, err)
			return nil, fmt.Errorf("loading Pulumi plan: %w", err)
		}

		resources, err := ingest.MapResources(plan.GetResourcesWithContext(ctx))
		if err != nil {
			log.Error().Ctx(ctx).Err(err).Msg("failed to map resources")
			audit.logFailure(ctx, err)
			return nil, fmt.Errorf("mapping resources: %w", err)
		}

		return resources, nil
	}

	// Auto-detect from Pulumi project (neither --pulumi-json nor --pulumi-state provided)
	stackFlag, err := cmd.Flags().GetString("stack")
	if err != nil {
		return nil, fmt.Errorf("reading --stack flag: %w", err)
	}
	return resolveResourcesFromPulumi(ctx, stackFlag, modePulumiExport)
}

// resolveFromDate returns the RFC3339-formatted start ("from") date to use for cost
// calculations.
//
// If params.fromStr is non-empty it is returned unchanged. If params.statePath is set
// or both params.planPath and params.statePath are empty (auto-detection mode), the
// function attempts to determine the earliest resource creation timestamp from
// resources and returns that timestamp formatted with time.RFC3339. If auto-detection
// fails, an error is returned advising the caller to provide --from explicitly. In
// all other cases the function returns an error indicating that a --from date is
// required.
func resolveFromDate(
	ctx context.Context,
	params costActualParams,
	resources []engine.ResourceDescriptor,
) (string, error) {
	log := logging.FromContext(ctx)

	// If --from was provided, use it directly
	if params.fromStr != "" {
		return params.fromStr, nil
	}

	// Auto-detect from state timestamps (applicable for --pulumi-state and auto-detection).
	// This is equivalent to "not --pulumi-json" because validateActualInputFlags enforces
	// mutual exclusivity between --pulumi-json and --pulumi-state.
	if params.planPath == "" {
		earliest, err := engine.FindEarliestCreatedTimestamp(resources)
		if err != nil {
			log.Error().Ctx(ctx).Err(err).
				Msg("failed to auto-detect --from date from state timestamps")
			return "", fmt.Errorf(
				"auto-detecting --from date: %w (use --from to specify explicitly)",
				err,
			)
		}
		fromStr := earliest.Format(time.RFC3339)
		log.Info().Ctx(ctx).Str("auto_detected_from", fromStr).
			Msg("auto-detected --from date from earliest resource timestamp")
		return fromStr, nil
	}

	// This shouldn't happen due to validation, but handle gracefully
	return "", errors.New("--from date is required")
}
