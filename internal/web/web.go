// Package web holds the embedded static frontend assets (HTML/CSS/JS). The
// frontend is a single-page app driven by vanilla JS talking the REST API.
// Keeping the UI minimal and framework-free matches the "frontend stack is
// flexible" requirement while preserving single-responsibility: this package
// only serves bytes; the router mounts it.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// all:assets embeds every file under assets/ recursively, including files
// that would otherwise be skipped (none here, but the form is portable).
//
//go:embed all:assets
var embedded embed.FS

// FS is the http.FileSystem over the embedded static assets, rooted at
// assets/static so the files are addressable as /index.html, /app.js, /app.css.
var FS http.FileSystem

func init() {
	sub, err := fs.Sub(embedded, "assets/static")
	if err != nil {
		panic("web: embed sub: " + err.Error())
	}
	FS = http.FS(sub)
}
