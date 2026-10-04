# Tasks: Kubernetes Workload Projected Cost

**Input**: Design documents from `specs/621-k8s-workload-projected-cost/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: TDD (Constitution Principle II). Each test task comes before its
implementation task and must fail first. Testify `require`/`assert` only.
Unit tests are colocated. Tests call `t.Parallel()` unless they use
`t.Setenv` or exec a freshly written binary (see CLAUDE.md "Parallel Tests").

**Completeness**: No stubs or TODOs (Principle VI).

**Story order**: US5 (attributes) is first because US1–US4 read them.

## Phase 1: Setup

- [X] T001 Bump `github.com/rshade/finfocus-spec` to v0.7.3 in `go.mod`, `plugins/kubernetes/go.mod`, and `plugins/jev/go.mod`, and run `go mod tidy` in each module (leave `internal/pluginupgrade/testdata/*/go.mod` fixtures alone)
- [X] T002 Add hop `{To: "v0.7.3", Summary: "Additive release; ResourceDescriptor.attributes; tag values up to 2048 bytes", Guide: "to-v0.7.3.md"}` to `internal/pluginupgrade/hops.go`
- [X] T003 [P] Write `agent-skills/finfocus-plugin-upgrade/references/to-v0.7.3.md` in the style of `to-v0.7.2.md`: no changes required; optional: read nested inputs with `pluginsdk.AttributeValue(req.GetResource().GetAttributes(), "path")` and fall back to tags when unset; tag values may be up to 2048 bytes; attributes are capped at 65536 bytes and hosts redact secrets
- [X] T004 List v0.7.3 in `agent-skills/finfocus-plugin-upgrade/SKILL.md` where v0.7.2 is listed, and rebuild `agent-skills/finfocus-plugin-upgrade/finfocus-plugin-upgrade.skill` from `agent-skills/` as a zip of `finfocus-plugin-upgrade/SKILL.md` and `finfocus-plugin-upgrade/references/`
- [X] T005 Run `go test ./internal/pluginupgrade/... ./internal/cli/ -run 'TestHops|TestPluginInit'` and `make build`; fix any compile errors from the bump

**Checkpoint**: All three modules build on v0.7.3; hop tests pass.

## Phase 2: Foundational

- [X] T006 Export `IsPulumiSecret(v any) bool` from `internal/history/cost_plan.go` (rename `isPulumiSecret`, update its callers and `internal/history/cost_plan_test.go`) so `engine` can share the one rule
- [X] T007 Add `Attributes *structpb.Struct` to `proto.ResourceDescriptor` in `internal/proto/adapter.go`, with a doc comment pointing to finfocus-spec's host redaction rule

**Checkpoint**: `go build ./...` passes.

## Phase 3: User Story 5 - Plugins receive a resource's full inputs (P1)

**Goal**: Every projected-cost, `Supports`, and `GetPricingSpec` descriptor
carries redacted, size-bounded `attributes`; caches and batches account for
them.

**Independent Test**: Build the request for a resource with a 10-level input,
a `password` key, a `__defaults` key, a Pulumi secret, and `ref.*` keys;
assert the plugin sees the full nested path and none of the others.

### Tests for User Story 5

- [X] T008 [P] [US5] Table tests for `BuildAttributes` in `internal/engine/attributes_test.go`: nested maps and lists kept; credential-like keys dropped at any depth (`password`, `dbPassword`, `api_key`, nested `secretRef.token`); `__` keys dropped at any depth; a value holding `4dabf18193072939515e22adb298388d` dropped; top-level `ref` and `ref.x.urn` dropped but nested `ref` kept; `tags`/`labels`/`annotations` kept; Pulumi unknown `04da6b54-80e4-46f7-96ec-b56ff0331ba9` kept; an unconvertible value (a Go `chan`) omitted without error; nil and empty input return nil; a map whose encoding exceeds `pluginsdk.MaxAttributesBytes` returns nil and logs a warning; an input exactly at the cap is kept
- [X] T009 [P] [US5] Tests for `attrsCacheSuffix` in `internal/engine/attributes_test.go`: deterministic across runs and map iteration orders; differs when a value 8 levels deep differs; empty for nil
- [X] T010 [P] [US5] Tests in `internal/proto/refs_test.go` that `PrepareProjectedDescriptor` copies `Attributes` onto `pbc.ResourceDescriptor.Attributes` and leaves `Tags` unchanged
- [X] T011 [P] [US5] Tests beside the existing `ProjectedResourceCacheKey` tests (locate with `rg -l ProjectedResourceCacheKey internal/engine/*_test.go`): two resources equal in tags but different below the 6-segment depth cap get different keys; identical inputs give the same key across runs and map orders; a resource with no properties has no `/attrs-` suffix; `/attrs-` sits after `/refs-` and before `/pricing-spec`
- [X] T012 [P] [US5] Tests beside the existing `checkPluginSupports` tests (locate with `rg -l checkPluginSupports internal/engine/*_test.go`) using a fake `Supports` client: the request carries `Attributes`; two Deployments with different attributes cause two `Supports` calls; two identical ones cause one; a resource without attributes keeps the old cache key
- [X] T013 [P] [US5] Tests in `internal/engine/engine_batch_test.go`: projected `buildBatchCostRequest` sets `Attributes` and actual does not; a chunk of 100 resources with 60 KiB attributes each splits so every request's summed descriptor `proto.Size` is under `maxBatchRequestBytes`; order and original indices are preserved; a single resource always forms a chunk; the plugin's `max_batch_size` re-chunking still applies
- [X] T014 [P] [US5] Tests in `internal/engine/pricing_spec_test.go` and `internal/engine/pricing_discovery_test.go` that `GetPricingSpec` requests carry `Attributes`
- [X] T015 [P] [US5] Test in `internal/engine/diff_test.go` that an update prices each side with its own attributes and a delete uses the old properties' attributes

### Implementation for User Story 5

- [X] T016 [US5] Implement `BuildAttributes(ctx, props map[string]any) *structpb.Struct` and `attrsCacheSuffix(*structpb.Struct) string` in `internal/engine/attributes.go` per research R3 and R5 (reuse `skipDottedSegment`, `history.IsPulumiSecret`; deterministic marshal; SHA-256, first 8 bytes hex)
- [X] T017 [US5] Copy `Attributes` in `PrepareProjectedDescriptor` in `internal/proto/refs.go` and pass it through `clientAdapter.GetProjectedCost` in `internal/proto/adapter.go`
- [X] T018 [US5] Set `Attributes: BuildAttributes(ctx, resource.Properties)` in `getProjectedCostFromPlugin` in `internal/engine/engine.go`
- [X] T019 [US5] In `checkPluginSupports` in `internal/engine/engine.go`, send `Attributes` on the `Supports` descriptor and append `attrs-<digest>` to the cache key when attributes are present; update the `supportsCache` field comment
- [X] T020 [US5] Append `/attrs-<digest>` in `generateProjectedCostResourceKey` in `internal/engine/engine.go` after the tags and refs suffixes
- [X] T021 [US5] Set `Attributes` for projected queries in `buildBatchCostRequest` and add byte-budget splitting (`maxBatchRequestBytes = 3 << 20`) after `chunkResources` in `internal/engine/engine_batch.go`, with a comment citing the 4 MiB grpc-go default
- [X] T022 [US5] Set `Attributes` in `fetchPluginPricingSpec` in `internal/engine/pricing_spec.go` and in `internal/engine/pricing_discovery.go`
- [X] T023 [US5] Add `internal/engine/attributes_bench_test.go` with `BenchmarkBuildAttributes` (build plus digest for a 40 KiB pod spec) and `BenchmarkSupportsCacheKeys` (count of distinct `Supports` cache keys before and after attributes for `examples/plans/aws-simple-plan.json`, reported with `b.ReportMetric`)
- [X] T024 [US5] First check whether the recorder advertises `batch_cost` (`plugins/recorder/plugin.go`); `BatchCost` is not recorded, so assert on the RPC it actually receives. Extend the recorder integration test in `test/integration/recorder_test.go` with a fixture resource holding a nested input, a `password`, and a Pulumi secret; assert the recorded `GetProjectedCost` request has the nested path in `attributes` and no password or secret value

**Checkpoint**: `go test -race ./internal/engine/... ./internal/proto/... ./internal/history/...` passes; US5 is complete on its own.

## Phase 4: Foundational for the plugin (blocks US1, US2, US4)

- [X] T025 [P] Tests for `LoadConfig(getenv func(string) string) Config` in `plugins/kubernetes/config_test.go`: unset variables; valid rates including `0`; negative, `NaN`, `Inf`, and non-numeric rates rejected with the variable name; node count `0`, `-1`, `1.5` rejected; hours `0`, `745` rejected and `744` accepted
- [X] T026 Implement `Config` and `LoadConfig` in `plugins/kubernetes/config.go` per data-model.md (each field: value, set, error)
- [X] T027 [P] Table tests for `workload.ReadPodSpec(attrs, path)` in `plugins/kubernetes/workload/podspec_test.go`: one container; two containers summed through `usage.EffectiveRequests`; limits copied into absent requests (and `UsedLimits` set); init container larger than the app sum; a sidecar (`restartPolicy: Always`) added to the app sum; pod-level `resources.requests` overriding; `500m`, `0.5`, `2`, `1Gi`, `1G`, `512Mi`, `1e9`; an invalid quantity reported with its path (`spec.template.spec.containers.1.resources.requests.memory`); a Pulumi unknown in a quantity reported as unknown; an unknown in an unrelated field (`terminationGracePeriodSeconds`) ignored; no containers; CPU only and memory only declared
- [X] T028 Implement `ReadPodSpec` in `plugins/kubernetes/workload/podspec.go`: walk containers and init containers with `pluginsdk.AttributeValue`, parse quantities with `resource.ParseQuantity`, build a minimal `corev1.PodSpec`, apply limit→request defaulting, and call `usage.EffectiveRequests`

**Checkpoint**: `make test-kubernetes` passes.

## Phase 5: User Story 1 - Price a declared Deployment (P1)

**Goal**: Deployments and StatefulSets with rates configured are priced from
declared requests, with notes naming the method.

**Independent Test**: The Deployment in the fixture plan costs `54.75` USD with
rates 0.04 and 0.005, and its notes name the method and rate source.

### Tests for User Story 1

- [X] T029 [P] [US1] Table tests for `workload.Estimate(desc, cfg)` priced paths in `plugins/kubernetes/workload/estimate_test.go`: the 54.75 worked example; replicas absent → 1 pod with "default" source; `replicas: 0` → priced `$0` with a scaled-to-zero note; two containers; StatefulSet same as Deployment; a `0` rate priced; `unit_price` is the pod-hourly; note text names the method, the pod count and source, both variables, and "not a real node price"
- [X] T030 [P] [US1] Tests in `plugins/kubernetes/plugin_test.go`: `Info` lists `PLUGIN_CAPABILITY_PROJECTED_COSTS` with the existing two and no other pricing capability; `Supports` returns `Supported: true` for a priceable Deployment; `GetProjectedCost` returns `cost_per_month`, `unit_price`, `currency: "USD"`, and `billing_detail` matching `Estimate`; the plugin is built with a `ClusterFactory` that fails the test when called, proving projected pricing never touches a cluster (FR-016)
- [X] T031 [P] [US1] Add a round-trip case to `plugins/kubernetes/plugin_roundtrip_test.go` that serves the plugin over gRPC and prices a Deployment descriptor with attributes

### Implementation for User Story 1

- [X] T032 [US1] Implement `Estimate` in `plugins/kubernetes/workload/estimate.go` for Deployment and StatefulSet (`kubernetes:apps/v1:Deployment`, `kubernetes:apps/v1:StatefulSet`; pod count from `spec.replicas`, default 1; 730 hours)
- [X] T033 [US1] Wire `plugins/kubernetes/plugin.go`: `New` takes a `Config`; `Info` adds `PLUGIN_CAPABILITY_PROJECTED_COSTS`; `Supports` and `GetProjectedCost` call `Estimate` (a decline from `GetProjectedCost` returns `codes.FailedPrecondition` with the reason)
- [X] T034 [US1] Load `Config` with `os.Getenv` in `plugins/kubernetes/cmd/main.go` and log (debug) which variables are set, never their values beyond rates
- [X] T035 [US1] Add `examples/plans/k8s-workloads-plan.json`: a `pulumi preview --json` shaped plan with a Deployment (3 replicas, 500m/1Gi), a StatefulSet, a DaemonSet, a Job, a CronJob, a `kubernetes:core/v1:ConfigMap`, and an `aws:ec2/instance:Instance`, with inputs nested under `newState` as real previews do
- [X] T036 [US1] Integration test `TestKubernetesPlugin_PricesDeclaredWorkloads` in `test/integration/cost_cluster_test.go`: build and install the plugin (reuse the existing build helper), set the two rate variables with `t.Setenv`, run `cost projected --pulumi-json ../../examples/plans/k8s-workloads-plan.json --output json`, and assert the Deployment's monthly cost is 54.75 and its notes name the method; factor the build/install steps shared with `TestKubernetesPlugin_DoesNotPolluteCostProjected` into one helper in that file

**Checkpoint**: US1 independently demonstrable.

## Phase 6: User Story 2 - Be told why a workload is not priced (P1)

**Goal**: Every unpriceable workload is `NO_COST_DATA` with an actionable note.

**Independent Test**: With no rate variables set, every fixture workload shows
`NO_COST_DATA` and a note naming the missing variable; none shows `$0`.

### Tests for User Story 2

- [X] T037 [P] [US2] Decline-path table tests in `plugins/kubernetes/workload/estimate_test.go`, in the data-model decline order: no attributes; unknown replicas; unknown request; invalid quantity (names the path); no requests declared; CPU rate unset; memory rate unset; an invalid rate (names the variable and the parse error); every reason at most 160 characters
- [X] T038 [P] [US2] Tests in `plugins/kubernetes/plugin_test.go` that `Supports` returns `Supported: false` with the `Estimate` reason, and `GetProjectedCost` returns `codes.FailedPrecondition`, for the no-rate case
- [X] T039 [P] [US2] Integration test `TestKubernetesPlugin_ExplainsUnpricedWorkloads` in `test/integration/cost_cluster_test.go`: no rate variables; assert each workload is `NO_COST_DATA`, its note contains `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE` and `finfocus cost cluster`, and no workload reports a `0` monthly cost

### Implementation for User Story 2

- [X] T040 [US2] Implement the decline paths in `plugins/kubernetes/workload/estimate.go`, each reason naming its fix (variable, field path, or "set resource requests"), with "not known until deployment" for unknowns and "core sent no inputs; upgrade finfocus" for missing attributes; keep every reason within 160 characters (when both rates are unset, name the shared `FINFOCUS_KUBERNETES_*_HOURLY_RATE` pair compactly rather than spelling out both variables)

**Checkpoint**: US1 and US2 together; no `$0` without a real basis.

## Phase 7: User Story 3 - Leave everything else unchanged (P1)

**Goal**: Only the five kinds change; other types and providers behave as
before.

**Independent Test**: The ConfigMap is still declined with today's reason;
the EC2 instance's result is identical with and without the plugin's new
behavior.

### Tests for User Story 3

- [X] T041 [P] [US3] Tests in `plugins/kubernetes/plugin_test.go` that `Supports` declines `kubernetes:core/v1:ConfigMap`, `kubernetes:core/v1:Service`, `kubernetes:apps/v1:ReplicaSet`, `kubernetes:apps/v1:DeploymentPatch`, and an `aws:ec2/instance:Instance` with exactly `kubernetes plugin provides usage and allocation only`
- [X] T042 [US3] Extend `TestKubernetesPlugin_DoesNotPolluteCostProjected` in `test/integration/cost_cluster_test.go` (keep its AWS assertions) and add a case on the k8s fixture asserting the ConfigMap's note contains `declined by kubernetes` and the EC2 instance's JSON entry equals the entry from a run with no kubernetes plugin installed

### Implementation for User Story 3

- [X] T043 [US3] Make `Estimate` decline any type outside the five tokens, and any provider other than `kubernetes`, with today's reason, before reading attributes, in `plugins/kubernetes/workload/estimate.go`

**Checkpoint**: No regression for other resources.

## Phase 8: User Story 4 - DaemonSets, Jobs, and CronJobs (P2)

**Goal**: Hint-based pricing for the three kinds; `NO_COST_DATA` without the hint.

**Independent Test**: With hints set, each kind is priced to the cent;
without, each is `NO_COST_DATA` naming its hint.

### Tests for User Story 4

- [X] T044 [P] [US4] Table tests in `plugins/kubernetes/workload/estimate_test.go`: DaemonSet with node count 4 → 4 pods, note names the variable; DaemonSet without → decline naming `FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT`; Job with `parallelism: 2` and 10 hours → `2 × podHourly × 10`; Job without `parallelism` → 1 pod; CronJob reading `spec.jobTemplate.spec.template.spec` and `spec.jobTemplate.spec.parallelism`; Job or CronJob without hours → decline naming `FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH`
- [X] T045 [P] [US4] Extend `TestKubernetesPlugin_PricesDeclaredWorkloads` in `test/integration/cost_cluster_test.go` with both hint variables set and asserting the DaemonSet, Job, and CronJob costs

### Implementation for User Story 4

- [X] T046 [US4] Implement DaemonSet, Job, and CronJob in `Estimate` in `plugins/kubernetes/workload/estimate.go` per research R8

**Checkpoint**: All five kinds behave as specified.

## Phase 9: Polish

- [X] T047 [P] Document projected cost in `plugins/kubernetes/README.md`: the four variables, the formula and worked example, the five kinds, decline reasons, and that `cost cluster` is the authoritative live number
- [X] T048 [P] Add a "Price workloads declared in a Pulumi plan" section to `docs/src/content/docs/guides/cluster-costs.md` linking the README variables and the quickstart flow
- [X] T049 [P] Add CLAUDE.md gotchas: core sends redacted `attributes` (rules, cap, cache and Supports keys, 3 MiB batch budget) under Pulumi Integration Notes; kubernetes plugin projected pricing (env vars, decline via Supports, reuse of `usage.EffectiveRequests`) under Cluster allocation; finfocus-spec version references to v0.7.3
- [X] T050 [P] Note the v0.7.3 bump in `ROADMAP.md` only if the roadmap tracks spec versions (check first; do not touch the user's uncommitted `ROADMAP.md` in the main checkout)
- [X] T051 Set spec status to `Implemented` in `specs/621-k8s-workload-projected-cost/spec.md` and tick every task here
- [X] T052 Run `make validate`, `make test`, `make test-race`, `make test-kubernetes`, `make lint` (extended timeout), `make docs-lint`, markdownlint on changed Markdown outside `docs/`, `make check-plugin-boundaries`, and `go test -race -shuffle=on -count=3` on `internal/engine` and `plugins/kubernetes/...`; measure coverage with `go test -coverprofile` and report it: new code in `internal/engine/attributes.go` and the changed engine paths at 95% or more (critical path), `plugins/kubernetes/workload` and `plugins/kubernetes/config.go` at 80% or more; review `git diff -- internal/` for Kubernetes workload terms (Deployment, replicas, requests) outside test fixtures and record the result (SC-005, FR-008)
- [X] T053 Run the integration tests touched here: `go test -run 'TestKubernetesPlugin|TestRecorder' ./test/integration/...`
- [X] T054 Run the quickstart against the built binary with `examples/plans/k8s-workloads-plan.json` and read the table output for the five workloads, the ConfigMap, and the instance

## Dependencies

- Phase 1 → Phase 2 → US5 (Phase 3).
- Phase 4 depends only on Phase 1 (plugin module) and can run beside US5.
- US1 needs US5 (attributes on the wire) for its integration test (T036) and Phase 4; its unit tests need only Phase 4.
- US2 and US3 build on US1's `Estimate` and wiring.
- US4 builds on US1.
- Polish follows all stories.

## Parallel Examples

- US5 tests T008–T015 touch different files and can be written together.
- Phase 4 (T025–T028) can proceed in parallel with US5 implementation.
- US1 tests T029–T031 can be written together; T047–T049 docs together.

## Implementation Strategy

MVP is US5 + US1: core sends attributes, and a configured Deployment is
priced. US2 makes it safe to ship (no silent `$0`). US3 is a regression
guard. US4 extends to the remaining kinds.
