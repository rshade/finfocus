# Research: Plugin upgrades across finfocus-spec versions

Answers to the research questions in issue #270. Evidence comes from the
finfocus-spec repository (tags, `CHANGELOG.md`, `MIGRATION.md`,
`llm-migration.json`, signature diffs between tags) and from core.

## Decision

Build both, split by who is reliable at each part:

- `finfocus plugin upgrade` detects the version, plans hops, and applies the
  edits that need no judgment.
- The `finfocus-plugin-upgrade` agent skill runs the command and handles the
  manual steps, which need reading the plugin's code.

The command alone cannot fix a changed handler signature in someone else's
code. The skill alone would redo deterministic edits by hand on every run,
and its hop list would drift from the code. Sharing one hop table, with a
test that ties it to the guides and to core's spec version, keeps them in
step.

## Version detection (Q1)

| Source | Reliability | Use |
| --- | --- | --- |
| `go.mod` require line | Authoritative: it is what compiles | Planning |
| A `SpecVersion = "…"` constant in plugin code | Hand-maintained, often stale (`plugin init` writes one) | Updated, and reported when it disagrees or lacks the `v` prefix |
| `pluginsdk.SpecVersion` in the SDK | Release-managed since v0.5.0 | Core's target only |
| Manifest `spec_version` | A manifest schema version (`1.0`), not the SDK version | Ignored |

`go.mod` is parsed with `golang.org/x/mod/modfile`, which also rewrites it
without disturbing comments or formatting.

## Hops

A hop ends at a release that needs plugin-author action. Pre-1.0, patch
releases break too: v0.5.7 changed `DryRunHandler`.

| Hop to | Minimum Go | Automatic | Manual |
| --- | --- | --- | --- |
| v0.5.7 | 1.25.7 | — | `HandleDryRun(req)` → `HandleDryRun(ctx, req)` |
| v0.6.0 | — | — | `ValidateBatchCostRequest` returns `BatchCostValidationResult`; validation errors are `ValidationError` sentinels; oversized `ResourceDescriptor` fields are rejected |
| v0.6.1 | 1.27.1 | `go` directive | None required; `pluginsdk.Run` is an optional new entry point |
| v0.6.2 | — | — | A default `Supports` now reaches hosts as `Supported:false`; implement `SupportsProvider` to be routed. Hand-set `Capabilities` are checked against inferred ones |
| v0.7.0 | — | — | None; additive only |
| v0.7.1 | — | — | Scorer plugins only: `ScorerInfo.provider_request_id` is deprecated for `provider_request_ids` |

Sources: finfocus-spec tags and `go.mod` per tag; commits `15addc9` (DryRun ctx, ancestor of v0.5.7), `3c09962` (batch validation),
`b9f841d` (Go 1.27.1), `e51f8b4` (capabilities); `sdk/go/pluginsdk/env.go`;
`MIGRATION.md`. The finfocus-spec CHANGELOG lists the DryRun break under
0.6.0, but the code shipped in v0.5.7; the hop table follows the code.

The oldest supported version is v0.5.0, the first release under the
finfocus-spec module path. Nothing older is in use: every known plugin
(aws-public, kubernetes, jev) is on v0.7.0, and a pre-v0.5.0 plugin cannot
talk to a current host because the proto package name changed. Supporting
it would add an import-rewrite path for no users.

### `SpecVersion` needs the `v` prefix

Since v0.4.12, `GetPluginInfo` rejects a spec version that is not
`vMAJOR.MINOR.PATCH` (`pluginsdk.ValidateSpecVersion`). `plugin init` writes
`SpecVersion = "0.6.1"`, so a freshly scaffolded plugin returns `Internal`
from `GetPluginInfo`, and `plugin list` shows its spec version as `N/A`. This
was found while verifying this feature end to end. The upgrade always writes
the `v` form and warns when it replaces a bare one. Fixing the template
itself belongs to #248.

## Migration strategy (Q2)

| Option | Verdict |
| --- | --- |
| `go/ast` rewriting | Rejected for signatures: a handler change needs the body updated too, and the safe fix depends on the plugin |
| `modfile` edits | Used for `go.mod` |
| Agent with per-hop guides | Used for everything that needs judgment |

`SpecVersion` literals are found with `go/parser` and replaced by byte span,
so formatting and every other string stay as they were. Only top-level
version literals in non-test files that import finfocus-spec count; an
unrelated `SpecVersion` (an OpenAPI version, a pinned test value) is not the
plugin's.

## Command design (Q4)

`finfocus plugin upgrade`, a separate command, not `plugin init --upgrade`:
`init` creates a project, `upgrade` edits one, and they share no flags.
`plugin update` already exists for *installed* plugin binaries; the help text
says so to avoid confusion.

## User experience (Q5)

- The global `--dry-run` prints the plan and writes nothing.
- Apply refuses a dirty or non-git tree unless `--allow-dirty`, so rollback
  is `git checkout .` and the upgrade is one reviewable diff. No backup files.
- No interactive prompt. The skill asks for approval, and the dry run is the
  preview.
- `go mod tidy`, build, and test are printed as next steps, not run: they
  need the network and the author's toolchain.

## Guide storage and drift (Prompt architecture)

- Guides live in `agent-skills/finfocus-plugin-upgrade/references/`, one per
  hop, named `to-vX.Y.Z.md`. They are not embedded in the binary: the
  command prints the guide name, and the skill reads the file.
- A unit test fails when a hop has no guide, when a guide has no hop, or
  when the newest hop is below core's `pluginsdk.SpecVersion`. Bumping
  finfocus-spec in core therefore forces a hop and a guide in the same PR,
  even for an additive release.
- Guides are written by hand from the release diff. finfocus-spec's own
  migration aids (`MIGRATION.md`, `llm-migration.json`) cover only the
  pre-v0.5.0 module rename, so there is nothing to generate from.
- Making finfocus-spec PRs carry an upgrade note is a finfocus-spec process
  change, outside this repository.

## Risks

| Risk | Mitigation |
| --- | --- |
| Hop table misses a break | Guides end with build and test; the skill fixes from compiler errors |
| Core's compatibility check compares major versions only, so any 0.x plugin passes with any 0.x core | Recorded here; tightening it is a separate change |
| Skill edits pricing logic | The skill forbids it and limits edits to what a guide or compile error requires |
| `replace` directives to local checkouts | Left alone and reported |
