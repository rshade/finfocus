# Quickstart: Analyzer Quality Cluster

**Branch**: `603-analyzer-quality` | **Date**: 2026-02-24

## Overview

This cluster addresses 5 analyzer issues that improve the accuracy, usability,
and diagnosability of the FinFocus Pulumi analyzer integration.

## Issue Breakdown

| Issue | Priority | Summary | Dependencies |
|-------|----------|---------|--------------|
| #746 | P1 | Fix $0.00 stack summary bug | None |
| #755 | P2 | Policy pack directory setup | None |
| #754 | P3 | `--force` syncs policy pack binary | #755 |
| #756 | P4 | Post-install PATH instructions | #755 |
| #757 | P5 | `analyzer check` command | #755 |

## Implementation Order

1. **#746** (standalone bug fix — no dependencies)
2. **#755** (foundational — #754, #756, #757 depend on it)
3. **#754** (extends `--force` to sync policy pack)
4. **#756** (adds post-install output)
5. **#757** (new command — depends on policy pack structure from #755)

## File Impact Summary

### Modified Files

| File | Issues | Changes |
|------|--------|---------|
| `internal/analyzer/diagnostics.go` | #746 | Fix `StackSummaryDiagnostic` counting |
| `internal/analyzer/server.go` | #746 | Cache error costs in `Analyze()` |
| `internal/analyzer/install.go` | #755, #754 | Add policy pack setup + force sync |
| `internal/cli/analyzer_install.go` | #755, #756 | Policy pack output + PATH instructions |
| `internal/cli/analyzer.go` | #757 | Wire `check` subcommand |

### New Files

| File | Issues | Purpose |
|------|--------|---------|
| `internal/analyzer/policypack.go` | #755 | Policy pack directory setup logic |
| `internal/analyzer/check.go` | #757 | Check command logic |
| `internal/cli/analyzer_check.go` | #757 | CLI wiring for `analyzer check` |
| `internal/analyzer/policypack_test.go` | #755 | Policy pack tests |
| `internal/analyzer/check_test.go` | #757 | Check command tests |
| `internal/cli/analyzer_check_test.go` | #757 | CLI integration tests |

## Quick Verification

After implementation, verify with:

```bash
# Build
make build

# Run tests
go test ./internal/analyzer/...
go test ./internal/cli/... -run TestAnalyzer

# Full suite
make test
make lint

# Manual verification
./bin/finfocus analyzer install
./bin/finfocus analyzer check
./bin/finfocus analyzer check --output json
```
