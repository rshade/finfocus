# Feature Specification: Pass include_dismissed From cost recommendations

**Feature Branch**: `622-include-dismissed`

**Created**: 2026-10-04

**Status**: Implemented

**Input**: User description: "When the operator passes --include-dismissed, finfocus sets
GetRecommendationsRequest.include_dismissed so plugins that store dismissals return them.
Excluded recommendation IDs are still sent. The default command does not set the field.
Closes #545. Protocol: finfocus-spec specs/599-include-dismissed."

## Clarifications

The protocol rules live in finfocus-spec `specs/599-include-dismissed`. This spec covers only the
host. `/speckit-clarify` was not required: the CLI flag already exists, and the exclusion list
keeps its current meaning so older plugins stay correct.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The audit flag reaches the plugin (Priority: P1)

An operator who already uses `--include-dismissed` sees local dismissed and snoozed rows. With this
feature, the same flag also tells each recommendations plugin to include recommendations that the
plugin itself dismissed. A run without the flag does not set the field.

**Why this priority**: This is the core half of #545. The flag exists; it never reached the plugin.

**Independent Test**: Drive `GetRecommendationsForResourcesWithDismissed` with a recording plugin
and confirm the request has `IncludeDismissed` true and still carries the host's excluded IDs.
The default method leaves the field false.

**Acceptance Scenarios**:

1. **Given** dismissed recommendation `rec-1` in the local store, **When** the operator lists with
   `--include-dismissed`, **Then** the plugin request sets `include_dismissed` and still lists
   `rec-1` in `excluded_recommendation_ids`.
2. **Given** the same store, **When** the operator lists without the flag, **Then**
   `include_dismissed` is false and `rec-1` is still excluded.
3. **Given** the overview and the Pulumi analyzer, **When** they load recommendations, **Then** they
   keep calling the default method and do not set the field.

---

### User Story 2 - The cache does not mix the two answers (Priority: P1)

The two requests can return different recommendations from a plugin that stores dismissals. A cache
entry written for one must not be served for the other.

**Why this priority**: A shared cache key would hide plugin-side dismissals, or show them on a
normal run, until the entry expired.

**Independent Test**: Fetch with the flag, then fetch without it, against one cache. The plugin is
called twice. Fetch the flagged request again. The plugin is not called a third time.

**Acceptance Scenarios**:

1. **Given** a cached default result, **When** the flagged request runs, **Then** the plugin is
   called again.
2. **Given** a cached flagged result, **When** the same flagged request runs, **Then** the plugin
   is not called.
3. **Given** a default request, **When** it is repeated, **Then** it still hits the cache key used
   before this feature.

---

### User Story 3 - The operator can read what the flag does (Priority: P2)

The command help and the command reference say that `--include-dismissed` shows local dismissed and
snoozed rows and asks plugins to include recommendations they have dismissed.

**Why this priority**: The flag's help text currently describes only the local merge.

**Independent Test**: Read the flag's help and the command reference row. Both mention plugins.

**Acceptance Scenarios**:

1. **Given** `finfocus cost recommendations --help`, **When** the operator reads
   `--include-dismissed`, **Then** the text mentions plugins as well as local dismissed rows.
2. **Given** the command reference, **When** the operator reads that flag, **Then** the description
   matches the help.

### Edge Cases

- A plugin that ignores the new field still receives `excluded_recommendation_ids`, so host
  dismissals stay hidden and the local merge still appends them.
- An excluded ID is not returned by a plugin that honors both fields, so the local merge does not
  duplicate it.
- Recommendations a plugin returns only because the flag is set are not labeled dismissed. The
  protocol has no status on the recommendation. Local rows keep their Dismissed or Snoozed label.
- Batch requests (more than 100 resources) set the same field on every batch.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `--include-dismissed` MUST set `include_dismissed` on every `GetRecommendations`
  request for that command, including batched requests.
- **FR-002**: The same command MUST still send the host's excluded recommendation IDs.
- **FR-003**: A recommendations fetch that is not `--include-dismissed` MUST leave the field false.
  Overview and the analyzer MUST stay on that path.
- **FR-004**: The recommendations cache key MUST differ when the field is true, and MUST stay the
  same as today's key when the field is false.
- **FR-005**: The adapter MUST copy the internal `IncludeDismissed` value onto the proto request.
- **FR-006**: Help text and the command reference MUST describe the plugin request. The local merge
  of dismissal records MUST stay in place.
- **FR-007**: Core MUST NOT invent provider pricing or a second dismissal store. It depends on
  finfocus-spec `specs/599-include-dismissed` for the field.

### Key Entities

- **include_dismissed**: The proto bool from finfocus-spec. True only for the flagged command.
- **Excluded recommendation IDs**: Host dismissal IDs sent on every recommendations fetch, flagged
  or not.
- **Local merge**: Existing append of dismissal records that the plugin response did not already
  contain. Unchanged.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A recording plugin observes `IncludeDismissed` true only on the flagged fetch, with
  the excluded ID still present.
- **SC-002**: Default and flagged fetches of the same resources miss each other's cache entries.
- **SC-003**: A repeated default fetch still uses one plugin call after the first, as it does today.
- **SC-004**: `make test` and `make lint` pass for the touched packages.

## Assumptions

- finfocus-spec has published `include_dismissed` as field 8 before this branch's module pin moves
  to that commit. Until the spec release is tagged, the pin may be the spec commit that adds the
  field. `pluginsdk.SpecVersion` stays `v0.7.4` until that release, so no upgrade hop is added here.
  `plugins/kubernetes` and `plugins/jev` require that same pin. CI fails when their
  `finfocus-spec` version differs from the root module.
- Provider plugins in other repositories are not edited. Plugins with no dismissal store already
  ignore the field. The aws-public recommendations handler does not store dismissals.
- The in-repo recorder records the request proto, so the field is captured without recorder changes.
- Roadmap copies that list #545 are checked when this host behavior lands.
