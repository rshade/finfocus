# Data Model: Recommendation Dismissal and Lifecycle Management

**Feature**: 508-recommendation-dismissal
**Date**: 2026-02-05

## Entities

### DismissalStore

Top-level container for all dismissal state. Persisted as `~/.finfocus/dismissed.json`.

| Field      | Type                        | Description                          |
| ---------- | --------------------------- | ------------------------------------ |
| Version    | int                         | Schema version for migration support |
| Dismissals | map[string]DismissalRecord  | Keyed by recommendation ID           |

### DismissalRecord

A single recommendation's dismissal state.

| Field            | Type                  | Required | Description                                      |
| ---------------- | --------------------- | -------- | ------------------------------------------------ |
| RecommendationID | string                | Yes      | Unique ID from plugin                            |
| Status           | DismissalStatus       | Yes      | "dismissed" or "snoozed"                         |
| Reason           | DismissalReason       | Yes      | Enum reason for dismissal                        |
| CustomReason     | string                | No       | Free-text note (required when Reason is OTHER)   |
| DismissedAt      | time.Time             | Yes      | When the dismissal was recorded                  |
| DismissedBy      | string                | No       | User identifier for audit (future use)           |
| ExpiresAt        | *time.Time            | No       | Snooze expiry; nil means permanent dismissal     |
| LastKnown        | LastKnownRecommendation | No     | Snapshot of recommendation at dismissal time     |
| History          | []LifecycleEvent      | Yes      | Ordered list of all actions on this recommendation |

### DismissalStatus

Enum: the current lifecycle state of a dismissal record.

| Value     | Description                                 |
| --------- | ------------------------------------------- |
| dismissed | Permanently dismissed (no expiry)           |
| snoozed   | Temporarily dismissed (has ExpiresAt)       |

### DismissalReason

Enum: matches `finfocus.v1.DismissalReason` proto enum.

| Value                | Proto Value | CLI Flag               |
| -------------------- | ----------- | ---------------------- |
| NOT_APPLICABLE       | 1           | `not-applicable`       |
| ALREADY_IMPLEMENTED  | 2           | `already-implemented`  |
| BUSINESS_CONSTRAINT  | 3           | `business-constraint`  |
| TECHNICAL_CONSTRAINT | 4           | `technical-constraint` |
| DEFERRED             | 5           | `deferred`             |
| INACCURATE           | 6           | `inaccurate`           |
| OTHER                | 7           | `other`                |

### LastKnownRecommendation

Snapshot of recommendation details at the time of dismissal, used for `--include-dismissed` merged view.

| Field            | Type    | Description                           |
| ---------------- | ------- | ------------------------------------- |
| Description      | string  | Recommendation description text       |
| EstimatedSavings | float64 | Estimated monthly savings             |
| Currency         | string  | Currency code (e.g., "USD")           |
| Type             | string  | Action type (e.g., "RIGHTSIZE")       |
| ResourceID       | string  | Affected resource identifier          |

### LifecycleEvent

A single timestamped action in a recommendation's history.

| Field        | Type            | Description                          |
| ------------ | --------------- | ------------------------------------ |
| Action       | LifecycleAction | What happened                        |
| Reason       | DismissalReason | Reason at time of action             |
| CustomReason | string          | Custom reason at time of action      |
| Timestamp    | time.Time       | When the action occurred             |
| ExpiresAt    | *time.Time      | Snooze expiry if action was snooze   |

### LifecycleAction

Enum: the type of lifecycle event.

| Value       | Description                              |
| ----------- | ---------------------------------------- |
| dismissed   | Recommendation was permanently dismissed |
| snoozed     | Recommendation was snoozed with expiry   |
| undismissed | Recommendation was re-enabled            |

## State Transitions

```text
                    dismiss
        ┌──────────────────────────┐
        │                          ▼
    ┌───────┐    snooze     ┌───────────┐
    │Active │──────────────►│ Snoozed   │
    └───────┘               └───────────┘
        ▲                     │    │  ▲
        │    undismiss        │    │  │
        ├─────────────────────┘    │  │
        │                          │  │
        │    undismiss     ┌───────┘  │ re-snooze
        │                  ▼          │ (update expiry)
        │              ┌───────────┐  │
        ├──────────────│ Dismissed │──┘
        │  undismiss   └───────────┘
        │                  │  ▲
        │                  │  │ dismiss (overwrite)
        │                  └──┘
        │
        │    auto-unsnooze (expiry passed)
        └──────── (at query time) ────────
```

**Allowed transitions**:

- Active -> Dismissed (dismiss command)
- Active -> Snoozed (snooze command with --until)
- Dismissed -> Active (undismiss command)
- Snoozed -> Active (undismiss command OR auto-unsnooze at query time)
- Dismissed -> Snoozed (snooze command, direct transition)
- Snoozed -> Dismissed (dismiss command, direct transition)
- Snoozed -> Snoozed (snooze command, update expiry date)

## Validation Rules

- `RecommendationID`: non-empty string
- `Reason`: must be a valid DismissalReason enum value
- `CustomReason`: required when Reason is OTHER; max 500 characters (matches proto constraint)
- `ExpiresAt`: must be in the future when snoozing; nil for permanent dismissal
- `DismissalStore.Version`: must be 1 (current schema version)
- Duplicate dismiss of same ID: rejected with informational message
- State file corruption: treated as empty store with warning

## Relationships

```text
DismissalStore
  └── 1:N ── DismissalRecord (keyed by RecommendationID)
                ├── 1:1 ── LastKnownRecommendation (snapshot)
                └── 1:N ── LifecycleEvent (ordered by Timestamp)
```
