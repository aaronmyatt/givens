package app

import (
	"errors"
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/tags"
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

// SearchResultsMax bounds each kind's results.
const SearchResultsMax = 200

// SearchResult is one row of the ⌘K results list.
type SearchResult struct {
	Name       string   `json:"name"`       // a capability, scenario, step, script or helper
	Detail     string   `json:"detail"`     // the matching story line, script line or helper
	Capability string   `json:"capability"` // the capability it belongs to, or ""
	Uses       int      `json:"uses"`       // for a step: scenarios using it word for word
	Matches    []string `json:"matches"`    // each highlighted part of Name and Detail
}

// SearchGroup is one kind of result under its heading, best matches first.
type SearchGroup struct {
	Kind    string         `json:"kind"` // one of the Search* kinds
	Results []SearchResult `json:"results"`
}

// SearchResults is what the ⌘K panel shows right now.
type SearchResults struct {
	Query  string        `json:"query"`  // the text in the search box
	Filter string        `json:"filter"` // the chosen kind filter, or "" for every kind
	Groups []SearchGroup `json:"groups"` // only kinds with results, in Search* order
}

// search is the ⌘K panel's state.
type search struct {
	query  string
	filter string
}

// query is a search box's text split into a tag expression and words.
type query struct {
	expression tags.Expression
	tagged     bool   // the text holds a tag expression
	words      string // what's left, lower case, trimmed
	problem    string // why the tag expression doesn't parse, or ""
}

// readQuery splits a search box's text: tags and and, or, not and brackets make the tag
// expression; the rest are words to find.
func readQuery(text string) query {
	assert.That(len(text) <= NameLengthMax*4, "a query stays bounded")
	expression, words := make([]string, 0, 4), make([]string, 0, 4)
	for _, token := range strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").
		Replace(text)) {
		switch {
		case strings.HasPrefix(token, "@"), token == "and", token == "or", token == "not",
			token == "(", token == ")":
			expression = append(expression, token)
		default:
			words = append(words, token)
		}
	}
	read := query{tagged: len(expression) > 0, words: strings.ToLower(strings.Join(words, " "))}
	read.expression, read.problem = parseTags(expression)
	return read
}

// SearchResults is the ⌘K panel's query, filter and grouped results, or the zero value while
// search is closed. A query that matches nothing, or a tag expression that doesn't parse,
// has no groups. Needed by every scenario in capabilities/search-everything and the search
// scenarios in capabilities/tags.
func (w *Window) SearchResults() SearchResults {
	assert.That(w != nil, "search results run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.search == nil {
		return SearchResults{Groups: []SearchGroup{}}
	}
	if memo := w.searchMemo; memo != nil && memo.query == w.search.query &&
		memo.filter == w.search.filter {
		return memo.results
	}
	results := SearchResults{
		Query: w.search.query, Filter: w.search.filter, Groups: make([]SearchGroup, 0, 5),
	}
	defer func() {
		w.searchMemo = &searchMemo{query: results.Query, filter: results.Filter, results: results}
	}()
	read := readQuery(w.search.query)
	if w.project == nil || read.problem != "" || (read.words == "" && !read.tagged) {
		return results
	}
	for _, group := range w.searchGroups(read) {
		if len(group.Results) > 0 && (results.Filter == "" || results.Filter == group.Kind) {
			results.Groups = append(results.Groups, group)
		}
	}
	return results
}

// searchGroups finds every kind of result. A tag expression narrows the search to scenarios.
func (w *Window) searchGroups(read query) []SearchGroup {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(read.problem == "", "the query parsed")
	scenarios := SearchGroup{Kind: SearchScenarios, Results: w.findScenarios(read)}
	if read.tagged {
		return []SearchGroup{scenarios}
	}
	return []SearchGroup{
		{Kind: SearchStories, Results: w.findStories(read.words)},
		scenarios,
		{Kind: SearchSteps, Results: w.findSteps(read.words)},
		{Kind: SearchScripts, Results: w.findScripts(read.words)},
		{Kind: SearchHelpers, Results: w.findHelpers(read.words)},
	}
}

// FilterSearch shows only one kind of result, by its heading ("Scripts"), or every kind for "".
// Needed by "Show only one kind of result" in capabilities/search-everything.
func (w *Window) FilterSearch(kind string) error {
	assert.That(w != nil, "filter runs on a window")
	assert.That(len(kind) <= NameLengthMax, "the kind fits NameLengthMax")
	kinds := []string{"", SearchStories, SearchScenarios, SearchSteps, SearchScripts,
		SearchHelpers}
	if !slices.Contains(kinds, kind) {
		return errors.New(kind + " isn't a kind of search result")
	}
	if w.search == nil {
		return errors.New("search isn't open")
	}
	w.search.filter = kind
	return nil
}

// matchesIn lists each place word appears in the texts, as written there, in reading order.
func matchesIn(word string, texts ...string) []string {
	assert.That(word == strings.ToLower(word), "the word is lower case")
	assert.That(len(texts) > 0, "there's text to look in")
	matches := make([]string, 0, 2)
	if word == "" {
		return matches
	}
	for _, text := range texts {
		lower := strings.ToLower(text)
		// bound: each pass moves at past one match, and lower is finite.
		for at := strings.Index(lower, word); at >= 0; {
			matches = append(matches, text[at:at+len(word)])
			next := strings.Index(lower[at+len(word):], word)
			if next < 0 {
				break
			}
			at += len(word) + next
		}
	}
	return matches
}
