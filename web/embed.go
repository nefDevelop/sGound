package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var Dist embed.FS

func GetFrontendAssets() (fs.FS, error) {
	return fs.Sub(Dist, "dist")
}
