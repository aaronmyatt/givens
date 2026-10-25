package app

import (
	"path/filepath"

	"givens/internal/assert"
)

// setupFile is true while the editor holds one of a scenario's setup files: its script, its
// setup steps or its teardown.
func (e *Editor) setupFile() bool {
	assert.That(e != nil, "setup file runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	if e.window.project == nil {
		return false
	}
	for _, capability := range e.window.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if e.path == scenario.Setup || e.path == scenario.SetupCards ||
				e.path == scenario.Teardown {
				return true
			}
		}
	}
	return false
}

// deleteFile asks, then moves the file open in the editor to the Trash and closes the editor,
// unsaved changes and all. A setup script another scenario runs with a from line asks "Delete
// anyway", as deleting the scenario does. Needed by "Delete a setup file while editing it"
// and "Delete a setup script another builds on".
func (w *Window) deleteFile() error {
	assert.That(w.editor != nil, "there's a file open to delete")
	assert.That(w.editor.setupFile(), "only a setup file is offered for deleting")
	path := w.editor.path
	builder := ""
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if scenario.Setup != path {
				continue
			}
			found, err := w.builtOn(scenario.Key(), "")
			if err != nil {
				return err
			}
			builder = found
		}
	}
	w.askDelete(builder, builder+" builds on this scenario", "Delete "+filepath.Base(path)+"?",
		func() error {
			if err := w.app.trash(path); err != nil {
				return err
			}
			w.editor = nil
			return w.reload()
		})
	return nil
}
