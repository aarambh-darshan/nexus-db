package studio

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

func init() {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return
	}
	staticHandler = http.FileServer(http.FS(subFS))
}
