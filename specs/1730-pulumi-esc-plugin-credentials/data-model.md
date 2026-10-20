# Data Model: Pulumi ESC Credentials for Plugins

None of these types are persisted. ESC values live in memory only, for the
launch of plugin processes (FR-013).

## Ref (`internal/esc`)

One ESC environment reference.

| Field | Type | Notes |
| --- | --- | --- |
| `Org` | `string` | Empty means it is resolved per R4: the stack's org for stack imports, the default org for configured references |
| `Project` | `string` | `default` when the reference has a single segment |
| `Name` | `string` | Required |
| `Version` | `string` | Revision number or tag. Empty means latest |

- **Syntax**: `[[org/]project/]name[@version]`. Each segment and the version
  match `^[A-Za-z0-9][A-Za-z0-9._-]*$`. One segment is `default/name`, two
  are `project/name`, and three are `org/project/name`. Anything else is a
  parse error.
- **`String()`** returns the canonical `org/project/name@version` without an
  empty org or version. This is the only form used in warnings, debug logs,
  the web `escEnvironment` field, and the cache fingerprint.
- **Org validation**: the SDK rejects an org shorter than 3 or longer than 40
  characters. `ParseRef` applies the same bounds so the error is a
  configuration error (FR-015), not a runtime failure.

## Config (`internal/config`)

```go
type ESCConfig struct {
    Environments *[]string                  `json:"environments,omitempty"`
    Plugins      map[string]ESCPluginConfig `json:"plugins,omitempty"`
}

type ESCPluginConfig struct {
    Environments *[]string `json:"environments,omitempty"`
    Variables    []string  `json:"variables,omitempty"`
}
```

- A `nil` pointer means not set. A non-nil pointer to an empty slice means
  disabled (FR-003).
- **Validation (FR-015)**: every entry passes `esc.ParseRef`. Every variable
  matches `^[A-Za-z_][A-Za-z0-9_]*$` and is not a reserved startup name. Each
  failure is reported at its key path, for example
  `esc.plugins.vantage.variables[1]`, and the exit code is 2.
- **Tier merge**: `Config.ESC` is replaced as a whole by the project tier
  (`ShallowMergeYAML`).

## StackContext (`internal/pulumi`)

What can be known about the stack without the Pulumi CLI.

| Field | Source |
| --- | --- |
| `ProjectDir` | `FindProject(cwd)` |
| `ProjectName` | `Pulumi.yaml` `name` |
| `BackendURL` | `Pulumi.yaml` `backend.url`, when present |
| `Stack` | `--stack`, else the workspace file (R3), else empty |
| `Environment` | The stack file's `environment` value, parsed as below |

`StackEnvironment` is one of these:

- `Imports []string`, from the list form or a map with only `imports`.
- `Unsupported`, from any other form (R2).
- `Absent`, when the key is missing or the stack is unknown.

## Selection (`internal/esc`)

The ordered references that apply to one plugin:

```text
esc.plugins.<plugin>.environments (if set)
  else esc.environments (if set)
  else StackContext.Environment imports (if Imports)
  else none
```

An `Unsupported` stack environment yields an `unsupported_definition` failure
for each plugin that falls through to it. `analyzer serve` stops before the
stack step (R10).

## Delivery (`internal/esc`)

What one plugin may receive from ESC.

| Field | Type | Notes |
| --- | --- | --- |
| `Values` | `map[string]string` | Every resolved name except reserved names (FR-008). `pluginhost` filters it to allowed names (FR-005) |
| `Origins` | `map[string]Ref` | The reference that won each name, for the web view and debug logs |
| `Fingerprints` | `map[string]string` | Per name: `ref` and a revision key (the ETag for latest, the number for `@N`, the resolved revision for `@tag`). Values never enter the cache key (R7 and R12) |

- **Allowed names** = the `Env` of each advertised setting ∪
  `esc.plugins.<plugin>.variables`. The pluginhost computes these. `Source`
  returns the merged environment, and the pluginhost filters it.
- **Merge rule**: references are applied in order, and a later reference
  overwrites an earlier one per name (FR-004).

## Failure (`internal/esc`)

```go
type Category string // not_logged_in | unsupported_backend | not_found |
                     // access_denied | timed_out | unsupported_definition | unavailable
type Failure struct {
    Ref      Ref // zero for failures before any ref, such as no token
    Category Category
}
```

- `Error()` returns `ESC environment <ref> skipped: <category text>`. It has
  no wrapped cause, so no SDK text can leak through `%w`.
- **Warnings**: one per `(Source, Ref, Category)` (FR-009), logged at Warn
  with `component=esc`, `operation=resolve`, `ref`, and `category`.

## Source (`internal/esc`)

The per-command resolver held on the context (`esc.WithSource` and
`esc.SourceFromContext`).

**State**:

```text
created ──first Delivery()──▶ resolving(ref) ──▶ opened(ref) | failed(ref)
```

- **Per reference**: a singleflight entry, and the result is memoized for the
  `Source`'s lifetime. A `Source` is never reused across commands, web
  reloads, or MCP calls (FR-012).
- **Budget**: each `Source` has 5 seconds of ESC wait time (FR-010, SC-004).
  - Time is charged only while at least one ESC call is in flight.
    Overlapping calls are charged once, as the union of their intervals.
  - A call that starts gets a deadline equal to the remaining budget.
  - A call that starts with no budget left fails at once as `timed_out`.
  - Idle time between calls is never charged. A plugin opened late in a
    command, such as the scorer, still gets the unused budget.
- **No selection**: no network access, no file reads beyond the stack
  context, and no warnings.

## Web view additions (`internal/webui`)

```go
type PluginSettingValue struct {
    // existing fields …
    EffectiveSource string `json:"effectiveSource"` // gains "esc"
    ESCEnvironment  string `json:"escEnvironment,omitempty"` // Ref.String() only
}
```

When `effectiveSource` is `esc`, `envSet` keeps its meaning: whether the
shell also has a value, which ESC overrides.
