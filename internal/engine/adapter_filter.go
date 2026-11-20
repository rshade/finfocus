package engine

import "github.com/rshade/finfocus/internal/pluginhost"

// adapterFilter returns whether a client may answer a request restricted to
// adapter (--adapter). The name is resolved once against every client with
// pluginhost.ResolveName, so a plugin installed under the name wins over one
// that only reports it. A name that only plugins report admits every plugin
// reporting it. An empty adapter admits every client.
func (e *Engine) adapterFilter(adapter string) func(*pluginhost.Client) bool {
	if adapter == "" {
		return func(*pluginhost.Client) bool { return true }
	}
	chosen, kind, _ := pluginhost.ResolveName(e.clients, adapter)
	switch {
	case chosen == nil:
		// A client the engine was not built with (a router-only test double)
		// still answers to its own names.
		return func(c *pluginhost.Client) bool { return c.MatchesName(adapter) }
	case kind == pluginhost.MatchReported:
		return func(c *pluginhost.Client) bool { return c != nil && c.Name == adapter }
	default:
		return func(c *pluginhost.Client) bool { return c == chosen }
	}
}
