package world

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"givens/internal/assert"
)

// The fake $EDITOR: a script in the scenario's folder that logs how it was called, then runs
// the next edit a step gave it on the file, as a developer would type it. With no edits left
// it changes nothing, as quitting without saving does. It's the developer's editor for every
// step, through EDITOR, as GIVENS_OPEN_COMMAND stands in for open.
const fakeEditorScript = `#!/bin/bash
# The fake editor. Its last argument is the file; the others, like +17, are logged.
set -euo pipefail
dir=%q
echo "$*" >> "$dir/editor.log"
count=$(( $(wc -l < "$dir/editor.log") ))
edit="$dir/edit-$count.sh"
if [ -f "$edit" ]; then bash "$edit" "${!#}"; fi
`

// FakeEditor makes the fake editor the scenario's $EDITOR, and gives it edits to make, one
// per time it opens: each a bash snippet whose $1 is the file. VISUAL is cleared, so EDITOR
// is the one used.
func (w *World) FakeEditor(edits ...string) error {
	assert.That(w != nil, "the editor belongs to a world")
	assert.That(len(edits) < 64, "a scenario gives a few edits")
	return w.FakeEditorNamed("fake-editor", edits...)
}

// EditorDir is the fake editor's folder: its log, and the edits it makes, as edit-N.sh.
func (w *World) EditorDir() string {
	assert.That(w != nil, "the editor belongs to a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	return filepath.Join(w.Dir, "editor")
}

// FakeEditorNamed is FakeEditor under another name, like vim, which opens at a line.
func (w *World) FakeEditorNamed(name string, edits ...string) error {
	assert.That(w != nil, "the editor belongs to a world")
	assert.That(name != "" && !strings.Contains(name, "/"), "the editor has a name")
	dir := w.EditorDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for index, edit := range edits {
		name := filepath.Join(dir, "edit-"+strconv.Itoa(w.edits+index+1)+".sh")
		if err := os.WriteFile(name, []byte(edit+"\n"), 0o644); err != nil {
			return err
		}
	}
	w.edits += len(edits)
	script := filepath.Join(dir, name)
	if err := os.WriteFile(script, []byte(fmt.Sprintf(fakeEditorScript, dir)), 0o755); err != nil {
		return err
	}
	if err := os.Unsetenv("VISUAL"); err != nil {
		return err
	}
	return os.Setenv("EDITOR", script)
}

// EditorCalls lists how the fake editor was called, one line per time it opened.
func (w *World) EditorCalls() ([]string, error) {
	assert.That(w != nil, "the editor belongs to a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	text, err := os.ReadFile(filepath.Join(w.Dir, "editor", "editor.log"))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(text), "\n"), "\n"), nil
}
