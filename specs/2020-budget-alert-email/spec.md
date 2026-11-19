---
title: "SMTP budget alert notifications"
description: "Deliver opted-in budget threshold alerts to email recipients."
layout: "docs"
---

**Status**: Implemented

Implemented on branch `2020-budget-alert-email` for
[#1702](https://github.com/rshade/finfocus/issues/1702).

## User scenarios and testing

A budget owner configures email on any alert scope and opts in on a cost run.
Each exceeded threshold delivers one plain-text message per email destination,
with all configured To and Cc recipients in the same transaction. The message
contains the budget, scope, spend, percentage, threshold, status, and FinFocus
version. Forecast alerts identify the forecast and use forecast values.

A budget owner can preview which recipients would be notified with
`--dry-run`. Runs without opt-in print the existing hint. Missing allowed
variables and failed delivery produce one destination warning on stderr.
Command data and exit status are independent of notification delivery.

## Requirements

- **FR-001**: Accept `email` on global, provider, tag, and type budget alerts.
  Validate required SMTP host, sender and To addresses, optional Cc, port,
  and TLS mode in both configuration validation and cost preflight.
- **FR-002**: Require verified STARTTLS by default on port 587. Support verified
  implicit TLS on port 465 and plaintext loopback relays on port 25. Allow
  explicit ports and optional PLAIN authentication.
- **FR-003**: Reuse the existing dispatcher, opt-in, dry run, and whole-delivery
  deadline. On failure, report a safe reason without changing stdout or exit
  status. Deliver to all recipients in one SMTP transaction.
- **FR-004**: Reuse the shared variable allowlist and project-config restriction.
  Reject invalid addresses or CR/LF in headers after expansion. Mask SMTP
  credentials in displayed config and delivery reporting.
- **FR-005**: Send an RFC 5322 plain-text message with From, To, optional Cc,
  Subject, Date, and Message-ID. Default to the FinFocus budget alert subject
  and allow a custom subject with a `{{threshold}}` token.

## Web UI Fit

Not applicable. This adds an outbound transport for the existing opt-in CLI
side effect. The budget dashboard shows threshold evaluation and omits
notification destinations, and opening it has no notification opt-in.

## Success criteria

- One message reaches every configured recipient on an opted-in crossing.
- Dry runs and runs without opt-in deliver zero messages.
- A stalled relay is bounded by the destination deadline, 10 seconds by
  default. Failures leave cost output and exit status intact.
- Configuration and delivery output contain no SMTP credential values.

## Scope

SMTP relay delivery only. HTML, attachments, retries, provider-specific
mail APIs, and configurable TLS trust are outside this feature.
