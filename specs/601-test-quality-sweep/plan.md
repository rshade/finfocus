# Implementation Plan: Test Quality Sweep

**Branch**: `601-test-quality-sweep` | **Date**: 2026-02-22 | **Spec**: `specs/601-test-quality-sweep/spec.md`
**Input**: Feature specification from `/specs/601-test-quality-sweep/spec.md`

## Summary

Batch test quality improvements across 11 GitHub issues: fix vacuous tests, close
leaked resources, ensure hermetic env isolation, consolidate duplicate tests into
table-driven suites, fix masked/always-skipped integration tests, verify defensive
copies, and fix test data quality issues. All changes are test-only -- no production
code modifications.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: `github.com/stretchr/testify` (assertions, already a dep)
**Storage**: N/A (test-only changes, no persistent storage)
**Testing**: `go test`, `make test`, `make lint`
**Target Platform**: Linux, macOS, Windows (cross-platform, same as existing)
**Project Type**: Single Go module
**Performance Goals**: N/A (no runtime performance impact)
**Constraints**: Zero regressions; `make test` and `make lint` must pass
**Scale/Scope**: ~15 test files across 6 packages; net reduction of ~300+ lines

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: N/A -- test-only changes, no production code
- [x] **Test-Driven Development**: All changes improve test quality; 80%+ coverage maintained
- [x] **Cross-Platform Compatibility**: Test changes are platform-agnostic
- [x] **Documentation Integrity**: N/A -- no API or doc changes needed
- [x] **Protocol Stability**: N/A -- no protocol changes
- [x] **Implementation Completeness**: All 11 issues will be fully resolved, no stubs
- [x] **Quality Gates**: `make test` + `make lint` required before completion
- [x] **Multi-Repo Coordination**: N/A -- all changes within finfocus-core only

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/601-test-quality-sweep/
├── plan.md              # This file
├── research.md          # Phase 0 output (below)
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (files to modify)

```text
internal/
├── cli/
│   ├── cost_projected_test.go       # #782: consolidate 4 duplicate tests
│   ├── plugin_validate_test.go      # #774: use setupTestEnv, precise assertions
│   └── init_cache_test.go           # #683: wantNil field usage (already correct)
├── config/
│   ├── config_test.go               # #784: stubHome must clear FINFOCUS_HOME
│   └── budget_scoped_test.go        # #786: set ExitOnThreshold=true
├── engine/
│   ├── cache/
│   │   ├── cache_test.go            # #683: fix string(rune()) data generation
│   │   └── store_test.go            # #683: wantNil field + string(rune()) fix
│   ├── budget_health_test.go        # #683: fix string(rune()) data generation
│   └── overview_enrich_test.go      # #683: unused wantNilErr field
├── ingest/
│   └── pulumi_plan_test.go          # #776: merge flat tests into table-driven
├── pluginhost/
│   └── client_test.go               # #775: consolidate 5 GetPluginInfo tests
│                                     # #785: add defer client.Close()
└── tui/
    └── overview_model_test.go        # #722: verify defensive copy independence

test/integration/
├── plugin_version_test.go           # #737: gate behind build tag + env check
├── cli_streaming_test.go            # #737: conditional skip on binary availability
├── audit_test.go                    # #743: already gated by nightly build tag
└── trace_propagation_test.go        # #743: already gated by nightly build tag
```

**Structure Decision**: No new files or directories. All changes are edits to
existing test files within the established Go package structure.

## Complexity Tracking

No constitution violations. No complexity justification needed.

---

## Phase 0: Research

### R-001: Vacuous Test - ExitOnThreshold (Issue #786)

**Decision**: Add `ExitOnThreshold: ptr(true)` to the "valid exit code 0" test case
in `budget_scoped_test.go:152-158`.

**Rationale**: Without `ExitOnThreshold: true`, an `ExitCode` of `ptr(0)` is
meaningless -- the validation path that checks exit code ranges is only exercised
when `ExitOnThreshold` is enabled. The test currently passes vacuously.

**Alternatives considered**: Adding a separate test case instead of fixing the
existing one -- rejected because the existing case name ("valid exit code 0
(warning mode)") explicitly describes the scenario that requires ExitOnThreshold.

---

### R-002: Leaked Plugin Clients (Issue #785)

**Decision**: Add `defer client.Close()` after successful client creation in
`TestNewClient_Success` and `TestClient_APIUsage` in `client_test.go`.

**Rationale**: Plugin clients hold gRPC connections and goroutines. Without Close(),
the race detector may report leaked goroutines, causing flaky CI failures.

**Alternatives considered**: Using `t.Cleanup()` -- rejected because `defer
client.Close()` is the standard pattern used throughout the codebase and is simpler.

---

### R-003: Hermetic Env Isolation (Issue #784)

**Decision**: Add `t.Setenv("FINFOCUS_HOME", "")` to `stubHome(t)` in
`config_test.go:14-21`.

**Rationale**: `GetConfigDir()` checks `FINFOCUS_HOME` first (highest priority).
If a developer has this set, config tests silently resolve to their real config
directory instead of the test temp dir.

**Alternatives considered**: Using `t.Setenv("FINFOCUS_HOME", dir)` pointing to
the temp dir -- rejected because the intent of stubHome is to isolate via `HOME`,
and explicitly clearing `FINFOCUS_HOME` ensures the `HOME`-based path resolution
is exercised.

---

### R-004: Consolidate Cost Projected Tests (Issue #782)

**Decision**: Consolidate `TestCostProjectedCmd_TableOutput`,
`TestCostProjectedCmd_NDJSONOutput`, `TestCostProjectedCmd_FilterByType`, and
`TestCostProjectedCmd_FilterByProvider` into a single table-driven test.

**Rationale**: All four tests share identical setup (mock plan JSON, create command,
execute, check output). Only the flags and expected output fragments differ -- perfect
candidates for table-driven consolidation.

**Alternatives considered**: Keeping separate tests with a shared helper -- rejected
because table-driven is the project standard and eliminates more duplication.

---

### R-005: Consolidate Pulumi Plan Flat Tests (Issue #776)

**Decision**: Merge remaining flat tests in `pulumi_plan_test.go` into the existing
table-driven suites, converting manual `t.Errorf`/`t.Fatalf` to testify assertions
and removing duplicate cases.

**Rationale**: The file already has well-structured table-driven tests
(`getLoadPulumiPlanTestData`, `getPulumiPlanGetResourcesTestData`). Any remaining
flat tests should be absorbed into these suites for consistency. Note: recent PRs
(#794, #795) may have already addressed some of this -- verify current state before
making changes.

**Alternatives considered**: None -- table-driven consolidation is the clear approach.

---

### R-006: Consolidate GetPluginInfo Tests (Issue #775)

**Decision**: Consolidate 5 `TestGetPluginInfo_*` functions (lines 104-214 in
`client_test.go`) into a single table-driven test with struct fields for:
mock response, mock error, strict mode, expected error, expected log messages.

**Rationale**: All five functions follow the same pattern: create mock server,
configure response, create client, call GetPluginInfo, assert result. The only
variations are the mock response and assertion.

**Alternatives considered**: None -- straightforward table-driven consolidation.

---

### R-007: Consolidate Plugin Validate Tests (Issue #774)

**Decision**: Ensure all test functions in `plugin_validate_test.go` use the
existing `setupTestEnv(t)` helper. Replace fragile substring assertions
(`strings.Contains`) with precise testify assertions (`assert.Contains` with
exact expected values).

**Rationale**: `setupTestEnv(t)` already exists (line 18-23) and handles the
common `FINFOCUS_LOG_LEVEL` and `FINFOCUS_HOME` setup. Some tests may duplicate
this setup inline.

**Alternatives considered**: None -- the helper already exists.

---

### R-008: Masked Errors in Integration Tests (Issue #743)

**Decision**: For integration tests with `//go:build nightly` tag (audit_test.go,
trace_propagation_test.go), the `testing.Short()` gating is correct behavior --
these run only in nightly CI. For any integration tests that suppress logs globally,
route log output through `zerolog.TestWriter(t)` so WARN/ERROR messages appear in
test output when tests fail.

**Rationale**: `zerolog.TestWriter(t)` routes log output through Go's testing.T,
which means logs are only shown when tests fail (or with `-v`). This preserves
clean output for passing tests while making error diagnostics visible on failure.

**Alternatives considered**: Using `zerolog.ConsoleWriter` -- rejected because
TestWriter integrates with Go's test output buffering and is the recommended
pattern for test logging.

---

### R-009: Always-Skipped Integration Tests (Issue #737)

**Decision**: The three tests in `plugin_version_test.go` are permanently skipped
stubs. They are already gated behind the `integration` build tag. Options:

1. Make them check `FINFOCUS_TEST_PLUGIN_PATH` env var and skip only when unset
2. Gate behind a more specific build tag like `pluginbinary`

Choose option 1: check env var, skip with descriptive message when unset, execute
the real test when the env var provides a plugin binary path.

For `cli_streaming_test.go`: skips are conditional on binary availability (correct
behavior -- `t.Skip("finfocus binary not available")`). These are NOT always-skipped.

**Rationale**: The plugin_version_test.go tests have documented prerequisites and
the unit tests already cover the logic. Making them conditional on an env var lets
CI enable them when binaries are available without requiring build tag changes.

**Alternatives considered**: Deleting the tests -- rejected because they provide
value when a plugin binary is available. Building the plugin in TestMain -- rejected
because it introduces cross-package build dependencies in test setup.

---

### R-010: Defensive Copy Verification (Issue #722)

**Decision**: In `TestOverviewModel_DataReadyMsg` (`overview_model_test.go`), after
the model update, mutate the original `testRows` slice (e.g., change a URN) and
assert the model's internal state is unaffected.

**Rationale**: The current test verifies the state transition but not that the copy
is independent. A simple mutation-after-copy test proves defensive copy semantics.

**Alternatives considered**: Using `reflect.DeepEqual` before/after -- rejected
because direct field assertion is clearer and follows the project's testify pattern.

---

### R-011: Test Data Quality (Issue #683)

**Decision**: Three sub-fixes:

1. Replace `string(rune(i))` / `string(rune('0'+i%10))` with `fmt.Sprintf` or
   `strconv.Itoa` in `budget_health_test.go`, `cache_test.go`, `store_test.go`
2. Make recommendation key assertions check exact values (not just prefix)
3. Verify `wantNilErr` field in `overview_enrich_test.go` test structs -- analysis
   confirms the field IS used in assertions at line ~698 (no code changes needed)

**Rationale**: `string(rune(0))` produces a null byte; `string(rune(1))` produces
SOH control character. These are not printable and can mask encoding bugs. Using
`fmt.Sprintf("item-%d", i)` produces clean, debuggable test data.

**Alternatives considered**: None -- the fix is straightforward.

---

### R-012: InitCache Test Isolation (Issue #683 / FR-014)

**Decision**: The `init_cache_test.go` already uses `t.Setenv(cache.EnvCacheDir,
t.TempDir())` for isolation (line 97). The `wantNil` field IS used in assertions
(lines 108-115). No changes needed for this file -- it's already correctly isolated
and structured.

**Rationale**: Verified by reading the test file -- all test cases properly set
`wantNil` and it's consumed in the if/else assertion block.

**Alternatives considered**: N/A -- file is already correct.

---

## Phase 1: Design

### No Data Model or API Contracts Needed

This feature is entirely test-quality improvements. There are no new data types,
API endpoints, or external contracts to design. The "design" phase consists of
the change patterns documented in the research above.

### Change Pattern Summary

| Pattern | Files Affected | Approach |
|---------|---------------|----------|
| Fix vacuous assertion | budget_scoped_test.go | Add missing field value |
| Add defer Close() | client_test.go (2 sites) | Insert defer after creation |
| Clear env var | config_test.go | Add t.Setenv line to helper |
| Table-driven consolidation | cost_projected_test.go, client_test.go, plugin_validate_test.go | Merge N tests into 1 with table |
| Merge flat into existing suite | pulumi_plan_test.go | Absorb into existing table |
| Test-aware logging | Integration tests (if needed) | zerolog.TestWriter(t) |
| Conditional skip | plugin_version_test.go | Check env var before skip |
| Mutation-after-copy test | overview_model_test.go | Append mutation assertion |
| Fix string generation | 4 files | Replace rune cast with Sprintf |
| Remove unused field | overview_enrich_test.go | Delete struct field |

### Risk Assessment

- **Low risk**: All changes are test-only; production code is untouched
- **Regression risk**: Table-driven consolidation could accidentally drop a unique
  assertion -- mitigate by diffing before/after test names with `go test -v`
- **CI risk**: Making always-skipped tests runnable could expose latent failures
  in CI -- mitigate by keeping env-var gating for external dependencies

## Post-Design Constitution Re-Check

- [x] **Test-Driven Development**: Improvements strengthen TDD compliance
- [x] **Implementation Completeness**: All 11 issues fully addressed, no stubs
- [x] **Quality Gates**: Plan requires `make test` + `make lint` verification

---

## Next Steps

Run `/speckit.tasks` to generate the ordered task list from this plan.
