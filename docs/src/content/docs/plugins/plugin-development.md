---
title: Plugin Development Guide
description: How to develop plugins for FinFocus Core
layout: default
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

## Best Practices

1. **Error Handling**: Return appropriate gRPC error codes
   (NotFound, InvalidArgument, etc.).
2. **Timeouts**: Respect context cancellations and timeouts.
3. **Performance**: Optimize for batch processing where possible.
