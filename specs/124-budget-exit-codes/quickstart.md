# Quickstart: Budget Threshold Exit Codes

**Feature Branch**: `124-budget-exit-codes`
**Date**: 2026-01-24

## Overview

This feature enables CI/CD pipelines to detect budget threshold violations through exit codes, without parsing command output.

## Quick Setup

### 1. Configure Budget with Exit Behavior

Add to `~/.finfocus/config.yaml`:

```yaml
cost:
  budgets:
    amount: 100
    currency: USD
    alerts:
      - threshold: 80
        type: actual
      - threshold: 100
        type: actual
    exit_on_threshold: true
    exit_code: 2
```

### 2. Run Cost Command

```bash
finfocus cost projected --pulumi-json plan.json
echo "Exit code: $?"
```

### 3. Expected Behavior

| Scenario                    | Exit Code      |
|-----------------------------|----------------|
| Cost under all thresholds   | 0              |
| Cost exceeds 80% threshold  | 2 (configured) |
| Cost exceeds 100% threshold | 2 (configured) |
| No budget configured        | 0              |
| Error during evaluation     | 1              |

## Environment Variable Configuration

Override config file settings per-environment:

```bash
# Enable exit behavior for CI
export FINFOCUS_BUDGET_EXIT_ON_THRESHOLD=true
export FINFOCUS_BUDGET_EXIT_CODE=2

finfocus cost projected --pulumi-json plan.json
```

## CLI Flag Override

Override at runtime for one-off checks:

```bash
# Force exit behavior even if not in config
finfocus cost projected --pulumi-json plan.json --exit-on-threshold --exit-code 3
```

## CI/CD Integration Example

### GitHub Actions

```yaml
jobs:
  cost-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Pulumi Preview
        run: pulumi preview --json > plan.json

      - name: Check Cost Budget
        run: |
          finfocus cost projected --pulumi-json plan.json --exit-on-threshold
        continue-on-error: false  # Fail pipeline if budget exceeded
```

### GitLab CI

```yaml
cost-check:
  script:
    - pulumi preview --json > plan.json
    - finfocus cost projected --pulumi-json plan.json --exit-on-threshold
  allow_failure: false
```

### Jenkins

```groovy
pipeline {
    stages {
        stage('Cost Check') {
            steps {
                sh 'pulumi preview --json > plan.json'
                sh 'finfocus cost projected --pulumi-json plan.json --exit-on-threshold'
            }
        }
    }
}
```

## Debugging

Enable debug output to see exit code reasoning:

```bash
finfocus cost projected --pulumi-json plan.json --exit-on-threshold --debug
```

Sample debug output:

```text
DBG exit code evaluation component=cli reason="threshold 80% exceeded" exit_code=2
```

## Common Patterns

### Warning-Only (No Pipeline Failure)

Set `exit_code: 0` to log warnings without failing:

```yaml
cost:
  budgets:
    amount: 100
    currency: USD
    exit_on_threshold: true
    exit_code: 0  # Logs threshold exceeded but exits 0
```

### Different Codes for Different Thresholds

Currently, all thresholds use the same exit code. For differentiated behavior, use multiple runs or external scripting:

```bash
# Check 100% threshold
if finfocus cost projected --pulumi-json plan.json --exit-on-threshold; then
  echo "Budget OK"
else
  echo "Budget EXCEEDED"
  exit 2
fi
```

## Validation

Test your configuration:

```bash
# Verify config is valid
finfocus config show

# Test with a known high-cost plan
finfocus cost projected --pulumi-json high-cost-plan.json --debug
echo "Exit code: $?"
```
