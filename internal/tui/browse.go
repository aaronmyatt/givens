package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/app/model"
	"givens/internal/assert"
)

// The project: Tab moves between Capabilities, Views, Tags and Helpers, as the core's sidebar
// groups them. On Capabilities, Views and Tags, each is a heading over its scenarios and their
// status, folded away until Space (or Enter on a view or tag) unfolds it; z folds or unfolds
// them all. Enter on a capability opens its story to read. j and k move, wrapping at the
// ends; g and G jump to them. h goes up to the heading, then folds it; l unfolds, then goes
// in, then drops in; the arrows do the same. What's under the cursor shows beside the list: a
// scenario's steps, or a capability's story. On a scenario, under any heading, Enter drops in,
// e edits, s edits its setup, r shows its runs, n writes a new scenario in its capability; /
// searches.

// tabs are the sidebar's kinds, as Tab cycles through them.
func tabs() [][2]string {
	all := [][2]string{{"capability", "Capabilities"}, {"view", "Views"}, {"tag", "Tags"},
		{"helper", "Helpers"}}
	assert.That(len(all) == 4, "four tabs")
	assert.That(all[0][0] == "capability", "the list starts on capabilities")
	return all
}

// row is one line of the list: a heading, or a scenario under it.
type row struct {
	heading    string // a capability, view or tag, with its count; "" for a scenario
	key        string // a scenario's run key, like checkout/card-declined
	name       string
	status     string
	capability string // for a heading on Capabilities, the capability's name
	helper     string // for a helper on Helpers, its file and name: "_shared/users.sh:seed"
	sidebar    string // for a heading on Views or Tags, the item to select
	folded     bool   // a heading whose scenarios aren't listed under it
	todo       bool   // a scenario tagged @todo, flagged ⚑ in the list
}

// browseState is the project page's own state.
type browseState struct {
	tab      int
	cursor   int
	top      int               // the first row on show, once the list is longer than the page
	unfolded map[string]bool   // headings whose scenarios are listed, by foldID; none at first
	history  bool              // h: the scenario's runs show in place of its steps
	pickings []model.DropInRow // an outline's Examples rows to pick from, or nil
}

// newBrowse starts on Capabilities, with the cursor at the top.
func newBrowse(window *app.Window) browseState {
	assert.That(window != nil, "the TUI shows a window")
	assert.That(len(tabs()) == 4, "four tabs")
	return browseState{unfolded: make(map[string]bool, 16)}
}

// list is the rows of the tab on show.
func (m *Model) list() []row {
	assert.That(m.browse.tab >= 0 && m.browse.tab < len(tabs()), "a tab is on show")
	assert.That(m.window != nil, "the model has a window")
	kind := tabs()[m.browse.tab][0]
	if kind == "helper" {
		return m.helperRows()
	}
	rows := make([]row, 0, 32)
	for _, item := range m.window.Sidebar() {
		if item.Kind != kind {
			continue
		}
		heading := fmt.Sprintf("%s (%d)", item.Name, item.Count)
		if kind == "capability" {
			folded := !m.browse.unfolded[foldID(kind, item.Name)]
			rows = append(rows, row{heading: heading, capability: item.Name, folded: folded})
			if !folded {
				rows = append(rows, m.scenarioRows(item.Name)...)
			}
			continue
		}
		opened := m.browse.unfolded[foldID(kind, item.Name)]
		rows = append(rows, row{heading: heading, sidebar: item.Name, folded: !opened})
		if opened {
			// Not ListedScenarios: moving on to a scenario selects it, which unselects the view.
			rows = append(rows, listedRows(m.window.ScenariosIn(item.Name))...)
		}
	}
	return rows
}

// scenarioRows are a capability's scenarios.
func (m *Model) scenarioRows(capability string) []row {
	assert.That(capability != "", "a capability is named")
	assert.That(m.window != nil, "the model has a window")
	found, err := m.window.Capability(capability)
	if err != nil {
		return []row{}
	}
	return listedRows(found.Scenarios)
}

// listedRows are scenarios as rows.
func listedRows(scenarios []app.Scenario) []row {
	assert.That(len(scenarios) < 1<<16, "scenarios stay bounded")
	rows := make([]row, 0, len(scenarios))
	for _, scenario := range scenarios {
		rows = append(rows, row{key: scenarioKey(scenario), name: scenario.Name,
			status: scenario.Status, todo: scenario.Todo})
	}
	assert.That(len(rows) == len(scenarios), "a row per scenario")
	return rows
}

// scenarioKey is a scenario's run key, which the window finds it by.
func scenarioKey(scenario app.Scenario) string {
	assert.That(scenario.Slug != "", "a scenario has a slug")
	assert.That(scenario.Capability != "", "a scenario has a capability")
	return strings.ToLower(strings.ReplaceAll(scenario.Capability, " ", "-")) + "/" +
		scenario.Slug
}

// current is the row under the cursor, kept on the list.
func (m *Model) current(rows []row) (row, bool) {
	assert.That(m.browse.cursor >= 0, "the cursor is on the list")
	assert.That(len(rows) < 1<<16, "rows stay bounded")
	if len(rows) == 0 {
		return row{}, false
	}
	m.browse.cursor = min(m.browse.cursor, len(rows)-1)
	return rows[m.browse.cursor], true
}

// browseKey takes a key on the project page.
func (m *Model) browseKey(name string, text string) tea.Cmd {
	assert.That(m.mode == modeBrowse, "the project is on show")
	assert.That(name != "" || text != "", "a key has a name")
	if m.browse.pickings != nil {
		return m.pickRowKey(name)
	}
	if question, _ := m.window.Question(); question != "" {
		return m.questionKey(name)
	}
	rows := m.list()
	switch name {
	case "q", "esc":
		return tea.Quit
	case "tab":
		m.browse = browseState{tab: (m.browse.tab + 1) % len(tabs()), unfolded: m.browse.unfolded}
		return nil
	case "j", "down", "k", "up", "g", "home", "G", "end":
		if len(rows) > 0 {
			m.moveTo(rows, cursorAfter(name, m.browse.cursor, len(rows)))
		}
		return nil
	case "/":
		m.openSearch()
		return nil
	case "z":
		m.foldAll(rows)
		return nil
	}
	for _, pair := range windowOfferKeys() {
		if pair[0] == name && slices.Contains(m.window.Offers(), pair[1]) {
			if err := m.window.Choose(pair[1]); err != nil {
				m.note = "✗ " + err.Error()
			}
			return nil
		}
	}
	if tabs()[m.browse.tab][0] == "helper" {
		return m.helperKey(rows, name)
	}
	return m.rowKey(rows, name)
}

// windowOfferKeys are the keys for the core's offers about the whole project, shown under the
// list while they're offered: making a capabilities folder, and updating the kit.
func windowOfferKeys() [][2]string {
	keys := [][2]string{{"c", app.OfferCreateFolder}, {"u", app.OfferUpdateKit}}
	assert.That(len(keys) == 2, "two project offers have keys")
	assert.That(keys[0][0] != keys[1][0], "each has its own key")
	return keys
}

// moveTo puts the cursor on a row, and selects its scenario, so its steps show.
func (m *Model) moveTo(rows []row, index int) {
	assert.That(index >= 0, "the cursor is on the list")
	assert.That(index < len(rows) || len(rows) == 0, "the cursor is on the list")
	m.browse.cursor, m.browse.history = index, false
	m.browse.top = scrolledTop(index, m.browse.top, m.listLines(), len(rows))
	if picked, ok := m.current(rows); ok && picked.key != "" {
		if err := m.window.Select(picked.key); err != nil {
			m.note = "✗ " + err.Error()
		}
	}
}

// MoveTo puts the cursor on the scenario, or the view, tag or capability, named, as moving
// down the list to it would, and says whether it's listed. A scenario's capability unfolds.
func (m *Model) MoveTo(name string) bool {
	assert.That(m.mode == modeBrowse, "the project is on show")
	assert.That(name != "", "a scenario is named")
	if found, err := m.window.Scenario(name); err == nil {
		m.browse.unfolded[foldID("capability", found.Capability)] = true
	}
	rows := m.list()
	for index, listed := range rows {
		if listed.name == name || strings.HasPrefix(listed.heading, name+" (") {
			m.moveTo(rows, index)
			return true
		}
	}
	return false
}

// rowKey takes a key about the row under the cursor.
func (m *Model) rowKey(rows []row, name string) tea.Cmd {
	assert.That(m.mode == modeBrowse, "the project is on show")
	assert.That(name != "", "a key has a name")
	picked, ok := m.current(rows)
	if !ok {
		return nil
	}
	switch name {
	case "space":
		m.foldKey(rows)
		return nil
	case "h", "left":
		m.upKey(rows)
		return nil
	case "l", "right":
		m.inKey(rows)
		return nil
	}
	if picked.key == "" {
		if name == "enter" && picked.capability != "" {
			m.openStory(picked.capability)
		}
		if name == "enter" && picked.sidebar != "" {
			m.fold(rows, m.browse.cursor, false)
		}
		return nil
	}
	switch name {
	case "enter":
		m.enterScenario(picked.key)
	case "e":
		return m.editPath(picked.key)
	case "s":
		return m.editSetup(picked.key)
	case "r":
		m.browse.history = !m.browse.history
	case "n":
		return m.newScenario(picked.key)
	}
	return nil
}

// foldKey is Space: it folds or unfolds the heading the cursor is on, or folds the one it's
// under and puts the cursor on it.
func (m *Model) foldKey(rows []row) {
	assert.That(len(rows) > 0, "the list has a row")
	assert.That(m.browse.cursor < len(rows), "the cursor is on the list")
	index := headingAbove(rows, m.browse.cursor)
	m.fold(rows, index, index != m.browse.cursor || !rows[index].folded)
}

// upKey is h or ←: from a scenario, up to its heading; on a heading, fold it.
func (m *Model) upKey(rows []row) {
	assert.That(len(rows) > 0, "the list has a row")
	assert.That(m.browse.cursor < len(rows), "the cursor is on the list")
	index := headingAbove(rows, m.browse.cursor)
	if index != m.browse.cursor {
		m.moveTo(rows, index)
		return
	}
	if !rows[index].folded {
		m.fold(rows, index, true)
	}
}

// inKey is l or →: on a folded heading, unfold it; on an unfolded one, down to its first
// scenario; on a scenario, drop in, as Enter does.
func (m *Model) inKey(rows []row) {
	assert.That(len(rows) > 0, "the list has a row")
	assert.That(m.browse.cursor < len(rows), "the cursor is on the list")
	picked, next := rows[m.browse.cursor], m.browse.cursor+1
	switch {
	case picked.key != "":
		m.enterScenario(picked.key)
	case picked.folded:
		m.fold(rows, m.browse.cursor, false)
	case next < len(rows) && rows[next].key != "":
		m.moveTo(rows, next)
	}
}

// headingAbove is the index of the heading the row at index is, or is under.
func headingAbove(rows []row, index int) int {
	assert.That(index >= 0 && index < len(rows), "the row is on the list")
	assert.That(rows[0].key == "", "the list starts with a heading")
	// bound: index falls by one a pass, and the first row is a heading.
	for index > 0 && rows[index].key != "" {
		index--
	}
	return index
}

// foldAll folds every heading on the tab away (capabilities, views or tags), or, when all
// are folded already, lists them all. The cursor stays on the heading it was on, or under.
func (m *Model) foldAll(rows []row) {
	assert.That(m.browse.unfolded != nil, "folds are kept")
	assert.That(len(rows) < 1<<16, "rows stay bounded")
	if tabs()[m.browse.tab][0] == "helper" || len(rows) == 0 {
		return
	}
	heading := rows[headingAbove(rows, min(m.browse.cursor, len(rows)-1))].heading
	unfold := true
	for _, listed := range rows {
		if listed.heading != "" && !listed.folded {
			unfold = false
		}
	}
	for _, listed := range rows {
		if listed.heading == "" {
			continue
		}
		if unfold {
			m.browse.unfolded[m.headingID(listed)] = true
		} else {
			delete(m.browse.unfolded, m.headingID(listed))
		}
	}
	rows = m.list()
	for index, listed := range rows {
		if listed.heading == heading {
			m.moveTo(rows, index)
		}
	}
}

// foldID names a heading in the folds kept across Tab: its tab's kind and its name, so a tag
// and a capability that share a name fold apart.
func foldID(kind string, name string) string {
	assert.That(kind != "", "a heading is on a tab")
	assert.That(name != "", "a heading has a name")
	return kind + ":" + name
}

// headingID is a heading row's foldID on the tab on show.
func (m *Model) headingID(heading row) string {
	assert.That(heading.key == "", "a heading folds")
	assert.That(heading.capability != "" || heading.sidebar != "", "a heading has a name")
	if heading.capability != "" {
		return foldID("capability", heading.capability)
	}
	return foldID(tabs()[m.browse.tab][0], heading.sidebar)
}

// fold folds a heading's scenarios away, or lists them, and puts the cursor on it. Every tab
// folds alike: capabilities, views and tags.
func (m *Model) fold(rows []row, index int, folded bool) {
	assert.That(index >= 0 && index < len(rows), "the heading is on the list")
	assert.That(rows[index].key == "", "a heading folds")
	if folded {
		delete(m.browse.unfolded, m.headingID(rows[index]))
	} else {
		m.browse.unfolded[m.headingID(rows[index])] = true
	}
	rows = m.list()
	m.moveTo(rows, min(index, len(rows)-1))
}

// editSetup opens a scenario's setup in the developer's editor: its script, or its steps
// done by hand, or a new script when it has neither, as the core's Edit script, Edit setup
// and Create script do.
func (m *Model) editSetup(key string) tea.Cmd {
	assert.That(key != "", "a scenario is named")
	assert.That(m.mode == modeBrowse, "the project is on show")
	if err := m.window.Select(key); err != nil {
		m.note = "✗ " + err.Error()
		return nil
	}
	offers := m.window.Offers()
	for _, offer := range []string{app.OfferEditScript, app.OfferEditSetup,
		app.OfferCreateScript} {
		if slices.Contains(offers, offer) {
			err := m.window.Choose(offer)
			return m.beginEdit(m.window.Editor(), err)
		}
	}
	m.note = "✗ This scenario's setup can't be edited here"
	return nil
}

// enterScenario drops in to a scenario, or, for an outline, lists its rows to pick from.
func (m *Model) enterScenario(key string) {
	assert.That(key != "", "a scenario is named")
	assert.That(m.mode == modeBrowse, "the project is on show")
	if err := m.window.Select(key); err != nil {
		m.note = "✗ " + err.Error()
		return
	}
	if rows := m.window.DropInRows(); len(rows) > 0 {
		m.browse.pickings, m.browse.cursor = rows, 0
		return
	}
	if err := m.startDropIn(key); err != nil {
		m.note = "✗ " + err.Error()
	}
}

// pickRowKey moves through an outline's rows; Enter drops in to one.
func (m *Model) pickRowKey(name string) tea.Cmd {
	assert.That(m.browse.pickings != nil, "rows are being picked from")
	assert.That(m.browse.cursor >= 0, "the cursor is on the list")
	rows := m.browse.pickings
	switch name {
	case "j", "down", "k", "up", "g", "home", "G", "end":
		m.browse.cursor = cursorAfter(name, m.browse.cursor, len(rows))
	case "esc", "q", "h", "left":
		m.browse.pickings, m.browse.cursor = nil, 0
	case "enter", "l", "right":
		picked := rows[min(m.browse.cursor, len(rows)-1)]
		m.browse.pickings, m.browse.cursor = nil, 0
		for _, value := range picked.Values {
			if err := m.window.ChooseOnRow(value, app.OfferDropIn); err != nil {
				m.note = "✗ " + err.Error()
				return nil
			}
			break
		}
		m.session, m.mode, m.dropIn = m.window.Session(), modeDropIn, dropInState{}
		if m.session == nil {
			m.mode = modeBrowse
		}
	}
	return nil
}

// questionKey answers the question up in the window: y yes, n no.
func (m *Model) questionKey(name string) tea.Cmd {
	assert.That(m.window != nil, "the model has a window")
	assert.That(name != "", "a key has a name")
	offers := m.window.Offers()
	choice := map[string]int{"y": 0, "t": 0, "n": 1, "d": 1, "esc": 1}
	if index, ok := choice[name]; ok && index < len(offers) {
		if err := m.window.Choose(offers[index]); err != nil {
			m.note = "✗ " + err.Error()
		}
		if session := m.window.Session(); session != nil {
			m.session, m.mode, m.dropIn = session, modeDropIn, dropInState{}
		}
	}
	return nil
}

// newScenario writes a new scenario in the capability of the scenario under the cursor:
// its name typed in the developer's editor, as givens new scenario does.
func (m *Model) newScenario(key string) tea.Cmd {
	assert.That(key != "", "a scenario is named")
	assert.That(m.exec != nil, "the model can run the editor")
	capability, _, _ := strings.Cut(key, "/")
	file, err := os.CreateTemp("", "givens-*-scenario.txt")
	if err == nil {
		_, err = file.WriteString("# A new scenario in " + capability + ". Name it, then save " +
			"and quit.\nName: \n")
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
		return namedMsg{capability: capability, file: file.Name(), err: err}
	})
}

// namedMsg is the editor closing on a new scenario's name.
type namedMsg struct {
	capability string // the new scenario's capability; "" for a new helper
	file       string
	err        error
}

// named writes the new scenario once its name is typed, or the new helper.
func (m *Model) named(message namedMsg) tea.Cmd {
	assert.That(message.file != "", "the name was in a file")
	assert.That(m.window != nil, "the model has a window")
	defer func() { _ = os.Remove(message.file) }()
	text, err := os.ReadFile(message.file)
	if message.capability == "" {
		return m.namedHelper(fields(string(text)), errors.Join(err, message.err))
	}
	name := fields(string(text))["Name"]
	if err == nil && message.err == nil && name != "" {
		if err = m.window.Select(message.capability); err == nil {
			err = m.window.NewScenario(model.ScenarioDraft{Name: name})
		}
	}
	switch {
	case err != nil || message.err != nil:
		m.note = fmt.Sprintf("✗ No scenario was written: %v", firstError(err, message.err))
	case name == "":
		m.note = "No scenario was written: its Name was empty"
	default:
		m.note = "Created the scenario " + name + ", flagged @todo"
		m.window.CloseEditor()
	}
	return nil
}

// fields are the "Key: value" lines of a template the developer filled in, by key.
func fields(text string) map[string]string {
	assert.That(len(text) <= 1<<20, "a template stays bounded")
	found := make(map[string]string, 4)
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && !strings.HasPrefix(key, "#") && !strings.Contains(key, " ") {
			found[key] = strings.TrimSpace(value)
		}
	}
	assert.That(len(found) <= strings.Count(text, ":"), "each field has a colon")
	return found
}

// firstError is the first error that isn't nil.
func firstError(first error, second error) error {
	assert.That(first != nil || second != nil, "there's an error")
	assert.That(first == nil || first.Error() != "", "an error says what's wrong")
	if first != nil {
		return first
	}
	return second
}

// browseView draws the tabs, the list scrolled to keep the cursor on show, and what's under
// the cursor beside it.
func (m *Model) browseView() string {
	assert.That(m.mode == modeBrowse, "the project is on show")
	assert.That(m.window != nil, "the model has a window")
	if m.browse.pickings != nil {
		return m.pickRowView()
	}
	names := make([]string, 0, len(tabs()))
	for index, tab := range tabs() {
		if index == m.browse.tab {
			names = append(names, "["+tab[1]+"]")
		} else {
			names = append(names, " "+tab[1]+" ")
		}
	}
	page := m.window.Title() + "   " + strings.Join(names, " ") + "\n\n"
	rows := m.list()
	picked, _ := m.current(rows)
	room := m.listLines()
	top := scrolledTop(m.browse.cursor, m.browse.top, room, len(rows))
	shown := make([]string, 0, room)
	for index := top; index < min(top+room, len(rows)); index++ {
		shown = append(shown, m.rowLine(rows[index], index == m.browse.cursor))
	}
	if len(rows) > 0 {
		details := wrapped(m.details(picked), m.detailsWidth())
		page += m.besideOrUnder(shown, details, m.bodyLines(len(m.messages())))
	}
	page += "\n" + lines(m.messages())
	offered := make([]string, 0, 2)
	for _, pair := range windowOfferKeys() {
		if slices.Contains(m.window.Offers(), pair[1]) {
			offered = append(offered, pair[0]+" "+pair[1])
		}
	}
	if len(offered) > 0 {
		page += strings.Join(offered, " · ") + "\n"
	}
	return page + m.browseKeys(picked) + "\n"
}

// listLines is how many rows of the list are on show: the page's body beside the details,
// or two thirds of it over them.
func (m *Model) listLines() int {
	assert.That(m.width >= 40, "the view has room")
	assert.That(m.height > 0, "the terminal has a height")
	body := m.bodyLines(len(m.messages()))
	if m.width >= sideBySideColumnsMin {
		return body
	}
	return max(body*2/3, bodyLinesMin)
}

// rowLine draws a row: a heading with + when folded and − when listed, or a scenario, its
// name cut to nameColumns, and its status.
func (m *Model) rowLine(listed row, on bool) string {
	assert.That(listed.heading != "" || listed.key != "" || listed.helper != "",
		"a row is a heading, a scenario or a helper")
	assert.That(nameColumns > 0, "a name has room")
	flag := "    "
	if listed.todo {
		flag = "  ⚑ " // @todo: it needs work
	}
	text := flag + fit(listed.name, nameColumns) + " " + listed.status
	if listed.heading != "" {
		text = "− " + listed.heading
		if listed.folded {
			text = "+ " + listed.heading
		}
	}
	return fit(m.cursorLine(text, on, m.listWidth()), m.listWidth())
}

// details is what's under the cursor: a scenario's steps or runs, or a capability's story
// and the top of its about.md.
func (m *Model) details(picked row) string {
	assert.That(m.window != nil, "the model has a window")
	assert.That(picked.key == "" || picked.heading == "", "a row is one thing")
	if picked.key != "" {
		return strings.TrimPrefix(m.shownScenario(picked), "\n")
	}
	if picked.helper != "" {
		return m.helperDetails(picked.helper)
	}
	if picked.capability == "" {
		return ""
	}
	found, err := m.window.Capability(picked.capability)
	if err != nil {
		return "✗ " + err.Error()
	}
	text := "── " + found.Name + "\n" + strings.Join(found.Story, "\n") + "\n"
	if found.About != "" {
		text += "\n" + found.About
	}
	return text
}

// browseKeys are the keys that work on the row under the cursor.
func (m *Model) browseKeys(picked row) string {
	assert.That(len(tabs()) == 4, "four tabs")
	assert.That(picked.key == "" || picked.heading == "", "a row is one thing")
	keys := "enter l drop in · h up · e edit · s setup · r runs · n new"
	switch {
	case picked.key == "" && picked.heading == "" && picked.helper == "":
		return "tab views · / search · q quit" // nothing's listed
	case tabs()[m.browse.tab][0] == "helper":
		keys = "enter e edit · n new helper"
	case picked.capability != "":
		keys = "enter read the story · space h l fold"
	case picked.sidebar != "":
		keys = "enter list · space h l fold"
	}
	if tabs()[m.browse.tab][0] != "helper" {
		keys += " · z fold all"
	}
	return "tab views · j k g G move · " + keys + " · / search · q quit"
}

// shownScenario is the scenario under the cursor: its steps and setup script, or its runs.
func (m *Model) shownScenario(picked row) string {
	assert.That(m.window != nil, "the model has a window")
	if picked.key == "" {
		return ""
	}
	snapshot := m.window.Snapshot()
	if snapshot.Scenario == nil {
		return ""
	}
	page := "\n── " + snapshot.Scenario.Name + " · " + snapshot.Scenario.Status + "\n"
	if m.browse.history {
		if len(snapshot.Runs) == 0 {
			return page + "Never run\n"
		}
		for _, run := range snapshot.Runs {
			page += run.Text + "  " + run.Note + "\n"
		}
		return page
	}
	page += lines(snapshot.Scenario.Steps)
	scenario, err := m.window.Scenario(picked.key)
	if err == nil && scenario.Setup != "" {
		page += "Setup: " + picked.key[:strings.Index(picked.key, "/")] + "/scripts/" +
			picked.key[strings.Index(picked.key, "/")+1:] + ".sh\n"
	}
	// What its setup needs installed, like "Requires bb", or that it has no setup yet.
	return page + lines(snapshot.Labels)
}

// pickRowView lists an outline's rows to pick one from.
func (m *Model) pickRowView() string {
	assert.That(m.browse.pickings != nil, "rows are being picked from")
	assert.That(m.browse.cursor >= 0, "the cursor is on the list")
	page := "Pick a row to drop in to\n\n"
	for index, picked := range m.browse.pickings {
		values := make([]string, 0, len(picked.Values))
		for name, value := range picked.Values {
			values = append(values, name+"="+value)
		}
		text := strings.Join(values, " ") + "   " + picked.Status
		page += m.cursorLine(text, index == m.browse.cursor, m.width) + "\n"
	}
	return page + "\nj k g G move · enter l drop in · esc h back\n"
}
