package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/slug"
	"givens/internal/watch"
)

// capabilitiesFolderName is the folder Open looks for inside a project folder.
const capabilitiesFolderName = "capabilities"

// findCapabilities resolves what Open was given to a capabilities folder: the folder itself
// when it holds capabilities or is named "capabilities", else its "capabilities" subfolder.
// found is false when neither is there; path is then the absolute folder given.
func findCapabilities(path string) (string, bool, error) {
	assert.That(path != "", "a path is given")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, errors.New(absolute + " isn't a folder")
	}
	inside := filepath.Join(absolute, capabilitiesFolderName)
	if info, err := os.Stat(inside); err == nil && info.IsDir() {
		return inside, true, nil
	}
	if filepath.Base(absolute) == capabilitiesFolderName {
		return absolute, true, nil
	}
	loaded, err := project.Load(absolute)
	if err != nil {
		return "", false, err
	}
	return absolute, len(loaded.Capabilities) > 0, nil
}

// reload reads the window's capabilities folder again and starts watching it. A window with
// no capabilities folder has no project.
func (w *Window) reload() error {
	assert.That(w != nil, "reload runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has a project")
	if !w.found {
		w.project, w.watcher = nil, nil
		return nil
	}
	// The watcher's first look comes before the load, so a write in between is seen next time.
	watcher, err := watch.New(w.path, time.Now())
	if err != nil {
		return err
	}
	loaded, err := project.Load(w.path)
	if err != nil {
		return err
	}
	w.project, w.watcher = loaded, watcher
	return nil
}

// refresh reloads the project when another tool changed its files. Givens keeps no copy of
// its own, so every read of a window starts here. The watcher limits looks to one per
// watch.Interval of wall-clock time; the app's Clock is for dates, not for this.
// A folder that can't be read keeps showing what was last read.
func (w *Window) refresh() {
	assert.That(w != nil, "refresh runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.watcher == nil {
		return
	}
	changed, err := w.watcher.Changed(time.Now())
	if err != nil || !changed {
		return
	}
	if loaded, err := project.Load(w.path); err == nil {
		w.project = loaded
	}
}

// findCapability finds a capability by its name or slug.
func (w *Window) findCapability(name string) (project.Capability, bool) {
	assert.That(w != nil, "find runs on a window")
	assert.That(name != "", "a capability has a name")
	w.refresh()
	if w.project == nil {
		return project.Capability{}, false
	}
	for _, capability := range w.project.Capabilities {
		if capability.Name == name || capability.Slug == name {
			return capability, true
		}
	}
	return project.Capability{}, false
}

// findScenario finds a scenario by its name, "Capability/Scenario", or its run key.
func (w *Window) findScenario(name string) (project.Scenario, bool) {
	assert.That(w != nil, "find runs on a window")
	assert.That(name != "", "a scenario has a name")
	w.refresh()
	if w.project == nil {
		return project.Scenario{}, false
	}
	capabilityName, scenarioName, qualified := strings.Cut(name, "/")
	for _, capability := range w.project.Capabilities {
		if qualified && capability.Name != capabilityName && capability.Slug != capabilityName {
			continue
		}
		want := name
		if qualified {
			want = scenarioName
		}
		for _, scenario := range capability.Scenarios {
			if scenario.Name == want || scenario.Slug == slug.Of(want) {
				return scenario, true
			}
		}
	}
	return project.Scenario{}, false
}

// capabilityOf is a loaded capability as a window shows it.
func capabilityOf(loaded project.Capability) Capability {
	assert.That(loaded.Slug != "", "a capability has a slug")
	assert.That(loaded.Dir != "", "a capability has a folder")
	shown := Capability{
		Name: loaded.Name, Slug: loaded.Slug, Story: append([]string{}, loaded.Story...),
		Scenarios: make([]Scenario, 0, len(loaded.Scenarios)),
		Warning:   loaded.ParseError,
	}
	if loaded.About != "" {
		if about, err := os.ReadFile(loaded.About); err == nil {
			shown.About = string(about)
		}
	}
	for _, scenario := range loaded.Scenarios {
		shown.Scenarios = append(shown.Scenarios, scenarioOf(loaded.Name, scenario))
	}
	return shown
}

// scenarioOf is a loaded scenario as a window shows it.
func scenarioOf(capability string, loaded project.Scenario) Scenario {
	assert.That(loaded.Capability != "", "a scenario belongs to a capability")
	assert.That(len(loaded.Steps) <= project.HelperFilesMax, "steps stay bounded")
	steps := make([]string, 0, len(loaded.Background)+len(loaded.Steps))
	for _, step := range append(append([]project.Step{}, loaded.Background...),
		loaded.Steps...) {
		steps = append(steps, step.Keyword+" "+step.Text)
	}
	shown := Scenario{
		Name: loaded.Name, Slug: loaded.Slug, Capability: capability,
		Error: loaded.Error, Tags: append([]string{}, loaded.Tags...), Steps: steps,
	}
	if loaded.Setup != "" {
		if setup, err := os.ReadFile(loaded.Setup); err == nil {
			shown.Setup = string(setup)
		}
	}
	return shown
}
