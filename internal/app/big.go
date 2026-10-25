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
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/kit"
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

// Feature: Tags
//
// capabilities/tags. Kinds, timeouts, groups and to do flags are plain Gherkin tags, so a test
// runner filters on the ones Givens uses. internal/project reads a scenario's tags, the
// Feature's and Rule's included, and internal/tags parses Cucumber's tag expressions. Each tag
// is a view: the Views section lists and selects them, through tagViews and isViewTag in
// big_helpers.go, and "An error state", "A tag view" and "A tag on the Feature" land there.
// Search's readQuery (search.go) splits a search into words and a tag expression, which
// findScenarios (search_find.go) matches. "A runner filters on the same tags" is
// cucumber-js's, and the ⚑ on a flagged scenario is internal/tui's.

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

// Feature: Update the kit
//
// capabilities/update-the-kit. Givens ships the kit (internal/kit) and keeps a project's
// capabilities/_shared/givens.sh in step with it by its version line. Code elsewhere: core
// glue shows kitMessages under the list (Snapshot, snapshot.go), adds kitOffers and runs
// updateKit on "Update kit" (allOffers and Choose, offers.go), and calls restoreKit whenever
// the project is read (reload and refresh, project.go). App's kits field is in app.go and
// Window's kitProblem in window.go; internal/kit reads a kit's version; "Compare" opens the
// kit in Write stories' editor (Edit, editor.go); the TUI's "u" key is internal/tui/browse.go.

// Limits for Update the kit.
const (
	// KitReleasesMax bounds how many kit versions Givens remembers.
	KitReleasesMax = 256
	// KitLengthMax bounds one kit's text, in bytes.
	KitLengthMax = 1 << 20
)

// KitVersionPrefix starts the kit's first line: "# givens kit 2".
const KitVersionPrefix = "# givens kit "

// KitRelease is one version of the kit, as Givens shipped it. Givens keeps every release it
// has shipped so it can tell an unedited old kit from one edited by hand.
type KitRelease struct {
	Version int    `json:"version"`
	Text    string `json:"text"` // the whole givens.sh, starting with its version line
}

// ShipKits sets the kit releases this app knows, oldest first; the last is the version it
// ships. Without a call the app uses the releases built into it. Acceptance steps call it so
// a scenario controls "Givens ships version N". Needed by every scenario in
// capabilities/update-the-kit.
func (a *App) ShipKits(releases []KitRelease) error {
	assert.That(a != nil, "ship kits runs on an app")
	assert.That(len(releases) > 0, "Givens ships at least one kit")
	assert.That(len(releases) <= KitReleasesMax, "releases fit KitReleasesMax")
	for _, release := range releases {
		assert.That(release.Version > 0, "kit versions start at 1")
		versioned := strings.HasPrefix(release.Text, KitVersionPrefix)
		assert.That(versioned, "a kit starts with its version line")
		assert.That(len(release.Text) <= KitLengthMax, "a kit fits KitLengthMax")
	}
	a.kits = append([]KitRelease{}, releases...)
	return nil
}

// shippedKits are the releases this app knows, oldest first: ShipKits's, or the one built in.
func (a *App) shippedKits() []KitRelease {
	assert.That(a != nil, "shipped kits runs on an app")
	assert.That(len(a.kits) <= KitReleasesMax, "releases stay bounded")
	if len(a.kits) > 0 {
		return a.kits
	}
	return []KitRelease{{Version: kit.ShippedVersion(), Text: kit.Script}}
}

// Kit messages and actions.
const (
	OfferUpdateKit     = "Update kit"
	OfferReplaceAnyway = "Replace anyway"
	MessageKitEdited   = "givens.sh has changes of its own"
	MessageNewerKit    = "This project uses a newer kit. Update Givens to edit its scripts."
	kitFile            = "_shared/givens.sh"
)

// Scenario: An update is available
// Scenario: The project's kit is newer than Givens

// kitState reads the project's kit: its text, its version (0 when unreadable), and the
// release Givens ships last.
func (w *Window) kitState() (string, int, KitRelease) {
	assert.That(w != nil, "kit state runs on a window")
	assert.That(w.found, "the window shows a capabilities folder")
	shipped := w.app.shippedKits()
	latest := shipped[len(shipped)-1]
	text, err := os.ReadFile(filepath.Join(w.path, filepath.FromSlash(kitFile)))
	if err != nil {
		return "", 0, latest
	}
	version, _ := kit.Version(string(text))
	return string(text), version, latest
}

// kitMessages say where the project's kit stands against the one Givens ships.
func (w *Window) kitMessages() []string {
	assert.That(w != nil, "kit messages run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if !w.found || w.kind != WindowProject {
		return []string{}
	}
	if w.kitProblem != "" {
		return []string{w.kitProblem}
	}
	_, version, latest := w.kitState()
	switch {
	case version == 0:
		return []string{}
	case version < latest.Version:
		return []string{fmt.Sprintf("Kit update available (%d → %d)", version, latest.Version)}
	case version > latest.Version:
		return []string{MessageNewerKit}
	}
	return []string{}
}

// kitOffers is "Update kit" while the kit is behind the one Givens ships, or nothing.
func (w *Window) kitOffers() []string {
	assert.That(w != nil, "kit offers run on a window")
	assert.That(w.project != nil, "kit offers need a project")
	if messages := w.kitMessages(); len(messages) > 0 &&
		strings.HasPrefix(messages[0], "Kit update available") {
		return []string{OfferUpdateKit}
	}
	return []string{}
}

// Scenario: Update the kit
// Scenario: The kit was edited by hand

// updateKit replaces the project's kit with the latest Givens ships, when it's an unedited
// copy of a release; otherwise it asks first, offering "Compare" and "Replace anyway".
func (w *Window) updateKit() error {
	assert.That(w != nil, "update kit runs on a window")
	assert.That(w.found, "the window shows a capabilities folder")
	text, version, latest := w.kitState()
	path := filepath.Join(w.path, filepath.FromSlash(kitFile))
	replace := func() error {
		if err := os.WriteFile(path, []byte(latest.Text), 0o644); err != nil {
			return err
		}
		return w.reload()
	}
	for _, release := range w.app.shippedKits() {
		if release.Version == version && release.Text == text {
			return replace()
		}
	}
	w.askEither(MessageKitEdited, OfferReplaceAnyway, replace, OfferCompare, func() error {
		_, err := w.Edit(kitFile)
		return err
	})
	return nil
}

// Scenario: A missing kit is put back

// restoreKit writes the kit Givens ships when the capabilities folder has none, since every
// setup script's first line sources it. It runs whenever the project is read from disk: on
// opening, and after any change to its files. A write that fails shows across the top.
// Needed by "A missing kit is put back".
func (w *Window) restoreKit() {
	assert.That(w.found, "the window shows a capabilities folder")
	assert.That(filepath.IsAbs(w.path), "the capabilities folder is absolute")
	path := filepath.Join(w.path, filepath.FromSlash(kitFile))
	w.kitProblem = ""
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return
	}
	shipped := w.app.shippedKits()
	latest := shipped[len(shipped)-1]
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.kitProblem = "Couldn't put back " + kitFile + ": " + err.Error()
		return
	}
	if err := os.WriteFile(path, []byte(latest.Text), 0o644); err != nil {
		w.kitProblem = "Couldn't put back " + kitFile + ": " + err.Error()
	}
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
