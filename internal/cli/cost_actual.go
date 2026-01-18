package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/spf13/cobra"
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

// NewCostActualCmd creates the "actual" subcommand which fetches historical cloud-provider billing
// costs for resources described in a Pulumi preview JSON or estimates costs from Pulumi state.
//
// The command is configured with flags:
//   - --pulumi-json: path to Pulumi preview JSON output (mutually exclusive with --pulumi-state)
//   - --pulumi-state: path to Pulumi state JSON from `pulumi stack export` (mutually exclusive with --pulumi-json)
//   - --from: start date (YYYY-MM-DD or RFC3339, auto-detected from state if using --pulumi-state)
//   - --to: end date (YYYY-MM-DD or RFC3339; defaults to now)
//   - --adapter: restrict to a specific adapter plugin
//   - --output: output format (table, json, ndjson; defaults from configuration)
//   - --group-by: grouping or tag filter (resource, type, provider, date, daily, monthly, or tag:key=value)
//
// When using --pulumi-state:
//   - The --from date is auto-detected from the earliest Created timestamp if not provided
//   - Cost estimation is based on resource runtime: hourly_rate × runtime.Hours()
//   - Plugin GetActualCost is tried first; state-based estimation is used as fallback
//
// The returned *cobra.Command is ready to be added to the CLI command tree.
func NewCostActualCmd() *cobra.Command {
	var params costActualParams

	cmd := &cobra.Command{
		Use:   "actual",
		Short: "Fetch actual historical costs",
		Long: `Fetch actual historical costs for resources from cloud provider billing APIs,
or estimate costs from Pulumi state file timestamps.

When using --pulumi-state, costs are estimated based on resource runtime calculated
from the Created timestamp. The --from date is auto-detected from the earliest
timestamp if not provided.`,
		Example: `  # Get costs for the last 7 days (to defaults to now)
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

  # Use RFC3339 timestamps
  finfocus cost actual --pulumi-json plan.json --from 2025-01-01T00:00:00Z --to 2025-01-31T23:59:59Z`,
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
	cmd.Flags().StringArrayVar(&params.filter, "filter", []string{},
		"Resource filter expressions (e.g., 'type=aws:ec2/instance', 'tag:env=prod')")

	// Note: --pulumi-json and --from are no longer required - validation is done in executeCostActual

	return cmd
}

// executeCostActual orchestrates the "actual" cost workflow for a Pulumi plan or state.
// It validates input flags, loads resources, parses the time range, opens adapter plugins,
// fetches/estimates actual costs, renders the output, and emits audit entries.
//
// When using --pulumi-state:
//   - Resources are loaded from Pulumi state JSON (`pulumi stack export`)
//   - The --from date is auto-detected from the earliest Created timestamp if not provided
//   - Cost estimation uses runtime calculation: hourly_rate × time.Since(created).Hours()
//
// When using --pulumi-json:
//   - Resources are loaded from Pulumi preview JSON
//   - The --from flag is required
//   - Costs are fetched from cloud provider billing APIs
//
// cmd is the Cobra command whose context and output writer are used.
// params supplies the paths, adapter, output format, time range strings, grouping, and filter expressions.
//
// Returns an error when:
//   - Both or neither --pulumi-json and --pulumi-state are provided
//   - --from is missing when using --pulumi-json
//   - Resource loading fails
//   - Time range parsing fails
//   - Plugin communication fails
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

	resources, err := loadActualResources(ctx, params, audit)
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
	}

	resultWithErrors, err := engine.New(clients, nil).GetActualCostWithOptionsAndErrors(ctx, request)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to fetch actual costs")
		audit.logFailure(ctx, err)
		return fmt.Errorf("fetching actual costs: %w", err)
	}

	if renderErr := RenderActualCostOutput(ctx, cmd, params.output, resultWithErrors, actualGroupBy, params.estimateConfidence); renderErr != nil {
		return renderErr
	}

	log.Info().Ctx(ctx).Str("operation", "cost_actual").Int("result_count", len(resultWithErrors.Results)).
		Dur("duration_ms", time.Since(audit.start)).Msg("actual cost calculation complete")

	totalCost := 0.0
	for _, r := range resultWithErrors.Results {
		totalCost += r.TotalCost
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

// renderActualCostOutput renders actual cost results to the provided writer.
// If actualGroupBy denotes a time-based grouping, it creates cross-provider aggregations;
// otherwise it renders the raw results.
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

// validateActualInputFlags validates that exactly one of --pulumi-json or --pulumi-state is provided,
// and that --from is provided when using --pulumi-json.
func validateActualInputFlags(params costActualParams) error {
	hasPlan := params.planPath != ""
	hasState := params.statePath != ""

	// Check mutual exclusivity
	if hasPlan && hasState {
		return errors.New("--pulumi-json and --pulumi-state are mutually exclusive; use only one")
	}

	// Check at least one is provided
	if !hasPlan && !hasState {
		return errors.New("either --pulumi-json or --pulumi-state is required")
	}

	// When using --pulumi-json, --from is required
	if hasPlan && params.fromStr == "" {
		return errors.New("--from is required when using --pulumi-json")
	}

	// When using --pulumi-state, --from is optional (auto-detected from timestamps)
	// No additional validation needed here

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

// buildActualAuditParams constructs the audit parameter map for actual cost command.
func buildActualAuditParams(params costActualParams) map[string]string {
	auditParams := map[string]string{
		"from":                params.fromStr,
		"to":                  params.toStr,
		"adapter":             params.adapter,
		"output":              params.output,
		"group_by":            params.groupBy,
		"estimate_confidence": strconv.FormatBool(params.estimateConfidence),
	}
	if params.planPath != "" {
		auditParams["plan_path"] = params.planPath
	}
	if params.statePath != "" {
		auditParams["state_path"] = params.statePath
	}
	return auditParams
}

// loadActualResources loads resources from either plan or state file based on params.
func loadActualResources(
	ctx context.Context,
	params costActualParams,
	audit *auditContext,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	if params.statePath != "" {
		return loadResourcesFromState(ctx, params.statePath, audit)
	}

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

// resolveFromDate determines the 'from' date, auto-detecting from state if needed.
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

	// Auto-detect from state timestamps (only applicable for --pulumi-state)
	if params.statePath != "" {
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
