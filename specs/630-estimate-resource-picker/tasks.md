---
title: "Estimate picker tasks"
description: "Dependency-ordered implementation and verification"
layout: "docs"
---

## Phase 1: Setup

- [x] T001 Reconcile issue and write specification in specs/630-estimate-resource-picker/spec.md.
- [x] T002 Research cached pricing and write specs/630-estimate-resource-picker/plan.md and contracts/estimate-resources.md.

## Phase 2: Foundation

- [x] T003 Write conservative classification tests in internal/engine/resource_billability_test.go.
- [x] T004 Implement classification and selection in internal/engine/resource_billability.go; reuse bookkeeping rule in engine.go.

## Phase 3: US1 - Eligible Defaults

Independent test: mixed providers/stack/cloud resources retain only confirmed candidates.

- [x] T005 [US1] Write handler filter and metadata tests in internal/webui/handlers_estimate_test.go.
- [x] T006 [US1] Implement list filtering after source overlays in internal/webui/handlers_estimate.go.

## Phase 4: US2 - Show All

Independent test: toggle round trips retain selection and edits, and restore an empty list.

- [x] T007 [US2] Write toggle, empty, and stale-response tests in internal/webui/frontendtest/estimate.test.mjs and browser acceptance in test/e2e/web_estimate_picker_test.go using fixtures/web/estimate-picker-*.json.
- [x] T008 [US2] Implement checkbox and preserved selection/editor in internal/webui/static/views/estimate.js; align the native checkbox in internal/webui/static/styles.css.

## Phase 5: US3 - Interactive Parity

Independent test: shared mixed-plan selection skips providers in terminal mode.

- [x] T009 [US3] Add plan-selection parity tests in internal/cli/cost_estimate_interactive_test.go.
- [x] T010 [US3] Apply shared selection in internal/cli/cost_estimate.go.

## Phase 6: Polish

- [x] T011 Update README.md, docs/src/content/docs/guides/web-ui.md and specs/627-web-ui/contracts/http-api.md.
- [x] T012 Run repository gates and browser checks; record results in specs/630-estimate-resource-picker/quickstart.md.
- [x] T013 Review base c9634c6 diff and scout touched files; finalize specs/630-estimate-resource-picker/spec.md status.

## Dependencies and Execution

T001 → T002 → T003 → T004. US1 follows foundation; US2 follows US1.
US3 uses foundation independently. Documentation and review follow all stories.
Handler tests and CLI parity tests can be prepared independently in separate files.
Deliver US1 default filtering first, then the escape hatch and interactive parity;
all stories and gates are required for completion.
