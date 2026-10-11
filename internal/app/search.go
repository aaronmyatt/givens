package app

import (
	"unicode/utf8"

	"givens/internal/assert"
)

// Search result kinds, in the order the results list groups them.
// See capabilities/search-everything.
const (
	SearchStories   = "Stories"
	SearchScenarios = "Scenarios"
	SearchSteps     = "Steps"
	SearchScripts   = "Scripts"
	SearchHelpers   = "Helpers"
)

// SearchResult is one row of the ⌘K results list.
type SearchResult struct {
	Name       string   // the row's title: a capability, scenario, step, script file or helper
	Detail     string   // the row's second line: the matching story line, script line or helper
	Capability string   // the capability the result belongs to, or "" for a shared helper
	Uses       int      // for a step: how many scenarios use it word for word; 0 otherwise
	Matches    []string // each highlighted part of Name and Detail, in reading order
}

// SearchGroup is one kind of result under its heading, best matches first.
type SearchGroup struct {
	Kind    string // one of the Search* kinds
	Results []SearchResult
}

// SearchResults is what the ⌘K panel shows right now.
type SearchResults struct {
	Query  string        // the text in the search box
	Filter string        // the chosen kind filter, or "" for every kind
	Groups []SearchGroup // only kinds with results, in Search* order
}

// RuleGroup is one Rule: block of a capability, as the capability page groups it.
type RuleGroup struct {
	Name      string   // the text after "Rule:"
	Scenarios []string // the names of the scenarios under it, in file order
}

// SearchResults is the ⌘K panel's query, filter and grouped results, or the zero value while
// search is closed or the query matches nothing (including a tag expression that doesn't
// parse). Needed by every scenario in capabilities/search-everything and the search scenarios
// in capabilities/tags.
func (w *Window) SearchResults() SearchResults {
	assert.That(w != nil, "search results run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return SearchResults{}
}

// FilterSearch shows only one kind of result, by its heading ("Scripts"), or every kind for "".
// Needed by "Show only one kind of result" in capabilities/search-everything.
func (w *Window) FilterSearch(kind string) error {
	assert.That(w != nil, "filter runs on a window")
	assert.That(len(kind) <= NameLengthMax, "the kind fits NameLengthMax")
	return ErrNotImplemented
}

// ListedScenarios is the list the main pane shows for what's selected: a view ("Error states"),
// a tag ("payments"), or a step search result (the scenarios using that step). Needed by
// capabilities/tags, "List by tag" in capabilities/use-givens-from-the-terminal and "Find every
// use of a step" in capabilities/search-everything.
func (w *Window) ListedScenarios() []Scenario {
	assert.That(w != nil, "listed scenarios run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}

// RuleGroups lists a capability's Rule: blocks with the scenarios under each, in file order.
// Needed by "Rules group scenarios" in capabilities/gherkin-files.
func (w *Window) RuleGroups(capability string) ([]RuleGroup, error) {
	assert.That(w != nil, "rule groups run on a window")
	assert.That(utf8.ValidString(capability), "the capability's name is text")
	return nil, ErrNotImplemented
}
