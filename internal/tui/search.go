package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
)

// "/" searches everything, as ⌘K does in the app: stories, scenarios, steps, scripts and
// helpers, grouped. Typing narrows it; ↑ and ↓ move, wrapping at the ends; Enter on a
// scenario shows it; Esc closes.

// searchState is the search panel's own state.
type searchState struct {
	query  string
	cursor int
}

// openSearch opens the search panel.
func (m *Model) openSearch() {
	assert.That(m.mode == modeBrowse, "search opens over the project")
	assert.That(m.window != nil, "the model has a window")
	if err := m.window.Press(app.KeySearch); err != nil {
		m.note = "✗ " + err.Error()
		return
	}
	m.mode, m.search = modeSearch, searchState{}
}

// found is every result, in the order they're drawn: kind, then name.
func (m *Model) found() [][2]string {
	assert.That(m.mode == modeSearch, "search is open")
	assert.That(m.window != nil, "the model has a window")
	results := make([][2]string, 0, 16)
	for _, group := range m.window.SearchResults().Groups {
		for _, result := range group.Results {
			results = append(results, [2]string{group.Kind, result.Name})
		}
	}
	return results
}

// searchKey takes a key in the search panel.
func (m *Model) searchKey(name string, text string) tea.Cmd {
	assert.That(m.mode == modeSearch, "search is open")
	assert.That(len(m.search.query) <= app.NameLengthMax*4, "the query stays bounded")
	results := m.found()
	switch {
	case name == "esc":
		m.closeSearch()
	case (name == "down" || name == "up") && len(results) > 0:
		m.search.cursor = cursorAfter(name, m.search.cursor, len(results))
	case name == "enter" && m.search.cursor < len(results):
		m.showResult(results[m.search.cursor])
	case name == "backspace" && m.search.query != "":
		_, size := utf8.DecodeLastRuneInString(m.search.query)
		m.retype(m.search.query[:len(m.search.query)-size])
	case text != "":
		m.retype(m.search.query + text)
	}
	return nil
}

// retype puts query in the search box, as the app's box takes typing.
func (m *Model) retype(query string) {
	assert.That(utf8.ValidString(query), "a query is text")
	assert.That(m.mode == modeSearch, "search is open")
	if err := m.window.Press(app.KeySearch); err == nil && query != "" {
		err = m.window.Type(query)
		if err != nil {
			m.note = "✗ " + err.Error()
		}
	}
	m.search = searchState{query: query}
}

// closeSearch closes the panel, back to the project.
func (m *Model) closeSearch() {
	assert.That(m.mode == modeSearch, "search is open")
	assert.That(m.window != nil, "the model has a window")
	if err := m.window.Press(app.KeyEscape); err != nil {
		m.note = "✗ " + err.Error()
	}
	m.mode = modeBrowse
}

// showResult closes the panel and shows a scenario result in the list.
func (m *Model) showResult(result [2]string) {
	assert.That(result[0] != "", "a result has a kind")
	assert.That(result[1] != "", "a result has a name")
	assert.That(m.mode == modeSearch, "search is open")
	m.closeSearch()
	if result[0] != app.SearchScenarios || !m.MoveTo(result[1]) {
		m.note = result[0] + ": " + result[1]
	}
}

// searchView draws the box and the results.
func (m *Model) searchView() string {
	assert.That(m.mode == modeSearch, "search is open")
	assert.That(m.search.cursor >= 0, "the cursor is on the list")
	page := "Search: " + m.search.query + "▏\n\n"
	kind := ""
	for index, result := range m.found() {
		if result[0] != kind {
			kind = result[0]
			page += kind + "\n"
		}
		page += m.cursorLine("  "+result[1], index == m.search.cursor, m.width) + "\n"
	}
	if strings.TrimSpace(m.search.query) != "" && len(m.found()) == 0 {
		page += lines(m.messages())
	}
	return page + "\ntype to search · ↑ ↓ move · enter show · esc close\n"
}
