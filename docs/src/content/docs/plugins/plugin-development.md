---
title: Plugin Development Guide
description: How to develop plugins for FinFocus Core
---

This guide explains how to develop plugins for FinFocus Core.

## Protocol Overview

Plugins communicate with the Core engine via gRPC. The protocol is defined
in the [finfocus-spec](https://github.com/rshade/finfocus-spec) repository.

## Conformance Testing

To ensure your plugin is compliant with the protocol, use the conformance
testing tool:

```bash
finfocus plugin conformance ./path/to/your-plugin
```

The suite probes with an AWS EC2 instance. If your plugin lists providers
through `GetPluginInfo` and none is `aws`, the probes that need a supported
resource type are skipped and the report says why. Probe with a resource your
plugin prices instead:

```bash
finfocus plugin conformance \
  --provider kubernetes \
  --resource-type kubernetes:apps/v1:Deployment \
  ./path/to/your-plugin
```

`GetProjectedCost_ValidResource` and `Batch_Handling` price the probe
resource, so your plugin must support its type. The error-injection
probes (`PermissionDenied`, `InternalError`, `Unavailable`) send sentinel
resource types that only a plugin built to recognize them answers with the
matching status, so they run with the built-in probe only and are skipped
when you set `--provider`. `plugin certify` counts a skipped probe as not
certified.

### Plugin Name Checks

Every plugin reports its install name, the `<name>` in
`~/.finfocus/plugins/<name>/<version>/` and in its binary filename
`finfocus-plugin-<name>`. Return that name from `Name()` and from
`GetPluginInfo`, and use it for every result `Source`. Declare it once as a
constant (`const PluginName = "<name>"`). `finfocus plugin init` generates
one.

The suite checks the name three ways:

| Check | Warns when |
| --- | --- |
| `Name_MatchesInstallName` | `Name()` differs from the install name |
| `Name_MatchesPluginInfo` | `GetPluginInfo` reports another name, or isn't implemented |
| `Name_IsCanonicalForm` | The name has uppercase letters, doubled hyphens, or the `finfocus-plugin-` prefix |

These checks only warn. A warning appears in the table, JSON, and JUnit
reports and in the summary count, but it never fails the run, changes the
exit code, or blocks `plugin certify`.

The install name comes from the plugin directory that holds the binary, or
else from its filename. For a binary built outside that layout, pass the name:

```bash
finfocus plugin conformance --expected-name acme-cost ./bin/finfocus-plugin-acme-cost
```

FinFocus still routes to a plugin whose names differ, and logs a warning. To
rename a published plugin, first add its old name to the plugin's entry in
the FinFocus plugin catalog (`aliases`), so routing rules and `--adapter`
values written with the old name keep working.

## Best Practices

1. **Error Handling**: Return appropriate gRPC error codes
   (NotFound, InvalidArgument, etc.).
2. **Timeouts**: Respect context cancellations and timeouts.
3. **Performance**: Optimize for batch processing where possible.
