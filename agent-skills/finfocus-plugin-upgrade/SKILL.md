---
name: finfocus-plugin-upgrade
description: >
  Upgrade an existing FinFocus plugin project to a newer finfocus-spec SDK
  version. Use when a plugin's go.mod requires an older
  github.com/rshade/finfocus-spec, or when a plugin no longer builds after an
  SDK bump. Triggers on: "upgrade plugin", "bump finfocus-spec",
  "migrate plugin", "plugin SDK upgrade", "HandleDryRun ctx",
  "plugin upgrade", or any task updating a FinFocus plugin to a new spec
  version.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus Plugin Upgrade

Move a plugin project to a newer finfocus-spec version. `finfocus plugin
upgrade` makes the edits that need no judgment. This skill covers the rest,
using one migration guide per hop in `references/`.

A **hop** ends at a release that needs plugin-author action. Hops are not
minor versions: v0.5.7, a patch release, changed a handler signature.

## Rules

- Show the plan and get approval before editing anything.
- Work on a new branch from a clean tree. Never stash or discard the user's
  changes yourself; ask them to commit or stash.
- Change only what a guide or a compile error requires. Do not change
  pricing math, adopt optional features, reformat files, or edit `vendor/`.
- Never delete or skip a failing test to get green. Fix the cause or stop
  and report it.
- Do not commit unless the user asks.

## 1. Plan

Run from the plugin's repository root:

```bash
finfocus plugin upgrade --dry-run --output json
```

If `plugin upgrade` is an unknown command, finfocus is too old: ask the user
to update it. The command never uses the network. The default target is the
finfocus-spec version that this finfocus build uses.

Read the JSON:

| Field | Meaning |
| --- | --- |
| `plan.current`, `plan.target` | Version in `go.mod` and the version to move to |
| `plan.up_to_date` | `true`: stop, there is nothing to do |
| `plan.required_go` | `go` directive will be raised to this |
| `plan.hops[]` | Ordered hops: `to`, `summary`, `automatic`, `manual`, `guide` |
| `plan.warnings[]` | Stale `SpecVersion` constants, `replace` directives |

Present the current and target versions, each hop's manual steps, and the
warnings. Ask whether to proceed, and whether to stop at an earlier hop
(`--to`).

Errors stop the skill:

| Error | Action |
| --- | --- |
| `not a FinFocus plugin` | Wrong directory, or `go.mod` lacks the spec module |
| `plugin is too old to upgrade` | Below v0.5.0. Suggest `finfocus plugin init` and porting the pricing code |
| `plugin is newer than this finfocus build` | Update finfocus first |

## 2. Branch and apply

```bash
git switch -c upgrade-finfocus-spec-<target>
finfocus plugin upgrade            # add --to <version> if the user chose one
go mod tidy
```

The command refuses a dirty or non-git tree. Pass `--allow-dirty` only if
the user explicitly accepts mixing the upgrade with other changes. It prints
the changed files. In `go.mod` it changes the finfocus-spec requirement and
the `go` line, and it re-renders the file the way `go mod edit` does. In Go
sources it changes only top-level `SpecVersion` version literals, in files
that import finfocus-spec, and always writes them with the `v` prefix that
`GetPluginInfo` requires.

`--to` accepts only releases this finfocus build knows: the hop versions and
its own. If the user wants a different patch release, bump `go.mod` by hand.

## 3. Work through the hops

`go.mod` now points at the target, so build errors from every hop appear at
once. Use the hops to explain them:

1. For each hop in `plan.hops`, oldest first, read `references/<guide>` and
   do its manual steps.
2. Run `go build ./... && go vet ./...` after each hop. Map each remaining
   error to the guide that explains it.
3. If an error fits no guide, check the finfocus-spec release notes for
   versions between `current` and `target`. If the fix is still unclear, stop
   and report it rather than guessing.

## 4. Verify

```bash
go build ./...
go vet ./...
go test ./...
```

Run the project's own lint target too (`make lint` or `golangci-lint run`)
if it has one. Then smoke-test against finfocus with a throwaway home:

```bash
export FINFOCUS_HOME="$(mktemp -d)"
mkdir -p "$FINFOCUS_HOME/plugins/<name>/0.0.0-upgrade"
go build -o "$FINFOCUS_HOME/plugins/<name>/0.0.0-upgrade/finfocus-plugin-<name>" ./cmd/<entrypoint>
finfocus plugin list --output json
```

`plugin list` starts the plugin and asks it for its info. It exits 0 even
when the plugin fails, so read the JSON rather than the exit code:

- `specVersion` must equal the target. `N/A` means `GetPluginInfo` failed.
- `notes` must be absent. `Failed: …` means the plugin did not start.

`finfocus plugin validate` checks only the binary and manifest on disk. It
does not start the plugin, so it is not a smoke test. If the plugin needs
credentials to start, report the smoke test as skipped and say why.

## 5. Report

Report the versions moved between, files changed, manual steps done for each
hop, verification results, and anything left open. Leave the branch for
review.

## References

| Guide | Hop |
| --- | --- |
| [to-v0.5.7.md](references/to-v0.5.7.md) | `HandleDryRun` takes a context |
| [to-v0.6.0.md](references/to-v0.6.0.md) | Batch validation result type, typed validation errors |
| [to-v0.6.1.md](references/to-v0.6.1.md) | Go 1.27.1 |
| [to-v0.6.2.md](references/to-v0.6.2.md) | `Supports` reaches the host; capability checks |
| [to-v0.7.0.md](references/to-v0.7.0.md) | Additive only |
| [to-v0.7.1.md](references/to-v0.7.1.md) | Additive; scorer request id field deprecated |
| [to-v0.7.2.md](references/to-v0.7.2.md) | Additive; manifest validation; provider means the cloud |
| [to-v0.7.3.md](references/to-v0.7.3.md) | Additive; `ResourceDescriptor.attributes`; 2048-byte tag values |
