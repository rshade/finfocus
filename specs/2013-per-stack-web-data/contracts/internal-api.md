# Internal Contracts: Per-Stack-Safe Web Data Path

No HTTP or protocol change (Web UI Fit: Not applicable). These are the
package-private and package-level signatures the tasks implement and #2039
consumes.

## `internal/pulumi`

```go
// ListStacks runs `pulumi stack ls --json` in projectDir.
func ListStacks(ctx context.Context, projectDir string) ([]StackInfo, error)

// ValidateStacks returns an error naming every requested stack that is not in
// available, followed by the available names. Nil when all are present.
func ValidateStacks(requested []string, available []StackInfo) error
// names are trimmed; org/project/stack matches its bare stack name
```

`GetCurrentStack` keeps its signature, error text, and `NoCurrentStackError`.

## `internal/cli`

```go
func openSharedStores(ctx context.Context, cmd *cobra.Command, cfg *config.Config) *sharedStores
func (s *sharedStores) Close(ctx context.Context) // idempotent, nil-safe

// Engine builder that uses injected stores and opens nothing.
func newEngineWithStores(ctx context.Context,
    clients []*pluginhost.Client,
    cfg *config.Config, stores *sharedStores) *engine.Engine

// overviewPipelineConfig gains:
//   stores *sharedStores // nil: the pipeline opens history per call, as today

// ESC
func stackESCSource(ctx context.Context, root *esc.Source, projectDir, stack string) *esc.Source
func (d *webData) bindStackESC(ctx context.Context) context.Context

// Runtime
func newWebStack(ctx context.Context, cmd *cobra.Command, params overviewParams,
    dates engine.DateRange, audit *auditContext, stores *sharedStores) *webStack
func (s *webStack) start()
func (s *webStack) stop()
```

## Behavioral contracts

- `stackESCSource` with `stack == ""` returns `root` unchanged.
- Each `webStack` creates its own passphrase channel; its session's submit
  callback sends on that channel only and ends on its own pipeline context.
- With `stores == nil`, every changed function behaves as before the change.
