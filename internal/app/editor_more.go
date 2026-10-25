package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"givens/internal/assert"
	"givens/internal/spec"
)

// Editor actions.
const (
	OfferPreview   = "Preview"
	OfferKeepMine  = "Keep mine"
	OfferUseTheirs = "Use theirs"
	OfferCompare   = "Compare"
	// OfferCloseEditor closes the editor; with unsaved changes it asks first.
	OfferCloseEditor = "Close"
	// OfferDiscardChanges answers that question: close, losing what wasn't saved.
	OfferDiscardChanges = "Discard changes"
	// OfferDeleteFile moves a setup file open in the editor to the Trash, after asking.
	OfferDeleteFile = "Delete file"
	// SuggestionsMax bounds the steps offered while typing.
	SuggestionsMax = 20
)

// stepKeywords start a step line.
func stepKeywords() []string {
	keywords := []string{"Given", "When", "Then", "And", "But", "*"}
	assert.That(len(keywords) == 6, "Gherkin has six step keywords")
	assert.That(keywords[5] == "*", "the bullet is a keyword too")
	return keywords
}

// typeText types into the editor: the first text starts a new line at the cursor, as a step
// in the scenario's block; more text adds to that line.
func (e *Editor) typeText(text string) error {
	assert.That(e != nil, "type runs on an editor")
	assert.That(!strings.Contains(text, "\n"), "typing adds to one line")
	lines := strings.Split(e.text, "\n")
	if e.typing < 0 || e.typing >= len(lines) {
		at := min(max(e.cursor, 0), len(lines))
		lines = append(lines[:at], append([]string{"    " + text}, lines[at:]...)...)
		e.typing = at
	} else {
		lines[e.typing] += text
	}
	joined := strings.Join(lines, "\n")
	if len(joined) > EditorTextBytesMax {
		return errors.New("the file is full")
	}
	e.text = joined
	return nil
}

// typedStep splits the line being typed into its keyword and the start of its text.
func (e *Editor) typedStep() (string, string, bool) {
	assert.That(e != nil, "typed step runs on an editor")
	assert.That(e.typing < len(strings.Split(e.text, "\n")) || e.typing < 0, "typing is a line")
	if e.typing < 0 {
		return "", "", false
	}
	line := strings.TrimSpace(strings.Split(e.text, "\n")[e.typing])
	keyword, partial, _ := strings.Cut(line, " ")
	return keyword, partial, partial != ""
}

// Suggestions lists the steps offered for the step being typed, drawn from the project's
// steps: those starting with what's typed, most used first, each with "used in N". Needed by
// "Reuse an existing step".
func (e *Editor) Suggestions() []StepSuggestion {
	assert.That(e != nil, "suggestions runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	_, partial, ok := e.typedStep()
	if !ok {
		return []StepSuggestion{}
	}
	return e.suggestionsFor(partial)
}

// SuggestionsFor lists the steps offered for a step line as the Mac editor holds it: "Given a
// card" offers steps starting "a card". A line that isn't a step, or a step already written
// out word for word, offers none.
func (e *Editor) SuggestionsFor(line string) []StepSuggestion {
	assert.That(e != nil, "suggestions runs on an editor")
	assert.That(!strings.Contains(line, "\n"), "suggestions are for one line")
	keyword, partial, _ := strings.Cut(strings.TrimSpace(line), " ")
	if !isStepKeyword(keyword) || strings.TrimSpace(partial) == "" {
		return []StepSuggestion{}
	}
	offered := e.suggestionsFor(partial)
	if len(offered) == 1 && offered[0].Text == partial {
		return []StepSuggestion{}
	}
	return offered
}

// suggestionsFor lists the project's steps starting with partial, most used first.
func (e *Editor) suggestionsFor(partial string) []StepSuggestion {
	assert.That(e.window != nil, "an editor belongs to a window")
	assert.That(partial != "", "suggestions start from what's typed")
	if e.window.project == nil {
		return []StepSuggestion{}
	}
	uses := make(map[string]int, 32)
	for _, capability := range e.window.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, step := range uniqueSteps(scenario) {
				if strings.HasPrefix(strings.ToLower(step.Text), strings.ToLower(partial)) {
					uses[step.Text]++
				}
			}
		}
	}
	texts := make([]string, 0, len(uses))
	for text := range uses {
		texts = append(texts, text)
	}
	sort.Slice(texts, func(a, b int) bool {
		return uses[texts[a]] > uses[texts[b]] ||
			(uses[texts[a]] == uses[texts[b]] && texts[a] < texts[b])
	})
	suggestions := make([]StepSuggestion, 0, min(len(texts), SuggestionsMax))
	for _, text := range texts[:min(len(texts), SuggestionsMax)] {
		suggestions = append(suggestions, StepSuggestion{
			Text: text, Label: fmt.Sprintf("used in %d", uses[text]),
		})
	}
	return suggestions
}

// ChooseSuggestion replaces the step being typed with an offered step, word for word.
func (e *Editor) ChooseSuggestion(text string) error {
	assert.That(e != nil, "choose suggestion runs on an editor")
	assert.That(text != "", "a suggestion has text")
	assert.That(!strings.Contains(text, "\n"), "a suggestion is one step")
	keyword, _, ok := e.typedStep()
	if !ok {
		return errors.New("no step is being typed")
	}
	for _, suggestion := range e.Suggestions() {
		if suggestion.Text != text {
			continue
		}
		lines := strings.Split(e.text, "\n")
		lines[e.typing] = "    " + keyword + " " + text
		e.text, e.typing = strings.Join(lines, "\n"), -1
		return nil
	}
	return fmt.Errorf("%q isn't offered", text)
}

// Marks lists the problems marked in the editor's text, by line: what the parser couldn't
// read at the last save.
func (e *Editor) Marks() []EditorMark {
	assert.That(e != nil, "marks runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return append([]EditorMark{}, e.marks...)
}

// markParseErrors reads a saved .feature file and marks the line the parser stops at.
func (e *Editor) markParseErrors() {
	assert.That(e != nil, "mark runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	e.marks = []EditorMark{}
	if !strings.HasSuffix(e.path, ".feature") {
		return
	}
	if _, err := spec.ParseFeature(e.path); err != nil {
		line, message := spec.Explain(err)
		e.marks = append(e.marks, EditorMark{Line: int(line), Message: capitalized(message)})
	}
}

// capitalized starts a message with a capital letter, as a mark shows it.
func capitalized(message string) string {
	assert.That(len(message) <= NameLengthMax*4, "a message stays bounded")
	if message == "" {
		return message
	}
	return strings.ToUpper(message[:1]) + message[1:]
}

// conflict is true while the file on disk differs from the one the editor opened: another
// tool saved it.
func (e *Editor) conflict() bool {
	assert.That(e != nil, "conflict runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	onDisk, err := os.ReadFile(e.path)
	return err == nil && string(onDisk) != e.opened
}

// editorOffers are what the editor offers: the ways out of a conflict, or "Preview".
func (e *Editor) editorOffers() []string {
	assert.That(e != nil, "editor offers run on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	if e.conflict() {
		return []string{OfferKeepMine, OfferUseTheirs, OfferCompare}
	}
	if e.previewable() {
		return []string{OfferPreview, OfferCloseEditor}
	}
	if e.setupFile() {
		return []string{OfferDeleteFile, OfferCloseEditor}
	}
	return []string{OfferCloseEditor}
}

// setupFile is true while the editor holds one of a scenario's setup files: its script, its
// setup steps or its teardown.
func (e *Editor) setupFile() bool {
	assert.That(e != nil, "setup file runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	if e.window.project == nil {
		return false
	}
	for _, capability := range e.window.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if e.path == scenario.Setup || e.path == scenario.SetupCards ||
				e.path == scenario.Teardown {
				return true
			}
		}
	}
	return false
}

// deleteFile asks, then moves the file open in the editor to the Trash and closes the editor,
// unsaved changes and all. A setup script another scenario runs with a from line asks "Delete
// anyway", as deleting the scenario does. Needed by "Delete a setup file while editing it"
// and "Delete a setup script another builds on".
func (w *Window) deleteFile() error {
	assert.That(w.editor != nil, "there's a file open to delete")
	assert.That(w.editor.setupFile(), "only a setup file is offered for deleting")
	path := w.editor.path
	builder := ""
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if scenario.Setup != path {
				continue
			}
			found, err := w.builtOn(scenario.Key(), "")
			if err != nil {
				return err
			}
			builder = found
		}
	}
	w.askDelete(builder, builder+" builds on this scenario", "Delete "+filepath.Base(path)+"?",
		func() error {
			if err := w.app.trash(path); err != nil {
				return err
			}
			w.editor = nil
			return w.reload()
		})
	return nil
}

// previewable is true for what Preview can show: a scenario's block, or about.md.
func (e *Editor) previewable() bool {
	assert.That(e != nil, "previewable runs on an editor")
	assert.That(e.path != "", "an editor holds a file")
	return e.scenario != "" || filepath.Base(e.path) == "about.md"
}

// dirty is true while the editor holds changes that aren't saved.
func (e *Editor) dirty() bool {
	assert.That(e != nil, "dirty runs on an editor")
	assert.That(len(e.text) <= EditorTextBytesMax, "the text stays bounded")
	return e.text != e.opened
}

// closeEditor closes the editor and then runs next, asking first when changes aren't saved.
func (w *Window) closeEditor(next func() error) error {
	assert.That(w.editor != nil, "there's an editor to close")
	assert.That(next != nil, "something runs after closing")
	if !w.editor.dirty() {
		w.editor = nil
		return next()
	}
	question := "Close " + filepath.Base(w.editor.path) + "? Your changes aren't saved"
	w.ask(question, OfferDiscardChanges, func() error {
		w.editor = nil
		return next()
	})
	return nil
}

// chooseInEditor takes one of the editor's offers.
func (e *Editor) chooseInEditor(action string) error {
	assert.That(e != nil, "choose runs on an editor")
	assert.That(action != "", "an action has a label")
	switch action {
	case OfferKeepMine:
		if err := os.WriteFile(e.path, []byte(e.text), 0o644); err != nil {
			return err
		}
		e.opened, e.problems = e.text, []string{}
		return e.window.reload()
	case OfferUseTheirs:
		theirs, err := readEditable(e.path)
		if err != nil {
			return err
		}
		e.text, e.opened, e.problems = theirs, theirs, []string{}
		return nil
	case OfferCompare:
		theirs, err := readEditable(e.path)
		if err != nil {
			return err
		}
		e.problems = []string{MessageChangedOnDisk, "Theirs:\n" + theirs}
		return nil
	case OfferPreview:
		// Preview toggles: chosen again, it hides.
		e.previewing = !e.previewing
		return nil
	case OfferCloseEditor:
		return e.window.closeEditor(func() error { return nil })
	case OfferDeleteFile:
		return e.window.deleteFile()
	}
	return ErrNotImplemented
}

// Problems are what the parser can't read in the editor's text as it is now, unsaved: the
// marks a save would leave. Only a .feature file has them. The givens command checks them
// before it saves an edit made in $EDITOR. See capabilities/work-in-the-terminal.
func (e *Editor) Problems() []EditorMark {
	assert.That(e != nil, "problems runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	if strings.HasSuffix(e.path, ".sh") {
		return shellProblems(e.text)
	}
	if !strings.HasSuffix(e.path, ".feature") {
		return []EditorMark{}
	}
	if _, err := spec.ParseFeatureText(e.text); err != nil {
		line, message := spec.Explain(err)
		return []EditorMark{{Line: int(line), Message: capitalized(message)}}
	}
	return []EditorMark{}
}

// Line is the line the editor opened at, counting from 1, or 0 for the top of the file.
func (e *Editor) Line() int {
	assert.That(e != nil, "line runs on an editor")
	assert.That(e.line >= 0, "a line isn't negative")
	return e.line
}

// ChangedOnDisk is true while another tool has saved the file since the editor opened it, so
// Save would write nothing.
func (e *Editor) ChangedOnDisk() bool {
	assert.That(e != nil, "changed on disk runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return e.conflict()
}

// CloseEditor closes the editor without asking, whatever it holds unsaved: the givens
// command's own editing ends in the developer's editor, which saved or kept the text.
func (w *Window) CloseEditor() {
	assert.That(w != nil, "close editor runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.editor = nil
}

// shellProblems are the lines `bash -n` reports in a setup script or helper file's text, so a
// script that doesn't parse isn't saved from the developer's editor, as a .feature file that
// doesn't parse isn't. A check that can't run reports nothing: the file is saved as typed.
// Ref: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html (-n)
func shellProblems(text string) []EditorMark {
	assert.That(utf8.ValidString(text), "a script is text")
	assert.That(syntaxCheckTimeout > 0, "the check has a limit")
	ctx, cancel := context.WithTimeout(context.Background(), syntaxCheckTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", "-n")
	command.Stdin = strings.NewReader(text)
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return []EditorMark{}
	}
	marks := make([]EditorMark, 0, 2)
	for _, match := range bashError.FindAllStringSubmatch(string(output), helperProblemsMax) {
		// bash quotes the line it stopped on after its message, as a line of its own.
		line, convertErr := strconv.Atoi(match[1])
		if convertErr == nil && !strings.HasPrefix(match[2], "`") {
			marks = append(marks, EditorMark{Line: line, Message: capitalized(match[2])})
		}
	}
	return marks
}
