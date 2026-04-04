package history

import (
	"slices"
	"sort"

	"github.com/rs/zerolog"
)

// HistoricalResource represents an enriched view of a resource's history,
// grouping all cloud IDs that have been observed for a single URN.
type HistoricalResource struct {
	URN      string
	Type     string
	Provider string
	CloudIDs []string
	Tags     map[string]string
}

// Reader queries historical resource data from a Store.
type Reader struct {
	store  Store
	logger zerolog.Logger
}

// NewReader creates a new Reader.
func NewReader(store Store, logger zerolog.Logger) *Reader {
	return &Reader{
		store:  store,
		logger: logger,
	}
}

// GetResourcesForPeriod returns all resources observed within the given time range,
// grouped by URN with all unique CloudIDs collected into a single HistoricalResource.
//
// If the store is nil or disabled, returns an empty slice with no error.
func (hr *Reader) GetResourcesForPeriod(stack StackContext, from, to int64) ([]HistoricalResource, error) {
	if hr.store == nil || !hr.store.IsEnabled() {
		return []HistoricalResource{}, nil
	}

	entries, err := hr.store.GetAllForStack(stack.Hash(), from, to)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return []HistoricalResource{}, nil
	}

	// Sort entries by LastSeen ascending so most-recent tags win deterministically.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastSeen < entries[j].LastSeen
	})

	// Group entries by URN, collecting all unique CloudIDs and merged Tags
	grouped := make(map[string]*HistoricalResource)
	for _, entry := range entries {
		res, exists := grouped[entry.URN]
		if !exists {
			res = &HistoricalResource{
				URN:      entry.URN,
				Type:     entry.Type,
				Provider: entry.Provider,
				CloudIDs: []string{},
				Tags:     make(map[string]string),
			}
			grouped[entry.URN] = res
		}

		// Add CloudID if not already present
		if !slices.Contains(res.CloudIDs, entry.CloudID) {
			res.CloudIDs = append(res.CloudIDs, entry.CloudID)
		}

		// Merge tags — sorted by LastSeen so most-recent values win
		for k, v := range entry.Tags {
			res.Tags[k] = v
		}
	}

	// Convert map to slice
	result := make([]HistoricalResource, 0, len(grouped))
	for _, res := range grouped {
		result = append(result, *res)
	}

	return result, nil
}
