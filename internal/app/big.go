// The big file: Givens's behaviour, told in the order of its stories.
//
// Each story in capabilities/ that's moved here has a section, in the order of the
// capabilities folders. A section starts with a "// Feature: <name>" comment, the Feature's
// name word for word, and runs to the next one. Inside it, each of the Feature's Rules has a
// "// Rule: <name>" comment, in the Feature's order, and a "// Scenario: <name>" comment marks
// the code a scenario starts in where that helps a reader; several can share one piece of code.
// Code that two or more stories use is in big_helpers.go, so this file reads as the stories
// alone.
//
// TestBigFile in acceptance/ checks those comments against the feature files, and
// scripts/entry.sh finds the section for a Feature, Rule or Scenario name. The workflow is
// tasks/one-big-file/essay.md; the pilot is tasks/one-big-file/notes.md. The rest of
// internal/app isn't here yet.
//
// Code outside this file that a story depends on, and why it's elsewhere:
//   - internal/kit/givens.sh: bash, run in the developer's shell by every setup script.
//   - internal/tui, internal/termedit and cmd/givens: the edges. They import this package, so
//     moving them in would make it depend on Bubble Tea; their stories have pointer sections.

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
	"givens/internal/tags"
)

// Feature: Copy a setup script
//
// capabilities/copy-a-setup-script. A scenario's setup goes on the clipboard as a command to
// paste anywhere in the project. "Run from a subfolder" is the kit's: it moves to the repo's
// root before running anything (the cd after the helpers load, in internal/kit/givens.sh).

// Scenario: Copy
// Scenario: Capabilities folder isn't at the root
// Scenario: Copy steps done by hand

// copySetup puts a scenario's setup on the clipboard: `bash <path>.sh`, the path from the git
// repo's root, or for a setup done only by hand, its markdown.
func (w *Window) copySetup(scenario project.Scenario) error {
	assert.That(w != nil, "copy runs on a window")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	clipboard := w.app.config.Clipboard
	if scenario.Setup == "" && scenario.SetupCards != "" {
		text, err := helpers.ReadScript(scenario.SetupCards)
		if err != nil {
			return err
		}
		return clipboard.WriteText(text)
	}
	if scenario.Setup == "" {
		return errors.New(scenario.Name + " has no setup to copy")
	}
	relative, err := filepath.Rel(repoRoot(w.path), scenario.Setup)
	if err != nil {
		return err
	}
	return clipboard.WriteText("bash " + filepath.ToSlash(relative))
}

// Scenario: Copy from search

// KeyCopySetup copies the selected search result's setup command.
const KeyCopySetup = "⇧⌘C"

// copyFromSearch copies the setup of the scenario first among the search's results.
func (w *Window) copyFromSearch() error {
	assert.That(w != nil, "copy from search runs on a window")
	assert.That(w.search != nil, "search is open")
	for _, group := range w.SearchResults().Groups {
		if group.Kind != SearchScenarios || len(group.Results) == 0 {
			continue
		}
		first := group.Results[0]
		found, ok := w.findScenario(first.Capability + "/" + first.Name)
		if !ok {
			return errors.New(first.Name + ": " + ErrNotFound.Error())
		}
		return w.copySetup(found)
	}
	return errors.New("no scenario is listed to copy")
}

// Feature: Search everything
//
// capabilities/search-everything. ⌘K looks through stories, scenarios, steps, setup files and
// helpers at once, from an index read once each time the project's files change. SearchResults,
// the grouped results, is in big_helpers.go, since Copy from search and Tags' search scenarios
// read it too; so is uniqueSteps, which the editor's step suggestions share. A tag expression is
// read by parseTags, in the Tags section. Core glue does the rest: Press and Type (input.go)
// open the panel and fill its box, searchSnapshot and searchMessages (snapshot.go) show it and
// say "Nothing matches", and selectStep (selection.go) selects a step result, whose scenarios
// Views' inView lists through usesStep. The panel is internal/tui/search.go, and `givens
// search` is runSearch in cmd/givens/panels.go.

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

// Scenario: Search for a phrase
// Scenario: Nothing matches

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

// findScenarios finds scenarios matching the tag expression, whose names hold the words.
func (w *Window) findScenarios(read query) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(read.problem == "", "the query parsed")
	found := make([]SearchResult, 0, 8)
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if read.tagged && !read.expression.Match(scenario.Tags) {
				continue
			}
			if !strings.Contains(strings.ToLower(scenario.Name), read.words) {
				continue
			}
			found = append(found, SearchResult{
				Name: scenario.Name, Detail: capability.Name, Capability: capability.Name,
				Matches: matchesIn(read.words, scenario.Name, capability.Name),
			})
		}
	}
	return limit(found)
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

// limit keeps the first SearchResultsMax results.
func limit(found []SearchResult) []SearchResult {
	assert.That(found != nil, "results are a list")
	assert.That(SearchResultsMax > 0, "results have a limit")
	if len(found) > SearchResultsMax {
		return found[:SearchResultsMax]
	}
	return found
}

// searchIndex holds what ⌘K looks through, read and lower-cased once: every distinct step
// with its uses, and every setup file's lines. It's built on the first search after the
// project's files change, so typing a query reads nothing from disk.
type searchIndex struct {
	steps   []indexedStep
	scripts []indexedScript
}

// indexedStep is one step text, as first written, with how many scenarios use it.
type indexedStep struct {
	name       string // "Given a signed-in shopper"
	lower      string // the text alone, lower case, which is what's searched
	capability string // the first capability that uses it
	uses       int
}

// indexedScript is one setup, teardown or setup steps file.
type indexedScript struct {
	capability string
	name       string // relative to the capabilities folder, like "checkout/scripts/x.sh"
	lowerName  string
	lines      []string // trimmed
	lowerLines []string
}

// searchMemo is the last query's results, so a snapshot, its messages and the grouped
// results asked for straight after all share one search.
type searchMemo struct {
	query   string
	filter  string
	results SearchResults
}

// searchIndexOf builds the index for the window's project.
func (w *Window) searchIndexOf() *searchIndex {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(filepath.IsAbs(w.path), "the capabilities folder is absolute")
	if w.index != nil {
		return w.index
	}
	index := &searchIndex{steps: make([]indexedStep, 0, 64), scripts: make([]indexedScript, 0, 32)}
	at := make(map[string]int, 64)
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, step := range uniqueSteps(scenario) {
				if seen, ok := at[step.Text]; ok {
					index.steps[seen].uses++
					continue
				}
				at[step.Text] = len(index.steps)
				index.steps = append(index.steps, indexedStep{
					name: step.Keyword + " " + step.Text, lower: strings.ToLower(step.Text),
					capability: capability.Name, uses: 1,
				})
			}
			for _, path := range []string{scenario.Setup, scenario.SetupCards,
				scenario.Teardown} {
				if script, ok := w.indexScript(capability.Name, path); ok {
					index.scripts = append(index.scripts, script)
				}
			}
		}
	}
	w.index = index
	return index
}

// indexScript reads one setup file into the index, or reports there's none to read.
func (w *Window) indexScript(capability string, path string) (indexedScript, bool) {
	assert.That(capability != "", "a script belongs to a capability")
	assert.That(path == "" || filepath.IsAbs(path), "a script's path is absolute")
	if path == "" {
		return indexedScript{}, false
	}
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return indexedScript{}, false
	}
	text, err := helpers.ReadScript(path)
	if err != nil {
		return indexedScript{}, false
	}
	name := filepath.ToSlash(relative)
	lines := strings.Split(text, "\n")
	script := indexedScript{
		capability: capability, name: name, lowerName: strings.ToLower(name),
		lines: make([]string, 0, len(lines)), lowerLines: make([]string, 0, len(lines)),
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		script.lines = append(script.lines, trimmed)
		script.lowerLines = append(script.lowerLines, strings.ToLower(trimmed))
	}
	return script, true
}

// Scenario: Find a capability by its story

// findStories finds capabilities whose name or a story line holds words.
func (w *Window) findStories(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	found := make([]SearchResult, 0, 4)
	for _, capability := range w.project.Capabilities {
		for _, line := range capability.Story {
			if !strings.Contains(strings.ToLower(line), words) {
				continue
			}
			found = append(found, SearchResult{
				Name: capability.Name, Detail: line, Capability: capability.Name,
				Matches: matchesIn(words, capability.Name, line),
			})
			break
		}
	}
	return limit(found)
}

// Scenario: Find every use of a step

// findSteps finds steps holding the words, each step text once, with how many scenarios use
// it word for word.
func (w *Window) findSteps(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	found := make([]SearchResult, 0, 8)
	for _, step := range w.searchIndexOf().steps {
		if strings.Contains(step.lower, words) {
			found = append(found, SearchResult{
				Name: step.name, Capability: step.capability, Uses: step.uses,
				Matches: matchesIn(words, step.name, ""),
			})
		}
	}
	return limit(found)
}

// usesStep is true when one of a scenario's steps is step word for word, keyword aside: the
// scenarios "Find every use of a step" lists once a step result is selected.
func usesStep(scenario Scenario, step string) bool {
	assert.That(step != "", "a step is named")
	assert.That(scenario.Name != "", "the scenario has a name")
	for _, used := range scenario.Steps {
		if _, text, _ := strings.Cut(used, " "); text == step {
			return true
		}
	}
	return false
}

// Scenario: Show only one kind of result

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

// findScripts finds setup files whose name or text holds the words: those named for them
// first, then those that only mention them, each with its first matching line.
func (w *Window) findScripts(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	named, mentioned := make([]SearchResult, 0, 8), make([]SearchResult, 0, 8)
	for _, script := range w.searchIndexOf().scripts {
		detail := ""
		for index, line := range script.lowerLines {
			if strings.Contains(line, words) {
				detail = script.lines[index]
				break
			}
		}
		isNamed := strings.Contains(script.lowerName, words)
		if detail == "" && !isNamed {
			continue
		}
		result := SearchResult{
			Name: script.name, Detail: detail, Capability: script.capability,
			Matches: matchesIn(words, script.name, detail),
		}
		if isNamed {
			named = append(named, result)
		} else {
			mentioned = append(mentioned, result)
		}
	}
	return limit(append(named, mentioned...))
}

// Scenario: Find a helper

// findHelpers finds documented helpers whose name or description holds the words.
func (w *Window) findHelpers(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	found := make([]SearchResult, 0, 4)
	for _, helper := range w.Helpers() {
		shown := strings.ToLower(helper.Name + "\n" + helper.Description)
		if !strings.Contains(shown, words) {
			continue
		}
		found = append(found, SearchResult{
			Name: helper.Name, Detail: helper.Description, Capability: helper.Capability,
			Matches: matchesIn(words, helper.Name, helper.Description),
		})
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].Name < found[b].Name })
	return limit(found)
}

// Feature: Tags
//
// capabilities/tags. Kinds, timeouts, groups and to do flags are plain Gherkin tags, so a test
// runner filters on the ones Givens uses. internal/project reads a scenario's tags, the
// Feature's and Rule's included, and internal/tags parses Cucumber's tag expressions. Each tag
// is a view: the Views section lists and selects them, through tagViews and isViewTag in
// big_helpers.go, and "An error state", "A tag view" and "A tag on the Feature" land there.
// The Search everything section's readQuery splits a search into words and a tag expression,
// which its findScenarios matches. "A runner filters on the same tags" is cucumber-js's, and
// the ⚑ on a flagged scenario is internal/tui's.

// Scenario: Search by tag
// Scenario: Search with a tag expression
// Scenario: A tag and words together
// Scenario: A tag expression that doesn't parse

// parseTags reads a search's tag expression, such as "@error and not @slow", and says why it
// doesn't parse, as the search shows it ("Not a tag expression: …"), or "".
// Ref: https://cucumber.io/docs/cucumber/api/#tag-expressions
func parseTags(expression []string) (tags.Expression, string) {
	assert.That(len(expression) <= NameLengthMax*4, "an expression stays bounded")
	parsed, err := tags.Parse(strings.Join(expression, " "))
	if err != nil {
		problem := "Not" + strings.TrimPrefix(err.Error(), "not")
		if !errors.Is(err, tags.ErrSyntax) {
			problem = err.Error()
		}
		return tags.Expression{}, problem
	}
	assert.That(err == nil, "a parsed query has no problem")
	return parsed, ""
}

// Offers that flag a scenario to do, or take the flag off. See capabilities/tags.
const (
	OfferFlagTodo   = "Flag as to do"
	OfferRemoveTodo = "Remove to do flag"
)

// Scenario: Flag a scenario to do
// Scenario: Remove a to do flag

// todoOffers is the flag offer a scenario's page shows. A @todo from its Rule or Feature line
// isn't offered for removal: taking it off would unflag every scenario under that line.
func todoOffers(loaded project.Scenario) []string {
	assert.That(loaded.Name != "", "a shown scenario has a name")
	assert.That(len(loaded.OwnTags) <= len(loaded.Tags), "own tags are among its tags")
	switch {
	case !loaded.Todo:
		return []string{OfferFlagTodo}
	case slices.Contains(loaded.OwnTags, project.TagTodo):
		return []string{OfferRemoveTodo}
	}
	return []string{}
}

// flagTodo adds @todo to a scenario's own tags, or takes it off, in its .feature file. Only
// the tag lines above its Scenario: line change, so git diff shows one line.
// Needed by "Flag a scenario to do" and "Remove a to do flag" in capabilities/tags.
func (w *Window) flagTodo(scenario project.Scenario, flagged bool) error {
	assert.That(w != nil, "flag runs on a window")
	assert.That(scenario.Line > 0, "a loaded scenario has a line")
	capability, ok := w.findCapability(scenario.Capability)
	if !ok {
		return fmt.Errorf("%s: %w", scenario.Capability, ErrNotFound)
	}
	text, err := readEditable(capability.FeaturePath)
	if err != nil {
		return err
	}
	lines, err := withTodo(strings.Split(text, "\n"), int(scenario.Line-1), flagged)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(capability.FeaturePath), err)
	}
	if err := os.WriteFile(capability.FeaturePath, []byte(strings.Join(lines, "\n")),
		0o644); err != nil {
		return err
	}
	return w.reload()
}

// withTodo flags or unflags the scenario whose Scenario: line is lines[at] (counting from 0).
// Gherkin tags sit on the lines just above the keyword they apply to.
// Ref: https://cucumber.io/docs/cucumber/api/#tags
func withTodo(lines []string, at int, flagged bool) ([]string, error) {
	assert.That(at >= 0, "lines count from 0")
	assert.That(len(lines) <= EditorTextBytesMax, "lines stay bounded")
	if at >= len(lines) || !strings.Contains(lines[at], "Scenario") {
		return nil, fmt.Errorf("line %d isn't a Scenario: line", at+1)
	}
	top := at
	// bound: top counts down to 0 at most.
	for top > 0 && strings.HasPrefix(strings.TrimSpace(lines[top-1]), "@") {
		top--
	}
	if flagged {
		if top == at {
			indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " \t"))]
			return slices.Insert(lines, at, indent+project.TagTodo), nil
		}
		lines[at-1] += " " + project.TagTodo
		return lines, nil
	}
	kept := make([]string, 0, len(lines))
	for index, line := range lines {
		fields := strings.Fields(line)
		if index < top || index >= at || !slices.Contains(fields, project.TagTodo) {
			kept = append(kept, line)
			continue
		}
		// A tag line left with no tags goes; one with others keeps them, in order.
		fields = slices.DeleteFunc(fields, func(field string) bool {
			return field == project.TagTodo
		})
		if len(fields) > 0 {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			kept = append(kept, indent+strings.Join(fields, " "))
		}
	}
	if slices.Equal(kept, lines) {
		return nil, fmt.Errorf("line %d's own tags have no %s", at+1, project.TagTodo)
	}
	assert.That(len(kept) <= len(lines), "unflagging adds no line")
	return kept, nil
}

// Feature: The kit
//
// capabilities/the-kit. A pointer: the kit is bash, so none of its code is here. It's
// internal/kit/givens.sh, built into the givens binary by internal/kit/kit.go and copied into
// each project as capabilities/_shared/givens.sh; every scenario is a bats test in
// internal/kit/kit.bats, run under bash 3.2 by scripts/check.sh kit. Strict mode, the "Your
// turn" heading, quiet mode, colour, need, fail and warn, --when, and steps by hand and asks in
// a terminal are all functions in givens.sh.

// Feature: Trust a project
//
// capabilities/trust-a-project. Givens asks before it runs any of a project's scripts, and
// remembers the answer by the capabilities folder's path, in its own data folder: never in the
// project, so a cloned repo can't mark itself trusted. Reading, searching and copying need no
// trust. Drop in (startSession) asks here before its setup runs.

// Trust lives in the app's own data folder, one list of folders for each answer.
const (
	trustedFileName   = "trusted.txt"
	untrustedFileName = "untrusted.txt"
	// TrustedMax bounds the trusted folders Givens remembers.
	TrustedMax = 1024
)

// Trust prompt buttons and the message once scripts are off.
const (
	OfferTrust            = "Trust"
	OfferDontTrust        = "Don't trust"
	OfferTrustThisProject = "Trust this project"
	MessageScriptsOff     = "Scripts are off for this project"
)

// Scenario: Asked before the first run
// Scenario: Trust is remembered
// Scenario: Moved or re-cloned project

// trustOf reads what the developer chose for this window's project: trusted, refused ("Don't
// trust"), or neither yet. It goes by path, so a moved or re-cloned project is neither.
func (w *Window) trustOf() (bool, bool, error) {
	assert.That(w != nil, "trust of runs on a window")
	assert.That(filepath.IsAbs(w.path), "a window's folder is absolute")
	trusted, err := w.app.Trusted(w.path)
	if err != nil {
		return false, false, err
	}
	refused, err := w.app.untrusted(w.path)
	return trusted, refused, err
}

// runTrusted begins a Drop in's setup if the project is trusted, asks "Run scripts from this
// project?" if it's neither trusted nor refused, and opens the page with scripts off if it's
// refused. A setup with no script needs no trust: nothing runs.
func (w *Window) runTrusted(session *Session, runsScripts bool, trusted bool,
	refused bool) error {
	assert.That(w != nil, "run trusted runs on a window")
	assert.That(session != nil && session.window == w, "the session is this window's")
	switch {
	case runsScripts && refused:
		session.scriptsOff()
	case runsScripts && !trusted:
		w.askEither("Run scripts from this project? "+filepath.Dir(w.path), OfferTrust,
			func() error {
				if err := w.app.MarkTrusted(w.path); err != nil {
					return err
				}
				return session.begin()
			}, OfferDontTrust, func() error {
				session.scriptsOff()
				return w.app.markUntrusted(w.path)
			})
	default:
		return session.begin()
	}
	return nil
}

// MarkTrusted records a project's folder as trusted, as choosing "Trust" on the "Run scripts
// from this project?" prompt does. path is a capabilities folder, or the project folder holding
// one, as Open takes. Drop in scenarios that aren't about trust use it to start trusted.
func (a *App) MarkTrusted(path string) error {
	assert.That(a != nil, "mark trusted runs on an app")
	assert.That(path != "", "a trusted folder has a path")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	folder, _, err := findCapabilities(path)
	if err != nil {
		return err
	}
	if err := a.forget(untrustedFileName, folder); err != nil {
		return err
	}
	return a.remember(trustedFileName, folder)
}

// Trusted is true when folder, a capabilities folder as a window holds it, was marked trusted.
func (a *App) Trusted(folder string) (bool, error) {
	assert.That(a != nil, "trusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	trusted, err := a.folders(trustedFileName)
	if err != nil {
		return false, err
	}
	return slices.Contains(trusted, folder), nil
}

// trustedFolders reads the trusted folders.
func (a *App) trustedFolders() ([]string, error) {
	assert.That(a != nil, "trusted folders runs on an app")
	assert.That(a.config.AppDataDir != "", "trust lives in the app's data folder")
	return a.folders(trustedFileName)
}

// Scenario: Choose not to trust

// markUntrusted records "Don't trust" for a capabilities folder: its scripts stay off until
// the developer chooses "Trust this project".
func (a *App) markUntrusted(folder string) error {
	assert.That(a != nil, "mark untrusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	if err := a.forget(trustedFileName, folder); err != nil {
		return err
	}
	return a.remember(untrustedFileName, folder)
}

// untrusted is true when the developer chose "Don't trust" for folder.
func (a *App) untrusted(folder string) (bool, error) {
	assert.That(a != nil, "untrusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	folders, err := a.folders(untrustedFileName)
	if err != nil {
		return false, err
	}
	return slices.Contains(folders, folder), nil
}

// scriptsOff opens the Drop in page with nothing run: the developer chose "Don't trust".
// Reading and copying still work.
func (s *Session) scriptsOff() {
	assert.That(s != nil, "scripts off runs on a session")
	assert.That(s.process == nil, "nothing has run")
	s.status, s.message, s.untrusted = SessionStopped, MessageScriptsOff, true
}

// trustThisProject trusts the project from a Drop in with scripts off, then drops in again.
func (s *Session) trustThisProject() error {
	assert.That(s != nil, "trust this project runs on a session")
	assert.That(s.untrusted, "scripts are off")
	if err := s.window.app.MarkTrusted(s.window.path); err != nil {
		return err
	}
	return s.restart(false)
}

// Feature: Use Givens from the terminal
//
// capabilities/use-givens-from-the-terminal. A pointer: the commands are cmd/givens, an edge
// that imports this package, so none of their code is here. list and check are
// cmd/givens/list.go (runList, runCheck); run, with steps by hand and code blocks in a
// terminal, is cmd/givens/run.go (runSetup, runBlocks); main.go routes each command. "List by
// tag" parses its expression with internal/tags.

// Feature: Views
//
// capabilities/views. Views gather scenarios and helpers from every capability: Error states,
// Unverified and Shared, plus one view per tag, each worked out from the loaded project with
// nothing stored. A view is selected by name through selectNow (selection.go); the TUI's Views,
// Tags and Helpers tabs are internal/tui, and snapshot.go reads what's listed. tagViews and
// isViewTag, which make each tag a view, are in big_helpers.go, since Tags lists them too.

// Views, under the capabilities in the sidebar. Each other tag is a view of its own, named
// after it without its @. See capabilities/views.
const (
	ViewErrorStates = "Error states"
	ViewUnverified  = "Unverified"
	ViewShared      = "Shared"
)

// Sidebar kinds below the capabilities.
const (
	KindView = "view"
	KindTag  = "tag"
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
			status := w.statusOf(scenario)
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

// Scenario: See every error state
// Scenario: See what needs checking again

// ListedScenarios is the list the main pane shows for what's selected: a view ("Error
// states"), a tag ("payments"), or a step search result (the scenarios using that step).
// Needed by capabilities/tags, "List by tag" in capabilities/use-givens-from-the-terminal and
// "Find every use of a step" in capabilities/search-everything.
func (w *Window) ListedScenarios() []Scenario {
	assert.That(w != nil, "listed scenarios run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.scenariosIn(w.selectedView, w.selectedStep)
}

// ScenariosIn lists a view's scenarios ("Error states", "Unverified") or a tag's
// ("payments"), as ListedScenarios does once it's selected, but leaves the selection alone:
// the TUI lists a view under its heading while the cursor moves on to its scenarios, which
// selects each one. "Shared" lists helpers, not scenarios, so it has none.
func (w *Window) ScenariosIn(view string) []Scenario {
	assert.That(w != nil, "scenarios in runs on a window")
	assert.That(view != "", "a view or tag is named")
	return w.scenariosIn(view, "")
}

// scenariosIn lists the scenarios in a view or tag, or that use a step.
func (w *Window) scenariosIn(view string, step string) []Scenario {
	assert.That(w.app != nil, "a window belongs to an app")
	assert.That(view == "" || step == "", "a view or a step, not both")
	w.refresh()
	listed := make([]Scenario, 0, 16)
	if w.project == nil || (view == "" && step == "") || view == ViewShared {
		return listed
	}
	for _, capability := range w.project.Capabilities {
		for _, loaded := range capability.Scenarios {
			shown := w.scenarioOf(capability, loaded)
			if view == ViewUnverified && loaded.Outline {
				listed = append(listed, w.unverifiedRows(shown, loaded)...)
			} else if inView(shown, view, step) {
				listed = append(listed, shown)
			}
		}
	}
	return listed
}

// inView is true when a scenario belongs in a view or tag, or uses a step.
func inView(scenario Scenario, view string, step string) bool {
	assert.That(view != "" || step != "", "a view or step is named")
	assert.That(scenario.Name != "", "the scenario has a name")
	if step != "" {
		return usesStep(scenario, step)
	}
	switch view {
	case ViewErrorStates:
		return scenario.Error
	case ViewUnverified:
		return !strings.HasPrefix(scenario.Status, statusVerified)
	}
	return slices.Contains(scenario.Tags, "@"+view)
}

// unverifiedRows lists an outline's rows that aren't verified, each as its own entry, named
// after the outline and the row: "Card rejected by the bank: stolen".
func (w *Window) unverifiedRows(shown Scenario, loaded project.Scenario) []Scenario {
	assert.That(loaded.Outline, "only an outline has rows")
	assert.That(shown.Name == loaded.Name, "the shown scenario is the loaded one")
	listed := make([]Scenario, 0, 2)
	for _, row := range rowsOf(loaded) {
		status := rowStatus(w.project, loaded, row, w.app.config.Clock.Now())
		if strings.HasPrefix(status, statusVerified) {
			continue
		}
		entry := shown
		entry.Name, entry.Status = shown.Name+": "+rowName(row.values), status
		listed = append(listed, entry)
	}
	return listed
}

// Scenario: Browse helpers

// ViewHelper is one helper listed in the sidebar's "Shared" view.
type ViewHelper struct {
	Name        string `json:"name"`        // "seed_shopper"
	Signature   string `json:"signature"`   // the text after "# @helper"
	Description string `json:"description"` // the comment lines under it, joined with spaces
	File        string `json:"file"`        // where it's defined, in the capabilities folder
	Callers     int    `json:"callers"`     // how many setup scripts call it
}

// ViewHelpers lists the helpers the "Shared" view shows, when it's selected: every helper
// documented with a "# @helper" comment, in capabilities/_shared and each capability's
// _shared folder. Needed by "Browse helpers" in capabilities/views. The scenarios in the
// other views ("Error states", "Unverified", a tag) are listed by ListedScenarios.
func (w *Window) ViewHelpers() []ViewHelper {
	assert.That(w != nil, "view helpers run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	listed := make([]ViewHelper, 0, 8)
	if w.selectedView != ViewShared {
		return listed
	}
	for _, helper := range w.Helpers() {
		listed = append(listed, ViewHelper{
			Name: helper.Name, Signature: helper.Signature, Description: helper.Description,
			File: helper.DefinedIn, Callers: len(helper.Callers),
		})
	}
	return listed
}

// Scenario: A view with nothing in it

// emptyView says a selected view lists nothing: "No error states yet", or "" when it lists
// something or no view is selected.
func (w *Window) emptyView() string {
	assert.That(w != nil, "empty view runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.selectedView == "" || w.selectedView == ViewShared || len(w.ListedScenarios()) > 0 {
		return ""
	}
	switch w.selectedView {
	case ViewErrorStates:
		return "No error states yet"
	case ViewUnverified:
		return "Every scenario is verified"
	}
	return "No scenarios are tagged @" + w.selectedView
}

// Feature: Work in the terminal
//
// capabilities/work-in-the-terminal. A pointer: the TUI and the editor hand-off are edges that
// import this package, so none of their code is here. The TUI is internal/tui: tui.go (the
// Bubble Tea model), browse.go (the tabs and lists), story.go, search.go, helpers.go,
// dropin.go and checklist.go (the Drop in page), layout.go, and edit.go, which hands a file to
// the developer's editor through internal/termedit. cmd/givens starts it (tui.go) and runs the
// terminal commands (edit.go, new.go, manage.go, panels.go). What each screen does is the
// other stories' code, here and in internal/app, read through snapshot.go.
