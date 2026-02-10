# Research: Recommendation Dismissal and Lifecycle Management

**Feature**: 508-recommendation-dismissal
**Date**: 2026-02-05

## R1: Proto Availability for DismissRecommendation

**Decision**: finfocus-spec v0.5.5 fully supports dismissal. No spec upgrade needed.

**Rationale**: Confirmed in Go module cache at `/mnt/c/GitHub/go/pkg/mod/github.com/rshade/finfocus-spec@v0.5.5/sdk/go/proto/finfocus/v1/`:

- `DismissRecommendation` RPC exists in `costsource_grpc.pb.go`
- `DismissRecommendationRequest` has fields: `RecommendationId`, `Reason` (DismissalReason enum), `CustomReason`, `ExpiresAt` (timestamppb.Timestamp), `DismissedBy`
- `DismissRecommendationResponse` has fields: `Success`, `Message`, `DismissedAt`, `ExpiresAt`, `RecommendationId`
- `DismissalReason` enum: UNSPECIFIED(0), NOT_APPLICABLE(1), ALREADY_IMPLEMENTED(2), BUSINESS_CONSTRAINT(3), TECHNICAL_CONSTRAINT(4), DEFERRED(5), INACCURATE(6), OTHER(7)
- `PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS` (capability 11) exists in `enums.pb.go`
- `ExcludedRecommendationIds` field exists on `GetRecommendationsRequest` for client-side filtering

**Alternatives considered**: Upgrading finfocus-spec to add `include_dismissed` field -- deferred to follow-up ticket.

## R2: Plugin Capability Detection

**Decision**: Use existing `ConvertCapabilities()` in `internal/pluginhost/host.go` which already converts `PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS` to the string `"dismiss_recommendations"`. Check `client.Metadata.Capabilities` slice.

**Rationale**: The capability string `"dismiss_recommendations"` is already handled at `host.go:181`. The `PluginMetadata.Capabilities` field (type `[]string`) at `host.go:78-86` is populated during plugin handshake. No new code needed for capability detection itself -- just a helper to check membership.

**Alternatives considered**: Adding a dedicated `HasCapability()` method to `pluginhost.Client` -- this is the cleanest approach and follows the pattern of the existing `pluginhost` package.

## R3: Local State File Format and Location

**Decision**: JSON file at `~/.finfocus/dismissed.json` with versioned schema.

**Rationale**: JSON is consistent with the proposed format in issue #464, simpler to read/write than YAML for flat state data, and easily inspectable by users. The `~/.finfocus/` directory already exists for config and plugins. A version field enables future schema migrations.

**Schema**:

```json
{
  "version": 1,
  "dismissals": {
    "rec-123abc": {
      "recommendation_id": "rec-123abc",
      "status": "dismissed",
      "reason": "BUSINESS_CONSTRAINT",
      "custom_reason": "Burst capacity requirement",
      "dismissed_at": "2026-01-19T10:30:00Z",
      "dismissed_by": "",
      "expires_at": null,
      "last_known": {
        "description": "Rightsize web-server from m5.xlarge to m5.large",
        "estimated_savings": 45.00,
        "currency": "USD",
        "type": "RIGHTSIZE",
        "resource_id": "aws:ec2:Instance::web-server"
      },
      "history": [
        {
          "action": "dismissed",
          "reason": "BUSINESS_CONSTRAINT",
          "custom_reason": "Burst capacity requirement",
          "timestamp": "2026-01-19T10:30:00Z"
        }
      ]
    }
  }
}
```

**Key design decisions**:

- Map keyed by recommendation ID for O(1) lookups
- `last_known` captures recommendation details at dismissal time for `--include-dismissed` merge view
- `history` array tracks lifecycle events for the `history` subcommand
- `status` field: "dismissed" (permanent) or "snoozed" (has expires_at)
- `expires_at` is nullable -- null means permanent dismissal, non-null means snoozed

**Alternatives considered**:

- YAML format: Rejected because config.yaml uses YAML but this is state data, not configuration. JSON is more natural for structured state.
- SQLite: Over-engineered for a single-user CLI tool with expected <1,000 entries.
- Embedded in config.yaml: Rejected to keep config and state separate.

## R4: CostSourceClient Adapter Extension

**Decision**: Add `DismissRecommendation` method to the `CostSourceClient` interface in `internal/proto/adapter.go`, with internal request/response types that wrap the proto types.

**Rationale**: Follows the existing adapter pattern where each proto method has:

1. An entry in the `CostSourceClient` interface (line 428)
2. Internal request/response types in the same package
3. A `clientAdapter` method that converts internal types to proto types

The `DismissRecommendation` adapter will:

- Accept internal `DismissRecommendationRequest` (with Go-native types)
- Convert to `pbc.DismissRecommendationRequest` (with proto types like `timestamppb.Timestamp`)
- Call `c.client.DismissRecommendation(ctx, protoReq, opts...)`
- Convert `pbc.DismissRecommendationResponse` back to internal type

**Alternatives considered**: Calling proto client directly from engine -- rejected because it bypasses the adapter abstraction and makes testing harder.

## R5: Engine Orchestration for Dismiss

**Decision**: Add `DismissRecommendation()` method to `Engine` that tries plugin first (if capable), always persists locally.

**Rationale**: The engine already orchestrates between plugins for `GetRecommendationsForResources()`. The dismiss flow is simpler (single ID, not batch) but follows the same plugin iteration pattern. The engine calls each connected plugin's `DismissRecommendation` if the plugin has the capability, then always persists locally.

**Flow**:

1. Load local state file
2. Check if already dismissed (return early with message)
3. For each connected plugin with `dismiss_recommendations` capability:
   a. Call `DismissRecommendation` RPC
   b. Log success/failure per plugin
4. Persist dismissal locally (always, regardless of plugin result)
5. Return result with plugin status

For `GetRecommendationsForResources()`, modify to:

1. Load local state, extract non-expired dismissed IDs
2. Pass IDs via `ExcludedRecommendationIds` in the request
3. After receiving results, apply local filter as belt-and-suspenders

**Alternatives considered**: CLI-level filtering only (no engine changes) -- rejected because the engine already owns the recommendation flow and `ExcludedRecommendationIds` must be set on the proto request.

## R6: CLI Subcommand Structure

**Decision**: Convert the existing `recommendations` command into a parent command with subcommands: `list` (default), `dismiss`, `snooze`, `undismiss`, `history`.

**Rationale**: The current `recommendations` command runs directly (no subcommands). To add dismiss/snooze/undismiss/history, the cleanest approach is:

- Make the current behavior the `list` subcommand (or default when no subcommand given)
- Add `dismiss`, `snooze`, `undismiss`, `history` as peer subcommands
- `dismiss` and `snooze` require `--pulumi-json` to connect to plugins
- `undismiss` and `history` operate on local state only (no plugin needed)

Registration in `cost_recommendations.go`:

```go
cmd.AddCommand(
    newRecommendationsDismissCmd(),
    newRecommendationsSnoozeCmd(),
    newRecommendationsUndismissCmd(),
    newRecommendationsHistoryCmd(),
)
```

The existing command keeps its `RunE` for backward compatibility (running `finfocus cost recommendations --pulumi-json plan.json` still works as the list behavior).

**Alternatives considered**:

- Separate top-level commands (`finfocus dismiss-recommendation`) -- rejected, breaks CLI hierarchy conventions.
- Single `dismiss` command with mode flags -- rejected, Cobra subcommands are more idiomatic.

## R7: Confirmation Prompt Pattern

**Decision**: Use `fmt.Fprintf(cmd.ErrOrStderr())` for prompts and `bufio.Scanner` for reading confirmation, with `--force` flag to skip.

**Rationale**: FinFocus doesn't currently have a confirmation prompt pattern, but the standard Go CLI approach is reading from stdin. Bubble Tea is available but over-engineered for a yes/no prompt. The `--force` flag follows common CLI conventions (e.g., `rm -f`, `kubectl delete --force`).

**Alternatives considered**: Bubble Tea interactive prompt -- rejected as too heavyweight for a simple yes/no.

## R8: DismissalReason Enum Mapping

**Decision**: The original issue #464 proposed different reason values (false-positive, intentional, not-actionable) than what the proto actually defines. The implementation MUST use the proto's `DismissalReason` enum values.

**Proto values** (finfocus-spec v0.5.5):

| Proto Enum Value             | CLI Flag Value         | Description                            |
| ---------------------------- | ---------------------- | -------------------------------------- |
| DISMISSAL_REASON_NOT_APPLICABLE      | `not-applicable`       | Recommendation doesn't apply           |
| DISMISSAL_REASON_ALREADY_IMPLEMENTED | `already-implemented`  | Already acted on this recommendation   |
| DISMISSAL_REASON_BUSINESS_CONSTRAINT | `business-constraint`  | Business reason prevents action        |
| DISMISSAL_REASON_TECHNICAL_CONSTRAINT| `technical-constraint` | Technical limitation prevents action   |
| DISMISSAL_REASON_DEFERRED            | `deferred`             | Will address later                     |
| DISMISSAL_REASON_INACCURATE          | `inaccurate`           | Recommendation data is wrong           |
| DISMISSAL_REASON_OTHER               | `other`                | Custom reason (requires --note)        |

The `dismissal_reasons.go` utility file follows the `action_types.go` pattern for parsing and label generation.

**Alternatives considered**: Mapping issue #464's proposed values to proto values -- rejected because it creates a confusing translation layer.
