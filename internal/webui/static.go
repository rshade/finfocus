package webui

import "embed"

// staticAssets holds the embedded no-build SPA served by the web UI.
//
//go:embed static
var staticAssets embed.FS
