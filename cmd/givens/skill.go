package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"givens/internal/app"
	"givens/internal/assert"
)

// runSkill handles `givens skill`: writes the agent skill Givens ships into the project the
// working folder is in. See capabilities/use-givens-from-the-terminal.
func runSkill(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 0 {
		fmt.Println("usage: givens skill")
		return exitUsage
	}
	root, err := findCapabilities(".")
	if err != nil {
		complain(err)
		return exitProblems
	}
	// The project is the folder holding capabilities/, where .claude/ sits beside it.
	// Ref: https://pkg.go.dev/path/filepath#Dir
	written, err := app.WriteSkill(filepath.Dir(root))
	if err != nil {
		complain(err)
		return exitProblems
	}
	fmt.Println("Wrote " + strings.Join(written, " and "))
	return exitOK
}
