# Feature Specification: Prometheus Historical Cluster Usage

**Feature Branch**: `623-prometheus-usage-source`
**Created**: 2026-10-04
**Status**: Draft
**Input**: User description: "SP4 Prometheus usage source — historical Kubernetes cluster cost (roadmap #1529). A metrics-backed usage source answers what a namespace cost over a past window. Run-rate allocation stays as delivered in `specs/613-cost-cluster/`. Datadog and OpenCost stay later."

## Overview

`finfocus cost cluster` today answers only a live question: at the current
requests and the current monthly node price, what is each workload's run-rate?
Kubernetes keeps no usage history, so a past window such as "namespace X last
week" cannot be answered from the cluster API.

This feature adds a Prometheus-backed usage source and teaches cluster
allocation to price that window from actual spend. Workloads are charged for
resource-hours they consumed. The existing allocator still splits the bill.
The live run-rate command is unchanged when the user does not ask for a window.

### Non-goals

- Datadog and OpenCost usage sources.
- Metrics that are not Kubernetes workload or node usage (virtual machines,
  application metrics). Usage rows keep the same subject keys as the run-rate
  source.
- A second allocator. Allocation policy and the split stay with the kubernetes
  plugin.
- A registry install entry before the first `prometheus-v0.1.0` release exists.
  The entry follows the same gate as the kubernetes plugin: it is added only
  once that release tag is real.
- Changing run-rate collection, run-rate pricing, or allocation policy.
- A new finfocus-spec contract. Historical mode, the window fields, and the
  usage metric names already exist.
- Vendor billing changes (spot price lists, Fargate rates). A missing actual
  price stays unpriced.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ask What a Namespace Cost Last Week (Priority: P1)

As a FinOps engineer, I want `finfocus cost cluster` over a past window, with
the Prometheus usage source selected, so that I see what each workload
actually consumed and what share of the node's bill that consumption earned.

**Why this priority**: This is the question the run-rate command cannot answer.
Without a windowed report, the source has no user-visible value.

**Independent Test**: With Prometheus holding a fixed week of usage for a known
workload, and with node spend for that same week supplied by an actual-cost
source, `finfocus cost cluster --from <start> --to <end> --usage-source prometheus`
reports groups whose costs sum to the priced window total within relative
epsilon 1e-6. The report's mode is historical and its period is that window,
not a 730-hour month.

**Acceptance Scenarios**:

1. **Given** a window and the Prometheus source, **When** the user runs
   `finfocus cost cluster --from 2026-09-01 --to 2026-09-08`, **Then** the
   report covers that window only, labels the mode historical, and allocates
   each priced node's actual spend for the window.
2. **Given** `--namespace payments` inside that window, **When** the command
   runs, **Then** workload rows are exact for that namespace and idle and
   cluster rows are omitted with the same footer notice the run-rate command
   already uses.
3. **Given** `--group-by label:team`, **When** a workload has no such label in
   the stored usage, **Then** its cost groups under `<none>` and is not dropped.
4. **Given** no window, **When** the user runs `finfocus cost cluster`, **Then**
   the report stays the current run-rate report (monthly, projected node
   prices, request-based usage). Output for that command does not change.
5. **Given** `--from` without `--to`, **When** the command runs, **Then** the
   window ends at the current time. **Given** `--to` before `--from`, or a
   window flag the date parser does not accept, **When** the command runs,
   **Then** it exits 1 before any plugin is contacted. Accepted date forms are
   `2006-01-02` and RFC3339, the same forms the other cost commands accept.

---

### User Story 2 - Measure Consumption, Not Requests (Priority: P1)

As a FinOps engineer, I want the historical report to charge what the workload
used, so that a namespace that reserved CPU it never consumed is not billed as
if it had used the reservation.

**Why this priority**: Historical billing that repeats the request snapshot
answers a different question than "what did this cost". The run-rate source
already owns requests. This source exists to add consumption.

**Independent Test**: A workload that requested 2 cores and consumed 0.5 cores
for the whole window is charged on 0.5 core-hours per hour of the window. A
counter reset in the middle of the window does not drop or double-count the
hours on either side of the reset.

**Acceptance Scenarios**:

1. **Given** a windowed stats request, **When** the Prometheus source answers,
   **Then** the response is historical and each workload row carries
   `cpu_usage` and `mem_usage` integrated over the window (`core-hours` and
   `GiB-hours`). It does not carry `cpu_request` or `mem_request`.
2. **Given** the same response, **When** node rows are included, **Then** each
   node carries allocatable CPU and memory integrated over the same window, in
   the same hour units, so a workload's share is its resource-hours divided by
   the node's available resource-hours.
3. **Given** a pod that existed for part of the window, **When** hours are
   integrated, **Then** only the time it existed is counted. A pod that never
   ran in the window produces no row.
4. **Given** a counter reset, a missing series, a namespace filter, a label
   selector, and an empty window, **When** the hour calculation is reviewed
   against its checked examples, **Then** each example states the hours that
   must be reported, and a calculation change that alters those hours fails
   the example.
5. **Given** the stored samples start after the requested start, or a series
   has a gap the integral cannot cover, **When** the source answers, **Then**
   it still returns the hours it can support, names the gap in a warning, and
   the report is marked incomplete. It does not present a short history as a
   full window.

---

### User Story 3 - Price the Window from Actual Spend (Priority: P1)

As a FinOps engineer, I want each node's charge to be what that node cost
during the window, so that the allocated total matches the bill for those
days rather than a projected month scaled by a guess.

**Why this priority**: Usage hours without a windowed price still cannot
answer "what did this cost". Run-rate pricing must stay on the no-window path.

**Independent Test**: A historical stats response is priced with the actual
cost of each priceable resource over the same start and end. The run-rate path
still prices with the projected monthly cost. A historical response is no
longer rejected as unsupported.

**Acceptance Scenarios**:

1. **Given** historical usage, **When** allocation runs, **Then** each
   priceable resource is priced with its actual cost for the requested window.
   The projected monthly price is not consulted for that run.
2. **Given** a run-rate stats response, **When** allocation runs, **Then**
   pricing stays on the projected monthly path and the result mode stays
   run-rate.
3. **Given** a source returns historical usage when the user did not ask for a
   window, or returns run-rate usage when the user did, **When** allocation
   runs, **Then** the command fails and names the mismatch. It does not
   allocate one mode's rows with the other mode's prices.
4. **Given** actual cost is missing, errors, or is `$0`, **When** that node is
   priced, **Then** it is unpriced (`priced=false`, cost 0, a note), the same
   rule as a `$0` projected price. The report is incomplete. If every node is
   unpriced, the command fails. Actual cost is never replaced with an
   on-demand projection.
5. **Given** a spot node whose actual spend is known, **When** it is priced,
   **Then** the allocated amount is that spend. The run-rate note "spot node
   priced on-demand" is not applied on top of a real bill.

---

### User Story 4 - Identify Nodes That No Longer Exist (Priority: P2)

As a platform engineer, I want a node that was replaced during the window to
remain priceable, so that last week's bill still includes machines that are
gone from the live cluster.

**Why this priority**: A live API join is enough for nodes that are still
present, and it is not enough for the windows this feature exists to explain.
The source needs a record of identity taken during the window.

**Independent Test**: A node that has no live API object, but whose instance
type, region, and provider were recorded as metrics during the window, still
produces a priceable descriptor. A node with neither a recorded identity nor a
live object produces no priceable descriptor, a warning, and an incomplete
report.

**Acceptance Scenarios**:

1. **Given** node-label metrics recorded during the window, **When** the source
   builds priceable resources, **Then** instance type, region, provider, and
   capacity type match the identity the run-rate collector would have read,
   and the priceable id equals the node subject on that node's usage rows.
2. **Given** those recorded labels are absent and the node still exists,
   **When** the source builds priceable resources, **Then** it reads the live
   cluster API with the same identity rules as the run-rate collector.
3. **Given** neither source can identify the node, **When** the report is
   built, **Then** that node is omitted from the priceable set, a warning
   names it, and the report is incomplete.
4. **Given** the metrics store holds more than one cluster and the request
   does not select one, **When** the source answers, **Then** it fails and
   names the clusters it saw. It never adds their hours together. A store that
   holds one cluster uses that cluster. A requested scope that disagrees with
   the stored cluster label fails.

---

### User Story 5 - Prove It in a Local Cluster (Priority: P2)

As a maintainer, I want the historical path proven on a local cluster with
Prometheus installed beside the existing cluster-cost end-to-end setup, so
that conservation for a known window is checked without a cloud account.

**Why this priority**: The run-rate path is already proven on a local cluster.
A historical source that cannot be tested the same way will rot. The local
cluster cannot produce a cloud bill, so the proof uses known prices and known
usage, and asserts conservation and the integrated hours.

**Independent Test**: The local-cluster job installs Prometheus into the same
cluster the run-rate end-to-end test uses, runs fixed workloads for a fixed
window (or replays a fixed window), and asserts that allocated rows conserve
the known node prices within 1e-6 and that the reported resource-hours match
the fixture.

**Acceptance Scenarios**:

1. **Given** the local cluster job, **When** it runs, **Then** Prometheus is
   installed in that cluster and the historical allocation assertion runs
   against it.
2. **Given** a node the local pricing stand-in prices at a known amount,
   **When** historical allocation completes, **Then** row totals equal that
   amount within relative epsilon 1e-6, idle included.
3. **Given** Prometheus cannot be reached, **When** the user asks for a
   window, **Then** the command fails with a connection error. It does not
   return an empty successful report.

---

### User Story 6 - Ship the Source on Its Own (Priority: P3)

As a plugin author, I want the Prometheus source to be its own plugin that
only reports usage, so that I can install it next to the kubernetes plugin
and keep using that plugin's allocator.

**Why this priority**: The split is what makes Datadog and OpenCost able to
follow the same shape. It is not required to demonstrate one local report,
which is why it ranks after the report itself.

**Independent Test**: The plugin module builds without importing finfocus core
packages. It declares usage stats only. A windowed request with end after start is served.
A request missing either bound, or with end not after start, is rejected, with
the message saying this source is historical only. The kubernetes plugin still rejects a windowed
request with its existing run-rate-only message.

**Acceptance Scenarios**:

1. **Given** both the kubernetes and Prometheus usage sources are installed,
   **When** the user does not pass `--usage-source`, **Then** selection fails
   listing the candidates, the same rule as any other ambiguous capability.
   The window is not silently routed.
2. **Given** `--usage-source kubernetes` and a window, **When** the command
   runs, **Then** it fails with the kubernetes source's existing run-rate-only
   error.
3. **Given** `--usage-source prometheus` and no window, **When** the command
   runs, **Then** the Prometheus source rejects the request as historical
   only.
4. **Given** the first `prometheus-v0.1.0` release does not exist yet,
   **When** this feature is implemented, **Then** no registry install entry is
   added. After that tag exists, the entry uses the monorepo prefix
   `prometheus-` and the canonical version `v0.1.0`, the same rule as
   `kubernetes-`.

---

### Edge Cases

- **Prometheus address missing**: outside the cluster the command fails before
  querying and names `FINFOCUS_PROMETHEUS_URL`. A bearer token, when set in
  `FINFOCUS_PROMETHEUS_BEARER_TOKEN`, is sent and never written to logs,
  errors, or fixtures.
- **In-cluster default**: when the URL is unset and the process is running
  in-cluster, the source uses the in-cluster Prometheus service. Kind and any
  other out-of-cluster run set the URL explicitly.
- **Auth or TLS failure**: the command fails. It does not fall back to an
  anonymous query or to run-rate data.
- **Empty cluster in a valid window**: valid. All priced cost is idle.
- **Usage with no priceable nodes**: incomplete, and fatal when nothing is
  priced. Same rule as run-rate.
- **Namespace or label selector**: applied to the stored rows. Idle and
  cluster rows follow the existing `--namespace` omission rule.
- **Unknown requested metric**: ignored with a warning, same as the run-rate
  source. This source does not serve request metrics.
- **Fargate and spot identity**: passed through with the same subject and
  capacity labels the allocator already understands. This feature does not
  add Fargate prices. Unpriced Fargate stays a note, not idle.
- **Control plane**: included only when the source can identify the cluster
  the same way the run-rate source does. Otherwise it is omitted with a
  warning. No new control-plane discovery.
- **Mixed currencies**: the existing mixed-currency error.
- **Slow query**: the existing plugin timeout applies.
- **Window in the future, too far in the past, or end not after start**:
  rejected before plugins are contacted, using the same date rules as
  `finfocus cost actual`. A date-only `--from` and `--to` on the same calendar
  day are both midnight UTC, so that pair is zero-length and fails. A one-day
  window is `--from` that date and `--to` the next date, or an RFC3339 end
  later on the same day.
- **Retention shorter than the window**: treated as a gap (warning,
  incomplete), not as zero usage.
- **Duplicate pod names across namespaces**: stay separate rows.
- **Plugin selected for pricing**: the Prometheus source declines pricing
  requests, so `cost projected` and `cost actual` do not call it for
  resource prices.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `finfocus cost cluster` MUST accept `--from` and `--to`. Both
  accept `2006-01-02` and RFC3339. `--to` defaults to now when `--from` is set.
  `--from` is required when `--to` is set. A reversed or zero-length window,
  or an unparseable value, MUST fail before any plugin is loaded.
- **FR-002**: With a window, the stats request MUST carry that start and end.
  Without a window, the stats request MUST omit both, and behavior MUST match
  the run-rate command as specified in `specs/613-cost-cluster/`.
- **FR-003**: Plugin selection MUST stay explicit flag, otherwise the single
  installed plugin that declares usage stats. Zero candidates and several
  candidates remain fatal and list the candidates. A window MUST NOT change
  that rule.
- **FR-004**: The Prometheus source MUST be a separate plugin that reports
  usage only. It MUST NOT allocate. Cluster allocation MUST keep using the
  installed allocator (the kubernetes plugin).
- **FR-005**: The Prometheus source MUST reject a stats request that is missing
  either bound, or whose end is not after its start, and MUST answer a request
  that has both bounds in order with historical mode. The kubernetes source
  MUST keep rejecting a request that has either bound, with its current
  run-rate-only error.
- **FR-006**: Historical workload rows MUST use `cpu_usage` and `mem_usage`
  only, integrated as `core-hours` and `GiB-hours` over the window. They MUST
  NOT include request metrics. Because the allocator charges
  `max(request, usage)`, usage-only rows charge consumption.
- **FR-007**: Historical node rows MUST use allocatable CPU and memory
  integrated over the same window and the same hour units. A workload's share
  of a node is its resource-hours over the node's available resource-hours.
  Subject keys MUST stay the run-rate set (`cluster`, `namespace`,
  `controller_kind`, `controller`, `pod`, `node`, `label.<key>`, `kind`).
- **FR-008**: Integration MUST count only time the subject existed, MUST sum
  across a counter reset without dropping or double-counting either side, and
  MUST honor the namespace and label selector. Checked examples MUST cover
  counter reset, partial lifetime, missing series, namespace filter, label
  selector, and an empty window. A change that alters the hours in an example
  MUST fail that example.
- **FR-009**: When stored samples do not cover the requested window, the
  source MUST return the hours it can support, MUST warn with the gap, and
  the report MUST be incomplete. A store that cannot be reached MUST fail the
  command rather than return an empty success.
- **FR-010**: Node identity (instance type, region, provider, capacity type)
  MUST come from node-label metrics recorded during the window. When those
  metrics are absent and the node still exists, identity MUST come from the
  live cluster API using the run-rate collector's rules. When neither works,
  the node MUST be omitted from the priceable set with a warning, and the
  report MUST be incomplete. The priceable id MUST equal the node subject.
- **FR-011**: The source MUST limit rows to the requested scope. Multiple
  clusters in one store without a selected scope MUST fail and name those
  clusters. One cluster MUST be used as that scope. A scope that disagrees
  with the stored cluster label MUST fail.
- **FR-012**: Historical allocation MUST price each priceable resource with
  actual cost over the same start and end. It MUST NOT use projected monthly
  cost on that path. Run-rate allocation MUST keep using projected monthly
  cost. A mode mismatch between the user's window and the stats response MUST
  fail before allocation.
- **FR-013**: A missing actual price, a pricing error, or a `$0` actual price
  MUST mark that resource unpriced, never free, and MUST NOT substitute a
  projected on-demand price. Spot spend that was actually returned MUST be
  allocated as returned. All nodes unpriced MUST remain fatal. Conservation
  MUST stay the existing core check at relative epsilon 1e-6.
- **FR-014**: Historical output MUST label the mode historical and the period
  as the requested window. It MUST NOT label that report monthly or cite
  730 hours. Table, JSON, and NDJSON stay the shapes specified for
  `cost cluster`, with those two fields changed for a windowed run. Run-rate
  output MUST stay `mode=run-rate` and `period=monthly`.
- **FR-015**: The Prometheus address MUST be `FINFOCUS_PROMETHEUS_URL` when
  the process is outside the cluster, and the in-cluster Prometheus service
  only when that variable is unset and the process is in-cluster. An optional
  bearer token MUST be read from `FINFOCUS_PROMETHEUS_BEARER_TOKEN` and MUST
  NOT appear in logs, errors, or committed fixtures. A missing address outside
  the cluster MUST fail before a query.
- **FR-016**: The source MUST decline pricing requests so other cost commands
  do not route resource pricing to it. Unknown requested metrics MUST be
  ignored with a warning.
- **FR-017**: The plugin MUST be releasable on its own: it MUST NOT depend on
  finfocus core packages, and the existing plugin-boundary check MUST cover
  it. Its tests MUST meet the repository bar (at least 80% of the plugin, and
  at least 95% of the new historical pricing branch in core).
- **FR-018**: The local-cluster end-to-end job MUST install Prometheus into
  the cluster it already creates and MUST assert historical conservation and
  the fixture's resource-hours. That job MUST NOT require a cloud billing
  account. It uses a pricing stand-in with known amounts.
- **FR-019**: No registry install entry MUST be added until a
  `prometheus-v0.1.0` release exists. Once it exists, the entry MUST use
  prefix `prometheus-` and canonical version `v0.1.0`.
- **FR-020**: The published cluster-cost guide MUST document the window
  flags, the Prometheus source, the address variable, and the requirement
  for node-label metrics. It MUST stop describing every historical request
  as unsupported. The run-rate limitations that remain (spot projection,
  Fargate) stay documented on the no-window path.

### Key Entities *(include if feature involves data)*

- **Historical usage row**: one subject over one window. Workload metrics are
  `cpu_usage` and `mem_usage`. Node metrics are allocatable CPU and memory.
  Amounts are resource-hours (`core-hours`, `GiB-hours`), not per-hour rates.
  Subject keys match the run-rate row.
- **Window**: the start and end instants on the stats request and on the
  actual-cost request. Date-only flags are midnight UTC, and end must be after
  start. The report's period is this pair.
- **Node identity**: instance type, region, provider, and capacity type for a
  node that existed during the window, taken from recorded node-label metrics
  or, if those are missing, from the live API. It produces the same priceable
  descriptor shape the run-rate path prices.
- **Windowed price**: the actual spend of one priceable resource over the
  window. `$0`, an error, or a missing price means unpriced. It is not a
  monthly projection.
- **Checked hour example**: a fixed inputs-to-hours case for the query layer
  (reset, partial lifetime, missing series, filters, empty window). Examples
  are the contract for the hour calculation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can answer "what did namespace X cost between these two
  times" in one `finfocus cost cluster` command, and the printed mode and
  period name that window.
- **SC-002**: For a fixed workload that used half of what it requested, the
  charged CPU hours equal the used hours, not the requested hours.
- **SC-003**: Allocated historical rows sum to the priced window total within
  relative epsilon 1e-6, including the local-cluster proof with known prices.
- **SC-004**: A node replaced during the window is still priced when its
  identity was recorded then, and is reported incomplete when it was not.
- **SC-005**: A run of `finfocus cost cluster` with no window matches the
  current run-rate report. A reversed window, a missing Prometheus address,
  and an unreachable Prometheus each fail before any cost total is printed.
- **SC-006**: The six checked hour examples fail when the reported hours
  change. Plugin coverage stays at or above 80%, and the historical pricing
  branch stays at or above 95%.
- **SC-007**: With both usage sources installed, omitting `--usage-source`
  fails and names both. The kubernetes source still refuses a window. No
  registry entry appears before `prometheus-v0.1.0` exists.

## Assumptions

- Roadmap item #1529 is this feature. Run-rate allocation from
  `specs/613-cost-cluster/` and the usage contract from
  `specs/612-k8s-cost-allocation/` are on main. Historical mode is rejected
  today on purpose, until this source exists.
- The allocator's `max(request, usage)` rule is unchanged. Sending usage
  without requests is how historical mode charges consumption.
- "Recorded during the window" means node-label metrics already stored in
  Prometheus (kube-state-metrics or an equivalent scrape). This feature does
  not install recording rules beyond documenting that those metrics must be
  present. The live API is only the fallback for nodes that still exist.
- Kind has no cloud bill. The end-to-end proof checks hours and conservation
  against known prices. It does not claim those prices are an invoice.
- Actual-cost plugins that cannot see a Kubernetes node's cloud id return
  unpriced, and the report says so. This feature does not add a new cloud
  pricing map.
- Window flags use the same parser and limits as `finfocus cost actual`:
  date-only values are midnight UTC, `--to` defaults to now, end must be after
  start, and a future or too-old timestamp is rejected. This feature does not
  invent an end-of-day rule for date-only values.
- In-cluster Prometheus service discovery is a convenience for a plugin
  running inside the cluster. Every documented out-of-cluster path, including
  the local-cluster job, sets `FINFOCUS_PROMETHEUS_URL`.
- The first implementation does not publish `prometheus-v0.1.0`. FR-019 is
  gated on that tag and is not a reason to block the source, the command, or
  the local-cluster proof.
