// Package contracts defines the interfaces for the Resource History Store.
// These are design contracts — they will be moved to internal/history/
// during implementation. This file is a planning artifact, NOT compiled code.
package contracts

// HistoryStore is the primary interface for resource history persistence.
// It follows the same optional-dependency pattern as cache.Cache.
// When nil or disabled, callers degrade gracefully to current behavior.
type HistoryStore interface {
	// Upsert records a resource observation. If the (URN, CloudID) pair
	// already exists for this stack, only LastSeen is updated.
	// New CloudID for the same URN creates a new entry.
	Upsert(entry ResourceHistoryEntry) error

	// UpsertBatch records multiple observations atomically.
	// Used by state snapshot writer (typically 10-500 resources per call).
	UpsertBatch(entries []ResourceHistoryEntry) error

	// GetCloudIDsForURN returns all cloud IDs ever observed for a URN
	// within a stack, optionally filtered to a time range.
	// Returns entries where [FirstSeen, LastSeen] overlaps [from, to].
	GetCloudIDsForURN(stackHash, urnHash string, from, to int64) ([]ResourceHistoryEntry, error)

	// GetAllForStack returns all history entries for a stack,
	// optionally filtered to a time range.
	GetAllForStack(stackHash string, from, to int64) ([]ResourceHistoryEntry, error)

	// GetDeletedResources returns history entries that exist in the store
	// but are NOT in the provided set of current URN hashes.
	// Used to find resources deleted since the last observation.
	GetDeletedResources(
		stackHash string, currentURNHashes map[string]bool, from, to int64,
	) ([]ResourceHistoryEntry, error)

	// CleanupExpired removes entries with LastSeen older than the
	// retention window. Returns count of removed entries.
	CleanupExpired(retentionDays int) (int, error)

	// IsEnabled returns whether the store is active.
	IsEnabled() bool

	// Close releases resources. Safe to call multiple times.
	Close() error
}

// HistoryWriter provides high-level write operations that abstract
// the source-specific logic for recording resource observations.
type HistoryWriter interface {
	// RecordStateSnapshot records all resources from a pulumi stack export.
	// Called during cost actual and overview commands.
	RecordStateSnapshot(stack StackContext, resources []StateResource) error

	// RecordPlanLineage records old/new cloud IDs from plan operations.
	// Called during overview (after pulumi preview).
	RecordPlanLineage(stack StackContext, steps []PlanStep) error

	// RecordAnalyzerEvent records resource observations from analyzer.
	// Called during pulumi up via the analyzer server.
	RecordAnalyzerEvent(stack StackContext, event AnalyzerResource) error
}

// HistoryReader provides high-level read operations for cost queries.
type HistoryReader interface {
	// GetResourcesForPeriod returns all resources (current + deleted)
	// that were active during the given time range for a stack.
	// This is the primary read path for enhanced actual cost queries.
	GetResourcesForPeriod(stack StackContext, from, to int64) ([]HistoricalResource, error)
}

// --- Supporting Types ---

// ResourceHistoryEntry is the stored record for a single observation.
type ResourceHistoryEntry struct {
	URN       string            // Full Pulumi URN
	CloudID   string            // Cloud provider resource ID
	Type      string            // Pulumi type token
	Provider  string            // Cloud provider name
	FirstSeen int64             // Unix timestamp
	LastSeen  int64             // Unix timestamp
	Source    string            // "state_snapshot", "plan_lineage", "analyzer_event"
	Tags      map[string]string // Resource tags at observation time
}

// StackContext identifies the Pulumi stack for scoping history queries.
type StackContext struct {
	Organization string
	Project      string
	Stack        string
}

// StateResource represents a resource from pulumi stack export.
type StateResource struct {
	URN      string
	CloudID  string
	Type     string
	Provider string
	Tags     map[string]string
}

// PlanStep represents a plan operation with lineage data.
type PlanStep struct {
	Op         string // "replace", "delete", "create", "update"
	URN        string
	Type       string
	Provider   string
	OldCloudID string // From OldState (empty for create)
	NewCloudID string // From NewState (empty for delete)
	Tags       map[string]string
}

// AnalyzerResource represents a resource observed during analyzer events.
type AnalyzerResource struct {
	URN        string
	Type       string
	Provider   string
	CloudID    string // Available when DryRun=false
	Properties map[string]interface{}
}

// HistoricalResource is the enriched output from history queries,
// containing all known cloud IDs for a resource URN.
type HistoricalResource struct {
	URN      string
	Type     string
	Provider string
	CloudIDs []string // All cloud IDs active during the queried period
	Tags     map[string]string
}
