---
title: "Estimate pricing inputs quickstart"
description: "How to check the feature by hand"
layout: "docs"
---

1. Build: `make build`.
2. Start the dashboard from a populated project:
   `PULUMI_CONFIG_PASSPHRASE='' ./bin/finfocus --web --no-browser --port <port>`
   run from `../finfocus-demo`.
3. Open Estimate and pick an `aws:ec2/instance:Instance`. Confirm the first rows
   are `instanceType` and `availabilityZone` (and `region` when the descriptor
   carries one; the region often comes from the provider, so it may be absent),
   and the ARN, cloud ID, and URN appear under read-only details.
4. Change `instanceType` from `t3.micro` to `t3.large`. Confirm the estimate and
   delta change.
5. Turn on Show all properties. Confirm the other safe properties follow, edit
   one, turn the toggle off, and confirm the edit stays visible.
6. Pick an unmapped type (for example the demo's `aws:ec2/vpc:Vpc`). Confirm
   the fallback note.
6a. Usage rows (spec FR-012): the demo instance has no `tenancy`, so confirm an
   empty, editable `tenancy` row follows the present inputs, and that the page
   sends nothing for it until you type a value. Pick the demo volume and confirm
   it leads with `type` and `size`.
6b. Plugin scope (research R14): on the volume, switch Pricing mode to the
   civo-public mode. Confirm `size` (a `plugin-input` row) disappears and `type`
   (the SKU row) stays; type a `tenancy` value on the instance, switch modes, and
   confirm the value stays visible as an edited property.
6c. Usage value reaches the plugin (research R13): on a Lambda function or S3
   bucket descriptor, set a usage row (`requestsPerMonth`, `storageClass`) and
   confirm the modified cost moves.
7. Submit an identity override:
   `curl -s -X POST -H 'Content-Type: application/json'
   -d '{"urn":"<urn>","overrides":{"arn":"x"}}' <url>/api/estimate/recalculate`
   (with the session cookie). Expect 400 `identity_override`.
8. Terminal: `finfocus cost estimate --pulumi-json <plan>` interactive; confirm the
   identity values appear in a read-only block the cursor cannot enter, and the
   show-all key reveals the other properties.
9. Stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.

## Verification results (2026-11-20, usage rows)

| Check | Result |
| --- | --- |
| `go test ./internal/viewmodel/ ./internal/tui/ ./internal/webui/` | pass |
| `go test -race -shuffle=on -count=3 ./internal/viewmodel/... ./internal/webui/...` | pass |
| `golangci-lint run` on those three packages | 0 issues |
| estimate page tests (`node --test internal/webui/frontendtest/estimate.test.mjs`) | 39 pass |
| `TestWebUIEstimatePricingInputs` | pass (empty `tenancy`, then `dedicated` kept) |
| `make test` with `NO_COLOR` unset | pass, re-run after the review fixes. An earlier run failed only in `internal/weblog` because `NO_COLOR=1` suppressed the colour escapes those tests require |
| `make test-race` with `NO_COLOR` unset | pass |
| `make validate` (`go mod tidy -diff`, `go vet ./...`) | pass |
| `make docs-lint` and markdownlint on the feature Markdown | pass |
| `make lint` | pass when `PNPM` is a working pnpm 12.21.0. The default `pnpm` shim on this machine cannot find `pnpm.cjs` |
| Coverage | `viewmodel` 97.3%, `tui` 89.6%, `webui` 90.6%, `engine` 92.7%. Classification and identity rejection (`BuildEstimatePropertyPlan`, `classifyRows`, `classifyMappedRows`, `appendReviewedAbsences`, `RejectIdentityOverrides`) are 100%. `NewEstimateInputContext` is 85.0% |
| Review (T033) | Two confirmed findings fixed: a redacted property is not recreated from an override, and usage rows sort after the present pricing inputs |

## Verification results (2026-11-19)

| Check | Result |
| --- | --- |
| `make validate`, `go vet ./...` | pass |
| `make test`, `make test-race` | pass |
| `go test -race -shuffle=on -count=3 ./internal/viewmodel/... ./internal/webui/...` | pass |
| `make test-frontend` | 264 pass |
| `make test-e2e-web` (includes `TestWebUIEstimatePricingInputs`) | pass |
| `make lint` | Go modules: 0 issues. The run stops at the VS Code step: `pnpm` has no version set in this environment |
| `make docs-lint`, markdownlint on changed Markdown | 0 issues |
| Coverage | `viewmodel` 97.1% (100% on the classification and identity-rejection paths), `tui` 89.7%, `webui` 90.5%, `engine` 92.7% |

An earlier `make test-race` run failed in unrelated packages while the disk was at
99%; it passed once rerun.
