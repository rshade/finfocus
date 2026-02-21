---

description: "Task list for 598-analyzer-routing-docs"

---

# Tasks: Document Routing Limits in Analyzer Mode

**Input**: Design documents from `/specs/598-analyzer-routing-docs/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅

**Tests**: This is a documentation-only feature. Per Constitution Principle II,
validation is `make docs-lint` (markdownlint). There are no Go code changes and
therefore no unit or integration tests. Both modified files MUST pass `make docs-lint`
with zero errors before tasks are marked complete.

**Completeness**: Per Constitution Principle VI, all documentation content must be
fully written. No placeholder text, `TODO` markers, or `[INSERT HERE]` stubs are
permitted in committed changes.

**Documentation**: This feature IS the documentation update. The quality gate is
`make docs-lint` passing with zero errors on both modified files.

**Organization**: Tasks are grouped by user story. US1 and US3 share a phase because
both are addressed by the same callout block in `docs/guides/routing.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Pre-flight)

**Purpose**: Verify the baseline documentation build is clean before any edits.

- [x] T001 Confirm `make docs-lint` passes with zero errors on the unmodified branch (baseline check)

---

## Phase 2: User Stories 1 & 3 — routing.md Callout (Priority: P1/P3) 🎯 MVP

**Goal**: A developer reading `docs/guides/routing.md` after a failed routing exclusion
in analyzer mode finds a clear, actionable callout explaining: (1) routing is not
consulted in analyzer/policy-pack mode, (2) global plugins cannot be excluded via
routing in any mode, (3) where to find the isolation procedure.

**Independent Test**: Read `docs/guides/routing.md` from the "Common Configuration
Patterns" section to the end. The callout must appear immediately before `## Validation`
and must contain all three facts (routing not applied, global plugins always fire,
link to analyzer-integration.md).

**Note**: US3 (global plugins cannot be excluded) is fully addressed by the same
callout as US1. No separate phase is needed for US3.

### Implementation for User Stories 1 & 3

- [x] T002 [US1] [US3] Add analyzer-mode callout blockquote to `docs/guides/routing.md` after the "Common Configuration Patterns" section (before `## Validation`): state that routing is not consulted in policy-pack mode, that global plugins always fire, and cross-reference `analyzer-integration.md#isolating-plugins-in-analyzer-mode`
- [x] T003 [P] [US1] Update the `## Changelog` section in `docs/guides/routing.md` with a new entry documenting the analyzer-mode callout addition

**Checkpoint**: After T002 and T003, read `docs/guides/routing.md` and verify the
callout is visible, correctly placed, and all three FR-001/FR-002/FR-003 facts are
present. US1 and US3 acceptance scenarios are now independently verifiable.

---

## Phase 3: User Story 2 — analyzer-integration.md Isolation Procedure (Priority: P2)

**Goal**: An operator reading `docs/analyzer-integration.md` finds a complete,
tested step-by-step procedure for isolating plugins in analyzer mode using
`FINFOCUS_HOME`, including the file-level symlink requirement, the issue #750 warning,
and an inline bash example.

**Independent Test**: Read `docs/analyzer-integration.md` and locate the "Isolating
Plugins in Analyzer Mode" section. The section must contain: `FINFOCUS_HOME`
explanation, Step 1 (`mkdir -p`), Step 2 (`ln -sf` binary), Step 3 (`FINFOCUS_HOME=`
inline example), the `> **Warning:**` callout for issue #750, and the empty-directory
behavior note.

### Implementation for User Story 2

- [x] T004 [US2] Add `## Isolating Plugins in Analyzer Mode` section to `docs/analyzer-integration.md` immediately before `## See Also`, containing: (a) explanation of `FINFOCUS_HOME` as the only isolation mechanism, (b) `> **Warning:**` callout noting directory-level symlinks are broken until [issue #750](https://github.com/rshade/finfocus/issues/750), (c) Step 1 `mkdir -p` real directory creation, (d) Step 2 `ln -sf` file-level binary symlink, (e) Step 3 inline `bash` code block with `FINFOCUS_HOME=~/.finfocus/demo pulumi preview --policy-pack ...`, (f) note that empty `FINFOCUS_HOME` loads zero plugins with no fallback, (g) `FINFOCUS_HOME` precedence reference. **Note**: when copying content from `plan.md` Phase 1 design block, remove the backslash escapes before fenced code fences (`` \``` `` → `` ``` ``)

**Checkpoint**: After T004, read `docs/analyzer-integration.md` from "Isolating Plugins
in Analyzer Mode" to "See Also". All US2 acceptance scenarios are now verifiable.

---

## Phase 4: Polish & Validation

**Purpose**: Confirm both modified files meet quality gates with zero lint errors.

- [x] T005 [P] Run `make docs-lint` on the full docs directory and fix any markdownlint errors introduced by T002, T003, or T004 in `docs/guides/routing.md` and `docs/analyzer-integration.md`
- [x] T006 [P] Run `make lint` (full pipeline) to confirm no regressions in Go linting or other checks

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Pre-flight)**: No dependencies — run first
- **Phase 2 (US1/US3)**: Depends on Phase 1 completion
- **Phase 3 (US2)**: Independent of Phase 2 — can run in parallel with Phase 2 (different files)
- **Phase 4 (Polish)**: Depends on Phase 2 and Phase 3 completion

### User Story Dependencies

- **US1 (P1)**: T001 → T002 → T003
- **US3 (P3)**: Covered by US1 phase (same callout block); no additional tasks
- **US2 (P2)**: T001 → T004 (independent of US1/US3)

### Within Each Phase

- T002 must complete before T003 (changelog references the added callout)
- T005 and T006 are independent of each other [P]

### Parallel Opportunities

- T003 and T004 can run simultaneously (different files, no shared dependencies)
- T005 and T006 can run simultaneously

---

## Parallel Example: Phase 2 + Phase 3 Overlap

```text
After T001 (baseline check):

  Thread A: T002 → T003   (docs/guides/routing.md)
  Thread B: T004           (docs/analyzer-integration.md)

After both threads complete:

  T005 [P] + T006 [P]     (validation — run together)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Pre-flight (T001)
2. Complete Phase 2: US1/US3 (T002, T003)
3. **STOP and VALIDATE**: Run `make docs-lint` on `docs/guides/routing.md` only
4. US1 routing.md callout is independently verifiable at this point

### Incremental Delivery

1. Pre-flight (T001) → Foundation confirmed
2. US1/US3 callout (T002, T003) → `routing.md` complete
3. US2 isolation procedure (T004) → `analyzer-integration.md` complete
4. Polish (T005, T006) → All acceptance criteria met, issue #759 can be closed

### Minimal Approach (Single Developer)

```text
T001 → T002 → T003 → T004 → T005 → T006
```

Total estimated edits: ~15 lines in routing.md, ~40 lines in analyzer-integration.md.

---

## Notes

- [P] tasks = different files, no shared dependencies
- [Story] label maps task to specific user story for traceability
- US3 has no independent implementation tasks — it is fully addressed by the US1 callout
- `make docs-lint` is the sole quality gate for this feature (no Go code changes)
- Do not create new files; all changes are additive to the two existing docs files
- The plan.md contains the full verbatim callout and section content ready for insertion
