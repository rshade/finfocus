# Implementation Plan: Document Routing Limits in Analyzer Mode

**Branch**: `598-analyzer-routing-docs` | **Date**: 2026-02-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/598-analyzer-routing-docs/spec.md`

## Summary

Add documentation clarifying that routing configuration is not applied during
`pulumi preview --policy-pack` (analyzer/policy-pack mode) and that global plugins
cannot be excluded via routing. Provide an actionable `FINFOCUS_HOME`-based plugin
isolation procedure for users who need to control which plugins run in analyzer mode.

Two existing files are modified; no new files are created:

1. `docs/guides/routing.md` — add a `> **Note:**` callout after "Common Configuration
   Patterns", before "Validation", covering the three facts: no routing in analyzer mode,
   global plugins always fire, and pointer to isolation procedure.
2. `docs/analyzer-integration.md` — add `## Isolating Plugins in Analyzer Mode` section
   before `## See Also`, with step-by-step `FINFOCUS_HOME` procedure, file-level symlink
   instructions, issue #750 warning, and inline bash example.

## Technical Context

**Language/Version**: Markdown (documentation-only; no Go code changes)
**Primary Dependencies**: markdownlint-cli2 v0.18.1 (already installed)
**Storage**: N/A
**Testing**: `make docs-lint` (markdownlint validation)
**Target Platform**: GitHub Pages / Jekyll static site
**Performance Goals**: N/A
**Constraints**: No new files; both modified files must pass `make docs-lint` with zero errors
**Scale/Scope**: 2 markdown files, ~50–70 lines added total

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A — documentation-only; no code changes.
- [x] **Test-Driven Development**: N/A — validation is `make docs-lint` (markdownlint).
  Linting must pass before claiming tasks complete.
- [x] **Cross-Platform Compatibility**: N/A — documentation is platform-agnostic.
- [x] **Documentation Integrity**: YES — this feature IS a documentation integrity
  improvement. Both modified files must remain synchronized with the confirmed behavior
  described in the issue (finfocus v0.3.1, Pulumi CLI v3.223.0).
- [x] **Protocol Stability**: N/A — no protocol changes.
- [x] **Implementation Completeness**: All documentation content must be fully written.
  No `TODO` markers or placeholder text may remain in the committed changes.
- [x] **Quality Gates**: `make docs-lint` must pass with zero errors after changes.
- [x] **Multi-Repo Coordination**: N/A — documentation only affects this repository.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/598-analyzer-routing-docs/
├── plan.md              # This file
├── research.md          # Phase 0 output (complete)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Files Modified

```text
docs/
├── guides/
│   └── routing.md           # ADD: callout after "Common Configuration Patterns"
└── analyzer-integration.md  # ADD: "Isolating Plugins in Analyzer Mode" section
```

No new files. No new directories. No source code changes.

## Phase 0: Research

All NEEDS CLARIFICATION items resolved. See `research.md` for full decision log.

**Summary of resolved decisions**:

| # | Decision | Resolution |
|---|----------|------------|
| 1 | Callout format in routing.md | `> **Note:**` blockquote; no heading, no ToC update |
| 2 | Callout placement in routing.md | After line 330 (end of "Common Configuration Patterns"), before "## Validation" |
| 3 | New section placement in analyzer-integration.md | Before `## See Also` (line 84) |
| 4 | Issue #750 URL | `https://github.com/rshade/finfocus/issues/750` |
| 5 | FR-007 shell example | Inline `bash` fenced code block; no new script file |
| 6 | Symlink warning label | `> **Warning:**` (distinct from `> **Note:**` callout) |

## Phase 1: Design

### routing.md Callout Content

Insert after line 330 (before `## Validation`):

```markdown

> **Note: Routing does not apply in analyzer/policy-pack mode**
>
> When finfocus runs as a Pulumi policy pack (`pulumi preview --policy-pack`),
> routing configuration is **not consulted**. All plugins discovered in the plugin
> directory are loaded and queried for every resource.
>
> Additionally, global plugins (those with `supported_providers: ["*"]`) cannot be
> excluded via routing even in standard CLI mode — they are always included for all
> resources.
>
> To exclude specific plugins when running as a policy pack, use `FINFOCUS_HOME`
> isolation. See [Isolating plugins in analyzer mode](../analyzer-integration.md#isolating-plugins-in-analyzer-mode).
```

### analyzer-integration.md New Section Content

Insert before `## See Also` (before line 84):

```markdown
## Isolating Plugins in Analyzer Mode

When finfocus runs as a Pulumi policy pack, routing configuration is not applied.
The only supported mechanism for controlling which plugins are loaded is `FINFOCUS_HOME`.

When `FINFOCUS_HOME` is set, finfocus uses `$FINFOCUS_HOME/plugins/` as its entire
plugin root. Any plugin absent from that directory is never loaded.

> **Warning: Directory-level symlinks are not supported** (issue [#750](https://github.com/rshade/finfocus/issues/750))
>
> The registry silently skips directory-level symlinks. If you symlink the entire
> plugin version directory, the plugin will not be found. Use file-level symlinks
> (symlink the binary itself) until issue #750 is resolved.

### Step-by-step: create an isolated plugin home

**Step 1**: Create real plugin directories (not symlinks):

\```bash
mkdir -p ~/.finfocus/demo/plugins/aws-public/v0.1.5
\```

**Step 2**: Symlink only the plugin binary (file-level):

\```bash
ln -sf ~/.finfocus/plugins/aws-public/v0.1.5/finfocus-plugin-aws-public-us-east-1 \
    ~/.finfocus/demo/plugins/aws-public/v0.1.5/finfocus-plugin-aws-public-us-east-1
\```

Repeat Step 1 and Step 2 for each plugin you want to include.

**Step 3**: Run `pulumi preview` with the isolated home:

\```bash
FINFOCUS_HOME=~/.finfocus/demo \
  pulumi preview --policy-pack /path/to/finfocus-policy-pack
\```

Only the plugins present in `~/.finfocus/demo/plugins/` will be loaded. If the
directory contains no plugins, finfocus returns zero cost data (it does **not**
fall back to `~/.finfocus`).

### Environment variable precedence

`FINFOCUS_HOME` takes highest precedence over `PULUMI_HOME/finfocus` and `~/.finfocus`.
See [Configuration Reference](reference/config-schema.md) for the full precedence order.
```

### No data-model.md, contracts/, or quickstart.md

This feature has no data entities, API endpoints, or new CLI commands. These Phase 1
artifacts are not applicable and are omitted.

## Phase 2: Tasks

See `tasks.md` (generated by `/speckit.tasks`).

## Implementation Order

1. Edit `docs/guides/routing.md` — insert callout (self-contained, no dependencies)
2. Edit `docs/analyzer-integration.md` — insert new section (self-contained, no dependencies)
3. Run `make docs-lint` — verify zero errors on both files
4. Run `make lint` — confirm no regressions in other linting passes
