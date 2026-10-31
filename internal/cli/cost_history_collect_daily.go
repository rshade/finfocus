package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

const dailyCompleteDays = 30

type dailyLoader func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error)

type dailyPricer func(
	context.Context, *cobra.Command, []engine.ResourceDescriptor, time.Time, time.Time,
) (*engine.CostResultWithErrors, error)

type dailyCollectDeps struct {
	now     func() time.Time
	load    dailyLoader
	price   dailyPricer
	cfg     *config.Config
	dir     string
	project string
	stack   string
}

type dailyResourceError struct {
	Resource string `json:"resource"`
	Message  string `json:"message"`
}

type dailyCollectReport struct {
	Stack            string               `json:"stack"`
	Project          string               `json:"project,omitempty"`
	From             string               `json:"from"`
	To               string               `json:"to"`
	StoredResources  int                  `json:"storedResources"`
	StoredPoints     int                  `json:"storedPoints"`
	SkippedResources int                  `json:"skippedResources"`
	PrunedPoints     int                  `json:"prunedPoints"`
	Errors           []dailyResourceError `json:"errors"`
	skippedIDs       []string
}

// NewCostHistoryCollectDailyCmd creates `cost history collect-daily`.
func NewCostHistoryCollectDailyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect-daily",
		Short: "Store daily actual costs for a stack",
		Long: `Prices actual cost for the stack and stores one point per resource, UTC day,
and currency. A day the plugin does not send is left untouched. The projected
timeline is not modified.

The default window is the last 30 complete UTC days. --from and --to accept
YYYY-MM-DD or RFC3339, the same formats as cost actual.`,
		Example: `  finfocus cost history collect-daily --stack dev
  finfocus cost history collect-daily --stack dev --from 2026-10-01 --to 2026-10-08
  finfocus cost history collect-daily --stack dev --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDailyCollect(cmd, dailyCollectDeps{})
		},
	}
	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD or RFC3339); default is 30 complete UTC days ago")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD or RFC3339); default excludes the current partial day")
	cmd.Flags().String("pulumi-json", "", "Path to Pulumi preview JSON output")
	cmd.Flags().String("pulumi-state", "", "Path to Pulumi state JSON from 'pulumi stack export'")
	cmd.Flags().String(
		"terraform-state", "",
		"Path to a Terraform state file (mutually exclusive with --pulumi-json/--pulumi-state)",
	)
	cmd.Flags().String("output", "", "Output format: table or json (default table)")
	return cmd
}

func runDailyCollect(cmd *cobra.Command, deps dailyCollectDeps) error {
	format, err := dailyOutputFormat(cmd)
	if err != nil {
		return err
	}
	deps = fillDailyDeps(deps)
	ctx := commandContext(cmd)
	from, to, err := dailyWindow(cmd, deps.now())
	if err != nil {
		return toValidationError(ctx, err)
	}
	if err = validateDailyInputs(cmd, from); err != nil {
		return toValidationError(ctx, err)
	}
	target, err := dailyTarget(ctx, cmd, deps)
	if err != nil {
		return err
	}
	resources, err := deps.load(ctx, cmd)
	if err != nil {
		return err
	}
	priced, err := priceDaily(ctx, cmd, deps, resources, from, to)
	if err != nil {
		return err
	}
	points, report := collectDailyPoints(priced, deps.now())
	report.Stack = target.Stack
	report.Project = target.Project
	report.From = from.UTC().Format(time.RFC3339)
	report.To = to.UTC().Format(time.RFC3339)
	pruned, err := writeDailyPoints(ctx, cmd, deps, target, points, report.skippedIDs, report.Errors)
	if err != nil {
		return err
	}
	report.PrunedPoints = pruned
	if err = renderDailyReport(cmd.OutOrStdout(), format, report); err != nil {
		return err
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("collecting daily actuals: %d resources failed", len(report.Errors))
	}
	return nil
}

func fillDailyDeps(deps dailyCollectDeps) dailyCollectDeps {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.load == nil {
		deps.load = loadDailyResources
	}
	if deps.price == nil {
		deps.price = priceDailyResources
	}
	if deps.cfg == nil {
		deps.cfg = config.GetGlobalConfig()
	}
	return deps
}

func dailyOutputFormat(cmd *cobra.Command) (string, error) {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	format := resolveOutputFormat(cmd, "output", output)
	if format == "" {
		format = outputFormatTable
	}
	if format != outputFormatTable && format != outputFormatJSON {
		return "", unsupportedOutputError(cmd.Context(), format, "table, json")
	}
	return format, nil
}

func dailyWindow(cmd *cobra.Command, now time.Time) (time.Time, time.Time, error) {
	fromStr, err := cmd.Flags().GetString("from")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	toStr, err := cmd.Flags().GetString("to")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	fromStr = strings.TrimSpace(fromStr)
	toStr = strings.TrimSpace(toStr)
	if fromStr == "" && toStr == "" {
		from, to := defaultDailyWindow(now)
		return from, to, nil
	}
	if fromStr == "" {
		return time.Time{}, time.Time{}, errors.New("--from is required when --to is set")
	}
	if toStr == "" {
		toStr = now.UTC().Format(time.RFC3339)
	}
	return ParseTimeRange(fromStr, toStr)
}

func defaultDailyWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -dailyCompleteDays), today
}

func validateDailyInputs(cmd *cobra.Command, from time.Time) error {
	plan, err := cmd.Flags().GetString("pulumi-json")
	if err != nil {
		return err
	}
	state, err := cmd.Flags().GetString("pulumi-state")
	if err != nil {
		return err
	}
	tf, err := cmd.Flags().GetString("terraform-state")
	if err != nil {
		return err
	}
	return validateActualInputFlags(costActualParams{
		planPath:       plan,
		statePath:      state,
		terraformState: tf,
		fromStr:        from.UTC().Format(time.RFC3339),
	})
}

func dailyTarget(ctx context.Context, cmd *cobra.Command, deps dailyCollectDeps) (historyTarget, error) {
	if deps.dir != "" {
		if strings.TrimSpace(deps.stack) == "" {
			return historyTarget{}, ErrStackRequired
		}
		return historyTarget{Project: deps.project, Stack: deps.stack}, nil
	}
	dir := gatedHistoryDir(ctx, deps.cfg)
	return resolveHistoryCommandTarget(cmd, dir)
}

func gatedHistoryDir(ctx context.Context, cfg *config.Config) string {
	projectDir := config.GetResolvedProjectDir()
	dir := resolveHistoryDir(ctx, cfg, projectDir)
	if projectHistoryDirectoryRecheckFails(ctx, cfg, projectDir, dir) {
		dir = filepath.Join(config.ResolveConfigDir(), "history")
	}
	return dir
}

func priceDaily(
	ctx context.Context,
	cmd *cobra.Command,
	deps dailyCollectDeps,
	resources []engine.ResourceDescriptor,
	from, to time.Time,
) (*engine.CostResultWithErrors, error) {
	if len(resources) == 0 {
		return &engine.CostResultWithErrors{}, nil
	}
	priced, err := deps.price(ctx, cmd, resources, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetching actual costs: %w", err)
	}
	if priced == nil {
		return &engine.CostResultWithErrors{}, nil
	}
	return priced, nil
}

func loadDailyResources(ctx context.Context, cmd *cobra.Command) ([]engine.ResourceDescriptor, error) {
	plan, err := cmd.Flags().GetString("pulumi-json")
	if err != nil {
		return nil, err
	}
	state, err := cmd.Flags().GetString("pulumi-state")
	if err != nil {
		return nil, err
	}
	tf, err := cmd.Flags().GetString("terraform-state")
	if err != nil {
		return nil, err
	}
	audit := newAuditContext(ctx, "cost history collect-daily", map[string]string{
		"plan_path": plan, "state_path": state, "terraform_state": tf,
	})
	return loadActualResources(ctx, cmd, costActualParams{
		planPath: plan, statePath: state, terraformState: tf,
	}, audit)
}

func priceDailyResources(
	ctx context.Context,
	cmd *cobra.Command,
	resources []engine.ResourceDescriptor,
	from, to time.Time,
) (*engine.CostResultWithErrors, error) {
	clients, cleanup, err := openPlugins(ctx, "", nil)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	eng, _, cleanupCache := newEngineWithCache(ctx, cmd, clients, nil, config.GetGlobalConfig())
	defer cleanupCache()
	return eng.GetActualCostWithOptionsAndErrors(ctx, engine.ActualCostRequest{
		Resources:               resources,
		From:                    from,
		To:                      to,
		RetainDailyObservations: true,
	})
}

func collectDailyPoints(
	priced *engine.CostResultWithErrors, written time.Time,
) ([]history.DailyPoint, dailyCollectReport) {
	report := dailyCollectReport{Errors: []dailyResourceError{}}
	if priced == nil {
		return nil, report
	}
	failed := map[string]struct{}{}
	skipped := map[string]struct{}{}
	var points []history.DailyPoint
	for i := range priced.Results {
		result := priced.Results[i]
		built, err := history.PointsFromObservations(
			result.ResourceID, observedCosts(result.DailyObservations), written,
		)
		if err != nil {
			failed[result.ResourceID] = struct{}{}
			report.Errors = append(report.Errors, dailyResourceError{Resource: result.ResourceID, Message: err.Error()})
			delete(skipped, result.ResourceID)
			continue
		}
		if len(built) == 0 {
			if result.Error != nil {
				failed[result.ResourceID] = struct{}{}
				report.Errors = append(report.Errors, dailyResourceError{
					Resource: result.ResourceID, Message: result.Error.Message,
				})
				continue
			}
			skipped[result.ResourceID] = struct{}{}
			continue
		}
		delete(skipped, result.ResourceID)
		delete(failed, result.ResourceID)
		points = append(points, built...)
	}
	for _, detail := range priced.Errors {
		if resourceStored(points, detail.ResourceID) {
			continue
		}
		if _, seen := failed[detail.ResourceID]; seen {
			continue
		}
		failed[detail.ResourceID] = struct{}{}
		delete(skipped, detail.ResourceID)
		message := "actual cost failed"
		if detail.Error != nil {
			message = detail.Error.Error()
		}
		report.Errors = append(report.Errors, dailyResourceError{Resource: detail.ResourceID, Message: message})
	}
	points = dropFailedPoints(points, failed)
	report.Errors = keepUnresolvedErrors(report.Errors, points)
	report.skippedIDs = mapKeys(skipped)
	report.SkippedResources = len(report.skippedIDs)
	report.StoredPoints = len(points)
	report.StoredResources = countResources(points)
	sortDailyErrors(report.Errors)
	return points, report
}

func observedCosts(points []engine.DailyObservation) []history.ObservedCost {
	if len(points) == 0 {
		return nil
	}
	out := make([]history.ObservedCost, len(points))
	for i, point := range points {
		out[i] = history.ObservedCost{At: point.Timestamp, Amount: point.Cost, Currency: point.Currency}
	}
	return out
}

func resourceStored(points []history.DailyPoint, id string) bool {
	for _, point := range points {
		if point.ResourceID == id {
			return true
		}
	}
	return false
}

func keepUnresolvedErrors(errs []dailyResourceError, points []history.DailyPoint) []dailyResourceError {
	kept := errs[:0]
	for _, item := range errs {
		if resourceStored(points, item.Resource) {
			continue
		}
		kept = append(kept, item)
	}
	if kept == nil {
		return []dailyResourceError{}
	}
	return kept
}

func dropFailedPoints(points []history.DailyPoint, failed map[string]struct{}) []history.DailyPoint {
	kept := points[:0]
	for _, point := range points {
		if _, bad := failed[point.ResourceID]; bad {
			continue
		}
		kept = append(kept, point)
	}
	return kept
}

func countResources(points []history.DailyPoint) int {
	seen := map[string]struct{}{}
	for _, point := range points {
		seen[point.ResourceID] = struct{}{}
	}
	return len(seen)
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func sortDailyErrors(errs []dailyResourceError) {
	slices.SortFunc(errs, func(a, b dailyResourceError) int {
		if a.Resource != b.Resource {
			return strings.Compare(a.Resource, b.Resource)
		}
		return strings.Compare(a.Message, b.Message)
	})
}

func writeDailyPoints(
	ctx context.Context,
	cmd *cobra.Command,
	deps dailyCollectDeps,
	target historyTarget,
	points []history.DailyPoint,
	skipped []string,
	errs []dailyResourceError,
) (int, error) {
	warnDailySkips(cmd, skipped, errs)
	dir := deps.dir
	if dir == "" {
		dir = gatedHistoryDir(ctx, deps.cfg)
	}
	name, err := history.DailyFileNameFor(target.Project, target.Stack)
	if err != nil {
		return 0, err
	}
	db, err := history.OpenDailyDB(joinDailyPath(dir, name), target.Project, target.Stack)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()
	if err = db.Replace(points); err != nil {
		return 0, err
	}
	return pruneDailyAge(db, deps.cfg, deps.now(), failedDailyResources(errs))
}

func warnDailySkips(cmd *cobra.Command, skipped []string, errs []dailyResourceError) {
	for _, id := range skipped {
		cmd.PrintErrf("Warning: %s has no daily costs; stored nothing\n", id)
	}
	for _, item := range errs {
		cmd.PrintErrf("Warning: %s\n", item.Message)
	}
}

func joinDailyPath(dir, name string) string {
	return filepath.Join(dir, name)
}

func failedDailyResources(errs []dailyResourceError) []string {
	ids := make([]string, 0, len(errs))
	for _, item := range errs {
		ids = append(ids, item.Resource)
	}
	return ids
}

func pruneDailyAge(db *history.DailyDB, cfg *config.Config, now time.Time, keep []string) (int, error) {
	cutoff, ok := dailyAgeCutoff(cfg, now)
	if !ok {
		return 0, nil
	}
	return db.PruneBefore(cutoff, keep)
}

func dailyAgeCutoff(cfg *config.Config, now time.Time) (time.Time, bool) {
	if cfg == nil || !cfg.Cost.History.Retention.AutoPrune || cfg.Cost.History.Retention.MaxAgeDays <= 0 {
		return time.Time{}, false
	}
	today := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -cfg.Cost.History.Retention.MaxAgeDays), true
}

func renderDailyReport(w io.Writer, format string, report dailyCollectReport) error {
	if format == outputFormatJSON {
		body, err := json.Marshal(report)
		if err != nil {
			return fmt.Errorf("encoding daily actuals summary: %w", err)
		}
		_, err = fmt.Fprintf(w, "%s\n", body)
		return err
	}
	label := report.Stack
	if report.Project != "" {
		label = report.Project + "/" + report.Stack
	}
	_, err := fmt.Fprintf(w, "Daily actuals for %s\nWindow: %s to %s\nStored %d resources (%d points).\n"+
		"Skipped %d resources with no daily costs.\n%d resources failed.\n",
		label, report.From, report.To,
		report.StoredResources, report.StoredPoints,
		report.SkippedResources, len(report.Errors))
	if err != nil || report.PrunedPoints == 0 {
		return err
	}
	_, err = fmt.Fprintf(w, "Pruned %d daily points (retention policy).\n", report.PrunedPoints)
	return err
}
