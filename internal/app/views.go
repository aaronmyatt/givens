package app

import (
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/project"
)

// Views, under the capabilities in the sidebar. Each other tag is a view of its own, named
// after it without its @. See capabilities/views.
const (
	ViewErrorStates = "Error states"
	ViewUnverified  = "Unverified"
	ViewShared      = "Shared"
)

// selectView selects a view by name, if there's one: a fixed view or a tag's.
func (w *Window) selectView(name string) bool {
	assert.That(w != nil, "select view runs on a window")
	assert.That(name != "", "a view has a name")
	if name != ViewErrorStates && name != ViewUnverified && name != ViewShared &&
		!slices.Contains(w.tagViews(), name) {
		return false
	}
	w.selected, w.selectedView = name, name
	w.selectedCapability, w.selectedScenario, w.selectedHelper = "", "", ""
	return true
}

// tagViews lists the tags that are views, without their @, in the order first seen. @error
// and @timeout have meanings of their own, so they aren't views.
func (w *Window) tagViews() []string {
	assert.That(w != nil, "tag views run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	views := make([]string, 0, 8)
	if w.project == nil {
		return views
	}
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, tag := range scenario.Tags {
				name := strings.TrimPrefix(tag, "@")
				if isViewTag(tag) && !slices.Contains(views, name) {
					views = append(views, name)
				}
			}
		}
	}
	return views
}

// isViewTag is true for a tag that's a view: any but @error and @timeout:….
func isViewTag(tag string) bool {
	assert.That(strings.HasPrefix(tag, "@"), "a tag starts with @")
	assert.That(len(tag) <= NameLengthMax, "a tag fits NameLengthMax")
	return tag != project.TagError && !strings.HasPrefix(tag, "@timeout")
}

// ListedScenarios is the list the main pane shows for what's selected: a view ("Error
// states"), a tag ("payments"), or a step search result (the scenarios using that step).
// Needed by capabilities/tags, "List by tag" in capabilities/use-givens-from-the-terminal and
// "Find every use of a step" in capabilities/search-everything.
func (w *Window) ListedScenarios() []Scenario {
	assert.That(w != nil, "listed scenarios run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	listed := make([]Scenario, 0, 16)
	if w.project == nil || (w.selectedView == "" && w.selectedStep == "") ||
		w.selectedView == ViewShared {
		return listed
	}
	for _, capability := range w.project.Capabilities {
		for _, loaded := range capability.Scenarios {
			shown := w.scenarioOf(capability, loaded)
			if w.inView(shown) {
				listed = append(listed, shown)
			}
		}
	}
	return listed
}

// inView is true when a scenario belongs in the selected view.
func (w *Window) inView(scenario Scenario) bool {
	assert.That(w.selectedView != "" || w.selectedStep != "", "a view or step is selected")
	assert.That(scenario.Name != "", "the scenario has a name")
	if w.selectedStep != "" {
		for _, step := range scenario.Steps {
			if _, text, _ := strings.Cut(step, " "); text == w.selectedStep {
				return true
			}
		}
		return false
	}
	switch w.selectedView {
	case ViewErrorStates:
		return scenario.Error
	case ViewUnverified:
		return !strings.HasPrefix(scenario.Status, statusVerified)
	}
	return slices.Contains(scenario.Tags, "@"+w.selectedView)
}

// ViewHelper is one helper listed in the sidebar's "Shared" view.
type ViewHelper struct {
	Name        string // "seed_shopper"
	Signature   string // the text after "# @helper": "seed_shopper [email]"
	Description string // the comment lines under it, joined with spaces
	File        string // where it's defined, relative to the capabilities folder
	Callers     int    // how many setup scripts call it
}

// ViewHelpers lists the helpers the "Shared" view shows, when it's selected: every helper
// documented with a "# @helper" comment, in capabilities/_shared and each capability's
// _shared folder. Needed by "Browse helpers" in capabilities/views. The scenarios in the
// other views ("Error states", "Unverified", a tag) are listed by ListedScenarios.
func (w *Window) ViewHelpers() []ViewHelper {
	assert.That(w != nil, "view helpers run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}

// Sidebar kinds below the capabilities.
const (
	KindView = "view"
	KindTag  = "tag"
)

// viewItems are the sidebar's views, then one view per tag, each with how many it lists.
func (w *Window) viewItems() []SidebarItem {
	assert.That(w.project != nil, "views are of a loaded project")
	assert.That(!w.holdRefresh || w.watcher != nil, "a snapshot looks at a watched folder")
	errorStates, unverified := 0, 0
	tagCounts := make(map[string]int, 8)
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if scenario.Error {
				errorStates++
			}
			status := StatusOf(w.project, scenario, w.app.config.Clock.Now())
			if !strings.HasPrefix(status, statusVerified) {
				unverified++
			}
			for _, tag := range scenario.Tags {
				tagCounts[strings.TrimPrefix(tag, "@")]++
			}
		}
	}
	items := []SidebarItem{
		{Name: ViewErrorStates, Kind: KindView, Count: errorStates},
		{Name: ViewUnverified, Kind: KindView, Count: unverified},
		{Name: ViewShared, Kind: KindView, Count: len(w.Helpers())},
	}
	for _, tag := range w.tagViews() {
		items = append(items, SidebarItem{Name: tag, Kind: KindTag, Count: tagCounts[tag]})
	}
	return items
}
