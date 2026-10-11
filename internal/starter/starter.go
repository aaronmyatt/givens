// Package starter holds what Givens writes into every new capabilities folder: GIVENS.md and
// the "Add a story" capability with a setup script per scenario (see the
// create-capabilities-folder scenario in Open a project).
package starter

import (
	"embed"
	"io/fs"
	"strings"

	"givens/internal/assert"
)

// files is the starter capability, compiled into the binary.
// Ref: https://pkg.go.dev/embed
//
//go:embed add-a-story
var files embed.FS //tiger:allow-global read-only files embedded at build time

// guide is GIVENS.md for a new project: Givens's own, without the part about this repository.
//
//go:embed GIVENS.md
var guide string //tiger:allow-global read-only file embedded at build time

// Files returns the starter capability, rooted at its folder name.
func Files() fs.FS {
	feature, err := fs.Stat(files, "add-a-story/add-a-story.feature")
	assert.That(err == nil, "the starter's .feature file is embedded")
	assert.That(feature.Size() > 0, "the starter's .feature file isn't empty")
	return files
}

// Guide is the GIVENS.md a new capabilities folder gets.
func Guide() string {
	assert.That(strings.HasPrefix(guide, "# GIVENS.md"), "the guide is GIVENS.md")
	assert.That(!strings.Contains(guide, "## This repository"), "the guide is project-neutral")
	return guide
}
