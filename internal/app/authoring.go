package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/slug"
)

// Authoring offers.
const (
	OfferNewStory = "New story"
)

// NewStory creates a capability from the New story dialog: <slug>/<slug>.feature with
// "Feature: <Name>" and the story as its description, then lists it in the sidebar.
// Needed by "Write a new story" in capabilities/write-stories-and-scenarios.
func (w *Window) NewStory(draft StoryDraft) error {
	assert.That(w != nil, "new story runs on a window")
	assert.That(utf8.ValidString(draft.Name), "the story's name is text")
	assert.That(len(draft.Name) <= NameLengthMax, "the name fits NameLengthMax")
	assert.That(len(draft.SoThat) <= StorySentenceLengthMax, "each story line stays bounded")
	if err := w.writable(); err != nil {
		return err
	}
	name := slug.Of(draft.Name)
	if name == "" || w.project == nil {
		return errors.New("a story needs a name with letters or digits, in an open project")
	}
	for _, part := range []string{draft.Name, draft.AsA, draft.IWant, draft.SoThat} {
		if strings.TrimSpace(part) == "" || strings.Contains(part, "\n") {
			return errors.New("each line of the story is one line of text")
		}
	}
	dir := filepath.Join(w.path, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("there's already a %s folder", name)
	}
	text := "Feature: " + draft.Name + "\n  As a " + draft.AsA + "\n  I want " + draft.IWant +
		"\n  So that " + draft.SoThat + "\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".feature"), []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.Select(draft.Name)
}

// NewScenario adds a scenario to the selected capability from the New scenario dialog: a
// "Scenario:" block tagged @todo (and @error for an error state) and scripts/<slug>.sh sourcing
// the kit, then opens the scenario in the editor. Needed by "Add a scenario to a capability",
// "A new scenario starts flagged to do" and "Reuse an existing step" in
// capabilities/write-stories-and-scenarios.
func (w *Window) NewScenario(draft ScenarioDraft) error {
	assert.That(w != nil, "new scenario runs on a window")
	assert.That(utf8.ValidString(draft.Name), "the scenario's name is text")
	assert.That(len(draft.Name) <= NameLengthMax, "the name fits NameLengthMax")
	if err := w.writable(); err != nil {
		return err
	}
	capability, ok := w.findCapability(w.selectedCapability)
	if w.selectedCapability == "" || !ok {
		return errors.New("select a capability to add a scenario to")
	}
	key := slug.Of(draft.Name)
	if key == "" || strings.Contains(draft.Name, "\n") {
		return errors.New("a scenario needs a one-line name with letters or digits")
	}
	if _, exists := w.findIn(capability.Slug, draft.Name); exists {
		return fmt.Errorf("%s already has a scenario called %q", capability.Name, draft.Name)
	}
	text, err := readEditable(capability.FeaturePath)
	if err != nil {
		return err
	}
	// A new scenario isn't built yet, so it starts flagged @todo.
	tags := project.TagTodo
	if draft.Error {
		tags = project.TagError + " " + tags
	}
	block := "\n  " + tags + "\n  Scenario: " + draft.Name + "\n"
	updated := strings.TrimRight(text, "\n") + "\n" + block
	if err := os.WriteFile(capability.FeaturePath, []byte(updated), 0o644); err != nil {
		return err
	}
	script := filepath.Join(capability.Dir, "scripts", key+".sh")
	if err := writeNew(script, KitSourceLine+"\n"); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	_, err = w.Edit(capability.Slug + "/" + draft.Name)
	return err
}

// writeNew writes a new file, making its folder, unless one is already there.
func writeNew(path string, text string) error {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	assert.That(utf8.ValidString(text), "the file is text")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// blockEnd is the line after a scenario's block: past its indented lines, from the line
// holding "Scenario:" (counting from 0).
func blockEnd(lines []string, start int) int {
	assert.That(start >= 0, "a block starts on a line")
	assert.That(len(lines) <= EditorTextBytesMax, "lines stay bounded")
	end := start + 1
	// bound: end only grows, up to len(lines).
	for end < len(lines) && strings.HasPrefix(lines[end], "    ") {
		end++
	}
	return min(end, len(lines))
}
