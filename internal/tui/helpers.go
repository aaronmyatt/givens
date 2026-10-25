package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
)

// The Helpers tab lists the project's shared helpers: the shell functions setup scripts call,
// from _shared folders. Beside the list, what one sets up, where it's defined and which
// scripts call it. Enter or e edits one at its line; n writes a new one.

// helperRows are the helpers, one row each: its name, where it's defined and its callers.
func (m *Model) helperRows() []row {
	assert.That(m.window != nil, "the model has a window")
	assert.That(tabs()[m.browse.tab][0] == "helper", "Helpers is on show")
	helpers := m.window.Helpers()
	rows := make([]row, 0, len(helpers))
	for _, helper := range helpers {
		rows = append(rows, row{helper: helper.DefinedIn + ":" + helper.Name, name: helper.Name,
			status: fmt.Sprintf("%s · called by %d", helper.DefinedIn, len(helper.Callers))})
	}
	assert.That(len(rows) == len(helpers), "a row per helper")
	return rows
}

// helperKey takes a key on the Helpers tab: Enter, e or l edits the helper under the cursor;
// n writes a new one.
func (m *Model) helperKey(rows []row, name string) tea.Cmd {
	assert.That(tabs()[m.browse.tab][0] == "helper", "Helpers is on show")
	assert.That(name != "", "a key has a name")
	if name == "n" {
		return m.newHelper()
	}
	picked, ok := m.current(rows)
	if !ok || (name != "enter" && name != "e" && name != "l" && name != "right") {
		return nil
	}
	err := m.window.Select(picked.helper)
	if err == nil {
		err = m.window.Choose(app.OfferEdit)
	}
	return m.beginEdit(m.window.Editor(), err)
}

// helperDetails is a helper as it shows beside the list.
func (m *Model) helperDetails(id string) string {
	assert.That(id != "", "a helper is named")
	assert.That(strings.Contains(id, ":"), "a helper's id is its file and name")
	for _, helper := range m.window.Helpers() {
		if helper.DefinedIn+":"+helper.Name != id {
			continue
		}
		text := "── " + helper.Signature + "\n" + helper.Description + "\n\n" +
			fmt.Sprintf("Defined in %s, line %d\n", helper.DefinedIn, helper.Line)
		if helper.Overrides != "" {
			text += helper.Overrides + "\n"
		}
		if helper.Warning != "" {
			text += "⚠ " + helper.Warning + "\n"
		}
		if len(helper.Callers) == 0 {
			return text + "No setup script calls it\n"
		}
		return text + "Called by:\n  " + strings.Join(helper.Callers, "\n  ") + "\n"
	}
	return ""
}

// newHelper asks for a new helper's name and folder in the developer's editor, as givens
// helper new takes them.
func (m *Model) newHelper() tea.Cmd {
	assert.That(m.exec != nil, "the model can run the editor")
	assert.That(m.editor == nil, "one file is edited at a time")
	file, err := os.CreateTemp("", "givens-*-helper.txt")
	if err == nil {
		_, err = file.WriteString("# A new helper: a shell function setup scripts call. Name " +
			"it, say where it goes, then save\n# and quit. In is a capability's folder, like " +
			"checkout, or _shared for every capability.\nName: \nIn: _shared\n")
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		m.note = "✗ " + err.Error()
		return nil
	}
	command := exec.Command("sh", "-c", `${VISUAL:-${EDITOR:-vi}} "$@"`, "sh", file.Name())
	return m.exec(command, func(err error) tea.Msg {
		return namedMsg{file: file.Name(), err: err}
	})
}

// namedHelper writes the new helper once it's named, and opens it at its function.
func (m *Model) namedHelper(fields map[string]string, err error) tea.Cmd {
	assert.That(fields != nil, "the template was read")
	assert.That(m.window != nil, "the model has a window")
	name, in := fields["Name"], fields["In"]
	if in == "_shared" {
		in = ""
	}
	if err == nil && name == "" {
		m.note = "No helper was written: its Name was empty"
		return nil
	}
	if err == nil {
		err = m.window.NewHelper(in, name)
	}
	if err != nil {
		m.note = "✗ No helper was written: " + err.Error()
		return nil
	}
	return m.beginEdit(m.window.Editor(), nil)
}
