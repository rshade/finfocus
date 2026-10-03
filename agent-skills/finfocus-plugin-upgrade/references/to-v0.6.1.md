# Upgrade to finfocus-spec v0.6.1: Go 1.27.1

v0.6.1 requires Go 1.27.1. `finfocus plugin upgrade` raises the `go`
directive in `go.mod`. The API is unchanged.

## Manual

1. Update the Go version everywhere it is pinned outside `go.mod`:
   `actions/setup-go` inputs, `mise.toml` or `.tool-versions`, Dockerfile
   base images (`golang:1.27.1`), and `golangci-lint` settings.
2. A `toolchain` line in `go.mod` older than 1.27.1 is now meaningless:
   remove it or raise it.

## Optional

`pluginsdk.Run(ServeConfig) int` is a new entry point. It handles flags,
signals, and exit codes and returns the process exit code:

```go
func main() {
    os.Exit(pluginsdk.Run(pluginsdk.ServeConfig{Plugin: plugin.New()}))
}
```

`pluginsdk.Serve(ctx, cfg)` still works. Switch only if the plugin wants the
standard flag handling.
