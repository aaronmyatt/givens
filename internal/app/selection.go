package app

import (
	"fmt"
	"unicode/utf8"

	"givens/internal/assert"
)

// Select selects a sidebar item, scenario, search result or helper by its name. A capability
// shows its story and scenarios; a scenario shows its steps next to its capability's.
// See capabilities/read-a-capability.
func (w *Window) Select(name string) error {
	assert.That(w != nil, "select runs on a window")
	assert.That(utf8.ValidString(name), "the name is text")
	if capability, ok := w.findCapability(name); ok {
		w.selected, w.selectedCapability, w.selectedScenario = name, capability.Slug, ""
		return nil
	}
	if scenario, ok := w.findScenario(name); ok {
		w.selected, w.selectedCapability = name, scenario.Capability
		w.selectedScenario = scenario.Capability + "/" + scenario.Name
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

// shownCapability is the capability on show, or nil when none is selected or it's gone.
func (w *Window) shownCapability() *Capability {
	assert.That(w != nil, "shown capability runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.selectedCapability == "" {
		return nil
	}
	loaded, ok := w.findCapability(w.selectedCapability)
	if !ok {
		return nil
	}
	shown := capabilityOf(loaded)
	return &shown
}

// shownScenario is the scenario on show, or nil when none is selected or it's gone.
func (w *Window) shownScenario() *Scenario {
	assert.That(w != nil, "shown scenario runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.selectedScenario == "" {
		return nil
	}
	loaded, ok := w.findScenario(w.selectedScenario)
	if !ok {
		return nil
	}
	capability, _ := w.findCapability(loaded.Capability)
	shown := scenarioOf(capability, loaded)
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
