# Contract: Semantic Exit Codes

## Exit Code Table

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | Command completed, costs within budget (or no budget configured) |
| 0 | Warning-only | Budget exceeded but exit_code configured as 0 |
| 1 | General failure | Command error, invalid flags, plugin failure, budget evaluation error |
| 2-255 | Budget violation | Budget threshold exceeded with configured `exit_code` value |

## Configuration

```yaml
budget:
  monthly_limit: 100.00
  currency: USD
  exit_on_threshold: true
  exit_code: 2  # Custom exit code for budget violations (0-255)
```

## Behavior

### Detection Mechanism

```text
main() receives error from run()
    → errors.As(err, *BudgetExitError)
    → if match: os.Exit(budgetErr.ExitCode)
    → else: os.Exit(1)
```

### Edge Cases

1. **exit_code = 0**: Treated as warning-only mode. Budget violation is logged to
   stderr but process exits with code 0 (success).
2. **Multiple scope violations**: When multiple budget scopes are violated with
   different exit codes, the highest exit code takes precedence (already handled
   by `BudgetStatus.GetExitCode()`).
3. **Budget evaluation error**: Always exits with code 1 regardless of configured
   exit_code (errors in evaluation logic, not threshold violations).
4. **No budget configured**: Exit code determined by command success/failure only.

### Affected Commands

| Command | Budget Exit Support |
|---------|-------------------|
| `cost projected` | Yes (already returns BudgetExitError) |
| `cost actual` | Yes (fix: must return BudgetExitError instead of logging) |
| `plugin list` | No (no budget evaluation) |
| `analyzer serve` | No (long-running server) |

## Key Invariants

1. Exit code 0 always means success or warning-only
2. Exit code 1 always means general failure
3. Exit codes 2-255 always mean budget violation with that specific code
4. The exit code is deterministic given the same budget config and cost results
5. Exit codes are propagated from both `cost projected` and `cost actual`
