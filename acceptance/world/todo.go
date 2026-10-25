package world

import (
	"fmt"

	"givens/internal/app"
	"givens/internal/assert"
)

// ShownFlagged says whether the scenario called name is flagged @todo where the front
// window shows it: in the capability's list, on its own page, or in a view's list. It fails
// when the window doesn't show it at all. See "A scenario flagged to do" in capabilities/tags.
func (w *World) ShownFlagged(name string) (bool, error) {
	assert.That(w != nil, "shown flagged runs on a world")
	assert.That(name != "", "a scenario has a name")
	window, err := w.Front()
	if err != nil {
		return false, err
	}
	snapshot := window.Snapshot()
	shown := append(make([]app.Scenario, 0, 16), snapshot.Listed...)
	if snapshot.Capability != nil {
		shown = append(shown, snapshot.Capability.Scenarios...)
	}
	if snapshot.Scenario != nil {
		shown = append(shown, *snapshot.Scenario)
	}
	for _, scenario := range shown {
		if scenario.Name == name {
			return scenario.Todo, nil
		}
	}
	return false, fmt.Errorf("the window doesn't show %q; it shows %q", name, snapshot.Selected)
}
