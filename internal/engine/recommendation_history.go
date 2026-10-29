package engine

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// recommendationHistoryTimeout bounds one GetRecommendationHistory RPC.
// Tests shorten it. Production matches the other per-call plugin deadlines.
var recommendationHistoryTimeout = perResourceTimeout //nolint:gochecknoglobals // tests shorten it

// GetRecommendationHistory returns local and plugin lifecycle events for one
// recommendation, oldest first, plus one warning for each plugin that failed
// to answer. A plugin that does not implement history is skipped with no
// warning. Warnings name the plugin and do not include the plugin error.
// It does not modify the dismissal store.
// The caller's cancellation stops the query and returns no events. A plugin
// that times out, or that reports cancellation on its own, is logged, warned,
// and skipped.
func (e *Engine) GetRecommendationHistory(
	ctx context.Context,
	store *config.DismissalStore,
	recommendationID string,
) ([]config.LifecycleEvent, []string, error) {
	if recommendationID == "" {
		return nil, nil, errors.New("recommendation ID is required")
	}
	if store == nil {
		return nil, nil, errors.New("dismissal store is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	local := localRecommendationHistory(store, recommendationID)
	pluginEvents, warnings, err := e.pluginRecommendationHistory(ctx, recommendationID)
	if err != nil {
		return nil, nil, err
	}
	return mergeRecommendationHistory(local, pluginEvents), warnings, nil
}

func localRecommendationHistory(store *config.DismissalStore, recommendationID string) []config.LifecycleEvent {
	record, ok := store.Get(recommendationID)
	if !ok || record == nil || len(record.History) == 0 {
		return nil
	}
	copied := make([]config.LifecycleEvent, len(record.History))
	for i, ev := range record.History {
		copied[i] = cloneLifecycleEvent(ev)
	}
	return copied
}

func (e *Engine) pluginRecommendationHistory(
	ctx context.Context,
	recommendationID string,
) ([]config.LifecycleEvent, []string, error) {
	log := logging.FromContext(ctx)
	var merged []config.LifecycleEvent
	var warnings []string
	for _, client := range e.clients {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		callCtx, cancel := context.WithTimeout(ctx, recommendationHistoryTimeout)
		events, err := recommendationHistoryFromClient(callCtx, client, recommendationID)
		cancel()
		if err != nil {
			// A deadline on this one call is that plugin's failure. Stop only
			// when the caller is done, matching batch cost's parent-context check.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, nil, ctxErr
			}
			if status.Code(err) == codes.Unimplemented {
				log.Debug().
					Str("component", "engine").
					Str("operation", "recommendation_history").
					Str("plugin", pluginClientName(client)).
					Msg("plugin does not support recommendation history")
				continue
			}
			log.Warn().
				Str("component", "engine").
				Str("operation", "recommendation_history").
				Str("plugin", pluginClientName(client)).
				Err(err).
				Msg("plugin recommendation history failed")
			warnings = append(warnings, recommendationHistoryWarning(pluginClientName(client)))
			continue
		}
		merged = append(merged, events...)
	}
	return merged, warnings, nil
}

// recommendationHistoryWarning is the browser-facing notice for one plugin.
// The plugin error stays in the log and is not copied here.
func recommendationHistoryWarning(plugin string) string {
	return plugin + ": recommendation history unavailable"
}

func recommendationHistoryFromClient(
	ctx context.Context,
	client *pluginhost.Client,
	recommendationID string,
) ([]config.LifecycleEvent, error) {
	if client == nil || client.API == nil {
		return nil, errors.New("plugin client is unavailable")
	}
	resp, err := client.API.GetRecommendationHistory(ctx, &proto.GetRecommendationHistoryRequest{
		RecommendationID: recommendationID,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("plugin returned an empty recommendation history response")
	}
	events := make([]config.LifecycleEvent, 0, len(resp.Events))
	for _, ev := range resp.Events {
		events = append(events, lifecycleFromPlugin(ev))
	}
	return events, nil
}

func lifecycleFromPlugin(ev proto.RecommendationLifecycleEvent) config.LifecycleEvent {
	return cloneLifecycleEvent(config.LifecycleEvent{
		Action:       config.LifecycleAction(ev.Action),
		Reason:       ev.Reason,
		CustomReason: ev.CustomReason,
		Actor:        ev.Actor,
		Timestamp:    ev.Timestamp,
		ExpiresAt:    ev.ExpiresAt,
	})
}

func cloneLifecycleEvent(ev config.LifecycleEvent) config.LifecycleEvent {
	if ev.ExpiresAt != nil {
		expires := *ev.ExpiresAt
		ev.ExpiresAt = &expires
	}
	return ev
}

func pluginClientName(client *pluginhost.Client) string {
	if client == nil || client.Name == "" {
		return "unnamed"
	}
	return client.Name
}

// mergeRecommendationHistory orders events oldest first.
// Actor is the last sort key so equal changes stay adjacent for dedup.
func mergeRecommendationHistory(local, plugin []config.LifecycleEvent) []config.LifecycleEvent {
	combined := make([]config.LifecycleEvent, 0, len(local)+len(plugin))
	combined = append(combined, local...)
	combined = append(combined, plugin...)
	slices.SortStableFunc(combined, compareHistory)

	merged := make([]config.LifecycleEvent, 0, len(combined))
	for i := 0; i < len(combined); {
		j := i + 1
		for j < len(combined) && sameHistoryIdentity(combined[i], combined[j]) {
			j++
		}
		merged = append(merged, preferHistoryActors(combined[i:j])...)
		i = j
	}
	return merged
}

func compareHistory(a, b config.LifecycleEvent) int {
	if c := compareHistoryTime(a.Timestamp, b.Timestamp); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Action), string(b.Action)); c != 0 {
		return c
	}
	if c := strings.Compare(a.Reason, b.Reason); c != 0 {
		return c
	}
	if c := strings.Compare(a.CustomReason, b.CustomReason); c != 0 {
		return c
	}
	if c := compareExpiry(a.ExpiresAt, b.ExpiresAt); c != 0 {
		return c
	}
	return strings.Compare(a.Actor, b.Actor)
}

func compareHistoryTime(a, b time.Time) int {
	aAbsent := a.IsZero()
	bAbsent := b.IsZero()
	if aAbsent != bAbsent {
		if aAbsent {
			return 1
		}
		return -1
	}
	if aAbsent {
		return 0
	}
	return a.UTC().Compare(b.UTC())
}

func compareExpiry(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	default:
		return a.UTC().Compare(b.UTC())
	}
}

func sameHistoryIdentity(a, b config.LifecycleEvent) bool {
	if a.Action != b.Action || a.Reason != b.Reason || a.CustomReason != b.CustomReason {
		return false
	}
	if compareHistoryTime(a.Timestamp, b.Timestamp) != 0 {
		return false
	}
	return compareExpiry(a.ExpiresAt, b.ExpiresAt) == 0
}

// preferHistoryActors keeps one row per named actor.
// A blank actor is kept only when no copy of the same change names one.
func preferHistoryActors(group []config.LifecycleEvent) []config.LifecycleEvent {
	named := make([]config.LifecycleEvent, 0, len(group))
	seen := make(map[string]struct{}, len(group))
	var blank config.LifecycleEvent
	hasBlank := false
	for _, ev := range group {
		if ev.Actor == "" {
			if !hasBlank {
				blank = ev
				hasBlank = true
			}
			continue
		}
		if _, ok := seen[ev.Actor]; ok {
			continue
		}
		seen[ev.Actor] = struct{}{}
		named = append(named, ev)
	}
	if len(named) > 0 {
		return named
	}
	if hasBlank {
		return []config.LifecycleEvent{blank}
	}
	return nil
}
