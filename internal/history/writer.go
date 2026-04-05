package history

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// StateResource represents a resource from a Pulumi stack export.
type StateResource struct {
	URN      string            `json:"urn"`
	CloudID  string            `json:"cloud_id"`
	Type     string            `json:"type"`
	Provider string            `json:"provider"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// PlanStep represents a plan operation with lineage data.
type PlanStep struct {
	Op         string            `json:"op"`
	URN        string            `json:"urn"`
	Type       string            `json:"type"`
	Provider   string            `json:"provider"`
	OldCloudID string            `json:"old_cloud_id,omitempty"`
	NewCloudID string            `json:"new_cloud_id,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// AnalyzerResource represents analyzer event data.
type AnalyzerResource struct {
	URN        string         `json:"urn"`
	Type       string         `json:"type"`
	Provider   string         `json:"provider"`
	CloudID    string         `json:"cloud_id,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// maxEntriesPerStep is the maximum number of history entries a single plan step
// can produce (old + new cloud ID for replace operations).
const maxEntriesPerStep = 2

// copyTags returns a defensive copy of a tag map.
// Returns an empty map for nil input.
func copyTags(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// extractTagsFromProperties extracts a flat tag map from a resource properties map.
// It checks "tagsAll" first (AWS complete tag set including default tags), then "tags".
// Returns an empty map when properties is nil or contains no recognised tag keys.
func extractTagsFromProperties(properties map[string]any) map[string]string {
	if properties == nil {
		return make(map[string]string)
	}

	for _, key := range []string{"tagsAll", "tags"} {
		v, found := properties[key]
		if !found {
			continue
		}
		switch m := v.(type) {
		case map[string]any:
			tags := make(map[string]string, len(m))
			for k, val := range m {
				if s, ok := val.(string); ok {
					tags[k] = s
				} else {
					tags[k] = fmt.Sprintf("%v", val)
				}
			}
			return tags
		case map[string]string:
			tags := make(map[string]string, len(m))
			for k, val := range m {
				tags[k] = val
			}
			return tags
		}
	}

	return make(map[string]string)
}

// Writer converts domain events into history store entries.
// It handles nil/disabled stores gracefully (fire-and-forget pattern).
type Writer struct {
	store  Store
	logger zerolog.Logger
}

// NewWriter creates a new Writer with the provided store and logger.
func NewWriter(store Store, logger zerolog.Logger) *Writer {
	return &Writer{
		store:  store,
		logger: logger,
	}
}

// RecordStateSnapshot records resources from a stack export as history entries.
// Resources with empty CloudID are skipped (logged at debug level).
// If the store is nil or disabled, this is a no-op.
// Errors during store operations are logged as warnings but not propagated (fire-and-forget).
func (w *Writer) RecordStateSnapshot(stackCtx StackContext, resources []StateResource) {
	if w.store == nil || !w.store.IsEnabled() {
		return
	}

	now := time.Now().Unix()
	entries := make([]ResourceHistoryEntry, 0, len(resources))

	for _, res := range resources {
		if res.CloudID == "" {
			w.logger.Debug().
				Str("urn", res.URN).
				Msg("skipping resource with empty CloudID")
			continue
		}

		entry := ResourceHistoryEntry{
			URN:       res.URN,
			CloudID:   res.CloudID,
			Type:      res.Type,
			Provider:  res.Provider,
			FirstSeen: now,
			LastSeen:  now,
			Source:    SourceStateSnapshot,
			Tags:      copyTags(res.Tags),
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return
	}

	if err := w.store.UpsertBatch(stackCtx.Hash(), entries); err != nil {
		w.logger.Warn().Err(err).Int("count", len(entries)).Msg("failed to record state snapshot")
	}
}

// RecordPlanLineage records old/new cloud IDs from plan operations.
// For replace ops, both the old and new cloud IDs are recorded as separate entries.
// For delete ops, the old cloud ID is recorded. For create ops, the new cloud ID is recorded.
// Steps where both cloud IDs are empty are skipped.
// If the store is nil or disabled, this is a no-op.
// Errors are logged as warnings but not propagated (fire-and-forget).
func (w *Writer) RecordPlanLineage(stackCtx StackContext, steps []PlanStep) {
	if w.store == nil || !w.store.IsEnabled() {
		return
	}

	now := time.Now().Unix()
	// Each step can produce at most 2 entries (old + new cloud ID for replace ops).
	entries := make([]ResourceHistoryEntry, 0, len(steps)*maxEntriesPerStep)

	for _, step := range steps {
		if step.OldCloudID != "" {
			entries = append(entries, ResourceHistoryEntry{
				URN:       step.URN,
				CloudID:   step.OldCloudID,
				Type:      step.Type,
				Provider:  step.Provider,
				FirstSeen: now,
				LastSeen:  now,
				Source:    SourcePlanLineage,
				Tags:      copyTags(step.Tags),
			})
		}

		if step.NewCloudID != "" {
			entries = append(entries, ResourceHistoryEntry{
				URN:       step.URN,
				CloudID:   step.NewCloudID,
				Type:      step.Type,
				Provider:  step.Provider,
				FirstSeen: now,
				LastSeen:  now,
				Source:    SourcePlanLineage,
				Tags:      copyTags(step.Tags),
			})
		}
	}

	if len(entries) == 0 {
		return
	}

	if err := w.store.UpsertBatch(stackCtx.Hash(), entries); err != nil {
		w.logger.Warn().Err(err).Int("count", len(entries)).Msg("failed to record plan lineage")
	}
}

// RecordAnalyzerEvent records a resource observation from an analyzer event.
// When CloudID is non-empty (DryRun=false), the resource is stored.
// When CloudID is empty (DryRun=true), the event is skipped since there is
// no concrete cloud identity to track.
// If the store is nil or disabled, this is a no-op.
// Errors are logged as warnings but not propagated (fire-and-forget).
func (w *Writer) RecordAnalyzerEvent(stackCtx StackContext, event AnalyzerResource) {
	if w.store == nil || !w.store.IsEnabled() {
		return
	}

	if event.CloudID == "" {
		w.logger.Debug().
			Str("urn", event.URN).
			Msg("skipping analyzer event with empty CloudID (DryRun=true)")
		return
	}

	now := time.Now().Unix()
	entry := ResourceHistoryEntry{
		URN:       event.URN,
		CloudID:   event.CloudID,
		Type:      event.Type,
		Provider:  event.Provider,
		FirstSeen: now,
		LastSeen:  now,
		Source:    SourceAnalyzerEvent,
		Tags:      extractTagsFromProperties(event.Properties),
	}

	if err := w.store.Upsert(stackCtx.Hash(), entry); err != nil {
		w.logger.Warn().Err(err).
			Str("urn", event.URN).
			Str("cloud_id", event.CloudID).
			Msg("failed to record analyzer event")
	}
}
