package app

import (
	"givens/internal/assert"
)

// Prompt labels.
const (
	OfferCancel = "Cancel"
)

// prompt is a question the window asks before an action it can't undo: the message across the
// top, and a confirm button beside "Cancel". While it's up, it's all the window offers.
type prompt struct {
	message string
	confirm string
	run     func() error
}

// ask puts up a prompt. Choosing confirm runs run; choosing "Cancel" does nothing.
func (w *Window) ask(message string, confirm string, run func() error) {
	assert.That(message != "", "a prompt says why it asks")
	assert.That(confirm != "" && confirm != OfferCancel, "a prompt has its own confirm button")
	assert.That(run != nil, "a prompt has something to run")
	w.prompt = &prompt{message: message, confirm: confirm, run: run}
}

// answer takes a prompt's button, and reports whether action was one.
func (w *Window) answer(action string) (bool, error) {
	assert.That(w != nil, "answer runs on a window")
	assert.That(action != "", "an action has a label")
	if w.prompt == nil || (action != w.prompt.confirm && action != OfferCancel) {
		return false, nil
	}
	asked := w.prompt
	w.prompt = nil
	if action == OfferCancel {
		return true, nil
	}
	return true, asked.run()
}
