# Research: Split Project-Local and User-Global Configuration

**Feature**: 591-config-split | **Date**: 2026-02-14

## Research Tasks & Findings

### R1: Current Config Resolution Architecture

**Question**: How does FinFocus currently resolve its configuration directory?

**Finding**: `internal/config/config.go:157-179` implements `ResolveConfigDir()` with
a 4-level precedence chain:

1. `$FINFOCUS_HOME` (explicit override)
2. `$PULUMI_HOME/finfocus/` (Pulumi ecosystem integration)
3. `$HOME/.finfocus/` (standard default)
4. `$CWD/.finfocus` (last-resort fallback)

All global resources (plugins, cache, logs, config) resolve from this single directory.

**Decision**: Preserve `ResolveConfigDir()` as the **global** resource resolver.
Add a new `ResolveProjectConfigDir()` for project-specific resources.

**Rationale**: Minimal disruption to existing code. Global resources (plugins, cache,
logs) continue using `ResolveConfigDir()`. Only config and dismissals get project-local
overrides.

**Alternatives Considered**:

- Modify `ResolveConfigDir()` to return both paths: Rejected because it changes the
  return signature, breaking all callers.
- Single `ResolveConfigDir()` with mode parameter: Rejected as it conflates two
  distinct resolution semantics.

---

### R2: Dismissal Store Hardcoded Path Bug

**Question**: Why doesn't the dismissal store respect `FINFOCUS_HOME`?

**Finding**: `internal/config/dismissed.go:93-100` hardcodes `os.UserHomeDir()`:

```go
func NewDismissalStore(filePath string) (*DismissalStore, error) {
    if filePath == "" {
        homeDir, err := os.UserHomeDir()
        // ...
        filePath = filepath.Join(homeDir, ".finfocus", "dismissed.json")
    }
    // ...
}
```

The `loadDismissalStore()` helper in `internal/cli/cost_recommendations_dismiss.go:273`
calls `config.NewDismissalStore("")`, always hitting this bug.

**Decision**: Fix `NewDismissalStore("")` to use a new resolution function that:

1. Checks project-local `.finfocus/dismissed.json` first (if in Pulumi project)
2. Falls back to `ResolveConfigDir() + "/dismissed.json"`

**Rationale**: Using `ResolveConfigDir()` fixes the `FINFOCUS_HOME` bug. Adding
project-local awareness solves the cross-project dismissal leakage (User Story 2).

**Alternatives Considered**:

- Pass explicit path from CLI: Rejected because it pushes project detection logic
  into every CLI command instead of centralizing it.

---

### R3: Existing Pulumi Project Detection

**Question**: Can we reuse `pulumi.FindProject()` for config resolution?

**Finding**: `internal/pulumi/pulumi.go:88-110` already implements walk-up search for
`Pulumi.yaml`/`Pulumi.yml`. It:

- Resolves to absolute path first
- Walks up to filesystem root
- Returns `ErrNoProject` when not found
- Already tested and used by auto-detect in PR #586

**Decision**: Reuse `pulumi.FindProject()` directly in the new project config
resolution function. No duplication needed.

**Rationale**: Proven, tested code that already handles edge cases (filesystem root,
relative paths, both `.yaml` and `.yml` extensions).

**Alternatives Considered**:

- Copy logic into config package: Rejected to avoid code duplication. Import is clean.
- Abstract into a shared utility: Over-engineering for a single reuse.

---

### R4: Config Merge Strategy

**Question**: How should project-local config merge with global config?

**Finding**: The spec mandates **shallow merge** at the top-level key level. The
current `Config` struct has these top-level YAML keys: `output`, `plugins`, `logging`,
`analyzer`, `plugin_host`, `cost`, `routing`.

**Decision**: Implement shallow merge where project config top-level keys completely
replace global defaults. Keys not present in project config inherit from global.

**Implementation approach**:

1. Load global config (existing `New()` flow)
2. Load project config YAML into raw `map[string]interface{}`
3. For each key present in project config, re-marshal that section and unmarshal
   onto the global config struct

**Rationale**: Shallow merge is explicitly specified. Using YAML raw nodes avoids
needing reflection-based field merging. It keeps the implementation simple and
predictable.

**Alternatives Considered**:

- Deep recursive merge: Rejected per spec ("adds complexity with minimal benefit").
- Overlay via environment variables only: Insufficient for structured config like budgets.
- Load project config as full `Config` and field-copy: Requires knowing which fields
  were explicitly set vs zero values.

---

### R5: `.gitignore` Generation

**Question**: What should the auto-generated `.gitignore` contain?

**Finding**: The spec says to prevent "accidentally committing user-specific data
like dismissal state." The project `.finfocus/` may contain:

- `config.yaml` (project-specific, **should** be committed)
- `dismissed.json` (user-specific, **should NOT** be committed)
- `dismissed.json.lock` (ephemeral, should not be committed)
- `dismissed.json.tmp` (ephemeral, should not be committed)

**Decision**: Generate `.gitignore` with:

```gitignore
# FinFocus project-local data (auto-generated)
# Config is tracked; user-specific state is not.
dismissed.json
dismissed.json.lock
dismissed.json.tmp
*.log
```

**Rationale**: `config.yaml` should be version-controlled (it defines project budgets,
output preferences). Only user-specific state (dismissals) and ephemeral files are
excluded.

**Alternatives Considered**:

- Ignore everything (`*`): Prevents committing `config.yaml`, losing the core benefit.
- Only ignore `dismissed.json`: Missing lock/tmp files and logs.

---

### R6: `config init` Enhancement

**Question**: How should `config init` behave inside a Pulumi project?

**Finding**: Current `config init` (`internal/cli/config_init.go`) always creates
config at the global `~/.finfocus/config.yaml` path. It has no project awareness.

**Decision**: Enhance `config init` to detect context:

- **Inside Pulumi project**: Create `$PROJECT/.finfocus/config.yaml` +
  `$PROJECT/.finfocus/.gitignore`
- **Outside Pulumi project**: Keep current behavior (global config)
- Add `--global` flag to force global init even inside a project

**Rationale**: The spec requires `config init` to create project-local config when
inside a project (FR-010). The `--global` flag provides an escape hatch.

**Alternatives Considered**:

- Separate `config init --project` subcommand: More complex, but the automatic
  detection is more ergonomic.
- Always require `--project-dir`: Verbose and against the auto-detect design.

---

### R7: CLI Flag Integration

**Question**: Where should `--project-dir` be added?

**Finding**: The root command in `internal/cli/root.go` uses `PersistentPreRunE` for
setup. Adding `--project-dir` as a persistent flag makes it available to all
subcommands.

**Decision**: Add `--project-dir` as a persistent flag on the root command. In
`PersistentPreRunE`, resolve the project directory and store it in a package-level
variable or context.

**Rationale**: Persistent flags propagate to all subcommands. The project directory
is needed by both cost commands (for config) and recommendation commands (for
dismissals).

**Alternatives Considered**:

- Per-command flag: Duplicative, inconsistent UX.
- Only env var: Less discoverable for CLI users.

---

### R8: Thread Safety of Config Merging

**Question**: Is the singleton config pattern safe with project-local config?

**Finding**: `internal/config/integration.go` uses `GlobalConfig` with `sync.RWMutex`.
The singleton is initialized once via `InitGlobalConfig()`. With project-local config,
the merged result should also be a singleton per invocation.

**Decision**: Initialize merged config in `PersistentPreRunE` after project dir is
resolved. The global singleton stores the already-merged config. No concurrent
writes after initialization.

**Rationale**: CLI commands are single-threaded request-response. Config is loaded
once at startup, then read-only.

**Alternatives Considered**:

- Per-request config: Over-engineering for a CLI tool.
- Config middleware: Adds unnecessary abstraction layer.

---

### R9: Backward Compatibility Verification

**Question**: How do we ensure existing users see no behavior change?

**Finding**: Backward compatibility (FR-009, SC-003, SC-006) requires:

1. No `Pulumi.yaml` in CWD hierarchy → same as today
2. No project `.finfocus/` even if `Pulumi.yaml` exists → global fallback
3. All existing tests pass without modification
4. `FINFOCUS_HOME` continues to work for global resources

**Decision**: The config resolution is additive. If no project config is found,
the existing `ResolveConfigDir()` + `New()` flow is used unchanged.

**Rationale**: The only behavioral change is when BOTH conditions are true:
(a) inside a Pulumi project AND (b) a `.finfocus/config.yaml` exists at the
project root. This is a new setup that no existing user has.

---

### R10: Performance of Directory Walk-Up

**Question**: Is walk-up performance acceptable for deep directory trees?

**Finding**: `pulumi.FindProject()` calls `os.Stat()` for `Pulumi.yaml` and
`Pulumi.yml` at each level. For a 50-level-deep tree, that's 100 stat calls maximum.
Each stat call is ~1-10μs on modern filesystems.

**Decision**: No optimization needed. 100 stat calls × 10μs = 1ms worst case, well
within the 100ms budget (SC-004).

**Rationale**: Filesystem stat is extremely fast. The walk-up terminates at the first
`Pulumi.yaml` found (typically within 1-5 levels).

**Alternatives Considered**:

- Cache the result: Unnecessary for sub-millisecond operation.
- Use filesystem watchers: Over-engineering for a one-shot CLI.
