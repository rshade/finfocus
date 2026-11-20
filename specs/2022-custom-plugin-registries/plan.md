---
title: "Custom plugin registry implementation plan"
description: "Share registry source management across CLI and web."
layout: "docs"
---

## Components

- Configuration reads and writes global-only sources and trust confirmations.
- A registry client fetches bounded validated indices and caches ETags.
- The shared registry service resolves source and plugin identity, protects
  built-in names, authorizes installs, and keeps checksum enforcement in the
  existing installer.
- CLI commands and Plugins page callbacks use the shared service. The web
  handler compares the current plan with the request before installing.

## Verification

Test config origin, index requests and redirects, shadowing, first-install
confirmation, checksum failure, web rendering, and CLI command behavior.
