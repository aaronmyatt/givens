package serve

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"givens/internal/app"
	"givens/internal/assert"
)

// needs maps the methods in macos/Sources/GivensUI/Needs/*.swift, one capability's calls at a
// time, as each capability is wired into the UI shell.
func (s *Server) needs() map[string]handler {
	assert.That(s != nil, "needs runs on a server")
	assert.That(s.core != nil, "the server has an app")
	return map[string]handler{
		// capabilities/gherkin-files: Needs/gherkin-files.swift.
		"window.ruleGroups":    s.onWindow(ruleGroups),
		"window.edit":          s.onWindow(edit),
		"window.setEditorText": s.onWindow(setEditorText),
		"window.saveEditor":    s.onWindow(saveEditor),
		"window.rename":        s.onWindow(rename),
		// capabilities/share-setup-between-scenarios: Needs/share-setup-between-scenarios.swift.
		"window.helpers":          s.onWindow(sharedHelpers),
		"window.requiredCommands": s.onWindow(requiredCommands),
		"window.editingHelper":    s.onWindow(editingHelper),
		"window.newHelper":        s.onWindow(newHelper),
		"window.saveHelper":       s.onWindow(saveHelper),
		// capabilities/tags and search-everything: Needs/tags.swift, search-everything.swift.
		"window.searchResults":   s.onWindow(searchResults),
		"window.listedScenarios": s.onWindow(listedScenarios),
		"window.filterSearch":    s.onWindow(filterSearch),
	}
}

func ruleGroups(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "rule groups runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: rule groups needs a capability", ErrBadRequest)
	}
	return window.RuleGroups(given.Name)
}

func edit(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "edit runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Target == "" || filepath.IsAbs(given.Target) ||
		len(given.Target) > app.PathLengthMax {
		return nil, fmt.Errorf("%w: edit names a scenario or a relative path", ErrBadRequest)
	}
	_, err := window.Edit(given.Target)
	return nil, err
}

func setEditorText(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "set editor text runs on a window")
	assert.That(utf8.ValidString(given.Text), "JSON strings are text")
	if window.Editor() == nil {
		return nil, fmt.Errorf("%w: no file is open in the editor", ErrBadRequest)
	}
	if len(given.Text) > app.EditorTextBytesMax {
		return nil, fmt.Errorf("%w: the text is over %d bytes", ErrBadRequest,
			app.EditorTextBytesMax)
	}
	return nil, window.Editor().SetText(given.Text)
}

func saveEditor(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "save editor runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if window.Editor() == nil {
		return nil, fmt.Errorf("%w: no file is open in the editor", ErrBadRequest)
	}
	return nil, window.Editor().Save()
}

func rename(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "rename runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || given.NewName == "" || len(given.NewName) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: rename needs a name and a new name", ErrBadRequest)
	}
	return nil, window.Rename(given.Name, given.NewName)
}

func sharedHelpers(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "helpers runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return window.Helpers(), nil
}

func requiredCommands(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "required commands runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Scenario == "" || len(given.Scenario) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: required commands names a scenario", ErrBadRequest)
	}
	return window.RequiredCommands(given.Scenario)
}

func editingHelper(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "editing helper runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return window.EditingHelper(), nil
}

func newHelper(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "new helper runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Capability == "" || given.Name == "" || strings.ContainsAny(given.Name, "/ \n") {
		return nil, fmt.Errorf("%w: a helper needs a capability and a function name",
			ErrBadRequest)
	}
	return nil, window.NewHelper(given.Capability, given.Name)
}

func saveHelper(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "save helper runs on a window")
	assert.That(utf8.ValidString(given.Text), "JSON strings are text")
	return nil, window.SaveHelper(given.Text)
}

func searchResults(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "search results runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return window.SearchResults(), nil
}

func listedScenarios(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "listed scenarios runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return window.ListedScenarios(), nil
}

func filterSearch(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "filter search runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if len(given.Kind) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: a kind fits %d bytes", ErrBadRequest, app.NameLengthMax)
	}
	return nil, window.FilterSearch(given.Kind)
}
