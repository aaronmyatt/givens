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
		// capabilities/open-a-project: Needs/open-a-project.swift.
		"window.branches":   s.onWindow(branches),
		"window.openBranch": s.onWindow(openBranch),
		// capabilities/views: Needs/views.swift.
		"window.viewHelpers": s.onWindow(viewHelpers),
		// capabilities/write-stories-and-scenarios: Needs/write-stories-and-scenarios.swift.
		"window.newStory":             s.onWindow(newStory),
		"window.newScenario":          s.onWindow(newScenario),
		"window.editorMarks":          s.onEditor(editorMarks),
		"window.stepSuggestions":      s.onEditor(stepSuggestions),
		"window.chooseStepSuggestion": s.onEditor(chooseStepSuggestion),
		"window.editorPreview":        s.onEditor(editorPreview),
		"window.delete":               s.onWindow(deleteItem),
	}
}

// onEditor finds the editor open in the window a request names, then runs call on it.
func (s *Server) onEditor(call func(*app.Editor, params) (any, error)) handler {
	assert.That(s != nil, "on editor runs on a server")
	assert.That(call != nil, "an editor method has a call")
	return s.onWindow(func(window *app.Window, given params) (any, error) {
		if window.Editor() == nil {
			return nil, fmt.Errorf("%w: no file is open in the editor", ErrBadRequest)
		}
		return call(window.Editor(), given)
	})
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

func attach(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "attach runs on a session")
	assert.That(utf8.ValidString(given.Path), "JSON strings are text")
	if !filepath.IsAbs(given.Path) || len(given.Path) > app.PathLengthMax {
		return nil, fmt.Errorf("%w: an attached file has an absolute path", ErrBadRequest)
	}
	return nil, session.Attach(given.Path)
}

func detach(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "detach runs on a session")
	assert.That(utf8.ValidString(given.Name), "JSON strings are text")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: detach needs a file name", ErrBadRequest)
	}
	return nil, session.Detach(given.Name)
}

func branches(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "branches runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if window.Kind() != app.WindowProject {
		return nil, fmt.Errorf("%w: only a project window has branches", ErrBadRequest)
	}
	return window.Branches()
}

func openBranch(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "open branch runs on a window")
	assert.That(utf8.ValidString(given.Name), "JSON strings are text")
	if window.Kind() != app.WindowProject {
		return nil, fmt.Errorf("%w: only a project window has branches", ErrBadRequest)
	}
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: open branch needs a branch name", ErrBadRequest)
	}
	_, err := window.OpenBranch(given.Name)
	return nil, err
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

func viewHelpers(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "view helpers runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return window.ViewHelpers(), nil
}

func newStory(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "new story runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	for _, field := range []string{given.Name, given.AsA, given.IWant, given.SoThat} {
		if len(field) > app.StorySentenceLengthMax {
			return nil, fmt.Errorf("%w: a story line is at most %d bytes", ErrBadRequest,
				app.StorySentenceLengthMax)
		}
	}
	return nil, window.NewStory(app.StoryDraft{
		Name: given.Name, AsA: given.AsA, IWant: given.IWant, SoThat: given.SoThat,
	})
}

func newScenario(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "new scenario runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: a scenario needs a name", ErrBadRequest)
	}
	draft := app.ScenarioDraft{Name: given.Name, Error: given.Error == "true"}
	return nil, window.NewScenario(draft)
}

func editorMarks(editor *app.Editor, _ params) (any, error) {
	assert.That(editor != nil, "marks runs on an editor")
	assert.That(editor.Path() != "", "an editor holds a file")
	return editor.Marks(), nil
}

func stepSuggestions(editor *app.Editor, given params) (any, error) {
	assert.That(editor != nil, "suggestions runs on an editor")
	assert.That(editor.Path() != "", "an editor holds a file")
	// The Mac editor sends the line it's on; the steps send nothing and type through the core.
	if given.Text != "" {
		return editor.SuggestionsFor(given.Text), nil
	}
	return editor.Suggestions(), nil
}

func chooseStepSuggestion(editor *app.Editor, given params) (any, error) {
	assert.That(editor != nil, "choose suggestion runs on an editor")
	assert.That(editor.Path() != "", "an editor holds a file")
	if given.Text == "" || strings.Contains(given.Text, "\n") {
		return nil, fmt.Errorf("%w: a suggestion is one step", ErrBadRequest)
	}
	return nil, editor.ChooseSuggestion(given.Text)
}

func editorPreview(editor *app.Editor, _ params) (any, error) {
	assert.That(editor != nil, "preview runs on an editor")
	assert.That(editor.Path() != "", "an editor holds a file")
	return editor.Preview(), nil
}

func deleteItem(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "delete runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: delete names what to delete", ErrBadRequest)
	}
	return nil, window.Delete(given.Name)
}
