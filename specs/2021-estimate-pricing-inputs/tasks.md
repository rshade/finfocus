---
title: "Estimate pricing inputs tasks"
description: "Dependency-ordered implementation and verification"
layout: "docs"
---

**Input**: `specs/2021-estimate-pricing-inputs/` (spec, plan, research, data
model, contract, quickstart)

**Tests**: Mandatory and written first (constitution Principle II). Use testify
(`require` to stop, `assert` to continue) and table-driven cases. TUI changes
need golden files and a rendered view read by eye. Tests that set env or build a
root command are not parallel.

**Completeness**: No stubs or TODOs (Principle VI).

## Phase 1: Setup

- [X] T001 Verify the plugin-input names from research R2 and R3 against the installed aws-public: run `finfocus plugin inspect aws-public <type>` for each launch type and record the accepted names and the EC2 `region` finding in specs/2021-estimate-pricing-inputs/research.md. Drop any mapping the plugin does not read.
- [X] T002 Capture the real EC2 descriptor keys the web editor receives from `../finfocus-demo` (`GET /api/estimate/baseline`) and confirm the identity rule in research R5 against them; record which of `pulumi:*`, `terraform:*`, `id`, `arn`, `urn` occur in specs/2021-estimate-pricing-inputs/research.md.

## Phase 2: Foundation (blocks every story)

- [X] T003 [P] Write redaction tests for an exported `engine.RedactedProperties` (credential-like key, `__` key, Pulumi secret, unrepresentable value, descriptor over `MaxAttributesBytes` still returns its rows) in internal/engine/attributes_test.go.
- [X] T004 Export `RedactedProperties` from the existing `redactedProperties` in internal/engine/attributes.go and switch its callers; behavior is unchanged.
- [X] T005 [P] Write table-driven policy tests in internal/viewmodel/estimate_inputs_test.go: each launch type lists its reviewed keys first in mapping order; an absent reviewed key makes no row; unknown type and mapped-type-with-no-reviewed-key give `Curated` false with the fixed note and every safe row visible; a `$0` delta and an unchanged estimate never demote a pricing row; `pulumi:`/`terraform:` keys and exact `id`, `arn`, `urn` become `Details` while `subnetId` and a tag named `arn` under `tags` stay editable; an overridden `other` row is `DefaultVisible`; an override naming a property the descriptor lacks is accepted and makes no identity or pricing claim; a usage hint matches only an exact key; failed discovery gives no hints; credential-like keys and Pulumi secrets never appear in `Rows` or `Details`; the union of `Rows` and `Details` equals the redacted input keys.
- [X] T006 [P] Write the provenance test in internal/viewmodel/estimate_inputs_provenance_test.go: for every `sku` and `region` mapping entry, `proto.ResolveSKUAndRegion` over `{key: value}` returns that value, so no mapping claims a key the extraction does not read; also assert each mapping's resource segment resolves through `resourcetype.NormalizeProvider`.
- [X] T007 Implement `PropertyClass`, `InputProvenance`, `EstimatePropertyRow` additions, `EstimateDetail`, `EstimatePropertyPlan`, `BuildEstimatePropertyPlan`, and `IdentityOverrideError`/`RejectIdentityOverrides(overrides)` in internal/viewmodel/estimate_inputs.go, and the reviewed AWS mappings (EC2 instance, EBS volume, Lambda function, DynamoDB table, `lb` and `alb` load balancer, ElastiCache cluster, S3 bucket, NAT gateway, as confirmed by T001) as data in internal/viewmodel/estimate_inputs_mappings.go. Mapping keys use the SDK `pluginsdk/mapping` constants where they exist. The plan redacts through `engine.RedactedProperties` itself. Every exported symbol gets a godoc comment (constitution Principle IV).
- [X] T008 Add `class`, `provenance`, `editable`, `defaultVisible`, `overridden` to `EstimateProperty` and `details`, `curated`, `note` to `EstimateDisplay` in internal/viewmodel/estimate.go; make `BuildEstimateDisplay` take the usage hints and build rows through `BuildEstimatePropertyPlan`; keep `BuildEstimatePropertyRows` as the row builder the plan calls. Update internal/viewmodel/estimate_test.go and internal/viewmodel/estimate_spot_test.go for the new signature.

## Phase 3: US1 - Find the inputs that change the price (P1)

**Independent test**: the EC2 fixture's baseline response lists `instanceType` first, with `defaultVisible` true only for pricing inputs, and `t3.micro` to `t3.large` recalculates the same estimate as the CLI.

- [X] T009 [US1] Write handler tests in internal/webui/handlers_estimate_test.go: baseline and recalculate responses carry ordered `display.properties` with `class` and `defaultVisible`, and the EC2 `instanceType` is the first row (SC-001); `DiscoverPricingSpec` runs on both paths and gives the same classification on each; the `t3.large` override matches the engine result for the same request; the baseline-failure response carries the same fields with no deltas.
- [X] T010 [US1] Call `eng.DiscoverPricingSpec` before building `display` on both paths in `Server.serveEstimate`, pass the hints through, and give `baselineFailureDisplay` the same plan, in internal/webui/handlers_estimate.go.
- [X] T011 [P] [US1] Write page tests in internal/webui/frontendtest/estimate.test.mjs: rows render in the server's order; only `defaultVisible` rows render; a `terraform:`-prefixed row sent by the server renders as given, since the page no longer filters.
- [X] T012 [US1] Render only `defaultVisible` rows, in server order, and delete the `terraform:` prefix filter in `renderRows` in internal/webui/static/views/estimate.js.

## Phase 4: US2 - Keep every safe property reachable (P1)

**Independent test**: on a mapped and on an unmapped type, the union of default and advanced rows equals the safe non-identity properties.

- [X] T013 [US2] Add handler tests for the unknown-type and incomplete-metadata fallback (`curated` false, fixed `note`, every row `defaultVisible`) in internal/webui/handlers_estimate_test.go.
- [X] T014 [US2] Write page tests in internal/webui/frontendtest/estimate.test.mjs: the Show all properties checkbox reveals the other rows with no request; the note appears in the status line only when `curated` is false.
- [X] T015 [US2] Add the Show all properties checkbox beside Show all resources, filter rows by `defaultVisible` or the checkbox, and show `display.note` in the status line in internal/webui/static/views/estimate.js; align the control in internal/webui/static/styles.css.

## Phase 5: US3 - Identity metadata is read-only (P1)

**Independent test**: a recalculate request overriding `arn` answers 400 `identity_override` and prices nothing.

- [X] T016 [US3] Write handler tests in internal/webui/handlers_estimate_test.go: each identity key (`arn`, `urn`, `id`, `pulumi:cloudId`, `terraform:arn`) answers 400 `identity_override` naming the first sorted key, never echoes the value, and makes no engine call; the check runs before the resource lookup; a tag or `subnetId` override still works.
- [X] T017 [US3] Call `viewmodel.RejectIdentityOverrides` in `handleEstimateRecalculate` before `serveEstimate`, add the `identity_override` code to `writeError` use, in internal/webui/handlers_estimate.go.
- [X] T018 [US3] Write page tests in internal/webui/frontendtest/estimate.test.mjs: `display.details` renders as a read-only list with no input, and none of its keys can enter `overrides()`.
- [X] T019 [US3] Render `display.details` as a read-only definition list under the table in internal/webui/static/views/estimate.js, with the matching style in internal/webui/static/styles.css.

## Phase 6: US4 - State survives toggling and rerendering (P2)

**Independent test**: edit two inputs, toggle both ways, recalculate, and compare values, overrides, deltas, and focused element.

- [X] T020 [US4] Write page tests in internal/webui/frontendtest/estimate.test.mjs: a toggle round trip keeps current values, overrides, and deltas; an override on an `other` row stays visible with the toggle off; focus and selection stay on the same `key` across a recalculation rerender and a toggle; a late response after a toggle does not rewrite the editor.
- [X] T021 [US4] Make the toggle rebuild rows from the last response and keep typing state and focus by `key` in `renderRows` in internal/webui/static/views/estimate.js; fix only what T020 shows failing.

## Phase 7: US5 - Web and terminal agree (P2)

**Independent test**: the same descriptors give identical ordered keys and classes through the web display and the terminal model; the terminal never shows a credential-like key.

- [X] T022 [US5] Write terminal tests in internal/tui/estimate_model_test.go: rows come from `BuildEstimatePropertyPlan`; identity values sit in a read-only block the cursor cannot enter; the show-all key reveals the other rows; an override on an `other` row stays visible; a credential-like key and a Pulumi secret never render; `RejectIdentityOverrides` guards the override map; when `estimatePricingSpecMsg` delivers hints after the model is built, the rows are re-planned and the cursor row and every typed or committed edit are kept by key, and a row the user edited never disappears; `GetOverrides` still returns overrides for hidden rows; the help text lists the show-all key.
- [X] T023 [US5] Build rows from the plan in `initializeProperties`, bind show-all to the `a` key (free: `q`, Enter, Esc, the arrows, and Ctrl+C are taken) and list it in `RenderEstimateHelp` in internal/tui/delta_view.go, render only the visible subset with the cursor, `focusedRow`, and the property table using visible indexes while `GetOverrides` and `triggerRecalculation` keep reading every row, re-plan when discovery hints arrive (keep cursor and edits by key), add a read-only details block to `renderEditingView`, show the fallback note, and guard `triggerRecalculation` with `RejectIdentityOverrides` in internal/tui/estimate_model.go; feed discovery hints from `WithPricingDiscovery` into the plan in internal/tui/estimate_pricing.go.
- [X] T024 [P] [US5] Add golden snapshots for a curated type, the advanced view, and the fallback note in internal/tui/golden_test.go; regenerate with `UPDATE_GOLDEN=1 go test -run TestGolden ./internal/tui/...` and read each rendered view.
- [X] T025 [P] [US5] Write the parity test in internal/webui/parity_estimate_test.go: for the EC2, EBS, unknown-type, and credential-bearing fixtures, the web `display.properties` keys, order, and classes equal the terminal model's, and neither carries a hidden key.

## Phase 8: Web Parity (Principle VII)

- [X] T026 Record the Go methods the hook calls (`viewmodel.BuildEstimatePropertyPlan`, `BuildEstimateDisplay`, `RejectIdentityOverrides`, `engine.RedactedProperties`, `(*engine.Engine).DiscoverPricingSpec`) and the additive fields in specs/627-web-ui/contracts/http-api.md, matching specs/2021-estimate-pricing-inputs/contracts/http-api.md.
- [X] T027 [P] Add a browser acceptance test in test/e2e/web_estimate_pricing_inputs_test.go with a fixture beside test/e2e/fixtures/web/: EC2 `t3.micro` to `t3.large`, Show all properties round trip, and an identity override rejected; run `make test-e2e-web`.
- [ ] T028 Render Estimate with playwright-mcp against `../finfocus-demo` (`PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>`; screenshots under `.playwright-mcp/`), compare with the spec's Web UI Fit section, and fill the Render evidence line in specs/2021-estimate-pricing-inputs/spec.md; stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.
- [X] T029 Update docs/src/content/docs/guides/web-ui.md (the What-If section: default pricing inputs, Show all properties, read-only details, identity rejection, covered types, fallback note) and docs/src/content/docs/architecture/web-ui.md (layout slots); describe the terminal editor in docs/src/content/docs/guides/user-guide.md.
- [ ] T030 After the PR opens, file one UX-only `web-ui` follow-up issue (layout, rendering, interaction, copy, accessibility; no engine, API, redaction, or data-shape work) naming this issue, the PR, and the Estimate view; create the `web-ui` label first if absent.

## Phase 9: Polish and Verification

- [X] T031 [P] Add a short note to CLAUDE.md (Package-Specific Gotchas, CLI): the property policy lives in `internal/viewmodel`, JavaScript never classifies, identity keys are exact, and a new mapping needs its provenance test row.
- [X] T032 Run `make validate`, `make test`, `make test-race`, `make lint`, `make docs-lint`, and markdownlint on the changed Markdown outside `docs/`; run `go vet ./...`; run `go test -cover ./internal/viewmodel/... ./internal/tui/... ./internal/webui/... ./internal/engine/...` and confirm the changed code reaches 80% (95% for the classification and identity-rejection paths); run `go test -race -shuffle=on -count=3 ./internal/viewmodel/... ./internal/webui/...`; record results in specs/2021-estimate-pricing-inputs/quickstart.md.
- [ ] T033 Run the code-review skill and the scout agent against the recorded base `d603276b`, then run the `local-cli-review` skill with `--uncommitted`; verify each finding with `verify-fix` and fix only the ones that hold up.
- [ ] T034 Set the spec status to Implemented in specs/2021-estimate-pricing-inputs/spec.md and tick every task here.

## Dependencies and Execution

- T001 and T002 first; T001 can change the mapping list T007 implements.
- T003 → T004. T005 and T006 can be written beside T003. T007 needs T004, T005, T006.
  T008 needs T007. Every story needs T008.
- US1 (T009 to T012) is the MVP. US2 and US3 each build on the US1 handler and
  page, and touch the same two files, so run them in order, not in parallel.
- US4 follows US2 (it tests the toggle). US5 needs only Phase 2, so the terminal
  work (T022 to T024) can run beside US1 to US3 once T008 is done.
- Web Parity follows US1 to US5. T030 follows the PR. Polish is last.
- Within each story the test task comes first and must fail for the right reason
  before its implementation task.

## Parallel Examples

- After T001: T003, T005, T006 in three files.
- After T008: T011 (page tests), T022 (terminal tests), and T009 (handler tests).
- Phase 7: T024 and T025 after T023.
- Phase 8: T027 beside T026 and T029.

## Implementation Strategy

Deliver Phase 2 and US1 first: that alone makes EC2 usable and is the issue's
core. Add the escape hatch (US2) and read-only identity with server enforcement
(US3) before calling any of it shippable, since narrowing the default without
them would hide fields. US4 and US5 harden state and parity; all stories, the
Web Parity phase, and the gates are required for completion.
