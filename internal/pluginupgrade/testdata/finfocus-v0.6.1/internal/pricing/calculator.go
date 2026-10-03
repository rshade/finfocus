package pricing

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	// PluginVersion is the semantic version of this plugin.
	PluginVersion = "0.1.0"
	// SpecVersion is the finfocus-spec protocol version this plugin was compiled against.
	SpecVersion = "0.6.1"
)

const builtAgainst = "finfocus-spec 0.6.1"

// GetPluginInfo returns metadata about this plugin.
func GetPluginInfo(context.Context) *pbc.GetPluginInfoResponse {
	return &pbc.GetPluginInfoResponse{Version: PluginVersion, SpecVersion: SpecVersion}
}
