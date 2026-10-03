# Upgrade to finfocus-spec v0.5.7: DryRun takes a context

v0.5.7 added a `context.Context` parameter to `DryRunHandler`. This shipped
in a patch release, even though the finfocus-spec CHANGELOG lists it under
0.6.0. It breaks compilation for any plugin that implements dry run.

v0.5.7 also requires Go 1.25.7. `finfocus plugin upgrade` raises the `go`
directive.

## Manual

Change every `HandleDryRun` implementation:

```go
// Before
func (p *Plugin) HandleDryRun(req *pbc.DryRunRequest) (*pbc.DryRunResponse, error)

// After
func (p *Plugin) HandleDryRun(ctx context.Context, req *pbc.DryRunRequest) (*pbc.DryRunResponse, error)
```

Pass `ctx` on to any call inside the handler that accepts one. Do not
start using it for new behavior: dry run must stay free of external calls.

Update tests that call the handler directly to pass a context, such as
`context.Background()` or `t.Context()`.

## Find what is left

```bash
grep -rn 'HandleDryRun(req' --include='*.go' .
```

The build reports any signature still missing the context: the type no
longer satisfies `pluginsdk.DryRunHandler`.
