package app

import (
	"givens/internal/assert"
)

// ViewHelper is one helper listed in the sidebar's "Shared" view.
type ViewHelper struct {
	Name        string // "seed_shopper"
	Signature   string // the text after "# @helper": "seed_shopper [email]"
	Description string // the comment lines under it, joined with spaces
	File        string // where it's defined, relative to the capabilities folder
	Callers     int    // how many setup scripts call it
}

// ViewHelpers lists the helpers the "Shared" view shows, when it's selected: every helper
// documented with a "# @helper" comment, in capabilities/_shared and each capability's
// _shared folder. Needed by "Browse helpers" in capabilities/views. The scenarios in the
// other views ("Error states", "Unverified", a tag) are listed by ListedScenarios.
func (w *Window) ViewHelpers() []ViewHelper {
	assert.That(w != nil, "view helpers run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}
