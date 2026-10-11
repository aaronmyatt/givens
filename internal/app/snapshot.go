package app

import (
	"strconv"

	"givens/internal/assert"
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
	Offers            []string         `json:"offers"`
	Labels            []string         `json:"labels"`
	FolderPickerShown bool             `json:"folderPickerShown"`
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
}

// SessionSnapshot is the Drop in page. Records.swift's SessionRecord.
type SessionSnapshot struct {
	Scenario  string      `json:"scenario"`
	Status    string      `json:"status"`
	Output    []string    `json:"output"`
	Cards     []string    `json:"cards"`
	Checklist []CheckItem `json:"checklist"`
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
	var editor *EditorSnapshot
	if w.editor != nil {
		editor = w.editor.snapshot()
	}
	messages := make([]string, 0, 4)
	if w.prompt != nil {
		messages = append(messages, w.prompt.message)
	}
	return WindowSnapshot{
		ID: w.ID(), Kind: w.kind, Title: w.Title(), Path: w.Path(),
		Messages:   make([]string, 0, 4),
		Sidebar:    append(make([]SidebarItem, 0, 16), w.Sidebar()...),
		Selected:   w.Selected(),
		Capability: w.shownCapability(),
		Scenario:   w.shownScenario(),
		Editor:     editor,
		Offers:     append(make([]string, 0, 8), w.Offers()...),
		Labels:     make([]string, 0, 4),
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
	if snapshot.Search != nil {
		lines = append(lines, snapshot.Search.Results...)
	}
	if snapshot.Editor != nil {
		lines = append(lines, snapshot.Editor.Problems...)
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
	lines = append(lines, capability.About)
	rule := ""
	for _, scenario := range capability.Scenarios {
		// A Rule: block's name heads its scenarios, as the capability page groups them.
		if scenario.Rule != rule {
			lines, rule = append(lines, scenario.Rule), scenario.Rule
		}
		lines = append(lines, scenario.Name, scenario.Status, scenario.Warning)
	}
	assert.That(len(lines) >= 3, "a capability shows its name, warning and about")
	return lines
}

// scenarioLines is a scenario's text: name, status, warning, tags on one line, then steps.
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
		scenario.Steps...)
	assert.That(len(lines) >= 4, "a scenario shows its name, status, warning and tags")
	return lines
}
