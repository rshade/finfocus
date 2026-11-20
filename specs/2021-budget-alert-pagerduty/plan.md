---
title: "PagerDuty budget alert implementation plan"
description: "A PagerDuty Events API v2 sender in the existing dispatcher."
layout: "docs"
---

## Design

Extend the existing config, shared variable resolver, and destination
registry. Use the existing HTTPS client and request builder. Allow the
default US and explicit EU enqueue endpoints plus HTTPS loopback overrides
for tests. Validate the final endpoint after expansion.

Reuse the webhook event fields for PagerDuty custom details. Resolve Pulumi
project identity from the command's project configuration discovery. Carry
the budget evaluation time alongside the evaluated result so deduplication
uses the same local month as forecasting, while timestamps stay in UTC.
Escape field separators in the key, and hash keys above the limit.

Decode bounded JSON diagnostics for failure responses, using only the
message and errors fields. Redact the credential and remove control
characters before printing one warning.

## Verification

Use local TLS tests for request, response, cancellation, and redaction.
Exercise projected-cost opt-in and repeated-run deduplication, config display,
and existing web budget parity. Run the repository test and lint gates.
