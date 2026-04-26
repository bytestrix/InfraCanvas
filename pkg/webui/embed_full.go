//go:build embed_full

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var content embed.FS

// FS returns the real dashboard, embedded at build time from pkg/webui/dist/.
// Populate dist/ with `make build-frontend` before running `go build -tags embed_full`.
func FS() (fs.FS, error) {
	return fs.Sub(content, "dist")
}
