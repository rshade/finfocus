# Contract: `internal/esc` and Plugin-Host Seams

## Import rules

- `internal/esc` may import `internal/logging` and `internal/pulumi`.
- It MUST NOT import `cli`, `engine`, `pluginhost`, `registry`, `config`, or
  `webui`. Configuration is passed in as plain values.
- `internal/pluginhost` imports `internal/esc`.

## `internal/esc`

```go
// ParseRef parses [[org/]project/]name[@version].
func ParseRef(s string) (Ref, error)

// Options configures one Source. All fields are plain values.
type Options struct {
    Global      *[]string                  // esc.environments
    PerPlugin   map[string]PluginOptions   // esc.plugins
    Stack       pulumi.StackContext        // zero for analyzer serve
    UseStack    bool                       // false for analyzer serve
    Getenv      func(string) string        // os.Getenv in production
    HTTPClient  *http.Client               // nil means the default no-redirect client
    Budget      time.Duration              // 5s in production
    Warn        func(text string)          // user-facing warning sink; nil discards
}
type PluginOptions struct {
    Environments *[]string
    Variables    []string
}

func NewSource(opts Options) *Source
func WithSource(ctx context.Context, s *Source) context.Context
func SourceFromContext(ctx context.Context) (*Source, bool)

// Delivery returns the merged ESC environment for plugin's selection: every
// resolved name except reserved startup names, each with the reference that
// won it. It does not apply the allowlist; pluginhost does. A failure is
// reported through the Source's warning sink and an empty Delivery. Delivery
// never returns an error to the caller.
func (s *Source) Delivery(ctx context.Context, plugin string) Delivery

// Variables returns esc.plugins.<plugin>.variables, the configured names
// plugin may receive in addition to its advertised settings.
func (s *Source) Variables(plugin string) []string
```

### Behavior

| Input | Result |
| --- | --- |
| No selection for the plugin | Empty `Delivery`, no I/O, no warning |
| Selection, token missing or encrypted | Empty, one `not_logged_in` warning per `Source` |
| DIY backend | Empty, one `unsupported_backend` warning |
| Reference `404` / `403` / `401` | That reference contributes nothing, plus one warning for it. Other references still apply |
| Budget exceeded | Unopened references fail as `timed_out` |
| SDK panic | Recovered, `unavailable` |
| Two plugins share a reference | One open (singleflight + memo) |

**Warning text**: every warning is logged at Warn level and passed to
`Options.Warn`.

- Line-oriented commands, `--web`, and `mcp-server` set `Warn` to
  `cmd.PrintErrln("Warning: " + text)`, which is the existing CLI convention.
- The overview TUI collects warnings and prints them to stderr after the
  program exits, so the screen is never corrupted.

```text
ESC environment <ref> skipped (<category>); plugins use shell and config values
ESC environment <ref> skipped (not found or no access); plugins use shell and config values
ESC skipped (not logged in): set PULUMI_ACCESS_TOKEN or run `pulumi login`
```

The second line is the `not_found` form. Pulumi Cloud answers `404` both for
a missing environment and for one the token cannot see (research R12).

No value, URL query, response body, or SDK error string ever appears.

## `internal/pulumi`

These are moved from `internal/cli/overview.go` and extended. The old call
sites switch to the moved functions.

```go
func StackSettingsNameCandidates(stack string) []string      // moved
func ReadStackSettingsFile(projectDir, stack string) ([]byte, error) // moved
func SelectedStack(projectDir string) (string, error)        // R3 workspace file
func LoadStackContext(projectDir, stackFlag string) (StackContext, error)
```

`LoadStackContext` never runs the Pulumi CLI and never touches the network.

## `internal/pluginhost` changes

```go
// Before
func ResolveDeliveredEnv(ctx, plugin string, parent []string,
    project, global map[string]string, settings []SettingDesc) ([]string, string)

// After: the result type carries the override set
type Delivered struct {
    Pairs     []string        // KEY=VALUE to add
    Overrides map[string]bool // names whose parent value must be replaced (ESC)
    Digest    string
    Origins   map[string]esc.Ref
}
func ResolveDeliveredEnv(ctx context.Context, plugin string, parent []string,
    project, global map[string]string, settings []SettingDesc,
    fromESC esc.Delivery) Delivered
```

- **Precedence per name**: ESC (allowed) > parent > project > global >
  default. Names ESC does not supply keep the current order (FR-007).
- **Digest**: identical to today when `fromESC` is empty (SC-002). ESC names
  hash per research R7.
- **`resolveInjected`**: obtains `fromESC` from
  `esc.SourceFromContext(ctx).Delivery(ctx, plugin)`. When no `Source` is
  present, it passes an empty `Delivery`, so tests and probes keep today's
  behavior.
- **Allowlist**: allowed = the descriptor `Env` names ∪
  `src.Variables(plugin)`. The filter is applied inside `ResolveDeliveredEnv`,
  which is the only place it runs. `Delivered.Origins` lists only allowed
  names.
- **`WithDeliveredEnv`**: stores `Delivered`. `appendDeliveredEnv` replaces
  the inherited entry for each `Overrides` name and still never touches
  `pluginsdk.EnvPort`, `EnvLogFile`, or `EnvTraceID`.
- **Debug log (FR-016)**: `component=pluginhost`, `operation=resolve_settings`,
  `plugin`, `esc_env` (names), and `esc_refs` (`Ref.String()` values).

## `internal/cli` attachment points

| Entry point | `Source` |
| --- | --- |
| Root `PersistentPreRunE` (`root.go`) | `NewSource` from merged config, plus `LoadStackContext(FindProject(cwd), --stack if the command has it)`. `UseStack=true`. First parses every configured reference with `ParseRef`; a parse error returns `toValidationError` (exit 2) for every command except the `config` subtree, so `config set` and `config validate` can still repair or report it |
| `--web` `reloadPlugins` (`web_data.go`) | A new `Source` with the same options, stored on the session data for the settings view |
| `analyzer serve` (`analyzer_serve.go`) | `UseStack=false` (R10) |
| `mcp-server` | Inherited per call from `PersistentPreRunE` |

A failure in `LoadStackContext` is debug-logged and yields a zero
`StackContext`. It never fails the command.
