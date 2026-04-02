---
title: Error Codes
description: Reference for FinFocus error codes
layout: default
---

This page lists common error codes and messages returned by FinFocus.

## CLI Errors

### ERR-001: Config File Error

**Message**: "Failed to load configuration file"
**Cause**: The configuration file at `~/.finfocus/config.yaml` is invalid
or unreadable.
**Fix**: Check permissions and YAML syntax.

### ERR-002: Plugin Not Found

**Message**: "Plugin [name] not found"
**Cause**: The requested plugin is not installed in `~/.finfocus/plugins`.
**Fix**: Run `finfocus plugin install [name]`.

## Engine Errors

### ENG-001: Pricing Lookup Failed

**Message**: "No pricing data found for resource"
**Cause**: The resource type or SKU is not supported by the pricing provider.
**Fix**: Check if the resource is supported or add a local override in
`~/.finfocus/specs/`.

### ENG-002: Plugin Timeout

**Message**: "Plugin request timed out"
**Cause**: The plugin took too long to respond (default 10s).
**Fix**: Check network connectivity or increase timeout via environment
variable.
