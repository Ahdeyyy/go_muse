package webapi

import (
	"embed"
	"io/fs"
)

// dist holds the built Svelte single-page app. It is populated by
// `npm run build` in ../../web (Vite is configured to output here). A
// placeholder index.html is committed so `go build` works before the first
// frontend build.
//
//go:embed all:dist
var dist embed.FS

// staticFS returns the embedded SPA rooted at the dist directory.
func staticFS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time; this cannot fail
	}
	return sub
}
