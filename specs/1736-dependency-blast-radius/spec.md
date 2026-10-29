# Feature Specification: Dependency Blast Radius

**Feature Branch**: `1736-dependency-blast-radius`  
**Created**: 2026-10-28  
**Status**: Implemented (US1 to US3 and the web UX refinement)  
**Input**: User description: "Show, for a selected resource in the overview
detail view and the web dashboard, every resource that depends on it and the
monthly cost those dependents carry, built from the Pulumi dependency graph
that finfocus already ingests, so a person can see what a change to that
resource touches. Replaces the unshipped CostAllocationLineage plan from issue
366."

## Clarifications

### Session 2026-10-28

- Q: Is a resource's `provider` reference a dependency edge (ISS-001)? → A:
  Yes. The reference is an edge on that provider, whose URN is the reference
  with its last `::<id>` segment removed, in state and in preview steps alike.
- Q: Are `deletedWith` and `replaceWith` dependency edges too (ISS-001)? → A:
  Yes, both, from state and preview steps. They change the total only when a
  priced resource is reachable through them alone; otherwise they are a second
  route to a dependent already counted once.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See what depends on a resource (Priority: P1)

A person reviewing a stack in the interactive overview selects a resource and
opens its detail view. Alongside the resource's own cost, they see a
"Blast radius" section: the resources that depend on it, shown as a tree from
direct dependents outward, with each resource's type, name, status, and
monthly cost, and a total for everything downstream.

**Why this priority**: This is the whole point of the issue. Without it the
overview shows what a resource costs but not what a change to it puts at
risk, so a person judging a replacement or deletion has to read the Pulumi
program to find out.

**Independent Test**: Run the overview against a stack in which a database is
used by a service that is in turn used by a load balancer. Select the
database and open its detail. The section lists the service as a direct
dependent and the load balancer one level below it, with a total equal to the
sum of their monthly costs.

**Acceptance Scenarios**:

1. **Given** a resource with dependents, **When** the person opens its detail
   view, **Then** the section lists every dependent, indented by distance from
   the selected resource, and shows the count and the monthly total.
2. **Given** a resource nothing depends on, **When** the person opens its
   detail view, **Then** the section says that no resource depends on it
   instead of showing an empty tree or being absent.
3. **Given** two resources that both depend on a third, **When** the person
   opens the third's detail view, **Then** each dependent appears once and the
   total counts each once.

---

### User Story 2 - See the same blast radius in the web dashboard (Priority: P2)

A person using `finfocus --web` selects a resource in the Overview table and
sees the same section, with the same dependents, counts, and total, in the
resource's detail panel.

**Why this priority**: A user-facing CLI feature is not complete until the
web dashboard reaches it. The TUI is the first surface, but the web view is
required for completion.

**Independent Test**: Open the same stack in the terminal overview and in the
web dashboard, select the same resource in each, and compare the listed
dependents, count, and total. They match.

**Acceptance Scenarios**:

1. **Given** a resource with dependents, **When** the person opens its detail
   panel in the web dashboard, **Then** it shows the same dependents, count,
   and total as the terminal view.
2. **Given** a resource with no dependents, **When** the person opens its
   detail panel, **Then** it shows the same "nothing depends on this" message.

---

### User Story 3 - Trust the numbers when the stack is mid-change (Priority: P3)

A person previewing an update, or looking at a stack where some resources are
being created, replaced, or deleted, can tell which dependents are existing,
which are new, and which are going away, and can tell which dependents have no
price, so the total is never read as more complete than it is.

**Why this priority**: The base view works on a settled stack. The overview's
purpose includes planned changes, and a total that quietly mixes states or
hides unpriced resources would mislead the person the feature is for.

**Independent Test**: Run the overview against a preview that creates one
dependent, deletes another, and includes one resource finfocus cannot price.
The section marks each, and its total counts only the priced dependents that
will exist after the change, with a note giving the counts left out.

**Acceptance Scenarios**:

1. **Given** a dependent that the plan creates, **When** the section renders,
   **Then** it appears marked as new, with its projected monthly cost counted
   in the total.
2. **Given** a dependent that the plan deletes, **When** the section renders,
   **Then** it appears marked as being deleted and is not counted in the
   total, and the section says how many deleted dependents were left out.
3. **Given** a dependent with no price, **When** the section renders, **Then**
   it appears marked as unpriced, is not counted in the total, and the section
   says how many unpriced dependents were left out.
4. **Given** the overview was started without a plan (state only), **When**
   the section renders, **Then** it shows the dependents recorded in state,
   with no new-resource markers.

---

### Edge Cases

- A stack with no recorded dependencies at all (for example a Terraform
  project, or a state that carries none) shows that dependency information is
  not available, not that nothing depends on the resource.
- A dependency that names a resource missing from the stack (an unresolved or
  stale reference) is skipped, and the section notes how many references could
  not be resolved.
- A dependency cycle, which Pulumi does not produce but a hand-edited file
  could, must not hang or repeat entries; each resource appears once.
- A resource that is both a direct and an indirect dependent appears once, at
  its shortest distance.
- A very large downstream set stays usable: the tree can be scrolled in the
  terminal and in the web panel. The count and total sit directly under the
  section heading in the terminal, and in the web the caption stays in view
  while the rows scroll.
- A resource that is a component whose children carry the cost: the children
  are not treated as dependents merely because of the parent relationship.
- A provider, including a default provider (`default_<version>`), lists every
  resource it manages as a dependent. A cluster is reached the same way: its
  workloads depend on the cluster through the provider built from it. A
  provider reference whose URN is not in the stack counts as unresolved
  (FR-007).
- Resources whose cost basis differs (a deleted resource has no projected
  cost) are never summed together: the total is on one basis, projected
  monthly, and the section says so.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The overview detail view MUST show a "Blast radius" section for
  the selected resource listing every resource that depends on it, directly or
  through other resources.
- **FR-002**: The section MUST present dependents as a tree ordered by
  distance from the selected resource, and MUST show each resource at most
  once, at its shortest distance.
- **FR-003**: For each dependent the section MUST show its resource type,
  name, lifecycle status (existing, new, being replaced, being deleted), and
  projected monthly cost or an "unpriced" marker.
- **FR-004**: The section MUST show the number of dependents and one total of
  their projected monthly cost. The total MUST count only priced dependents
  that will exist after the change, MUST NOT include the selected resource's
  own cost, and the section MUST state how many dependents the total leaves
  out and why.
- **FR-005**: Dependencies MUST come from the dependency relationships Pulumi
  records between resources: `dependencies`, every `propertyDependencies`
  list, `deletedWith`, every `replaceWith` entry, and the resource's
  `provider` reference (an edge on the provider resource, so the resources a
  provider manages are dependents of it and of whatever the provider depends
  on). These are the links Pulumi's own dependents query follows. The
  parent-child relationship MUST NOT, by itself, make a resource a dependent.
- **FR-006**: A resource with no dependents MUST show an explicit "nothing
  depends on this resource" message. A source that carries no dependency
  information MUST show an explicit "dependency information is not available"
  message instead.
- **FR-007**: The section MUST tolerate references to missing resources and
  dependency cycles without error, and MUST report the number of references
  it could not resolve.
- **FR-008**: The web dashboard MUST show the same section, with the same
  dependents, counts, totals, and messages, in the Overview resource detail
  panel. The web MUST NOT compute the graph or the total itself.
- **FR-009**: The terminal and web results MUST come from one computation of
  the blast radius for a given resource and run, so the two cannot disagree.
- **FR-010**: The section MUST be present in state-only mode and after a
  preview is applied to a state-only run, and MUST update when the preview
  adds or changes resources.
- **FR-011**: Opening a resource's detail view MUST NOT make additional plugin
  calls to price dependents: it uses costs the overview already holds.
- **FR-012**: Resource properties and tags shown or hidden elsewhere in the
  overview MUST NOT be exposed through this section; it shows only type, name,
  status, and cost.
- **FR-013**: Non-interactive output formats (table, JSON, NDJSON) are
  unchanged by this feature.

### Key Entities *(include if feature involves data)*

- **Dependency edge**: A recorded relationship stating that one resource
  depends on another, as Pulumi records it (FR-005 lists the kinds). It has a
  dependent, a dependency, and no cost of its own.
- **Blast radius**: For one selected resource, the set of resources reachable
  by following dependency edges backward, each with its distance from the
  selected resource, its status, and its priced or unpriced state.
- **Blast-radius total**: The count of dependents and the sum of their
  projected monthly cost, with the number of dependents left out and the
  reason for each group left out (deleted, unpriced, unresolved).

## Assumptions

- Downstream means every resource that depends on the selected one,
  transitively. A direct-only view is not offered in this version.
- Terraform and OpenTofu projects have no dependency information in this
  version and show the "not available" message.
- The cost shown is the overview's projected monthly figure, on the same basis
  as the Monthly column. Month-to-date and drift figures are not summed.
- A deleted dependent is listed because a person needs to know it is affected,
  but it is not counted, because it will not exist after the change.
- A dependency on a component counts as a dependency on everything inside it.
  Pulumi does this when it records the edge: `dependsOn` a component lists the
  component's members, not the component (confirmed against a real state file,
  research R1). The graph needs no parent walk. Components appear in the tree
  when they list a dependency themselves, unpriced and uncounted, so the tree
  stays connected.
- The feature adds no new setting and no new command.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the Overview view. The section is one more block in
  the Overview resource detail panel that opens from a table row.
- **Web data**: The ordered list of dependents (type, name, status, distance,
  monthly cost or unpriced marker), the dependent count, the monthly total, and
  the counts left out of the total with their reasons, plus the two empty
  messages. The web shows these values as the terminal does and computes none
  of them.
- **Layout fit**: Fills the detail slot below the resource's cost and property
  details, as a captioned table (`.detail-table`) with numeric columns aligned,
  indented by distance, with the total in the caption.
- **Additions**: two, both client-side and both reusing existing slots: the
  dependents in the block are buttons that open that resource's detail panel,
  and the detail header gains a `Back to NAME` button. Neither adds a route,
  parameter, or payload field.
- **Render evidence**: 2026-10-28, existing Overview detail panel rendered
  with playwright-mcp from a throwaway local-backend Go stack (a component
  `grp` with two `random:RandomId` children and a resource that depends on it;
  `../finfocus-demo` stack `dev` had no deployed resources). The panel is a
  `dl` of URN, Type, Status, Actual MTD, Projected, Delta and Drift under a
  "Close detail" header, below the table and budget block, so the new block
  fits as one more section beneath Drift. Screenshot:
  `.playwright-mcp/1736-overview-detail-before.png` (gitignored).
- **Implemented render**: 2026-10-28, `finfocus --web --pulumi-state
  internal/cli/testdata/blast_radius_state.json` (the T015 fixture, no
  plugins, so every dependent shows `no price`; the `../finfocus-demo` stack
  `dev` still had no deployed resources) rendered with playwright-mcp. The
  Blast radius block sits under Drift as a captioned `.detail-table`:
  dependents indented by depth, the count and the
  "2 unpriced dependents are not in the total" note in the caption, and the
  Monthly column right-aligned. Screenshot:
  `.playwright-mcp/blast-radius-section.png` (gitignored). Priced totals and
  status markers are covered by the Go parity and golden tests.
- **UX refinement render**: 2026-10-29, `finfocus --web --pulumi-state
  internal/cli/testdata/blast_radius_state.json` (the T015 fixture; every
  dependent shows `no price`; `../finfocus-demo` has no deployed resources),
  rendered with playwright-mcp in light and dark themes at 1100 and 420 px.
  Screenshots: `.playwright-mcp/1736-blast-ux-light.png` and
  `.playwright-mcp/1736-blast-ux-dark-narrow.png` (gitignored). Keyboard: Tab
  focus on a dependent, Enter opens its detail with focus on the heading and a
  `Back to orders-db` button, Back returns. The render found two defects the
  unit tests could not: the dashboard CSP (`default-src 'self'`) blocks inline
  `style` attributes, so the caption height is now a class, and the table was
  419 px wide in a 321 px container, so the Monthly column was off-screen
  until the caption stopped setting the table width and the name column
  wrapped. Contrast, computed from the CSS custom properties on `--bg-subtle`
  (WCAG 2.2 AA needs 4.5:1 for text, 3:1 for focus): text 16.07:1 (light) and
  13.58:1 (dark); muted text 6.47:1 and 8.64:1; link 6.16:1 and 7.77:1; focus
  ring 6.16:1 and 7.77:1; warning-colored status labels 6.71:1 and 11.68:1.
- **Provider edges render (ISS-001)**: 2026-10-28, `finfocus --web` from the
  `initial-30` project (stack `dev`, 65 deployed resources, aws-public
  pricing) rendered with playwright-mcp. `civoCluster` shows 8 dependents
  (the `civo-k8s` provider at depth 1, seven Kubernetes resources at depth 2;
  1 before the change), `civo-k8s` shows 7 (0 before), `webVpc` is unchanged
  at "20 dependents, $91.39/month", and the default AWS provider shows 38
  dependents at $141.03/month, the stack's projected total. The terminal
  overview, driven through tmux, shows `civoClusterFirewall` with the same 9
  dependents, depths and note as `GET /api/overview/resource`. Through a
  preview, `../finfocus-demo` still shows `demo-vpc` at "3 dependents,
  $7.59/month". No handler, contract or panel change. Screenshot:
  `.playwright-mcp/blast-radius-initial30-civoCluster-provider-edges.png`
  (gitignored).
- **Web UX refinement (in scope)**: Decided 2026-10-28 to build the UX work
  inside this feature. The refinement covers only the detail-panel block in
  `internal/webui/static/`: no engine, API, redaction, or data-shape work.
  - **Layout**: the block stays readable for long lists (bounded height with
    the caption and column headers kept in view) and for narrow panels.
  - **Rendering**: the tree depth shows as indentation with connector glyphs;
    status (new, deleting, unpriced, component) is conveyed by a text label,
    never by color alone.
  - **Interaction**: selecting a dependent opens that resource's detail panel,
    reachable by keyboard (Tab, Enter) with focus returned sensibly on close.
  - **Copy**: the headings, empty messages, and left-out note read the same
    as the terminal's wording (FR-006).
  - **Accessibility**: the table has a caption and header scopes, the depth is
    exposed to assistive technology, and contrast meets the dashboard's
    existing standard in light and dark themes (WCAG 2.2 AA: 4.5:1 text, 3:1
    focus indicators).
- **Follow-up issue (decision)**: On 2026-10-29 the user decided not to file
  the `web-ui` follow-up issue, because the UX work is in this branch and
  nothing is left out. Principle VII.3 asks for that issue; this feature does
  not file it by the user's decision, and the constitution itself is unchanged.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A person can find out what depends on a given resource, and what
  it costs per month, in two steps from the overview table: select the row,
  open the detail view.
- **SC-002**: For every resource in the demo stack, the terminal and web
  sections show the same dependents, count, and total.
- **SC-003**: For every resource, the displayed total equals the sum of the
  monthly costs of the listed priced dependents that will exist after the
  change, and the left-out counts equal the number of listed dependents that
  are deleted or unpriced.
- **SC-004**: Opening a detail view on a 500-resource stack feels immediate:
  the section appears with no visible wait and triggers no new pricing
  requests.
- **SC-005**: A stack whose dependency data contains a cycle or a reference to
  a missing resource still renders every other part of the detail view, with
  no error, and reports the unresolved count.
