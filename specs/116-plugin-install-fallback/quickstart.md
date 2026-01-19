# Quickstart: Plugin Install Version Fallback

**Date**: 2026-01-18
**Feature**: 116-plugin-install-fallback

## Overview

This feature adds graceful fallback behavior when installing plugins where the requested version exists but lacks platform-compatible assets.

## Usage Scenarios

### Scenario 1: Interactive Mode (Default)

When running in a terminal, you'll be prompted if the requested version lacks assets:

```bash
$ finfocus plugin install aws-public@v0.1.3

Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64).
? Would you like to install the latest stable version (v0.1.2) instead? [y/N] y

Installing aws-public@v0.1.2 (fallback from v0.1.3)...
✓ Plugin installed successfully
  Name:    aws-public
  Version: v0.1.2 (requested: v0.1.3)
  Path:    ~/.finfocus/plugins/aws-public/v0.1.2
```

**Note**: Pressing Enter without typing Y or n defaults to "No" (abort).

### Scenario 2: CI/CD Mode (Automated Fallback)

In CI pipelines, use `--fallback-to-latest` for automatic fallback:

```bash
$ finfocus plugin install aws-public@v0.1.3 --fallback-to-latest

Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64).
Installing aws-public@v0.1.2 (fallback from v0.1.3)...
✓ Plugin installed successfully
  Name:    aws-public
  Version: v0.1.2 (requested: v0.1.3)
```

### Scenario 3: Strict Version Pinning

For reproducible builds requiring exact versions, use `--no-fallback`:

```bash
$ finfocus plugin install aws-public@v0.1.3 --no-fallback

Error: no asset found for linux/amd64. Available: []
```

## Quick Reference

Use Case | Command
---------|--------
Interactive install | `finfocus plugin install plugin@version`
CI with fallback | `finfocus plugin install plugin@version --fallback-to-latest`
Strict version | `finfocus plugin install plugin@version --no-fallback`

## Behavior Summary

Environment | Default Behavior | With `--fallback-to-latest` | With `--no-fallback`
------------|------------------|----------------------------|---------------------
Terminal (TTY) | Prompt user | Auto-fallback | Fail
Non-TTY (CI) | Fail | Auto-fallback | Fail

## Error Messages

- **Prompt declined**: "Installation aborted."
- **No stable versions**: "no stable releases found"
- **All versions lack assets**: "no compatible asset found for version X or any of 10 fallback releases"
