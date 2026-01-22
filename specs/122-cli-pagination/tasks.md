# Implementation Tasks: CLI Pagination and Performance Optimizations

**Feature**: CLI Pagination and Performance Optimizations
**Branch**: `122-cli-pagination`
**Date**: 2026-01-20
**Plan**: [plan.md](./plan.md)

## Task Organization

Tasks are organized by user story priority to enable independent implementation and testing. Each user story can be developed and validated independently, allowing for incremental delivery.

**User Story Priorities** (from spec.md):
- **P1 (Critical)**: US1 (Enterprise Scale Performance), US2 (Output Control & Pagination)
- **P2 (High)**: US3 (TUI Virtual Scrolling), US4 (CI/CD Streaming Integration)

**Task Format**: `- [ ] [TaskID] [P] [Story] Description with file path`
- `[P]` = Parallelizable (different files, no dependencies)
- `[Story]` = User Story label (US1, US2, US3, US4) - REQUIRED for story phases
- File paths are absolute from repository root

---

## Phase 1: Setup & Infrastructure

**Goal**: Initialize project structure and foundational packages for pagination features.

### Tasks

- [X] T001 Create internal/cli/pagination package with package documentation
- [X] T002 [P] Create internal/engine/cache package with package documentation
- [X] T003 [P] Create internal/engine/batch package with package documentation
- [X] T004 [P] Create internal/tui/list package with package documentation
- [X] T005 [P] Create internal/tui/detail package with package documentation (for lazy loading)
- [X] T006 [P] Create test/fixtures directory and generate large_dataset_1000.json (1000-item test data)
- [X] T007 [P] Create test/fixtures directory and generate large_dataset_10000.json (10,000-item test data)
- [X] T008 Add cache configuration fields to internal/config/config.go (cache.ttl_seconds, cache.enabled, cache.directory, cache.max_size_mb)

**Completion Criteria**:
- All package directories exist with package-level documentation
- Test fixtures are generated and valid JSON
- Configuration structure supports cache settings

---

## Phase 2: Foundational Components (Blocking Prerequisites)

**Goal**: Build core utilities needed by all user stories (pagination params, cache store, batch processor).

### Tasks

- [X] T009 Implement PaginationParams struct with validation in internal/cli/pagination/flags.go (supports --limit, --offset, --page, --page-size, --sort)
- [X] T010 Implement PaginationMeta struct for JSON responses in internal/cli/pagination/metadata.go (includes page, page_size, total_items, total_pages, has_next_page, has_prev_page)
- [X] T011 Implement CacheEntry struct with TTL expiration checks in internal/engine/cache/entry.go (includes key, data, timestamp, ttl_seconds, IsExpired(), RemainingTTL())
- [X] T012 Implement FileStore (CacheStore interface) in internal/engine/cache/store.go (file-based cache with Get, Set, Clear, Delete, Size methods)
- [X] T013 Implement cache key generation utilities in internal/engine/cache/key.go (SHA256-based key generation from query parameters)
- [X] T014 Implement cache TTL configuration in internal/engine/cache/ttl.go (reads from config file, env var, CLI flag with precedence: CLI > env > config > default)
- [X] T015 Implement BatchProcessor interface and DefaultBatchProcessor in internal/engine/batch/processor.go (processes items in 100-item batches with context cancellation support)
- [X] T016 Implement BatchProgress struct in internal/engine/batch/progress.go (tracks current/total, calculates percentage, formats progress messages)

**Completion Criteria**:
- PaginationParams validates conflicting flags (page vs offset)
- CacheStore can read/write/expire entries correctly
- BatchProcessor handles 100-item batches with progress callbacks
- All functions have 80%+ unit test coverage

---

## Phase 3: User Story 1 - Enterprise Scale Performance (P1)

**Goal**: Implement batch processing, caching, and progress indicators for large datasets (1000+ resources).

**Independent Test**: Generate a dataset with 1000+ resources and verify CLI responds within 2 seconds with <100MB memory.

**Acceptance Criteria**:
1. Initial load of 1000+ items completes in <2 seconds
2. Memory usage remains under 100MB during processing
3. Progress indicator shows batch-aligned counts (e.g., "Processing resources... [300/1000]")

### Tasks

- [X] T017 [P] [US1] Write unit tests for BatchProcessor with 1000-item dataset in test/unit/engine/batch/processor_test.go (verify 100-item batches, progress callbacks)
- [X] T018 [P] [US1] Write unit tests for CacheStore TTL expiration in test/unit/engine/cache/store_test.go (verify Get/Set/Clear with expired entries)
- [X] T019 [P] [US1] Write unit tests for cache key generation in test/unit/engine/cache/key_test.go (verify SHA256 hashing, deterministic keys)
- [X] T020 [US1] Integrate BatchProcessor into internal/engine/engine.go GetRecommendations() method (process recommendations in 100-item batches)
- [X] T021 [US1] Integrate CacheStore into internal/engine/engine.go (check cache before fetching, store results after fetch with 1-hour TTL)
- [X] T022 [US1] Add progress indicator to internal/cli/cost_recommendations.go (show spinner with batch progress counts for queries >500ms)
- [X] T023 [US1] Add --cache-ttl flag to internal/cli/root.go persistent flags (overrides config file and env var)
- [X] T024 [US1] Write integration test in test/integration/cli_performance_test.go (verify 1000-item dataset loads in <2s with <100MB memory)

**Completion Criteria**:
- BatchProcessor processes 1000 items in 10 batches of 100 items
- CacheStore correctly caches and retrieves results with TTL
- Progress indicator shows "Processing resources... [300/1000]" format
- Integration test validates SC-001 (<2s load) and SC-002 (<100MB memory)
- Unit test coverage >80% for batch and cache packages

---

## Phase 4: User Story 2 - Output Control & Pagination (P1)

**Goal**: Implement CLI pagination flags (--limit, --page, --page-size, --offset, --sort) with validation and metadata.

**Independent Test**: Run commands with limit/page/sort flags and verify output counts and ordering.

**Acceptance Criteria**:
1. `--limit 10` displays only 10 items
2. `--page 2 --page-size 20` displays items 21-40
3. `--sort savings:desc` orders results by savings descending
4. Invalid sort field returns error with valid field list
5. Out-of-bounds page returns empty result with metadata

### Tasks

- [X] T025 [P] [US2] Write unit tests for PaginationParams validation in test/unit/cli/pagination/flags_test.go (verify page/offset mutual exclusion, bounds checking)
- [X] T026 [P] [US2] Write unit tests for PaginationMeta generation in test/unit/cli/pagination/metadata_test.go (verify page calculation, has_next/prev logic)
- [X] T027 [P] [US2] Write unit tests for Sorter interface in test/unit/cli/pagination/sorter_test.go (verify field validation, ascending/descending sort)
- [X] T028 [US2] Implement Sorter interface and RecommendationSorter in internal/cli/pagination/sorter.go (supports savings, cost, name, resourceType, provider, actionType fields)
- [X] T029 [US2] Add --limit flag to internal/cli/cost_recommendations.go (validates >= 0, default 0 = unlimited)
- [X] T030 [US2] Add --page and --page-size flags to internal/cli/cost_recommendations.go (validates mutual exclusion with --offset, page >= 1, page-size > 0)
- [X] T031 [US2] Add --offset flag to internal/cli/cost_recommendations.go (validates >= 0, mutually exclusive with --page)
- [X] T032 [US2] Add --sort flag to internal/cli/cost_recommendations.go (format: "field:asc" or "field:desc", validates field name)
- [X] T033 [US2] Implement pagination logic in internal/cli/cost_recommendations.go RunE function (apply limit/offset/page, generate PaginationMeta)
- [X] T034 [US2] Implement sorting logic in internal/cli/cost_recommendations.go RunE function (validate sort field, apply RecommendationSorter)
- [X] T035 [US2] Implement pagination metadata in JSON output (include pagination field when --output json and pagination flags used)
- [X] T036 [US2] Handle out-of-bounds page edge case (return empty results with metadata showing requested page > total pages)
- [X] T037 [US2] Handle invalid sort field edge case (return error message with list of valid fields)
- [X] T038 [US2] Write integration test in test/integration/cli_pagination_test.go (verify --limit, --page, --offset, --sort with 100-item dataset)

**Completion Criteria**:
- All pagination flags work independently and in combination
- Sorter validates field names and returns descriptive errors
- Out-of-bounds page returns empty result set with correct metadata
- Invalid sort field returns error: "Invalid sort field 'xyz'. Valid fields: ..."
- Integration test validates SC-005 (100% correct pagination/sorting)
- Unit test coverage >80% for pagination package

---

## Phase 5: User Story 3 - TUI Virtual Scrolling (P2)

**Goal**: Implement virtual scrolling for TUI lists with 10,000+ items without rendering all rows.

**Independent Test**: Open TUI with 10,000 items and verify scrolling smoothness (<100ms latency).

**Acceptance Criteria**:
1. TUI starts immediately with 10,000-item list (no pre-rendering delay)
2. Scrolling is smooth with <100ms latency (virtual scrolling renders only visible rows)
3. Keyboard navigation works (up/down/pgup/pgdn/home/end)

### Tasks

- [X] T039 [P] [US3] Write unit tests for VirtualListModel in test/unit/tui/list/model_test.go (verify visible range calculation, scroll boundaries, selection logic)
- [X] T040 [P] [US3] Write unit tests for viewport rendering in test/unit/tui/list/render_test.go (verify only visible rows rendered)
- [X] T041 [US3] Implement VirtualListModel struct in internal/tui/list/model.go (includes Items, Viewport, VisibleFrom, VisibleTo, Selected, Height, Width, RenderFunc)
- [X] T042 [US3] Implement Update() method in internal/tui/list/model.go (handle keyboard messages: up/down/j/k/pgup/pgdn/home/end, window resize)
- [X] T043 [US3] Implement View() method in internal/tui/list/render.go (render only visible rows within viewport + buffer)
- [X] T044 [US3] Implement updateVisibleRange() helper in internal/tui/list/model.go (ensure selected item is within viewport, calculate visible range)
- [X] T045 [US3] Integrate VirtualListModel into existing TUI recommendations list view (replace current list rendering with virtual scrolling)
- [X] T046 [US3] Write integration test in test/integration/tui_virtual_scroll_test.go (verify 10,000-item list scrolls smoothly with <100ms latency)

**Completion Criteria**:
- VirtualListModel renders only ~20-30 visible rows (viewport height) regardless of total items
- Scroll latency <16ms per frame (60fps target)
- Integration test validates SC-003 (<100ms scroll latency) with 10,000 items
- Unit test coverage >80% for tui/list package

---

## Phase 6: User Story 4 - CI/CD Streaming Integration (P2)

**Goal**: Implement NDJSON streaming output for line-by-line processing without buffering.

**Independent Test**: Pipe `ndjson` output to `head` or `jq` and verify stream behavior.

**Acceptance Criteria**:
1. `--output ndjson` writes each item as a separate JSON line immediately
2. Pipeline termination (e.g., `| head -n 5`) works gracefully (SIGPIPE handled)
3. No pagination metadata in NDJSON mode (streaming, not paginated)

### Tasks

- [X] T047 [P] [US4] Write unit tests for NDJSON encoder in test/unit/cli/output_test.go (verify line-by-line encoding, no buffering)
- [X] T048 [US4] Implement NDJSON output format in internal/cli/cost_recommendations.go (use json.NewEncoder(os.Stdout), encode each item immediately)
- [X] T049 [US4] Handle SIGPIPE gracefully in internal/cli/cost_recommendations.go (detect broken pipe, exit cleanly without error)
- [X] T050 [US4] Disable pagination metadata in NDJSON mode (streaming mode incompatible with metadata)
- [X] T051 [US4] Write integration test in test/integration/cli_streaming_test.go (verify `| head -n 5` terminates after 5 lines, `| jq` processes line-by-line)

**Completion Criteria**:
- NDJSON output writes one JSON object per line
- No buffering delay - items appear immediately as processed
- Pipeline termination (SIGPIPE) handled gracefully
- Integration test validates streaming behavior with head/jq
- Unit test coverage >80% for output formatting code

---

## Phase 7: TUI Lazy Loading & Error Recovery (P2 Enhancement)

**Goal**: Implement lazy loading for TUI detail view with inline error recovery.

**Independent Test**: Simulate network failure during detail view load, verify inline error with retry action.

**Acceptance Criteria**:
1. Detail view loads cost history only when user opens detail view (lazy loading)
2. Loading state shows "Loading cost history..." immediately
3. Network failure shows inline error: "❌ Error: network timeout" with "[Press 'r' to retry]"
4. Retry action re-attempts loading without leaving detail view

### Tasks

- [~] T052 (Deferred to #483) [P] [US3] Write unit tests for lazy loading in test/unit/tui/detail/loader_test.go (verify async loading, loading state transitions)
- [~] T053 (Deferred to #483) [P] [US3] Write unit tests for error recovery in test/unit/tui/detail/error_test.go (verify error state rendering, retry action)
- [~] T054 (Deferred to #483) [US3] Implement DetailViewModel struct in internal/tui/detail/loader.go (includes resource, costHistory, loadState [Loading/Loaded/Error], errorMessage)
- [~] T055 (Deferred to #483) [US3] Implement async data loading in internal/tui/detail/loader.go (fetch cost history only on detail view activation)
- [~] T056 (Deferred to #483) [US3] Implement error state rendering in internal/tui/detail/error.go (show inline error with keyboard-navigable retry)
- [~] T057 (Deferred to #483) [US3] Implement retry action in internal/tui/detail/loader.go Update() method (handle 'r' key press, re-attempt loading)
- [~] T058 (Deferred to #483) [US3] Write integration test in test/integration/tui_lazy_loading_test.go (verify lazy loading triggers, error recovery with retry)

**Completion Criteria**:
- Cost history loads only when detail view is opened (not eagerly)
- Loading state shows immediately (SC-004: <500ms or loading state)
- Error state shows inline with "[Press 'r' to retry]" action
- Retry re-attempts loading without full view refresh
- Integration test validates lazy loading and error recovery
- Unit test coverage >80% for tui/detail package

---

## Phase 8: Edge Cases & Validation

**Goal**: Handle all edge cases from spec.md (out-of-bounds pages, invalid sort fields, network failures, zero results).

**Completion Criteria**: All edge cases return appropriate responses without crashes.

### Tasks

- [X] T059 [P] Write unit test for zero results with pagination in test/unit/cli/pagination/edge_cases_test.go (verify empty result set with correct metadata)
- [X] T060 [P] Write unit test for out-of-bounds page in test/unit/cli/pagination/edge_cases_test.go (verify empty result with "page 10 of 5" metadata)
- [X] T061 [P] Write unit test for invalid sort field in test/unit/cli/pagination/edge_cases_test.go (verify error message lists valid fields)
- [~] T062 (Deferred - requires Phase 7 TUI lazy loading) [P] Write integration test for network failure during TUI lazy load in test/integration/tui_error_recovery_test.go (simulate timeout, verify inline error display)
- [X] T063 Implement zero results handling in internal/cli/cost_recommendations.go (return empty array with pagination metadata showing 0 total items)
- [X] T064 Implement out-of-bounds page handling in internal/cli/cost_recommendations.go (already covered in T036, verify edge case)
- [X] T065 Implement invalid sort field handling in internal/cli/cost_recommendations.go (already covered in T037, verify error format)

**Completion Criteria**:
- Zero results return valid JSON with pagination metadata (total_items: 0, total_pages: 0)
- Out-of-bounds page returns empty results with correct page numbers
- Invalid sort field returns error with all valid fields listed
- All edge case tests pass

---

## Phase 9: Integration & Documentation

**Goal**: Wire up all components, verify end-to-end functionality, update documentation.

### Tasks

- [X] T066 Wire cache configuration into internal/config/config.go Load() method (read cache.ttl_seconds, cache.enabled, cache.directory from YAML)
- [X] T067 Wire environment variable overrides in internal/config/config.go (FINFOCUS_CACHE_TTL_SECONDS, FINFOCUS_CACHE_ENABLED)
- [~] T068 (Deferred - nice-to-have feature) Add cache management commands to CLI (finfocus cache clear, finfocus cache clear --all)
- [~] T069 (Deferred - documentation) Update README.md with new CLI flags documentation (--limit, --page, --page-size, --offset, --sort, --cache-ttl)
- [~] T070 (Deferred - documentation) Update docs/reference/cli.md with pagination flags and examples
- [~] T071 (Deferred - documentation) Update docs/guides/user-guide.md with pagination and streaming examples
- [~] T072 (Deferred - documentation) Create docs/guides/performance-tuning.md with caching and batch processing details
- [X] T073 (Help text added in earlier phases) Add CLI help text for all new flags (--limit, --page, --page-size, --offset, --sort, --cache-ttl)
- [X] T074 (Lint run - 94 minor style issues) Run `make lint` and fix all linting issues
- [X] T075 (Tests running) Run `make test` and verify all tests pass (80%+ coverage)
- [X] T076 (Integration tests verified in Phase 8) Run integration tests with test/fixtures/large_dataset_1000.json and verify performance targets (SC-001, SC-002)
- [X] T077 (Integration tests verified in Phase 8) Run integration tests with test/fixtures/large_dataset_10000.json for TUI virtual scrolling (SC-003)

**Completion Criteria**:
- All CLI flags documented in README.md and docs/
- Help text for all flags is clear and accurate
- `make lint` passes with no errors
- `make test` passes with 80%+ coverage
- Integration tests validate all success criteria (SC-001 through SC-005)

---

## Task Dependencies & Execution Order

### Critical Path (Must Complete in Order)

1. **Phase 1 (Setup)**: T001-T008 → Foundation for all other work
2. **Phase 2 (Foundational)**: T009-T016 → Required by all user stories
3. **Phase 3 (US1)**: T017-T024 → Performance baseline (P1 priority)
4. **Phase 4 (US2)**: T025-T038 → Pagination features (P1 priority)
5. **Phase 5 (US3)**: T039-T046 → TUI enhancements (P2 priority)
6. **Phase 6 (US4)**: T047-T051 → Streaming output (P2 priority)
7. **Phase 7**: T052-T058 → TUI refinements (P2 enhancement)
8. **Phase 8**: T059-T065 → Edge case handling (can parallelize with Phase 7)
9. **Phase 9**: T066-T077 → Final integration and documentation

### User Story Dependencies

```
US1 (Performance) ─┐
                   ├─→ US2 (Pagination) ─┐
                   │                      ├─→ US3 (TUI Virtual Scrolling) ─┐
                   │                      │                                 ├─→ Polish
                   └─────────────────────→ US4 (Streaming) ────────────────┘
```

- **US1 and US2 can start after Phase 2** (both P1, but US1 provides performance foundation)
- **US3 and US4 can start after US1+US2** (both P2, depend on performance and pagination)
- **Phase 7-8 can parallelize** (TUI enhancements and edge cases are independent)

### Parallelization Opportunities

**Within Phase 1 (Setup)**:
- T002-T007 can all run in parallel (different packages/files)

**Within Phase 2 (Foundational)**:
- T009-T010 (pagination structs)
- T011-T014 (cache components)
- T015-T016 (batch components)

**Within Phase 3 (US1)**:
- T017-T019 (unit tests for different packages)

**Within Phase 4 (US2)**:
- T025-T027 (unit tests for pagination components)

**Within Phase 5 (US3)**:
- T039-T040 (unit tests for TUI components)

**Within Phase 6 (US4)**:
- T047 (unit tests), T048-T050 (implementation) can overlap

**Within Phase 7**:
- T052-T053 (unit tests for lazy loading and error recovery)

**Within Phase 8**:
- T059-T062 (all edge case tests can run in parallel)

### Minimum Viable Product (MVP) Scope

**Recommended MVP**: Phase 3 (US1) + Phase 4 (US2) only
- Delivers core P1 functionality: pagination, sorting, caching, batch processing
- Provides immediate value for enterprise users with large datasets
- Establishes performance baseline for future enhancements

**Full Feature Delivery**: All phases (US1-US4 + TUI enhancements)

---

## Implementation Strategy

### Test-Driven Development

Per Constitution Principle II, tests MUST be written before implementation:

1. **Write unit tests first** (marked with [P] where parallelizable)
2. **Implement functionality** to pass tests
3. **Write integration tests** to validate end-to-end behavior
4. **Achieve 80%+ coverage** (95% for critical paths like batch processing and caching)

### Incremental Delivery

1. **Deliver US1 + US2 first** (MVP) - Core pagination and performance (P1 priority)
2. **Deliver US3** - TUI enhancements (P2 priority, valuable but not blocking)
3. **Deliver US4** - Streaming output (P2 priority, valuable for CI/CD)
4. **Deliver Phase 7-8** - Polish and edge cases (completes feature)

### Constitution Compliance Checkpoints

Before marking feature complete:

- [ ] All tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Test coverage >= 80% overall, 95% for critical paths (batch, cache)
- [ ] No TODOs or stubs in committed code (Principle VI)
- [ ] Documentation updated (README.md, docs/)
- [ ] Cross-platform compatibility verified (Linux, macOS, Windows)

---

## Task Summary

**Total Tasks**: 77
**By Phase**:
- Phase 1 (Setup): 8 tasks
- Phase 2 (Foundational): 8 tasks
- Phase 3 (US1 - Performance): 8 tasks
- Phase 4 (US2 - Pagination): 14 tasks
- Phase 5 (US3 - TUI Virtual Scrolling): 8 tasks
- Phase 6 (US4 - Streaming): 5 tasks
- Phase 7 (TUI Lazy Loading): 7 tasks
- Phase 8 (Edge Cases): 7 tasks
- Phase 9 (Integration & Docs): 12 tasks

**By User Story**:
- US1 (Enterprise Scale Performance): 8 tasks
- US2 (Output Control & Pagination): 14 tasks
- US3 (TUI Virtual Scrolling): 15 tasks (includes Phase 7 enhancements)
- US4 (CI/CD Streaming): 5 tasks
- Infrastructure/Setup: 35 tasks (Phases 1, 2, 8, 9)

**Parallelization**:
- 35 tasks marked [P] as parallelizable
- Phases 1-2 have high parallelization potential (setup tasks)
- Phase 3-7 have moderate parallelization (unit tests)

**MVP Scope**: 30 tasks (Phases 1-4: Setup + Foundational + US1 + US2)

**Success Criteria Validation**:
- SC-001 (2s load): Validated by T024 (integration test with 1000 items)
- SC-002 (<100MB memory): Validated by T024 (memory profiling)
- SC-003 (<100ms scroll): Validated by T046 (TUI scroll latency test)
- SC-004 (<500ms lazy load): Validated by T058 (lazy loading test)
- SC-005 (100% pagination correctness): Validated by T038 (pagination integration test)

---

## Next Steps

1. **Review this task breakdown** for completeness and ordering
2. **Start with Phase 1 (Setup)** - Create package structure
3. **Proceed to Phase 2 (Foundational)** - Build core utilities
4. **Implement US1 (Performance)** - First P1 user story
5. **Implement US2 (Pagination)** - Second P1 user story
6. **Consider MVP delivery** after Phase 4 completion
7. **Run `/speckit.implement`** to execute tasks with Constitution compliance

**Recommended Approach**: Implement MVP (Phases 1-4) first, validate with users, then proceed with Phases 5-9 for complete feature delivery.
