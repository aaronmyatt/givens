package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// sharedFolder is where helpers live: _shared at the root, or in a capability's folder.
const sharedFolder = "_shared"

// Helpers is the "Shared" section of the sidebar: every documented helper in the project's
// _shared folders, _shared's first, in file order. A helper defined twice in one folder is
// flagged; a capability's helper that shadows a shared one says so.
func (w *Window) Helpers() []Helper {
	assert.That(w != nil, "helpers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	if w.project == nil {
		return []Helper{}
	}
	// Kept until a file changes: finding callers reads every script once per helper.
	if w.helpers != nil {
		return append([]Helper{}, w.helpers...)
	}
	shared, err := helpers.InFolder(w.path, "", "")
	if err != nil {
		return []Helper{}
	}
	listed := w.listHelpers(shared, "", nil)
	for _, capability := range w.project.Capabilities {
		own, err := helpers.InFolder(w.path, capability.Slug, capability.Slug)
		if err != nil {
			continue
		}
		listed = append(listed, w.listHelpers(own, capability.Name, shared)...)
	}
	w.helpers = listed
	return append([]Helper{}, listed...)
}

// listHelpers turns one folder's documented helpers into Shared list entries.
func (w *Window) listHelpers(found []helpers.Helper, capability string,
	shared []helpers.Helper) []Helper {
	assert.That(w.project != nil, "helpers are listed for a loaded project")
	assert.That(len(found) <= helpers.HelpersMax, "helpers stay bounded")
	listed := make([]Helper, 0, len(found))
	for _, helper := range found {
		if !helper.Documented() {
			continue
		}
		entry := Helper{
			Name: helper.Name, Signature: helper.Signature, Description: helper.Description,
			Capability: capability, DefinedIn: helper.File, Line: helper.Line,
			Callers: w.callersOf(helper),
		}
		for _, other := range found {
			if other.Name == helper.Name && other.File != helper.File {
				entry.Warning = "Also defined in " + other.File
			}
		}
		for _, other := range shared {
			if other.Name == helper.Name && entry.Overrides == "" {
				entry.Overrides = "Overrides " + other.File
			}
		}
		listed = append(listed, entry)
	}
	return listed
}

// callersOf lists, sorted, the setup scripts that call a helper by name: every capability's
// for a shared helper, or only its own capability's.
func (w *Window) callersOf(helper helpers.Helper) []string {
	assert.That(w.project != nil, "callers are found in a loaded project")
	assert.That(helper.Name != "", "a helper has a name")
	calls := regexp.MustCompile(`(?m)^[^#\n]*\b` + regexp.QuoteMeta(helper.Name) + `\b`)
	callers := make([]string, 0, 8)
	for _, capability := range w.project.Capabilities {
		if helper.Capability != "" && helper.Capability != capability.Slug {
			continue
		}
		for _, scenario := range capability.Scenarios {
			if scenario.Setup == "" {
				continue
			}
			text, err := helpers.ReadScript(scenario.Setup)
			if err != nil || !calls.MatchString(text) {
				continue
			}
			if relative, err := filepath.Rel(w.path, scenario.Setup); err == nil {
				callers = append(callers, filepath.ToSlash(relative))
			}
		}
	}
	sort.Strings(callers)
	return callers
}

// findHelper finds a documented helper in the Shared list by name, or by
// "<file relative to capabilities>:<name>" when two share it.
func (w *Window) findHelper(name string) (Helper, bool) {
	assert.That(w != nil, "find helper runs on a window")
	assert.That(name != "", "a helper has a name")
	for _, helper := range w.Helpers() {
		if helper.Name == name || helper.DefinedIn+":"+helper.Name == name {
			return helper, true
		}
	}
	return Helper{}, false
}

// elsewhere says where a helper the setup couldn't find is defined, when another capability's
// _shared folder has it: "mock_payments is defined in checkout/_shared/payments.sh, which only
// Checkout's scripts can call". It's "" when no other capability defines it.
func (w *Window) elsewhere(scenario project.Scenario, output []string) string {
	assert.That(w.project != nil, "helpers are found in a loaded project")
	assert.That(len(output) <= 1<<20, "output stays bounded")
	for _, line := range output {
		before, found := strings.CutSuffix(line, ": command not found")
		if !found {
			continue
		}
		missing := before[strings.LastIndex(before, " ")+1:]
		for _, capability := range w.project.Capabilities {
			if capability.Slug == scenario.Capability {
				continue
			}
			own, err := helpers.InFolder(w.path, capability.Slug, capability.Slug)
			if err != nil {
				continue
			}
			for _, helper := range own {
				if helper.Name == missing {
					return fmt.Sprintf("%s is defined in %s, which only %s's scripts can call",
						missing, helper.File, capability.Name)
				}
			}
		}
	}
	return ""
}
