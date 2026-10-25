package main

import (
	"fmt"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/termedit"
)

// runEdit opens a scenario's .feature file at its line, or any file under capabilities/ (an
// about.md, a setup script, a teardown, a helper), in the developer's editor, and saves it
// once it's valid. See capabilities/work-in-the-terminal.
func runEdit(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 1 || args[0] == "" || filepath.IsAbs(args[0]) {
		fmt.Println("usage: givens edit <capability>/<scenario> | <file under capabilities/>")
		return exitUsage
	}
	_, window, err := openProject()
	if err != nil {
		complain(err)
		return exitProblems
	}
	editor, err := window.Edit(args[0])
	if err != nil {
		complain(err)
		return exitProblems
	}
	outcome, edit, err := termedit.Run(editor)
	if err != nil {
		complain(err)
		return exitProblems
	}
	return reportEdit(outcome, edit, filepath.Base(editor.Path()))
}

// reportEdit says how an edit ended, and returns the exit code.
func reportEdit(outcome termedit.Outcome, edit *termedit.Edit, name string) int {
	assert.That(edit != nil, "an edit ran")
	assert.That(name != "", "the file has a name")
	switch outcome {
	case termedit.Saved:
		fmt.Println("Saved " + name)
	case termedit.NoChanges:
		fmt.Println("No changes")
	case termedit.Abandoned:
		fmt.Printf("%s isn't saved: it still has problems. Your edit is in %s\n", name,
			edit.File())
		return exitProblems
	case termedit.ChangedOnDisk:
		fmt.Printf("%s changed on disk while you edited it. Your edit is in %s\n", name,
			edit.File())
		return exitProblems
	}
	return exitOK
}
