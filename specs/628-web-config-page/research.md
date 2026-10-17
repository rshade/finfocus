# Research: Web Config Page

## Decision: Patch the tier file with HuJSON

**Decision**: Writes go through `github.com/tailscale/hujson` `Value.Patch`
(RFC 6902) on the bytes of the chosen tier file. `Config.Save` stays as it
is for callers that already rewrite a struct, and `config set` stops using it.

**Rationale**: `Save` runs `json.MarshalIndent` on the struct from `New()`,
which has already folded environment variables, dropped comments, and filled
every default. Patching the raw file changes only the requested keys.

**Alternatives considered**: Keep `Save` and strip env afterward (cannot
restore comments or distinguish an omitted key from a default). A full
rewrite of `Save` (every current caller depends on the rewrite).

## Decision: Content hash as the version

**Decision**: `baseVersion` is the hex SHA-256 of the tier file's bytes.
A missing file uses the fixed token `absent`.

**Rationale**: Mtime plus size misses same-size edits and depends on
filesystem timestamp resolution. The hash matches the bytes the patch reads.

**Alternatives considered**: Mtime and size, as the issue allowed either.

## Decision: One credential-name rule in `config`

**Decision**: Move the fragment list in `engine.isCredentialKey` to
`config.IsCredentialName`. `engine.isCredentialKey` calls it.
`engine.IsHiddenPropertyKey` still means a `__` prefix or a credential name.
`MaskedForDisplay` and the config view call `IsCredentialName` for plugin
setting names. Notification URLs and headers stay on the existing
`MaskedForDisplay` path, which the view also uses.

**Rationale**: `config` cannot import `engine` (`engine` already imports
`config`). A second fragment list would violate the single-rule note in
`CLAUDE.md`.

**Alternatives considered**: A callback injected from `engine` into `config`
(harder to use from `config get`). Duplicating the list in the web handler.

## Decision: Environment wins in the view

**Decision**: The page's effective value is environment, then the project
file's top-level section, then the global file, then the built-in default.
Environment is applied after the project overlay for this view.

**Rationale**: Issue #1763 requires an environment override to win even when
a file stores another value. `NewWithProjectDir` today calls `New()` (which
applies environment) and then replaces whole sections from the project file,
so a project `output` section drops `FINFOCUS_OUTPUT_*` for that section.
This feature does not change that loader. The page reports the precedence
the issue specifies.

**Alternatives considered**: Display `NewWithProjectDir` unchanged (would
hide environment values the issue's acceptance scenario requires).

## Decision: Scoped budget fields, flat aliases at the command

**Decision**: The page edits `cost.budgets.global.amount`, `currency`, and
`period`. `config set cost.budgets.amount` (and currency and period) still
works and writes those scoped fields. `Config.Set` keeps rejecting
`cost.budgets.global.amount` so existing in-memory callers stay the same.
The file writer accepts the scoped path.

**Rationale**: The flat fields are warned and ignored on load. Writing them
would not change the budget the program uses.

## Decision: Web UI transport stays behind callbacks

**Decision**: `webui.SessionOptions` gains `ConfigView`, `ConfigPreview`, and
`ConfigSave`. `internal/cli/web.go` wires them to the config package and
writes the audit record (keys and tier, never values). `webui` does not
import `cli`. The view document types live in `internal/config` and `webui`
may import `config` for those types.

**Rationale**: Matches `EstimateResources` and the issue's boundary. Handler
tests stub the callbacks and never touch the real home directory.

## Decision: No `Format()` after the patch

**Decision**: After `Patch`, pack the bytes without `Value.Format`.

**Rationale**: `Format` rewrites whitespace for the whole document. The
acceptance criteria require comments, key order, and unknown keys to remain.
`Patch` inserts a new member at the end of its object and replaces a value
in place.

## Decision: Shared path lock

**Decision**: Extract the dismissal lock into `acquirePathLock` and use it
for config writes. The write itself is `CreateTemp` in the same directory,
`Sync`, then `Rename`. The version is checked again inside the lock.

**Rationale**: `Save` uses one fixed `.tmp` name and takes no lock. Two
savers can clobber each other or leave a partial file if the process stops
before `Rename`. `Rename` on the same directory is the crash boundary the
issue asks for.
