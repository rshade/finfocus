package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
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
	jobs               int
}

// defaultToNow returns s if non-empty, otherwise returns the current time in RFC3339 format.
func defaultToNow(s string) string {
	if s == "" {
		return time.Now().Format(time.RFC3339)
	}
	return s
}

// NewCostActualCmd creates the "actual" subcommand for fetching actual historical costs.
//
// The command retrieves historical costs from cloud provider billing APIs or estimates
// costs based on Pulumi preview JSON or Pulumi state timestamps. When neither
// --pulumi-json nor --pulumi-state is provided the current Pulumi project is
// auto-detected and `pulumi stack export` is used; in that mode the --from date is
// auto-detected from the earliest resource Created timestamp. Common flags registered
// include --pulumi-json, --pulumi-state, --from, --to, --adapter, --output, --group-by,
// --estimate-confidence, --fallback-estimate, --filter, and --jobs. Validation of
// flag combinations is performed by executeCostActual.
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
	cmd.Flags().IntVarP(&params.jobs, "jobs", "j", 0,
		"Number of parallel workers (0 = auto based on CPU count)")

	// Note: --pulumi-json and --from are no longer required - validation is done in executeCostActual

	return cmd
}

// executeCostActual orchestrates the "actual" cost workflow: it validates input flags,
// loads and filters resources, resolves the time range, opens adapter plugins, fetches
// historical costs via the engine, merges recommendations, renders output, evaluates
// budget status, and records audit information. It returns an error when any step fails
// (flag validation, resource loading, time parsing, plugin initialization, cost retrieval,
// rendering, or budget evaluation) and nil on success.
func executeCostActual(cmd *cobra.Command, params costActualParams) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)

	if err := validateActualParams(params); err != nil {
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

	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	eng, historyStore, combinedCleanup := newEngineWithCacheAndHistory(ctx, cmd, clients, nil)
	defer combinedCleanup()
	eng = eng.WithJobs(params.jobs)

	recordDescriptorHistory(ctx, historyStore, resources)

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

	// Enrich with historical resources before filtering so merged entries
	// are also subject to filter criteria.
	resources = enrichWithHistoricalResources(ctx, historyStore, resources, from, to)

	resources, err = ApplyFilters(ctx, resources, params.filter)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("invalid filter expression")
		audit.logFailure(ctx, err)
		return fmt.Errorf("applying filters: %w", err)
	}

	request := buildActualCostRequest(params, resources, from, to)
	start := time.Now()
	resultWithErrors, err := eng.GetActualCostWithOptionsAndErrors(ctx, request)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to fetch actual costs")
		audit.logFailure(ctx, err)
		return fmt.Errorf("fetching actual costs: %w", err)
	}

	fetchAndMergeRecommendations(ctx, eng, resources, resultWithErrors.Results)

	if renderErr := RenderActualCostOutput(
		ctx, cmd, params.output, resultWithErrors, request.GroupBy, params.estimateConfidence,
	); renderErr != nil {
		return renderErr
	}

	printTimingOutput(cmd, start, len(resources), params.output)

	log.Info().Ctx(ctx).Str("operation", "cost_actual").Int("result_count", len(resultWithErrors.Results)).
		Dur("duration_ms", time.Since(audit.start)).Msg("actual cost calculation complete")

	totalCost := sumTotalCosts(resultWithErrors.Results)
	if budgetErr := evaluateBudgetStatus(cmd, resultWithErrors.Results, totalCost); budgetErr != nil {
		audit.logFailure(ctx, budgetErr)
		return budgetErr
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
// If groupBy is of the form "tag:key=value", it returns a map containing {key: value} and an empty actualGroupBy.
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

// validateActualParams validates all parameters for the actual cost command.
func validateActualParams(params costActualParams) error {
	if params.jobs < 0 {
		return fmt.Errorf("--jobs must be non-negative, got %d", params.jobs)
	}
	return validateActualInputFlags(params)
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

// loadActualResources loads resource descriptors for the actual-cost command.
// It chooses the source based on params:
// - If params.statePath is set, it loads resources from the given Pulumi state file.
// - If params.planPath is set, it loads and maps resources from the given Pulumi plan JSON.
// - Otherwise it auto-detects resources from the current Pulumi project, using the value of the command's --stack flag.
//
// Parameters:
//   - ctx: request context used for logging and cancellation.
//   - cmd: the cobra command used to read the --stack flag when auto-detecting.
//   - params: command parameters that determine which source to use (planPath, statePath).
//   - audit: audit context to record failures when loading or mapping resources.
//
// Returns:
//   - a slice of engine.ResourceDescriptor on success.
//   - a non-nil error if loading or mapping fails, if reading the --stack flag fails, or if auto-detection cannot resolve resources.
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

// resolveFromDate determines the RFC3339-formatted start date ("from") to use for the actual cost
// calculation.
//
// If params.fromStr is non-empty, it is returned unchanged. If params.planPath is empty (i.e. not
// using a Pulumi plan JSON), the function attempts to auto-detect the earliest resource Created
// timestamp from resources and returns that timestamp formatted as RFC3339. If auto-detection fails,
// an error is returned advising to specify --from explicitly. If params.planPath is set and no
// fromStr was provided, an error is returned indicating that a --from date is required.
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

// buildActualCostRequest constructs an ActualCostRequest from the resolved parameters.
func buildActualCostRequest(
	params costActualParams,
	resources []engine.ResourceDescriptor,
	from, to time.Time,
) engine.ActualCostRequest {
	tags, actualGroupBy := parseTagFilter(params.groupBy)
	return engine.ActualCostRequest{
		Resources:          resources,
		From:               from,
		To:                 to,
		Adapter:            params.adapter,
		GroupBy:            actualGroupBy,
		Tags:               tags,
		EstimateConfidence: params.estimateConfidence,
		FallbackEstimate:   params.fallbackEstimate,
	}
}

// recordDescriptorHistory records engine ResourceDescriptors to the history store
// (fire-and-forget). Skips when store is nil or disabled.
func recordDescriptorHistory(ctx context.Context, store history.Store, resources []engine.ResourceDescriptor) {
	if store == nil || !store.IsEnabled() {
		return
	}
	stackCtx := detectHistoryStackContext(ctx)
	writer := history.NewWriter(store, *logging.FromContext(ctx))
	stateResources := convertDescriptorsToHistoryState(resources)
	writer.RecordStateSnapshot(stackCtx, stateResources)
}

// enrichWithHistoricalResources queries the history store for historical and
// deleted cloud IDs within the date range and merges them into the resource
// list. Returns resources unchanged if history is nil or disabled.
func enrichWithHistoricalResources(
	ctx context.Context,
	store history.Store,
	resources []engine.ResourceDescriptor,
	from, to time.Time,
) []engine.ResourceDescriptor {
	if store == nil || !store.IsEnabled() {
		return resources
	}
	log := logging.FromContext(ctx)

	// Capture current URN hashes before merging historical entries so
	// GetDeletedResources correctly identifies resources no longer in state.
	currentURNHashes := buildCurrentURNHashSet(resources)

	stackCtx := detectHistoryStackContext(ctx)
	reader := history.NewReader(store, *log)
	historical, histErr := reader.GetResourcesForPeriod(stackCtx, from.Unix(), to.Unix())
	if histErr != nil {
		log.Warn().Ctx(ctx).Err(histErr).
			Str("component", "history").
			Msg("failed to query historical resources, continuing without history")
	} else if len(historical) > 0 {
		resources = MergeHistoricalResources(resources, historical)
		log.Debug().Ctx(ctx).
			Str("component", "history").
			Int("historical_resources", len(historical)).
			Int("total_resources", len(resources)).
			Msg("merged historical cloud IDs into resource list")
	}

	// Also include deleted resources (in history but not in current state).
	deleted, delErr := store.GetDeletedResources(
		stackCtx.Hash(), currentURNHashes, from.Unix(), to.Unix(),
	)
	if delErr != nil {
		log.Warn().Ctx(ctx).Err(delErr).
			Str("component", "history").
			Msg("failed to query deleted resources, continuing without them")
	} else if len(deleted) > 0 {
		deletedHistorical := entriesToHistoricalResources(deleted)
		resources = MergeHistoricalResources(resources, deletedHistorical)
		log.Debug().Ctx(ctx).
			Str("component", "history").
			Int("deleted_resources", len(deleted)).
			Int("total_resources", len(resources)).
			Msg("merged deleted resource cloud IDs into resource list")
	}

	return resources
}

// buildCurrentURNHashSet builds a set of URN hashes from the current
// ResourceDescriptor list, used to identify deleted resources.
func buildCurrentURNHashSet(resources []engine.ResourceDescriptor) map[string]bool {
	hashes := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			hashes[history.URNHash(r.ID)] = true
		}
	}
	return hashes
}

// entriesToHistoricalResources converts raw history entries into
// HistoricalResource structs suitable for MergeHistoricalResources.
func entriesToHistoricalResources(entries []history.ResourceHistoryEntry) []history.HistoricalResource {
	grouped := make(map[string]*history.HistoricalResource)
	for _, e := range entries {
		hr, exists := grouped[e.URN]
		if !exists {
			hr = &history.HistoricalResource{
				URN:      e.URN,
				Type:     e.Type,
				Provider: e.Provider,
				CloudIDs: []string{},
				Tags:     make(map[string]string),
			}
			grouped[e.URN] = hr
		}
		if !slices.Contains(hr.CloudIDs, e.CloudID) {
			hr.CloudIDs = append(hr.CloudIDs, e.CloudID)
		}
	}
	result := make([]history.HistoricalResource, 0, len(grouped))
	for _, hr := range grouped {
		result = append(result, *hr)
	}
	return result
}

// convertDescriptorsToHistoryState converts engine ResourceDescriptors to
// history StateResources for recording to the history store.
func convertDescriptorsToHistoryState(resources []engine.ResourceDescriptor) []history.StateResource {
	result := make([]history.StateResource, 0, len(resources))
	for _, r := range resources {
		cloudID, _ := r.Properties["pulumi:cloudId"].(string)
		if cloudID == "" {
			continue
		}
		result = append(result, history.StateResource{
			URN:      r.ID,
			CloudID:  cloudID,
			Type:     r.Type,
			Provider: r.Provider,
		})
	}
	return result
}

// MergeHistoricalResources enriches a current ResourceDescriptor list with
// historical cloud IDs from the history store. For each HistoricalResource,
// it creates a new ResourceDescriptor per cloud ID that is not already present
// in the current list. This ensures billing queries include costs from replaced
// or deleted resources.
//
// The function preserves the engine's single-cloud-ID-per-resource invariant:
// a resource replaced mid-month produces TWO ResourceDescriptors (one per
// cloud ID), each flowing through the existing adapter pipeline unchanged.
func MergeHistoricalResources(
	current []engine.ResourceDescriptor,
	historical []history.HistoricalResource,
) []engine.ResourceDescriptor {
	if len(historical) == 0 {
		return current
	}

	// Build set of existing (provider, cloudID) pairs for deduplication.
	// Keyed by "provider|cloudID" composite to avoid collisions when different
	// providers happen to share the same cloud ID string while still deduplicating
	// same-provider entries correctly.
	existingCloudIDs := make(map[string]bool)
	for i := range current {
		if cloudID, ok := current[i].Properties["pulumi:cloudId"]; ok {
			if idStr, isStr := cloudID.(string); isStr {
				existingCloudIDs[current[i].Provider+"|"+idStr] = true
			}
		}
	}

	merged := make([]engine.ResourceDescriptor, 0, len(current)+len(historical))
	merged = append(merged, current...)

	for _, hr := range historical {
		for _, cloudID := range hr.CloudIDs {
			compositeKey := hr.Provider + "|" + cloudID
			if existingCloudIDs[compositeKey] {
				continue
			}

			existingCloudIDs[compositeKey] = true
			merged = append(merged, engine.ResourceDescriptor{
				ID:       hr.URN,
				Type:     hr.Type,
				Provider: hr.Provider,
				Properties: map[string]interface{}{
					"pulumi:cloudId": cloudID,
				},
			})
		}
	}

	return merged
}
