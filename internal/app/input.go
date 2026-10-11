package app

import (
	"errors"
	"unicode/utf8"

	"givens/internal/assert"
)

// Shortcuts.
const (
	KeySearch = "⌘K"
	KeyEscape = "⎋"
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
	}
	return ErrNotImplemented
}
