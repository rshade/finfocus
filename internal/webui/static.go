package webui

import (
	"embed"
	"io/fs"

	zerologviewer "github.com/rshade/finfocus/packages/zerolog-viewer"
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

// viewerFiles returns the zerolog viewer's embedded sources rooted at src, so
// "/static/zerolog-viewer/index.js" maps to "index.js" after prefix stripping.
func viewerFiles() fs.FS {
	sub, err := fs.Sub(zerologviewer.FS, "src")
	if err != nil {
		panic(err)
	}
	return sub
}
