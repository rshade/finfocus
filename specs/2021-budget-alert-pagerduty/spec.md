---
title: "PagerDuty budget alert notifications"
description: "Send opted-in budget threshold alerts to PagerDuty."
layout: "docs"
---

**Status**: Implemented

Issue [#1703](https://github.com/rshade/finfocus/issues/1703) extends the
budget notifications introduced in spec 625.

## User scenarios

An on-call engineer configures a PagerDuty integration key and severity on a
budget alert. An opted-in cost run sends one trigger per exceeded threshold
and destination. The alert shows the same budget, spend, and threshold as the
webhook event, with forecasts explicitly identified.

Repeated crossings of the same monthly threshold use the same deduplication
key while an incident is open. Per-run mode lets PagerDuty issue a new key.
Dry runs show a trigger preview on stderr. Delivery failures produce one
warning and preserve cost output and exit status.

## Requirements

- **FR-001**: Accept the type in all budget scopes. Validate routing key,
  severity, deduplication mode, and optional HTTPS endpoint.
- **FR-002**: Send an Events API v2 trigger with severity, project when known,
  budget facts, and a bounded summary. Treat only HTTP 202 as success.
- **FR-003**: Compute a stable bounded key from project, scope, threshold,
  alert type, and the evaluated month. Omit the key in per-run mode.
- **FR-004**: Reuse notification opt-in, dry runs, deadline, concurrent
  delivery, and the shared secret-reference restrictions. Mask the routing
  key throughout reporting and config display. Report sanitized response
  diagnostics without retrying.
- **FR-005**: Deliver triggers only. PagerDuty or its operators manage
  incident closure.

## Web UI Fit

Not applicable. PagerDuty is an outbound transport used by the existing
opt-in CLI notification flow. Budget evaluation remains available in the web
dashboard, which omits notification destinations and credentials.

## Success criteria

Local TLS and CLI tests verify the trigger, status handling, stable keys,
credential handling, deadline, and unchanged JSON output and exit status.
