# Research: Cluster Allocation Web Hook

## R1. Where the JSON contract lives

- **Decision**: Move `clusterOutput` and `clusterPolicyOutput` from
  `internal/cli/cost_cluster_render.go` to `internal/engine` as
  `ClusterReport` and `ClusterPolicy`, with `NewClusterReport` replacing
  `newClusterOutput`. JSON tags are unchanged.
- **Rationale**: `internal/webui` cannot import `cli`. `engine` already
  defines the report's parts (`ClusterGroup`, `CapacityGroup`,
  `PricedSummary`) and other JSON contracts the dashboard returns.
- **Alternatives**: a new `clusterreport` package (one more package for one
  struct); a webui-side copy (two definitions drift, breaks parity).

## R2. Command-free builder and error typing

- **Decision**: `parseClusterQuery(ctx, clusterQuery) (clusterInput, error)`
  validates group-by, selectors, window, and the policy (file path for the
  CLI, text for the web). `buildClusterReport(ctx, clients, pricer, input)`
  selects plugins, checks a supplied policy, runs
  `engine.RunClusterAllocation`, and groups.
  `buildClusterPolicy(ctx, clients, input)` is `--show-policy`. Every
  failure the CLI exits 2 on is wrapped in `*engine.ClusterInputError`; a
  missing capability is `*engine.ClusterPluginMissingError`. The CLI maps
  the first through `toValidationError` with the inner error's message, so
  text and exit codes do not change.
- **Rationale**: one classification serves both surfaces; the CLI keeps its
  ax error envelope; the web maps the same types to 400 and "unavailable".
- **Alternatives**: classify by error text (brittle); return ax errors and
  sniff exit codes in the web (ties webui to ax internals and the CLI).
- **Ordering note**: the engine is built before plugin selection so the
  builder can take a pricer. A bad usage profile now fails before a plugin
  selection error when both are present; output for a single fault is
  unchanged.

## R3. Policy from the page

- **Decision**: The web takes `policy` text, parsed by a new
  `config.ParseAllocationPolicy(ctx, reader, source)` that shares the
  HuJSON parsing, size limit, and error rewriting with the file path. Its
  source label is `request body`. With no text, the web resolves the
  discovered policy like the CLI without `--policy`.
- **Rationale**: the forecast hook set the rule that the server never opens
  a path from the page; `--allowed-host` and `--trusted-proxy` let other
  tailnet users reach the dashboard. Text keeps parity for `--policy`.
- **Alternatives**: the issue's `policyPath` (arbitrary file read by any
  dashboard client); no policy input (loses `--policy` parity).

## R4. "Unavailable" answer

- **Decision**: HTTP 200 with `{"status":"unavailable","code":
  "cluster_plugin_missing","capability":"usage_stats","error":"<CLI
  message>"}`. A successful report carries `"status":"ok"`.
- **Rationale**: the issue requires a typed status, not a 5xx; this matches
  the estimate baseline's `status: "unavailable"` marker.
- **Alternatives**: 404 or 503 (reads as a server fault); 409 (implies a
  retry can succeed).

## R5. Display strings

- **Decision**: `viewmodel.BuildClusterReportDisplay(report)` returns
  money strings (`FormatMoney` with the report currency), shares to one
  decimal, the mode line, `source · digest[:12]`, summarized warnings, and
  incomplete lines. The CLI table calls the same viewmodel helpers for the
  mode line, warning grouping, and incomplete lines; its `%.2f` amounts are
  unchanged.
- **Rationale**: FR-004; one definition of the text both surfaces show.
- **Alternatives**: format in JavaScript (forbidden by FR-004 and the
  layout rules).

## R6. Engine, clients, and concurrency

- **Decision**: `webData.cluster` takes the query gate through
  `queryEngine`, reads `d.clients` under it, and passes the engine as the
  pricer. The policy query takes the same gate.
- **Rationale**: same pattern as forecast and recommendation history; the
  engine already carries the usage profile; a reload swaps both together.

## R7. Parity test

- **Decision**: In `internal/cli`, start fake `UsageSourceService` and
  `AllocatorService` servers over `bufconn`, build a `webData` with those
  clients and a fake pricer, and compare the JSON `renderClusterResult`
  writes with the body `POST /api/cluster/query` returns (minus `status`
  and `display`) for run-rate, windowed, and scoped requests.
- **Rationale**: exercises the real handler, redaction, and builder; the
  CLI path differs only in plugin discovery, which the existing
  `TestCostCluster_*` binary tests cover.

## R8. No server path in a web answer

- **Decision**: A discovered policy file is named by its tier
  (`project allocation.hujson`, `global allocation.hujson`) in the web
  answer's `policy.source` and in any parse error, through
  `config.ResolveDiscoveredAllocationPolicy`. The CLI keeps the path.
- **Rationale**: the dashboard already hides server paths (`publicValidation`
  clears `File`; config write errors become a generic 500). This is the only
  field where web and CLI JSON may differ, and only when a policy file is
  discovered.

## R9. Group keys are data, not property names

- **Decision**: `engine.ClusterGroup.Key` and
  `viewmodel.ClusterGroupDisplay.Key` carry `datakey:"true"`. The redactor
  (`sensitiveRecord`) skips a record's `key` or `property` name when the Go
  struct behind it tags that field and the field's string equals the decoded
  value; otherwise the old rule stands.
- **Rationale**: `RedactJSON` drops any array record whose `key` looks like a
  hidden property name. That removed the `__idle__` and `__cluster__` groups,
  and would remove a namespace such as `vault-secrets`, so the groups no
  longer summed to the total. The tag extends the existing data-key mark to
  record names; it is generic, not a payload-specific case.

## R10. No generic query deadline

- **Decision**: Cluster queries start through `Session.beginQueryWithin(ctx,
  0)`: no 2-minute `queryTimeout`. Request and session cancellation still
  apply.
- **Rationale**: every cluster plugin call carries its own deadline
  (`engine.ClusterCallTimeouts`: GetStats up to 30 minutes for a long
  window). The generic cap would fail a window the CLI completes.
