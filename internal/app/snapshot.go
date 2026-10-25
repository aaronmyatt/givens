package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"givens/internal/assert"
	"givens/internal/runs"
)

// The snapshot is the whole app as `givens serve` sends it after every call. Its JSON keys
// match macos/Sources/GivensUI/Models/Records.swift; Swift decodes every list as non-optional,
// so lists are never nil here.
// Ref: https://pkg.go.dev/encoding/json#Marshal

// AppSnapshot is every window, front first, and the welcome window's recent list.
type AppSnapshot struct {
	Windows []WindowSnapshot `json:"windows"`
	Recents []string         `json:"recents"`
}

// WindowSnapshot is one window as serve describes it. Records.swift's WindowSnapshot.
type WindowSnapshot struct {
	ID                string           `json:"id"`
	Kind              string           `json:"kind"`
	Title             string           `json:"title"`
	Path              string           `json:"path"`
	Messages          []string         `json:"messages"`
	Sidebar           []SidebarItem    `json:"sidebar"`
	Selected          string           `json:"selected"`
	Capability        *Capability      `json:"capability"`
	Scenario          *Scenario        `json:"scenario"`
	Search            *SearchSnapshot  `json:"search"`
	Editor            *EditorSnapshot  `json:"editor"`
	Session           *SessionSnapshot `json:"session"`
	Listed            []Scenario       `json:"listed"` // a view's, tag's or step's scenarios
	Rows              []DropInRow      `json:"rows"`   // the selected outline's Examples rows
	Runs              []RunLine        `json:"runs"`   // the shown scenario's runs, newest first
	Offers            []string         `json:"offers"`
	Labels            []string         `json:"labels"`
	FolderPickerShown bool             `json:"folderPickerShown"`
	// Branch is the branch a read-only window shows, or "" for the checked-out folder.
	Branch string `json:"branch"`
}

// SearchSnapshot is the ⌘K search. Records.swift's SearchRecord.
type SearchSnapshot struct {
	Query   string   `json:"query"`
	Results []string `json:"results"`
}

// EditorSnapshot is the editor pane. Records.swift's EditorRecord.
type EditorSnapshot struct {
	Path     string   `json:"path"`
	Text     string   `json:"text"`
	Problems []string `json:"problems"`
	Preview  []string `json:"preview"` // what "Preview" shows, when it's chosen
	Dirty    bool     `json:"dirty"`   // the text has changes that aren't saved
	// Scenario is the scenario the editor opened at, its block, or "" for a whole file.
	Scenario string `json:"scenario"`
}

// SessionSnapshot is the Drop in page. Records.swift's SessionRecord.
type SessionSnapshot struct {
	Scenario  string      `json:"scenario"`
	Status    string      `json:"status"`
	Output    []string    `json:"output"`
	Cards     []string    `json:"cards"`
	Checklist []CheckItem `json:"checklist"`
	// Parts are the checklist cut into runs of one kind, in order, as the page numbers them.
	Parts []ChecklistPart `json:"parts"`
	// Attachments are the paths of files "Record run" will keep with the run.
	Attachments []string `json:"attachments"`
	// Recorded is true once "Record run" has saved the checklist as it's marked now.
	Recorded bool `json:"recorded"`
}

// Snapshot is the whole app, front window first.
func (a *App) Snapshot() AppSnapshot {
	assert.That(a != nil, "snapshot runs on an app")
	assert.That(len(a.windows) <= WindowsMax, "windows stay bounded")
	snapshot := AppSnapshot{
		Windows: make([]WindowSnapshot, 0, len(a.windows)),
		Recents: append(make([]string, 0, RecentsMax), a.Recents()...),
	}
	for _, window := range a.windows {
		snapshot.Windows = append(snapshot.Windows, window.Snapshot())
	}
	return snapshot
}

// ID names the window for its life; serve's requests name windows by it.
func (w *Window) ID() string {
	assert.That(w != nil, "id runs on a window")
	assert.That(w.id > 0, "a window was numbered when it opened")
	return "w" + strconv.Itoa(w.id)
}

// Snapshot is what the window shows right now. Visible and the Swift views both read it.
func (w *Window) Snapshot() WindowSnapshot {
	assert.That(w != nil, "snapshot runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	w.holdRefresh = true
	defer func() { w.holdRefresh = false }()
	var editor *EditorSnapshot
	if w.editor != nil {
		editor = w.editor.snapshot()
	}
	messages, labels := make([]string, 0, 4), make([]string, 0, 4)
	if w.prompt != nil {
		messages = append(messages, w.prompt.message)
	}
	var session *SessionSnapshot
	if w.session != nil {
		session = w.session.snapshot()
		messages = append(messages, w.session.messages()...)
		labels = append(labels, w.session.hints...)
	} else if w.selectedScenario != "" {
		labels = append(labels, w.requirementLabels()...)
		if loaded, ok := w.findScenario(w.selectedScenario); ok && noSetup(loaded) {
			labels = append(labels, MessageNoSetup)
		}
	} else if w.selectedHelper != "" {
		labels = append(labels, w.helperLabels()...)
	}
	if w.kind == WindowWelcome {
		if recents := w.app.Recents(); len(recents) > 0 {
			labels = append(append(labels, LabelRecent), recents...)
		}
	} else if w.project == nil {
		messages = append(messages, MessageNoCapabilities)
	} else {
		messages = append(messages, w.kitMessages()...)
	}
	messages = append(append(messages, w.branchMessages()...), w.runsMessages()...)
	if empty := w.emptyView(); empty != "" {
		messages = append(messages, empty)
	}
	if w.selectedView == ViewShared {
		for _, helper := range w.ViewHelpers() {
			labels = append(labels, helper.Signature, helper.Description,
				fmt.Sprintf("%s, called by %d scripts", helper.File, helper.Callers))
		}
	}
	searching := w.searchSnapshot()
	if searching != nil {
		messages = append(messages, w.searchMessages()...)
	}
	return WindowSnapshot{
		ID: w.ID(), Kind: w.kind, Title: w.Title(), Path: w.Path(),
		Messages:   messages,
		Sidebar:    append(make([]SidebarItem, 0, 16), w.Sidebar()...),
		Selected:   w.Selected(),
		Capability: w.shownCapability(),
		Scenario:   w.shownScenario(),
		Editor:     editor,
		Session:    session,
		Search:     searching,
		Listed:     append(make([]Scenario, 0, 16), w.ListedScenarios()...),
		Rows:       w.DropInRows(),
		Runs:       w.shownHistory(),
		Offers:     append(make([]string, 0, 8), w.Offers()...),
		Branch:     w.branch,
		Labels:     labels,
	}
}

// visibleLines lists a snapshot's text in the order the views lay it out, the same order as
// WindowModel.visible in macos/Sources/GivensUI/Models/WindowModel.swift. Empty lines are
// left out.
func visibleLines(snapshot WindowSnapshot) []string {
	assert.That(snapshot.ID != "", "the snapshot is of a window")
	lines := append(append(make([]string, 0, 64), snapshot.Messages...), snapshot.Labels...)
	for _, item := range snapshot.Sidebar {
		lines = append(lines, item.Name, item.Name+" "+strconv.Itoa(item.Count), item.Warning)
	}
	if snapshot.Capability != nil {
		lines = append(lines, capabilityLines(*snapshot.Capability)...)
	}
	if snapshot.Scenario != nil {
		lines = append(lines, scenarioLines(*snapshot.Scenario)...)
	}
	for _, scenario := range snapshot.Listed {
		lines = append(lines, scenario.Name+" — "+scenario.Status, scenario.Capability)
	}
	for _, run := range snapshot.Runs {
		lines = append(lines, run.Text, run.Note)
	}
	for _, row := range snapshot.Rows {
		lines = append(lines, rowLine(row))
	}
	if snapshot.Search != nil {
		lines = append(lines, snapshot.Search.Results...)
	}
	if snapshot.Editor != nil {
		lines = append(append(lines, snapshot.Editor.Problems...), snapshot.Editor.Preview...)
	}
	if session := snapshot.Session; session != nil {
		lines = append(append(lines, session.Output...), session.Cards...)
		for _, item := range session.Checklist {
			lines = append(lines, item.Text)
		}
	}
	shown := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			shown = append(shown, line)
		}
	}
	return shown
}

// capabilityLines is a capability's text: name, warning, story, about.md, then scenarios,
// each Rule: block's name before its first scenario.
func capabilityLines(capability Capability) []string {
	assert.That(capability.Name != "", "a shown capability has a name")
	lines := append([]string{capability.Name, capability.Warning}, capability.Story...)
	rule := ""
	for _, scenario := range capability.Scenarios {
		// A Rule: block's name heads its scenarios, as the capability page groups them.
		if scenario.Rule != rule {
			lines, rule = append(lines, scenario.Rule), scenario.Rule
		}
		lines = append(lines, scenario.Name, scenario.Status, scenario.Warning)
	}
	// about.md comes after the scenarios, so a long one doesn't push them off the screen.
	lines = append(lines, capability.About)
	assert.That(len(lines) >= 3, "a capability shows its name, warning and about")
	return lines
}

// scenarioLines is a scenario's text: name, status, warning, tags on one line, steps, then
// the setup script beside them.
func scenarioLines(scenario Scenario) []string {
	assert.That(scenario.Name != "", "a shown scenario has a name")
	tags := ""
	for index, tag := range scenario.Tags {
		if index > 0 {
			tags += " "
		}
		tags += tag
	}
	lines := append([]string{scenario.Name, scenario.Status, scenario.Warning, tags},
		linedUp(scenario.Steps)...)
	lines = append(lines, scenario.Setup)
	assert.That(len(lines) >= 4, "a scenario shows its name, status, warning and tags")
	return lines
}

// requirementLabels say what the selected scenario's setup needs installed: "Requires bb", and
// for a missing command, "bb: Not installed. brew install borkdude/brew/babashka".
func (w *Window) requirementLabels() []string {
	assert.That(w != nil, "requirements run on a window")
	assert.That(w.selectedScenario != "", "a scenario is selected")
	required, err := w.RequiredCommands(w.selectedScenario)
	if err != nil {
		return []string{}
	}
	labels := make([]string, 0, 2*len(required))
	for _, command := range required {
		labels = append(labels, "Requires "+command.Command)
		if !command.Installed {
			labels = append(labels, command.Command+": Not installed. "+command.Hint)
		}
	}
	return labels
}

// helperLabels show the selected helper: its signature and description, where it's defined,
// and each script that calls it.
func (w *Window) helperLabels() []string {
	assert.That(w != nil, "helper labels run on a window")
	assert.That(w.selectedHelper != "", "a helper is selected")
	helper, ok := w.findHelper(w.selectedHelper)
	if !ok {
		return []string{}
	}
	labels := []string{helper.Signature, helper.Description, "Defined in " + helper.DefinedIn}
	return append(labels, helper.Callers...)
}

// searchSnapshot is the ⌘K panel as the window shows it: each group's heading, then each
// result's title and second line. nil while search is closed.
func (w *Window) searchSnapshot() *SearchSnapshot {
	assert.That(w != nil, "search snapshot runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.search == nil {
		return nil
	}
	results := w.SearchResults()
	lines := make([]string, 0, 32)
	for _, group := range results.Groups {
		lines = append(lines, group.Kind)
		for _, result := range group.Results {
			lines = append(lines, result.Name)
			if result.Detail != "" {
				lines = append(lines, result.Detail)
			}
		}
	}
	return &SearchSnapshot{Query: results.Query, Results: lines}
}

// searchMessages say why there are no results: a tag expression that doesn't parse, or
// nothing matching.
func (w *Window) searchMessages() []string {
	assert.That(w.search != nil, "search is open")
	assert.That(len(w.search.query) <= NameLengthMax*4, "the query stays bounded")
	if strings.TrimSpace(w.search.query) == "" {
		return []string{}
	}
	if read := readQuery(w.search.query); read.problem != "" {
		return []string{read.problem}
	}
	if len(w.SearchResults().Groups) == 0 {
		return []string{`Nothing matches "` + w.search.query + `"`}
	}
	return []string{}
}

// linedUp pads each step's keyword on the left to the longest keyword's width, so the step
// texts start in one column, as the scenario page shows them: "Given …", " When …".
func linedUp(steps []string) []string {
	assert.That(len(steps) <= SetupCardsMax*4, "steps stay bounded")
	width := 0
	for _, step := range steps {
		keyword, _, _ := strings.Cut(step, " ")
		width = max(width, len(keyword))
	}
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		keyword, text, _ := strings.Cut(step, " ")
		lines = append(lines, strings.Repeat(" ", width-len(keyword))+keyword+" "+text)
	}
	assert.That(len(lines) == len(steps), "each step is one line")
	return lines
}

// rowLine is an outline row as one line: its values in column-name order, then its status,
// like "card=expired — Never run". WindowModel.rowLine in Swift says it the same way.
func rowLine(row DropInRow) string {
	assert.That(len(row.Values) <= runs.FieldsMax, "a row stays bounded")
	assert.That(row.Status != "", "a row has a status")
	columns := make([]string, 0, len(row.Values))
	for column := range row.Values {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		values = append(values, column+"="+row.Values[column])
	}
	return strings.Join(values, ", ") + " — " + row.Status
}
