---
title: "Historical Carbon Web UI Follow-up"
description: "Local UX-only follow-up for the authenticated historical carbon report hook"
layout: "docs"
---

## Scope

Add a browser report form for the existing authenticated
`POST /api/carbon/actual/query` domain hook. This is a local `web-ui` follow-up;
no external issue or message has been posted.

## Acceptance criteria

- Offer complete-month start and exclusive-end dates, accounting method, scope,
  and optional account, region, and service arrays.
- Render source, provider, grouping dimensions, original periods, model version,
  retrieval time, and `gCO2e` values from returned records.
- Label the total as the sum of returned available records. Show missing data
  distinctly from explicit zero; preserve selected query context on empty reports.
- Display actionable opt-in guidance for disabled providers and safe errors for
  unavailable AWS data. Avoid presenting partial data after query failure.
- Use the existing session authentication, origin checks, cancellation, and
  shutdown lifecycle. Reuse the server report response without client arithmetic.
- Verify keyboard access, narrow viewport behavior, loading and empty states,
  and parity with `finfocus carbon actual --output json`.
