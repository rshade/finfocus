package history

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	exportAttempts   = 3
	defaultParallel  = 4
	exportRetryDelay = 20 * time.Millisecond
)

// Exporter reads Pulumi history and one checkpoint export for one stack.
type Exporter interface {
	History(ctx context.Context) ([]byte, error)
	Export(ctx context.Context, version int) ([]byte, error)
}

// Pricer runs the projected-cost plugin pipeline for one checkpoint.
type Pricer func(ctx context.Context, resources []PriceResource) ([]PriceQuote, error)

// CollectOptions controls one collect run.
type CollectOptions struct {
	From        time.Time
	Versions    int
	Parallel    int
	SkipDestroy bool
	Confirm     func(warning string) bool
	Pricer      Pricer
	OpenPricer  func(ctx context.Context) (Pricer, error)
	OnPlan      func(CollectPlan)
	Progress    func(CollectProgress)
}

// CollectPlan is the work selected before exports start.
type CollectPlan struct {
	TotalHistory int
	LastVersion  uint64
	CollectedAt  time.Time
	New          int
	FirstNew     int
	LastNew      int
	Reset        bool
	Fresh        bool
}

// CollectProgress is one finished checkpoint, including skipped exports.
type CollectProgress struct {
	Done    int
	Total   int
	Version int
	When    time.Time
}

// CollectResult counts stored and skipped checkpoints.
type CollectResult struct {
	Stored  int
	Skipped []SkippedVersion
	Plan    CollectPlan
}

// SkippedVersion is a checkpoint dropped after export retries failed.
type SkippedVersion struct {
	Version int
	Err     error
}

// Collect prices new checkpoints and stores them. Export failures are skipped.
// A missing plugin or an encrypted pricing property stops the run. Earlier
// successful writes stay in the database.
func Collect(
	ctx context.Context,
	db *CostDB,
	exp Exporter,
	opt CollectOptions,
) (CollectResult, error) {
	prepared, err := prepareCollect(ctx, db, exp, opt)
	if err != nil || len(prepared.selected) == 0 {
		return prepared.result, err
	}
	pricer, err := resolvePricer(ctx, prepared.selected, opt)
	if err != nil {
		return prepared.result, err
	}
	return runCollect(ctx, db, exp, pricer, opt, prepared)
}

type preparedCollect struct {
	selected []StackUpdate
	result   CollectResult
}

func prepareCollect(ctx context.Context, db *CostDB, exp Exporter, opt CollectOptions) (preparedCollect, error) {
	raw, err := exp.History(ctx)
	if err != nil {
		return preparedCollect{}, fmt.Errorf("fetching stack history: %w", err)
	}
	updates, err := ParseStackHistory(raw)
	if err != nil {
		return preparedCollect{}, err
	}
	stats, err := db.Stats()
	if err != nil {
		return preparedCollect{}, err
	}
	reset, err := maybeReset(db, updates, stats.LastVersion, opt.Confirm)
	if err != nil {
		return preparedCollect{}, err
	}
	if reset {
		stats.LastVersion = 0
	}
	selected := SelectUpdates(updates, SelectOptions{
		From:        opt.From,
		LastVersion: stats.LastVersion,
		Limit:       opt.Versions,
		SkipDestroy: opt.SkipDestroy,
	})
	plan := CollectPlan{
		TotalHistory: len(updates),
		LastVersion:  stats.LastVersion,
		CollectedAt:  stats.CollectedAt,
		New:          len(selected),
		Reset:        reset,
		Fresh:        stats.Snapshots == 0 && !reset,
	}
	if len(selected) > 0 {
		plan.FirstNew = selected[0].Version
		plan.LastNew = selected[len(selected)-1].Version
	}
	if opt.OnPlan != nil {
		opt.OnPlan(plan)
	}
	return preparedCollect{selected: selected, result: CollectResult{Plan: plan}}, nil
}

func maybeReset(db *CostDB, updates []StackUpdate, last uint64, confirm func(string) bool) (bool, error) {
	maxVersion := MaxVersion(updates)
	if !VersionReset(maxVersion, last) {
		return false, nil
	}
	warning := VersionResetWarning(maxVersion, last)
	if confirm == nil || !confirm(warning) {
		return false, fmt.Errorf("%w: v%d after v%d", ErrVersionResetDeclined, maxVersion, last)
	}
	if err := db.Reset(); err != nil {
		return false, err
	}
	return true, nil
}

func resolvePricer(ctx context.Context, selected []StackUpdate, opt CollectOptions) (Pricer, error) {
	if opt.Pricer != nil || !needsPrice(selected) || opt.OpenPricer == nil {
		return opt.Pricer, nil
	}
	return opt.OpenPricer(ctx)
}

func needsPrice(updates []StackUpdate) bool {
	for _, update := range updates {
		if update.Kind != historyKindDestroy {
			return true
		}
	}
	return false
}

type collectJob struct {
	db       *CostDB
	exp      Exporter
	pricer   Pricer
	selected []StackUpdate
	progress func(CollectProgress)
}

func runCollect(
	ctx context.Context,
	db *CostDB,
	exp Exporter,
	pricer Pricer,
	opt CollectOptions,
	prepared preparedCollect,
) (CollectResult, error) {
	parallel := opt.Parallel
	if parallel < 1 {
		parallel = defaultParallel
	}
	job := collectJob{db: db, exp: exp, pricer: pricer, selected: prepared.selected, progress: opt.Progress}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(parallel)
	var mu sync.Mutex
	done := 0
	result := prepared.result
	for _, update := range prepared.selected {
		group.Go(func() error {
			return job.finish(groupCtx, update, &mu, &done, &result)
		})
	}
	err := group.Wait()
	return result, err
}

func (j collectJob) finish(
	ctx context.Context,
	update StackUpdate,
	mu *sync.Mutex,
	done *int,
	result *CollectResult,
) error {
	err := j.one(ctx, update)
	mu.Lock()
	defer mu.Unlock()
	*done++
	if j.progress != nil {
		j.progress(CollectProgress{Done: *done, Total: len(j.selected), Version: update.Version, When: update.Start})
	}
	if errors.Is(err, ErrSkipVersion) {
		result.Skipped = append(result.Skipped, SkippedVersion{Version: update.Version, Err: err})
		return nil
	}
	if err != nil {
		return err
	}
	result.Stored++
	return nil
}

func (j collectJob) one(ctx context.Context, update StackUpdate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if update.Kind == historyKindDestroy {
		return j.db.Put(ZeroSnapshot(update.Start, update.Version), AnnotationFrom(update))
	}
	data, err := exportVersion(ctx, j.exp, update.Version)
	if err != nil {
		return err
	}
	resources, err := resourcesFromExport(data)
	if err != nil {
		if errors.Is(err, ErrSkipVersion) {
			return fmt.Errorf("version %d: %w", update.Version, err)
		}
		return err
	}
	if name := EncryptedProperty(resources); name != "" {
		return EncryptedError(name)
	}
	if len(resources) == 0 {
		return j.db.Put(ZeroSnapshot(update.Start, update.Version), AnnotationFrom(update))
	}
	if j.pricer == nil {
		return errors.New("pricer is required")
	}
	quotes, err := j.pricer(ctx, resources)
	if err != nil {
		return fmt.Errorf("pricing version %d: %w", update.Version, err)
	}
	snapshot, err := SnapshotFromResults(update.Start, update.Version, resources, quotes)
	if err != nil {
		return err
	}
	return j.db.Put(snapshot, AnnotationFrom(update))
}

func resourcesFromExport(data []byte) ([]PriceResource, error) {
	var file struct {
		Deployment struct {
			Resources []struct {
				URN     string         `json:"urn"`
				Type    string         `json:"type"`
				Custom  bool           `json:"custom"`
				Inputs  map[string]any `json:"inputs"`
				Outputs map[string]any `json:"outputs"`
			} `json:"resources"`
		} `json:"deployment"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSkipVersion, err)
	}
	resources := make([]PriceResource, 0, len(file.Deployment.Resources))
	for _, resource := range file.Deployment.Resources {
		if !resource.Custom {
			continue
		}
		resources = append(resources, PriceResource{
			ID:         resource.URN,
			Type:       resource.Type,
			Provider:   providerOf(resource.Type),
			Properties: mergeProperties(resource.Outputs, resource.Inputs),
		})
	}
	return resources, nil
}

func providerOf(resourceType string) string {
	provider, _, _ := strings.Cut(resourceType, ":")
	if provider == "" {
		return unknownProvider
	}
	return provider
}

func mergeProperties(base, overlay map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(overlay))
	maps.Copy(merged, base)
	maps.Copy(merged, overlay)
	return merged
}

func exportVersion(ctx context.Context, exp Exporter, version int) ([]byte, error) {
	var last error
	for attempt := range exportAttempts {
		if err := waitRetry(ctx, attempt); err != nil {
			return nil, err
		}
		data, err := exp.Export(ctx, version)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		last = err
	}
	return nil, fmt.Errorf("%w: version %d: %w", ErrSkipVersion, version, last)
}

func waitRetry(ctx context.Context, attempt int) error {
	if attempt == 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(attempt) * exportRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
