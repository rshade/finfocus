# Implementation Plan: Analyzer Language Plugin

**Branch**: `1734-analyzer-language-plugin` | **Date**: 2026-10-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1734-analyzer-language-plugin/spec.md`

## Summary

Pulumi 3.270.0 starts every directory policy pack through a
`pulumi-language-<runtime>` plugin and no longer falls back to
`pulumi-analyzer-policy-<runtime>`, so FinFocus analyzer mode fails (#1895).
The FinFocus binary gains a third start-up name, `pulumi-language-finfocus`,
under which it serves Pulumi's `LanguageRuntime` gRPC service. Its
`RunPlugin` re-executes the same binary as the analyzer, with an explicit
`Args[0]` so that a symlinked entry cannot fall into CLI mode, in the policy
pack directory, streaming its output back unchanged and stopping it
gracefully on cancel. `finfocus analyzer install` writes the new entry beside
the legacy one (and fixes the missing `.exe` on Windows). The nightly E2E
setup, an opt-in integration test with a Pulumi version matrix, and the docs
move to the new layout. A spike on 2026-10-22 showed this design turns
Pulumi 3.271.0 from failing to passing while 3.268.0 keeps passing.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)
**Primary Dependencies**: `github.com/pulumi/pulumi/sdk/v3` v3.270.0
(`proto/go` `LanguageRuntime` types only), `google.golang.org/grpc`, Cobra,
zerolog, testify. No new module; `sdk/.../rpcutil` is avoided (R10).
**Storage**: N/A (files in the policy pack directory only)
**Testing**: testify unit tests with an in-process gRPC server (`bufconn`) and
the helper-process pattern for the child; the opt-in integration test
`test/integration/analyzer_policy_pack_test.go` against real Pulumi binaries;
the nightly analyzer E2E.
**Target Platform**: Linux and macOS (amd64, arm64), Windows (amd64)
**Project Type**: single Go CLI
**Performance Goals**: no noticeable change in preview time. In the spike the
empty-project test took 2.07s on Pulumi 3.268.0 through the legacy entry and
2.02s through the language host; analysis itself is untouched.
**Constraints**: only the port on standard output; analyzer stopped within
Pulumi's 5-second grace period (3-second `WaitDelay`, language host shutdown
capped at 4 seconds); no change to where analyzer mode reads config or writes
summaries and history (FR-009); graceful plugin cleanup on Linux and macOS
only (FR-007).
**Scale/Scope**: about 8 Go files touched or added, 7 docs and 2 agent-skill
files, 2 workflows.

No `NEEDS CLARIFICATION` remains: the working-directory question was resolved
in the spec (option C), and the rest is settled in [research.md](research.md).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Host-side integration with Pulumi's
  plugin model; no FinFocus cost plugin, provider knowledge, or
  `finfocus-spec` change involved.
- [x] **Test-Driven Development**: The break-check test already exists and
  fails on `main` with Pulumi 3.271.0. Unit tests for the language host, the
  pack layout, and the start-up dispatch are written first. Coverage is
  measured, not assumed (T011, T033): at least 80% for each changed package,
  and 95% for the critical paths the constitution names, which here are the
  CLI entry points (`cli.RunAnalyzerLanguageHost`, `startModeFor`) and the
  plugin communication (`LanguageHost.RunPlugin`, `SetupPolicyPack`).
- [x] **Cross-Platform Compatibility**: Signal handling is split into
  `languagehost_unix.go` / `languagehost_windows.go` (documented, R5); entry
  names go through one suffix-aware function, which also fixes the existing
  Windows `.exe` gap (R8). The nightly `windows-latest` unit job covers it.
- [x] **Documentation Integrity**: Seven docs and the analyzer-setup agent
  skill (with its `.skill` archive) are updated in the same change (R11).
  `CHANGELOG.md` is left to release-please.
- [x] **Protocol Stability**: No FinFocus protocol change. The feature
  implements Pulumi's existing `LanguageRuntime` service and keeps the legacy
  entry for older Pulumi releases.
- [x] **Implementation Completeness**: No stubs or TODOs. The RPCs left to
  `UnimplementedLanguageRuntimeServer` are program-hosting RPCs that do not
  apply to a policy pack runtime; `Unimplemented` is the protocol's answer
  (R2), and the same set `pulumi-policy-opa` leaves out.
- [x] **Persistence Model**: No new persistent state.
- [x] **Web Parity**: Spec records `Not applicable` with a reason; Principle
  VII names `analyzer serve` as an exempt example. No hook, render check, or
  `web-ui` issue.
- [x] **Quality Gates**: `make lint`, `make test`, the integration test on both
  Pulumi versions, and `make validate` before completion.
- [x] **Multi-Repo Coordination**: `finfocus-action` (FR-013 follow-up issue)
  and `finfocus-demo` (remove the 3.268.0 pin after release, SC-003) are
  listed in the spec's Dependencies. No `finfocus-spec` or plugin change.

**Violations Requiring Justification**: None.

**Post-design re-check (after Phase 1)**: Still passes. The contracts add one
JSON field (additive) and no command or flag, so the MCP golden tool list is
unchanged.

## Project Structure

### Documentation (this feature)

```text
specs/1734-analyzer-language-plugin/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── language-host.md
│   └── cli-analyzer-install.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks, not created here
```

### Source Code (repository root)

```text
cmd/finfocus/
└── main.go                         # third start-up name -> cli.RunAnalyzerLanguageHost

internal/analyzer/
├── languagehost.go                 # LanguageHost: LanguageRuntimeServer, RunPlugin, stream writers
├── languagehost_unix.go            # child Cancel = SIGINT; host signal set (//go:build unix)
├── languagehost_windows.go         # child Cancel = Kill (//go:build windows)
├── languagehost_test.go            # bufconn server, helper-process child, Args[0], exit codes, cancel
├── policypack.go                   # entry list, LanguagePluginBinaryName, PolicyPackEntryPath, SetupPolicyPack
├── policypack_test.go              # both entries, idempotence, .exe names (runs natively on windows-latest)
├── install.go                      # InstallResult.PolicyPackLanguagePlugin; sync via SetupPolicyPack
└── install_test.go

internal/cli/
├── analyzer_language.go            # RunAnalyzerLanguageHost: logging, listen, print port, serve until signal
├── analyzer_language_test.go
├── analyzer_install.go             # "Language plugin:" line
└── analyzer_install_test.go

test/integration/
└── analyzer_policy_pack_test.go    # language plugin, legacy-only (<3.270), plugin cleanup subtests

test/e2e/
└── analyzer_e2e_test.go            # ensureAnalyzerConfig writes both entries

.github/workflows/
├── ci.yml                          # analyzer-policy-pack job, matrix ubuntu/macos × pulumi 3.268.0/latest
└── nightly.yml                     # same test with latest

docs/src/content/docs/
├── getting-started/analyzer-setup.md
├── analyzer-integration.md
├── architecture/analyzer.md
├── deployment/deployment.md
├── guides/routing.md
└── support/faq.md

agent-skills/finfocus-analyzer-setup/
├── SKILL.md
├── references/troubleshooting.md
└── finfocus-analyzer-setup.skill   # rebuilt archive (SKILL.md + references/)
```

**Structure Decision**: Single Go project. The language host lives in
`internal/analyzer` next to the analyzer server it starts, and its process
entry point in `internal/cli` next to `RunAnalyzerServe`, which keeps the
existing split (gRPC servers in `analyzer`, process lifecycle and logging in
`cli`). `main.go` stays a dispatcher.

## Implementation Order

Matches the phases in [tasks.md](tasks.md).

1. Baseline: record the break check on `main` (T001).
2. Pack layout: entry list, suffix-aware paths, both entries in
   `SetupPolicyPack`, tests first including the Windows names (Foundational,
   T002-T003).
3. Language host server and `RunPlugin` with injected executable, process
   entry, `main.go` dispatch, signal handling per platform (US1, T004-T011).
   This is the MVP.
4. `InstallResult` field, install output, `--force` path through
   `SetupPolicyPack` (US2, T012-T015).
5. Integration subtests, E2E pack setup, CI and nightly jobs (US3,
   T016-T020).
6. Docs, FAQ entry, agent skill and archive (US4, T021-T028).
7. `CLAUDE.md`, follow-up issue drafts for `finfocus-action` (FR-013) and the
   analyzer project directory (FR-014), demo and Windows checks, final gates
   (T029-T033).

## Risks

| Risk | Mitigation |
| --- | --- |
| Pulumi changes `RunPlugin` request fields again | The integration test runs against `latest` on every PR and nightly |
| Windows behavior is only unit-tested | Native `windows-latest` unit job; one recorded manual preview (T032) |
| Windows kill skips plugin cleanup | Scoped out in the spec (FR-007, SC-005); same as Pulumi's forcible path |
| Analyzer cleanup races Pulumi's 5-second kill | 3-second `WaitDelay`, 4-second host shutdown cap; the plugin cleanup subtest checks real PIDs on Linux and macOS |
| Users never re-run `analyzer install` | FAQ quotes the exact error and the one-line fix; release note via the `fix` commit |

## Complexity Tracking

No constitution violations; nothing to justify.
