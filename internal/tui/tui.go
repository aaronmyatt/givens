// Package tui is the givens command's TUI (a text UI: full screen, keyboard driven): the Mac
// app's window, drawn in a terminal over the same internal/app Window and Session. It's a
// Bubble Tea model: Update takes key presses and ticks, View draws the window's snapshot.
// Nothing in internal/app knows it's here. See capabilities/work-in-the-terminal.
// Ref: https://github.com/charmbracelet/bubbletea
//
// Only Update touches the Window and the Session: tea.Cmds run on other goroutines, and the
// app has no locks. The commands this package returns only wait (a tick) or run the
// developer's editor.
package tui

import (
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
const (
	// TickInterval is how often a Drop in looks at setup: 20 looks a second.
	TickInterval = 50 * time.Millisecond
	// outputLinesShown bounds the setup output drawn at once; the rest scrolls.
	outputLinesShown = 18
)

// Exec runs a command with the terminal, as the developer's editor needs, and sends the
// callback's message when it ends. tea.ExecProcess by default; the acceptance steps run the
// command straight away, as they have no terminal.
type Exec func(command *exec.Cmd, done tea.ExecCallback) tea.Cmd

// mode is what the TUI shows.
type mode int

const (
	modeBrowse mode = iota // the project: capabilities, views and tags, and what's selected
	modeDropIn             // a Drop in's page
	modeSearch             // the search panel over the project
	modeStory              // a capability's story and about.md, to read
)

// Model is the TUI: one project window.
type Model struct {
	window  *app.Window
	exec    Exec
	mode    mode
	session *app.Session

	// What's said under the page until the next key: "Saved card-declined.sh", a problem.
	note   string
	ticks  int // ticks so far, which turn the spinner
	width  int
	height int
	plain  bool // drawing for Text: no styling

	browse browseState // see browse.go
	search searchState // see search.go
	story  storyState  // see story.go
	dropIn dropInState // see dropin.go
	editor *editing    // the file in the developer's editor, or nil; see edit.go
}

// tickMsg is a tick: a Drop in polls setup on it.
type tickMsg time.Time

// Tick is a tick message, for a driver with no Program, like the acceptance steps, which
// send their own in place of the ones Init and Update ask for.
func Tick() tea.Msg {
	now := time.Now()
	assert.That(!now.IsZero(), "a tick has a time")
	assert.That(TickInterval > 0, "ticks are spaced")
	return tickMsg(now)
}

// IsTick is true for a tick message, which a driver with no Program drops.
func IsTick(message tea.Msg) bool {
	assert.That(TickInterval > 0, "ticks are spaced")
	_, ok := message.(tickMsg)
	assert.That(!ok || message != nil, "a tick is a message")
	return ok
}

// New is the TUI on window, showing the project. exec runs the developer's editor; nil means
// tea.ExecProcess.
func New(window *app.Window, run Exec) *Model {
	assert.That(window != nil, "the TUI shows a window")
	assert.That(window.Kind() == app.WindowProject, "the TUI shows a project")
	if run == nil {
		run = tea.ExecProcess
	}
	model := &Model{window: window, exec: run, width: 100, height: 30}
	model.browse = newBrowse(window)
	return model
}

// Init asks for the first tick.
func (m *Model) Init() tea.Cmd {
	assert.That(m != nil, "init runs on a model")
	assert.That(m.window != nil, "the model has a window")
	return tick()
}

// tick waits one TickInterval. Ref: https://pkg.go.dev/charm.land/bubbletea/v2#Tick
func tick() tea.Cmd {
	assert.That(TickInterval > 0, "ticks are spaced")
	assert.That(TickInterval < time.Second, "a Drop in looks often")
	return tea.Tick(TickInterval, func(at time.Time) tea.Msg { return tickMsg(at) })
}

// Update takes one message: a key, a tick, the terminal's size, or the editor closing.
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	assert.That(m != nil, "update runs on a model")
	assert.That(m.window != nil, "the model has a window")
	switch message := message.(type) {
	case tickMsg:
		m.ticks++
		if m.mode == modeDropIn {
			m.pollDropIn()
		}
		return m, tick()
	case tea.WindowSizeMsg:
		m.width, m.height = max(message.Width, 40), max(message.Height, 10)
		m.browse.top = scrolledTop(m.browse.cursor, m.browse.top, m.listLines(), len(m.list()))
		return m, nil
	case editedMsg:
		return m, m.edited(message)
	case noteMsg:
		m.noted(message)
		return m, nil
	case namedMsg:
		return m, m.named(message)
	case tea.KeyPressMsg:
		return m.key(message.String(), message.Text)
	}
	return m, nil
}

// key handles one key press: name is the key's name, like "enter" or "p"; text is what it
// types, if anything.
func (m *Model) key(name string, text string) (tea.Model, tea.Cmd) {
	assert.That(m != nil, "key runs on a model")
	assert.That(name != "" || text != "", "a key has a name")
	m.note = ""
	if name == "ctrl+c" {
		return m, tea.Quit
	}
	var cmd tea.Cmd
	switch m.mode {
	case modeDropIn:
		cmd = m.dropInKey(name, text)
	case modeSearch:
		cmd = m.searchKey(name, text)
	case modeStory:
		cmd = m.storyKey(name)
	default:
		cmd = m.browseKey(name, text)
	}
	return m, cmd
}

// View draws the mode on show, with the window's messages and the keys that work under it.
func (m *Model) View() tea.View {
	assert.That(m != nil, "view runs on a model")
	assert.That(m.width >= 40, "the view has room")
	var page string
	switch m.mode {
	case modeDropIn:
		page = m.dropInView()
	case modeSearch:
		page = m.searchView()
	case modeStory:
		page = m.storyView()
	default:
		page = m.browseView()
	}
	if m.note != "" {
		page += "\n" + m.note + "\n"
	}
	view := tea.NewView(page)
	view.AltScreen = true
	view.WindowTitle = "Givens · " + m.window.Title()
	return view
}

// Text is everything View draws, as plain text with no styling: what the acceptance steps
// read.
func (m *Model) Text() string {
	assert.That(m != nil, "text runs on a model")
	assert.That(m.window != nil, "the model has a window")
	m.plain = true
	defer func() { m.plain = false }()
	return m.View().Content
}

// Session is the Drop in on show, or nil.
func (m *Model) Session() *app.Session {
	assert.That(m != nil, "session runs on a model")
	assert.That(m.mode != modeDropIn || m.session != nil, "a Drop in has a session")
	return m.session
}

// messages are the window's messages: setup failed, still running, a question.
func (m *Model) messages() []string {
	assert.That(m != nil, "messages run on a model")
	assert.That(m.window != nil, "the model has a window")
	return m.window.Snapshot().Messages
}

// lines joins lines as a page shows them, one per line.
func lines(parts ...[]string) string {
	assert.That(len(parts) < 64, "a page has a bounded number of parts")
	assert.That(outputLinesShown > 0, "a page shows some lines")
	var text strings.Builder
	for _, part := range parts {
		for _, line := range part {
			text.WriteString(line + "\n")
		}
	}
	return text.String()
}
