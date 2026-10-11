package app

import (
	"fmt"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/watch"
)

// Window is one Givens window: the welcome window, or one project.
type Window struct {
	// statuses are scenario statuses kept until files change or the day turns; see statusOf.
	statuses  map[string]string
	statusDay string
	helpers   []Helper // the Shared list, kept until files change; see Helpers

	app     *App
	id      int // numbered from 1 in opening order; see ID
	kind    string
	path    string           // the capabilities folder, or the folder opened when found is false
	found   bool             // path is a capabilities folder
	project *project.Project // nil when found is false
	watcher *watch.Watcher   // nil when found is false

	selected           string   // the name Select was given, or ""
	selectedCapability string   // the slug of the capability on show, or ""
	selectedScenario   string   // "<capability slug>/<name>" of the scenario on show, or ""
	selectedHelper     string   // "<file>:<name>" of the helper on show, or ""
	selectedView       string   // the view on show: "Error states", "Unverified", a tag, or ""
	editor             *Editor  // the file open in the editor, or nil
	prompt             *prompt  // the question on show, or nil
	session            *Session // the Drop in on show, or nil
	holdRefresh        bool     // a snapshot is being built, and has already looked
	search             *search  // the ⌘K panel while it's open, or nil
	selectedStep       string   // the step result on show, as its text, or ""
	folderPicker       bool     // "Open folder…" showed the folder picker
}

// SidebarItem is one line in a project window's sidebar.
type SidebarItem struct {
	Name    string `json:"name"`    // "Checkout", "Error states", "payments", "seed_shopper"
	Kind    string `json:"kind"`    // "capability", "view", "tag" or "helper"
	Count   int    `json:"count"`   // scenarios in a capability or view
	Warning string `json:"warning"` // why the item is marked, or ""
}

// Capability is one capability as a window shows it.
type Capability struct {
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	Story     []string   `json:"story"` // the As a, I want and So that lines
	About     string     `json:"about"` // about.md, rendered; "" when there is none
	Scenarios []Scenario `json:"scenarios"`
	Warning   string     `json:"warning"`
}

// Scenario is one scenario as a window shows it.
type Scenario struct {
	Name       string   `json:"name"`
	Slug       string   `json:"slug"`
	Capability string   `json:"capability"` // the capability's name
	Rule       string   `json:"rule"`       // the Rule: block it's under, or ""
	Error      bool     `json:"error"`      // tagged @error
	Tags       []string `json:"tags"`       // every tag, Feature tags included
	Steps      []string `json:"steps"`      // "Given …", "When …", "Then …", Background first
	Setup      string   `json:"setup"`      // the setup script's text, or ""
	Status     string   `json:"status"`     // "Verified today", "Never run", "Unverified: …"
	Warning    string   `json:"warning"`
}

// Kind is WindowWelcome or WindowProject.
func (w *Window) Kind() string {
	assert.That(w != nil, "kind runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.kind
}

// Title is the window's title: the capabilities folder's path for a project window.
func (w *Window) Title() string {
	assert.That(w != nil, "title runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.kind == WindowWelcome {
		return "Welcome to Givens"
	}
	return w.path
}

// Path is the capabilities folder a project window shows, or "" for the welcome window.
func (w *Window) Path() string {
	assert.That(w != nil, "path runs on a window")
	assert.That(len(w.path) <= PathLengthMax, "the path fits PathLengthMax")
	return w.path
}

// Sidebar lists capabilities with their counts, then views, tags and shared helpers.
func (w *Window) Sidebar() []SidebarItem {
	assert.That(w != nil, "sidebar runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	if w.project == nil {
		return []SidebarItem{}
	}
	items := make([]SidebarItem, 0, len(w.project.Capabilities)+8)
	for _, capability := range w.project.Capabilities {
		items = append(items, SidebarItem{
			Name: capability.Name, Kind: "capability", Count: len(capability.Scenarios),
			Warning: capabilityWarning(capability),
		})
	}
	return append(items, w.viewItems()...)
}

// Visible is every piece of text the window shows right now: messages, warnings, labels,
// story lines, steps, output lines and search results. It's what "they see" checks.
func (w *Window) Visible() []string {
	assert.That(w != nil, "visible runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return visibleLines(w.Snapshot())
}

// Capability returns a capability by its name.
func (w *Window) Capability(name string) (Capability, error) {
	assert.That(w != nil, "capability runs on a window")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	loaded, ok := w.findCapability(name)
	if !ok {
		return Capability{}, fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	return w.capabilityOf(loaded), nil
}

// Scenario returns a scenario by its name. Names are unique within the project's sample data;
// when two capabilities share a name, use "Capability/Scenario".
func (w *Window) Scenario(name string) (Scenario, error) {
	assert.That(w != nil, "scenario runs on a window")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	loaded, ok := w.findScenario(name)
	if !ok {
		return Scenario{}, fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	capability, _ := w.findCapability(loaded.Capability)
	return w.scenarioOf(capability, loaded), nil
}
