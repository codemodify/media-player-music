// Package frontend is the web UI's page, built from ../../web by
// web/build.sh: index.html and the bundled app.js.
package frontend

import "embed"

// Files are the page's files.
//
//go:embed index.html app.js app.js.map
var Files embed.FS
