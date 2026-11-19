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

## Best Practices

1. **Error Handling**: Return appropriate gRPC error codes
   (NotFound, InvalidArgument, etc.).
2. **Timeouts**: Respect context cancellations and timeouts.
3. **Performance**: Optimize for batch processing where possible.
