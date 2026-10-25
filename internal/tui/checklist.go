package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
)

// Once setup is done, the Drop in page is a checklist: j and k (or the arrows) move, wrapping
// at the ends, and g and G jump to them. Enter marks a step done (a Then step passed), p
// passes, f fails, x clears. n writes the run's note in the developer's editor, a attaches a
// file, r records the run. An ask card takes what's typed. The core's Session keeps every
// mark.

// inputBytesMax bounds what's typed into an ask card or an attach path.
const inputBytesMax = 4096

// checklistItemsMax bounds the steps one checklist is expected to hold.
const checklistItemsMax = 1024

// input is a line being typed: an ask card's value, or the path of a file to attach.
type input struct {
	purpose string // "ask" or "attach"
	text    string
}

// noteMsg is the developer's editor closing on the run's note, held in file.
type noteMsg struct {
	file string
	err  error
}

// checklistKey takes a key the checklist or an input answers, and says whether it did.
func (m *Model) checklistKey(name string, text string, offers []string) (bool, tea.Cmd) {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(len(offers) < 64, "offers stay bounded")
	if m.dropIn.input == nil && slices.Contains(offers, app.OfferContinue) {
		m.dropIn.input = &input{purpose: "ask"}
	}
	if m.dropIn.input != nil {
		return true, m.inputKey(name, text)
	}
	if m.session.Status() != app.SessionReady {
		return false, nil
	}
	items := m.session.Checklist()
	m.dropIn.item = min(m.dropIn.item, max(len(items)-1, 0))
	switch name {
	case "j", "down", "k", "up", "g", "G":
		if len(items) > 0 {
			m.dropIn.item = cursorAfter(name, m.dropIn.item, len(items))
		}
	case "enter", "p", "f", "x":
		m.markItem(items, name)
	case "n":
		return true, m.editNote()
	case "a":
		m.dropIn.input = &input{purpose: "attach"}
	default:
		return false, nil
	}
	return true, nil
}

// markItem marks the checklist item under the cursor: Enter done (or passed, for a Then
// step), p passed, f failed, x cleared.
func (m *Model) markItem(items []app.CheckItem, key string) {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(key != "", "a key marks")
	if len(items) == 0 {
		return
	}
	item := items[m.dropIn.item]
	mark := map[string]string{"p": app.MarkPass, "f": app.MarkFail, "x": app.MarkClear}[key]
	if key == "enter" {
		mark = app.MarkDone
		if item.Kind == "then" {
			mark = app.MarkPass
		}
	}
	if err := m.session.Mark(item.Text, mark); err != nil {
		m.note = "✗ " + err.Error()
		return
	}
	if mark != app.MarkClear && m.dropIn.item < len(items)-1 {
		m.dropIn.item++
	}
}

// inputKey types into the line being typed; Enter gives it, Esc drops it.
func (m *Model) inputKey(name string, text string) tea.Cmd {
	assert.That(m.dropIn.input != nil, "a line is being typed")
	assert.That(len(m.dropIn.input.text) <= inputBytesMax, "what's typed stays bounded")
	line := m.dropIn.input
	switch {
	case name == "enter":
		m.dropIn.input = nil
		m.giveInput(line)
	case name == "esc":
		m.dropIn.input = nil
	case name == "backspace" && line.text != "":
		_, size := utf8.DecodeLastRuneInString(line.text)
		line.text = line.text[:len(line.text)-size]
	case text != "" && len(line.text)+len(text) <= inputBytesMax:
		line.text += text
	}
	return nil
}

// giveInput answers the ask card, or attaches the file.
func (m *Model) giveInput(line *input) {
	assert.That(line != nil, "a line was typed")
	assert.That(line.purpose == "ask" || line.purpose == "attach", "a line has a purpose")
	var err error
	if line.purpose == "ask" {
		if err = m.window.Type(line.text); err == nil {
			err = m.window.Choose(app.OfferContinue)
		}
	} else {
		err = m.session.Attach(strings.TrimSpace(line.text))
	}
	if err != nil {
		m.note = "✗ " + err.Error()
	}
}

// editNote opens the run's note, as it is so far, in the developer's editor.
func (m *Model) editNote() tea.Cmd {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(m.exec != nil, "the model can run the editor")
	file, err := os.CreateTemp("", "givens-*-note.md")
	if err == nil {
		_, err = file.WriteString(m.session.Note())
		err = errors.Join(err, file.Close())
	}
	if err != nil {
		m.note = "✗ " + err.Error()
		return nil
	}
	command := exec.Command("sh", "-c", `${VISUAL:-${EDITOR:-vi}} "$@"`, "sh", file.Name())
	return m.exec(command, func(err error) tea.Msg { return noteMsg{file: file.Name(), err: err} })
}

// noted takes the note back once the editor closed.
func (m *Model) noted(message noteMsg) {
	assert.That(message.file != "", "the note was in a file")
	assert.That(m != nil, "noted runs on a model")
	defer func() { _ = os.Remove(message.file) }()
	text, err := os.ReadFile(message.file)
	if err == nil && message.err == nil && m.session != nil {
		err = m.session.SetRunNote(strings.TrimRight(string(text), "\n"))
	}
	if err = errors.Join(err, message.err); err != nil {
		m.note = "✗ " + err.Error()
	}
}

// checklistView draws the scenario's Given steps, then the checklist with the cursor on one
// step, the note and attachments.
func (m *Model) checklistView(snapshot *app.SessionSnapshot) string {
	assert.That(snapshot != nil, "a Drop in is on show")
	assert.That(m.dropIn.item >= 0, "the cursor is on the list")
	page := ""
	if line := m.dropIn.input; line != nil {
		page += "\n> " + line.text + "▏\n"
	}
	page += "\n"
	for _, given := range m.session.Givens() {
		page += "      " + given + "\n" // under the checklist's text, past "  [ ] "
	}
	for index, item := range snapshot.Checklist {
		text := markBox(item.Mark) + " " + item.Text
		page += m.cursorLine(text, index == m.dropIn.item, m.width) + "\n"
	}
	if outcome := outcomeLine(snapshot.Checklist, snapshot.Recorded); outcome != "" {
		page += "\n" + outcome + "\n"
	}
	if note := m.session.Note(); note != "" {
		page += "\nNote: " + strings.SplitN(note, "\n", 2)[0] + "\n"
	}
	for _, path := range snapshot.Attachments {
		page += "Attached: " + path + "\n"
	}
	return page
}

// outcomeLine says how the Drop in went once every step is marked, like "1 step failed · r
// to record", or "✓ Run recorded: 1 step failed" once it's recorded; it's empty while a step
// is still unmarked.
func outcomeLine(items []app.CheckItem, recorded bool) string {
	assert.That(len(items) < checklistItemsMax, "a checklist stays bounded")
	failed := 0
	for _, item := range items {
		if item.Mark == "" {
			return ""
		}
		if item.Mark == app.MarkFail {
			failed++
		}
	}
	assert.That(failed <= len(items), "no more steps fail than there are")
	if len(items) == 0 {
		return ""
	}
	outcome := "all steps passed"
	if failed == 1 {
		outcome = "1 step failed"
	} else if failed > 1 {
		outcome = fmt.Sprintf("%d steps failed", failed)
	}
	if recorded {
		return "✓ Run recorded: " + outcome
	}
	return strings.ToUpper(outcome[:1]) + outcome[1:] + " · r to record"
}

// markBox draws a mark: [ ], [✓] done, [✓] passed, [✗] failed.
func markBox(mark string) string {
	assert.That(len(mark) < 16, "a mark is a word")
	assert.That(app.MarkDone != app.MarkPass, "done and passed are told apart")
	switch mark {
	case app.MarkDone, app.MarkPass:
		return "[✓]"
	case app.MarkFail:
		return "[✗]"
	}
	return "[ ]"
}

// checklistKeys are the checklist's keys, once setup is done.
func (m *Model) checklistKeys(offers []string) []string {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(len(offers) < 64, "offers stay bounded")
	if m.dropIn.input != nil {
		return []string{"enter give it", "esc drop it"}
	}
	if m.session.Status() != app.SessionReady {
		return nil
	}
	return []string{"j k g G move", "enter done", "p f pass/fail step", "x clear", "n note",
		"a attach"}
}
