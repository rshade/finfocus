# Research: Budget Threshold Exit Codes

**Feature Branch**: `124-budget-exit-codes`
**Date**: 2026-01-24

## Research Summary

All technical unknowns from the Technical Context have been resolved through codebase analysis.

## Decisions

### 1. Configuration Structure Location

**Decision**: Extend existing `BudgetConfig` struct in `internal/config/budget.go`

**Rationale**: The `BudgetConfig` struct already contains all budget-related settings (Amount, Currency, Period, Alerts). Adding `ExitOnThreshold` and `ExitCode` fields keeps all budget configuration cohesive.

**Alternatives Considered**:

- Separate `ExitConfig` struct: Rejected—fragments budget configuration unnecessarily
- Top-level config fields: Rejected—exit codes are budget-specific, not global

### 2. Environment Variable Naming

**Decision**: Use `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` and `FINFOCUS_BUDGET_EXIT_CODE`

**Rationale**: Follows established pattern (`FINFOCUS_*` prefix) and maintains consistency with existing env vars like `FINFOCUS_CACHE_ENABLED`, `FINFOCUS_LOG_LEVEL`.

**Alternatives Considered**:

- `FINFOCUS_EXIT_*`: Rejected—too generic, not budget-specific
- `FINFOCUS_COST_EXIT_*`: Rejected—budget is the specific subsystem, not cost

### 3. Exit Logic Location

**Decision**: Add `ShouldExit()` and `GetExitCode()` methods to `BudgetStatus` in `internal/engine/budget_cli.go`

**Rationale**: `BudgetStatus` already contains threshold evaluation results (`HasExceededAlerts()`). Adding exit determination here keeps the logic cohesive and testable.

**Alternatives Considered**:

- Separate exit evaluation function: Rejected—duplicates threshold checking logic
- CLI-only implementation: Rejected—logic should be testable in isolation

### 4. CLI Integration Point

**Decision**: Call exit logic after `renderBudgetIfConfigured()` in cost commands

**Rationale**: The existing `renderBudgetIfConfigured()` function in `cost_budget.go` evaluates the budget and renders status. Exit code evaluation naturally follows after output is complete.

**Alternatives Considered**:

- PersistentPostRunE hook: Rejected—budget context not available at that level
- Inside renderBudgetIfConfigured: Rejected—mixing concerns (render vs exit)

### 5. CLI Flag Registration

**Decision**: Add flags to parent `cost` command in `cost.go`, not individual subcommands

**Rationale**: Exit behavior applies to both `cost projected` and `cost actual`. Adding to parent command avoids duplication and ensures consistent flag handling.

**Alternatives Considered**:

- Per-subcommand flags: Rejected—duplication, inconsistent behavior risk
- Root command flags: Rejected—exit codes are cost-specific, not global

### 6. Exit Code Range Validation

**Decision**: Validate exit codes 0-255 in `BudgetConfig.Validate()`

**Rationale**: Unix exit codes are limited to 0-255. Validation at config level provides early feedback and prevents runtime surprises.

**Alternatives Considered**:

- No validation (let OS clamp): Rejected—silent behavior change is confusing
- Allow negative values: Rejected—no valid use case, increases error surface

### 7. Configuration Precedence

**Decision**: CLI flags > Environment Variables > Config File > Defaults

**Rationale**: Standard precedence pattern used throughout FinFocus (see `applyEnvironmentOverrides()`). Most specific (runtime) overrides least specific (defaults).

**Alternatives Considered**:

- Env vars override CLI flags: Rejected—counterintuitive, breaks standard patterns
- Config file overrides env vars: Rejected—breaks 12-factor app principles

### 8. Default Exit Code Value

**Decision**: Default exit code is 1 when `exit_on_threshold: true`

**Rationale**: Exit code 1 is the standard Unix convention for general errors/failures. Code 0 is reserved for success.

**Alternatives Considered**:

- Default to 0: Rejected—defeats purpose of exit_on_threshold
- Default to 2: Rejected—no standard meaning, would confuse users

### 9. Exit Code 0 Behavior

**Decision**: Allow explicit `exit_code: 0` to disable exit even with `exit_on_threshold: true`

**Rationale**: Provides an escape hatch for users who want threshold logging/alerts without pipeline failure. Edge case documented in spec.

**Alternatives Considered**:

- Reject exit_code 0: Rejected—limits user flexibility unnecessarily
- Ignore exit_on_threshold if code is 0: Rejected—implicit behavior is confusing

## Existing Code Patterns to Follow

### Environment Variable Override Pattern

From `internal/config/config.go`:

```go
func applyEnvironmentOverrides(cfg *Config) {
    if enabled := os.Getenv("FINFOCUS_CACHE_ENABLED"); enabled != "" {
        cfg.Cost.Cache.Enabled = enabled == "true" || enabled == "1"
    }
    if ttl := os.Getenv("FINFOCUS_CACHE_TTL_SECONDS"); ttl != "" {
        if v, err := strconv.Atoi(ttl); err == nil {
            cfg.Cost.Cache.TTLSeconds = v
        }
    }
}
```

### Budget Status Methods Pattern

From `internal/engine/budget_cli.go`:

```go
func (s *BudgetStatus) HasExceededAlerts() bool {
    for _, alert := range s.Alerts {
        if alert.Status == ThresholdStatusExceeded {
            return true
        }
    }
    return false
}
```

### Config Validation Pattern

From `internal/config/budget.go`:

```go
func (b BudgetConfig) Validate() error {
    if b.Amount < 0 {
        return ErrBudgetAmountNegative
    }
    // ... additional validations
    return nil
}
```

## Files to Modify

| File                              | Change                                                         |
|-----------------------------------|----------------------------------------------------------------|
| `internal/config/budget.go`       | Add `ExitOnThreshold`, `ExitCode` fields to `BudgetConfig`     |
| `internal/config/budget.go`       | Add validation for exit code range (0-255)                     |
| `internal/config/config.go`       | Add env var overrides in `applyEnvironmentOverrides()`         |
| `internal/engine/budget_cli.go`   | Add `ShouldExit()`, `GetExitCode()` methods to `BudgetStatus`  |
| `internal/cli/cost.go`            | Add `--exit-on-threshold`, `--exit-code` flags                 |
| `internal/cli/cost_projected.go`  | Call exit logic after render                                   |
| `internal/cli/cost_actual.go`     | Call exit logic after render                                   |

## New Files to Create

| File                                   | Purpose                                            |
|----------------------------------------|----------------------------------------------------|
| `internal/config/budget_test.go`       | Tests for new exit code config fields              |
| `internal/engine/budget_cli_test.go`   | Tests for ShouldExit/GetExitCode (extend existing) |
| `test/integration/budget_exit_test.go` | CI/CD simulation integration tests                 |

## Dependencies Confirmed

- **Issue #217 (MVP Budget Alerts)**: The `BudgetStatus.HasExceededAlerts()` method exists and works correctly. Exit code feature can build on this.
- **No cross-repo changes**: Feature is entirely within finfocus-core.
