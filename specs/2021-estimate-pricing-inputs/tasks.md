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

**Clarified design (2026-11-20)**: T001–T032 implement the policy from before the
usage-row clarifications. T035–T054 are the delta in plan.md Design (research
R12–R17). Do not redo a checked task.

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
- [X] T028 Render Estimate with playwright-mcp against `../finfocus-demo` (`PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>`; screenshots under `.playwright-mcp/`), compare with the spec's Web UI Fit section, and fill the Render evidence line in specs/2021-estimate-pricing-inputs/spec.md; stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.
- [X] T029 Update docs/src/content/docs/guides/web-ui.md (the What-If section: default pricing inputs, Show all properties, read-only details, identity rejection, covered types, fallback note) and docs/src/content/docs/architecture/web-ui.md (layout slots); describe the terminal editor in docs/src/content/docs/guides/user-guide.md.
- [X] T030 After the PR opens, file one UX-only `web-ui` follow-up issue (layout, rendering, interaction, copy, accessibility; no engine, API, redaction, or data-shape work) naming this issue, the PR, and the Estimate view; create the `web-ui` label first if absent. *(2026-11-20: waived. The web Estimate view ships in this feature and calls the same Go plan as the terminal. No separate UX follow-up issue.)*

## Phase 9: Polish and Verification

- [X] T031 [P] Add a short note to CLAUDE.md (Package-Specific Gotchas, CLI): the property policy lives in `internal/viewmodel`, JavaScript never classifies, identity keys are exact, and a new mapping needs its provenance test row.
- [X] T032 Run `make validate`, `make test`, `make test-race`, `make lint`, `make docs-lint`, and markdownlint on the changed Markdown outside `docs/`; run `go vet ./...`; run `go test -cover ./internal/viewmodel/... ./internal/tui/... ./internal/webui/... ./internal/engine/...` and confirm the changed code reaches 80% (95% for the classification and identity-rejection paths); run `go test -race -shuffle=on -count=3 ./internal/viewmodel/... ./internal/webui/...`; record results in specs/2021-estimate-pricing-inputs/quickstart.md.
- [X] T033 Run the code-review skill and the scout agent against the recorded base `d603276b`, then run the `local-cli-review` skill with `--uncommitted`; verify each finding with `verify-fix` and fix only the ones that hold up.
- [X] T034 Set the spec status to Implemented in specs/2021-estimate-pricing-inputs/spec.md and tick every task here.

## Dependencies and Execution

- T001 and T002 first; T001 can change the mapping list T007 implements.
- T003 → T004. T005 and T006 can be written beside T003. T007 needs T004, T005, T006.
  T008 needs T007. Every story needs T008.
- US1 (T009 to T012) is the MVP. US2 and US3 each build on the US1 handler and
  page, and touch the same two files, so run them in order, not in parallel.
- US4 follows US2 (it tests the toggle). US5 needs only Phase 2, so the terminal
  work (T022 to T024) can run beside US1 to US3 once T008 is done.
- Web Parity follows US1 to US5. T030 is waived. Polish is last.
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

The clarified delta starts at Phase 10. Its MVP is Phase 10 plus US1 (T035–T041):
an absent `plugin-input` becomes a usage row while aws-public serves the
estimate. US2, US4, and US5 then lock plugin scope, mode switches, and parity.
US3 needs no new task; T035 asserts a usage key is not an identity key.

## Phase 10: Foundational delta (blocks the clarified stories)

**Goal**: The shared policy emits usage rows and applies `plugin-input` rows
only for the mapping's plugin. No story is done until this phase compiles.

- [X] T035 Write failing table-driven tests in internal/viewmodel/estimate_inputs_test.go against `NewEstimateInputContext` and the new `BuildEstimatePropertyPlan` signature: a Lambda `requestsPerMonth` and an S3 `storageClass` absent from the descriptor are usage rows (empty original, editable, default-visible, `Usage` true, mapping order, curated true) when the context's selected plugin or, under Automatic, its discovered plugins include `aws-public`; an absent EC2 `region` makes no row; a context whose selected plugin is `civo-public` drops `plugin-input` and usage rows and keeps `instanceType`; an empty discovery keeps only `sku` and `region` rows, so S3 is not curated; a non-empty usage override is the current value and an empty one is not overridden; an override whose usage row does not apply becomes an `other` overridden row; a hidden override key makes no row; a usage key is not rejected by `RejectIdentityOverrides`.
- [X] T036 [P] Extend internal/viewmodel/estimate_inputs_provenance_test.go so the RDS `instanceClass` sku row and the RDS `region` and `availabilityZone` rows resolve through `proto.ResolveSKUAndRegion`, and every mapping's `Plugin` is `aws-public`.
- [X] T037 Add `InputMapping.Plugin`, `EstimateInputContext`, and `NewEstimateInputContext(discovery, pricingModeID)` in internal/viewmodel/estimate_inputs.go. `BuildEstimatePropertyPlan` takes that context instead of bare hints. Trim a `finfocus-plugin-` prefix before comparing plugin names. Emit usage rows and the surviving-override rows from research R15. Set `Curated` from plan.md Design. Set `Plugin` to `aws-public` on every mapping and add the RDS mapping (`instanceClass` sku, `region` and `availabilityZone` region, `engine`, `storageType`, `allocatedStorage`, `multiAz` plugin-input) in internal/viewmodel/estimate_inputs_mappings.go, using `pluginsdk/mapping` constants where they exist. Update every current caller so the package compiles, including internal/viewmodel/estimate.go, internal/viewmodel/estimate_pricing.go, internal/webui/handlers_estimate.go, and internal/tui/estimate_properties.go. Godoc every new exported symbol.
- [X] T038 Make `BuildEstimateDisplay` and `baselineFailureDisplay` take an `EstimateInputContext` and pass it through, in internal/viewmodel/estimate.go and internal/webui/handlers_estimate.go. `SetCurrentValue` stays the one visibility rule.

## Phase 11: US1 - Usage rows the user can fill (P1)

**Goal**: An absent reviewed `plugin-input` is an empty row, and a typed value is sent as an override.

**Independent test**: Open What-If for a Lambda or S3 descriptor that lacks the usage key. The row is empty and editable. A value changes the recalculated estimate the same way the CLI override does. Leaving it empty sends nothing.

- [X] T039 [US1] Add handler tests in internal/webui/handlers_estimate_test.go: the baseline for a Lambda without `requestsPerMonth` returns that usage row (`usage` true, empty `originalValue`, `defaultVisible` true) after the present pricing inputs; a recalculate whose override is `requestsPerMonth=1000000` reaches the engine; an empty usage value is absent from the engine overrides.
- [X] T040 [P] [US1] Add page tests in internal/webui/frontendtest/estimate.test.mjs: a usage row renders an empty input; a typed value is in `overrides()`; clearing it removes the override; the row stays in server order.
- [X] T041 [US1] Build the context with `viewmodel.NewEstimateInputContext(found, query.PricingMode)` on both baseline and recalculate in internal/webui/handlers_estimate.go. In internal/webui/static/views/estimate.js, send a usage input only when its value is non-empty.

## Phase 12: US2 - Another plugin drops the usage rows (P1)

**Goal**: A pricing mode from another plugin removes `plugin-input` and usage rows and keeps `sku` and `region` rows. A type left with no applied row takes the fallback.

**Independent test**: On an EC2 instance, select a non-aws-public mode and confirm `instanceType` stays and `tenancy` goes. On an S3 bucket with only that plugin discovered, confirm the fallback note and every safe property.

- [X] T042 [US2] Add handler tests in internal/webui/handlers_estimate_test.go: discovery that reports only `civo-public` leaves an EC2 `instanceType` row and no `tenancy` usage row, and leaves an S3 bucket uncurated with the fixed note and every safe row `defaultVisible`.
- [X] T043 [P] [US2] Add a page test in internal/webui/frontendtest/estimate.test.mjs: that uncurated response shows the note and every row without a further request.

## Phase 13: US4 - A removed usage value stays an edit (P2)

**Goal**: Switching mode replans before pricing, and a value entered on a row the new mode removes stays visible as an edited property.

**Independent test**: Type `tenancy`, switch to a mode that drops that row, and confirm the value, the override, and the focused key survive.

- [X] T044 [US4] Add terminal tests in internal/tui/estimate_model_test.go: `selectPricingMode` replans with the new mode's plugin before recalculating; a committed usage edit stays by key when the new plan has no usage row; the cursor row is kept.
- [X] T045 [P] [US4] Add page tests in internal/webui/frontendtest/estimate.test.mjs: after a mode change, a late response that returns the typed key as an `other` overridden row keeps the typed value and focus on that key.
- [X] T046 [US4] Call `replan` from `selectPricingMode` with the selected mode before `triggerRecalculation` in internal/tui/estimate_model.go. Pass the mode's plugin through `buildPlan` in internal/tui/estimate_properties.go. Keep edits and focus by key.

## Phase 14: US5 - Both transports use the context (P2)

**Goal**: The web display and the terminal model produce the same keys, order, and classes for a usage row and for a non-serving plugin.

**Independent test**: One S3 descriptor with aws-public discovered, and one EC2 descriptor with another plugin selected, match across `BuildEstimateDisplay` and the terminal plan.

- [X] T047 [US5] Extend internal/webui/parity_estimate_test.go with the S3 usage-row fixture and the EC2 non-aws-public fixture. Compare keys, order, and classes. Neither side shows a hidden key.
- [X] T048 [P] [US5] Regenerate the curated, advanced, and fallback goldens in internal/tui/golden_test.go with `UPDATE_GOLDEN=1 go test -run TestGolden ./internal/tui/...` when a fixture gains a usage row, and read each rendered view.

## Phase 15: Web parity for the delta (Principle VII)

- [X] T049 Record `viewmodel.NewEstimateInputContext` and the `usage` field in specs/627-web-ui/contracts/http-api.md so it matches specs/2021-estimate-pricing-inputs/contracts/http-api.md.
- [X] T050 [P] Describe usage rows, plugin scope, and the RDS mapping in docs/src/content/docs/guides/web-ui.md and the terminal editor in docs/src/content/docs/guides/user-guide.md.
- [X] T051 [P] Extend test/e2e/web_estimate_pricing_inputs_test.go so a usage row round-trips an entered value and sends nothing while empty, beside the existing EC2 and identity cases. Run `make test-e2e-web`.
- [X] T052 Render Estimate with playwright-mcp against `initial-30` stack `dev` and record a usage row (empty `tenancy` on the instance, or `type` and `size` on the volume) in the Render evidence line of specs/2021-estimate-pricing-inputs/spec.md. Stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.

## Phase 16: Polish for the delta

- [X] T053 [P] Add the usage-row and plugin-scope rule to the What-If note in CLAUDE.md (Package-Specific Gotchas, CLI): JavaScript never classifies, and a new `sku` or `region` mapping needs its provenance test row.
- [X] T054 Re-run `make validate`, `make test`, `make test-race`, `make lint`, `make docs-lint`, and markdownlint on the changed Markdown outside `docs/`. Run `go vet ./...` and `go test -cover ./internal/viewmodel/... ./internal/tui/... ./internal/webui/... ./internal/engine/...`. Confirm the changed code reaches 80% (95% on classification and identity rejection). Run `go test -race -shuffle=on -count=3 ./internal/viewmodel/... ./internal/webui/...`. Record the results in specs/2021-estimate-pricing-inputs/quickstart.md.

## Delta dependencies

- T035 and T036 first. T037 needs both. T038 needs T037.
- US1 (T039–T041) needs T038. US2 tests need T041. US4 needs T038; T046 needs T044. US5 needs T046 and T041.
- Web parity follows US1–US5. T030 is waived. T033 reviews the whole delta. T034 is last.

## Delta parallel examples

- T035 beside T036.
- After T038: T039 and T040.
- After T041: T042 and T043.
- After T038: T044 and T045, then T046.
- After the stories: T047 and T048; then T049, T050, and T051.
