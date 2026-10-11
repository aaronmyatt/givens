package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// Delete buttons.
const (
	OfferDelete       = "Delete"
	OfferDeleteAnyway = "Delete anyway"
)

// Delete asks to delete a scenario or a capability by its name, offering "Delete" and
// "Cancel", or "Delete anyway" when another setup builds on it. Choosing it removes the
// scenario's block and moves its setup files, or the capability's folder, to the Trash.
// Needed by the delete scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Delete(name string) error {
	assert.That(w != nil, "delete runs on a window")
	assert.That(name != "", "delete names what to delete")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	if capability, ok := w.findCapability(name); ok {
		builder, err := w.builtOn(capability.Slug+"/", capability.Slug)
		if err != nil {
			return err
		}
		run := func() error { return w.deleteCapability(capability) }
		w.askDelete(builder, builder+" builds on a scenario in "+capability.Name,
			"Delete "+capability.Name+"?", run)
		return nil
	}
	scenario, ok := w.findScenario(name)
	if !ok {
		return fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	builder, err := w.builtOn(scenario.Key(), "")
	if err != nil {
		return err
	}
	run := func() error { return w.deleteScenario(scenario) }
	w.askDelete(builder, builder+" builds on this scenario", "Delete "+scenario.Name+"?", run)
	return nil
}

// askDelete asks before a delete: "Delete anyway" when something builds on what's deleted.
func (w *Window) askDelete(builder string, builtMessage string, message string,
	run func() error) {
	assert.That(message != "", "a delete asks something")
	assert.That(run != nil, "a delete has something to run")
	if builder != "" {
		w.ask(builtMessage, OfferDeleteAnyway, run)
		return
	}
	w.ask(message, OfferDelete, run)
}

// builtOn names a scenario whose setup builds, through a from line, on the run key prefix,
// leaving out scenarios in the capability skip; "" when none does.
func (w *Window) builtOn(prefix string, skip string) (string, error) {
	assert.That(w.project != nil, "deletes happen in a loaded project")
	assert.That(prefix != "", "a key or prefix is given")
	for _, capability := range w.project.Capabilities {
		if capability.Slug == skip {
			continue
		}
		for _, scenario := range capability.Scenarios {
			if scenario.Setup == "" {
				continue
			}
			text, err := helpers.ReadScript(scenario.Setup)
			if err != nil {
				return "", err
			}
			for _, from := range helpers.Froms(text) {
				if from.Key == prefix || (strings.HasSuffix(prefix, "/") &&
					strings.HasPrefix(from.Key, prefix)) {
					return scenario.Name, nil
				}
			}
		}
	}
	return "", nil
}

// deleteScenario removes a scenario's block, with its tags and the blank line before it, and
// moves its setup files to the Trash.
func (w *Window) deleteScenario(scenario project.Scenario) error {
	assert.That(scenario.Line > 0, "a loaded scenario has a line")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	capability, ok := w.findCapability(scenario.Capability)
	if !ok {
		return fmt.Errorf("%s: %w", scenario.Capability, ErrNotFound)
	}
	text, err := readEditable(capability.FeaturePath)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	start := int(scenario.Line) - 1
	end := blockEnd(lines, start)
	first := start
	tag := regexp.MustCompile(`^\s*@`)
	// bound: first only falls, to 0 at most.
	for first > 0 && tag.MatchString(lines[first-1]) {
		first--
	}
	if first > 0 && strings.TrimSpace(lines[first-1]) == "" {
		first--
	}
	kept := append(append([]string{}, lines[:first]...), lines[end:]...)
	if err := os.WriteFile(capability.FeaturePath, []byte(strings.Join(kept, "\n")),
		0o644); err != nil {
		return err
	}
	for _, path := range []string{scenario.Setup, scenario.SetupCards, scenario.Teardown} {
		if path == "" {
			continue
		}
		if err := w.app.trash(path); err != nil {
			return err
		}
	}
	w.selected, w.selectedScenario = "", ""
	return w.reload()
}

// deleteCapability moves a capability's folder to the Trash.
func (w *Window) deleteCapability(capability project.Capability) error {
	assert.That(filepath.IsAbs(capability.Dir), "a capability's folder is absolute")
	assert.That(strings.HasPrefix(capability.Dir, w.path), "it's in this project")
	if err := w.app.trash(capability.Dir); err != nil {
		return err
	}
	if w.selectedCapability == capability.Slug {
		w.selected, w.selectedCapability, w.selectedScenario = "", "", ""
	}
	return w.reload()
}

// trash moves a path to the Trash. Under the Mac app the move happens a moment later, so
// until it's gone the path is pending and loads leave it out.
func (a *App) trash(path string) error {
	assert.That(a != nil, "trash runs on an app")
	assert.That(filepath.IsAbs(path), "a trashed path is absolute")
	if err := a.config.Trash.Move(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if a.pendingTrash == nil {
			a.pendingTrash = make(map[string]bool, 4)
		}
		a.pendingTrash[path] = true
	}
	return nil
}

// withoutTrashed leaves out of a loaded project what's waiting to go to the Trash, and
// forgets paths the Trash now has.
func (a *App) withoutTrashed(loaded *project.Project) {
	assert.That(a != nil, "without trashed runs on an app")
	assert.That(loaded != nil, "a project was loaded")
	for path := range a.pendingTrash {
		if _, err := os.Stat(path); err != nil {
			delete(a.pendingTrash, path)
		}
	}
	if len(a.pendingTrash) == 0 {
		return
	}
	kept := loaded.Capabilities[:0]
	for _, capability := range loaded.Capabilities {
		if a.pendingTrash[capability.Dir] {
			continue
		}
		for index := range capability.Scenarios {
			scenario := &capability.Scenarios[index]
			for _, path := range []*string{&scenario.Setup, &scenario.SetupCards,
				&scenario.Teardown} {
				if a.pendingTrash[*path] {
					*path = ""
				}
			}
		}
		kept = append(kept, capability)
	}
	loaded.Capabilities = kept
}
