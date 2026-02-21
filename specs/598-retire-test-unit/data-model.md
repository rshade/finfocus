# Data Model: Retire test/unit/ and Fix Test Discovery

**Branch**: `598-retire-test-unit` | **Date**: 2026-02-20

No production data model changes. This document captures the **file inventory**,
**classification**, and **migration mapping** that serves as the authoritative reference
for task execution.

---

## File Inventory & Classification

All 34 files in `test/unit/`. Classification is preliminary — FR-003 requires
per-function code comparison before finalizing. See research.md Decision 2.

**Legend**:
- `MERGE` — destination file exists; functions must be merged (unique kept, duplicates dropped)
- `MV` — destination file does NOT exist; clean `git mv` applies
- `VERIFY` — file exists in destination with same name; verify function overlap first

### Package: `cli/` (11 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 1 | `cli/cost_actual_test.go` | `cli/cost_actual_test.go` | MERGE | Different functions confirmed |
| 2 | `cli/cost_projected_test.go` | `cli/cost_projected_test.go` | MERGE | Verify functions |
| 3 | `cli/flags_test.go` | `cli/flags_test.go` | MV | No destination exists |
| 4 | `cli/output_test.go` | `cli/output_test.go` | MV | No destination exists |
| 5 | `cli/plugin_install_fallback_test.go` | `cli/plugin_install_fallback_test.go` | MV | No destination exists |
| 6 | `cli/plugin_test.go` | `cli/plugin_test.go` | MERGE | Multiple plugin_* files exist in internal/cli; verify functions |
| 7 | `cli/prompt_test.go` | `cli/prompt_test.go` | MV | No destination exists |

### Package: `cli/pagination/` (4 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 8 | `cli/pagination/edge_cases_test.go` | `cli/pagination/edge_cases_test.go` | MV | No destination exists |
| 9 | `cli/pagination/flags_test.go` | `cli/pagination/flags_test.go` | MV | No destination exists |
| 10 | `cli/pagination/metadata_test.go` | `cli/pagination/metadata_test.go` | MV | No destination exists |
| 11 | `cli/pagination/sorter_test.go` | `cli/pagination/sorter_test.go` | MV | No destination exists |

### Package: `config/` (4 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 12 | `config/budget_scoped_test.go` | `config/budget_scoped_test.go` | MV | No same-named destination |
| 13 | `config/config_test.go` | `config/config_test.go` | MERGE | Verify functions vs existing |
| 14 | `config/env_test.go` | `config/env_test.go` | MV | No destination exists |
| 15 | `config/load_test.go` | `config/load_test.go` | MV | No destination exists |

### Package: `engine/` (3 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 16 | `engine/budget_scope_test.go` | `engine/budget_scope_test.go` | MV | No same-named destination |
| 17 | `engine/engine_test.go` | `engine/engine_test.go` | MERGE | Different functions confirmed |
| 18 | `engine/render_test.go` | `engine/render_test.go` | MV | No destination exists |

### Package: `engine/batch/` (1 file)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 19 | `engine/batch/processor_test.go` | `engine/batch/processor_test.go` | MV | Destination dir exists; no file conflict |

### Package: `engine/cache/` (2 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 20 | `engine/cache/key_test.go` | `engine/cache/key_test.go` | MV | No destination exists |
| 21 | `engine/cache/store_test.go` | `engine/cache/store_test.go` | MV | No destination exists |

### Package: `ingest/` (2 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 22 | `ingest/mapper_test.go` | `ingest/mapper_test.go` | MV | `map_resource_test.go` exists but different name |
| 23 | `ingest/plan_test.go` | `ingest/plan_test.go` | VERIFY | `pulumi_plan_test.go` exists; verify no function overlap |

### Package: `pluginhost/` (3 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 24 | `pluginhost/client_test.go` | `pluginhost/client_test.go` | MERGE | `client_test.go` exists in internal/ |
| 25 | `pluginhost/discovery_test.go` | `pluginhost/discovery_test.go` | MV | No destination exists |
| 26 | `pluginhost/lifecycle_test.go` | `pluginhost/lifecycle_test.go` | MV | No destination exists |

### Package: `registry/` (3 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 27 | `registry/fallback_test.go` | `registry/fallback_test.go` | MV | No destination exists |
| 28 | `registry/manifest_test.go` | `registry/manifest_test.go` | MV | No destination exists |
| 29 | `registry/scan_test.go` | `registry/scan_test.go` | MV | `registry_test.go` exists but different name |

### Package: `spec/` (3 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 30 | `spec/load_test.go` | `spec/load_test.go` | MV | `loader_test.go` exists but different name |
| 31 | `spec/parse_test.go` | `spec/parse_test.go` | MV | No destination exists |
| 32 | `spec/spec_test.go` | `spec/spec_test.go` | MV | No destination exists |

### Package: `tui/list/` (2 files)

| # | Source (`test/unit/`) | Destination (`internal/`) | Classification | Notes |
|---|---|---|---|---|
| 33 | `tui/list/model_test.go` | `tui/list/model_test.go` | MV | No tests in destination dir |
| 34 | `tui/list/render_test.go` | `tui/list/render_test.go` | MV | No tests in destination dir |

---

## Summary Counts (Preliminary)

| Classification | Count | Action |
|---|---|---|
| MV (clean move) | 24 | `git mv source destination`, verify compile |
| MERGE (function merge) | 8 | Audit functions, merge unique, drop duplicates |
| VERIFY (needs deeper check) | 2 | Run function-level diff before deciding |
| **Total** | **34** | |

---

## Non-Entities (Out of Scope)

- `plugins/recorder/*_test.go` — in `plugins/recorder/`, not `test/unit/`; tracked separately
- `test/integration/` — integration tests; not being moved
- `test/e2e/` — E2E tests; not being moved
- `test/mocks/` — Mock implementations; not being moved (may be imported by migrated tests)
- `test/fixtures/` — Test data; not being moved (path references in migrated tests must remain valid)

---

## Import Path Considerations

Migrated tests that import from `test/mocks/` must retain those import paths unchanged:

```go
// This import is valid before and after migration (test/mocks/ is not being moved)
import "github.com/rshade/finfocus/test/mocks/plugin"
```

Tests that import from `test/unit/` helper packages (if any exist) must have those
helpers moved inline or to a shared `internal/testutil/` package as appropriate.

---

## Template Entity

| File | Current Value | Required Value |
|---|---|---|
| `.specify/templates/tasks-template.md` line 160 | `in tests/unit/` | `colocated in \`internal/[package]/[feature]_test.go\`` |
