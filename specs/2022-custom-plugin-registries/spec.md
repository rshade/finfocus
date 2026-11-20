---
title: "Custom plugin registries"
description: "Browse and install plugins from global registry sources."
layout: "docs"
---

**Status**: Implemented (issue #1766)

Implements [#1766](https://github.com/rshade/finfocus/issues/1766).

## User scenarios

- A user adds an HTTPS index to global config through the CLI or Plugins
  page. The user can list, remove, and refresh their sources.
- The Plugins page lists discoverable custom plugins with their source and
  community or experimental trust level. Built-in names retain precedence.
- Before the first install from a source, the user types its exact name.
  A checksum is required for every install or update from that source.
  Changing the source definition invalidates the prior confirmation.

## Requirements

- Validate indices against the pinned FinFocus registry schema. Fetch them
  with HTTPS, bounded responses and timeouts, same-origin redirects, and ETags.
- Treat source declarations and confirmations as global-only data.
- Reject ambiguous or unsafe installs, including built-in name collisions,
  absent checksums, and a catalog change between preview and confirmation.
- An optional SHA-256 pin applies to the downloaded index. Additional
  signature infrastructure remains out of scope.

## Web UI Fit

Extend the Plugins page with a Registries panel above the existing catalog.
Keep catalog details in the table, including the source and trust level.
Use the existing install confirmation and progress surfaces. The CLI and
web callbacks use the same registry service for source changes and install
authorization.
