---
title: "Estimate picker implementation plan"
description: "Shared conservative classification and transport selection"
layout: "docs"
---

**Branch**: `630-estimate-resource-picker` | **Date**: 2026-10-18 | **Spec**: [spec.md](spec.md)

## Summary

Add a shared engine classification using the existing Pulumi prefix and cached
pricing discovery. Filter only confirmed non-billable candidates in the web list,
return classifications and a preferred URN, and use the same selector for the
interactive CLI. Keep raw source access for baseline/recalculate and Show all.

## Technical Context

- Language: Go 1.27.1; vanilla JavaScript modules.
- Dependencies: existing engine, router, net/http, testify, Node test runner.
- Storage: existing engine-local pricing discovery cache only; no new persistence.
- Platforms: Linux, macOS, Windows; browser desktop and narrow layouts.
- Performance: zero pricing RPCs when listing; classification uses cached metadata.
- Scope: estimate selection only; no overview cost or protocol changes.

## Constitution Check

- [x] Plugin-first: explicit plugin metadata; no cloud catalogue.
- [x] TDD: classifier, handler, frontend, and interactive regression tests first.
- [x] Cross-platform: no platform-specific runtime code.
- [x] Documentation: README, user and API docs, feature status in this change.
- [x] Protocol: no protocol changes or cross-repo dependency.
- [x] Completeness: all tasks require real behavior and checks.
- [x] Persistence: no additional state requirement.
- [x] Quality gates: validate, test, race, lint, docs, frontend, review scheduled.

Post-design check: no violations; existing TUI view rendering is unchanged.

## Project Structure

- `internal/engine/resource_billability.go`: classification and shared candidate selector.
- `internal/webui/handlers_estimate.go`: list filter, classes, preferred URN.
- `internal/webui/static/views/estimate.js`: toggle and selection preservation.
- `internal/cli/cost_estimate.go`: shared initial plan selection.
- Colocated Go tests and `internal/webui/frontendtest/estimate.test.mjs`.
- `specs/630-estimate-resource-picker/`: spec, research, model, contract, tasks.

## Implementation Phases

1. Tests and engine classification; keep source order and identities unchanged.
2. Handler list contract and browser toggle with stale-response protection.
3. CLI initial selection parity, documentation, full verification and review.
