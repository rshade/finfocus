---
title: Plugin Certification
description: Plugin certification process and quality standards for FinFocus.
---
Certification ensures that a FinFocus plugin is fully compatible with the
core engine and meets quality standards.

## Certification Process

1. **Conformance Testing**: The plugin must pass 100% of the conformance test suite.
2. **Performance**: The plugin must respond within 10 seconds for standard requests.
3. **Stability**: The plugin must handle errors gracefully and not crash.

## Running Certification

Use the `finfocus plugin certify` command:

```bash
finfocus plugin certify ./path/to/plugin
```

## Report

The command generates a `certification.md` report that can be distributed
with your plugin.
