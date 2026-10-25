package app

import (
	"os"

	"givens/internal/assert"
)

// capabilityOf is a loaded capability as a window shows it.
func (w *Window) capabilityOf(loaded LoadedCapability) Capability {
	assert.That(loaded.Slug != "", "a capability has a slug")
	assert.That(loaded.Dir != "", "a capability has a folder")
	shown := Capability{
		Name: loaded.Name, Slug: loaded.Slug, Story: append([]string{}, loaded.Story...),
		Scenarios: make([]Scenario, 0, len(loaded.Scenarios)),
		Warning:   capabilityWarning(loaded),
	}
	if loaded.About != "" {
		if about, err := os.ReadFile(loaded.About); err == nil {
			shown.About = withImagePlaceholders(string(about), loaded.Dir)
		}
	}
	for _, scenario := range loaded.Scenarios {
		shown.Scenarios = append(shown.Scenarios, w.scenarioOf(loaded, scenario))
	}
	return shown
}

// scenarioOf is a loaded scenario as a window shows it.
func (w *Window) scenarioOf(capability LoadedCapability, loaded LoadedScenario) Scenario {
	assert.That(loaded.Capability != "", "a scenario belongs to a capability")
	assert.That(len(loaded.Steps) <= HelperFilesMax, "steps stay bounded")
	steps := make([]string, 0, len(loaded.Background)+len(loaded.Steps))
	for _, step := range append(append([]GherkinStep{}, loaded.Background...),
		loaded.Steps...) {
		steps = append(steps, step.Keyword+" "+step.Text)
	}
	shown := Scenario{
		Name: loaded.Name, Slug: loaded.Slug, Capability: capability.Name, Rule: loaded.Rule,
		Warning: scenarioWarning(capability, loaded),
		Error:   loaded.Error, Todo: loaded.Todo, Tags: append([]string{}, loaded.Tags...),
		Steps: steps,
	}
	if loaded.Setup != "" {
		if setup, err := os.ReadFile(loaded.Setup); err == nil {
			shown.Setup = string(setup)
		}
	}
	shown.Status = w.statusOf(loaded)
	return shown
}
