package serve

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
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
		// capabilities/drop-into-a-scenario: Needs/drop-into-a-scenario.swift.
		"session.cards":           s.onSession(cards),
		"session.chooseOnCard":    s.onSession(chooseOnCard),
		"session.copyCodeBlock":   s.onSession(copyCodeBlock),
		"session.clickInlineCode": s.onSession(clickInlineCode),
		"session.setRunNote":      s.onSession(setRunNote),
		"window.dropInRows":       s.onWindow(dropInRows),
		"window.chooseOnRow":      s.onWindow(chooseOnRow),
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

func cards(session *app.Session, _ params) (any, error) {
	assert.That(session != nil, "cards runs on a session")
	assert.That(session.Status() != "", "a session has a status")
	return session.Cards(), nil
}

func chooseOnCard(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "choose on card runs on a session")
	assert.That(app.SetupCardsMax > 0, "cards have a limit")
	card, err := indexOf(given.Card, app.SetupCardsMax)
	if err != nil {
		return nil, err
	}
	if given.Action == "" || len(given.Action) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: choose on card needs an action", ErrBadRequest)
	}
	return nil, session.ChooseOnCard(card, given.Action)
}

func copyCodeBlock(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "copy code block runs on a session")
	assert.That(app.CodeBlocksMax > 0, "blocks have a limit")
	card, err := indexOf(given.Card, app.SetupCardsMax)
	if err != nil {
		return nil, err
	}
	block, err := indexOf(given.Block, app.CodeBlocksMax)
	if err != nil {
		return nil, err
	}
	return nil, session.CopyCodeBlock(card, block)
}

func clickInlineCode(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "click inline code runs on a session")
	assert.That(app.SetupCardsMax > 0, "cards have a limit")
	card, err := indexOf(given.Card, app.SetupCardsMax)
	if err != nil {
		return nil, err
	}
	if given.Code == "" || len(given.Code) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: inline code has text", ErrBadRequest)
	}
	return nil, session.ClickInlineCode(card, given.Code)
}

func setRunNote(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "set run note runs on a session")
	assert.That(utf8.ValidString(given.Note), "JSON strings are text")
	if len(given.Note) > app.NameLengthMax || strings.Contains(given.Note, "\n") {
		return nil, fmt.Errorf("%w: a note is one line of %d bytes at most", ErrBadRequest,
			app.NameLengthMax)
	}
	return nil, session.SetRunNote(given.Note)
}

func dropInRows(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "drop in rows runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return window.DropInRows(), nil
}

func chooseOnRow(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "choose on row runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Row == "" || given.Action == "" || len(given.Row) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: choose on row names a row and an action", ErrBadRequest)
	}
	return nil, window.ChooseOnRow(given.Row, given.Action)
}

// indexOf reads an index sent as a number or a number in a string, from 0 to below max.
func indexOf(raw json.RawMessage, max int) (int, error) {
	assert.That(max > 0, "indexes have a limit")
	assert.That(len(raw) < LineLengthBytesMax, "an index came in one line")
	text := string(raw)
	if unquoted, err := strconv.Unquote(text); err == nil {
		text = unquoted
	}
	index, err := strconv.Atoi(text)
	if err != nil || index < 0 || index >= max {
		return 0, fmt.Errorf("%w: %s isn't an index below %d", ErrBadRequest, raw, max)
	}
	return index, nil
}
