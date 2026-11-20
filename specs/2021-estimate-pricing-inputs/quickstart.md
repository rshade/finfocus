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
6. Pick an unmapped type (for example an RDS instance). Confirm the fallback note.
7. Submit an identity override:
   `curl -s -X POST -H 'Content-Type: application/json'
   -d '{"urn":"<urn>","overrides":{"arn":"x"}}' <url>/api/estimate/recalculate`
   (with the session cookie). Expect 400 `identity_override`.
8. Terminal: `finfocus cost estimate --pulumi-json <plan>` interactive; confirm the
   identity values appear in a read-only block the cursor cannot enter, and the
   show-all key reveals the other properties.
9. Stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.

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
