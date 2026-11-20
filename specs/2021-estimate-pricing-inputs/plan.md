---
title: "Estimate pricing inputs plan"
description: "Implementation plan for the shared What-If property policy"
layout: "docs"
---

**Branch**: `2021-estimate-pricing-inputs` | **Date**: 2026-11-20 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/2021-estimate-pricing-inputs/spec.md`

## Summary

What-If builds one editor row per safe property (about 68 on an EC2 instance).
A shared Go policy in `internal/viewmodel` classifies each row as a pricing
input, identity metadata, or other. Pricing inputs come from reviewed mappings
plus pricing-spec usage hints. A reviewed `plugin-input` key the descriptor
does not carry is an empty editable usage row (FR-012). Those rows, and the
present `plugin-input` rows, apply only when the mapping's plugin serves the
estimate; `sku` and `region` rows always apply. The terminal model and the web
handlers call the same plan, and the page only renders and toggles. The server
rejects identity-key overrides. No new plugin call and no protocol change.

## Technical Context

**Language/Version**: Go 1.27.4 (`go.mod`); plain browser JavaScript for the page
**Primary Dependencies**: `internal/engine` (`BuildAttributes`,
`PricingDiscovery`, `PricingMetricHint`), `internal/proto`
(`ResolveSKUAndRegion`), finfocus-spec v0.7.12 `pluginsdk/mapping` key
constants, Bubble Tea v2 (TUI), testify
**Storage**: N/A
**Testing**: `go test` with testify; `internal/tui` golden files; node harnesses
under `internal/webui/frontendtest`; `make test-e2e-web`
**Target Platform**: Linux, macOS, Windows (CLI and local web server)
**Project Type**: Single Go module with an embedded web frontend
**Performance Goals**: Classification is a pure in-memory pass over one
descriptor. It reads the pricing-spec discovery the estimate already runs
(spec FR-010) and adds no RPC.
**Constraints**: No second credential-name list; no `unsafe`; the policy does
not import `cli`, `registry`, or `pluginhost`; no new plugin protocol field
**Scale/Scope**: One descriptor of up to a few hundred properties; ten reviewed
AWS types, RDS included, S3 and NAT curated through usage rows

## Constitution Check

- [x] **Plugin-First Architecture**: Pricing stays in the plugin; core only
  orders and labels rows. The reviewed AWS mapping table is provider knowledge
  in core, which this principle and CLAUDE.md discourage. It is justified in
  Complexity Tracking, not waved through.
- [x] **Test-Driven Development**: Table-driven policy tests, handler tests, JS
  harness tests, and TUI golden files are written before the code. The usage-row
  and plugin-scope cases follow the same rule.
- [x] **Cross-Platform Compatibility**: Pure Go and JS; no paths or processes.
- [x] **Documentation Integrity**: The Estimate docs page and the web UI
  architecture page list the covered types, fallback, advanced view, and usage
  rows (FR-011).
- [x] **Protocol Stability**: No proto change. The web JSON gains additive
  fields, including `usage` on a property row.
- [x] **Implementation Completeness**: No stubs. Types not reviewed use a real
  fallback, not a TODO. A usage row is a real row, not a placeholder.
- [x] **Persistence Model**: No state is stored.
- [x] **Web Parity**: Spec has `Web UI Fit` (extends Estimate). The hook's Go
  methods are named in [contracts/http-api.md](contracts/http-api.md). Tasks
  carry the render check and the `web-ui` follow-up.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`, and
  `make docs-lint` run before handoff.
- [x] **Multi-Repo Coordination**: None. aws-public v0.2.5 is read, not changed.
  Plugin-declared inputs stay a future finfocus-spec change.

**Violations Requiring Justification**: One, the AWS mapping table (see
Complexity Tracking).

Post-design re-check: unchanged. Usage rows and the plugin name on a mapping
are still presentation data. A usage value reaches the plugin through the
override merge the estimate already performs (research R13). No protocol
surface is added.

## Design

The clarifications in spec.md (2026-11-19) are the design. Research R12 through
R17 records the evidence.

**Usage rows (FR-012, R12, R13).** A `sku` or `region` key must be a Pulumi
input. A `plugin-input` key is a name the mapping's plugin reads. When the
descriptor has the key, it is an ordinary pricing row. When it does not, the
plan emits a usage row: empty `OriginalValue`, editable, `DefaultVisible`, in
mapping order after the present pricing inputs. An empty usage row sends no
override. A non-empty value is an override for a key the descriptor lacks, which
`mergePropertiesWithOverrides` already adds as a string. Identity keys and
hidden keys never become usage rows.

**Which plugin serves the estimate (R14).** `InputMapping.Plugin` is
`aws-public` for every mapping in this feature. `NewEstimateInputContext`
takes the discovery result and the selected pricing-mode id. It keeps the
metric hints, the plugin names on `PricingDiscovery.Modes` with the
`finfocus-plugin-` prefix trimmed, and the selected mode's plugin (empty for
Automatic). `plugin-input` and usage rows apply when the mapping's plugin
equals the selected plugin, or, under Automatic, when that plugin is among the
discovered names. `sku` and `region` rows apply for every plugin. An empty
discovery applies neither `plugin-input` nor usage rows: EC2, EBS, and RDS stay
curated through their SKU key, and S3 and NAT take the fallback.

**Curated (FR-004, FR-012).** `Curated` is true when the type has a mapping and
at least one reviewed row applies: a present `sku` or `region` key, a present
`plugin-input` whose plugin serves, or a usage row whose plugin serves. S3 and
NAT are curated while aws-public serves them, even though the descriptor carries
none of their reviewed keys. They take the fallback note when it does not.

**Overrides that outlive a row (R15, FR-007).** `BuildEstimatePropertyRows`
drops an override the descriptor lacks. The plan adds the usage row when it
applies, and otherwise an `other` row marked overridden, so a value typed in a
usage row stays visible after a mode switch removes that row. The terminal
replans when discovery arrives and when the selected mode changes (R16). The
web page already re-requests on a mode change.

**RDS (R17).** `aws:rds/instance:Instance` is a reviewed mapping: `instanceClass`
(sku), `region` and `availabilityZone` (region), and `engine`, `storageType`,
`allocatedStorage`, `multiAz` (aws-public `plugin-input`). R3's "$0 stub"
exclusion is retired.

**Status of this design.** Usage rows, `InputMapping.Plugin`,
`NewEstimateInputContext`, the RDS mapping, and the terminal mode-switch
replan are implemented by T035–T051. The review pass (T033) fixed two
findings. T030, the UX-only follow-up issue, is waived. The verification
gate (T054) passed. The `initial-30` render
(T052) is recorded.

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
├── estimate_inputs.go          # plan, usage rows, identity rule, context
├── estimate_inputs_mappings.go # reviewed mappings, including RDS
├── estimate_inputs_test.go     # policy tests, including usage and scope
├── estimate_inputs_provenance_test.go
└── estimate.go                 # display calls the plan; SetCurrentValue
internal/webui/
├── handlers_estimate.go        # context from discovery and pricing mode
├── handlers_estimate_test.go
├── static/views/estimate.js    # usage row, toggle, details, note
└── frontendtest/
internal/tui/
├── estimate_model.go           # replan on discovery and mode change
├── estimate_properties.go
└── testdata/ (golden)
docs/src/content/docs/          # covered types, usage rows, fallback
```

**Structure Decision**: The policy sits in `internal/viewmodel`, the package
both transports already share for estimate rows. It must not import `cli`,
`registry`, or `pluginhost`. Plugin-name trimming is a string prefix on the
discovery's reported name, not a registry lookup.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --- | --- | --- |
| Reviewed AWS input mappings live in core (Principle I: core stays provider-agnostic) | Issue #1732 asks for reviewed mappings for supported AWS types, and the editor needs them before any plugin call. Core already holds AWS extraction knowledge for the same reason: `internal/skus/aws.go`, `awsResourceRegion`, and the SDK `ExtractAWSSKU` keys. The table is presentation data. It prices nothing, calls no provider, and a test (`estimate_inputs_provenance_test.go`) proves every `sku` and `region` row against the extraction code. Usage rows name plugin attribute keys the same way. | A plugin-declared input schema is the clean design, but it needs a finfocus-spec change that the issue puts out of scope. Treating every property as a pricing input is the problem the issue reports. Learning inputs from deltas fails on free tiers and combined deltas. Dropping plugin-only keys leaves S3 and NAT uncurated, which the clarification rejected. |

Exit path: when finfocus-spec carries plugin-declared pricing inputs, the mapping
table becomes the fallback for plugins that declare none, and the policy prefers
what the plugin declares. Recorded for `/speckit.revisit` as a deliberate,
temporary exception.
