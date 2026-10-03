package plugin

import (
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

var (
	// SpecVersion is reported by GetPluginInfo.
	SpecVersion = "v0.5.2"
	name        = "modern"
)

func Info() *pbc.GetPluginInfoResponse {
	return &pbc.GetPluginInfoResponse{Name: name, SpecVersion: SpecVersion}
}
