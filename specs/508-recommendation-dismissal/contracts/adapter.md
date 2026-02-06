# Contract: CostSourceClient Adapter Extension

**Feature**: 508-recommendation-dismissal
**Date**: 2026-02-05

## Interface Addition

Add to `CostSourceClient` interface in `internal/proto/adapter.go`:

```go
DismissRecommendation(
    ctx context.Context,
    in *DismissRecommendationRequest,
    opts ...grpc.CallOption,
) (*DismissRecommendationResponse, error)
```

## Internal Request Type

```go
type DismissRecommendationRequest struct {
    RecommendationID string
    Reason           pbc.DismissalReason
    CustomReason     string
    ExpiresAt        *time.Time
    DismissedBy      string
}
```

## Internal Response Type

```go
type DismissRecommendationResponse struct {
    Success          bool
    Message          string
    DismissedAt      time.Time
    ExpiresAt        *time.Time
    RecommendationID string
}
```

## Engine Methods

### DismissRecommendation

```go
func (e *Engine) DismissRecommendation(
    ctx context.Context,
    req DismissRequest,
) (*DismissResult, error)
```

**DismissRequest**:

```go
type DismissRequest struct {
    RecommendationID string
    Reason           string          // CLI flag value (e.g., "business-constraint")
    CustomReason     string          // --note value
    ExpiresAt        *time.Time      // --until value (nil = permanent dismiss)
    Recommendation   *Recommendation // Current recommendation details for LastKnown snapshot
}
```

**DismissResult**:

```go
type DismissResult struct {
    RecommendationID string
    PluginDismissed  bool    // true if plugin accepted the dismissal
    PluginName       string  // which plugin handled it
    PluginMessage    string  // plugin's response message
    LocalPersisted   bool    // true if local state was updated
    Warning          string  // non-fatal warning (e.g., plugin failed but local succeeded)
}
```

### UndismissRecommendation

```go
func (e *Engine) UndismissRecommendation(
    ctx context.Context,
    recommendationID string,
) (*UndismissResult, error)
```

**UndismissResult**:

```go
type UndismissResult struct {
    RecommendationID string
    WasDismissed     bool   // false if wasn't dismissed
    Message          string
}
```

### GetRecommendationHistory

```go
func (e *Engine) GetRecommendationHistory(
    ctx context.Context,
    recommendationID string,
) ([]LifecycleEvent, error)
```

## DismissalStore Interface

```go
type DismissalStore interface {
    Load() error
    Save() error
    Get(recommendationID string) (*DismissalRecord, bool)
    Set(record *DismissalRecord) error
    Delete(recommendationID string) error
    GetDismissedIDs() []string          // For ExcludedRecommendationIds
    GetAllRecords() map[string]*DismissalRecord  // For --include-dismissed
    GetExpiredSnoozes() []*DismissalRecord  // For auto-unsnooze
    CleanExpiredSnoozes() error
}
```

## CLI Commands

### dismiss

```text
finfocus cost recommendations dismiss <recommendation-id> [flags]

Flags:
  -r, --reason string   Dismissal reason (required): not-applicable, already-implemented,
                         business-constraint, technical-constraint, deferred, inaccurate, other
  -n, --note string     Free-text explanation (required for "other" reason)
  -f, --force           Skip confirmation prompt
      --pulumi-json     Path to Pulumi preview JSON (required for plugin communication)
      --adapter         Use specific adapter plugin
```

### snooze

```text
finfocus cost recommendations snooze <recommendation-id> [flags]

Flags:
  -r, --reason string   Dismissal reason (default: "deferred"): same values as dismiss
  -n, --note string     Free-text explanation
      --until string    Snooze until date (required, ISO 8601 or YYYY-MM-DD)
  -f, --force           Skip confirmation prompt
      --pulumi-json     Path to Pulumi preview JSON (required for plugin communication)
      --adapter         Use specific adapter plugin
```

### undismiss

```text
finfocus cost recommendations undismiss <recommendation-id> [flags]

Flags:
  -f, --force           Skip confirmation prompt
```

### history

```text
finfocus cost recommendations history <recommendation-id> [flags]

Flags:
      --output string   Output format: table, json (default: table)
```

### list (modified)

```text
finfocus cost recommendations [list] [flags]

New flags:
      --include-dismissed   Show dismissed/snoozed recommendations in output
```
