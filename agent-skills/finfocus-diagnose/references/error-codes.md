# Error codes and sentinel errors

Structured codes on a cost result (`internal/engine/types.go`):

| Code | Constant | When it is set |
| --- | --- | --- |
| `PLUGIN_ERROR` | `ErrCodePluginError` | Plugin RPC failed on the adapter path, and the error is not a deadline. |
| `VALIDATION_ERROR` | `ErrCodeValidationError` | `pluginsdk` rejected the request before a price was returned. The note is `VALIDATION:` plus a space and the validation error. |
| `TIMEOUT_ERROR` | `ErrCodeTimeoutError` | The adapter saw `context.DeadlineExceeded` or gRPC code `DeadlineExceeded`. |
| `NO_COST_DATA` | `ErrCodeNoCostData` | The engine finished the plugin chain and the spec fallback with no price. |

A deadline (`context.DeadlineExceeded` or gRPC `DeadlineExceeded`) is `TIMEOUT_ERROR`. Every other plugin RPC error on the adapter path is `PLUGIN_ERROR`.

## Notes prefixes

`GetProjectedCostWithErrors` and `GetActualCostWithErrors` in `internal/proto/adapter.go` write notes with `fmt.Sprintf`, so the prefix includes a space:

- validation failure: `VALIDATION:` plus a space, and code `VALIDATION_ERROR`. Monthly cost is 0. The resource is not sent to the plugin.
- plugin RPC failure: `ERROR:` plus a space, and code `TIMEOUT_ERROR` or `PLUGIN_ERROR`.

The batch validator writes the same `VALIDATION:` placeholder, including the trailing space.

`cost projected` does not call those adapter functions. It calls `Engine.GetProjectedCostWithErrors`. An empty plugin response inside `getProjectedCostFromPlugin` is returned as `no cost data available`. A failed plugin call keeps the plugin's reason: the gRPC status renders as `<Code>: <message>` (for example `InvalidArgument: region is required for GetProjectedCost`), so the errors list entry reads `plugin call failed: InvalidArgument: ...`. After the fallback chain, the placeholder code is `NO_COST_DATA` and the note contains `No pricing information available`; when a plugin failed with anything other than `no cost data available`, the first such error is appended to the note in parentheses (after any `declined by` reasons, capped at 160 bytes).

Actual cost behaves differently:

- Without `--fallback-estimate`, the resource is omitted. The log line is `no actual cost data available (use --fallback-estimate to include $0 placeholders)`.
- With `--fallback-estimate`, a plugin error produces the note `ERROR: plugin call failed: <reason>`, with the first plugin error's text (gRPC status as `<Code>: <message>`, capped at 160 bytes). No plugin error produces `No actual cost data available`.

Overview and the analyzer treat a note as an error when it has prefix `VALIDATION:` or `ERROR:` (the check does not require the space).

A cache hit appends (cached) to the adapter field, including the leading space.

## Sentinel errors

| Error string | Meaning |
| --- | --- |
| `no cost data available` | Plugin returned no result row on the projected path. |
| `mixed currencies not supported in cross-provider aggregation` | Aggregation saw more than one currency. |
| `invalid groupBy type for cross-provider aggregation` | Grouping is not a time bucket. |
| `empty results provided for aggregation` | Aggregation was called with no rows. |
| `invalid date range: end date must be after start date` | Actual-cost range is reversed. |
| `resource validation failed` | Resource type, id, or properties failed `ResourceDescriptor.Validate`. |
| `configuration file appears corrupted` | Strict mode could not parse the config file. |
| `plugin spec version incompatible with core` | Strict spec compatibility rejected the plugin. |
| `cache entry not found` | Key is absent. |
| `cache entry expired` | Key existed and the TTL elapsed. |
| `cache key cannot be empty` | Caller passed an empty key. |
| `cache TTL out of range` | Stored TTL is outside 1..604800 seconds. |
| `cache is disabled` | Cache operations were called while the store is off. |
| `cache database locked by another process` | BoltDB open hit a lock timeout. |
| `one or more plugins failed validation` | `plugin validate` failed at least one plugin. |
| `TTL must be between 60 and 604800 seconds` | Requested TTL is outside `MinTTLSeconds`..`MaxTTLSeconds`. |

BoltDB corruption is not a finfocus sentinel. Open treats `ErrInvalid`, `ErrChecksum`, and `ErrVersionMismatch` from `go.etcd.io/bbolt/errors` as corruption, deletes `cache.db`, and creates a new file. A lock is `ErrTimeout` from that package and becomes `cache database locked by another process`.

## gRPC codes on the adapter path

| Condition | Structured code | Note prefix |
| --- | --- | --- |
| `pluginsdk` validation error | `VALIDATION_ERROR` | `VALIDATION:` plus a space |
| `context.DeadlineExceeded` or `codes.DeadlineExceeded` | `TIMEOUT_ERROR` | `ERROR:` plus a space |
| Any other RPC error | `PLUGIN_ERROR` | `ERROR:` plus a space |

`codes.InvalidArgument` is not rewritten. On the adapter path it is `PLUGIN_ERROR`. On `cost projected` the engine keeps the status text: `getProjectedCostFromPlugin` returns `<Code>: <message>` for any RPC error, and `no cost data available` only for an empty result list. A regionless resource that a plugin rejects with `InvalidArgument: region is required for GetProjectedCost` therefore surfaces that reason on the row and in `.finfocus.errors`; region resolution sources are the resource's `region` or `availabilityZone` inputs, an ARN property, then the `AWS_REGION`/`AWS_DEFAULT_REGION` environment variables. Core does not read the Pulumi provider resource or stack config (`aws:region`), so set one of those sources.

The routing struct comment says `InvalidArgument` does not fall back. `Engine.GetProjectedCostWithErrors` does not implement that exception. Any error from `getProjectedCostFromPlugin` tries the next plugin when that plugin's fallback flag is enabled. The flag defaults to true.

A non-empty cost result stops the chain, including a monthly cost of 0. Higher priority is a higher number (`sortByPriority`, stable, descending). Plugins in that order are tried one at a time, including plugins that share a priority. No `routing` section means every loaded plugin is eligible, except types that start with `pulumi:`.

A plugin that does not implement `Supports` returns `Supports capability not implemented by this plugin`. The engine treats that reason as fail-open and still calls the plugin. A real decline (wrong region, wrong provider) skips the plugin and the placeholder note can include the decline reason after `No pricing information available`.
