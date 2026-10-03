---
name: finfocus-diagnose
description: >
  Diagnose and debug FinFocus connectivity, config resolution, cache issues,
  plugin errors, and zero cost results. Use when a cost command returns $0,
  a plugin will not start, config.hujson is ignored, or output shows
  PLUGIN_ERROR, VALIDATION_ERROR, TIMEOUT_ERROR, or NO_COST_DATA. Triggers on:
  "diagnose", "debug", "connectivity", "zero cost", "cache issue",
  "plugin error", "config resolution", "PLUGIN_ERROR", "VALIDATION_ERROR",
  "TIMEOUT_ERROR", "NO_COST_DATA".
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Diagnose FinFocus

This skill is FinFocus-specific. Read the reference that matches the symptom
before changing config or deleting cache files.

Install: `npx skills add rshade/finfocus -s finfocus-diagnose`.

## 1. Name the symptom

| What you see | Open |
| --- | --- |
| `connection refused`, `broken pipe`, `EOF`, `no such file or directory`, bind timeout | [Plugin connectivity](references/decision-trees.md) |
| The wrong plugins load, or an edit has no effect | [Config resolution](references/diagnostic-commands.md) |
| Stale prices, (cached) on the adapter, or a locked `cache.db` | [Cache](references/decision-trees.md) |
| Every resource is `$0`, or a note starts with `VALIDATION:` or `ERROR:` | [Zero cost](references/decision-trees.md) |
| `PLUGIN_ERROR`, `VALIDATION_ERROR`, `TIMEOUT_ERROR`, `NO_COST_DATA` | [Error codes](references/error-codes.md) |

Commands, flags, and environment variables:
[references/diagnostic-commands.md](references/diagnostic-commands.md).

## 2. Collect evidence before changing anything

```bash
finfocus plugin list --verbose
finfocus plugin validate
finfocus config routes list
finfocus config list
finfocus cost projected --debug --pulumi-json plan.json
```

`finfocus config routes test <resource-type> [region]` simulates selection.
It does not launch plugin binaries.

Log level precedence is `--debug`, then `FINFOCUS_LOG_LEVEL`, then the config file, then `info`.
The default format value is `text`. `FINFOCUS_LOG_FORMAT=json` switches to JSON.
`FINFOCUS_TRACE_ID` overrides the generated trace id. gRPC metadata uses `x-finfocus-trace-id`.

## 3. Apply the matching fix

- Connectivity: confirm the binary, the executable bit, and `plugin.manifest.json`, then retry. Core sets `--port` and `FINFOCUS_PLUGIN_PORT`. It does not set `PORT`.
- Config: edit `config.hujson`. A project file replaces a whole top-level key of the global file.
- Cache: `FINFOCUS_CACHE_ENABLED=false` skips the cache. Deleting `cache.db` drops stored prices. Corruption is recreated on open.
- Zero cost: read the note and the structured code before treating `$0` as a failure. A returned monthly cost of 0 does not fall back to the next plugin.

Do not invent a timeout flag. The constants and the path that uses them are in the decision tree.
