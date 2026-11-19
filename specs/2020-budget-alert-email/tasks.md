---
title: "SMTP budget alert tasks"
description: "Implementation and verification tasks for issue 1702."
layout: "docs"
---

## Configuration

- [x] T001 Add failing validation, project-source and masking tests in
  `internal/config/notification_test.go` (FR-001, FR-004).
- [x] T002 Implement SMTP settings, validation and masking in `internal/config`
  (FR-001, FR-002, FR-004).

## Delivery

- [x] T003 Add local relay tests for message content, TLS modes, auth, SMTP
  failures, cancellation and deadlines in `internal/notification` (FR-002–005).
- [x] T004 Implement the SMTP sender and email resolution in
  `internal/notification` (FR-002–005).
- [x] T005 Verify opt-in, dry-run reporting, warnings, and JSON output in
  `test/integration/budget_notifications_test.go` and wire email preview
  details in `internal/cli/budget_notify.go` (FR-003, FR-004).

## Documentation and verification

- [x] T006 Update the budget guide with STARTTLS and implicit TLS examples.
- [x] T007 Run the focused tests, coverage, repository checks, and review.
