package cli

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/resourcetype"
)

// recordTerraformProjection stores a successfully priced projection in the
// Terraform project's timeline. History failures never fail the cost command.
func recordTerraformProjection(
	ctx context.Context,
	cmd *cobra.Command,
	results []engine.CostResult,
) {
	for _, name := range []string{flagPulumiJSON, flagPulumiState, flagTerraformState} {
		if cmd.Flag(name) != nil {
			value, err := cmd.Flags().GetString(name)
			if err == nil && value != "" {
				return
			}
		}
	}
	cfg := config.GetGlobalConfig()
	if cfg == nil {
		cfg = config.New()
	}
	enabled := cfg.Cost.History.IsEnabled()
	if value := os.Getenv(config.HistoryEnvEnabled); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			enabled = parsed
		}
	}
	if !enabled {
		return
	}
	source, err := DetectProjectSource(".")
	if err != nil || source.Kind != ProjectKindTerraform {
		return
	}
	log := logging.FromContext(ctx)
	if err = writeTerraformProjection(ctx, cmd, cfg, results); err != nil {
		log.Warn().
			Ctx(ctx).
			Err(err).
			Str("component", "history").
			Str("operation", "record_projection").
			Msg("could not record Terraform projection")
	}
}

func writeTerraformProjection(
	ctx context.Context,
	cmd *cobra.Command,
	cfg *config.Config,
	results []engine.CostResult,
) error {
	dir := resolveHistoryDir(ctx, cfg, config.GetResolvedProjectDir())
	if projectHistoryDirectoryRecheckFails(ctx, cfg, config.GetResolvedProjectDir(), dir) {
		dir = filepath.Join(config.ResolveConfigDir(), "history")
	}
	target, err := resolveHistoryCommandTarget(cmd, dir)
	if err != nil {
		return err
	}
	db, err := history.OpenCostDBFor(target.Path, target.Project, target.Stack)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err = db.SetProjectLabel(target.Label); err != nil {
		return err
	}
	stats, err := db.Stats()
	if err != nil {
		return err
	}
	snapshots, err := terraformProjectionSnapshots(results, stats.LastVersion)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		annotation := history.CostAnnotation{
			Version: snapshot.Version,
			Kind:    "projection",
			Message: "Terraform projected cost",
		}
		if err = db.Put(snapshot, annotation); err != nil {
			return err
		}
	}
	return nil
}

func terraformProjectionSnapshots(
	results []engine.CostResult,
	lastVersion uint64,
) ([]history.CostSnapshot, error) {
	version, err := strconv.Atoi(strconv.FormatUint(lastVersion, 10))
	if err != nil {
		return nil, fmt.Errorf("reading projection version: %w", err)
	}
	if len(results) >= math.MaxInt-version {
		return nil, errors.New("projection history version limit reached")
	}
	version++
	snapshots := map[string]*history.CostSnapshot{}
	currencies := []string{}
	for _, result := range results {
		if result.Error != nil {
			continue
		}
		currency := result.Currency
		if currency == "" {
			currency = defaultCurrency
		}
		snapshot := snapshots[currency]
		if snapshot == nil {
			snapshot = &history.CostSnapshot{
				Timestamp:  time.Now().UTC(),
				Version:    version,
				Currency:   currency,
				ByProvider: map[string]float64{},
				ByType:     map[string]float64{},
				Resources:  []history.CostResource{},
			}
			version++
			snapshots[currency] = snapshot
			currencies = append(currencies, currency)
		}
		provider := resourcetype.ExtractProvider(result.ResourceType)
		snapshot.TotalMonthly += result.Monthly
		snapshot.ByProvider[provider] += result.Monthly
		snapshot.ByType[result.ResourceType] += result.Monthly
		snapshot.ResourceCount++
		snapshot.Resources = append(
			snapshot.Resources,
			history.CostResource{
				URN:         result.ResourceID,
				Type:        result.ResourceType,
				Provider:    provider,
				MonthlyCost: result.Monthly,
			},
		)
	}
	if len(results) == 0 {
		currencies = append(currencies, defaultCurrency)
		snapshots[defaultCurrency] = &history.CostSnapshot{
			Timestamp:  time.Now().UTC(),
			Version:    version,
			Currency:   defaultCurrency,
			ByProvider: map[string]float64{},
			ByType:     map[string]float64{},
			Resources:  []history.CostResource{},
		}
	}
	ordered := make([]history.CostSnapshot, 0, len(currencies))
	for _, currency := range currencies {
		ordered = append(ordered, *snapshots[currency])
	}
	return ordered, nil
}

// convertEngineStateToHistoryState converts engine.StateResource to
// history.StateResource for recording to the history store.
func convertEngineStateToHistoryState(resources []engine.StateResource) []history.StateResource {
	result := make([]history.StateResource, 0, len(resources))
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		result = append(result, history.StateResource{
			URN:      r.URN,
			CloudID:  r.ID,
			Type:     r.Type,
			Provider: resourcetype.ExtractProvider(r.Type),
			Tags:     history.ExtractTagsFromProperties(r.Properties),
		})
	}
	return result
}

// recordHistorySnapshot is a fire-and-forget helper that records state resources
// to the history store. Errors are logged at WARN level but never returned.
func recordHistorySnapshot(
	ctx context.Context,
	store history.Store,
	resources []engine.StateResource,
) {
	if store == nil || !store.IsEnabled() {
		return
	}
	stackCtx, ok := detectHistoryStackContext(ctx)
	if !ok {
		return
	}
	log := logging.FromContext(ctx)
	writer := history.NewWriter(store, *log)
	stateResources := convertEngineStateToHistoryState(resources)
	writer.RecordStateSnapshot(stackCtx, stateResources)
}

// recordHistoryPlanLineage is a fire-and-forget helper that records plan lineage
// (replace/delete cloud IDs) to the history store. Only steps with non-empty
// cloud IDs are recorded.
func recordHistoryPlanLineage(
	ctx context.Context,
	store history.Store,
	planSteps []engine.PlanStep,
) {
	if store == nil || !store.IsEnabled() {
		return
	}
	stackCtx, ok := detectHistoryStackContext(ctx)
	if !ok {
		return
	}
	log := logging.FromContext(ctx)
	writer := history.NewWriter(store, *log)
	historySteps := convertEnginePlanStepsToHistoryPlanSteps(planSteps)
	if len(historySteps) > 0 {
		writer.RecordPlanLineage(stackCtx, historySteps)
	}
}

// convertEnginePlanStepsToHistoryPlanSteps converts engine.PlanStep to
// history.PlanStep for recording to the history store. Only steps with
// non-empty cloud IDs are included.
func convertEnginePlanStepsToHistoryPlanSteps(steps []engine.PlanStep) []history.PlanStep {
	result := make([]history.PlanStep, 0, len(steps))
	for _, s := range steps {
		if s.OldCloudID == "" && s.NewCloudID == "" {
			continue
		}
		result = append(result, history.PlanStep{
			Op:         s.Op,
			URN:        s.URN,
			Type:       s.Type,
			Provider:   resourcetype.ExtractProvider(s.Type),
			OldCloudID: s.OldCloudID,
			NewCloudID: s.NewCloudID,
		})
	}
	return result
}

// detectHistoryStackContext attempts to detect the Pulumi project and stack
// for history scoping. Returns false if either project name or stack cannot
// be fully resolved, so callers can skip history operations rather than
// recording under a meaningless zero-value hash.
func detectHistoryStackContext(ctx context.Context) (history.StackContext, bool) {
	if selected, ok := ctx.Value(historyStackContextKey{}).(history.StackContext); ok {
		return selected, true
	}
	log := logging.FromContext(ctx)

	projectDir, err := pulumidetect.FindProject(".")
	if err != nil {
		log.Debug().Ctx(ctx).
			Str("component", "history").
			Msg("no Pulumi project detected, using empty stack context for history")
		return history.StackContext{}, false
	}

	projectName, nameErr := pulumidetect.GetProjectName(projectDir)
	if nameErr != nil {
		log.Debug().Ctx(ctx).Err(nameErr).
			Str("component", "history").
			Msg("could not read Pulumi project name, using empty stack context")
		return history.StackContext{}, false
	}

	stack, stackErr := pulumidetect.GetCurrentStack(ctx, projectDir)
	if stackErr != nil || stack == "" {
		log.Debug().Ctx(ctx).Err(stackErr).
			Str("component", "history").
			Msg("could not detect current stack, using empty stack context")
		return history.StackContext{}, false
	}

	return history.StackContext{
		Project: projectName,
		Stack:   stack,
	}, true
}

// initHistoryFromConfig initializes a history store using values from the
// provided configuration and environment variables.
//
// Resolution precedence for each setting:
//   - enabled: env var FINFOCUS_HISTORY_ENABLED > config > default (true)
//   - retention_days: env var FINFOCUS_HISTORY_RETENTION_DAYS > config > default (90)
//   - directory: FINFOCUS_HISTORY_DIR > cost.history.directory > <ResolveConfigDir()>/history
//
// Returns nil when history is disabled or initialization fails (logged at WARN).
// projectDir is the project-tier directory that built cfg; it decides whether
// cost.history.directory came from an untrusted project overlay.
// The caller should defer the returned cleanup function.
func initHistoryFromConfig(
	ctx context.Context,
	cfg *config.Config,
	projectDir string,
) (history.Store, func()) {
	store, cleanup, _ := initHistoryFromConfigWithError(ctx, cfg, projectDir)
	return store, cleanup
}

// initHistoryFromConfigWithError preserves the initialization cause for required imports.
func initHistoryFromConfigWithError(
	ctx context.Context,
	cfg *config.Config,
	projectDir string,
) (history.Store, func(), error) {
	log := logging.FromContext(ctx)
	noopCleanup := func() {}

	retentionDays := resolvedHistoryRetention(ctx, cfg)
	// Use IsEnabled() which handles nil (omitted) vs explicit false correctly.
	enabled := cfg.Cost.History.IsEnabled()

	// Override with env vars
	if envEnabled := os.Getenv(config.HistoryEnvEnabled); envEnabled != "" {
		if parsed, err := strconv.ParseBool(envEnabled); err == nil {
			enabled = parsed
		} else {
			log.Warn().Ctx(ctx).
				Str("component", "history").
				Str("env_var", config.HistoryEnvEnabled).
				Str("value", envEnabled).
				Msg("invalid history enabled env var, ignoring")
		}
	}

	if !enabled {
		return nil, noopCleanup, nil
	}

	historyDirectory := resolveHistoryDir(ctx, cfg, projectDir)
	if projectHistoryDirectoryRecheckFails(ctx, cfg, projectDir, historyDirectory) {
		historyDirectory = filepath.Join(config.ResolveConfigDir(), "history")
	}

	store, err := history.NewBoltStore(ctx, historyDirectory, true, retentionDays)
	if err != nil {
		if errors.Is(err, history.ErrHistoryLocked) {
			log.Warn().Ctx(ctx).
				Str("component", "history").
				Msg("history database locked, proceeding without history")
		} else {
			log.Warn().Ctx(ctx).Err(err).
				Str("component", "history").
				Msg("history initialization failed, proceeding without history")
		}
		return nil, noopCleanup, err
	}

	log.Debug().Ctx(ctx).
		Str("component", "history").
		Bool("enabled", true).
		Int("retention_days", retentionDays).
		Str("history_dir", historyDirectory).
		Msg("history store initialized")

	cleanup := func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Warn().Ctx(ctx).Err(closeErr).
				Str("component", "history").
				Msg("failed to close history store")
		}
	}

	return store, cleanup, nil
}

// resolveHistoryDir determines the history directory using the resolution chain:
// FINFOCUS_HISTORY_DIR, then cost.history.directory, then <ResolveConfigDir()>/history.
// A project-tier cost.history.directory that is absolute, contains "..", or
// resolves outside the project is ignored, because a cloned repository
// controls that file.
func resolveHistoryDir(ctx context.Context, cfg *config.Config, projectDir string) string {
	dir, usedFallback := historyDirectory(ctx, cfg, projectDir)
	if usedFallback {
		logEvent := logging.FromContext(ctx).Warn().Ctx(ctx).
			Str("component", "history").
			Str("fallback_path", dir)
		if _, homeErr := os.UserHomeDir(); homeErr != nil {
			logEvent = logEvent.Err(homeErr)
		}
		logEvent.Msg("failed to determine home directory, using absolute fallback history path")
	}
	return dir
}

// historyDirectory is the shared history-directory resolver. It returns the
// directory and whether it is the working-directory fallback.
func historyDirectory(ctx context.Context, cfg *config.Config, projectDir string) (string, bool) {
	if dir := os.Getenv(config.HistoryEnvDir); dir != "" {
		return dir, false
	}
	if cfg != nil && cfg.Cost.History.Directory != "" {
		dir := cfg.Cost.History.Directory
		if !historyDirectoryIsProjectTier(cfg, projectDir) ||
			!projectHistoryDirectoryRejected(ctx, projectDir) {
			return dir, false
		}
	}
	dir := filepath.Join(config.ResolveConfigDir(), "history")
	if !config.UsesWorkingDirFallback() {
		return dir, false
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir, true
}

// historyDirectoryIsProjectTier reports whether the configured directory is the
// project overlay's value. A config built without the overlay carries a trusted
// global value that an unsafe overlay must not discard. The same path in both
// tiers counts as project-sourced.
func historyDirectoryIsProjectTier(cfg *config.Config, projectDir string) bool {
	project := config.ProjectHistoryDirectory(projectDir)
	return project != "" && filepath.Clean(project) == filepath.Clean(cfg.Cost.History.Directory)
}

// projectHistoryDirectoryRejected judges the project overlay's own
// cost.history.directory, never a match against the merged value, so it holds
// wherever the config was built. A merged value cannot name its tier, but an
// overlay that sets the key always supplies the merged value.
func projectHistoryDirectoryRejected(ctx context.Context, projectDir string) bool {
	dir := config.ProjectHistoryDirectory(projectDir)
	if dir == "" ||
		(projectHistoryDirectorySafe(projectDir, dir) && historyDatabaseFilesSafe(dir)) {
		return false
	}
	logging.FromContext(ctx).Warn().Ctx(ctx).
		Str("component", "history").
		Str("key", "cost.history.directory").
		Msg("ignoring history directory from project config that leaves the project or holds a non-regular database file")
	return true
}

// projectHistoryDirectoryRecheckFails creates the project-tier directory and
// re-runs the containment check on it, so a symlink planted in a parent that did
// not exist during validation is caught before Bolt opens a file there. It is a
// narrowing, not a lock: the directory can still be swapped between this check
// and the open.
func projectHistoryDirectoryRecheckFails(
	ctx context.Context,
	cfg *config.Config,
	projectDir, resolved string,
) bool {
	if os.Getenv(config.HistoryEnvDir) != "" || cfg == nil || cfg.Cost.History.Directory == "" {
		return false
	}
	dir := config.ProjectHistoryDirectory(projectDir)
	if dir == "" || resolved != dir {
		return false
	}
	if err := os.MkdirAll(dir, 0o750); err == nil &&
		projectHistoryDirectorySafe(projectDir, dir) && historyDatabaseFilesSafe(dir) {
		return false
	}
	logging.FromContext(ctx).Warn().Ctx(ctx).
		Str("component", "history").
		Str("key", "cost.history.directory").
		Msg("ignoring history directory from project config that leaves the project after creation")
	return true
}

// historyDatabaseFilesSafe reports whether every database file the history code
// opens in dir (history.db, the *.history.db cost files, and their compaction
// temp files) is absent or a regular file. A symlink there would make Bolt open or create its target
// outside the project. Lstat does not follow the link; the check is made
// before the open, so a swap in between is not covered.
func historyDatabaseFilesSafe(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Is(err, os.ErrNotExist)
	}
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, "history.db") && !strings.Contains(lower, ".history.db") {
			continue
		}
		info, lerr := os.Lstat(filepath.Join(dir, name))
		if lerr != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// projectHistoryDirectorySafe reports whether a project-tier relative path
// stays inside the project root (the parent of projectDir). The path is judged
// where it will be opened, relative to the working directory, with symlinks
// resolved on its deepest existing ancestor. Any error fails closed.
func projectHistoryDirectorySafe(projectDir, dir string) bool {
	if filepath.IsAbs(dir) || filepath.VolumeName(dir) != "" {
		return false
	}
	if runtime.GOOS == goosWindows && (strings.HasPrefix(dir, "/") || strings.HasPrefix(dir, `\`)) {
		return false
	}
	if slices.Contains(strings.Split(filepath.ToSlash(dir), "/"), "..") {
		return false
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(projectDir))
	if err != nil {
		return false
	}
	target, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	target, ok := evalDeepestExisting(target)
	if !ok {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func evalDeepestExisting(path string) (string, bool) {
	var tail []string
	for cur := path; ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(append([]string{resolved}, tail...)...), true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
}
