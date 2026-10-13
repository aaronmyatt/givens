package app

import (
	"errors"
	"path/filepath"
	"unicode/utf8"

	"givens/internal/assert"
)

// Shortcuts.
const (
	KeySearch = "⌘K"
	KeyEscape = "⎋"
	// KeyEdit opens the .feature file on show in the editor; see editShown.
	KeyEdit = "e"
)

// Type types text into whatever has focus: the search box, the editor, an ask card.
func (w *Window) Type(text string) error {
	assert.That(w != nil, "type runs on a window")
	assert.That(utf8.ValidString(text), "typed text is text")
	if w.search != nil {
		if len(w.search.query)+len(text) > NameLengthMax*4 {
			return errors.New("the search box is full")
		}
		w.search.query += text
		return nil
	}
	if w.session != nil && w.session.answering() {
		w.session.cardInput += text
		return nil
	}
	if w.editor != nil {
		return w.editor.typeText(text)
	}
	return ErrNotImplemented
}

// Press presses a key or shortcut: "⌘K", "⌘C", "⇧⌘C", "⌘↵", "⌘E", "↵".
func (w *Window) Press(keys string) error {
	assert.That(w != nil, "press runs on a window")
	assert.That(keys != "", "a shortcut names keys")
	switch keys {
	case KeySearch:
		w.search = &search{}
		return nil
	case KeyEscape:
		w.search = nil
		return nil
	case KeyCopySetup:
		if w.search == nil {
			return errors.New("search isn't open")
		}
		return w.copyFromSearch()
	case KeyEdit:
		return w.editShown()
	}
	return ErrNotImplemented
}

// editShown opens the editor at the scenario on show, at its block, or else at the top of the
// .feature file of the story on show. With the editor, a Drop in or search already open, the
// key is typed there instead, so it does nothing here. Needed by "Press e to edit a scenario"
// and "Press e to edit a story".
func (w *Window) editShown() error {
	assert.That(w != nil, "edit shown runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor != nil || w.session != nil || w.search != nil || w.prompt != nil {
		return nil
	}
	if w.selectedScenario != "" {
		_, err := w.Edit(w.selectedScenario)
		return err
	}
	capability, ok := w.storyCapability()
	if !ok {
		return nil
	}
	_, err := w.Edit(filepath.Join(capability.Slug, filepath.Base(capability.FeaturePath)))
	return err
}
