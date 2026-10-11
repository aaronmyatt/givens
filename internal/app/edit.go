package app

import (
	"strings"
	"unicode/utf8"

	"givens/internal/assert"
)

// Limits for editing. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// EditorTextBytesMax bounds a file open in the editor; it matches the spec reader's limit
	// on a .feature file.
	EditorTextBytesMax = 4 << 20
	// StorySentenceLengthMax bounds each line typed into the New story dialog.
	StorySentenceLengthMax = NameLengthMax
)

// StoryDraft is what a developer fills in on the New story dialog: the capability's name and
// the three halves of its story, without their "As a", "I want" and "So that" prefixes.
type StoryDraft struct {
	Name   string // "Refunds"
	AsA    string // "a shop owner"
	IWant  string // "to refund an order"
	SoThat string // "the shopper gets their money back without emailing support"
}

// ScenarioDraft is what a developer fills in on the New scenario dialog.
type ScenarioDraft struct {
	Name  string // "Expired card"
	Error bool   // tag it @error
}

// EditorMark is a problem the editor marks on one line of the file it holds.
type EditorMark struct {
	Line    int    // 1-based line number in the editor's text
	Message string // "Expected a step, a tag, a comment or a new scenario"
}

// StepSuggestion is one step offered while typing in the editor, drawn from steps already in
// the project.
type StepSuggestion struct {
	Text  string // the step without its keyword: "a signed-in shopper with an empty cart"
	Label string // how widely it's used: "used in 4"
}

// PreviewLine is one line of the editor's preview, as the library would show it.
type PreviewLine struct {
	Text string // the line as shown, keywords lined up for steps
	Mark string // the parser's message when the line doesn't parse, or ""
}

// Editor is the window's editor, holding one file: a .feature file, an about.md, a setup
// script or a helper. Its text is unsaved until Save.
type Editor struct {
	window   *Window
	path     string
	text     string // what the editor shows, unsaved changes included
	opened   string // the file's text when it was opened or last saved
	problems []string
}

// NewStory creates a capability from the New story dialog: <slug>/<slug>.feature with
// "Feature: <Name>" and the story as its description, then lists it in the sidebar.
// Needed by "Write a new story" in capabilities/write-stories-and-scenarios.
func (w *Window) NewStory(draft StoryDraft) error {
	assert.That(w != nil, "new story runs on a window")
	assert.That(utf8.ValidString(draft.Name), "the story's name is text")
	assert.That(len(draft.Name) <= NameLengthMax, "the name fits NameLengthMax")
	assert.That(len(draft.SoThat) <= StorySentenceLengthMax, "each story line stays bounded")
	return ErrNotImplemented
}

// NewScenario adds a scenario to the selected capability from the New scenario dialog: a
// "Scenario:" block (tagged @error for an error state) and scripts/<slug>.sh sourcing the kit,
// then opens the scenario in the editor. Needed by "Add a scenario to a capability" and "Reuse
// an existing step" in capabilities/write-stories-and-scenarios.
func (w *Window) NewScenario(draft ScenarioDraft) error {
	assert.That(w != nil, "new scenario runs on a window")
	assert.That(utf8.ValidString(draft.Name), "the scenario's name is text")
	assert.That(len(draft.Name) <= NameLengthMax, "the name fits NameLengthMax")
	return ErrNotImplemented
}

// Delete asks to delete a scenario or a capability by its name, offering "Delete" and
// "Cancel", or "Delete anyway" when another setup builds on it. Choosing it removes the
// scenario's block and moves its setup files, or the capability's folder, to the Trash.
// Needed by the delete scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Delete(name string) error {
	assert.That(w != nil, "delete runs on a window")
	assert.That(name != "", "delete names what to delete")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	return ErrNotImplemented
}

// Marks lists the problems marked in the editor's text, by line.
func (e *Editor) Marks() []EditorMark {
	assert.That(e != nil, "marks runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return nil
}

// Suggestions lists the steps offered for the step being typed, drawn from the project's
// steps. Needed by "Reuse an existing step".
func (e *Editor) Suggestions() []StepSuggestion {
	assert.That(e != nil, "suggestions runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return nil
}

// ChooseSuggestion replaces the step being typed with an offered step, word for word.
func (e *Editor) ChooseSuggestion(text string) error {
	assert.That(e != nil, "choose suggestion runs on an editor")
	assert.That(text != "", "a suggestion has text")
	assert.That(!strings.Contains(text, "\n"), "a suggestion is one step")
	return ErrNotImplemented
}

// Preview is what "Preview" shows for the editor's text, unsaved changes included: a
// scenario as it reads in the library, or about.md as the library renders it. Needed by the
// preview scenarios in capabilities/write-stories-and-scenarios.
func (e *Editor) Preview() []PreviewLine {
	assert.That(e != nil, "preview runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return nil
}
