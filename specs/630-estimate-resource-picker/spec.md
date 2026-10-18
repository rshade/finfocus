---
title: "Estimate resource picker"
description: "Conservative What-If resource selection and an escape hatch"
layout: "docs"
---

**Feature Branch**: `630-estimate-resource-picker`

**Created**: 2026-10-18

**Status**: Implemented

**Input**: Issue #1731

## User Scenarios & Testing

### User Story 1 - Start with estimation candidates (Priority: P1)

A developer opens What-If and sees cloud resources without Pulumi bookkeeping.

**Why this priority**: Avoid a provider baseline and irrelevant pricing warnings.
**Independent Test**: Load a mixed stack and check default candidates and selection.

**Acceptance Scenarios**:

1. **Given** providers, a stack, and EC2 instances, **When** What-If opens,
   **Then** providers and the stack are hidden and an eligible resource is selected.
2. **Given** zero spend, missing SKU/region, unsupported types, or failed discovery,
   **When** candidates are listed, **Then** uncertain resources remain accessible.
3. **Given** reliable explicit zero-cost metadata, **When** candidates are listed,
   **Then** the confirmed non-billable resource is hidden.

### User Story 2 - Restore all resources (Priority: P2)

A developer enables Show all resources to inspect any safe source descriptor.

**Why this priority**: Preserve access to previously selectable resources.
**Independent Test**: Toggle both ways with an eligible selection and an empty default list.

**Acceptance Scenarios**:

1. **Given** an eligible selected resource, **When** the toggle changes,
   **Then** its stable identity and current editor are preserved when source
   properties are unchanged. Changed preview properties refresh the editor and baseline.
2. **Given** only confirmed non-billable resources, **When** What-If opens,
   **Then** an explanation offers Show all resources, which restores them.
3. **Given** a resource selected only in the complete list, **When** hiding it,
   **Then** selection moves to an eligible candidate or clears the editor.

### User Story 3 - Consistent interactive selection (Priority: P2)

A developer opens interactive plan estimation and starts with the same eligible
resource policy as the browser.

**Why this priority**: Prevent web and terminal classification drift.
**Independent Test**: Apply the shared policy to the same ordered plan descriptors.

**Acceptance Scenarios**:

1. **Given** a provider before an estimation candidate, **When** interactive mode
   opens, **Then** it selects the candidate.
2. **Given** only bookkeeping entries, **When** interactive plan mode opens,
   **Then** it explains that no eligible candidate exists.

### Edge Cases

- Metadata absent, failed, stale after property edits, or conflicting: unknown stays visible.
- Zero rate and zero baseline alone never establish non-billability.
- New planned resources replace matching state descriptors without changing identities.
- Launch filters still constrain the complete list; synthetic rows never enter it.
- A late response after toggling or leaving the view cannot restore stale selection.

## Requirements

### Functional Requirements

- **FR-001**: Hide known Pulumi bookkeeping by default, using the shared engine rule.
- **FR-002**: Distinguish non-billable, potentially billable, and unknown resources;
  never infer non-billability from cost amounts, support, or missing inputs.
- **FR-003**: Use available explicit pricing metadata conservatively and only for
  effective pricing providers; missing/conflicting answers remain unknown.
- **FR-004**: Provide Show all resources after source merge and existing filters,
  preserving eligible selection, identities, source data, and edited values.
- **FR-005**: Prefer useful eligible candidates initially; explain an empty list
  and expose the escape hatch. Share the policy with interactive plan estimation.
- **FR-006**: Document the default, query contract, and escape hatch; do not change
  overview totals or trigger new pricing discovery on dropdown rendering.

### Key Entities

- **Candidate**: A safe source descriptor with stable identity and billability class.
- **Billability**: Confirmed non-billable, potentially billable, or unknown.
- **Selection**: A candidate identity retained while eligible in the visible list.

## Assumptions

- Pricing metadata is opportunistic: without already available reliable metadata,
  cloud resources remain unknown. Opening the picker does not probe plugins.
- Interactive plan mode has one initial resource, no picker; the shared default
  applies there. Explicit single-resource CLI requests remain available.
- Positive baseline may rank candidates but never decides visibility.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All bookkeeping fixtures are excluded by default and restored by the toggle.
- **SC-002**: Every uncertain fixture remains available, including zero-spend candidates.
- **SC-003**: Toggle round trips preserve eligible identities and property edits.
- **SC-004**: Web and terminal select eligible resources from the same mixed fixtures.
- **SC-005**: Listing candidates makes zero new pricing discovery calls.
