//go:build !dev
// +build !dev

package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// GetDist returns the embedded frontend distribution files
func GetDist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
