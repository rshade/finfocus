package router

import (
	"context"

	"github.com/rshade/finfocus/internal/engine"
)

// EngineAdapter bridges the router package's Router interface to the engine
// package's Router interface. The only difference between the two PluginMatch
// types is MatchReason: router uses an int enum, engine uses a string.
type EngineAdapter struct {
	router Router
}

// NewEngineAdapter creates an engine.Router that delegates to the given Router,
// converting []router.PluginMatch results to []engine.PluginMatch.
func NewEngineAdapter(r Router) engine.Router {
	return &EngineAdapter{router: r}
}

// SelectPlugins delegates to the underlying router and converts the result
// slice from router.PluginMatch to engine.PluginMatch.
func (a *EngineAdapter) SelectPlugins(
	ctx context.Context,
	resource engine.ResourceDescriptor,
	feature string,
) []engine.PluginMatch {
	matches := a.router.SelectPlugins(ctx, resource, feature)
	out := make([]engine.PluginMatch, len(matches))
	for i, m := range matches {
		out[i] = engine.PluginMatch{
			Client:      m.Client,
			Priority:    m.Priority,
			Fallback:    m.Fallback,
			MatchReason: m.MatchReason.String(),
			Source:      m.Source,
		}
	}
	return out
}

// ShouldFallback delegates directly to the underlying router.
func (a *EngineAdapter) ShouldFallback(pluginName string) bool {
	return a.router.ShouldFallback(pluginName)
}
