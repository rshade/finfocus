---
title: "Estimate pricing inputs plan"
description: "Implementation plan for the shared What-If property policy"
layout: "docs"
---

**Branch**: `2021-estimate-pricing-inputs` | **Date**: 2026-11-19 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/2021-estimate-pricing-inputs/spec.md`

## Summary

What-If builds one editor row per safe property (about 68 on an EC2 instance).
A shared Go policy in `internal/viewmodel` will classify each row as a pricing
input, identity metadata, or other. It returns the rows ordered with pricing
inputs first, each tagged with its class and whether it is visible by default,
plus a read-only details list and a fallback note. The terminal model and the
web handlers both call it, and the page only renders and toggles. Reviewed
mappings are presentation data with provenance, checked by a test against the
real SKU and region extraction. The server rejects identity-key overrides.
No plugin calls, no protocol change.

## Technical Context

**Language/Version**: Go 1.27.4 (`go.mod`); plain browser JavaScript for the page
**Primary Dependencies**: `internal/engine` (`BuildAttributes`, `PricingMetricHint`),
`internal/proto` (`ResolveSKUAndRegion`), finfocus-spec v0.7.12 `pluginsdk/mapping`
key constants, Bubble Tea v2 (TUI), testify
**Storage**: N/A
**Testing**: `go test` with testify; `internal/tui` golden files; node harnesses
under `internal/webui/frontendtest`; `make test-e2e-web`
**Target Platform**: Linux, macOS, Windows (CLI and local web server)
**Project Type**: Single Go module with an embedded web frontend
**Performance Goals**: Classification is a pure in-memory pass over one
descriptor; no added RPC or discovery call (spec FR-010)
**Constraints**: No second credential-name list; no `unsafe`; no provider names
in core logic beyond reviewed data rows; no new plugin protocol field
**Scale/Scope**: One descriptor of up to a few hundred properties; about 8
reviewed AWS types at launch

## Constitution Check

- [x] **Plugin-First Architecture**: Pricing stays in the plugin; core only
  orders and labels rows. The reviewed AWS mapping table is provider knowledge in
  core, which this principle and CLAUDE.md discourage. It is justified in
  Complexity Tracking, not waved through.
- [x] **Test-Driven Development**: Table-driven policy tests, handler tests, JS
  harness tests, and TUI golden files are written before the code.
- [x] **Cross-Platform Compatibility**: Pure Go and JS; no paths or processes.
- [x] **Documentation Integrity**: The Estimate docs page and the web UI
  architecture page list the covered types, fallback, and advanced view.
- [x] **Protocol Stability**: No proto change. The web JSON gains additive fields.
- [x] **Implementation Completeness**: No stubs. Types not reviewed use a real
  fallback, not a TODO.
- [x] **Persistence Model**: No state is stored.
- [x] **Web Parity**: Spec has `Web UI Fit` (extends Estimate). The hook's Go
  methods are named in [contracts/http-api.md](contracts/http-api.md). Tasks carry
  the render check and the `web-ui` follow-up.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`, and
  `make docs-lint` run before handoff.
- [x] **Multi-Repo Coordination**: None. aws-public is read, not changed.
  Plugin-declared inputs stay a future finfocus-spec change.

**Violations Requiring Justification**: One, the AWS mapping table (see
Complexity Tracking).

Post-design re-check: unchanged. The design adds no protocol surface, and the
only core knowledge of AWS is a data table whose rows a test proves against the
extraction code.

## Project Structure

### Documentation (this feature)

```text
specs/2021-estimate-pricing-inputs/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── http-api.md
└── tasks.md             # /speckit.tasks
```

### Source Code (repository root)

```text
internal/viewmodel/
├── estimate_inputs.go          # new: Classify, plan types, identity rule
├── estimate_inputs_mappings.go # new: reviewed mappings (data, no logic)
├── estimate_inputs_test.go     # new: table-driven policy tests
├── estimate_inputs_provenance_test.go # new: ties mappings to extraction
└── estimate.go                 # BuildEstimateDisplay, rows call the policy
internal/webui/
├── handlers_estimate.go        # reject identity overrides; add fields
├── handlers_estimate_test.go
├── static/views/estimate.js    # render class, toggle, details, note
└── frontendtest/               # toggle, focus, override tests
internal/tui/
├── estimate_model.go           # use plan; skip identity rows; toggle key
└── testdata/ (golden)          # regenerated
docs/src/content/docs/          # Estimate page and web-ui architecture page
```

**Structure Decision**: The policy sits in `internal/viewmodel`, the package
both transports already share for estimate rows. It must not import `cli`,
`registry`, or `pluginhost`, which it does not need.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --- | --- | --- |
| Reviewed AWS input mappings live in core (Principle I: core stays provider-agnostic) | Issue #1732 asks for reviewed mappings for supported AWS types, and the editor needs them before any plugin call. Core already holds AWS extraction knowledge for the same reason: `internal/skus/aws.go`, `awsResourceRegion`, and the SDK `ExtractAWSSKU` keys. The table is presentation data. It prices nothing, calls no provider, and a test (`estimate_inputs_provenance_test.go`) proves every `sku` and `region` row against the extraction code. | A plugin-declared input schema is the clean design, but it needs a finfocus-spec change that the issue puts out of scope. Treating every property as a pricing input is the problem the issue reports. Learning inputs from deltas fails on free tiers and combined deltas. |

Exit path: when finfocus-spec carries plugin-declared pricing inputs, the mapping
table becomes the fallback for plugins that declare none, and the policy prefers
what the plugin declares. Recorded for `/speckit.revisit` as a deliberate,
temporary exception.
