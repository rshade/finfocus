package cloudhistory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

const maxConcurrentSnapshots = 5

const historyLockedReason = "history_locked"

// Deps supplies persistence and injectable network, clock, and logging boundaries.
type Deps struct {
	Store      history.Store
	Cache      history.CloudCache
	StoreError error
	NewClient  func(esc.Session) pulumicloud.Client
	Detect     func(context.Context, esc.Session, pulumicloud.StackRef) pulumicloud.Availability
	Now        func() time.Time
	Logger     zerolog.Logger
}

// Request selects the stack, import mode, and bounded observation period.
type Request struct {
	Mode          config.PulumiCloudMode
	Session       esc.Session
	Stack         pulumicloud.StackRef
	StackContext  history.StackContext
	From          time.Time
	To            time.Time
	RetentionDays int
}

// Import lists state-changing updates and persists identity-only observations.
func Import(ctx context.Context, deps Deps, req Request) (Result, error) {
	var result Result
	if req.Mode == config.Off {
		return result, nil
	}
	if deps.Store == nil || !deps.Store.IsEnabled() {
		reason := "history_disabled"
		if deps.StoreError != nil {
			reason = "history_unavailable"
		}
		if errors.Is(deps.StoreError, history.ErrHistoryLocked) {
			reason = historyLockedReason
		}
		return unavailable(deps, req, result, reason)
	}
	if deps.Cache == nil {
		return unavailable(deps, req, result, "history_unavailable")
	}
	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}
	cutoff := now.AddDate(0, 0, -req.RetentionDays)
	if req.From.Before(cutoff) {
		req.From = cutoff
		result.Clamped = true
	}
	if req.From.After(now) || req.To.Before(req.From) {
		return result, nil
	}
	detect := deps.Detect
	if detect == nil {
		detect = pulumicloud.Detect
	}
	available := detect(ctx, req.Session, req.Stack)
	if !available.Available {
		return unavailable(deps, req, result, string(available.Reason))
	}
	factory := deps.NewClient
	if factory == nil {
		factory = func(s esc.Session) pulumicloud.Client { return pulumicloud.NewClient(s, nil) }
	}
	client := factory(req.Session)
	list, err := loadUpdateList(ctx, deps.Cache, client, req, now)
	if err != nil {
		return unavailable(deps, req, result, serviceReason(err))
	}
	result, err = importUpdates(ctx, deps, req, client, list, now, result)
	if err != nil {
		return result, err
	}
	if result.Incomplete() && req.Mode == config.Required {
		return result, requiredFailure("unreadable_updates")
	}
	return result, nil
}

func loadUpdateList(
	ctx context.Context,
	cache history.CloudCache,
	client pulumicloud.Client,
	req Request,
	now time.Time,
) (history.UpdateListRecord, error) {
	hash := cloudCacheNamespace(req)
	cached, found, err := cache.GetUpdateList(hash)
	if err != nil {
		return cached, err
	}
	age := now.Unix() - cached.FetchedAt
	if found && age >= 0 && age < 3600 {
		return cached, nil
	}
	etag := ""
	if found {
		etag = cached.ETag
	}
	list, err := client.ListUpdates(ctx, req.Stack, etag)
	if err != nil {
		return history.UpdateListRecord{}, err
	}
	if list.NotModified {
		if !found {
			return history.UpdateListRecord{}, errors.New("cloud update list returned 304 without a cached list")
		}
	} else {
		cached.ETag = list.ETag
		cached.Updates = make([]history.CloudUpdate, 0, len(list.Updates))
		for _, u := range list.Updates {
			cached.Updates = append(cached.Updates, history.CloudUpdate{
				Version: u.Version,
				Kind:    u.Kind,
				Result:  u.Result,
				Start:   u.Start,
				End:     u.End,
			})
		}
	}
	cached.FetchedAt = now.Unix()
	if putErr := cache.PutUpdateList(hash, cached); putErr != nil {
		return history.UpdateListRecord{}, putErr
	}
	return cached, nil
}

func importUpdates(
	ctx context.Context,
	deps Deps,
	req Request,
	client pulumicloud.Client,
	list history.UpdateListRecord,
	now time.Time,
	result Result,
) (Result, error) {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentSnapshots)
	var mu sync.Mutex
	for _, update := range list.Updates {
		at := update.End
		if at == 0 {
			at = update.Start
		}
		if at < req.From.Unix() || at > req.To.Unix() {
			continue
		}
		result.Listed++
		if update.Version <= 0 || !stateChanging(update.Kind) {
			result.Skipped++
			continue
		}
		group.Go(func() error {
			partial := importSnapshot(groupCtx, deps, req, client, update.Version, at, now)
			mu.Lock()
			result.FromCache += partial.FromCache
			result.Fetched += partial.Fetched
			result.Recorded += partial.Recorded
			result.Unreadable += partial.Unreadable
			mu.Unlock()
			return groupCtx.Err()
		})
	}
	if err := group.Wait(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func importSnapshot(
	ctx context.Context,
	deps Deps,
	req Request,
	client pulumicloud.Client,
	version int,
	at int64,
	now time.Time,
) Result {
	var result Result
	hash := cloudCacheNamespace(req)
	cached, found, err := deps.Cache.GetSnapshot(hash, version)
	if err != nil {
		result.Unreadable = 1
		return result
	}
	if found {
		result.FromCache = 1
	} else {
		ids, fetchErr := client.ExportIdentities(ctx, req.Stack, version)
		if fetchErr != nil {
			deps.Logger.Debug().
				Err(fetchErr).
				Int("version", version).
				Str("component", "cloudhistory").
				Str("operation", "export").
				Msg("cloud snapshot export failed")
			result.Unreadable = 1
			return result
		}
		result.Fetched = 1
		cached = history.SnapshotRecord{
			Version:    version,
			ObservedAt: at,
			FetchedAt:  now.Unix(),
			Resources:  make([]history.CloudIdentity, 0, len(ids)),
		}
		for _, id := range ids {
			cached.Resources = append(cached.Resources, history.CloudIdentity{
				URN:      id.URN,
				CloudID:  id.CloudID,
				Type:     id.Type,
				Provider: id.Provider,
			})
		}
		if putErr := deps.Cache.PutSnapshot(hash, cached); putErr != nil {
			result.Unreadable = 1
			return result
		}
	}
	writer := history.NewWriter(deps.Store, deps.Logger)
	if writeErr := writer.RecordCloudObservations(
		req.StackContext, cached.ObservedAt, cached.Resources,
	); writeErr != nil {
		result.Unreadable = 1
		return result
	}
	result.Recorded = len(cached.Resources)
	return result
}

func stateChanging(kind string) bool {
	switch kind {
	case "update", "refresh", "destroy", "import":
		return true
	default:
		return false
	}
}

func serviceReason(err error) string {
	switch {
	case errors.Is(err, pulumicloud.ErrUnauthorized):
		return string(pulumicloud.TokenRejected)
	case errors.Is(err, pulumicloud.ErrForbidden):
		return string(pulumicloud.NoPermission)
	case errors.Is(err, pulumicloud.ErrNotFound):
		return string(pulumicloud.UnknownStack)
	default:
		return string(pulumicloud.Unreachable)
	}
}

func unavailable(deps Deps, req Request, result Result, reason string) (Result, error) {
	result.Reason = reason
	if reason == historyLockedReason {
		deps.Logger.Warn().
			Str("component", "cloudhistory").
			Str("operation", "import").
			Str("reason", reason).
			Msg("Cloud history import skipped")
	} else {
		deps.Logger.Debug().
			Str("component", "cloudhistory").
			Str("operation", "import").
			Str("reason", reason).
			Msg("Cloud history import skipped")
	}

	if req.Mode == config.Required {
		return result, requiredFailure(reason)
	}
	return result, nil
}
