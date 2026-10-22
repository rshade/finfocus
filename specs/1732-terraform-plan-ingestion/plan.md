# Implementation Plan: Terraform Plan Ingestion

**Branch**: `1732-terraform-plan-ingestion` | **Date**: 2026-10-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/1732-terraform-plan-ingestion/spec.md`

## Summary

A Terraform directory gets a projected cost from the same commands that
already auto-detect a Pulumi project. FinFocus runs `tofu plan` into a private temp file, reads `tofu show -json`, and parses
it with the shipped `CostParseService` types. This is the first host use of
that contract: `pluginsdk.ValidateParseRequest` and
`ValidateParseResponse` gate the result. The validated response is then
mapped onto the descriptors the engine already prices.
Pulumi detection and preview stay on the current path when a Pulumi project
is the nearer marker. `--terraform-state` stays a state read. The web
dashboard uses the same plan loader for Terraform-compatible projects.

History for a Terraform directory is a hash of the normalized `origin`
remote plus the path relative to the git root, stored with the existing
`<project>@<stack>.history.db` naming. The workspace is included only when
`cost.terraform.include_workspace` or `--include-workspace` is set.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: finfocus-spec v0.7.9 (`pluginsdk.CostParseProvider`, `ValidateParseRequest`, `ValidateParseResponse`, `ResolveResourceTypes`; no bump), Cobra, ax-go
**Storage**: Existing BoltDB cost history. Optional `project_label` meta key. No new store and no schema bump
**Testing**: testify, `go test`, `make test`, `make lint`, `make validate`. Parser fixtures are committed `tofu show -json`. Runner tests use a fake binary
**Target Platform**: Linux, macOS, and Windows CLI
**Project Type**: Single Go CLI and plugin host
**Performance Goals**: After the plan JSON is in memory, parsing stays in process. No extra plugin round trip beyond the projected path Terraform state already uses
**Constraints**: Do not run init, apply, or import. Do not log the plan JSON or store credentials. Do not change Pulumi behavior when Pulumi is the nearer marker. Do not add a Terraform type table. The parser returns `*finfocus.v1.ParseResponse` and must pass the SDK response validator. Unknown names on that response are `unknown_properties`. The projected cache digest changes only after the host copies that list for `GetProjectedCost`
**Scale/Scope**: One shared detector and loader, one plan runner, one parser next to the state parser, and the projection, overview, estimate, recommendations, and history commands that already detect a Pulumi project

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Pricing stays in plugins. Core parses the plan and calls the existing `ResolveResourceTypes` path. No cloud API and no type map in core.
- [x] **Test-Driven Development**: Parser, runner, detector, identity, and CLI tests are part of the implementation. No new TUI layout, so no new golden view. New flags update the MCP tool golden.
- [x] **Cross-Platform Compatibility**: `LookPath`, `os.MkdirTemp`, and `exec.CommandContext`. Unix cancellation kills the plan process group. Windows kills the child process. The plan file is never written into the project on either.
- [x] **Documentation Integrity**: Command reference, config reference, and the overview and forecast command pages describe the new flags and the unchanged state flag. Release notes stay with release-please.
- [x] **Protocol Stability**: No proto change. The host calls the shipped `CostParseService` messages and validators in-process and does not serve the RPC. Two findings stay in this repo: `ParseResponse` has no operation, and the proto comment asks for a Pulumi type token while the validator accepts a Terraform type. Neither finding bumps finfocus-spec in this release.
- [x] **Implementation Completeness**: Detection, runner, parser, hints, identity, and the command list in the spec are all in scope. No stub command and no TODO.
- [x] **Persistence Model**: History remains optional. The new meta key is display text. Commands succeed when history is disabled. The label is not required to open an older database.
- [x] **Web Parity**: The existing web overview and query handlers call the shared Terraform loader. Contracts name the methods; browser validation and a UX-only `web-ui` follow-up are required.
- [x] **Quality Gates**: `make validate`, `make test`, and `make lint` before the work is called done.
- [x] **Multi-Repo Coordination**: No finfocus-spec bump and no plugin release. Plugins that already resolve Terraform state types receive planned resources through that same call.

Post-design re-check: the same gates hold. Using the shipped parse messages does not add a field. The operation map and the `GetProjectedCost` copy of `unknown_properties` sit outside the validated response. The history label does not make a command depend on a previous run.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/1732-terraform-plan-ingestion/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   ├── plan-json.md
│   ├── history-identity.md
│   └── web-ui.md
├── checklists/requirements.md
└── tasks.md                 # /speckit-tasks, not this command
```

### Source Code (repository root)

```text
internal/terraform/            # binary, plan runner, hints, root detection, identity
internal/ingest/terraform_plan.go
internal/ingest/terraform_map.go   # reused, not changed
internal/ingest/testdata/terraform/plan/   # genuine show -json fixture + README
internal/engine/engine.go      # skip terraform:unknownProperties in string tags
internal/cli/project_source.go # shared nearer-marker detector
internal/cli/terraform_projected.go  # plan run, parse, adapter to descriptors
internal/cli/infrastructure_inputs.go  # --terraform-plan, --include-workspace, exclusivity
internal/cli/common_execution.go
internal/cli/cost_projected.go
internal/cli/cost_forecast.go
internal/cli/cost_estimate.go
internal/cli/cost_recommendations.go
internal/cli/cost_actual.go
internal/cli/cost_history.go
internal/cli/cost_history_collect.go
internal/cli/history_record.go
internal/cli/overview.go
internal/cli/root.go
internal/config/budget.go
internal/history/cost_db.go    # optional project_label meta
docs/src/content/docs/reference/cli-commands.md
docs/src/content/docs/reference/config-reference.md
docs/src/content/docs/commands/overview.md
docs/src/content/docs/commands/forecast.md
internal/cli/testdata/mcp/tools.golden
```

**Structure Decision**: `internal/terraform` matches `internal/pulumi` for
process and identity. `internal/ingest` keeps both Terraform JSON parsers so
property conversion stays one function. The CLI calls one detector and one
loader. `internal/cli/esc_source.go` and analyzer install keep calling
`FindProject` and do not learn Terraform.

## Complexity Tracking

FR-034 forbids bundling or importing Terraform core, which is BSL-licensed.
The owner approved these guards so that import cannot return unnoticed:

- `.golangci.yml` `depguard` rule `bsl` denies BSL-licensed HashiCorp
  modules (terraform, vault, nomad, consul, packer, boundary) in every file.
- `make license-check` runs `go-licenses` against an allowlist in the root
  module and each nested plugin module, and `make lint` depends on it.
- `mise.toml` pins `go-licenses` and `opentofu`; the latter produces the
  fixtures and runs the integration tests.

## Design

The behavior is fixed in [research.md](research.md), [data-model.md](data-model.md),
and [contracts/](contracts/). What follows is how that lands in this repo.

### Detection

A new helper used by the loader, overview, bare `finfocus`, history, and
the snapshot recorder:

1. Walk upward for `Pulumi.yaml` or `Pulumi.yml`, using `internal/pulumi.FindProject`.
2. Look only in the working directory for a regular `.tf` or `.tf.json` file.
3. Same directory, Pulumi wins. Terraform files in the working directory and
   a Pulumi file only in a parent: Terraform wins.
4. Neither: a user-input error that names both.

### Runner and parser

`internal/terraform` runs the binary as [contracts/plan-json.md](contracts/plan-json.md)
specifies and returns the JSON bytes plus a typed hint error.
`internal/ingest` implements `pluginsdk.CostParseProvider.Parse` for format
`terraform-plan`. The host validates the request and the response with the
SDK functions. The wrapper also returns `map[source_id]operation` because
`ParseResponse` cannot carry an action. After validation, unknown paths are
copied into `terraform:unknownProperties` for `BuildAttributes` only.
The key is an engine property only (no descriptor field), and
`ConvertToProto` skips it. The attributes cache suffix therefore
changes when the unknown set changes, which is the existing rule that
anything sent to a plugin is part of the projected cache key. Types stay
Terraform types on the response. `ResolveResourceTypes` runs later, as it
does for state.

Two rules sit in the parser, not the engine. Leaves marked in
`before_sensitive` or `after_sensitive` are removed from the attributes and
the old values, so no Terraform-sensitive value reaches a plugin
(`IsPulumiSecret` cannot see Terraform's marker). Providers map as: the four
clouds as is, the nine utility providers (`random`, `null`, `helm`, `time`, `tls`, `local`,
`archive`, `external`, `http`) skipped with an INFO
`resource_skipped` and one stderr note, everything else `custom`. Mapping to
`custom` keeps a plan and a state of the same config in agreement and lets a
future plugin price `cloudflare_*` with no core change.

### Commands

`loadProjectedResources` grows a `--terraform-plan` arm and a Terraform
branch in its auto-detect arm. Forecast already uses that function.
Estimate and recommendations stop requiring `--pulumi-json` when detection
or `--terraform-plan` supplies resources. Dismiss and snooze are unchanged:
their `--pulumi-json` is already optional and only decides whether plugins
open, so they never plan. In `cost_projected.go` the field `planPath` holds
`--pulumi-json`; rename it `pulumiJSONPath` when the new flag lands so the
two cannot be confused.
Overview, for a Terraform directory, skips the Pulumi change-detection
prompt and always plans. Bare `finfocus` delegates to overview for a
Terraform directory the way it does for Pulumi.

`cost actual` does not plan. In a Terraform directory with no file it
returns the state-file hint at exit 2. `cost history collect` returns the
Pulumi-checkpoint hint at exit 2. The other history subcommands resolve the
Terraform identity and do not plan.

### Identity

Hash and label rules are in [contracts/history-identity.md](contracts/history-identity.md).
`project_label` is one optional meta write. Schema version stays 1.
`--include-workspace` is ignored for a Pulumi project so that flag cannot
change a Pulumi history key.

### Docs and goldens

Update the command reference, the config reference, and the overview and
forecast pages. Regenerate `internal/cli/testdata/mcp/tools.golden` because
the new flags are tools. Do not edit `CHANGELOG.md`. Verify existing web hooks and file a UX-only `web-ui` follow-up issue.

### Tests

Write the tests with the behavior, before calling the work done:

- Parser: committed `tofu show -json` covering create, update, delete,
  replace, no-op, a data source, a module address, a null, an unknown
  inside a list, a sensitive leaf in `before` and in `after`, a
  `resource_drift`-only plan, and one resource each from `random`, `null`,
  `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`, and `cloudflare`. Build a `ParseRequest`, call `Parse`, and
  require `pluginsdk.ValidateParseResponse` to accept the response. Assert
  the operation map, the skipped data source, the nine skipped providers,
  the `cloudflare` row with provider `custom`, the sensitive leaf absent from
  attributes and old values and absent from `unknown_properties`, and the
  unknown path with no list index. A hand-built response that fails
  the validator must fail the test.
- Runner: a fake `terraform` or `tofu` on PATH. Assert the arguments, the
  temp file is outside the project and removed on failure and cancel, stdout
  is not logged, and each hint phrase.
- Detector: the nearer-marker cases, including Pulumi beside `.tf` files.
- Identity: two temp git repos with the same origin URL in https and scp
  form, the `unscoped` stack, the workspace opt-in, and the absolute-path
  fallback. Assert the raw remote is not in the label.
- CLI: mutual exclusion exits 2. A Pulumi fixture does not invoke the
  Terraform runner. `cost actual` and `cost history collect` in a Terraform
  directory do not plan. `recommendations dismiss` and `snooze` never plan.
  `overview --terraform-plan` reads the file and starts no binary. A
  `--terraform-plan` file over 32 MiB exits 2.

Tests that write a binary and execute it are not parallel
(`//nolint:paralleltest` with that reason). Parser and identity tests call
`t.Parallel`.
