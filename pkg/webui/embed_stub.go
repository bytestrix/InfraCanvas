//go:build !embed_full

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:placeholder
var content embed.FS

// FS returns the placeholder dashboard. The binary was built without the
// `embed_full` tag, so the real frontend is not embedded.
func FS() (fs.FS, error) {
	return fs.Sub(content, "placeholder")
}
