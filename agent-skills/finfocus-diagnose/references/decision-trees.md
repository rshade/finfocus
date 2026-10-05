# Decision trees

## Plugin connectivity

Symptoms: `context deadline exceeded`, `connection refused`, `broken pipe`, `EOF`, `no such file or directory`, `address already in use`, `port is already allocated`.

1. `finfocus plugin list --verbose`. No row means the plugin is not installed under the active plugin directory. `FINFOCUS_PLUGIN_DIR` wins over the directory derived from `FINFOCUS_HOME`.
2. `finfocus plugin validate --plugin <name>`. Fix `plugin binary not found:`, `plugin binary is not executable`, or a `plugin.manifest.json` name/version mismatch before retrying a cost command.
3. Port collisions retry inside `StartWithRetry`. The launcher holds a listener, releases it, then the plugin binds `--port`. Retryable messages are `address already in use`, `bind: address already in use`, and `port is already allocated`.
4. The bind timeout is 60 seconds. The CI bind timeout is 120 seconds. The port retries are 5. The CI port retries are 10. The stdout port fallback is 5 seconds. Those CI values apply only when `CI` is `true`.
5. After `Kill()`, the host calls `cmd.Wait()`. A plugin left unreaped is a zombie. Do not leave a killed plugin without `Wait`.
6. `plugin spec version incompatible with core` appears only when `plugin_host.strict_compatibility` is true and the major spec version does not match. The default is permissive.

`FINFOCUS_ANALYZER_MODE=true` (`analyzer serve` sets this) discards plugin stdout and stderr when no log file is configured, so Pulumi's preview stays clean. A configured log file still receives that output, and that file wins over analyzer mode. A missing plugin log line in the terminal is expected in analyzer mode.

## Config resolution

Symptoms: an edit does nothing, or the wrong plugin directory is used.

1. `finfocus config list` and compare it with the file you edited.
2. Global file: `$FINFOCUS_HOME/config.hujson`, else `$PULUMI_HOME/finfocus/config.hujson`, else `~/.finfocus/config.hujson`.
3. Project file: `--project-dir`, else `FINFOCUS_PROJECT_DIR`, else the `Pulumi.yaml` walk. The project file replaces whole top-level keys.
4. If the project file sets `cost` and omits `routing`, routing still comes from the global file. If it sets `cost` with only a budget, it also replaces the global cache settings, because the merge is not deep.
5. `FINFOCUS_CONFIG_STRICT=true` panics instead of warning when the file cannot be read or parsed. Turn it off to see the warning and the defaults.
6. `finfocus config routes list` shows whether routing is configured. `finfocus config routes test aws:ec2/instance:Instance us-east-1` shows which plugin the rules would pick. Pulumi type tokens look like `aws:ec2/instance:Instance`. A plugin may expect `aws:ec2:Instance`. The test command shows the configured match; it does not prove the plugin accepts the token.

## Cache

The store is BoltDB. The file name is `cache.db` inside the cache directory (`<config dir>/cache` unless `FINFOCUS_CACHE_DIR` is set).

Buckets: `projected`, `actual`, `recommendations`, `resolve_types`, `scores`.

Example keys produced by the builders (empty segments are `_`):

- `projected/aws/aws:ec2/instance:Instance/us-east-1/t3.micro`
- `projected/_/_/_/_`
- `actual/aws/ec2+s3/2026-10-01/2026-10-03/_`
- `recommendations/multi/a+b/abc`

Projected keys are `projected/{provider}/{type}/{region}/{sku}`. Actual keys are `actual/{provider}/{types}/{from}/{to}/{filter-hash}` with types sorted and joined by `+`, dates `2006-01-02`, and `_` when the filter map is empty. Recommendation keys are `recommendations/multi/{sorted-types}/{inputs-hash}`.

| Symptom | What it means | What to do |
| --- | --- | --- |
| Adapter ends with (cached) | The price was served from BoltDB. | Rerun with `FINFOCUS_CACHE_ENABLED=false` to compare. |
| `cache database locked by another process` | Another process holds `cache.db`. | Stop the other finfocus process. The open already logs the lock and continues without cache. |
| A stored price is served after the plugin would now answer differently | A cache hit is returned until the entry expires. Recreation on open only happens for `ErrInvalid`, `ErrChecksum`, or `ErrVersionMismatch`. | Delete `cache.db`, or rerun with `FINFOCUS_CACHE_ENABLED=false`. |
| `expires_at` seems ignored | A timestamp in the past skips caching. A future timestamp beyond the max is capped. | TTL must be between 60 and 604800 seconds. The default TTL is 3600. |

`CalculatePluginTTL` uses a nil `expires_at` as the default TTL, a past timestamp as skip, a future timestamp inside the max as the remaining seconds, and a future timestamp past the max as 604800.

## Zero cost

`cost projected` calls `GetProjectedCostWithErrors`, which does not apply the per-resource timeout.
`GetProjectedCost` and `GetActualCost` do. The per-resource timeout is 5 seconds. The query timeout is 60 seconds, applied only when the context has no deadline yet.
There is no flag or environment variable for either constant.

```text
Resource shows $0
├── Note starts with "VALIDATION:"
│   └── pluginsdk rejected the request. Code VALIDATION_ERROR. Fix the resource, SKU, or region. This is not a plugin outage.
├── Note starts with "ERROR:"
│   ├── "ERROR: plugin call failed" on cost actual --fallback-estimate
│   └── "ERROR: " plus a deadline is TIMEOUT_ERROR on the adapter path. Any other RPC error there is PLUGIN_ERROR.
├── Code NO_COST_DATA and note contains "No pricing information available"
│   └── Every plugin returned an error or no rows, and no local spec matched.
│       A real plugin failure is appended to the note in parentheses
│       (e.g. "plugin call failed: InvalidArgument: region is required ...");
│       an empty result stays plain "No pricing information available" and the
│       errors list shows "plugin call failed: no cost data available".
├── Monthly cost 0 and no error note
│   └── The plugin priced the resource at 0. That is a result. The engine does not ask the next plugin.
└── The resource is missing from the output
    ├── Type starts with "pulumi:" (including pulumi:pulumi:Stack). Skipped, not priced.
    └── cost actual without --fallback-estimate omitted a resource with no actual cost.
```

Checks that explain a real zero or a missing row:

1. Plan JSON must include `newState` with `type` and `inputs`. Empty inputs leave SKU and region empty, and the plugin can reject or return no price.
2. `pulumi preview --json` nests those fields under `newState`. Rows that only have a top-level `type` are not enough.
3. Types that start with `pulumi:` are framework resources. The filter is the prefix `pulumi:`, not a separate stack-only comparison.
4. `finfocus plugin list` shows whether any plugin for that provider is installed.
5. `finfocus config routes test <resource-type> [region]` shows whether routing would select one.
6. Rerun with `FINFOCUS_CACHE_ENABLED=false`. A cached zero stays a zero until the entry expires or `cache.db` is removed.
7. Unknown instance types can be a genuine `$0` from the pricing plugin (the Kubernetes allocator treats `Monthly <= 0` as unpriced). On `cost projected` that `$0` is still a successful result.

Fallback, in the order the engine uses:

- A returned `CostResult`, including monthly cost 0, stops the chain.
- An error (including an empty result reported as `no cost data available`) tries the next plugin when fallback is enabled. Default is enabled.
- `InvalidArgument` is not a special stop. See the error-code reference.
- Higher priority number is tried first.
- If nothing returns a row, a local spec is tried, then the `NO_COST_DATA` placeholder.

## Logging

```bash
finfocus cost projected --debug --pulumi-json plan.json
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json
export FINFOCUS_TRACE_ID=external-trace-123
```

Log level precedence is `--debug`, then `FINFOCUS_LOG_LEVEL`, then the config file, then `info`.
Default format is `text`. When a log file is configured, plugin stdout and stderr are copied into that file, and that wins over analyzer mode. If there is no log file and `FINFOCUS_ANALYZER_MODE` is `true`, they are discarded. Otherwise they pass through only when stderr is an interactive terminal.
