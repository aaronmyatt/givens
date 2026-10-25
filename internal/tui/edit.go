package tui

import (
	"errors"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
	"givens/internal/termedit"
)

// Editing hands the terminal to the developer's editor through Exec, then takes the file
// back with termedit: saved once it's valid, reopened over problems. See internal/termedit.

// editing is a file in the developer's editor.
type editing struct {
	edit *termedit.Edit
	name string // the file's name, as the note under the page says it
}

// editedMsg is the editor closing, with the error it ended with, if any.
type editedMsg struct {
	err error
}

// editPath opens a file, relative to the capabilities folder, or a scenario by its run key,
// in the developer's editor.
func (m *Model) editPath(target string) tea.Cmd {
	assert.That(m.editor == nil, "one file is edited at a time")
	assert.That(target != "", "something is edited")
	return m.beginEdit(m.window.Edit(target))
}

// beginEdit hands the file in the window's editor to the developer's editor, unless opening
// it failed with err.
func (m *Model) beginEdit(editor *app.Editor, err error) tea.Cmd {
	assert.That(m.editor == nil, "one file is edited at a time")
	assert.That(m.exec != nil, "the model can run the editor")
	if err == nil && editor == nil {
		err = errors.New("nothing opened in the editor")
	}
	if err != nil {
		m.note = "✗ " + err.Error()
		return nil
	}
	edit, err := termedit.Begin(editor)
	if err != nil {
		m.window.CloseEditor()
		m.note = "✗ " + err.Error()
		return nil
	}
	m.editor = &editing{edit: edit, name: filepath.Base(editor.Path())}
	return m.runEditor()
}

// runEditor opens the developer's editor on the file being edited.
func (m *Model) runEditor() tea.Cmd {
	assert.That(m.editor.name != "", "the file has a name")
	assert.That(m.editor != nil, "a file is being edited")
	assert.That(m.exec != nil, "the model can run the editor")
	return m.exec(m.editor.edit.Command(), func(err error) tea.Msg { return editedMsg{err: err} })
}

// edited takes the file back once the editor closed: saved, reopened over problems, or kept
// aside, said in the note under the page.
func (m *Model) edited(message editedMsg) tea.Cmd {
	assert.That(m != nil, "edited runs on a model")
	assert.That(m.window != nil, "the model has a window")
	if m.editor == nil {
		return nil
	}
	editing := m.editor
	if message.err != nil {
		m.editor, m.note = nil, "✗ The editor didn't finish: "+message.err.Error()
		m.window.CloseEditor()
		return nil
	}
	outcome, again, err := editing.edit.Finish()
	if err == nil && again {
		return m.runEditor()
	}
	m.editor = nil
	m.window.CloseEditor()
	switch {
	case err != nil:
		m.note = "✗ " + err.Error()
	case outcome == termedit.Saved:
		m.note = "Saved " + editing.name
	case outcome == termedit.NoChanges:
		m.note = "No changes"
	case outcome == termedit.Abandoned:
		m.note = editing.name + " isn't saved: it still has problems. Your edit is in " +
			editing.edit.File()
	case outcome == termedit.ChangedOnDisk:
		m.note = editing.name + " changed on disk while you edited it. Your edit is in " +
			editing.edit.File()
	}
	return nil
}
