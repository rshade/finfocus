// Package zerologviewer embeds the browser source of the zerolog viewer so a
// Go server can serve it without a build step.
package zerologviewer

import "embed"

// FS holds the viewer's ES modules and stylesheet under the "src" directory.
// Use fs.Sub(FS, "src") to serve them at a URL prefix.
//
//go:embed src
var FS embed.FS
