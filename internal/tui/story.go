package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
)

// Enter on a capability opens its story to read: the As a, I want and So that lines, then
// about.md as it's written. j and k scroll a line, Space and b a page, g and G to the ends;
// e edits about.md, or adds one, in the developer's editor; Esc or h goes back to the list.

// storyState is the story page's own state.
type storyState struct {
	capability string // the capability's name, as the sidebar has it
	top        int    // the first line on show
}

// openStory shows a capability's story, from the top.
func (m *Model) openStory(capability string) {
	assert.That(capability != "", "a capability is named")
	assert.That(m.mode == modeBrowse, "a story opens from the list")
	m.mode, m.story = modeStory, storyState{capability: capability}
}

// storyLines are the story and about.md, wrapped to the terminal's width.
func (m *Model) storyLines() []string {
	assert.That(m.mode == modeStory, "a story is on show")
	assert.That(m.story.capability != "", "a capability is on show")
	found, err := m.window.Capability(m.story.capability)
	if err != nil {
		return []string{"✗ " + err.Error()}
	}
	text := strings.Join(found.Story, "\n") + "\n\n"
	if found.About == "" {
		text += "No about.md yet: press e to add one."
	} else {
		text += found.About
	}
	return wrapped(text, m.width)
}

// storyKey scrolls the story, edits its about.md, or goes back to the list.
func (m *Model) storyKey(name string) tea.Cmd {
	assert.That(m.mode == modeStory, "a story is on show")
	assert.That(m.story.top >= 0, "the story starts on show")
	page := m.bodyLines(0)
	last := max(len(m.storyLines())-page, 0)
	top := m.story.top
	switch name {
	case "esc", "q", "h", "left":
		m.mode = modeBrowse
	case "j", "down":
		top++
	case "k", "up":
		top--
	case "space", "pgdown", "f":
		top += page
	case "b", "pgup":
		top -= page
	case "g", "home":
		top = 0
	case "G", "end":
		top = last
	case "e":
		return m.editAbout()
	}
	m.story.top = min(max(top, 0), last)
	return nil
}

// editAbout opens the capability's about.md in the developer's editor, as the app's "Edit
// about.md" does, or writes one holding the capability's name first, as "Add about.md" does.
func (m *Model) editAbout() tea.Cmd {
	assert.That(m.mode == modeStory, "a story is on show")
	assert.That(m.editor == nil, "one file is edited at a time")
	err := m.window.Select(m.story.capability)
	if err == nil {
		offers := m.window.Offers()
		offer := app.OfferEditAbout
		if slices.Contains(offers, app.OfferAddAbout) {
			offer = app.OfferAddAbout
		}
		err = m.window.Choose(offer)
	}
	return m.beginEdit(m.window.Editor(), err)
}

// storyView draws the story page: the lines on show, and where they are in the story.
func (m *Model) storyView() string {
	assert.That(m.mode == modeStory, "a story is on show")
	assert.That(m.story.top >= 0, "the story starts on show")
	all := m.storyLines()
	page := m.bodyLines(0)
	top := min(m.story.top, max(len(all)-page, 0))
	text := "── " + m.story.capability + " · story and about.md\n\n"
	text += lines(all[top:min(top+page, len(all))])
	keys := "j k scroll · space b page · g G ends · e edit about.md · esc h back"
	if len(all) > page {
		keys += fmt.Sprintf(" · lines %d–%d of %d", top+1, min(top+page, len(all)), len(all))
	}
	return text + "\n" + keys + "\n"
}
