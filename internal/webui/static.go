package webui

import (
	"embed"
	"io/fs"
)

// staticAssets holds the embedded no-build SPA served by the web UI.
//
//go:embed static
var staticAssets embed.FS

// staticFiles returns the embedded SPA assets rooted at the static directory,
// so "/static/app.js" maps to "app.js" after prefix stripping.
func staticFiles() fs.FS {
	sub, err := fs.Sub(staticAssets, "static")
	if err != nil {
		// The path is a compile-time constant inside the embedded FS; this
		// cannot fail.
		panic(err)
	}
	return sub
}
