---
title: "Estimate picker validation"
description: "Regression checks and browser inspection"
layout: "docs"
---

## Automated Checks

```bash
go test ./internal/engine ./internal/webui ./internal/cli
node --experimental-vm-modules --test internal/webui/frontendtest/estimate.test.mjs
make validate
make test
make test-race
make lint
make docs-lint
```

## Browser Validation

Run `finfocus --web` against a mixed state/plan fixture or deployed stack.
Open What-If at desktop and narrow widths. Verify provider/stack entries are
hidden, priced candidates are useful initial selections, and unknown zero-spend
resources remain. Toggle Show all resources both ways and retain an eligible
selection and edits. With only bookkeeping sources, verify the empty explanation
and toggle restoration. Inspect the authenticated resources endpoint with and
without `showAll=true`; identities remain unchanged and secrets stay redacted.

## Verification Results (2026-10-18)

- Passed `make validate`, `make test`, `make test-race`, and `make docs-lint`.
- Passed `make lint GOLANGCI_LINT="$(mise which golangci-lint)"`; the default
  Go-bin linter was built with an older Go version than this project requires.
- Passed all 85 frontend regression tests, including 18 estimate tests.
- Targeted engine classification/selection coverage: 100% of statements.
- Passed real-binary browser acceptance with local plugins and mixed state/plan
  fixtures at 1280px and 390px. Inspected screenshots; no horizontal overflow.
  This check uses fixtures, not a live cloud deployment.
- Standards and spec reviews: no outstanding findings after cancellation and
  cleanup corrections. Scout opportunities were reported without scope expansion.
- Spec Kit analysis: all six functional requirements map to tasks; no ambiguous,
  duplicated, uncovered, or constitution-conflicting requirements remain.

The task setup script lacked template resolver functions; tasks used the checked-in
repository template, and the prerequisite checker passed.
