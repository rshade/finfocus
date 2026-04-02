---
title: Plugin Troubleshooting
description: Troubleshooting guide for common plugin issues
layout: default
---

## Common Issues

### 1. Conformance Test Failures

**Issue**: Tests fail with "protocol version mismatch".
**Fix**: Ensure your plugin implements the correct version of the gRPC
protocol.

### 2. Timeouts

**Issue**: Tests fail with "context deadline exceeded".
**Fix**: Your plugin is taking too long to respond. Optimize your code or
check network latency if calling external APIs.

### 3. Certification Failures

**Issue**: Certification fails.
**Fix**: Ensure ALL conformance tests pass. Certification requires
100% pass rate.
