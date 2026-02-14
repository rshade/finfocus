# Implementation Plan: CodeRabbit Cleanup

**Branch**: `589-coderabbit-cleanup` | **Date**: 2026-02-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/589-coderabbit-cleanup/spec.md`

## Summary

Resolve follow-up items from CodeRabbit review on the Pulumi auto-detect PR (#509). Changes span six groups: doc comment deduplication (11 functions), structured logging consistency (4 locations), code correctness fixes (7 items), test reliability improvements (6 tests), architecture refactor of global Runner to struct injection, and CI tooling formatting. All changes are localized edits within existing files with no new dependencies or API surface changes.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Cobra v1.10.2 (CLI), zerolog v1.34.0 (logging), testify v1.11.1 (testing)
**Storage**: N/A (no storage changes)
**Testing**: `go test` with testify assertions, `make test` and `make lint`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go project (CLI tool)
**Performance Goals**: N/A (cleanup, no performance-impacting changes)
**Constraints**: No new dependencies; maintain 80%+ test coverage; all changes backward-compatible
**Scale/Scope**: ~15 files modified across internal/cli, internal/proto, internal/pulumi, internal/skus, internal/ingest, internal/registry, docs/

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: N/A - cleanup within core orchestration layer, no provider-specific logic added
- [x] **Test-Driven Development**: Tests planned for all correctness changes (Group 3); test reliability improvements in Group 4; 80%+ coverage maintained
- [x] **Cross-Platform Compatibility**: No platform-specific changes; all edits are in portable Go code
- [x] **Documentation Integrity**: Group 1 fixes doc comments; Group 6 handles Prettier formatting for docs/
- [x] **Protocol Stability**: No protocol buffer changes
- [x] **Implementation Completeness**: All changes are complete edits; no stubs or TODOs
- [x] **Quality Gates**: `make lint` and `make test` required before completion
- [x] **Multi-Repo Coordination**: N/A - all changes within finfocus-core repository only

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/589-coderabbit-cleanup/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research findings
├── data-model.md        # Phase 1 output (minimal for cleanup)
├── quickstart.md        # Phase 1 implementation guide
├── checklists/
│   └── requirements.md  # Specification quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── common_execution.go      # Group 2: logging fields; Group 3c: audit entry
│   ├── cost_actual.go            # Group 3a: conditional; Group 3b: error handling
│   ├── cost_actual_test.go       # Group 4a: test directory isolation
│   ├── cost_projected.go         # Group 3c: audit entry; Group 3d: flag documentation
│   └── plugin_install.go         # Group 3e: empty key validation
├── engine/
│   └── engine.go                 # Group 2: logging operation field
├── ingest/
│   ├── pulumi_plan.go            # Group 2: logging operation field
│   └── state.go                  # Group 1: MapStateResource doc comment
├── proto/
│   ├── adapter.go                # Group 1: resolveSKUAndRegion, enrichTagsWithSKUAndRegion, toStringMap
│   └── adapter_test.go           # Group 1: mock comment correction
├── pulumi/
│   ├── errors.go                 # Group 3g: NotFoundError message
│   ├── pulumi.go                 # Group 1: doc comments; Group 5: PulumiClient struct
│   └── pulumi_test.go            # Group 4b: context cancellation; Group 5: test updates
├── registry/
│   └── registry.go               # Group 3f: region enrichment
└── skus/
    └── aws.go                    # Group 1: resolveAWSSKU, extractPulumiSegment

docs/
└── guides/
    └── routing.md                # Group 6: Prettier formatting
```

**Structure Decision**: Existing single-project Go layout. No new directories or files created. All changes modify existing files in-place.

## Implementation Groups

### Group 1: Doc Comment Cleanup (P1)

**Scope**: 11 functions across 5 files
**Approach**: For each function, collapse duplicated comment blocks into a single concise Go-style doc comment. Follow the pattern: `// FunctionName does X. It returns Y.`

| File | Function | Action |
|------|----------|--------|
| `internal/proto/adapter.go:707` | `resolveSKUAndRegion` | Merge two paragraphs into one |
| `internal/proto/adapter.go:871` | `enrichTagsWithSKUAndRegion` | Remove initial fragment, keep detailed block |
| `internal/proto/adapter.go:856` | `toStringMap` | Remove repeated first line |
| `internal/pulumi/pulumi.go:69` | `FindBinary` | Collapse to single sentence |
| `internal/pulumi/pulumi.go:80` | `FindProject` | Remove duplicated description block |
| `internal/pulumi/pulumi.go:112` | `GetCurrentStack` | Collapse to concise comment |
| `internal/pulumi/pulumi.go:224` | `Preview` | Remove one-liner, keep detailed block |
| `internal/pulumi/pulumi.go:242` | `StackExport` | Remove one-liner, keep detailed block |
| `internal/skus/aws.go:22` | `resolveAWSSKU` | Remove stuttered prefix |
| `internal/skus/aws.go:36` | `extractPulumiSegment` | Remove shorter stub, keep full comment |
| `internal/ingest/state.go:168` | `MapStateResource` | Complete truncated sentence about input precedence |
| `internal/proto/adapter_test.go:2990` | `mockPbcCostSourceServiceClient` | Change "the rest panic" to "the rest return empty success responses" |

### Group 2: Structured Logging Consistency (P2)

**Scope**: 4 log call sites across 3 files
**Approach**: Add missing `Str("component", ...)` and/or `Str("operation", ...)` fields.

| File | Line(s) | Current | Add |
|------|---------|---------|-----|
| `internal/cli/common_execution.go:215` | Debug log for project_dir | No component/operation | `Str("component", "pulumi")`, `Str("operation", "detect_project")` |
| `internal/engine/engine.go:1093-1098` | Debug log for state-based estimation | Has `component` | `Str("operation", "actual_cost_fallback")` |
| `internal/engine/engine.go:1113-1117` | Warn log for no actual cost | Has `component` | `Str("operation", "actual_cost_fallback")` |
| `internal/ingest/pulumi_plan.go:73-78` | Error log for parse failure | Has `component` | `Str("operation", "parse_plan")` |

### Group 3: Code Quality & Correctness (P1)

**3a. Simplify conditional** (`cost_actual.go:565-566`)

- Current: `params.statePath != "" || (params.planPath == "" && params.statePath == "")`
- Simplified: `params.planPath == ""`
- Verified: mutual exclusivity enforced by `validateActualInputFlags`

**3b. Handle GetString error** (`cost_actual.go:538-539`)

- Current: `stackFlag, _ := cmd.Flags().GetString("stack")`
- Fix: Capture error and return it

**3c. Add audit entry on auto-detect failure** (`cost_projected.go:149-161`)

- Current: `resolveResourcesFromPulumi` failure returns error without audit entry
- Fix: Call `audit.logFailure(ctx, err)` before returning, matching the `loadAndMapResources` pattern

**3d. Document flag optionality** (`cost_projected.go:81`)

- Add inline comment explaining `--pulumi-json` is intentionally optional for auto-detection

**3e. Validate empty metadata keys** (`plugin_install.go:470-488`)

- After `strings.TrimSpace(parts[0])`, check if key is empty
- If empty, append warning "ignored metadata entry %q: empty key" and `continue`

**3f. Fix registry region enrichment** (`registry.go:72-77`)

- Current: Region enrichment only when `meta == nil`
- Fix: Also check `if _, ok := meta["region"]; !ok` when meta is non-nil

**3g. Update NotFoundError message** (`errors.go:35-38`)

- Current: `"...provide --pulumi-json"`
- Fix: `"...provide --pulumi-json or --pulumi-state"`

### Group 4: Test Improvements (P2)

**4a. Directory isolation for auto-detection tests** (`cost_actual_test.go`)

- Tests affected: T023 (line 25), "neither" (line 257), `TestCostActualWithoutInputFlags` (line 401), `TestStackFlagExistsOnActual` (line 438)
- Pattern: `oldwd, _ := os.Getwd()` / `os.Chdir(t.TempDir())` / `t.Cleanup(func() { os.Chdir(oldwd) })`
- Use `require.NoError` for `Getwd` and `Chdir` calls

**4b. Deterministic context cancellation** (`pulumi_test.go`)

- Tests: `TestPreview_ContextCancellation` (line 313), `TestStackExport_ContextCancellation` (line 397)
- Current: `time.Sleep(60 * time.Millisecond)` after 50ms timeout context
- Fix: Call `cancel()` explicitly before invoking function under test; remove `time.Sleep`

### Group 5: Architecture — PulumiClient Struct (P3)

**Scope**: Replace global `Runner` variable with `PulumiClient` struct
**Impact analysis**:

| Call Site | Function | File |
|-----------|----------|------|
| Runtime | `Runner.Run()` | `pulumi.go:127` (GetCurrentStack), `pulumi.go:202` (runPulumiCommand) |
| Test injection | `Runner = mock` | `pulumi_test.go:39-41` |
| Package functions | `FindBinary`, `FindProject`, `GetCurrentStack`, `Preview`, `StackExport` | `common_execution.go:207-284`, `overview.go:228,266` |
| Integration tests | Same functions | `test/integration/pulumi_auto_test.go` |

**Design**:

```go
type PulumiClient struct {
    runner CommandRunner
}

func NewClient() *PulumiClient {
    return &PulumiClient{runner: &execRunner{}}
}

func NewClientWithRunner(r CommandRunner) *PulumiClient {
    return &PulumiClient{runner: r}
}
```

- Convert `FindBinary`, `FindProject` to remain package-level (no Runner dependency)
- Convert `GetCurrentStack`, `Preview`, `StackExport` to methods on `PulumiClient`
- Update `detectPulumiProject` and `resolveResourcesFromPulumi` in `common_execution.go` to accept/create `PulumiClient`
- Update `overview.go` call sites similarly

**Note**: This group may be deferred to a separate PR if the blast radius proves too large for a cleanup branch.

### Group 6: CI/Tooling (P3)

**Scope**: `docs/guides/routing.md`
**Action**: Run `npx prettier --write docs/guides/routing.md` and verify no CI errors remain

## Execution Order

1. **Group 1** (doc comments) - No dependencies, pure comment edits
2. **Group 3** (code quality) - Independent correctness fixes
3. **Group 2** (logging) - Independent logging additions
4. **Group 4** (tests) - After Group 3 since some test changes validate correctness fixes
5. **Group 6** (prettier) - Independent
6. **Group 5** (PulumiClient refactor) - Last due to largest blast radius; may be deferred
