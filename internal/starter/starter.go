// Package starter holds the "Add a story" capability that Givens writes into every new
// capabilities folder (see the create-capabilities-folder scenario in Open a project).
package starter

import (
	"embed"
	"io/fs"

	"givens/internal/assert"
)

// files is the starter capability, compiled into the binary.
// Ref: https://pkg.go.dev/embed
//
//go:embed add-a-story
var files embed.FS //tiger:allow-global read-only files embedded at build time

// Files returns the starter capability, rooted at its folder name.
func Files() fs.FS {
	feature, err := fs.Stat(files, "add-a-story/add-a-story.feature")
	assert.That(err == nil, "the starter's .feature file is embedded")
	assert.That(feature.Size() > 0, "the starter's .feature file isn't empty")
	return files
}
