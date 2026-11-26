# Internal API

No HTTP or protocol surface changes. Web UI Fit is `Not applicable`, so no
`http-api.md` is required.

```go
// package config
type ESCPluginConfig struct {
    Environments *[]string
    Variables    []string
    // Credentials maps a credential name the plugin reads to an ESC variable.
    Credentials map[string]string
}

// package esc
type PluginOptions struct {
    Environments *[]string
    Variables    []string
    Credentials  map[string]string
}
func (s *Source) Credentials(plugin string) map[string]string

// package pluginhost
type Client struct {
    // Conn is a grpc.ClientConnInterface; a holder of a process shared across
    // credentials gets a wrapper that attaches its own set to each call.
    Conn grpc.ClientConnInterface
}

// package proto
func NewCostSourceClient(conn grpc.ClientConnInterface) CostSourceClient
```

Unchanged: `Pool.Acquire`, `NewClient`, `Launcher`.

Wire: `x-finfocus-credential-<name>`, written only by
`pluginsdk.CredentialUnaryClientInterceptor`.
