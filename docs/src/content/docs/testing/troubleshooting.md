---
title: Testing Troubleshooting
description: Troubleshooting guide for common testing issues
layout: default
---

## Common Issues

### 1. Integration Tests Failing with "connection refused"

**Cause**: Mock plugin server not starting or port conflict.
**Fix**: Ensure no other process is using the test ports. Tests use dynamic ports where possible.

### 2. E2E Tests Failing on Binary Lookup

**Cause**: `finfocus` binary not found.
**Fix**: Run `make build` before running E2E tests.

### 3. Benchmark Variability

**Cause**: System load.
**Fix**: Run benchmarks on a quiet system. Use `benchstat` to compare results.

### 4. JSON Parsing Errors in Tests

**Cause**: Log output mixing with stdout.
**Fix**: Use `CLIHelper` which disables logging during test execution.
