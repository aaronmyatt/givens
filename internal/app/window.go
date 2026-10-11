package app

import (
	"unicode/utf8"

	"givens/internal/assert"
)

// Window is one Givens window: the welcome window, or one project.
type Window struct {
	app  *App
	kind string
	path string
}

// SidebarItem is one line in a project window's sidebar.
type SidebarItem struct {
	Name    string // "Checkout", "Error states", "payments", "seed_shopper"
	Kind    string // "capability", "view", "tag" or "helper"
	Count   int    // scenarios in a capability or view
	Warning string // why the item is marked, or ""
}

// Capability is one capability as a window shows it.
type Capability struct {
	Name      string
	Slug      string
	Story     []string // the As a, I want and So that lines
	About     string   // about.md, rendered; "" when there is none
	Scenarios []Scenario
	Warning   string
}

// Scenario is one scenario as a window shows it.
type Scenario struct {
	Name       string
	Slug       string
	Capability string   // the capability's name
	Error      bool     // tagged @error
	Tags       []string // every tag, Feature tags included
	Steps      []string // "Given …", "When …", "Then …", "And …", Background steps first
	Setup      string   // the setup script's text, or ""
	Status     string   // "Verified today", "Never run", "Last run failed", "Unverified: …"
	Warning    string
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
	return ""
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
	return nil
}

// Select selects a sidebar item, scenario, search result or helper by its name.
func (w *Window) Select(name string) error {
	assert.That(w != nil, "select runs on a window")
	assert.That(utf8.ValidString(name), "the name is text")
	return ErrNotImplemented
}

// Selected is the name of what's selected, or "".
func (w *Window) Selected() string {
	assert.That(w != nil, "selected runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return ""
}

// Visible is every piece of text the window shows right now: messages, warnings, labels,
// story lines, steps, output lines and search results. It's what "they see" checks.
func (w *Window) Visible() []string {
	assert.That(w != nil, "visible runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}

// Offers is every action the window offers right now, by its label: "Drop in", "Copy",
// "Trust", "Create script". It's what "they're offered" checks.
func (w *Window) Offers() []string {
	assert.That(w != nil, "offers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}

// Choose takes an offered action by its label.
func (w *Window) Choose(action string) error {
	assert.That(w != nil, "choose runs on a window")
	assert.That(action != "", "an action has a label")
	return ErrNotImplemented
}

// Type types text into whatever has focus: the search box, the editor, an ask card.
func (w *Window) Type(text string) error {
	assert.That(w != nil, "type runs on a window")
	assert.That(utf8.ValidString(text), "typed text is text")
	return ErrNotImplemented
}

// Press presses a key or shortcut: "⌘K", "⌘C", "⇧⌘C", "⌘↵", "⌘E", "↵".
func (w *Window) Press(keys string) error {
	assert.That(w != nil, "press runs on a window")
	assert.That(keys != "", "a shortcut names keys")
	return ErrNotImplemented
}

// Capability returns a capability by its name.
func (w *Window) Capability(name string) (Capability, error) {
	assert.That(w != nil, "capability runs on a window")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	return Capability{}, ErrNotImplemented
}

// Scenario returns a scenario by its name. Names are unique within the project's sample data;
// when two capabilities share a name, use "Capability/Scenario".
func (w *Window) Scenario(name string) (Scenario, error) {
	assert.That(w != nil, "scenario runs on a window")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	return Scenario{}, ErrNotImplemented
}

// DropIn drops in to a scenario: it runs the setup and opens the Drop in page.
func (w *Window) DropIn(scenario string) (*Session, error) {
	assert.That(w != nil, "drop in runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	return nil, ErrNotImplemented
}

// Editing is the path of the file open in the window's editor, or "".
func (w *Window) Editing() string {
	assert.That(w != nil, "editing runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return ""
}

// Session is the current Drop in, or nil.
func (w *Window) Session() *Session {
	assert.That(w != nil, "session runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}
