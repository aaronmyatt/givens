package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/project"
)

// A capability's about.md, edited from its story: "Edit about.md", or "Add about.md" when
// there's none yet.

// About.md actions, offered with the story of the capability on show.
const (
	OfferEditAbout = "Edit about.md"
	OfferAddAbout  = "Add about.md"
)

// storyCapability is the capability whose story is on show: the selected one, or the first
// when nothing is selected, as shownCapability picks it.
func (w *Window) storyCapability() (project.Capability, bool) {
	assert.That(w != nil, "story capability runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.project == nil {
		return project.Capability{}, false
	}
	name := w.selectedCapability
	if w.selected == "" && len(w.project.Capabilities) > 0 {
		name = w.project.Capabilities[0].Slug
	}
	if name == "" {
		return project.Capability{}, false
	}
	return w.findCapability(name)
}

// aboutOffers is "Edit about.md" or "Add about.md" while a story is on show and nothing is in
// the editor. Needed by "Edit a capability's about.md" and "Add an about.md".
func (w *Window) aboutOffers() []string {
	assert.That(w != nil, "about offers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor != nil || w.session != nil {
		return []string{}
	}
	capability, ok := w.storyCapability()
	if !ok {
		return []string{}
	}
	if capability.About == "" {
		return []string{OfferAddAbout}
	}
	return []string{OfferEditAbout}
}

// editAbout opens the story's about.md in the editor, first writing one that holds only the
// capability's name as a heading when there's none.
func (w *Window) editAbout() error {
	assert.That(w != nil, "edit about runs on a window")
	assert.That(w.editor == nil, "nothing is in the editor yet")
	capability, ok := w.storyCapability()
	if !ok {
		return fmt.Errorf("no capability is on show: %w", ErrNotFound)
	}
	path := filepath.Join(filepath.Dir(capability.FeaturePath), "about.md")
	// O_EXCL leaves an about.md another tool just wrote alone.
	// Ref: https://pkg.go.dev/os#O_EXCL
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		_, err = file.WriteString("# " + capability.Name + "\n\n")
		err = errors.Join(err, file.Close())
	}
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	_, err = w.Edit(filepath.Join(capability.Slug, "about.md"))
	return err
}
