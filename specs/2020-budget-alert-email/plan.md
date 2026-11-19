---
title: "SMTP budget alert implementation plan"
description: "Extend the existing budget alert dispatcher with SMTP delivery."
layout: "docs"
---

## Design

Use Go's `net/smtp` over a context-bounded connection. Apply the shared
deadline to the socket before the greeting and close it on cancellation.
Establish and verify TLS before using PLAIN auth. For plaintext, require a
loopback host and verify the connected peer before exchanging SMTP data.

Use `internal/config` for the SMTP settings, validation, project reference
checks, and display masking. The dispatcher expands email values with
`notification.Expand`, validates the resolved destination, and registers the
SMTP sender in the existing sender map. Report SMTP reply codes and stage
names through the shared redactor, discarding relay-provided text.

Use the existing budget event to render the subject and body. Format addresses
with `net/mail` and include one message ID per delivery. Send the message only
after the relay accepts every To and Cc envelope recipient.

## Verification

Exercise all TLS modes with local SMTP test relays, including a trusted test
certificate and certificate rejection. Cover credential masking, variable
scope restrictions, injection, SMTP failures, timeouts, and dry runs. Run a
projected-cost integration test with and without notification opt-in. Verify
the existing web budget payload continues to omit destinations.

## Constitution check

The feature uses only standard-library dependencies and the existing alert
event and config model. Tests use local relays and run before implementation.
No new protocol or local state is required. The spec records the Web UI Fit
exemption for outbound delivery.
