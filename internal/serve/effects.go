package serve

import (
	"path/filepath"

	"givens/internal/assert"
)

// Effect is something only the app can do, so serve asks it to: write the clipboard, move a
// file to the Trash. Swift's Effect.
type Effect struct {
	Kind string `json:"kind"` // "clipboard", "trash", "folderPicker", "openURL", "bringToFront"
	Text string `json:"text,omitempty"`
	Path string `json:"path,omitempty"`
}

// recorder is the core's clipboard and Trash under serve: it records effects for the reply
// instead of acting on them.
type recorder struct {
	clipboard string
	effects   []Effect
}

// WriteText asks the app to write the clipboard, and remembers the text for ReadText.
func (r *recorder) WriteText(text string) error {
	assert.That(r != nil, "write runs on a recorder")
	r.clipboard = text
	r.add(Effect{Kind: "clipboard", Text: text})
	assert.That(r.effects[len(r.effects)-1].Kind == "clipboard", "the write was recorded")
	return nil
}

// ReadText is what the core last wrote to the clipboard.
func (r *recorder) ReadText() (string, error) {
	assert.That(r != nil, "read runs on a recorder")
	assert.That(len(r.effects) <= EffectsPerCallMax, "effects stay bounded")
	return r.clipboard, nil
}

// Move asks the app to move path to the Trash, so Finder can put it back.
// Ref: https://developer.apple.com/documentation/appkit/nsworkspace/recycle(_:completionhandler:)
func (r *recorder) Move(path string) error {
	assert.That(r != nil, "move runs on a recorder")
	assert.That(filepath.IsAbs(path), "a trashed path is absolute")
	r.add(Effect{Kind: "trash", Path: path})
	return nil
}

// add records one effect; past EffectsPerCallMax the call is a bug.
func (r *recorder) add(effect Effect) {
	assert.That(effect.Kind != "", "an effect has a kind")
	assert.That(len(r.effects) < EffectsPerCallMax, "one call asks for few effects")
	r.effects = append(r.effects, effect)
}

// reset forgets the last call's effects.
func (r *recorder) reset() {
	assert.That(r != nil, "reset runs on a recorder")
	r.effects = make([]Effect, 0, 4)
	assert.That(len(r.effects) == 0, "a reset recorder holds no effects")
}

// taken returns this call's effects.
func (r *recorder) taken() []Effect {
	assert.That(r != nil, "taken runs on a recorder")
	assert.That(len(r.effects) <= EffectsPerCallMax, "effects stay bounded")
	return r.effects
}

// OpenURL asks the app to open a link in the default browser.
// Ref: https://developer.apple.com/documentation/appkit/nsworkspace/open(_:)
func (r *recorder) OpenURL(url string) error {
	assert.That(r != nil, "open runs on a recorder")
	assert.That(url != "", "a link has text")
	r.add(Effect{Kind: "openURL", Text: url})
	return nil
}

// ShowFolderPicker asks the app to show its folder picker, for "Open folder…".
// Ref: https://developer.apple.com/documentation/appkit/nsopenpanel
func (r *recorder) ShowFolderPicker() error {
	assert.That(r != nil, "show picker runs on a recorder")
	assert.That(len(r.effects) < EffectsPerCallMax, "there's room for an effect")
	r.add(Effect{Kind: "folderPicker"})
	return nil
}
