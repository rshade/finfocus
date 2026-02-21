# Audit: Function-Level Comparison for MERGE/VERIFY Files

**Date**: 2026-02-20
**Phase**: Phase 2 — Foundational Audit (T003–T009)

All functions in each `test/unit/` MERGE file compared to their `internal/` counterparts.
Legend: **UNIQUE** = function not present in internal/ file; **DUPLICATE** = same function name exists.

---

## T003: `cli/cost_actual_test.go` vs `internal/cli/cost_actual_test.go`

**Classification: MERGE — all functions UNIQUE**

| Function | Status |
|---|---|
| `TestCostActualCmd_Success` | UNIQUE |
| `TestCostActualCmd_MissingStartDate` | UNIQUE (stale assertion — fix inline) |
| `TestCostActualCmd_DefaultEndDate` | UNIQUE |
| `TestCostActualCmd_InvalidDateFormat` | UNIQUE |
| `TestCostActualCmd_RFC3339DateFormat` | UNIQUE |
| `TestCostActualCmd_GroupByResource` | UNIQUE |
| `TestCostActualCmd_GroupByType` | UNIQUE |
| `TestCostActualCmd_GroupByProvider` | UNIQUE |
| `TestCostActualCmd_GroupByDaily` | UNIQUE |
| `TestCostActualCmd_TableOutput` | UNIQUE |
| `TestCostActualCmd_NDJSONOutput` | UNIQUE |
| `TestCostActualCmd_AdapterFilter` | UNIQUE |

**Action**: Append all 12 functions to `internal/cli/cost_actual_test.go`; `git rm` source.
**Stale assertion**: `TestCostActualCmd_MissingStartDate` checks `"required flag"` but actual error is `"--from is required when using --pulumi-json"` — update expected string.

---

## T004: `cli/cost_projected_test.go` vs `internal/cli/cost_projected_test.go`

**Classification: MERGE — all functions UNIQUE**

| Function | Status |
|---|---|
| `TestCostProjectedCmd_Success` | UNIQUE (stale assertion — fix inline) |
| `TestCostProjectedCmd_MissingPlanFile` | UNIQUE |
| `TestCostProjectedCmd_InvalidJSON` | UNIQUE |
| `TestCostProjectedCmd_MultipleResources` | UNIQUE |
| `TestCostProjectedCmd_TableOutput` | UNIQUE |
| `TestCostProjectedCmd_NDJSONOutput` | UNIQUE |
| `TestCostProjectedCmd_FilterByType` | UNIQUE |
| `TestCostProjectedCmd_FilterByProvider` | UNIQUE |
| `TestCostProjectedCmd_EmptyPlan` | UNIQUE |
| `TestCostProjectedCmd_MissingRequiredFlag` | UNIQUE (stale assertion — fix inline) |
| `TestCostProjectedCmd_InvalidOutputFormat` | UNIQUE |
| `TestCostProjectedCmd_ComplexResourceProperties` | UNIQUE |

**Action**: Append all 12 functions to `internal/cli/cost_projected_test.go`; `git rm` source.
**Stale assertions**:
- `TestCostProjectedCmd_Success`: expects `"USD"` in Currency field, actual is `""` — investigate and fix.
- `TestCostProjectedCmd_MissingRequiredFlag`: expects `"required flag"` but actual error is `"reading --stack flag: flag accessed but not defined: stack"` — update expected string.

---

## T005: `cli/plugin_test.go` vs `internal/cli/plugin_*_test.go` files

**Classification: MERGE — all functions UNIQUE**

Internal `plugin_*_test.go` files checked: `plugin_list_test.go`, `plugin_validate_test.go`,
`plugin_install_test.go`, `plugin_remove_test.go`, `plugin_update_test.go`,
`plugin_certify_test.go`, `plugin_conformance_test.go`, `plugin_init_test.go`,
`plugin_init_fixtures_test.go`, `plugin_inspect_test.go`, `plugin_list_internal_test.go`,
`plugin_install_internal_test.go`.

| Function | Status | Best Destination |
|---|---|---|
| `TestPluginListCmd_NoPlugins` | UNIQUE | `internal/cli/plugin_list_test.go` |
| `TestPluginListCmd_WithPlugins` | UNIQUE | `internal/cli/plugin_list_test.go` |
| `TestPluginValidateCmd_NoPlugins` | UNIQUE | `internal/cli/plugin_validate_test.go` |
| `TestPluginValidateCmd_ValidPlugin` | UNIQUE | `internal/cli/plugin_validate_test.go` |
| `TestPluginValidateCmd_NonExecutable` | UNIQUE | `internal/cli/plugin_validate_test.go` |
| `TestPluginListCmd_VerboseOutput` | UNIQUE (stale assertion) | `internal/cli/plugin_list_test.go` |

**Action**: Split functions into two files:
- `TestPluginListCmd_*` → append to `internal/cli/plugin_list_test.go`
- `TestPluginValidateCmd_*` → append to `internal/cli/plugin_validate_test.go`
Then `git rm test/unit/cli/plugin_test.go`.
**Stale assertion**: `TestPluginListCmd_VerboseOutput` expects `"Executable"` but output format changed — update to match current output.

---

## T006: `config/config_test.go` vs `internal/config/config_test.go`

**Classification: MERGE — all functions UNIQUE**

| Function | Status |
|---|---|
| `TestGetConfigDir` | UNIQUE |
| `TestGetPluginDir` | UNIQUE |
| `TestGetSpecDir` | UNIQUE |
| `TestEnsureConfigDir` | UNIQUE |
| `TestEnsureSubDirs` | UNIQUE |
| `TestConfigPaths` | UNIQUE |
| `TestConfigPermissions` | UNIQUE |

**Action**: Append all 7 functions to `internal/config/config_test.go`; `git rm` source.

---

## T007: `engine/engine_test.go` vs `internal/engine/engine_test.go`

**Classification: MERGE — all functions UNIQUE**

| Function | Status |
|---|---|
| `TestGetProjectedCost_WithPlugin` | UNIQUE |
| `TestGetProjectedCost_MultipleResources` | UNIQUE |
| `TestGetProjectedCost_NoPlugin` | UNIQUE |
| `TestGetProjectedCost_PluginError` | UNIQUE |
| `TestGetProjectedCost_MultiPluginSupport` | UNIQUE |
| `TestGetProjectedCost_PartialData` | UNIQUE |
| `TestGetProjectedCost_HighCost` | UNIQUE |
| `TestGetProjectedCost_ZeroCost` | UNIQUE |
| `TestGetProjectedCost_MultiCurrency` | UNIQUE |
| `TestGetProjectedCost_WithBreakdown` | UNIQUE |
| `TestGetActualCost_WithPlugin` | UNIQUE |
| `TestGetActualCost_NoPlugin` | UNIQUE |
| `TestGetActualCost_TimeRange` | UNIQUE |

**Action**: Append all 13 functions to `internal/engine/engine_test.go`; `git rm` source.

---

## T008: `pluginhost/client_test.go` vs `internal/pluginhost/client_test.go`

**Classification: MERGE — all functions UNIQUE**

| Function | Status |
|---|---|
| `TestNewClient_Success` | UNIQUE |
| `TestNewClient_LauncherError` | UNIQUE |
| `TestNewClient_NameRPCError` | UNIQUE |
| `TestNewClient_NameRPCErrorWithCloseFail` | UNIQUE |
| `TestClient_Fields` | UNIQUE |
| `TestClient_APIUsage` | UNIQUE |
| `TestClient_Close` | UNIQUE |
| `TestClient_CloseError` | UNIQUE |
| `TestClient_MultipleCloses` | UNIQUE |
| `TestClient_ContextCancellation` | UNIQUE |

**Action**: Append all 10 functions to `internal/pluginhost/client_test.go`; `git rm` source.

---

## T009: `ingest/plan_test.go` vs `internal/ingest/pulumi_plan_test.go`

**Classification: MERGE** (different filenames but overlapping test subjects; all functions UNIQUE)

`test/unit/ingest/plan_test.go` tested as VERIFY; no function name appears in both files.

| Function (test/unit) | Status |
|---|---|
| `TestLoadPulumiPlan_ValidPlan` | UNIQUE |
| `TestLoadPulumiPlan_MultipleSteps` | UNIQUE |
| `TestLoadPulumiPlan_EmptyPlan` | UNIQUE |
| `TestLoadPulumiPlan_NonExistentFile` | UNIQUE |
| `TestLoadPulumiPlan_InvalidJSON` | UNIQUE |
| `TestLoadPulumiPlan_ComplexInputs` | UNIQUE |
| `TestGetResources_FiltersByOperation` | UNIQUE |
| `TestGetResources_ExtractsProvider` | UNIQUE |
| `TestGetResources_PreservesInputs` | UNIQUE |
| `TestGetResources_EmptyPlan` | UNIQUE |
| `TestGetResources_PreservesOrder` | UNIQUE |

**Action**: Since destination has `pulumi_plan_test.go` (different name), append all unique functions
to `internal/ingest/pulumi_plan_test.go`; `git rm test/unit/ingest/plan_test.go`.
(No clean `git mv` because `plan_test.go` would conflict with the intent of `pulumi_plan_test.go`.)

---

## Summary

| Task | File | Classification | Functions | Stale Assertions |
|---|---|---|---|---|
| T003 | cli/cost_actual_test.go | MERGE | 12 UNIQUE | 1 (MissingStartDate) |
| T004 | cli/cost_projected_test.go | MERGE | 12 UNIQUE | 2 (Success, MissingRequiredFlag) |
| T005 | cli/plugin_test.go | MERGE | 6 UNIQUE | 1 (VerboseOutput) |
| T006 | config/config_test.go | MERGE | 7 UNIQUE | 0 |
| T007 | engine/engine_test.go | MERGE | 13 UNIQUE | 0 |
| T008 | pluginhost/client_test.go | MERGE | 10 UNIQUE | 0 |
| T009 | ingest/plan_test.go | MERGE | 11 UNIQUE | 0 |

**Total**: 71 functions to merge; 4 stale assertions to fix inline during Phase 3 Batch-B.
