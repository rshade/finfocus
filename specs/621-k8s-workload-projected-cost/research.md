# Research: Kubernetes Workload Projected Cost

**Feature**: `621-k8s-workload-projected-cost` | **Date**: 2026-10-04

Facts below were read from core at `b9fdc2e` and finfocus-spec `v0.7.3`.

## R1. How a plugin's "why not" reaches the user

- **Decision**: The kubernetes plugin evaluates the workload in `Supports` and
  declines with an actionable reason when it cannot price it. `GetProjectedCost`
  runs the same evaluation and returns the price.
- **Rationale**: `NO_COST_DATA` is produced only by `projectedFallbackResult`
  (`internal/engine/pricing_spec.go:65-123`), and the only plugin text it keeps
  is a `Supports` decline reason (`engine.go:270-301`, `declineNotes`,
  truncated to 160 characters, at most 3 plugins). A `GetProjectedCost` gRPC
  error of any code becomes `ErrNoCostData` and its text is dropped
  (`adapter.go:1394-1399`, `engine.go:1786`). `GetProjectedCostResponse` has
  no "no price" field, and a `$0` response is a real price.
- **Alternatives considered**: Make core keep the text of a `GetProjectedCost`
  error as a note. Rejected: it invents a host convention the spec does not
  define, and transport errors would leak into notes. `Supports.reason` is the
  spec's channel for exactly this.

## R2. `Supports` must see attributes, and its cache must key on them

- **Decision**: `checkPluginSupports` sends `attributes` on its descriptor, and
  its cache key gains `attrs-<digest>` when attributes are present.
- **Rationale**: The cache key is
  `plugin:provider:type:region:sku:feature` (`engine.go:246`), held per engine
  run. Every Deployment has an empty SKU and region, so without the digest the
  first Deployment's answer would be reused for all of them. Both the
  per-resource path and the batch grouping (`engine_batch.go:119`,
  `selectPluginMatchesForResource` → `filterUnsupportedPlugins`) consult
  `Supports`, so one change covers both.
- **Cost**: Nearly every resource has properties, so nearly every resource
  has attributes. `Supports` is then called about once per distinct resource
  per plugin instead of once per type, SKU, and region; identical resources
  still share. `Supports` is a local RPC. This is accepted and measured by a
  benchmark.
- **Alternatives considered**: A digest only for plugins that "care". Rejected:
  core would need to know which plugins read attributes, which is plugin
  knowledge in core.

## R3. Building `attributes` (core, generic)

- **Decision**: A new `internal/engine/attributes.go` builds a
  `*structpb.Struct` from `ResourceDescriptor.Properties`:
  - drops any key, at any depth, that `skipDottedSegment` rejects (`__` prefix
    and the credential list in `flatten.go:134-150`);
  - drops any value that is a Pulumi secret (a map holding
    `4dabf18193072939515e22adb298388d`), using `history.IsPulumiSecret`
    (exported from `internal/history/cost_plan.go:219`; `engine` already
    imports `history`);
  - drops the top-level key `ref` and keys starting `ref.`, which are
    core-written reference tags (`refs.go:89-95`), not declared properties;
  - keeps Pulumi unknown values (`04da6b54-…`) so a plugin can say "not known
    until deployment";
  - keeps `tags`, `tagsAll`, `labels`, and `annotations`; those containers are
    only skipped for dotted tags to save tag count;
  - omits a value that `structpb.NewValue` cannot convert, logging at debug;
  - returns nil, with a warning, when `proto.Size` exceeds
    `pluginsdk.MaxAttributesBytes` (65536).
- **Rationale**: v0.7.3 makes host redaction REQUIRED with exactly these
  classes. Reusing `skipDottedSegment` keeps one credential list for tags and
  attributes. The secret check is new: core does not check secrets for tags
  either (see "Out-of-scope findings").
- **Alternatives considered**: Copy the secret check into engine. Rejected:
  two copies of one Pulumi rule drift.

## R4. Which RPCs carry attributes

- **Decision**: Projected cost (single `GetProjectedCost` and projected
  `BatchCost`), `Supports`, and `GetPricingSpec` (fallback and estimate
  discovery). Not actual cost (no descriptor is sent), not recommendations,
  not `EstimateCost` (already structured).
- **Rationale**: The spec says hosts send it on `ResourceDescriptor`;
  `Supports` and `GetPricingSpec` describe the same resource as
  `GetProjectedCost`, so a plugin's answers must agree. Recommendations have
  their own privacy model (`scoring.identifier_mode`, `field_allowlist`); adding
  a new data channel there needs its own decision.
- **Plumbing**: `proto.ResourceDescriptor` (`internal/proto/adapter.go:451-457`)
  gains `Attributes *structpb.Struct`; `PrepareProjectedDescriptor`
  (`internal/proto/refs.go:20-51`) copies it onto `pbc.ResourceDescriptor`.
  Callers: `getProjectedCostFromPlugin` (`engine.go:1730-1745`),
  `buildBatchCostRequest` (`engine_batch.go:306-312`, projected query only),
  `checkPluginSupports` (`engine.go:243-263`), `fetchPluginPricingSpec`
  (`pricing_spec.go:186-193`), `pricing_discovery.go:77-78`. Diff sides already
  arrive as separate `ResourceDescriptor`s (`diff.go:58-137`), so each side gets
  its own attributes with no diff-specific code. The unused
  `proto.GetProjectedCostWithErrors` (`adapter.go:106-211`) is left alone.

## R5. Cache key

- **Decision**: Append `/attrs-<digest>` after `/tags-…` and `/refs-…` in
  `generateProjectedCostResourceKey` (`engine.go:4013-4024`) when attributes are
  non-nil. Digest: SHA-256 of `proto.MarshalOptions{Deterministic: true}`
  output, hex of the first 8 bytes, the same width as `tagCacheSuffix`
  (`flatten.go:174-187`).
- **Rationale**: FR-006. The tag digest cannot see values below the depth cap.
  `ProjectedResourceCacheKey` calls the same function, so the exported key
  stays the default key. Every resource with properties gets the suffix, so
  existing cache entries miss once after upgrade; the cache is optional
  (Persistence Model), so that is a one-time cost, not a correctness issue.

## R6. Splitting BatchCost by size

- **Decision**: After `chunkResources` (`engine_batch.go:78-94`) splits by
  count, split each chunk again so its descriptors' summed `proto.Size` stays
  under `maxBatchRequestBytes = 3 << 20` (3 MiB). A resource always fits in a
  chunk on its own (64 KiB attributes plus tags is far under the budget).
- **Rationale**: Core dials gRPC only, with no `MaxCall*MsgSize` option
  (`internal/pluginhost/stdio.go:129-130`, `process.go:606-607`), so the 4 MiB
  grpc-go default applies. 100 resources × 64 KiB is 6.4 MiB. The 1 MiB
  headroom covers the envelope and tags. Connect/HTTP is not used by core.
- **Alternatives considered**: Raising `MaxCallSendMsgSize`. Rejected: the
  plugin server's receive limit is its own, and v0.7.3 tells hosts to split.

## R7. Plugin configuration

- **Decision**: Four environment variables, read once at startup into a
  `Config` passed to `kubernetes.New`:

  | Variable | Meaning |
  | --- | --- |
  | `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE` | USD per vCPU-hour |
  | `FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE` | USD per GiB-hour |
  | `FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT` | pods per DaemonSet |
  | `FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH` | run hours per Job and CronJob |

- **Rationale**: Plugins inherit the host environment
  (`internal/pluginhost/process.go:507`); `FINFOCUS_RECORDER_*` sets the
  naming precedent. A bad value never stops the plugin, since usage and
  allocation do not depend on it; it makes projected pricing decline with a
  reason naming the variable.
- **Validation**: rates finite and `>= 0` (a `0` rate is an explicit free
  price); node count an integer `>= 1`; hours finite, `> 0`, `<= 744` (the
  longest month).

## R8. Reading the workload

- **Decision**: Use `pluginsdk.AttributeValue` for paths and
  `k8s.io/apimachinery/pkg/api/resource.ParseQuantity` for quantities, which
  reports a bad value by its path (`apimachinery` and `k8s.io/api` are
  already dependencies of `plugins/kubernetes`).

  | Kind | Pod spec path | Pod count |
  | --- | --- | --- |
  | Deployment, StatefulSet | `spec.template.spec` | `spec.replicas`, default 1 |
  | DaemonSet | `spec.template.spec` | node-count variable |
  | Job | `spec.template.spec` | `spec.parallelism`, default 1 |
  | CronJob | `spec.jobTemplate.spec.template.spec` | `spec.jobTemplate.spec.parallelism`, default 1 |

- **Effective pod request**: reuse `usage.EffectiveRequests`
  (`plugins/kubernetes/usage/requests.go:21`), the rule the live `cost
  cluster` path already uses (init containers, sidecars, pod-level resources,
  overhead), so the static and live paths cannot diverge. The plugin builds a
  minimal `corev1.PodSpec` from the attributes (containers, init containers,
  their `resources` and `restartPolicy`, and pod-level `resources`) instead of
  unmarshalling the whole spec, so an unknown value in an unrelated field
  cannot fail the decode. A live pod's requests are already defaulted by the
  API server; a plan's are not, so the plugin first copies a container's limit
  into its request when the request is unset, as Kubernetes defaulting does.
  Pod overhead is set at admission and is absent from a plan.
- **No requests**: when neither CPU nor memory is declared on any container,
  decline. When one is declared, the other counts as 0 and the note says so.
- **Unknowns**: a Pulumi unknown at a count or quantity path declines with
  "not known until deployment".
- **Kinds**: exactly the five type tokens in FR-009. Any other type, and any
  resource whose provider is not `kubernetes`, declines with today's reason
  (`kubernetes plugin provides usage and allocation only`), as the issue's
  "exactly as today" criterion requires.

## R9. Result shape

- **Decision**: `cost_per_month = pods × podHourly × hours`; `unit_price =
  podHourly`; `currency = "USD"`; `billing_detail` is the note (FR-015). Info
  adds `PLUGIN_CAPABILITY_PROJECTED_COSTS` only; no batch capability, so core
  calls `GetProjectedCost` per resource.
- **Rationale**: The engine copies `billing_detail` into `Notes`
  (`engine.go:1747-1757`). A `$0` cost from `replicas: 0` or a `0` rate is a
  real price.

## R10. Spec version bump

- **Decision**: Move `go.mod`, `plugins/kubernetes/go.mod`, and
  `plugins/jev/go.mod` to v0.7.3. Add hop `v0.7.3` to
  `internal/pluginupgrade/hops.go` with guide
  `agent-skills/finfocus-plugin-upgrade/references/to-v0.7.3.md` ("additive;
  no changes required; optional `attributes`"), and rebuild
  `finfocus-plugin-upgrade.skill`. The `pluginupgrade/testdata` pins are
  fixtures and stay.
- **Rationale**: `TestHopsReachCoreSpecVersion` and `TestHopsMatchGuides`
  require it.

## Out-of-scope findings (report, do not fix here)

- `ConvertToProto` (`engine.go:1971-1978`) emits every top-level input as a
  collapsed tag, including credential-named keys such as `password`; only
  dotted keys are filtered.
- `ConvertValueToString` (`engine.go:1983-2024`) turns a Pulumi secret map
  into `fmt` text that includes its ciphertext, or returns its plaintext
  `value`.
- Both predate this feature and affect tags, not attributes. Each needs its
  own issue.
