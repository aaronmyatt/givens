package app

import (
	"fmt"
	"os/exec"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// RequiredCommands lists the commands a scenario's setup needs, whether each is installed, and
// its install hint: the `need` lines in its script, the setups it builds on, and every helper
// those call. Needed by "See what a scenario needs installed" and by Drop in's check before
// anything runs.
func (w *Window) RequiredCommands(scenario string) ([]RequiredCommand, error) {
	assert.That(w != nil, "required commands runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	found, ok := w.findScenario(scenario)
	if !ok {
		return nil, fmt.Errorf("%s: %w", scenario, ErrNotFound)
	}
	texts, err := w.setupTexts(found)
	if err != nil {
		return nil, err
	}
	all := ""
	for _, text := range texts {
		all += text + "\n"
	}
	needs := helpers.Needs(all)
	required := make([]RequiredCommand, 0, len(needs))
	for _, need := range needs {
		// LookPath searches PATH as a shell would. Ref: https://pkg.go.dev/os/exec#LookPath
		_, missing := exec.LookPath(need.Command)
		required = append(required, RequiredCommand{
			Command: need.Command, Hint: need.Hint, Installed: missing == nil,
		})
	}
	return required, nil
}

// setupTexts is every script a scenario's setup runs: its own, the setups it builds on through
// from lines, and the bodies of the helpers each of those calls, nested calls included.
func (w *Window) setupTexts(scenario project.Scenario) ([]string, error) {
	assert.That(w.project != nil, "setup is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	shared, err := helpers.InFolder(w.path, "", "")
	if err != nil {
		return nil, err
	}
	texts := make([]string, 0, 8)
	queue, seen := []project.Scenario{scenario}, map[string]bool{scenario.Key(): true}
	// bound: a scenario joins the queue once, and the project has finitely many.
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.Setup)
		if err != nil {
			return nil, err
		}
		own, err := helpers.InFolder(w.path, current.Capability, current.Capability)
		if err != nil {
			return nil, err
		}
		visible := helpers.Visible(shared, own)
		texts = append(texts, text)
		for _, name := range helpers.Called(text, visible) {
			texts = append(texts, visible[name].Body)
		}
		for _, from := range helpers.Froms(text) {
			if built, err := w.project.Find(from.Key); err == nil && !seen[built.Key()] {
				seen[built.Key()] = true
				queue = append(queue, built)
			}
		}
	}
	return texts, nil
}
