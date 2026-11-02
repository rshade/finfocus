package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pluginhost"
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
	DryRun           bool                 `json:"dryRun,omitempty"`
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
	cmd.Flags().String(
		"to", "",
		"End date (YYYY-MM-DD or RFC3339); default is the start of the current UTC day, or now when only --from is set",
	)
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
	dir := dailyHistoryDir(ctx, deps)
	target, err := dailyTarget(cmd, deps, dir)
	if err != nil {
		return err
	}
	resources, err := deps.load(ctx, cmd)
	if err != nil {
		return err
	}
	if inputStackMismatch(resources, history.StackContext{Stack: target.Stack}) {
		return toValidationError(ctx, fmt.Errorf(
			"the input resources belong to a different stack than %q; pass --stack for the stack that produced it",
			target.Stack,
		))
	}
	if project := projectOfResources(resources); project != "" && target.Project != "" && project != target.Project {
		return toValidationError(ctx, fmt.Errorf(
			"the input resources belong to project %q, not %q; run from that project or pass --stack org/%s/%s",
			project, target.Project, project, target.Stack,
		))
	}
	target, err = reconcileDailyTarget(dir, target, resources)
	if err != nil {
		return toValidationError(ctx, err)
	}
	priced, err := priceDaily(ctx, cmd, deps, resources, from, to)
	if err != nil {
		return err
	}
	points, report := collectDailyPoints(priced, resources, deps.now())
	report.Stack = target.Stack
	report.Project = target.Project
	report.From = from.UTC().Format(time.RFC3339)
	report.To = to.UTC().Format(time.RFC3339)
	if dryRunRequested(cmd) {
		report.DryRun = true
		warnDailySkips(cmd, report.skippedIDs, report.Errors)
	} else {
		pruned, writeErr := writeDailyPoints(cmd, deps, dir, target, points, report.skippedIDs, report.Errors)
		if writeErr != nil {
			return writeErr
		}
		report.PrunedPoints = pruned
	}
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
	rawFrom, err := cmd.Flags().GetString("from")
	if err != nil {
		return err
	}
	if tf != "" && strings.TrimSpace(rawFrom) == "" {
		return errors.New("--from is required when using --terraform-state")
	}
	return validateActualInputFlags(costActualParams{
		planPath:       plan,
		statePath:      state,
		terraformState: tf,
		fromStr:        from.UTC().Format(time.RFC3339),
	})
}

func dailyTarget(cmd *cobra.Command, deps dailyCollectDeps, dir string) (historyTarget, error) {
	if deps.dir != "" {
		if strings.TrimSpace(deps.stack) == "" {
			return historyTarget{}, ErrStackRequired
		}
		return historyTarget{Project: deps.project, Stack: deps.stack}, nil
	}
	if explicitPulumiInput(cmd) {
		stack, err := requiredStack(cmd)
		if err != nil {
			return historyTarget{}, err
		}
		return resolveHistoryTargetFor(dir, stack, currentProjectName())
	}
	return resolveHistoryCommandTarget(cmd, dir)
}

// explicitPulumiInput reports whether the resources come from a Pulumi file the
// caller named. That input is a Pulumi stack whatever the working directory is,
// so a Terraform directory must not decide where its series is stored.
func explicitPulumiInput(cmd *cobra.Command) bool {
	for _, name := range []string{"pulumi-json", "pulumi-state"} {
		if value, err := cmd.Flags().GetString(name); err == nil && value != "" {
			return true
		}
	}
	return false
}

func dailyHistoryDir(ctx context.Context, deps dailyCollectDeps) string {
	if deps.dir != "" {
		return deps.dir
	}
	return gatedHistoryDir(ctx, deps.cfg)
}

// reconcileDailyTarget names the project of a bare stack when nothing else did,
// so a stack collected once as org/project/stack and later as a bare name lands
// in the same file. The input's own URNs name the project first; failing that,
// one existing daily file for the stack does; several are ambiguous.
func reconcileDailyTarget(
	dir string, target historyTarget, resources []engine.ResourceDescriptor,
) (historyTarget, error) {
	if target.Project != "" {
		return target, nil
	}
	if project := projectOfResources(resources); project != "" {
		target.Project = project
		return target, nil
	}
	entries, err := os.ReadDir(historyDir(dir))
	if err != nil {
		return target, nil //nolint:nilerr // no history directory yet: a fresh file named by the stack alone
	}
	var projects []string
	for _, entry := range entries {
		project, stack, ok := history.DailyProjectFromFileName(entry.Name())
		if ok && !entry.IsDir() && stack == target.Stack {
			projects = append(projects, project)
		}
	}
	switch len(projects) {
	case 0:
		return target, nil
	case 1:
		target.Project = projects[0]
		return target, nil
	default:
		return historyTarget{}, fmt.Errorf(
			"stack %q exists in several projects (%s); run from a project directory or pass --stack org/project/%s",
			target.Stack, strings.Join(projects, ", "), target.Stack,
		)
	}
}

// projectOfResources is the one project the input's Pulumi URNs name, or "".
func projectOfResources(resources []engine.ResourceDescriptor) string {
	project := ""
	for _, resource := range resources {
		urn, _ := resource.Properties[engine.PropertyPulumiURN].(string)
		if urn == "" {
			urn = resource.ID
		}
		named := history.ProjectFromURN(urn)
		switch {
		case named == "":
			continue
		case project == "":
			project = named
		case project != named:
			return ""
		}
	}
	return project
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
	params := costActualParams{planPath: plan, statePath: state, terraformState: tf}
	resources, err := loadActualResources(ctx, cmd, params, audit)
	if err != nil {
		return nil, wrapActualLoadError(ctx, params, err)
	}
	return resources, nil
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
	eng, store, cleanupCache := newEngineWithCache(ctx, cmd, clients, nil, config.GetGlobalConfig())
	defer cleanupCache()
	return priceDailyWithEngine(ctx, eng, clients, store, resources, from, to)
}

// priceDailyWithEngine resolves Terraform types the way cost actual does, so a
// plugin that prices Pulumi tokens can price a Terraform state's resources.
func priceDailyWithEngine(
	ctx context.Context,
	eng *engine.Engine,
	clients []*pluginhost.Client,
	store cache.Cache,
	resources []engine.ResourceDescriptor,
	from, to time.Time,
) (*engine.CostResultWithErrors, error) {
	resources = resolveResourceTypes(ctx, clients, store, resources)
	return eng.GetActualCostWithOptionsAndErrors(ctx, dailyActualRequest(resources, from, to))
}

// dailyActualRequest asks for the plugin's day buckets and nothing else. A
// state estimate would replace a plugin's reported $0 day, which carries no
// observations, and would fill a gap the series must leave empty.
func dailyActualRequest(resources []engine.ResourceDescriptor, from, to time.Time) engine.ActualCostRequest {
	return engine.ActualCostRequest{
		Resources:               resources,
		From:                    from,
		To:                      to,
		RetainDailyObservations: true,
		SkipStateEstimate:       true,
	}
}

func collectDailyPoints(
	priced *engine.CostResultWithErrors, resources []engine.ResourceDescriptor, written time.Time,
) ([]history.DailyPoint, dailyCollectReport) {
	report := dailyCollectReport{Errors: []dailyResourceError{}}
	if priced == nil {
		return nil, report
	}
	failed := map[string]struct{}{}
	invalid := map[string]struct{}{}
	skipped := map[string]struct{}{}
	var points []history.DailyPoint
	for i := range priced.Results {
		result := priced.Results[i]
		if _, bad := invalid[result.ResourceID]; bad {
			continue
		}
		built, err := history.PointsFromObservations(
			result.ResourceID, observedCosts(result.DailyObservations), written,
		)
		if err != nil {
			failed[result.ResourceID] = struct{}{}
			invalid[result.ResourceID] = struct{}{}
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
	report.Errors = append(report.Errors, failUnansweredResources(priced, resources, points, failed, skipped)...)
	points, overflowed := mergeDailyPoints(points)
	report.Errors = append(report.Errors, failOverflowed(overflowed, invalid, failed, skipped)...)
	points = dropFailedPoints(points, failed)
	report.Errors = keepUnresolvedErrors(report.Errors, points)
	markOmittedAsSkipped(resources, priced, points, failed, skipped)
	report.skippedIDs = mapKeys(skipped)
	report.SkippedResources = len(report.skippedIDs)
	report.StoredPoints = len(points)
	report.StoredResources = countResources(points)
	sortDailyErrors(report.Errors)
	return points, report
}

// markOmittedAsSkipped counts a billable resource the engine returned neither
// a result nor an error for, which happens when every plugin declines it. The
// summary would otherwise report a nonempty stack as nothing stored, skipped, or
// failed.
func markOmittedAsSkipped(
	resources []engine.ResourceDescriptor,
	priced *engine.CostResultWithErrors,
	points []history.DailyPoint,
	failed, skipped map[string]struct{},
) {
	reported := make(map[string]struct{}, len(priced.Results)+len(priced.Errors))
	for i := range priced.Results {
		reported[priced.Results[i].ResourceID] = struct{}{}
	}
	for _, detail := range priced.Errors {
		reported[detail.ResourceID] = struct{}{}
	}
	for _, point := range points {
		reported[point.ResourceID] = struct{}{}
	}
	for _, resource := range resources {
		if engine.IsPulumiInternalType(resource.Type) {
			continue
		}
		if _, seen := reported[resource.ID]; seen {
			continue
		}
		if _, bad := failed[resource.ID]; bad {
			continue
		}
		skipped[resource.ID] = struct{}{}
	}
}

// failOverflowed marks resources whose merged daily sum overflowed as failed
// and returns their errors. A resource already rejected keeps its first error.
func failOverflowed(ids []string, invalid, failed, skipped map[string]struct{}) []dailyResourceError {
	var errs []dailyResourceError
	for _, id := range ids {
		if _, bad := invalid[id]; bad {
			continue
		}
		invalid[id] = struct{}{}
		failed[id] = struct{}{}
		delete(skipped, id)
		errs = append(errs, dailyResourceError{
			Resource: id, Message: fmt.Sprintf("resource %s: the daily cost is not a finite amount", id),
		})
	}
	return errs
}

// mergeDailyPoints sums points that share a resource, UTC day, and currency.
// A state export can hold an old resource and its replacement under one URN;
// both are priced, and writing the same key twice would keep only the last.
// Two finite amounts can still sum to infinity, which the store cannot encode
// and which would roll back every other resource's write; those resources are
// returned so the caller can fail them alone.
func mergeDailyPoints(points []history.DailyPoint) ([]history.DailyPoint, []string) {
	type key struct {
		resource string
		day      int64
		currency string
	}
	index := make(map[key]int, len(points))
	merged := make([]history.DailyPoint, 0, len(points))
	overflow := map[string]struct{}{}
	for _, point := range points {
		k := key{point.ResourceID, point.Day.Unix(), point.Currency}
		if at, seen := index[k]; seen {
			merged[at].Amount += point.Amount
			if math.IsInf(merged[at].Amount, 0) {
				overflow[point.ResourceID] = struct{}{}
			}
			continue
		}
		index[k] = len(merged)
		merged = append(merged, point)
	}
	return merged, mapKeys(overflow)
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

// failUnansweredResources turns the engine's per-resource errors into failures.
// An error for a resource that another result already stored is only a failed
// attempt a fallback plugin answered, unless fewer results came back than
// resources carry that ID.
func failUnansweredResources(
	priced *engine.CostResultWithErrors,
	resources []engine.ResourceDescriptor,
	points []history.DailyPoint,
	failed, skipped map[string]struct{},
) []dailyResourceError {
	answered := countResults(priced.Results)
	wanted := countDescriptors(resources)
	var errs []dailyResourceError
	for _, detail := range priced.Errors {
		id := detail.ResourceID
		if resourceStored(points, id) && answered[id] >= wanted[id] {
			continue
		}
		if _, seen := failed[id]; seen {
			continue
		}
		failed[id] = struct{}{}
		delete(skipped, id)
		message := "actual cost failed"
		if detail.Error != nil {
			message = detail.Error.Error()
		}
		errs = append(errs, dailyResourceError{Resource: id, Message: message})
	}
	return errs
}

// countResults and countDescriptors tell a failed attempt that a fallback plugin
// answered (every resource has a result; the error is noise) from a resource
// that failed outright beside another resource sharing its URN, such as a
// replacement. Only the second leaves a sum that understates the day.
func countResults(results []engine.CostResult) map[string]int {
	counts := make(map[string]int, len(results))
	for i := range results {
		counts[results[i].ResourceID]++
	}
	return counts
}

func countDescriptors(resources []engine.ResourceDescriptor) map[string]int {
	counts := make(map[string]int, len(resources))
	for _, resource := range resources {
		counts[resource.ID]++
	}
	return counts
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
	cmd *cobra.Command,
	deps dailyCollectDeps,
	dir string,
	target historyTarget,
	points []history.DailyPoint,
	skipped []string,
	errs []dailyResourceError,
) (int, error) {
	warnDailySkips(cmd, skipped, errs)
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
	stored := "Stored"
	if report.DryRun {
		stored = "Dry run: nothing written or pruned. Would store"
	}
	_, err := fmt.Fprintf(w, "Daily actuals for %s\nWindow: %s to %s\n%s %d resources (%d points).\n"+
		"Skipped %d resources with no daily costs.\n%d resources failed.\n",
		label, report.From, report.To, stored,
		report.StoredResources, report.StoredPoints,
		report.SkippedResources, len(report.Errors))
	if err != nil || report.PrunedPoints == 0 {
		return err
	}
	_, err = fmt.Fprintf(w, "Pruned %d daily points (retention policy).\n", report.PrunedPoints)
	return err
}
