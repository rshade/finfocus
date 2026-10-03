package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/cli/pagination"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/tui"
)

// Note: The engine.Recommendation struct has these fields:
// - ResourceID string
// - Type string (maps from proto ActionType)
// - Description string
// - EstimatedSavings float64
// - Currency string

const (
	// progressDelayMS is the delay before showing progress indicator (500ms).
	progressDelayMS = 500
	// progressTickerMS is the spinner update interval (100ms).
	progressTickerMS = 100
	// batchSize is the number of resources per batch for progress calculation.
	progressBatchSize = 100
	// statusActive is the default status label for active recommendations.
	statusActive engine.RecommendationStatus = engine.RecommendationStatusActive
)

// costRecommendationsParams holds the parameters for the recommendations command execution.
type costRecommendationsParams struct {
	planPath         string
	adapter          string
	output           string
	filter           []string
	verbose          bool
	limit            int
	page             int
	pageSize         int
	offset           int
	sort             string
	includeDismissed bool
	scoringDryRun    bool
	noScoring        bool
}

// NewCostRecommendationsCmd creates the "recommendations" subcommand that fetches cost optimization
// recommendations for resources described in a Pulumi preview JSON.
//
// The command is configured with flags:
//   - --pulumi-json (required): path to Pulumi preview JSON output
//   - --adapter: restrict to a specific adapter plugin
//   - --output: output format (table, json, ndjson; defaults from configuration)
//   - --filter: filter expressions for recommendations (e.g., 'action=MIGRATE', 'risk<=0.3')
//   - --sort: sort expression (e.g., 'savings:desc', 'risk')
//   - --scoring-dry-run: print the requests that scoring would send, and send nothing
//   - --no-scoring: skip the optional scoring step for this run
//
// The returned *cobra.Command is ready to be added to the CLI command tree.
func NewCostRecommendationsCmd() *cobra.Command {
	var params costRecommendationsParams

	cmd := &cobra.Command{
		Use:   "recommendations",
		Short: "Get cost optimization recommendations",
		Long: `Fetch cost optimization recommendations for resources from cloud provider APIs and plugins.

By default, shows a summary with the top 5 recommendations sorted by savings.
Use --verbose to see all recommendations with full details.

In interactive terminals, launches a TUI with:
  - Keyboard navigation (up/down arrows)
  - Filter by typing '/' and entering search text
  - Sort cycling by pressing 's'
  - Detail view by pressing Enter
  - Quit by pressing 'q' or Ctrl+C

Valid action types for filtering:
  RIGHTSIZE, TERMINATE, PURCHASE_COMMITMENT, ADJUST_REQUESTS, MODIFY,
  DELETE_UNUSED, MIGRATE, CONSOLIDATE, SCHEDULE, REFACTOR, OTHER`,
		Example: `  # Get all cost optimization recommendations (shows top 5 by savings)
  finfocus cost recommendations --pulumi-json plan.json

  # Show all recommendations with full details
  finfocus cost recommendations --pulumi-json plan.json --verbose

  # Output recommendations as JSON (includes summary section)
  finfocus cost recommendations --pulumi-json plan.json --output json

  # Output as newline-delimited JSON (first line is summary)
  finfocus cost recommendations --pulumi-json plan.json --output ndjson

  # Filter recommendations by action type
  finfocus cost recommendations --pulumi-json plan.json --filter "action=MIGRATE"

  # Filter by multiple action types (comma-separated)
  finfocus cost recommendations --pulumi-json plan.json --filter "action=RIGHTSIZE,TERMINATE"

  # Use a specific adapter plugin
  finfocus cost recommendations --pulumi-json plan.json --adapter kubecost

  # With scoring enabled in the config: list the lowest-risk recommendations first
  finfocus cost recommendations --pulumi-json plan.json --sort risk:asc --filter "risk<=0.3"

  # Show exactly what would be sent to the scorer plugin, without sending it
  finfocus cost recommendations --pulumi-json plan.json --scoring-dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeCostRecommendations(cmd, params)
		},
	}

	cmd.Flags().
		StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON output (required)")
	cmd.Flags().StringVar(&params.adapter, "adapter", "", "Use only the specified adapter plugin")

	// Use configuration default if no output format specified
	defaultFormat := config.GetDefaultOutputFormat()
	cmd.Flags().
		StringVar(&params.output, "output", defaultFormat, "Output format: table, json, or ndjson")
	cmd.Flags().StringArrayVar(&params.filter, "filter", []string{},
		"Filter expressions (e.g., 'action=MIGRATE,RIGHTSIZE')")
	cmd.Flags().BoolVar(&params.verbose, "verbose", false,
		"Show all recommendations with full details (default shows top 5 by savings)")
	cmd.Flags().IntVar(&params.limit, "limit", 0,
		"Maximum number of recommendations to return (0 = unlimited)")
	cmd.Flags().IntVar(&params.page, "page", 0,
		"Page number for page-based pagination (1-indexed, 0 = disabled)")
	cmd.Flags().IntVar(&params.pageSize, "page-size", 0,
		"Number of items per page (requires --page)")
	cmd.Flags().IntVar(&params.offset, "offset", 0,
		"Number of items to skip for offset-based pagination")
	cmd.Flags().StringVar(&params.sort, "sort", "",
		"Sort expression (e.g., 'savings:desc', 'name:asc'; with scoring: 'risk', 'priority', 'worth_acting')")
	cmd.Flags().BoolVar(&params.scoringDryRun, "scoring-dry-run", false,
		"Print exactly what would be sent to the scorer plugin and send nothing (requires scoring.enabled)")
	cmd.Flags().BoolVar(&params.noScoring, "no-scoring", false,
		"Skip the scoring step for this run even when scoring.enabled is set")
	cmd.Flags().BoolVar(&params.includeDismissed, "include-dismissed", false,
		"Show dismissed and snoozed recommendations alongside active ones")

	_ = cmd.MarkFlagRequired("pulumi-json")

	// Add subcommands for recommendation lifecycle management
	cmd.AddCommand(
		newRecommendationsDismissCmd(),
		newRecommendationsSnoozeCmd(),
		newRecommendationsUndismissCmd(),
		newRecommendationsHistoryCmd(),
	)
	addAccessibilityFlags(cmd)

	return cmd
}

// executeCostRecommendations orchestrates the cost recommendations workflow: it loads
// resources from a Pulumi plan, opens adapter plugins, initializes a cache, queries the
// recommendation engine, optionally merges dismissed/snoozed records, applies action-type
// filters, sorting, and pagination, renders the results, and records audit metadata.
// It returns an error if any step fails.
func executeCostRecommendations(cmd *cobra.Command, params costRecommendationsParams) error {
	ctx := cmd.Context()
	log := logging.FromContext(ctx)
	params.output = resolveOutputFormat(cmd, "output", params.output)
	if !isValidOutputFormat(engine.OutputFormat(config.GetOutputFormat(params.output))) {
		return fmt.Errorf("unsupported output format: %s (supported: table, json, ndjson)", params.output)
	}

	scoringCfg := config.New().Scoring.Resolve()
	if flagErr := validateScoringFlags(params, scoringCfg); flagErr != nil {
		return flagErr
	}

	log.Debug().Ctx(ctx).Str("operation", "cost_recommendations").Str("plan_path", params.planPath).
		Msg("starting recommendations fetch")

	// Setup audit context for logging
	auditParams := map[string]string{
		auditKeyPulumiJSON: params.planPath,
		auditKeyOutput:     params.output,
	}
	if len(params.filter) > 0 {
		auditParams["filter"] = strings.Join(params.filter, ",")
	}
	audit := newAuditContext(ctx, "cost recommendations", auditParams)

	// Load and map resources from Pulumi plan
	resources, err := loadAndMapResources(ctx, params.planPath, audit)
	if err != nil {
		return err
	}

	// Open plugin connections
	clients, cleanup, err := openPlugins(ctx, params.adapter, audit)
	if err != nil {
		return err
	}
	defer cleanup()

	// Create engine with optional cache and router
	eng, cacheStore, cacheCleanup := newEngineWithCache(ctx, cmd, clients, nil)
	defer cacheCleanup()

	// Fetch recommendations with progress indicator
	result, err := fetchRecommendationsWithProgress(ctx, cmd, eng, resources)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msg("failed to fetch recommendations")
		audit.logFailure(ctx, err)
		return fmt.Errorf("fetching recommendations: %w", err)
	}

	if result == nil {
		log.Warn().Ctx(ctx).Msg("no recommendation results returned")
		return nil
	}

	// Annotate active recommendations with status
	annotateActiveStatus(result)

	// Optional scoring step: rates active recommendations, never dismisses or hides any
	done, scorerCleanup, scoreErr := runScoringStep(ctx, cmd, params, scoringCfg, clients, cacheStore, audit, result)
	defer scorerCleanup()
	if scoreErr != nil || done {
		return scoreErr
	}

	// Merge dismissed/snoozed recommendations if --include-dismissed
	if params.includeDismissed {
		if mergeErr := mergeDismissedRecommendations(ctx, result); mergeErr != nil {
			log.Warn().Ctx(ctx).Err(mergeErr).
				Msg("failed to merge dismissed recommendations, continuing with active only")
		}
	}

	filteredResult, filteredRecommendations, paginationMeta, err := buildRecommendationsView(ctx, result, params)
	if err != nil {
		return err
	}

	// Render output
	if renderErr := RenderRecommendationsOutput(
		ctx, cmd, params.output, filteredResult, params.verbose, paginationMeta,
	); renderErr != nil {
		return renderErr
	}

	log.Info().Ctx(ctx).Str("operation", "cost_recommendations").
		Int("recommendation_count", len(filteredRecommendations)).
		Dur("duration_ms", time.Since(audit.start)).
		Msg("recommendations fetch complete")

	audit.logSuccess(ctx, len(filteredRecommendations), filteredResult.TotalSavings)
	return nil
}

// buildRecommendationsView applies filters, sorting and pagination to result. It returns the
// result to render, the full filtered list (before pagination), and the pagination metadata.
func buildRecommendationsView(
	ctx context.Context,
	result *engine.RecommendationsResult,
	params costRecommendationsParams,
) (*engine.RecommendationsResult, []engine.Recommendation, *pagination.PaginationMeta, error) {
	filtered, err := applyActionTypeFilters(ctx, result.Recommendations, params.filter)
	if err != nil {
		return nil, nil, nil, err
	}

	filtered, err = applyScoreFilters(filtered, params.filter)
	if err != nil {
		return nil, nil, nil, err
	}

	filtered, err = applySortExpression(ctx, filtered, params.sort)
	if err != nil {
		return nil, nil, nil, err
	}

	paginated, paginationMeta, err := paginateRecommendations(ctx, filtered, params)
	if err != nil {
		return nil, nil, nil, err
	}

	view := &engine.RecommendationsResult{
		Recommendations: paginated,
		Errors:          result.Errors,
		TotalSavings:    calculateTotalSavings(filtered),
		Currency:        result.Currency,
		Scoring:         result.Scoring,
	}
	if field := scoreSortField(params.sort); field != "" && view.Scoring != nil {
		summary := *view.Scoring
		summary.OrderedBy = field
		view.Scoring = &summary
	}
	return view, filtered, paginationMeta, nil
}

// fetchRecommendationsWithProgress fetches recommendations while showing a progress
// indicator for operations that exceed 500ms.
func fetchRecommendationsWithProgress(
	ctx context.Context,
	cmd *cobra.Command,
	eng *engine.Engine,
	resources []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()

	var spinnerWg sync.WaitGroup

	spinnerWg.Go(func() {
		showProgressIndicator(progressCtx, cmd, resources)
	})

	result, err := eng.GetRecommendationsForResources(ctx, resources)

	cancelProgress()
	spinnerWg.Wait()

	// Clear progress line if it was shown
	if term.IsTerminal(int(os.Stderr.Fd())) {
		fmt.Fprint(cmd.ErrOrStderr(), "\r\033[K")
	}

	return result, err
}

// annotateActiveStatus sets the status to "Active" for any recommendation without a status.
func annotateActiveStatus(result *engine.RecommendationsResult) {
	for i := range result.Recommendations {
		if result.Recommendations[i].Status == "" {
			result.Recommendations[i].Status = statusActive
		}
	}
}

// applyActionTypeFilters applies all action type filter expressions to recommendations.
func applyActionTypeFilters(
	ctx context.Context,
	recommendations []engine.Recommendation,
	filters []string,
) ([]engine.Recommendation, error) {
	filtered := recommendations
	for _, f := range filters {
		if f == "" {
			continue
		}
		var err error
		filtered, err = applyActionTypeFilter(ctx, filtered, f)
		if err != nil {
			return nil, err
		}
	}
	return filtered, nil
}

// applySortExpression applies a sort expression to recommendations.
// Returns the original slice unchanged if sortExpr is empty.
func applySortExpression(
	ctx context.Context,
	recommendations []engine.Recommendation,
	sortExpr string,
) ([]engine.Recommendation, error) {
	if sortExpr == "" {
		return recommendations, nil
	}

	log := logging.FromContext(ctx)
	sorter := pagination.NewRecommendationSorter()

	field, order, parseErr := pagination.ParseSortExpression(sortExpr)
	if parseErr != nil {
		return nil, fmt.Errorf("invalid sort expression: %w", parseErr)
	}

	if !sorter.IsValidField(field) {
		return nil, fmt.Errorf("invalid sort field: %q (valid fields: %s)",
			field, strings.Join(sorter.GetValidFields(), ", "))
	}

	sorted := sorter.Sort(recommendations, field, order)
	log.Debug().Ctx(ctx).
		Str("field", field).Str("order", order).
		Int("count", len(sorted)).
		Msg("applied sorting")

	return sorted, nil
}

// paginateRecommendations applies pagination to recommendations and returns the
// paginated slice along with pagination metadata.
func paginateRecommendations(
	ctx context.Context,
	recommendations []engine.Recommendation,
	params costRecommendationsParams,
) ([]engine.Recommendation, *pagination.PaginationMeta, error) {
	log := logging.FromContext(ctx)

	paginationParams := pagination.PaginationParams{
		Limit:    params.limit,
		Offset:   params.offset,
		Page:     params.page,
		PageSize: params.pageSize,
	}

	if validationErr := paginationParams.Validate(); validationErr != nil {
		return nil, nil, fmt.Errorf("invalid pagination parameters: %w", validationErr)
	}

	if !paginationParams.IsEnabled() {
		return recommendations, nil, nil
	}

	paginated := applyPaginationWindow(paginationParams, recommendations)

	log.Debug().Ctx(ctx).
		Int("total", len(recommendations)).
		Int("returned", len(paginated)).
		Msg("applied pagination")

	meta := pagination.NewPaginationMeta(paginationParams, len(recommendations))
	log.Debug().Ctx(ctx).
		Int("current_page", meta.CurrentPage).
		Int("total_pages", meta.TotalPages).
		Bool("has_next", meta.HasNext).
		Bool("has_previous", meta.HasPrevious).
		Msg("generated pagination metadata")

	return paginated, &meta, nil
}

// applyPaginationWindow slices recommendations based on pagination offset and limit,
// handling out-of-bounds page capping for page-based pagination.
func applyPaginationWindow(
	pp pagination.PaginationParams,
	recommendations []engine.Recommendation,
) []engine.Recommendation {
	offset, limit := pp.CalculateOffsetLimit()

	// Handle out-of-bounds page edge case (T036):
	// cap offset to the last available page for page-based pagination.
	if pp.IsPageBased() && offset >= len(recommendations) && len(recommendations) > 0 {
		pageSize := pp.PageSize
		if pageSize <= 0 {
			pageSize = len(recommendations)
		}
		offset = ((len(recommendations) - 1) / pageSize) * pageSize
	}

	if offset >= len(recommendations) {
		return []engine.Recommendation{}
	}

	end := offset + limit
	if limit == 0 || end > len(recommendations) {
		end = len(recommendations)
	}

	return recommendations[offset:end]
}

// applyActionTypeFilter filters recommendations by action type based on a filter expression.
// Filter format: "action=TYPE1,TYPE2,..."
// Matching is case-insensitive. Returns error for invalid action types.
func applyActionTypeFilter(
	ctx context.Context,
	recommendations []engine.Recommendation,
	filter string,
) ([]engine.Recommendation, error) {
	log := logging.FromContext(ctx)

	// Parse filter expression: "action=MIGRATE,RIGHTSIZE"
	parts := strings.SplitN(filter, "=", 2) //nolint:mnd // key=value format
	if len(parts) != 2 {                    //nolint:mnd // key=value has 2 parts
		return recommendations, nil // Not an action filter, return unchanged
	}

	key := strings.TrimSpace(strings.ToLower(parts[0]))
	if key != "action" {
		return recommendations, nil // Not an action filter, return unchanged
	}

	// Parse and validate action types using proto utilities
	actionTypesStr := strings.TrimSpace(parts[1])
	actionTypes, err := proto.ParseActionTypeFilter(actionTypesStr)
	if err != nil {
		return nil, fmt.Errorf("invalid action type filter: %w", err)
	}

	// Filter recommendations using proto.MatchesActionType
	var filtered []engine.Recommendation
	for _, rec := range recommendations {
		if proto.MatchesActionType(rec.Type, actionTypes) {
			filtered = append(filtered, rec)
		}
	}

	log.Debug().Ctx(ctx).
		Int("original_count", len(recommendations)).
		Int("filtered_count", len(filtered)).
		Str("filter", filter).
		Msg("applied action type filter")

	return filtered, nil
}

// calculateTotalSavings calculates the total estimated savings from recommendations.
func calculateTotalSavings(recommendations []engine.Recommendation) float64 {
	var total float64
	for _, rec := range recommendations {
		total += rec.EstimatedSavings
	}
	return total
}

// RenderRecommendationsOutput routes the recommendations results to the appropriate
// rendering function based on the output format and terminal mode.
// In interactive terminals, it launches the TUI; otherwise, it renders table output.
// Returns an error if result is nil.
func RenderRecommendationsOutput(
	_ context.Context,
	cmd *cobra.Command,
	outputFormat string,
	result *engine.RecommendationsResult,
	verbose bool,
	paginationMeta *pagination.PaginationMeta,
) error {
	if result == nil {
		return errors.New("render recommendations: result cannot be nil")
	}

	fmtType := engine.OutputFormat(config.GetOutputFormat(outputFormat))

	// Validate format is supported
	if !isValidOutputFormat(fmtType) {
		return fmt.Errorf("unsupported output format: %s", fmtType)
	}

	// JSON/NDJSON bypass TUI entirely
	switch fmtType {
	case engine.OutputJSON:
		return renderRecommendationsJSON(cmd.OutOrStdout(), result, paginationMeta)
	case engine.OutputNDJSON:
		// T050: Disable pagination metadata in NDJSON streaming mode
		// NDJSON is designed for line-by-line streaming without pagination
		err := renderRecommendationsNDJSON(cmd.OutOrStdout(), result, nil)
		// T049: Handle SIGPIPE gracefully for streaming output
		// When piped to commands like `head -n 5`, suppress broken pipe errors
		if isBrokenPipe(err) {
			return nil
		}
		return err
	case engine.OutputTable:
		// Fall through to terminal mode detection below
	}

	// For table output, detect terminal mode from flags and the environment.
	mode := outputModeFromCmd(cmd)

	switch mode {
	case tui.OutputModeInteractive:
		return runInteractiveRecommendations(result.Recommendations)

	case tui.OutputModeStyled:
		// Styled mode renders the summary with lipgloss styling
		return renderStyledRecommendationsOutput(cmd.OutOrStdout(), result, verbose)

	case tui.OutputModePlain:
		fallthrough
	default:
		return renderRecommendationsTableWithVerbose(cmd.OutOrStdout(), result, verbose)
	}
}

// runInteractiveRecommendations launches the interactive TUI for recommendations.
// Uses NewRecommendationsViewModel which starts with data already loaded.
func runInteractiveRecommendations(recommendations []engine.Recommendation) error {
	model := tui.NewRecommendationsViewModel(recommendations)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run interactive recommendations TUI: %w", err)
	}
	return nil
}

// renderStyledRecommendationsOutput renders styled output using TUI summary renderer.
func renderStyledRecommendationsOutput(
	w io.Writer,
	result *engine.RecommendationsResult,
	verbose bool,
) error {
	// Use TUI summary renderer for styled output
	summary := tui.NewRecommendationsSummary(result.Recommendations)
	fmt.Fprint(w, tui.RenderRecommendationsSummaryTUI(summary, tui.TerminalWidth()))

	// Then render the table
	return renderRecommendationsTableWithVerbose(w, result, verbose)
}

// tabPadding is the minimum column padding for tabwriter output.
const tabPadding = 2

// defaultTopRecommendations is the number of recommendations to show by default.
const defaultTopRecommendations = 5

// headerSeparatorLen is the length of the separator line below section headers.
const headerSeparatorLen = 40

// renderRecommendationsSummary renders a summary section showing aggregate statistics.
// This includes total count, total savings, and breakdown by action type.
func renderRecommendationsSummary(w io.Writer, recommendations []engine.Recommendation) {
	totalSavings := 0.0
	countByAction := make(map[string]int)
	savingsByAction := make(map[string]float64)
	currency := defaultCurrency

	for _, rec := range recommendations {
		totalSavings += rec.EstimatedSavings
		countByAction[rec.Type]++
		savingsByAction[rec.Type] += rec.EstimatedSavings
		if rec.Currency != "" {
			currency = rec.Currency
		}
	}

	fmt.Fprintln(w, "RECOMMENDATIONS SUMMARY")
	fmt.Fprintln(w, "=======================")
	fmt.Fprintf(w, "Total Recommendations: %d\n", len(recommendations))
	fmt.Fprintf(w, "Total Potential Savings: %.2f %s\n", totalSavings, currency)

	if len(countByAction) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "By Action Type:")

		// Sort action types for consistent output
		actionTypes := make([]string, 0, len(countByAction))
		for at := range countByAction {
			actionTypes = append(actionTypes, at)
		}
		sortActionTypes(actionTypes)

		for _, at := range actionTypes {
			count := countByAction[at]
			savings := savingsByAction[at]
			fmt.Fprintf(
				w,
				"  %s: %d (%.2f %s)\n",
				formatActionTypeLabel(at),
				count,
				savings,
				currency,
			)
		}
	}

	fmt.Fprintln(w)
}

// sortActionTypes sorts action type strings alphabetically.
func sortActionTypes(actionTypes []string) {
	slices.Sort(actionTypes)
}

// renderRecommendationsTableWithVerbose renders recommendations in table format.
// When verbose is false: shows summary section and top 5 recommendations sorted by savings.
// When verbose is true: shows summary section and ALL recommendations sorted by savings.
func renderRecommendationsTableWithVerbose(
	w io.Writer,
	result *engine.RecommendationsResult,
	verbose bool,
) error {
	// Render summary section first
	renderRecommendationsSummary(w, result.Recommendations)
	if result.Scoring != nil {
		fmt.Fprint(w, scoringSummaryLine(result.Scoring, result.Recommendations))
		fmt.Fprintln(w)
	}

	// Handle empty case
	if len(result.Recommendations) == 0 {
		fmt.Fprintln(w, "No recommendations available.")
		return nil
	}

	// Sort by savings unless the user asked for a score order
	sortLabel := "SAVINGS"
	sorted := sortRecommendationsBySavings(result.Recommendations)
	if result.Scoring != nil && result.Scoring.OrderedBy != "" {
		sorted = result.Recommendations
		sortLabel = strings.ToUpper(strings.ReplaceAll(result.Scoring.OrderedBy, "_", " "))
	}
	displayRecs := sorted
	showMoreHint := false

	// In non-verbose mode, limit to top 5
	if !verbose && len(sorted) > defaultTopRecommendations {
		displayRecs = sorted[:defaultTopRecommendations]
		showMoreHint = true
	}

	// Header for recommendations
	if verbose {
		fmt.Fprintf(w, "ALL %d RECOMMENDATIONS (SORTED BY %s)\n", len(displayRecs), sortLabel)
	} else {
		fmt.Fprintf(w, "TOP %d RECOMMENDATIONS BY %s\n", len(displayRecs), sortLabel)
	}
	fmt.Fprintln(w, strings.Repeat("-", headerSeparatorLen))

	tw := tabwriter.NewWriter(w, 0, 0, tabPadding, ' ', 0)

	// Detect if any recommendations have status annotations
	hasStatus := hasStatusAnnotations(displayRecs)
	showScores := hasScores(displayRecs)

	// Header
	header, rule := "RESOURCE\tACTION TYPE\tDESCRIPTION\tSAVINGS", "--------\t-----------\t-----------\t-------"
	if hasStatus {
		header = "STATUS\t" + header
		rule = "------\t" + rule
	}
	if showScores {
		header += "\tRISK\tFALSE POS\tWORTH\tPRIORITY\tREVIEW\tGROUP"
		rule += "\t----\t---------\t-----\t--------\t------\t-----"
	}
	fmt.Fprintln(tw, header)
	fmt.Fprintln(tw, rule)

	// Recommendations (top 5 or all in verbose)
	for _, rec := range displayRecs {
		writeRecommendationRow(tw, rec, hasStatus, showScores)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table writer: %w", err)
	}

	// Show hint if more recommendations exist
	if showMoreHint {
		remaining := len(sorted) - defaultTopRecommendations
		fmt.Fprintf(
			w,
			"\n... and %d more recommendation(s). Use --verbose to see all.\n",
			remaining,
		)
	}

	// Errors
	if result.HasErrors() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ERRORS")
		fmt.Fprintln(w, "======")
		fmt.Fprintln(w, result.ErrorSummary())
	}

	return nil
}

// writeRecommendationRow writes a single recommendation row to the tabwriter.
func writeRecommendationRow(tw *tabwriter.Writer, rec engine.Recommendation, hasStatus, showScores bool) {
	savings := ""
	if rec.EstimatedSavings > 0 {
		savings = fmt.Sprintf("%.2f %s", rec.EstimatedSavings, rec.Currency)
	}

	// Truncate long descriptions
	description := rec.Description
	const maxDescLen = 50
	if len(description) > maxDescLen {
		description = description[:maxDescLen-3] + "..."
	}

	row := fmt.Sprintf("%s\t%s\t%s\t%s",
		rec.ResourceID, formatActionTypeLabel(rec.Type), description, savings)
	if hasStatus {
		status := rec.Status
		if status == "" {
			status = statusActive
		}
		row = fmt.Sprintf("%s\t%s", status, row)
	}
	if showScores {
		row += "\t" + scoreCells(rec.Scores)
	}
	fmt.Fprintln(tw, row)
}

// renderRecommendationsJSON renders recommendations in JSON format.
func renderRecommendationsJSON(
	w io.Writer,
	result *engine.RecommendationsResult,
	paginationMeta *pagination.PaginationMeta,
) error {
	// Build summary from recommendations
	summary := buildJSONSummary(result.Recommendations)

	output := recommendationsJSONOutput{
		Summary:         summary,
		Recommendations: make([]recommendationJSON, 0, len(result.Recommendations)),
		TotalSavings:    result.TotalSavings,
		Currency:        result.Currency,
		Errors:          result.Errors,
		Pagination:      paginationMeta,
		Scoring:         newScoringJSON(result.Scoring),
	}

	for _, rec := range result.Recommendations {
		jsonRec := newRecommendationJSON(rec)
		output.Recommendations = append(output.Recommendations, jsonRec)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// renderRecommendationsNDJSON renders recommendations in NDJSON format.
// The first line is a summary object with type: "summary", followed by
// individual recommendation objects.
func renderRecommendationsNDJSON(
	w io.Writer,
	result *engine.RecommendationsResult,
	paginationMeta *pagination.PaginationMeta,
) error {
	encoder := json.NewEncoder(w)

	// Build and emit summary as first line
	jsonSum := buildJSONSummary(result.Recommendations)
	summary := ndjsonSummary{
		Type:              "summary",
		TotalCount:        jsonSum.TotalCount,
		TotalSavings:      jsonSum.TotalSavings,
		Currency:          jsonSum.Currency,
		CountByActionType: jsonSum.CountByActionType,
		SavingsByAction:   jsonSum.SavingsByAction,
		Pagination:        paginationMeta,
		Scoring:           newScoringJSON(result.Scoring),
	}
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("encoding NDJSON summary: %w", err)
	}

	// Emit individual recommendations
	for _, rec := range result.Recommendations {
		jsonRec := newRecommendationJSON(rec)
		if err := encoder.Encode(jsonRec); err != nil {
			return fmt.Errorf("encoding NDJSON: %w", err)
		}
	}
	return nil
}

// hasStatusAnnotations returns true if any recommendation has a non-empty, non-Active status.
// This is used to conditionally display the Status column in table output.
func hasStatusAnnotations(recs []engine.Recommendation) bool {
	for _, rec := range recs {
		if rec.Status != "" && rec.Status != statusActive {
			return true
		}
	}
	return false
}

// formatActionTypeLabel returns a human-readable label for an action type.
// Uses the proto utilities for consistent label formatting across the codebase.
func formatActionTypeLabel(actionType string) string {
	return proto.ActionTypeLabelFromString(actionType)
}

// isBrokenPipe checks if an error is a broken pipe error (SIGPIPE).
// This occurs when output is piped to commands like `head` that close the pipe early.
// T049: Handle SIGPIPE gracefully for streaming output.
func isBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	// Check for EPIPE (broken pipe) error
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		return errno == syscall.EPIPE
	}
	// Also check error message as fallback
	return strings.Contains(err.Error(), "broken pipe")
}

// mergeDismissedRecommendations loads the local dismissal store and appends
// dismissed/snoozed recommendations to the result with status annotations.
// Active recommendations already in the result are not duplicated.
func mergeDismissedRecommendations(ctx context.Context, result *engine.RecommendationsResult) error {
	log := logging.FromContext(ctx)

	store, err := loadDismissalStore()
	if err != nil {
		return fmt.Errorf("loading dismissal store for merge: %w", err)
	}

	allRecords := store.GetAllRecords()
	if len(allRecords) == 0 {
		return nil
	}

	originalLen := len(result.Recommendations)

	mergeDismissalRecordsIntoResult(allRecords, result)

	mergedCount := len(result.Recommendations) - originalLen

	log.Debug().Ctx(ctx).
		Int("dismissed_merged", mergedCount).
		Int("merged_total", len(result.Recommendations)).
		Msg("merged dismissed recommendations into results")

	return nil
}

// mergeDismissalRecordsIntoResult appends dismissed/snoozed records into the result,
// skipping any that match active recommendations. It uses separate maps for
// ResourceID and RecommendationID deduplication to prevent false matches.
func mergeDismissalRecordsIntoResult(records map[string]*config.DismissalRecord, result *engine.RecommendationsResult) {
	activeResourceIDs := make(map[string]bool, len(result.Recommendations))
	activeRecommendationIDs := make(map[string]bool, len(result.Recommendations))
	for _, rec := range result.Recommendations {
		activeResourceIDs[rec.ResourceID] = true
		activeRecommendationIDs[rec.ResourceID] = true
		if rec.ID != "" {
			activeRecommendationIDs[rec.ID] = true
		}
	}

	for _, record := range records {
		// Skip active (undismissed) records — they are kept only for history
		if record.Status == config.StatusActive {
			continue
		}

		// Skip if this dismissed record's resource matches an active recommendation
		if record.LastKnown != nil && activeResourceIDs[record.LastKnown.ResourceID] {
			continue
		}
		// Check the recommendation ID against active recommendation IDs
		if activeRecommendationIDs[record.RecommendationID] {
			continue
		}

		// Determine display status
		status := engine.RecommendationStatusDismissed
		if record.Status == config.StatusSnoozed {
			status = engine.RecommendationStatusSnoozed
		}

		// Convert LastKnown snapshot to Recommendation
		rec := engine.Recommendation{
			ID:     record.RecommendationID,
			Status: status,
		}

		if record.LastKnown != nil {
			rec.ResourceID = record.LastKnown.ResourceID
			rec.Type = record.LastKnown.Type
			rec.Description = record.LastKnown.Description
			rec.EstimatedSavings = record.LastKnown.EstimatedSavings
			rec.Currency = record.LastKnown.Currency
		} else {
			// Minimal info when no LastKnown snapshot exists
			rec.ResourceID = record.RecommendationID
			rec.Description = fmt.Sprintf("%s recommendation (no details available)", status)
		}

		result.Recommendations = append(result.Recommendations, rec)
	}
}

// JSON output structures for recommendations.
type recommendationsJSONOutput struct {
	Summary         jsonSummary                  `json:"summary"`
	Recommendations []recommendationJSON         `json:"recommendations"`
	TotalSavings    float64                      `json:"total_savings"`
	Currency        string                       `json:"currency"`
	Errors          []engine.RecommendationError `json:"errors,omitempty"`
	Pagination      *pagination.PaginationMeta   `json:"pagination,omitempty"`
	Scoring         *scoringJSON                 `json:"scoring,omitempty"`
}

// jsonSummary represents the summary section in JSON output.
type jsonSummary struct {
	TotalCount        int                `json:"total_count"`
	TotalSavings      float64            `json:"total_savings"`
	Currency          string             `json:"currency"`
	CountByActionType map[string]int     `json:"count_by_action_type"`
	SavingsByAction   map[string]float64 `json:"savings_by_action_type"`
}

// ndjsonSummary represents the summary line in NDJSON output.
type ndjsonSummary struct {
	Type              string                     `json:"type"`
	TotalCount        int                        `json:"total_count"`
	TotalSavings      float64                    `json:"total_savings"`
	Currency          string                     `json:"currency"`
	CountByActionType map[string]int             `json:"count_by_action_type"`
	SavingsByAction   map[string]float64         `json:"savings_by_action_type"`
	Pagination        *pagination.PaginationMeta `json:"pagination,omitempty"`
	Scoring           *scoringJSON               `json:"scoring,omitempty"`
}

type recommendationJSON struct {
	ResourceID       string  `json:"resource_id"`
	ActionType       string  `json:"action_type"`
	Description      string  `json:"description"`
	EstimatedSavings float64 `json:"estimated_savings,omitempty"`
	Currency         string  `json:"currency,omitempty"`
	Status           string  `json:"status,omitempty"`

	ID              string                      `json:"id,omitempty"`
	Category        string                      `json:"category,omitempty"`
	Priority        string                      `json:"priority,omitempty"`
	ConfidenceScore *float64                    `json:"confidence_score,omitempty"`
	Source          string                      `json:"source,omitempty"`
	CreatedAt       *time.Time                  `json:"created_at,omitempty"`
	Metadata        map[string]string           `json:"metadata,omitempty"`
	Reasoning       []string                    `json:"reasoning,omitempty"`
	Impact          *recommendationImpactJSON   `json:"impact,omitempty"`
	Resource        *recommendationResourceJSON `json:"resource,omitempty"`
	Scores          *recommendationScoresJSON   `json:"scores,omitempty"`
}

type recommendationImpactJSON struct {
	ProjectionPeriod     string   `json:"projection_period,omitempty"`
	CurrentCost          float64  `json:"current_cost,omitempty"`
	ProjectedCost        float64  `json:"projected_cost,omitempty"`
	SavingsPercentage    float64  `json:"savings_percentage,omitempty"`
	ImplementationCost   *float64 `json:"implementation_cost,omitempty"`
	MigrationEffortHours *float64 `json:"migration_effort_hours,omitempty"`
}

type recommendationResourceJSON struct {
	Name         string                         `json:"name,omitempty"`
	Provider     string                         `json:"provider,omitempty"`
	ResourceType string                         `json:"resource_type,omitempty"`
	Region       string                         `json:"region,omitempty"`
	SKU          string                         `json:"sku,omitempty"`
	Tags         map[string]string              `json:"tags,omitempty"`
	Utilization  *recommendationUtilizationJSON `json:"utilization,omitempty"`
}

type recommendationUtilizationJSON struct {
	CPUPercent     float64            `json:"cpu_percent,omitempty"`
	MemoryPercent  float64            `json:"memory_percent,omitempty"`
	StoragePercent float64            `json:"storage_percent,omitempty"`
	NetworkInMbps  float64            `json:"network_in_mbps,omitempty"`
	NetworkOutMbps float64            `json:"network_out_mbps,omitempty"`
	CustomMetrics  map[string]float64 `json:"custom_metrics,omitempty"`
}

// newRecommendationJSON maps an engine recommendation to its JSON/NDJSON representation.
func newRecommendationJSON(rec engine.Recommendation) recommendationJSON {
	out := recommendationJSON{
		ResourceID:       rec.ResourceID,
		ActionType:       rec.Type,
		Description:      rec.Description,
		EstimatedSavings: rec.EstimatedSavings,
		Currency:         rec.Currency,
		Status:           string(rec.Status),
		ID:               rec.ID,
		Category:         rec.Category,
		Priority:         rec.Priority,
		ConfidenceScore:  rec.ConfidenceScore,
		Source:           rec.Source,
		CreatedAt:        rec.CreatedAt,
		Metadata:         rec.Metadata,
		Reasoning:        rec.Reasoning,
		Scores:           newScoresJSON(rec.Scores),
	}

	if d := rec.ImpactDetail; d != nil {
		out.Impact = &recommendationImpactJSON{
			ProjectionPeriod:     d.ProjectionPeriod,
			CurrentCost:          d.CurrentCost,
			ProjectedCost:        d.ProjectedCost,
			SavingsPercentage:    d.SavingsPercentage,
			ImplementationCost:   d.ImplementationCost,
			MigrationEffortHours: d.MigrationEffortHours,
		}
	}

	if r := rec.ResourceInfo; r != nil {
		out.Resource = &recommendationResourceJSON{
			Name:         r.Name,
			Provider:     r.Provider,
			ResourceType: r.ResourceType,
			Region:       r.Region,
			SKU:          r.SKU,
			Tags:         r.Tags,
		}
		if u := r.Utilization; u != nil {
			out.Resource.Utilization = &recommendationUtilizationJSON{
				CPUPercent:     u.CPUPercent,
				MemoryPercent:  u.MemoryPercent,
				StoragePercent: u.StoragePercent,
				NetworkInMbps:  u.NetworkInMbps,
				NetworkOutMbps: u.NetworkOutMbps,
				CustomMetrics:  u.CustomMetrics,
			}
		}
	}

	return out
}

// buildJSONSummary constructs the summary structure for JSON/NDJSON output.
func buildJSONSummary(recommendations []engine.Recommendation) jsonSummary {
	countByAction := make(map[string]int)
	savingsByAction := make(map[string]float64)
	totalSavings := 0.0
	currency := defaultCurrency

	for _, rec := range recommendations {
		countByAction[rec.Type]++
		savingsByAction[rec.Type] += rec.EstimatedSavings
		totalSavings += rec.EstimatedSavings
		if rec.Currency != "" {
			currency = rec.Currency
		}
	}

	return jsonSummary{
		TotalCount:        len(recommendations),
		TotalSavings:      totalSavings,
		Currency:          currency,
		CountByActionType: countByAction,
		SavingsByAction:   savingsByAction,
	}
}

// showProgressIndicator displays a spinner with batch progress for queries >500ms.
// This function is designed to run in a goroutine and will only show progress if
// the operation takes longer than 500ms.
func showProgressIndicator(
	ctx context.Context,
	cmd *cobra.Command,
	resources []engine.ResourceDescriptor,
) {
	// Wait before showing progress
	timer := time.NewTimer(progressDelayMS * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		// Operation completed before threshold
		return
	case <-timer.C:
		// Operation exceeded threshold, show progress
	}

	// Only show progress if stderr is a terminal
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}

	// Spinner frames
	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frameIndex := 0

	// Calculate total batches
	totalBatches := len(resources) / progressBatchSize
	if len(resources)%progressBatchSize > 0 {
		totalBatches++
	}

	ticker := time.NewTicker(progressTickerMS * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update spinner frame
			frame := spinnerFrames[frameIndex%len(spinnerFrames)]
			frameIndex++

			// Show spinner with resource count and estimated batches
			msg := fmt.Sprintf("\r%s Fetching recommendations for %d resources (%d batches)...",
				frame, len(resources), totalBatches)
			fmt.Fprint(cmd.ErrOrStderr(), msg)
		}
	}
}
