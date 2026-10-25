package app

import (
	"givens/internal/assert"
)

// Prompt labels.
const (
	OfferCancel = "Cancel"
)

// prompt is a question the window asks before an action it can't undo: the message across the
// top, a confirm button, and a cancel button, "Cancel" unless the question names its own.
// While it's up, it's all the window offers.
type prompt struct {
	message  string
	confirm  string
	cancel   string
	run      func() error
	onCancel func() error // nil when cancelling does nothing
}

// ask puts up a prompt. Choosing confirm runs run; choosing "Cancel" does nothing.
func (w *Window) ask(message string, confirm string, run func() error) {
	assert.That(message != "", "a prompt says why it asks")
	assert.That(confirm != "" && confirm != OfferCancel, "a prompt has its own confirm button")
	assert.That(run != nil, "a prompt has something to run")
	w.prompt = &prompt{message: message, confirm: confirm, cancel: OfferCancel, run: run}
}

// askEither puts up a prompt whose two buttons each do something, like "Trust" and "Don't
// trust".
func (w *Window) askEither(message string, confirm string, run func() error, cancel string,
	onCancel func() error) {
	assert.That(confirm != "" && cancel != "" && confirm != cancel, "two distinct buttons")
	assert.That(run != nil && onCancel != nil, "both buttons do something")
	w.ask(message, confirm, run)
	w.prompt.cancel, w.prompt.onCancel = cancel, onCancel
}

// answer takes a prompt's button, and reports whether action was one.
func (w *Window) answer(action string) (bool, error) {
	assert.That(w != nil, "answer runs on a window")
	assert.That(action != "", "an action has a label")
	if w.prompt == nil || (action != w.prompt.confirm && action != w.prompt.cancel) {
		return false, nil
	}
	asked := w.prompt
	w.prompt = nil
	if action == asked.cancel {
		if asked.onCancel == nil {
			return true, nil
		}
		return true, asked.onCancel()
	}
	return true, asked.run()
}

// Question is the question up in the window, and the choice that answers it yes, like
// "Delete anyway"; "" when none is up. The givens command prints it, and --yes chooses it.
func (w *Window) Question() (string, string) {
	assert.That(w != nil, "question runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.prompt == nil {
		return "", ""
	}
	return w.prompt.message, w.prompt.confirm
}
