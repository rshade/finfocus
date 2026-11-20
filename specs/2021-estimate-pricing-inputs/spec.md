---
title: "Estimate pricing inputs"
description: "Show cost-affecting properties first in What-If, with read-only identity details and a Show all properties escape hatch"
layout: "docs"
---

**Feature Branch**: `2021-estimate-pricing-inputs`

**Created**: 2026-11-19

**Status**: Implemented

**Input**: Issue #1732 (follows `specs/630-estimate-resource-picker`, #1731)

## Clarifications

### Session 2026-11-19

- Q: Which names may a reviewed mapping list, given that several plugin-accepted
  names (S3 `storageClass`, NAT `dataProcessedGb`, DynamoDB, Lambda, and load
  balancer usage keys) are never properties of the Pulumi resource? → A: Usage
  rows. A `plugin-input` key the descriptor does not carry appears as an empty,
  editable usage row, so the user can supply a value the plugin reads.
- Q: Should a mapping apply whatever plugin prices the resource, or only the
  plugin its `plugin-input` names come from? → A: Per plugin. Each mapping names
  its plugin; its `plugin-input` and usage rows apply only when that plugin
  serves the resource's estimate. `sku` and `region` rows always apply.
  Recommendations are a separate route and are unaffected.
- Q: RDS was left on the fallback because aws-public's RDS pricing was a stub,
  but v0.2.5 prices it; what happens to RDS? → A: Add a reviewed mapping for
  `aws:rds/instance:Instance`: `instanceClass` (sku), `region` and
  `availabilityZone` (region), and `engine`, `storageType`, `allocatedStorage`,
  `multiAz` (aws-public `plugin-input`), verified like every other mapping.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Find the inputs that change the price (Priority: P1)

A developer opens What-If for an EC2 instance whose descriptor carries about 68
properties. The editor lists the reviewed pricing inputs first (instance type,
region, and the storage or usage inputs the pricing path reads), so changing
`t3.micro` to `t3.large` takes no scrolling or searching.

**Why this priority**: This is the problem the issue reports; without it the
editor is unusable on real stacks.
**Independent Test**: Open What-If for an EC2 fixture with state outputs and
bookkeeping metadata; the first rows are the reviewed inputs, and editing the
instance type recalculates the estimate.

**Acceptance Scenarios**:

1. **Given** a supported AWS resource type, **When** What-If opens, **Then** its
   reviewed pricing inputs appear first, each with its current value.
2. **Given** an EC2 instance on `t3.micro`, **When** the developer sets
   `t3.large`, **Then** the recalculated estimate and delta match the same
   override sent through the CLI.
3. **Given** a reviewed `plugin-input` key that the descriptor does not carry
   (such as a Lambda function's `requestsPerMonth`), **When** the editor opens,
   **Then** it appears as an empty, editable usage row after the present pricing
   inputs. A reviewed `sku` or `region` key that the descriptor does not carry
   gets no row.
4. **Given** a usage row, **When** the developer enters a value, **Then** the
   recalculation sends it as an override like any other edit; a usage row left
   empty sends nothing.

---

### User Story 2 - Keep every safe property reachable (Priority: P1)

A developer needs a property outside the reviewed set, or works with a resource
type that has no reviewed mapping. **Show all properties** lists every other safe
property as editable, and an unknown type opens with all safe properties and a
short explanation of why.

**Why this priority**: Narrowing the default must never lose a field; this is
the escape hatch the issue requires.
**Independent Test**: Toggle on a supported and on an unmapped resource; no
safe property is missing from the union of the default and advanced views.

**Acceptance Scenarios**:

1. **Given** a supported resource, **When** Show all properties is on, **Then**
   the remaining safe properties follow the pricing inputs, editable.
2. **Given** a resource type with no reviewed mapping, or a resource whose
   estimate is served by a plugin other than the mapping's and that carries no
   reviewed `sku` or `region` key, **When** What-If opens, **Then** all safe
   properties are editable and a note says the inputs are not curated for this
   type.
3. **Given** pricing-input metadata that is incomplete, **When** What-If opens,
   **Then** the editor takes the unknown-type fallback rather than a partial list.

---

### User Story 3 - Identity metadata is read-only (Priority: P1)

URNs, cloud IDs, ARNs, and timestamps appear in a read-only details area. They
cannot be edited or submitted as estimate overrides from the terminal or the
browser, and the web server rejects a request that tries.

**Why this priority**: Overriding identity would send the plugin a descriptor that
no longer names the resource; the web contract needs a server-side rule, not only
a disabled input.
**Independent Test**: POST a recalculate request that overrides an identity key;
it fails with a client error and prices nothing.

**Acceptance Scenarios**:

1. **Given** an EC2 instance, **When** What-If opens, **Then** its ARN, cloud ID,
   URN, and creation time are shown as read-only details.
2. **Given** a recalculate request overriding an identity key, **When** the
   server receives it, **Then** it answers a validation error naming the key.
3. **Given** the terminal editor, **When** it opens, **Then** identity values
   appear in a read-only block that the editing cursor cannot enter.

---

### User Story 4 - State survives toggling and rerendering (Priority: P2)

A developer edits two inputs, toggles Show all properties on and off, and the
editor keeps values, overrides, deltas, recalculated costs, and keyboard focus.
An override on a property outside the default set stays visible in an editor.

**Why this priority**: Losing an edit on a view change breaks trust in the
estimate, but only after Stories 1 to 3 exist.
**Independent Test**: Edit, toggle both ways, rerender after a recalculation, and
compare values and focused element.

**Acceptance Scenarios**:

1. **Given** edited values, **When** the toggle changes in either direction,
   **Then** every current value, override, and delta is unchanged.
2. **Given** an override on a property outside the default set, **When** the
   toggle is off, **Then** that property still shows in an editor.
3. **Given** focus on an input, **When** a recalculation rerenders the list,
   **Then** focus stays on the same input.

---

### User Story 5 - Web and terminal agree (Priority: P2)

The terminal estimate editor and the web view order and classify properties the
same way for the same descriptor.

**Why this priority**: The two surfaces drifted before; a shared policy removes the
class of bug.
**Independent Test**: Run the shared policy on the same descriptors for both
transports and compare the ordered keys and classes.

**Acceptance Scenarios**:

1. **Given** the same descriptor, **When** each transport builds its rows,
   **Then** the keys, order, and class of each row are identical.

---

### Edge Cases

- A `$0` delta never classifies a property as non-pricing; a conditional input
  (such as a free tier) stays in the pricing set, and a combined delta still
  appears only in the total change.
- A credential-like property or internal payload is absent from the default and
  advanced views, and from the read-only details.
- A property named like an identity key but carrying a user-chosen value, such as
  a tag, is classified by its exact reviewed key, not by a substring match.
- A request names a property the descriptor does not have: the existing
  behavior for new overrides is unchanged, except for identity keys. A usage row
  relies on this: its value reaches the plugin as a new override.
- A usage row whose value is cleared drops its override; an empty usage row is
  never sent and never counts as an override.
- Selecting a pricing mode from another plugin removes the mapping's
  `plugin-input` and usage rows and keeps its `sku` and `region` rows; an
  override already entered in a removed row stays visible as an edited property
  (FR-007). With the mapping's plugin not installed, the same rule applies.
- Late responses after a toggle or after leaving the view must not rewrite the
  editor.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: One shared policy MUST classify each safe property of a resource
  descriptor as a pricing input, identity metadata, or other, and the terminal
  and web MUST present that classification.
- **FR-002**: Pricing inputs MUST come from reviewed mappings for supported AWS
  resource types, plus usage hints from the plugin's pricing spec. Each input
  records its provenance: the SKU or region extraction rule that reads it
  (`sku`, `region`), a name the plugin documents as accepted (`plugin-input`), or
  an exact match with a pricing-spec usage metric (`usage`). A property MUST be
  marked a pricing input only when the selected estimation path consumes it. A
  `sku` or `region` key MUST be a property the Pulumi resource type carries. A
  `plugin-input` key MAY name a value the resource does not carry; when the
  descriptor lacks it, the policy emits an empty, editable usage row (FR-012).
  Each mapping names the plugin its `plugin-input` keys come from. Those keys
  and their usage rows apply only when that plugin serves the resource's
  estimate: it is among the plugins the estimate's pricing-spec discovery
  reports for the resource and, when a pricing mode is selected, it is that
  mode's plugin. `sku` and `region` keys come from core's extraction and apply
  whatever plugin prices the resource.
- **FR-003**: For a curated resource type, the default view MUST list pricing
  inputs first and show no other property, with two exceptions: a property that
  carries an override (FR-007) and every property of an uncurated type (FR-004).
  **Show all properties** MUST add every other safe, non-identity property as
  editable, after the pricing inputs.
- **FR-004**: A resource type with no reviewed mapping, or whose pricing-input
  metadata is incomplete, MUST open with all safe non-identity properties
  editable and a note saying why. No safe property may be missing from the union
  of the views.
- **FR-005**: Identity metadata (URN, cloud ID, ARN, creation and update
  timestamps) MUST appear as read-only details and MUST NOT be editable.
- **FR-006**: The web recalculate and baseline contracts MUST reject an override
  of an identity key with a validation error, and the terminal MUST refuse the
  edit. The rejection is made by the shared policy, not by the page.
- **FR-007**: Current values, overrides, deltas, recalculated costs, and focus
  MUST survive toggling and rerendering. An override outside the default set MUST
  remain visible in an editor in both modes.
- **FR-008**: The policy MUST NOT infer that a property does not affect cost from
  a zero delta, an unchanged estimate, or a missing delta. Conditional inputs and
  combined deltas MUST remain supported.
- **FR-009**: Secrets, credential-like properties, and internal payloads MUST stay
  redacted in every view and in the details, through the existing redaction rule
  and no second list.
- **FR-010**: The policy MUST NOT discover inputs by perturbing properties or by
  probing plugins repeatedly, and MUST NOT require a new plugin protocol field.
  The only plugin metadata it uses is the pricing-spec discovery the estimate
  already runs, which the engine caches per resource type. Selecting a pricing
  mode may change which `plugin-input` and usage rows apply (FR-002). It does
  not cause another discovery call.
- **FR-011**: The documentation MUST list the covered resource types, the
  fallback behavior, the advanced view, and the usage rows.
- **FR-012**: A usage row MUST have no original value, MUST be editable, and MUST
  appear in the default view after the present pricing inputs, in mapping order.
  Only a non-empty value is sent as an override. A usage row counts as a reviewed
  input, so a type whose reviewed keys are all absent from the descriptor is
  still curated and does not take the FR-004 fallback.

### Key Entities

- **Property class**: Pricing input, identity metadata, or other, assigned per
  property of a descriptor.
- **Input mapping**: A reviewed record tying a resource type to the property keys
  that its pricing path reads, with provenance.
- **Property presentation**: The ordered rows, the read-only details, and the
  fallback note for one descriptor and one set of overrides.
- **Usage row**: A pricing-input row for a reviewed `plugin-input` key the
  descriptor does not carry. It starts empty and becomes an override only when
  the user enters a value.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends Estimate.
- **Web data**: The classified, ordered property rows, the read-only identity
  details, the fallback note, and the override rejection, all produced by the
  shared Go policy that the terminal also calls. The page computes no
  classification.
- **Layout fit**: Fills the existing Estimate property editor: pricing inputs in
  the captioned table, a Show all properties control in the controls row beside
  Show all resources, the note in the status line, and identity details in the
  detail area.
- **Additions**: A Show all properties toggle and a read-only details block.
- **Render evidence**: 2026-11-19, Estimate rendered with Playwright against
  `../finfocus-demo` stack `dev` (preview-only creates). The EC2 instance showed
  `instanceType` and `region` in the captioned table, with Show all properties
  beside Show all resources in the controls row; the VPC (uncurated) showed
  every safe property and the fallback note. The render found the EBS mapping
  naming `volumeType`, which a Pulumi volume does not have: `type` was hidden.
  Fixed (research.md); after the fix the volume leads with `type` and `size`.
  2026-11-20: rendered again on `initial-30` stack `dev` (65 resources, empty
  passphrase). The NAT gateway `clientNatGateway` opens on an empty
  `dataProcessedGb` usage row, with aws-public's `per_hour_plus_data` mode
  listed. The EC2 instance `webServer` leads with `instanceType` (`t3.micro`),
  `region` (`us-west-2`), `availabilityZone` (`us-west-2c`), and `tenancy`
  (`default`); the baseline is `$8.39/mo`.
- **Follow-up issue scope**: Waived 2026-11-20 (T030). The web Estimate view
  ships in this feature. No separate UX follow-up issue.

## Assumptions

- Reviewed mappings start with the AWS resource types the aws-public plugin
  prices, RDS instances included; types beyond them use the fallback until
  reviewed. Each mapping is checked by the provenance test and against a real
  Pulumi descriptor of its type.
- Mappings live in core as presentation data, not provider pricing logic, and
  plugin-declared inputs remain a separate future change.
- The mapping's plugin scopes only the What-If editor's rows and the overrides
  it sends to the estimate. Recommendations route to every plugin that
  advertises them, so another plugin (such as CloudZero, once it advertises
  recommendations) still recommends for a resource that aws-public prices.
- Identity keys are an exact, reviewed list; Pulumi-internal `__` properties stay
  hidden by the existing filter.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For each documented AWS type, the reviewed pricing inputs are the
  first rows, and the EC2 instance type is the first row of its editor.
- **SC-002**: Every safe non-identity property of a fixture appears in the
  union of the default and advanced rows. The only rows beyond that set are
  applied usage rows and overrides for keys the descriptor does not carry.
- **SC-003**: Every identity-key override is rejected by the web contract and
  refused by the terminal, with zero estimates priced.
- **SC-004**: A toggle round trip leaves values, overrides, deltas, and focus
  unchanged in every fixture.
- **SC-005**: Web and terminal produce identical ordered keys and classes for the
  same descriptors.
- **SC-006**: No credential-like property appears in either view in any fixture.
