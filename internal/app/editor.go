package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"givens/internal/assert"
)

// MessageChangedOnDisk is shown instead of saving over another tool's changes.
const MessageChangedOnDisk = "This file changed on disk"

// Edit opens a scenario, by name, at its block in its .feature file, or a file, by its path
// relative to the capabilities folder ("checkout/about.md"), in the editor. The editor holds
// the file's whole text, comments and all, so saving writes back only what was typed.
// Needed by the editing, saving and preview scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Edit(target string) (*Editor, error) {
	assert.That(w != nil, "edit runs on a window")
	assert.That(target != "", "edit names a scenario or a file")
	assert.That(!filepath.IsAbs(target), "a file is named relative to the capabilities folder")
	if w.project == nil {
		return nil, fmt.Errorf("%s: %w", target, ErrNotFound)
	}
	path := filepath.Join(w.path, target)
	if scenario, ok := w.findScenario(target); ok {
		capability, _ := w.findCapability(scenario.Capability)
		path = capability.FeaturePath
	}
	if !strings.HasPrefix(path, w.path+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s is outside the capabilities folder: %w", target, ErrNotFound)
	}
	text, err := readEditable(path)
	if err != nil {
		return nil, err
	}
	w.editor = &Editor{window: w, path: path, text: text, opened: text, problems: []string{},
		typing: -1, marks: []EditorMark{}}
	if scenario, ok := w.findScenario(target); ok {
		w.editor.scenario = scenario.Name
		w.editor.line = int(scenario.Line)
		w.editor.cursor = blockEnd(strings.Split(text, "\n"), int(scenario.Line)-1)
	}
	return w.editor, nil
}

// readEditable reads a file the editor can hold: text, within EditorTextBytesMax.
func readEditable(path string) (string, error) {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s: %w", filepath.Base(path), ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	if len(content) > EditorTextBytesMax || !utf8.Valid(content) {
		return "", fmt.Errorf("%s isn't a text file Givens can edit", filepath.Base(path))
	}
	assert.That(len(content) <= EditorTextBytesMax, "editable text stays bounded")
	return string(content), nil
}

// Editor is the editor open in the window, or nil when nothing is being edited.
func (w *Window) Editor() *Editor {
	assert.That(w != nil, "editor runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.editor
}

// Editing is the path of the file open in the window's editor, or "".
func (w *Window) Editing() string {
	assert.That(w != nil, "editing runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor == nil {
		return ""
	}
	return w.editor.path
}

// Path is the absolute path of the file the editor holds.
func (e *Editor) Path() string {
	assert.That(e != nil, "path runs on an editor")
	assert.That(e.path == "" || filepath.IsAbs(e.path), "an editor's path is absolute")
	return e.path
}

// Text is the editor's text, unsaved changes included.
func (e *Editor) Text() string {
	assert.That(e != nil, "text runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return e.text
}

// SetText replaces the editor's text without saving, as typing would.
func (e *Editor) SetText(text string) error {
	assert.That(e != nil, "set text runs on an editor")
	assert.That(utf8.ValidString(text), "the editor holds text")
	assert.That(len(text) <= EditorTextBytesMax, "the text fits EditorTextBytesMax")
	e.text = text
	return nil
}

// Save writes the editor's text to its file. The text is the whole file as edited, so only
// the lines that were changed differ. When the file changed on disk since it was opened, it
// shows MessageChangedOnDisk instead and writes nothing. Needed by "Saving changes only what
// was changed" and "The file changes on disk while it's being edited".
func (e *Editor) Save() error {
	assert.That(e != nil, "save runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	onDisk, err := readEditable(e.path)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if onDisk != e.opened {
		e.problems = []string{MessageChangedOnDisk}
		return nil
	}
	if err := os.WriteFile(e.path, []byte(e.text), 0o644); err != nil {
		return err
	}
	e.opened, e.problems = e.text, []string{}
	e.markParseErrors()
	return e.window.reload()
}

// snapshot is the editor as the window shows it.
func (e *Editor) snapshot() *EditorSnapshot {
	assert.That(e != nil, "snapshot runs on an editor")
	assert.That(e.problems != nil, "problems is a list")
	problems := append([]string{}, e.problems...)
	if e.conflict() && !slices.Contains(problems, MessageChangedOnDisk) {
		problems = append(problems, MessageChangedOnDisk)
	}
	for _, problem := range e.helperProblems {
		problems = append(problems, fmt.Sprintf("line %d: %s", problem.Line, problem.Message))
	}
	for _, mark := range e.marks {
		problems = append(problems, fmt.Sprintf("line %d: %s", mark.Line, mark.Message))
	}
	preview := make([]string, 0, 16)
	if e.previewing {
		for _, line := range e.Preview() {
			preview = append(preview, strings.TrimSpace(line.Text+" "+line.Mark))
		}
	}
	return &EditorSnapshot{Path: e.path, Text: e.text, Problems: problems, Preview: preview}
}
