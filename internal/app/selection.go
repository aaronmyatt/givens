package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"givens/internal/assert"
)

// Select selects a sidebar item, scenario, search result or helper by its name. A capability
// shows its story and scenarios; a scenario shows its steps next to its capability's.
// See capabilities/read-a-capability. Selecting closes the editor and leaves an open Drop in,
// asking first when changes aren't saved or marks aren't recorded.
func (w *Window) Select(name string) error {
	assert.That(w != nil, "select runs on a window")
	assert.That(utf8.ValidString(name), "the name is text")
	w.branchProblem, w.runsNote = "", ""
	if w.editor != nil {
		return w.closeEditor(func() error { return w.leaveThenSelect(name) })
	}
	return w.leaveThenSelect(name)
}

// leaveThenSelect leaves an open Drop in, asking first when its marks aren't recorded, then
// selects by name.
func (w *Window) leaveThenSelect(name string) error {
	assert.That(w.editor == nil, "the editor is closed before selecting")
	assert.That(utf8.ValidString(name), "the name is text")
	if w.session == nil {
		return w.selectNow(name)
	}
	if !w.session.marked() || w.session.recorded {
		if err := w.leaveSession(); err != nil {
			return err
		}
		return w.selectNow(name)
	}
	question := "Leave " + w.session.scenario.Name + "? Its marks aren't recorded"
	w.ask(question, OfferLeave, func() error {
		if err := w.leaveSession(); err != nil {
			return err
		}
		return w.selectNow(name)
	})
	return nil
}

// leaveSession closes the Drop in page: setup still running is stopped, and its arguments
// folder removed.
func (w *Window) leaveSession() error {
	assert.That(w.session != nil, "there's a Drop in to leave")
	session := w.session
	assert.That(session.window == w, "the Drop in is this window's")
	w.session = nil
	session.scripts = nil
	if err := session.stopBlock(); err != nil {
		return err
	}
	session.removeArguments()
	if session.process == nil {
		return nil
	}
	return session.process.Stop()
}

// selectNow selects by name, with no Drop in open.
func (w *Window) selectNow(name string) error {
	assert.That(w.session == nil, "a Drop in is left before selecting")
	assert.That(utf8.ValidString(name), "the name is text")
	w.selectedView, w.selectedStep = "", ""
	if w.selectStep(name) {
		return nil
	}
	if capability, ok := w.findCapability(name); ok {
		w.selected, w.selectedCapability, w.selectedScenario = name, capability.Slug, ""
		w.selectedHelper = ""
		return nil
	}
	if scenario, ok := w.findScenario(name); ok {
		w.selected, w.selectedCapability = name, scenario.Capability
		w.selectedScenario, w.selectedHelper = scenario.Capability+"/"+scenario.Name, ""
		return nil
	}
	if w.selectView(name) {
		return nil
	}
	if w.selectHelper(name) {
		return nil
	}
	return fmt.Errorf("%s: %w", name, ErrNotFound)
}

// Selected is the name of what's selected, or "".
func (w *Window) Selected() string {
	assert.That(w != nil, "selected runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.selected
}

// shownScenario is the scenario on show, or nil when none is selected or it's gone.
func (w *Window) shownScenario() *Scenario {
	assert.That(w != nil, "shown scenario runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	name := w.selectedScenario
	if w.session != nil {
		// The Drop in page shows the scenario dropped in to, steps and setup script.
		name = w.session.scenario.Capability + "/" + w.session.scenario.Name
	}
	if name == "" {
		return nil
	}
	loaded, ok := w.findScenario(name)
	if !ok {
		return nil
	}
	capability, _ := w.findCapability(loaded.Capability)
	shown := w.scenarioOf(capability, loaded)
	return &shown
}

// RuleGroup is one Rule: block of a capability, as the capability page groups it.
type RuleGroup struct {
	Name      string   `json:"name"`      // the text after "Rule:"
	Scenarios []string `json:"scenarios"` // the names of the scenarios under it, in file order
}

// RuleGroups lists a capability's Rule: blocks with the scenarios under each, in file order.
// Needed by "Rules group scenarios" in capabilities/gherkin-files.
func (w *Window) RuleGroups(capability string) ([]RuleGroup, error) {
	assert.That(w != nil, "rule groups run on a window")
	assert.That(utf8.ValidString(capability), "the capability's name is text")
	loaded, ok := w.findCapability(capability)
	if !ok {
		return nil, fmt.Errorf("%s: %w", capability, ErrNotFound)
	}
	groups := make([]RuleGroup, 0, 4)
	for _, scenario := range loaded.Scenarios {
		if scenario.Rule == "" {
			continue
		}
		last := len(groups) - 1
		if last < 0 || groups[last].Name != scenario.Rule {
			groups = append(groups, RuleGroup{Name: scenario.Rule, Scenarios: []string{}})
			last++
		}
		groups[last].Scenarios = append(groups[last].Scenarios, scenario.Name)
	}
	return groups, nil
}

// selectStep selects a step from the search results by its name, "Then they see …", so the
// main pane lists the scenarios that use it.
func (w *Window) selectStep(name string) bool {
	assert.That(w != nil, "select step runs on a window")
	assert.That(name != "", "a step has a name")
	for _, group := range w.SearchResults().Groups {
		if group.Kind != SearchSteps {
			continue
		}
		for _, result := range group.Results {
			if result.Name == name {
				_, text, _ := strings.Cut(name, " ")
				w.selected, w.selectedStep = name, text
				w.selectedCapability, w.selectedScenario, w.selectedHelper = "", "", ""
				return true
			}
		}
	}
	return false
}
