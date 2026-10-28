---
title: "Pulumi Cloud fixtures"
description: "Recorded update-list provenance and synthetic export fixture"
layout: "docs"
---

## Update lists

Captured on 2026-10-27 with `pulumi api` against a real test stack using
`GET /api/stacks/{org}/{project}/{stack}/updates`, `pageSize=100`, pages 1
and 2. `updates.json` contains 100 updates; `updates-page2.json` contains
31. Both pages and their concatenation are newest first (descending version).
The service returns flat update fields. Messages, environment keys and values,
and config keys and values were scrubbed. Times, versions, kinds, results,
and resource change counts retain the recorded response shape and values.

## Export

`export.json` is synthetic. It deliberately contains a fake secret envelope
and resource categories that the identity decoder must discard.
