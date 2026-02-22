# Research: Test Quality Sweep

**Branch**: `601-test-quality-sweep` | **Date**: 2026-02-22

## R-001: Vacuous Test - ExitOnThreshold (#786)

**File**: `internal/config/budget_scoped_test.go:152-158`

**Current State**: The "valid exit code 0 (warning mode)" test case creates a
`ScopedBudget` with `ExitCode: ptr(0)` but omits `ExitOnThreshold`. Without
`ExitOnThreshold: true`, the validation path that checks exit code ranges is
never exercised -- the test passes vacuously.

**Decision**: Add `ExitOnThreshold: ptr(true)` to the test case.
**Rationale**: The case name describes "warning mode" which requires ExitOnThreshold.
**Alternatives**: Separate test case -- rejected (existing case already describes this).

---

## R-002: Leaked Plugin Clients (#785)

**Files**: `internal/pluginhost/client_test.go` -- `TestNewClient_Success` (line ~217)
and `TestClient_APIUsage` (line ~341)

**Current State**: Both tests create plugin clients via `pluginhost.NewClient()` but
never call `client.Close()`. The gRPC connection and associated goroutines leak.

**Decision**: Add `defer client.Close()` after each successful client creation.
**Rationale**: Standard cleanup pattern; prevents goroutine leaks detected by `-race`.
**Alternatives**: `t.Cleanup()` -- rejected (defer is simpler and matches codebase style).

---

## R-003: Hermetic Env Isolation (#784)

**File**: `internal/config/config_test.go:14-21`

**Current State**: `stubHome(t)` sets `HOME` and `USERPROFILE` to a temp dir but
does not clear `FINFOCUS_HOME`. Since `GetConfigDir()` checks `FINFOCUS_HOME` first,
if a developer has it set, tests silently use their real config directory.

**Decision**: Add `t.Setenv("FINFOCUS_HOME", "")` to `stubHome(t)`.
**Rationale**: Ensures HOME-based resolution is exercised regardless of developer env.
**Alternatives**: Set FINFOCUS_HOME to temp dir -- rejected (intent is to test HOME path).

---

## R-004: Consolidate Cost Projected Tests (#782)

**File**: `internal/cli/cost_projected_test.go`

**Current State**: Four near-identical test functions:

- `TestCostProjectedCmd_TableOutput` (~line 447)
- `TestCostProjectedCmd_NDJSONOutput` (~line 474)
- `TestCostProjectedCmd_FilterByType` (~line 505)
- `TestCostProjectedCmd_FilterByProvider` (~line 543)

All share: create mock plan JSON, build command, set flags, execute, check output.

**Decision**: Single table-driven test with struct fields for flags and expected output.
**Rationale**: ~350 lines of duplication eliminated.
**Alternatives**: Shared helper with separate tests -- rejected (table-driven is cleaner).

---

## R-005: Consolidate Pulumi Plan Flat Tests (#776)

**File**: `internal/ingest/pulumi_plan_test.go`

**Current State**: The file already has well-structured table-driven suites. Recent
PRs #794 and #795 consolidated many flat tests. Verify remaining flat tests and
merge any stragglers.

**Decision**: Merge remaining flat tests into existing table-driven suites; convert
manual assertions to testify.
**Rationale**: Consistency with existing table-driven pattern.
**Alternatives**: None.

---

## R-006: Consolidate GetPluginInfo Tests (#775)

**File**: `internal/pluginhost/client_test.go:104-214`

**Current State**: Five separate functions:

1. `TestGetPluginInfo_Success` (104-125)
2. `TestGetPluginInfo_Unimplemented` (127-143)
3. `TestGetPluginInfo_Timeout` (145-165)
4. `TestGetPluginInfo_StrictMode_BlocksIncompatible` (167-189)
5. `TestGetPluginInfo_PermissiveMode_AllowsIncompatible` (191-214)

All follow: create mock server, configure response, create client, call method, assert.

**Decision**: Single table-driven test with struct for mock config, strict mode, and
expected behavior.
**Rationale**: Reduces ~110 lines to ~60 while preserving all scenarios.
**Alternatives**: None.

---

## R-007: Consolidate Plugin Validate Tests (#774)

**File**: `internal/cli/plugin_validate_test.go`

**Current State**: `setupTestEnv(t)` exists at line 18-23. Some test functions may
duplicate its setup inline. Substring assertions used where exact values are known.

**Decision**: Ensure all tests use `setupTestEnv(t)`. Replace `strings.Contains`
with testify `assert.Contains` using exact expected values.
**Rationale**: DRY principle; precise assertions catch more bugs.
**Alternatives**: None.

---

## R-008: Masked Errors in Integration Tests (#743)

**Files**: `test/integration/audit_test.go`, `test/integration/trace_propagation_test.go`

**Current State**: Both files use `//go:build nightly` tag and `testing.Short()`
checks. These are already properly gated -- they run in nightly CI, not in regular
`make test`. No global log suppression found (`zerolog.Nop()` not used).

**Decision**: Verify no WARN/ERROR suppression exists. If `setupTestEnv` in any
integration test sets `FINFOCUS_LOG_LEVEL=error`, consider routing through
`zerolog.TestWriter(t)` for better diagnostics.
**Rationale**: Test-aware logging preserves clean output while showing errors on failure.
**Alternatives**: None needed if no suppression found.

---

## R-009: Always-Skipped Integration Tests (#737)

**File**: `test/integration/plugin_version_test.go`

**Current State**: Three test functions are always-skipped stubs with `t.Skip()`:

- `TestPluginInitialization_CompatibleVersion` (line 41)
- `TestPluginInitialization_IncompatibleVersion_Warning` (line 49)
- `TestPluginInitialization_LegacyPlugin_NoGetPluginInfo` (line 57)

Already gated behind `//go:build integration` tag. The package doc (lines 5-29)
documents prerequisites and notes that unit tests provide coverage.

**Decision**: Replace unconditional `t.Skip()` with env-var check:
`if os.Getenv("FINFOCUS_TEST_PLUGIN_PATH") == ""`. Implement actual test logic
that uses the provided binary path.
**Rationale**: Allows CI to opt-in when binaries are available; preserves skip for
environments without prerequisites.
**Alternatives**: Delete tests (rejected -- valuable when binaries available).
Build in TestMain (rejected -- cross-package build dependency).

---

## R-010: Defensive Copy Verification (#722)

**File**: `internal/tui/overview_model_test.go:547-573`

**Current State**: `TestOverviewModel_DataReadyMsg` creates `testRows`, sends as
message, verifies model state -- but doesn't verify copy independence.

**Decision**: After model update, mutate `testRows[0].URN` and assert
`model.allRows[0].URN` is unchanged.
**Rationale**: Proves defensive copy works; catches regressions if copy is removed.
**Alternatives**: reflect.DeepEqual -- rejected (direct field assert is clearer).

---

## R-011: Test Data Quality (#683)

**Files affected**:

- `internal/engine/budget_health_test.go:458-459` -- `string(rune('0'+i%10))`
- `internal/engine/cache/cache_test.go:355,365` -- `string(rune('A'+idx%26))`
- `internal/engine/cache/store_test.go:271` -- `string(rune('0'+i))`
- `internal/engine/overview_enrich_test.go:646-674` -- unused `wantNilErr` field

**Current State**: `string(rune(0))` = null byte, `string(rune(1))` = SOH.
Low values produce non-printable control characters that can mask encoding bugs.
The `wantNilErr` field is declared but not consumed in assertions.

**Decision**:

1. Replace rune-based generation with `fmt.Sprintf("item-%d", i)` or equivalent
2. Remove unused `wantNilErr` or connect it to assertions
3. Make recommendation key assertions exact (not prefix-only)

**Rationale**: Clean, printable test data improves debuggability.
**Alternatives**: None.

---

## R-012: InitCache Test Isolation (FR-014)

**File**: `internal/cli/init_cache_test.go`

**Current State**: Already uses `t.Setenv(cache.EnvCacheDir, t.TempDir())` (line 97)
and `t.Setenv(cache.EnvTTLSeconds, "")` (line 92). The `wantNil` field IS used in
the if/else assertion block (lines 108-115).

**Decision**: No changes needed -- this file is already correctly isolated.
**Rationale**: Verified by reading the actual test code.
