// Package web embeds the browser application into the server binary.
package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed assets
var embeddedFiles embed.FS

// FileSystem returns the embedded frontend with assets as its root.
func FileSystem() (fs.FS, error) {
	root, err := fs.Sub(embeddedFiles, "assets")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	return root, nil
}
