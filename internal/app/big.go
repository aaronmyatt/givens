// The big file: Givens's behaviour, told in the order of its stories.
//
// Each story in capabilities/ has a section, in the order of the capabilities folders. A
// section starts with a "// Feature: <name>" comment, the Feature's name word for word, and
// runs to the next one. Inside it, each of the Feature's Rules has a
// "// Rule: <name>" comment, in the Feature's order, and a "// Scenario: <name>" comment marks
// the code a scenario starts in where that helps a reader; several can share one piece of code.
// Code that two or more stories use is in big_helpers.go, so this file reads as the stories
// alone.
//
// TestBigFile in acceptance/ checks those comments against the feature files, and
// scripts/entry.sh finds the section for a Feature, Rule or Scenario name. The workflow is
// tasks/one-big-file/essay.md; how it got here is tasks/one-big-file/notes.md.
//
// Code outside this file that a story depends on, and why it's elsewhere:
//   - internal/kit/givens.sh: bash, run in the developer's shell by every setup script.
//   - internal/tui, internal/termedit and cmd/givens: the edges. They import this package, so
//     moving them in would make it depend on Bubble Tea; their stories have pointer sections.
//   - internal/app/model: the data models, types and constants with no code: Drop in's
//     (dropin.go: the card limits and kinds, URLOpener, CodeBlock, SetupCard, DropInRow,
//     CheckItem), Write stories' (edit.go: the drafts, marks, suggestions, preview lines and
//     EditorTextBytesMax) and Share setup's (helpers.go: Helper, RequiredCommand,
//     HelperProblem, HelperEdit). A package of their own, so the stories read without them.
//   - internal/browser: Mix shell's Chrome of Givens's own, a wrapper on chromedp, the
//     third-party CDP client; a package of its own, so the dependency stays in one place.
//   - snapshot.go, offers.go (Offers, Choose, allOffers), selection.go, input.go, prompt.go
//     and project.go (capabilityOf, scenarioOf): the core glue every story runs on. They turn
//     a label, a key or a selection into a call to a story, and build what the TUI and the
//     steps read; kept apart so this file reads as the stories, not the plumbing.
//   - app.go and window.go: the App and Window types and the API the edges hold.
//   - internal/spec, project, runs, runref, branch, setup, tags, slug, markdown, helpers,
//     kit, starter and watch: domain packages, each with one job and its own tests.

package app

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"givens/internal/app/model"
	"givens/internal/assert"
	"givens/internal/branch"
	"givens/internal/browser"
	"givens/internal/helpers"
	"givens/internal/kit"
	"givens/internal/markdown"
	"givens/internal/project"
	"givens/internal/runref"
	"givens/internal/runs"
	"givens/internal/setup"
	"givens/internal/slug"
	"givens/internal/spec"
	"givens/internal/starter"
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

// Feature: Drop into a scenario
//
// capabilities/drop-into-a-scenario. A Drop in runs a scenario's setup with the kit's quiet
// mode, teardown first, and walks the developer through its When and Then steps as a
// checklist, then records the run. A Session is one Drop in, and the Drop in page is what it
// shows and offers; its data model (the card limits and kinds, URLOpener, CodeBlock, SetupCard,
// DropInRow and CheckItem) is internal/app/model's dropin.go.
//
// Drop in calls the stories that came out of it: trustOf and runTrusted (Trust a project)
// before setup runs, and runTrusted calls begin back; trustThisProject on the page; keepRun
// (Keep runs off main) to record; givensBin, runBlocksIn, runLanguage, runNextCard, runState,
// pollBlock, stopBlock and openLink (Mix shell, browser and hand steps) for GIVENS_BIN, run
// blocks and the Givens browser; elsewhere (Share setup between scenarios) on a failed setup's
// page; copySetup (Copy a setup script) for "Copy".
//
// In big_helpers.go: checklistOf, fail, resume and restart, since Mix shell's cards and Trust's
// "Trust this project" call them too; editFile and KitSourceLine, which Write stories' setup
// files open and start with, as addTeardown's teardown does; outlineRow, rowsOf, rowStatus and
// rowName, since Views' unverifiedRows lists an outline's rows with them; removeArguments,
// which Open a project's CloseWindow calls, and scratchDir, the folder it removes, which Mix
// shell's run blocks share; RequiredCommands, which precheck checks; and scenarioWarning, which
// shows examplesWarning and a @timeout tag that can't be read (MessageTimeoutTag). StatusOf
// (big_helpers.go) sums up an outline with outlineStatus. Read a capability's run history lists
// a run's files with runFiles.
//
// A from line is the kit's from (internal/kit/givens.sh): it runs the scenario built on, its
// teardown first, with a row's values, and guards against a loop, so "A scenario that builds on
// another", "Each level tears down its own state", "Build on one row of an outline" and "A loop
// run from the terminal" are bash; fromProblem here catches what's wrong before anything runs.
// "Running setup twice gives the same state" is the setup's own rule; "Reset state" is restart.
// "Runs on two branches merge cleanly" is git's: runs/** merge=union, which Open a project's
// createCapabilities writes in .gitattributes.
//
// Core glue calls in, a line each: allOffers and Choose (offers.go) call offers and choose;
// chooseOnScenarioPage calls DropIn, addTeardown, scenarioFileOf and editFile, and
// scenarioPageOffers offers the edit offers and "Add teardown"; Snapshot (snapshot.go) calls
// snapshot, messages and DropInRows and shows hints, in the SessionSnapshot type, and rowLine
// says a row as one line; scenarioOffers (offers.go) has the unnamed-Examples branch; Select's
// leaveThenSelect (selection.go) asks with marked before leaving and calls leaveSession; Type
// (input.go) fills an ask card through answering; Quit (app.go) calls removeArguments.
// Window's session field is in window.go. The TUI's Drop in page is internal/tui/dropin.go and
// checklist.go; `givens drop-in` is cmd/givens.

// Session states.
const (
	SessionRunning = "running" // setup is running
	SessionWaiting = "waiting" // setup is paused at a card: by_hand, ask, or a markdown step
	SessionReady   = "ready"   // setup finished; the checklist is shown
	SessionFailed  = "failed"  // setup or a card failed; no checklist
	SessionStopped = "stopped" // the developer chose "Stop"
)

// Drop in page actions.
const (
	OfferResetState     = "Reset state"
	OfferEditTeardown   = "Edit teardown"
	OfferRunSetupAnyway = "Run setup anyway"
	OfferAddTeardown    = "Add teardown"
	OfferStop           = "Stop"
	OfferKeepWaiting    = "Keep waiting"
	OfferTryAgain       = "Try again"
	OfferEditScript     = "Edit script"
)

// pollInterval is how often Wait looks at a running setup.
const pollInterval = 20 * time.Millisecond

// Session is one Drop in: a setup run and the checklist that follows.
type Session struct {
	window   *Window
	scenario project.Scenario
	row      []runs.Pair // the outline row's values, or nil
	examples string      // the row's Examples block's slug, or ""

	process  *setup.Process
	block    *blockRun // a markdown card's run block running now, or nil
	scripts  []string  // scripts still to run after the current one: teardown, then setup
	teardown bool      // the script running now is the teardown

	skipTeardown bool   // "Run setup anyway" leaves a failing teardown out
	argumentsDir string // where GIVENS_ARG_<n> files are, or ""
	untrusted    bool   // the developer chose "Don't trust", so scripts are off

	// Cards: by_hand and ask cards the script printed, then the markdown setup's steps.
	cards          []model.SetupCard
	snippets       []string // each card's text to copy
	parsed         int      // the current script's output lines read so far
	reading        int      // text lines still to read for the last card
	display        []string // the current script's output, card lines taken out
	intro          string   // the markdown setup's text above its first step
	markdownFrom   int      // the index of the first markdown card, or -1
	cardInput      string   // what's typed into a waiting ask card
	cardFailed     bool     // a by_hand card was marked failed
	markdownFailed bool     // a markdown card was marked failed
	waitingSince   time.Time
	waited         time.Duration // time spent waiting at cards, which isn't slow
	env            []string
	earlier        []string // output of scripts that already finished
	started        time.Time

	status      string
	message     string   // across the top: "Setup failed (exit 3)", "This scenario needs bb"
	hints       []string // install hints, each with a copy button
	keepWaiting bool
	checklist   []model.CheckItem
	note        string
	attachments []string // paths of files "Record run" copies into the run's folder
	recorded    bool     // "Record run" saved this checklist, so leaving loses nothing
}

// Session is the Drop in on show, or nil.
func (w *Window) Session() *Session {
	assert.That(w != nil, "session runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.session
}

// Scenario: Drop in
// Scenario: The setup script fails
// Scenario: Teardown runs before setup
// Scenario: The teardown fails

// DropIn drops in to a scenario: it runs the setup and opens the Drop in page. Before anything
// runs it checks the commands the setup needs and the setups it builds on.
func (w *Window) DropIn(scenario string) (*Session, error) {
	return w.dropIn(scenario, false)
}

// dropIn drops in, leaving the teardown out when skipTeardown is set: "Run setup anyway".
func (w *Window) dropIn(scenario string, skipTeardown bool) (*Session, error) {
	assert.That(w != nil, "drop in runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	found, ok := w.findScenario(scenario)
	if !ok {
		return nil, fmt.Errorf("%s: %w", scenario, ErrNotFound)
	}
	return w.startSession(found, outlineRow{}, skipTeardown)
}

// startSession opens the Drop in page for a scenario, or one row of an outline, and runs its
// setup once the project is trusted. A setup with no script needs no trust: nothing runs.
func (w *Window) startSession(found project.Scenario, row outlineRow, skipTeardown bool) (
	*Session, error) {
	assert.That(w != nil, "start session runs on a window")
	assert.That(found.Capability != "", "the scenario belongs to a capability")
	if w.branch != "" {
		return nil, fmt.Errorf("%s isn't checked out, so its setup scripts don't run", w.branch)
	}
	trusted, refused, err := w.trustOf()
	if err != nil {
		return nil, err
	}
	session := &Session{
		window: w, scenario: found, status: SessionRunning, skipTeardown: skipTeardown,
		started: w.app.config.Clock.Now(), env: setupEnv(found, row.values),
		markdownFrom: -1, row: row.values, examples: row.examples,
	}
	if w.session != nil {
		w.session.removeArguments()
	}
	w.session = session
	// A markdown setup's run blocks are code too, so they're asked about as a script is.
	runsScripts := found.Setup != "" || found.Teardown != "" ||
		len(runBlocksIn(found.SetupCards)) > 0
	return session, w.runTrusted(session, runsScripts, trusted, refused)
}

// begin checks what setup needs, then starts its scripts: teardown first, then setup.
func (s *Session) begin() error {
	assert.That(s.status == SessionRunning, "a session begins running")
	assert.That(s.process == nil, "a session begins once")
	s.started = s.window.app.config.Clock.Now()
	if problem, hints, err := s.window.precheck(s.scenario); err != nil || problem != "" {
		s.hints = hints
		if err == nil {
			s.fail(problem)
		}
		return err
	}
	arguments, err := s.writeArguments()
	if err != nil {
		return err
	}
	bin, err := s.givensBin()
	if err != nil {
		return err
	}
	s.env = append(append(s.env, arguments...), bin)
	for _, script := range []string{s.scenario.Teardown, s.scenario.Setup} {
		if script != "" && !(s.skipTeardown && script == s.scenario.Teardown) {
			s.scripts = append(s.scripts, script)
		}
	}
	if len(s.scripts) == 0 {
		s.finishScripts()
		return nil
	}
	s.start()
	return nil
}

// setupEnv is the variables a Drop in runs setup with. GIVENS_QUIET hides the "your turn"
// lines, which the checklist shows instead; GIVENS_UI makes by_hand and ask cards. begin adds
// GIVENS_BIN and the arguments' files.
// See capabilities/the-kit/about.md.
func setupEnv(scenario project.Scenario, row []runs.Pair) []string {
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	assert.That(len(row) <= helpers.HelpersMax, "a row stays bounded")
	env := []string{"GIVENS_QUIET=1", "GIVENS_UI=1", "GIVENS_SCENARIO=" + scenario.Key()}
	for _, value := range row {
		env = append(env, "GIVENS_EX_"+value.Name+"="+value.Value)
	}
	return env
}

// start runs the next script in the queue, or finishes when there's none.
func (s *Session) start() {
	assert.That(s.status == SessionRunning, "only a running session starts a script")
	assert.That(len(s.scripts) > 0, "there's a script to start")
	s.earlier, s.display, s.parsed = append(s.earlier, s.display...), nil, 0
	script := s.scripts[0]
	s.scripts = s.scripts[1:]
	s.teardown = script == s.scenario.Teardown
	if s.teardown {
		s.earlier = append(s.earlier, "→ teardown")
	}
	process, err := setup.Start(setup.Request{
		Script: script, Dir: filepath.Dir(s.window.path), Env: s.env,
	})
	if err != nil {
		s.process = nil
		s.fail("Setup couldn't start: " + err.Error())
		return
	}
	s.process = process
}

// poll moves a running session on when its script has ended.
func (s *Session) poll() {
	assert.That(s != nil, "poll runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	if s.block != nil {
		s.pollBlock()
		return
	}
	if (s.status != SessionRunning && s.status != SessionWaiting) || s.process == nil {
		return
	}
	s.readOutput()
	if s.reading == 0 {
		s.finishCardText()
	}
	result, ended := s.process.Wait(time.Millisecond)
	if !ended || s.markdownFrom >= 0 {
		return
	}
	s.readOutput()
	s.resume()
	switch {
	case result.Err == setup.ErrStopped:
		s.status = SessionStopped
	case result.Err != nil:
		s.fail("Setup failed: " + result.Err.Error())
	case result.ExitCode != 0 && s.cardFailed:
		s.fail(MessageStoppedAtByHand)
	case result.ExitCode != 0 && s.teardown:
		s.fail("Teardown failed (exit " + strconv.Itoa(result.ExitCode) + ")")
	case result.ExitCode != 0:
		s.fail("Setup failed (exit " + strconv.Itoa(result.ExitCode) + ")")
	case len(s.scripts) > 0:
		s.start()
	default:
		s.finishScripts()
	}
}

// finishScripts moves on once every script ran: to the markdown setup's cards, or to the
// checklist.
func (s *Session) finishScripts() {
	assert.That(s.status == SessionRunning, "the scripts ran to their end")
	assert.That(len(s.scripts) == 0, "no script is left to run")
	if s.scenario.SetupCards != "" && s.markdownFrom < 0 {
		if err := s.addMarkdownCards(); err != nil {
			s.fail("The markdown setup couldn't be read: " + err.Error())
		}
		return
	}
	s.status, s.checklist = SessionReady, checklistOf(s.scenario, s.row)
}

// Status is one of the Session* states.
func (s *Session) Status() string {
	assert.That(s != nil, "status runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return s.status
}

// Output is the setup's output so far, one line per entry.
func (s *Session) Output() []string {
	assert.That(s != nil, "output runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return append(append(make([]string, 0, len(s.earlier)+16), s.earlier...), s.display...)
}

// Wait waits up to limit for setup to finish or pause, then returns Status.
func (s *Session) Wait(limit time.Duration) string {
	assert.That(s != nil, "wait runs on a session")
	assert.That(limit > 0, "a wait has a limit")
	deadline := time.Now().Add(limit)
	for polls := int64(0); polls <= int64(limit/pollInterval); polls++ {
		// A session with no script running is waiting on a question, like the trust prompt.
		idle := s.process == nil && s.block == nil
		if s.Status() != SessionRunning || idle || time.Now().After(deadline) {
			break
		}
		time.Sleep(pollInterval)
	}
	return s.Status()
}

// Checklist is the manual Givens, then the When and Then steps in the order they're written.
// It's empty until setup finishes.
func (s *Session) Checklist() []model.CheckItem {
	assert.That(s != nil, "checklist runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return append(make([]model.CheckItem, 0, len(s.checklist)), s.checklist...)
}

// ScenarioKey is the Drop in's scenario as a run key, like checkout/card-declined, by which
// the window finds it again.
func (s *Session) ScenarioKey() string {
	assert.That(s != nil, "scenario key runs on a session")
	assert.That(s.scenario.Capability != "", "the scenario belongs to a capability")
	return s.scenario.Key()
}

// Givens are the scenario's Given steps, Background first, placeholders filled in. Setup
// does them, so they aren't on the checklist; the givens command's TUI shows them above it,
// so the whole scenario reads on the Drop in page.
func (s *Session) Givens() []string {
	assert.That(s != nil, "givens runs on a session")
	assert.That(s.scenario.Name != "", "the session has a scenario")
	return givensOf(s.scenario, s.row)
}

// givensOf are a scenario's Given steps, Background first, each And and But after a Given
// with them, placeholders filled in: what setup sets up, which the checklist leaves out.
func givensOf(scenario project.Scenario, row []runs.Pair) []string {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= helpers.HelpersMax, "steps stay bounded")
	givens := make([]string, 0, len(scenario.Background)+len(scenario.Steps))
	for _, steps := range [][]project.Step{scenario.Background, scenario.Steps} {
		kind := "given"
		for _, step := range steps {
			switch step.Keyword {
			case "Given", "When", "Then":
				kind = strings.ToLower(step.Keyword)
			}
			if kind == "given" {
				givens = append(givens, step.Keyword+" "+fillPlaceholders(step.Text, row))
			}
		}
	}
	return givens
}

// FileOf is the file an "Edit script", "Edit setup" or "Edit teardown" offer opens, as an
// absolute path, or "" for any other offer. The givens command's TUI opens it in the
// developer's editor, where the app opens its own.
func (s *Session) FileOf(offer string) string {
	assert.That(s != nil, "file of runs on a session")
	assert.That(offer != "", "an offer has a label")
	file := scenarioFileOf(offer)
	if file == nil {
		return ""
	}
	return file(s.scenario)
}

// Note is the run's note as written so far, which "Record run" keeps with the run.
func (s *Session) Note() string {
	assert.That(s != nil, "note runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return s.note
}

// Scenario: A command the setup needs isn't installed

// precheck finds what would stop setup before it runs: a from line naming no scenario, or a
// command the setup or its helpers need that isn't installed. It returns the message to show,
// and any install hints.
func (w *Window) precheck(scenario project.Scenario) (string, []string, error) {
	assert.That(w.project != nil, "a Drop in runs in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	if scenario.Setup == "" && len(runBlocksIn(scenario.SetupCards)) == 0 {
		return "", nil, nil
	}
	if scenario.Setup != "" {
		if problem, err := w.fromProblem(scenario); err != nil || problem != "" {
			return problem, nil, err
		}
	}
	required, err := w.RequiredCommands(scenario.Capability + "/" + scenario.Name)
	if err != nil {
		return "", nil, err
	}
	for _, command := range required {
		if !command.Installed {
			return "This scenario needs " + command.Command, []string{command.Hint}, nil
		}
	}
	return "", nil, nil
}

// Scenario: Setup is still running after two minutes
// Scenario: A longer timeout
// Scenario: A timeout Givens can't read
// Scenario: Waiting doesn't count as slow

// stillRunning is the "Still running after …" message once setup has run past its limit on
// the app's clock, or "" before then, after "Keep waiting", or once setup has ended.
func (s *Session) stillRunning() string {
	assert.That(s != nil, "still running runs on a session")
	assert.That(!s.started.IsZero(), "a session records when it started")
	limit := s.scenario.StillRunning
	if limit <= 0 {
		limit = project.StillRunningDefault
	}
	if s.status != SessionRunning || s.keepWaiting ||
		s.window.app.config.Clock.Now().Sub(s.started)-s.waited < limit {
		return ""
	}
	return "Still running after " + spokenDuration(limit)
}

// spokenDuration says a limit as a person would: "2 minutes", "90 seconds".
func spokenDuration(limit time.Duration) string {
	assert.That(limit > 0, "a limit is positive")
	assert.That(limit%time.Second == 0, "limits are whole seconds")
	count, unit := int64(limit/time.Second), "second"
	if limit%time.Minute == 0 {
		count, unit = int64(limit/time.Minute), "minute"
	}
	if count != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", count, unit)
}

// Scenario: Steps done by hand come first

// firstYouLine matches a `first_you '…'` line in a setup script: a Given done by hand, which
// the checklist shows first. Quiet mode hides the kit's own "First, …" line.
const firstYouPattern = `(?m)^\s*first_you\s+(.+?)\s*$`

var firstYouLine = regexp.MustCompile(firstYouPattern) //tiger:allow-global compiled once

// unquote takes the quotes off one bash word, as the kit would see it: 'x' or "x".
func unquote(word string) string {
	assert.That(!strings.Contains(word, "\n"), "a word is on one line")
	if len(word) >= 2 && word[0] == '\'' && word[len(word)-1] == '\'' {
		return word[1 : len(word)-1]
	}
	if len(word) >= 2 && word[0] == '"' && word[len(word)-1] == '"' {
		return strings.ReplaceAll(word[1:len(word)-1], `\"`, `"`)
	}
	return word
}

// Scenario: Record a run
// Scenario: Record a run before marking every check
// Scenario: Record a run with a note in Markdown
// Scenario: A run records who ran it

// Marks.
const (
	MarkDone = "done"
	MarkPass = "pass"
	MarkFail = "fail"
	// MarkClear takes a mark off, like unticking a When step.
	MarkClear = "clear"
)

// MessageMarkEachCheck is shown when "Record run" comes before every check is marked.
const MessageMarkEachCheck = "Mark each check first"

// Mark marks a checklist item: "done" for Givens and Whens, "pass" or "fail" for Thens.
func (s *Session) Mark(item string, mark string) error {
	assert.That(s != nil, "mark runs on a session")
	assert.That(mark == MarkDone || mark == MarkPass || mark == MarkFail || mark == MarkClear,
		"a mark is done, pass, fail or clear")
	s.poll()
	for index := range s.checklist {
		if s.checklist[index].Text != item {
			continue
		}
		// A changed mark is a checklist that "Record run" hasn't saved yet.
		if mark == MarkClear {
			s.checklist[index].Mark, s.recorded = "", false
			return nil
		}
		then := s.checklist[index].Kind == "then"
		if then == (mark == MarkDone) {
			return fmt.Errorf("%q can't be marked %q", item, mark)
		}
		s.checklist[index].Mark, s.recorded = mark, false
		return nil
	}
	return fmt.Errorf("the checklist has no %q: %w", item, ErrNotFound)
}

// SetRunNote sets the note that "Record run" saves with the run. Needed by "Record a run".
func (s *Session) SetRunNote(note string) error {
	assert.That(s != nil, "set note runs on a session")
	assert.That(len(note) <= runs.NoteBytesMax, "a note fits NoteBytesMax")
	s.note = note
	return nil
}

// RecordRun records the run with note, which may be "": one line in today's file in runs/,
// passing when every Then step passed. A note of more than one line is kept whole as note.md
// in the run's folder, and the line keeps its first line. Until every check is marked it
// records nothing and shows MessageMarkEachCheck instead.
func (s *Session) RecordRun(note string) error {
	assert.That(s != nil, "record run runs on a session")
	assert.That(len(note) <= runs.NoteBytesMax, "a note fits NoteBytesMax")
	if s.Status() != SessionReady {
		return errors.New("setup hasn't finished, so there's no run to record")
	}
	if warning := examplesWarning(s.scenario); warning != "" {
		return errors.New(warning)
	}
	result := runs.ResultPass
	for _, item := range s.checklist {
		if item.Mark == "" {
			s.message = MessageMarkEachCheck
			return nil
		}
		if item.Mark == MarkFail {
			result = runs.ResultFail
		}
	}
	if note == "" {
		note = s.note
	}
	window := s.window
	parts, err := FingerprintOf(window.project, s.scenario, s.row)
	if err != nil {
		return err
	}
	run := runs.Run{
		Time: window.app.config.Clock.Now(), Key: s.key(), Result: result,
		Author: runs.Author(filepath.Dir(window.path)), Values: s.row,
		Fingerprint: parts, Note: note,
	}
	// Where the run goes is Keep runs off main's: the ref, or capabilities/runs.
	err = window.keepRun(run, s.attachments, note)
	if err != nil {
		return err
	}
	s.message, s.recorded = "", true
	return nil
}

// key is the run key this Drop in records against.
func (s *Session) key() string {
	assert.That(s.scenario.Slug != "", "the scenario has a slug")
	assert.That(s.scenario.Capability != "", "the scenario belongs to a capability")
	return runs.Key(s.scenario.Key(), s.examples)
}

// Scenario: Attach proof to a run

// Files attached to a Drop in: proof, like a screenshot, that "Record run" copies into the
// run's own folder in runs/. See internal/runs/files.go for where they go.

// Attach adds the file at path to the files "Record run" keeps with the run. A file with the
// same name as one already attached takes its place. Needed by "Attach proof to a run".
func (s *Session) Attach(path string) error {
	assert.That(s != nil, "attach runs on a session")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	if !filepath.IsAbs(path) {
		return fmt.Errorf("an attached file needs an absolute path, not %q", path)
	}
	if err := runs.CheckFile(path); err != nil {
		return err
	}
	name := filepath.Base(path)
	if name == runs.NoteFileName {
		return fmt.Errorf("%s is the run note's own name; rename the file to attach it", name)
	}
	kept := slices.DeleteFunc(s.attachments, func(attached string) bool {
		return filepath.Base(attached) == name
	})
	if len(kept) >= runs.FilesPerRunMax {
		return fmt.Errorf("a run keeps at most %d files", runs.FilesPerRunMax)
	}
	s.attachments = append(kept, path)
	assert.That(len(s.attachments) <= runs.FilesPerRunMax, "attachments stay bounded")
	return nil
}

// Detach takes the attached file called name off again.
func (s *Session) Detach(name string) error {
	assert.That(s != nil, "detach runs on a session")
	assert.That(len(s.attachments) <= runs.FilesPerRunMax, "attachments stay bounded")
	before := len(s.attachments)
	s.attachments = slices.DeleteFunc(s.attachments, func(attached string) bool {
		return filepath.Base(attached) == name
	})
	if len(s.attachments) == before {
		return fmt.Errorf("no file called %q is attached: %w", name, ErrNotFound)
	}
	return nil
}

// Attachments lists the paths of the attached files, in the order they were attached.
func (s *Session) Attachments() []string {
	assert.That(s != nil, "attachments runs on a session")
	assert.That(len(s.attachments) <= runs.FilesPerRunMax, "attachments stay bounded")
	return append(make([]string, 0, len(s.attachments)), s.attachments...)
}

// runFiles lists the paths of the files kept with run, for its line in the history. A folder
// that can't be read lists none: the run itself still shows.
func runFiles(capabilities string, run runs.Run) []string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(run.Key != "", "a run has a key")
	names, err := runs.Files(capabilities, run)
	if err != nil {
		return []string{}
	}
	folder := runs.FolderOf(capabilities, run)
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(folder, name))
	}
	return paths
}

// Scenario: Add a teardown

// addTeardown creates the selected scenario's scripts/<slug>.teardown.sh, sourcing the kit,
// and opens it in the editor. Givens runs it before every setup run.
func (w *Window) addTeardown() error {
	assert.That(w != nil, "add teardown runs on a window")
	assert.That(w.selectedScenario != "", "a scenario is selected")
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok || scenario.Setup == "" {
		return fmt.Errorf("%s has no setup to tear down: %w", w.selectedScenario, ErrNotFound)
	}
	path := strings.TrimSuffix(scenario.Setup, ".sh") + ".teardown.sh"
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	text := KitSourceLine + "\n# Clears what " + scenario.Name +
		"'s setup leaves behind. It runs before every setup run,\n" +
		"# so it must cope when there's nothing to clear.\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.editFile(path)
}

// Scenario: Builds on a scenario that doesn't exist
// Scenario: Setups that build on each other in a loop
// Scenario: Build on an outline without a row

// fromProblem finds what's wrong with the setups a scenario builds on: a from line naming no
// scenario, an outline named without a row, or setups that build on each other in a loop.
func (w *Window) fromProblem(scenario project.Scenario) (string, error) {
	assert.That(w.project != nil, "from lines are read in a loaded project")
	assert.That(scenario.Setup != "", "the scenario has a setup script")
	type visit struct {
		scenario project.Scenario
		path     []string // slugs from the scenario dropped in to
	}
	stack := []visit{{scenario: scenario, path: []string{scenario.Slug}}}
	visits := 0
	// A loop is reported the first time a slug repeats on a path.
	// bound: visits stop at the most scenarios a project can hold.
	for len(stack) > 0 && visits <= spec.CapabilitiesMax*spec.ScenariosPerFeatureMax {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visits++
		if current.scenario.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.scenario.Setup)
		if err != nil {
			return "", err
		}
		for _, from := range helpers.Froms(text) {
			built, err := w.project.Find(from.Key)
			if err != nil {
				return "from " + from.Key + ": there's no scenario with that name", nil
			}
			if problem := outlineWithoutRow(built, from.Values); problem != "" {
				return problem, nil
			}
			path := append(append([]string{}, current.path...), built.Slug)
			if slices.Contains(current.path, built.Slug) {
				return "These setups build on each other in a loop: " +
					strings.Join(path, " → "), nil
			}
			stack = append(stack, visit{scenario: built, path: path})
		}
	}
	return "", nil
}

// outlineWithoutRow says an outline was named without a row to build on, or "".
func outlineWithoutRow(built project.Scenario, values []string) string {
	assert.That(built.Slug != "", "the scenario has a slug")
	assert.That(len(values) <= helpers.HelpersMax, "values stay bounded")
	if !built.Outline || len(values) > 0 {
		return ""
	}
	example := ""
	for _, block := range built.Examples {
		if len(block.Header) > 0 && len(block.Rows) > 0 && example == "" {
			example = ", like " + block.Header[0] + "=" + block.Rows[0][0]
		}
	}
	return built.Slug + " is an outline: pick a row" + example
}

// Scenario: Steps stay in the order they're written

// ChecklistPart is one numbered part of the Drop in page: a run of checklist items of one kind.
// Records.swift's ChecklistPart.
type ChecklistPart struct {
	Kind  string            `json:"kind"` // "given" (done by hand), "when" or "then"
	Items []model.CheckItem `json:"items"`
}

// Parts cuts the checklist into runs of one kind, keeping its order, so a scenario that goes
// When, Then, When, Then shows four parts, each Then right after the When it checks. Needed
// by "Steps stay in the order they're written".
func (s *Session) Parts() []ChecklistPart {
	assert.That(s != nil, "parts runs on a session")
	checklist := s.Checklist()
	parts := make([]ChecklistPart, 0, len(checklist))
	for _, item := range checklist {
		if last := len(parts) - 1; last >= 0 && parts[last].Kind == item.Kind {
			parts[last].Items = append(parts[last].Items, item)
			continue
		}
		parts = append(parts, ChecklistPart{Kind: item.Kind, Items: []model.CheckItem{item}})
	}
	assert.That(len(parts) <= len(checklist), "each part holds at least one item")
	return parts
}

// Scenario: Reset state part way through

// The Drop in page: what it shows and offers, and what its buttons do.

// snapshot is the Drop in page as the window shows it.
func (s *Session) snapshot() *SessionSnapshot {
	assert.That(s != nil, "snapshot runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return &SessionSnapshot{
		Scenario: s.scenario.Name, Status: s.Status(), Output: s.Output(),
		Cards: s.cardLines(), Checklist: s.Checklist(), Parts: s.Parts(),
		Attachments: s.Attachments(), Recorded: s.recorded,
	}
}

// messages are the lines across the top of the Drop in page.
func (s *Session) messages() []string {
	assert.That(s != nil, "messages runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	messages := make([]string, 0, 2)
	if s.message != "" {
		messages = append(messages, s.message)
	}
	if still := s.stillRunning(); still != "" {
		messages = append(messages, still)
	}
	if s.status == SessionFailed {
		if where := s.window.elsewhere(s.scenario, s.Output()); where != "" {
			messages = append(messages, where)
		}
	}
	return messages
}

// offers are the Drop in page's buttons for where setup stands.
func (s *Session) offers() []string {
	assert.That(s != nil, "offers runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	offers := make([]string, 0, 4)
	if pending := s.pendingCard(); pending >= 0 {
		offers = append(offers, s.cards[pending].Offers...)
	}
	switch s.Status() {
	case SessionRunning:
		if s.stillRunning() != "" {
			offers = append(offers, OfferKeepWaiting)
		}
		offers = append(offers, OfferStop)
	case SessionReady:
		// Unnamed Examples blocks would share their runs, so a row can't be recorded.
		if examplesWarning(s.scenario) == "" {
			offers = append(offers, OfferRecordRun)
		}
		offers = append(offers, OfferResetState)
	case SessionFailed, SessionStopped:
		offers = append(offers, s.failedOffers()...)
	}
	if len(s.hints) > 0 || s.untrusted {
		offers = append(offers, OfferCopy)
	}
	return offers
}

// choose takes one of the Drop in page's offers.
func (s *Session) choose(action string) error {
	assert.That(s != nil, "choose runs on a session")
	assert.That(action != "", "an action has a label")
	window := s.window
	if pending := s.pendingCard(); pending >= 0 &&
		slices.Contains(s.cards[pending].Offers, action) {
		return s.ChooseOnCard(pending, action)
	}
	switch action {
	case OfferStop:
		s.scripts = nil
		if s.block != nil {
			return s.stopBlock()
		}
		if s.process == nil {
			return nil
		}
		return s.process.Stop()
	case OfferKeepWaiting:
		s.keepWaiting = true
		return nil
	case OfferTryAgain, OfferResetState:
		return s.restart(false)
	case OfferRunSetupAnyway:
		return s.restart(true)
	case OfferEditScript, OfferEditTeardown, OfferEditSetup:
		// The page draws the Drop in over the editor, so leave it first. These are offered
		// only once setup has ended, so there are no marks to lose.
		path := scenarioFileOf(action)(s.scenario)
		if err := window.leaveSession(); err != nil {
			return err
		}
		return window.editFile(path)
	case OfferRecordRun:
		return s.RecordRun(s.note)
	case OfferCopy:
		if len(s.hints) == 0 {
			return window.copySetup(s.scenario)
		}
		return window.app.config.Clipboard.WriteText(s.hints[0])
	case OfferTrustThisProject:
		return s.trustThisProject()
	}
	return ErrNotImplemented
}

// failedOffers are the Drop in page's buttons once setup has failed or been stopped.
func (s *Session) failedOffers() []string {
	assert.That(s.status == SessionFailed || s.status == SessionStopped, "setup has ended")
	assert.That(s.scenario.Name != "", "the session has a scenario")
	if s.teardown && s.status == SessionFailed {
		return []string{OfferEditTeardown, OfferRunSetupAnyway}
	}
	if s.markdownFailed {
		return []string{OfferEditSetup, OfferTryAgain}
	}
	if s.untrusted {
		return []string{OfferTrustThisProject}
	}
	offers := make([]string, 0, 2)
	if s.scenario.Setup != "" {
		offers = append(offers, OfferEditScript)
	}
	return append(offers, OfferTryAgain)
}

// scenarioFileOf says which of a scenario's setup files an edit offer opens, or nil for any
// other offer.
func scenarioFileOf(action string) func(project.Scenario) string {
	assert.That(action != "", "an action has a label")
	assert.That(OfferEditScript != OfferEditSetup, "each file has its own offer")
	switch action {
	case OfferEditScript:
		return func(scenario project.Scenario) string { return scenario.Setup }
	case OfferEditSetup:
		return func(scenario project.Scenario) string { return scenario.SetupCards }
	case OfferEditTeardown:
		return func(scenario project.Scenario) string { return scenario.Teardown }
	}
	return nil
}

// leaveSession closes the Drop in page: setup still running is stopped, and its arguments
// folder removed.
func (w *Window) leaveSession() error {
	assert.That(w.session != nil, "there's a Drop in to leave")
	session := w.session
	assert.That(session.window == w, "the Drop in is this window's")
	w.session = nil
	session.scripts = nil
	if err := session.stopBlock(); err != nil {
		return err
	}
	session.removeArguments()
	if session.process == nil {
		return nil
	}
	return session.process.Stop()
}

// Scenario: A step done by hand
// Scenario: A step done by hand fails
// Scenario: A value from elsewhere
// Scenario: Continue with nothing typed
// Scenario: A step by hand in a scenario it builds on

// Card actions.
const (
	OfferDone      = "Done"
	OfferFailed    = "Failed"
	OfferOpen      = "Open"
	OfferContinue  = "Continue"
	OfferEditSetup = "Edit setup"
)

// Messages on the Drop in page about cards.
const (
	MessageEnterValue      = "Enter a value first"
	MessageStoppedAtByHand = "Setup stopped at a step done by hand"
	// MessageNoStepsYet follows the markdown setup's file name, as in "pay-in-app.md has no
	// steps yet".
	MessageNoStepsYet = " has no steps yet"
)

// cardHeader starts the line the kit prints for a card under GIVENS_UI=1: the record
// separator, then "givens-card". See _givens_card in internal/kit/givens.sh.
const cardHeader = "\x1egivens-card\t"

// Cards lists the Drop in page's setup cards in the order setup reached them: by_hand and ask
// cards from the script, then the markdown setup's cards. Needed by every card scenario.
func (s *Session) Cards() []model.SetupCard {
	assert.That(s != nil, "cards runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return append(make([]model.SetupCard, 0, len(s.cards)), s.cards...)
}

// readOutput moves the current script's new output into display, taking card lines out and
// turning them into cards.
func (s *Session) readOutput() {
	assert.That(s != nil, "read output runs on a session")
	assert.That(s.process != nil, "a script is running")
	lines := s.process.Output()
	for _, line := range lines[s.parsed:] {
		switch {
		case s.reading > 0:
			last := &s.cards[len(s.cards)-1]
			last.Text = strings.TrimPrefix(last.Text+"\n"+line, "\n")
			s.snippets[len(s.snippets)-1] = last.Text
			s.reading--
		case strings.HasPrefix(line, cardHeader):
			s.addScriptCard(strings.Split(strings.TrimPrefix(line, cardHeader), "\t"))
		default:
			s.display = append(s.display, line)
		}
	}
	s.parsed = len(lines)
	if s.reading == 0 && s.answering() && s.status == SessionRunning {
		s.status, s.waitingSince = SessionWaiting, s.window.app.config.Clock.Now()
	}
}

// addScriptCard adds a by_hand or ask card from the kit's fields: kind, where, link, and how
// many lines of text follow.
func (s *Session) addScriptCard(fields []string) {
	assert.That(s != nil, "add card runs on a session")
	assert.That(len(s.cards) <= model.SetupCardsMax, "cards stay bounded")
	if len(fields) != 4 || len(s.cards) >= model.SetupCardsMax {
		return
	}
	lines, err := strconv.Atoi(fields[3])
	if err != nil || lines < 0 {
		lines = 0
	}
	card := model.SetupCard{
		Kind: fields[0], Link: fields[2], Links: []string{}, Images: []string{},
		Blocks: []model.CodeBlock{}, Output: []string{},
	}
	if card.Kind == model.CardAsk {
		card.Offers = []string{OfferContinue}
	} else {
		card.Kind, card.Where = model.CardByHand, fields[1]
		card.Offers = []string{OfferDone, OfferFailed}
		if lines > 0 {
			card.Offers = append(card.Offers, OfferCopy)
		}
		if card.Link != "" {
			card.Offers = append(card.Offers, OfferOpen)
		}
	}
	s.cards, s.snippets, s.reading = append(s.cards, card), append(s.snippets, ""), lines
}

// answering is true while the last script card waits for an answer.
func (s *Session) answering() bool {
	assert.That(s != nil, "answering runs on a session")
	assert.That(len(s.cards) == len(s.snippets), "each card has a snippet slot")
	if len(s.cards) == 0 {
		return false
	}
	last := s.cards[len(s.cards)-1]
	return last.Kind != model.CardMarkdown && last.Mark == ""
}

// finishCardText finishes a script card once its text is read: an ask card's prompt.
func (s *Session) finishCardText() {
	assert.That(s != nil, "finish card runs on a session")
	assert.That(s.reading == 0, "the card's text is all read")
	if len(s.cards) == 0 {
		return
	}
	last := &s.cards[len(s.cards)-1]
	if last.Kind == model.CardAsk && last.Prompt == "" {
		last.Prompt = last.Text
	}
}

// ChooseOnCard takes an action offered on one card, by the card's index in Cards: "Done",
// "Failed", "Copy", "Open" or "Continue".
func (s *Session) ChooseOnCard(card int, action string) error {
	assert.That(s != nil, "choose on card runs on a session")
	assert.That(action != "", "an action has a label")
	s.poll()
	if card < 0 || card >= len(s.cards) {
		return fmt.Errorf("there's no card %d", card)
	}
	chosen := &s.cards[card]
	switch action {
	case OfferCopy:
		return s.window.app.config.Clipboard.WriteText(s.snippets[card])
	case OfferOpen:
		return s.window.openLink(chosen.Link)
	}
	if chosen.Mark != "" {
		return fmt.Errorf("card %d is already marked %q", card, chosen.Mark)
	}
	if chosen.Kind == model.CardMarkdown {
		return s.markMarkdownCard(card, action)
	}
	return s.answerScriptCard(chosen, action)
}

// answerScriptCard answers the waiting by_hand or ask card on the script's stdin.
func (s *Session) answerScriptCard(chosen *model.SetupCard, action string) error {
	assert.That(chosen != nil, "a card is chosen")
	assert.That(chosen.Kind != model.CardMarkdown, "script cards are answered on stdin")
	if s.process == nil || s.status != SessionWaiting {
		return errors.New("setup isn't waiting at this card")
	}
	answer, mark := "done", MarkDone
	switch {
	case action == OfferContinue && chosen.Kind == model.CardAsk:
		if s.cardInput == "" {
			s.message = MessageEnterValue
			return nil
		}
		answer = s.cardInput
	case action == OfferFailed && chosen.Kind == model.CardByHand:
		answer, mark, s.cardFailed = "failed", "failed", true
	case action != OfferDone || chosen.Kind != model.CardByHand:
		return fmt.Errorf("%q isn't offered on this card", action)
	}
	if err := s.process.Answer(answer); err != nil {
		return err
	}
	chosen.Mark, s.cardInput, s.message = mark, "", ""
	s.resume()
	return nil
}

// pendingCard is the card the window's own buttons answer: the script card waiting for an
// answer, or the first markdown card not yet marked; -1 when none waits.
func (s *Session) pendingCard() int {
	assert.That(s != nil, "pending card runs on a session")
	assert.That(len(s.cards) <= model.SetupCardsMax, "cards stay bounded")
	if s.status != SessionWaiting {
		return -1
	}
	for index, card := range s.cards {
		if card.Mark == "" {
			return index
		}
	}
	return -1
}

// PendingCard is the card waiting on the developer, as an index into Cards, or -1.
func (s *Session) PendingCard() int {
	assert.That(s != nil, "pending card runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return s.pendingCard()
}

// Scenario: A step in the browser

// UseURLOpener sets what opens links, like a by_hand --url card's "Open". Needed by
// "A step in the browser".
func (a *App) UseURLOpener(opener model.URLOpener) error {
	assert.That(a != nil, "use url opener runs on an app")
	assert.That(opener != nil, "an opener is given")
	a.opener = opener
	return nil
}

// Scenario: Setup steps shown as cards
// Scenario: Cards after a script
// Scenario: A card marked failed
// Scenario: Setup steps not written yet
// Scenario: A code block that never closes

// addMarkdownCards adds one card per numbered step of scripts/<slug>.md, once its scripts
// have run, and waits for each to be marked.
func (s *Session) addMarkdownCards() error {
	assert.That(s.scenario.SetupCards != "", "the scenario has a markdown setup")
	assert.That(s.markdownFrom < 0, "markdown cards are added once")
	text, err := helpers.ReadScript(s.scenario.SetupCards)
	if err != nil {
		return err
	}
	parsed, err := markdown.Parse(text)
	if err != nil {
		return err
	}
	// With no step there's no card to mark, so waiting could never end. "Write setup steps"
	// starts the file as "1." alone, which is an empty list item, not a step.
	// Ref: https://spec.commonmark.org/0.31.2/#list-items
	if len(parsed.Steps) == 0 {
		s.fail(filepath.Base(s.scenario.SetupCards) + MessageNoStepsYet)
		s.markdownFailed = true
		return nil
	}
	s.intro, s.markdownFrom = parsed.Intro, len(s.cards)
	for _, step := range parsed.Steps {
		card := model.SetupCard{
			Kind: model.CardMarkdown, Text: markdown.Rendered(step.Text),
			Links:  append([]string{}, step.Links...),
			Images: append([]string{}, step.Images...), Blocks: make([]model.CodeBlock, 0, 2),
			Offers: []string{OfferDone, OfferFailed}, Inline: step.Inline,
			Output: []string{},
		}
		for index, block := range step.Blocks {
			runs := runLanguage(block) != ""
			card.Blocks = append(card.Blocks, model.CodeBlock{
				Language: block.Language, Code: block.Code, Copyable: block.Closed, Run: runs,
			})
			if runs {
				card.RunBlocks = append(card.RunBlocks, index)
			}
			if !block.Closed {
				card.Warning = fmt.Sprintf("The code block starting on line %d never closes",
					block.Line)
			}
		}
		// A card that runs marks itself, so it has nothing to choose.
		if len(card.RunBlocks) > 0 {
			card.Offers = []string{}
		}
		s.cards, s.snippets = append(s.cards, card), append(s.snippets, "")
	}
	return s.runNextCard()
}

// markMarkdownCard marks one markdown card; the last Done shows the checklist, and a Failed
// stops setup there.
func (s *Session) markMarkdownCard(card int, action string) error {
	assert.That(s.cards[card].Kind == model.CardMarkdown, "the card is a markdown step")
	assert.That(s.markdownFrom >= 0, "markdown cards were added")
	steps := len(s.cards) - s.markdownFrom
	switch action {
	case OfferDone:
		s.cards[card].Mark = MarkDone
	case OfferFailed:
		s.cards[card].Mark = "failed"
		s.resume()
		s.fail(fmt.Sprintf("Setup stopped at step %d of %d", card-s.markdownFrom+1, steps))
		s.markdownFailed = true
		return nil
	default:
		return fmt.Errorf("%q isn't offered on a markdown card", action)
	}
	return s.runNextCard()
}

// cardLines are the cards as the Drop in page shows them: the markdown setup's intro first,
// then each card's heading, text, links and warning.
func (s *Session) cardLines() []string {
	assert.That(s != nil, "card lines run on a session")
	assert.That(len(s.cards) <= model.SetupCardsMax, "cards stay bounded")
	lines := make([]string, 0, 4*len(s.cards)+1)
	if s.intro != "" {
		lines = append(lines, s.intro)
	}
	for index, card := range s.cards {
		switch card.Kind {
		case model.CardByHand:
			lines = append(lines, "By hand, in "+card.Where)
		case model.CardMarkdown:
			lines = append(lines, fmt.Sprintf("Step %d", index-s.markdownFrom+1)+
				s.runState(index))
		}
		lines = append(append(append(lines, card.Text), card.Images...), card.Output...)
		if card.Warning != "" {
			lines = append(lines, card.Warning)
		}
	}
	return lines
}

// Scenario: Copy a code block
// Scenario: Several code blocks

// CopyCodeBlock copies one code block of one card to the clipboard: exactly its code, with no
// fences, list number or indent. Needed by "Copy a code block" and "Several code blocks".
func (s *Session) CopyCodeBlock(card int, block int) error {
	assert.That(s != nil, "copy code block runs on a session")
	assert.That(block >= 0 && block < model.CodeBlocksMax, "a block index is in range")
	if card < 0 || card >= len(s.cards) || block >= len(s.cards[card].Blocks) {
		return fmt.Errorf("card %d has no code block %d", card, block)
	}
	if !s.cards[card].Blocks[block].Copyable {
		return errors.New("the code block never closes, so it has no Copy button")
	}
	return s.window.app.config.Clipboard.WriteText(s.cards[card].Blocks[block].Code)
}

// Scenario: Copy inline code

// ClickInlineCode copies a card's inline code span, like `sam@example.com`, to the clipboard.
// Needed by "Copy inline code".
func (s *Session) ClickInlineCode(card int, code string) error {
	assert.That(s != nil, "click inline code runs on a session")
	assert.That(code != "", "inline code has text")
	if card < 0 || card >= len(s.cards) {
		return fmt.Errorf("there's no card %d", card)
	}
	for _, span := range s.cards[card].Inline {
		if span == code {
			return s.window.app.config.Clipboard.WriteText(code)
		}
	}
	return fmt.Errorf("card %d has no inline code %q", card, code)
}

// CopyCard copies a card's code: its first closed code block, or else its first inline code
// span. The givens command's TUI does it on c, where the app has a button per block and span.
func (s *Session) CopyCard(card int) error {
	assert.That(s != nil, "copy card runs on a session")
	assert.That(len(s.cards) <= model.SetupCardsMax, "cards stay bounded")
	if card < 0 || card >= len(s.cards) {
		return fmt.Errorf("there's no card %d", card)
	}
	for index, block := range s.cards[card].Blocks {
		if block.Copyable {
			return s.CopyCodeBlock(card, index)
		}
	}
	if len(s.cards[card].Inline) > 0 {
		return s.ClickInlineCode(card, s.cards[card].Inline[0])
	}
	return errors.New("this card has no code to copy")
}

// Scenario: See every row on one page
// Scenario: Drop in to one row
// Scenario: Switch to another row

// Row actions and the question before leaving a row with marks.
const (
	OfferSwitchRow = "Switch to this row"
	OfferLeave     = "Leave"
)

// fillPlaceholders puts a row's values in place of its <column> placeholders.
func fillPlaceholders(text string, row []runs.Pair) string {
	assert.That(len(row) <= runs.FieldsMax, "a row stays bounded")
	assert.That(len(text) <= NameLengthMax*64, "a step stays bounded")
	for _, value := range row {
		text = strings.ReplaceAll(text, "<"+value.Name+">", value.Value)
	}
	return text
}

// rowNamed finds the selected outline's row with value among its values, or "column=value".
func (w *Window) rowNamed(name string) (project.Scenario, outlineRow, bool) {
	assert.That(w != nil, "row named runs on a window")
	assert.That(name != "", "a row is named by a value")
	if w.selectedScenario == "" {
		return project.Scenario{}, outlineRow{}, false
	}
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok || !scenario.Outline {
		return project.Scenario{}, outlineRow{}, false
	}
	for _, row := range rowsOf(scenario) {
		for _, value := range row.values {
			if value.Value == name || value.Name+"="+value.Value == name {
				return scenario, row, true
			}
		}
	}
	return project.Scenario{}, outlineRow{}, false
}

// DropInRows lists the Examples rows of the selected outline's Drop in page, one after
// another, in file order. Only one row is live at a time; the others are greyed out, with
// "Switch to this row" in place of "Drop in". Needed by the outline scenarios.
func (w *Window) DropInRows() []model.DropInRow {
	assert.That(w != nil, "drop in rows runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.selectedScenario == "" {
		return []model.DropInRow{}
	}
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok || !scenario.Outline {
		return []model.DropInRow{}
	}
	live := w.session != nil && w.session.scenario.Key() == scenario.Key() &&
		w.session.row != nil
	shown := make([]model.DropInRow, 0, 4)
	for _, row := range rowsOf(scenario) {
		entry := model.DropInRow{
			Examples: row.examples, Values: make(map[string]string, len(row.values)),
			Steps:  make([]string, 0, len(scenario.Steps)),
			Status: rowStatus(w.project, scenario, row, w.app.config.Clock.Now()),
			Offers: []string{OfferDropIn}, Disabled: []string{}, Checklist: []model.CheckItem{},
		}
		for _, value := range row.values {
			entry.Values[value.Name] = value.Value
		}
		for _, step := range scenario.Steps {
			entry.Steps = append(entry.Steps, step.Keyword+" "+fillPlaceholders(step.Text,
				row.values))
		}
		if live && slices.Equal(w.session.row, row.values) {
			entry.Live, entry.Checklist = true, w.session.Checklist()
		} else if live {
			entry.Greyed, entry.Offers = true, []string{OfferSwitchRow}
			entry.Disabled = []string{OfferDropIn}
		}
		shown = append(shown, entry)
	}
	return shown
}

// ChooseOnRow takes an action on one Examples row of the selected outline: "Drop in" or
// "Switch to this row". row is one of the row's values, like "expired". Switching away from a
// row with marks asks first, since its marks aren't recorded. Needed by "Drop in to one row"
// and "Switch to another row".
func (w *Window) ChooseOnRow(row string, action string) error {
	assert.That(w != nil, "choose on row runs on a window")
	assert.That(row != "", "a row is named by one of its values")
	scenario, found, ok := w.rowNamed(row)
	if !ok {
		return fmt.Errorf("the selected scenario has no row %q: %w", row, ErrNotFound)
	}
	switch action {
	case OfferDropIn:
		_, err := w.startSession(scenario, found, false)
		return err
	case OfferSwitchRow:
		if w.session == nil || !w.session.marked() {
			_, err := w.startSession(scenario, found, false)
			return err
		}
		question := fmt.Sprintf("Leave the %s row? Its marks aren't recorded",
			rowName(w.session.row))
		w.ask(question, OfferLeave, func() error {
			_, err := w.startSession(scenario, found, false)
			return err
		})
		return nil
	}
	return fmt.Errorf("%q isn't offered on a row", action)
}

// marked is true once any checklist item has a mark.
func (s *Session) marked() bool {
	assert.That(s != nil, "marked runs on a session")
	assert.That(len(s.checklist) <= model.SetupCardsMax*4, "the checklist stays bounded")
	for _, item := range s.checklist {
		if item.Mark != "" {
			return true
		}
	}
	return false
}

// Scenario: Record a run for one row

// outlineStatus sums up an outline's rows: "Never run" until one has run, "Verified …" once
// all are, and "1 of 2 rows verified" between.
func outlineStatus(loaded *project.Project, scenario project.Scenario, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(scenario.Outline, "only an outline has rows")
	rows := rowsOf(scenario)
	verifiedRows, everRun, last := 0, 0, StatusNeverRun
	for _, row := range rows {
		status := rowStatus(loaded, scenario, row, now)
		if status != StatusNeverRun {
			everRun++
		}
		if strings.HasPrefix(status, statusVerified) {
			verifiedRows, last = verifiedRows+1, status
		}
	}
	switch {
	case everRun == 0:
		return StatusNeverRun
	case verifiedRows == len(rows):
		return last
	}
	return fmt.Sprintf("%d of %d rows verified", verifiedRows, len(rows))
}

// Scenario: Step data reaches the script

// writeArguments writes each doc string and data table in the scenario's steps, Background
// first, to a file of its own, and returns GIVENS_ARG_<n>=<path> for each, counting from 1.
// A data table is CSV with its header row first. The files live in a temporary folder the
// next Drop in in this window removes.
// Ref: https://pkg.go.dev/encoding/csv
func (s *Session) writeArguments() ([]string, error) {
	assert.That(s != nil, "arguments are written for a session")
	assert.That(s.window != nil, "a session belongs to a window")
	steps := append(append(s.scenario.Background[:0:0], s.scenario.Background...),
		s.scenario.Steps...)
	env := make([]string, 0, 2)
	for _, step := range steps {
		if step.DocString == "" && step.Table == nil {
			continue
		}
		if _, err := s.scratchDir(); err != nil {
			return nil, err
		}
		number := strconv.Itoa(len(env) + 1)
		path := filepath.Join(s.argumentsDir, "argument-"+number)
		if err := writeArgument(path, step.DocString, step.Table); err != nil {
			return nil, err
		}
		env = append(env, "GIVENS_ARG_"+number+"="+path)
	}
	assert.That(len(env) <= len(steps), "each argument comes from a step")
	return env, nil
}

// writeArgument writes a doc string as it is, or a table as CSV.
func writeArgument(path string, docString string, table [][]string) error {
	assert.That(filepath.IsAbs(path), "the argument file is absolute")
	assert.That(len(docString) <= helpers.FileSizeBytesMax, "a doc string stays bounded")
	if table == nil {
		return os.WriteFile(path, []byte(docString), 0o644)
	}
	var text strings.Builder
	writer := csv.NewWriter(&text)
	if err := writer.WriteAll(table); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text.String()), 0o644)
}

// Scenario: Several unnamed Examples blocks

// MessageNameExamples is the warning on an outline whose Examples blocks can't be told apart.
const MessageNameExamples = "Name each Examples block so runs can tell them apart"

// examplesWarning is the warning on an outline with more than one unnamed Examples block: runs
// are recorded against the block's name, so unnamed blocks would share them.
func examplesWarning(scenario project.Scenario) string {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Examples) <= project.HelperFilesMax, "blocks stay bounded")
	unnamed := 0
	for _, block := range scenario.Examples {
		if block.Slug == "" {
			unnamed++
		}
	}
	if unnamed > 1 {
		return MessageNameExamples
	}
	return ""
}

// Feature: Gherkin files
//
// capabilities/gherkin-files. Givens reads each capability's one .feature file with the
// official Cucumber Gherkin parser (internal/spec, read into a project by internal/project), so
// a test runner and Givens read the same scenarios. A file that doesn't parse, or a second
// .feature file in a folder, is a warning on the capability with its fix. Code elsewhere:
// slugClash and scenarioWarning, which warns on a scenario as shown, are in big_helpers.go,
// since Write stories' rename and Drop in's @timeout and Examples warnings use them too;
// scenarioOffers (offers.go) offers only "Copy" on a clashing slug. capabilityOf and scenarioOf
// (project.go) and Sidebar (window.go) show the warnings; core glue's allOffers and Choose
// (offers.go) call gherkinOffers, editBrokenFeature and moveToOwnCapability, and
// OfferMoveToOwnCapability is with the page offers there. "Comments are kept" is Write
// stories' editor (Save, and Edit in big_helpers.go), "Step definitions are left alone" is
// internal/project's loader, and "A test runner reads the same files" is cucumber-js.

// Scenario: Rules group scenarios

// RuleGroup is one Rule: block of a capability, as the capability page groups it.
type RuleGroup struct {
	Name      string   `json:"name"`      // the text after "Rule:"
	Scenarios []string `json:"scenarios"` // the names of the scenarios under it, in file order
}

// RuleGroups lists a capability's Rule: blocks with the scenarios under each, in file order.
// Needed by "Rules group scenarios" in capabilities/gherkin-files.
func (w *Window) RuleGroups(capability string) ([]RuleGroup, error) {
	assert.That(w != nil, "rule groups run on a window")
	assert.That(utf8.ValidString(capability), "the capability's name is text")
	loaded, ok := w.findCapability(capability)
	if !ok {
		return nil, fmt.Errorf("%s: %w", capability, ErrNotFound)
	}
	groups := make([]RuleGroup, 0, 4)
	for _, scenario := range loaded.Scenarios {
		if scenario.Rule == "" {
			continue
		}
		last := len(groups) - 1
		if last < 0 || groups[last].Name != scenario.Rule {
			groups = append(groups, RuleGroup{Name: scenario.Rule, Scenarios: []string{}})
			last++
		}
		groups[last].Scenarios = append(groups[last].Scenarios, scenario.Name)
	}
	return groups, nil
}

// Scenario: A file that isn't valid Gherkin
// Scenario: Two .feature files in one capability

// capabilityWarning says what's wrong with a capability's folder, or "".
func capabilityWarning(loaded project.Capability) string {
	assert.That(loaded.Slug != "", "a capability has a slug")
	assert.That(loaded.Ignored != nil || loaded.ParseError != "", "ignored files were listed")
	if loaded.ParseError != "" {
		return loaded.ParseError
	}
	if len(loaded.Ignored) > 0 {
		return filepath.Base(loaded.Ignored[0]) +
			" isn't read: a capability has one .feature file, named after its folder"
	}
	return ""
}

// problemCapabilities lists capabilities with something to fix, the selected one first.
func (w *Window) problemCapabilities() []project.Capability {
	assert.That(w != nil, "problems runs on a window")
	assert.That(w.project != nil, "problems are in a loaded project")
	found := make([]project.Capability, 0, 4)
	for _, capability := range w.project.Capabilities {
		if capability.ParseError == "" && len(capability.Ignored) == 0 {
			continue
		}
		if capability.Slug == w.selectedCapability {
			found = append([]project.Capability{capability}, found...)
		} else {
			found = append(found, capability)
		}
	}
	return found
}

// gherkinOffers adds each fix a capability's warning carries to offers, once each.
func (w *Window) gherkinOffers(offers []string) []string {
	assert.That(w != nil, "gherkin offers run on a window")
	assert.That(w.project != nil, "fixes are offered in a loaded project")
	// A capability's warning in the sidebar carries its fix, so fixes are offered with
	// nothing selected; the selected capability's come first.
	for _, capability := range w.problemCapabilities() {
		for _, offer := range capabilityOffers(capability) {
			if !slices.Contains(offers, offer) {
				offers = append(offers, offer)
			}
		}
	}
	return offers
}

// capabilityOffers is what a capability's page offers: a way to fix what's wrong with it.
func capabilityOffers(capability project.Capability) []string {
	assert.That(capability.Slug != "", "a capability has a slug")
	assert.That(capability.FeaturePath != "", "a capability has a .feature file")
	if capability.ParseError != "" {
		return []string{editPrefix + filepath.Base(capability.FeaturePath)}
	}
	if len(capability.Ignored) > 0 {
		return []string{OfferMoveToOwnCapability}
	}
	return []string{}
}

// editPrefix starts an offer to open a file in the editor: "Edit checkout.feature".
const editPrefix = "Edit "

// editBrokenFeature takes an "Edit <file>" offer: it opens that .feature file of a capability
// with something to fix. edited is false for any other action.
func (w *Window) editBrokenFeature(action string) (bool, error) {
	assert.That(w != nil, "edit broken feature runs on a window")
	assert.That(action != "", "an action has a label")
	if file, ok := strings.CutPrefix(action, editPrefix); ok {
		for _, capability := range w.problemCapabilities() {
			if filepath.Base(capability.FeaturePath) == file {
				_, err := w.Edit(filepath.Join(capability.Slug, file))
				return true, err
			}
		}
	}
	return false, nil
}

// moveToOwnCapability moves an extra .feature file, the selected capability's first, into a
// folder of its own, named after its slug, so it's read as a capability.
func (w *Window) moveToOwnCapability() error {
	assert.That(w != nil, "move runs on a window")
	assert.That(w.project != nil, "a move happens in a loaded project")
	source := ""
	for _, capability := range w.problemCapabilities() {
		if source == "" && len(capability.Ignored) > 0 {
			source = capability.Ignored[0]
		}
	}
	if source == "" {
		return fmt.Errorf("no capability has an extra .feature file: %w", ErrNotFound)
	}
	name := slug.Of(strings.TrimSuffix(filepath.Base(source), ".feature"))
	if name == "" {
		return fmt.Errorf("%s makes no folder name", filepath.Base(source))
	}
	dir := filepath.Join(w.path, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("there's already a %s folder", name)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.Rename(source, filepath.Join(dir, name+".feature")); err != nil {
		return err
	}
	return w.reload()
}

// Feature: Keep runs off main
//
// capabilities/keep-runs-off-main. In a git repository with an origin remote, runs go to the
// git ref refs/givens/runs, which Givens fetches, merges and pushes itself; without one, they go
// in capabilities/runs. Code elsewhere: Drop in's RecordRun records through
// keepRun; Open a project's Open calls openRunStore, and its OpenBranch links a branch's copy to
// the ref (runs.LinkRefCopy), which is "Another branch shows runs kept off main"; refresh
// (big_helpers.go) calls pollRunsSync. Core glue shows runsMessages (Snapshot, snapshot.go),
// adds runsOffers and runs "Move runs off main", "Share runs" and "Change run limits"
// (allOffers and Choose, offers.go), and Select clears runsNote (selection.go). Window's runs
// fields are in window.go's type; internal/runref does the git work and internal/runs writes,
// reads and trims runs; `givens sync` is cmd/givens/panels.go.

// In a git repository with a remote, a project keeps its runs off main by default: in the git
// ref refs/givens/runs, which Givens fetches and pushes itself; internal/runref does the git
// work. Without a remote, runs go in capabilities/runs. See capabilities/keep-runs-off-main.

// Offers and messages about runs kept off main.
const (
	OfferMoveRunsOffMain = "Move runs off main"
	OfferShareRuns       = "Share runs"
	// OfferChangeRunLimits opens the dialog that takes the ref's limits for SetRunLimits.
	OfferChangeRunLimits = "Change run limits"
	// messageMoveOffMain asks before runs already in capabilities/runs are moved.
	messageMoveOffMain = "Move the runs in capabilities/runs to refs/givens/runs on " +
		runref.Remote + "? They go to the Trash here, and new runs already go to the ref."
	// MessageSyncingRuns shows while the sync a project's opening started is going.
	MessageSyncingRuns = "Syncing runs with " + runref.Remote
	// messageRunsNotSynced starts what the window says when the sync on open went wrong.
	messageRunsNotSynced = "Runs couldn't be synced: "
	// MessageRunsMoved says what's left for the developer to do: commit the change.
	MessageRunsMoved = "Runs moved off main. Commit the change to capabilities/runs: the " +
		"old run files went to the Trash, and off-main.md says where runs are."
)

// ErrRunsOnMain is returned by the limits calls in a project whose runs are in
// capabilities/runs, which has no limits.
var ErrRunsOnMain = errors.New("runs are kept off main only in a git repository with a " +
	"remote, and only there do they have limits")

// offMainText is capabilities/runs/off-main.md: what anyone looking in runs/ reads.
const offMainText = `# Runs are kept off main

Givens keeps this project's runs in the git ref ` + "`" + runref.Ref + "`" + `, not on a
branch. It fetches and pushes the ref itself, so runs aren't in pull requests, and a plain
clone doesn't download them. See capabilities/keep-runs-off-main in the Givens repository.

To look by hand:

    git fetch origin ` + runref.Ref + `
    git ls-tree -r FETCH_HEAD
`

// Scenario: Runs are kept off main by default
// Scenario: Without a remote, runs stay in capabilities/runs
// Scenario: Two devices record at the same time
// Scenario: Record a run while the remote can't be reached
// Scenario: Old runs are dropped

// keepRun writes a recorded run where the project keeps its runs: into the ref's copy and out
// to the remote when they're kept off main, or else in capabilities/runs. Drop in's RecordRun
// calls it.
func (w *Window) keepRun(run runs.Run, attachments []string, note string) error {
	assert.That(w != nil, "keep run runs on a window")
	assert.That(run.Key != "", "a run has a key")
	if w.keepsRunsOffMain() {
		// The run goes into the ref's copy, then out to the remote.
		write := func(tree string) error { return writeRun(tree, run, attachments, note) }
		return w.afterRunsChange(w.runStore.Change(write, runref.NetworkTimeout))
	}
	return writeRun(w.path, run, attachments, note)
}

// writeRun writes a run under root (the capabilities folder, or the ref's copy): its
// attached files first, so a line is never there without them, then its note, then its
// line. A second run of the scenario in the same second moves on a second, so it gets a
// files folder of its own.
func writeRun(root string, run runs.Run, attachments []string, note string) error {
	assert.That(filepath.IsAbs(root), "runs are written under an absolute folder")
	assert.That(run.Key != "", "a run has a key")
	at, err := runs.FreeTime(root, run.Key, run.Time)
	if err != nil {
		return err
	}
	run.Time = at
	if err := runs.CopyFiles(root, run, attachments); err != nil {
		return err
	}
	if strings.Contains(note, "\n") {
		run.Note = runs.Summary(note)
		if err := runs.WriteNote(root, run, note); err != nil {
			return err
		}
	}
	return runs.Append(root, run)
}

// KeepsRunsOffMain is true when the window's runs go to refs/givens/runs. `givens sync`
// says so when they don't.
func (w *Window) KeepsRunsOffMain() bool {
	assert.That(w != nil, "keeps runs off main runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.keepsRunsOffMain()
}

// keepsRunsOffMain is true when the window's runs go to the ref: in a git repository with a
// remote.
func (w *Window) keepsRunsOffMain() bool {
	assert.That(w != nil, "keeps runs off main runs on a window")
	assert.That(w.runStore == nil || w.branch == "", "only a working tree keeps runs")
	return w.runStore != nil
}

// afterRunsChange forgets statuses and histories, which the change may have moved on, and
// counts the runs still to share. A remote that can't be reached isn't an error: the runs are
// kept here, and the window says how many wait.
func (w *Window) afterRunsChange(changed error) error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(w.app != nil, "a window belongs to an app")
	w.statuses, w.histories = nil, nil
	unshared, err := w.runStore.Unshared()
	if err != nil {
		return errors.Join(changed, err)
	}
	w.runsUnshared = unshared
	if errors.Is(changed, runref.ErrUnreachable) {
		return nil
	}
	return changed
}

// Scenario: Runs show up on another device
// Scenario: Open a project whose remote doesn't answer

// openRunStore finds the window's runs store when its project is in a git repository with a
// remote, and starts a sync, so another device's runs show up. The sync runs on a goroutine
// of its own, so a remote that can't be reached doesn't hold the window up: Store keeps no
// state in memory, and the lock in its folder holds a Record run's change until the sync is
// done. With nothing here and nothing on the remote, the sync makes nothing.
// Ref: https://go.dev/ref/spec#Go_statements
func (w *Window) openRunStore() {
	assert.That(w != nil, "open run store runs on a window")
	assert.That(w.branch == "", "a branch's window keeps no runs of its own")
	store, ok := runref.Open(w.path)
	if !ok || !w.found || !store.HasRemote() {
		return
	}
	w.runStore = &store
	if w.app.config.NoSyncOnOpen {
		return
	}
	// Buffered, so the goroutine ends even when the window closes before the result is read.
	done := make(chan error, 1)
	w.runsSync = done
	go func() { done <- store.Change(nil, runref.OpenTimeout) }()
}

// pollRunsSync picks up the sync on open's result once it's done, without waiting for it.
// Ref: https://go.dev/ref/spec#Select_statements
func (w *Window) pollRunsSync() {
	assert.That(w != nil, "poll runs sync runs on a window")
	assert.That(w.runsSync == nil || w.runStore != nil, "only a runs store syncs")
	if w.runsSync == nil {
		return
	}
	select {
	case changed := <-w.runsSync:
		w.runsSynced(changed)
	default:
	}
}

// WaitForRunsSync waits up to limit for the sync on open to finish, and says whether it did.
// A window with no sync going returns true at once. The acceptance steps wait with it, as a
// developer waits for "Syncing runs with origin" to go.
func (w *Window) WaitForRunsSync(limit time.Duration) bool {
	assert.That(w != nil, "wait for runs sync runs on a window")
	assert.That(limit > 0, "a wait has a limit")
	if w.runsSync == nil {
		return true
	}
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case changed := <-w.runsSync:
		w.runsSynced(changed)
		return true
	case <-timer.C:
		return false
	}
}

// runsSynced takes in the sync on open's result. It has no caller to return an error to, so
// one is said in the window.
func (w *Window) runsSynced(changed error) {
	assert.That(w.runsSync != nil, "a sync was going")
	assert.That(w.runStore != nil, "runs are kept in a store")
	w.runsSync = nil
	if err := w.afterRunsChange(changed); err != nil {
		w.runsNote = messageRunsNotSynced + err.Error()
	}
}

// runsMessages say a sync on open is going, how many runs wait to be shared, and what's left
// after moving them.
func (w *Window) runsMessages() []string {
	assert.That(w != nil, "runs messages run on a window")
	assert.That(w.runsUnshared >= 0, "a count isn't negative")
	messages := make([]string, 0, 3)
	if w.runsSync != nil {
		messages = append(messages, MessageSyncingRuns)
	}
	if w.runsUnshared == 1 {
		messages = append(messages, "1 run not shared yet")
	} else if w.runsUnshared > 1 {
		messages = append(messages, fmt.Sprintf("%d runs not shared yet", w.runsUnshared))
	}
	if w.runsNote != "" {
		messages = append(messages, w.runsNote)
	}
	return messages
}

// Scenario: Share runs once the remote is back

// runsOffers is "Move runs off main" while capabilities/runs still holds runs, "Share runs"
// while runs wait to be shared, and "Change run limits"; all only when runs are kept off main.
func (w *Window) runsOffers() []string {
	assert.That(w != nil, "runs offers run on a window")
	assert.That(w.runsUnshared >= 0, "a count isn't negative")
	offers := make([]string, 0, 2)
	if w.runStore == nil {
		return offers
	}
	if len(w.runsOnMain()) > 0 {
		offers = append(offers, OfferMoveRunsOffMain)
	}
	if w.runsUnshared > 0 {
		offers = append(offers, OfferShareRuns)
	}
	return append(offers, OfferChangeRunLimits)
}

// SyncRuns shares runs kept off main, and fetches other devices' runs: the Mac app calls it
// every few minutes. A remote that can't be reached is said in the window, not returned.
func (w *Window) SyncRuns() error {
	assert.That(w != nil, "sync runs runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if !w.keepsRunsOffMain() {
		return nil
	}
	return w.syncRuns(runref.NetworkTimeout)
}

// syncRuns fetches, merges and pushes the ref, within timeout.
func (w *Window) syncRuns(timeout time.Duration) error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(timeout > 0, "a sync has a time limit")
	return w.afterRunsChange(w.runStore.Change(nil, timeout))
}

// Scenario: Move runs off main

// runsOnMain lists the day logs still in capabilities/runs: runs recorded before the project
// had a remote, which "Move runs off main" moves.
func (w *Window) runsOnMain() []string {
	assert.That(w != nil, "runs on main runs on a window")
	assert.That(filepath.IsAbs(w.path) || w.path == "", "a project's path is absolute")
	logs, err := filepath.Glob(filepath.Join(runs.Dir(w.path), "*.log"))
	if err != nil {
		return []string{}
	}
	return logs
}

// askMoveRunsOffMain asks before the runs in capabilities/runs are moved.
func (w *Window) askMoveRunsOffMain() error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(len(w.runsOnMain()) > 0, "there are runs to move")
	w.ask(messageMoveOffMain, OfferMoveRunsOffMain, w.moveRunsOffMain)
	return nil
}

// moveRunsOffMain brings capabilities/runs into the ref and shares it, then sends the old run
// files to the Trash and writes off-main.md, for the developer to commit. Givens doesn't
// commit to their branch. Needed by "Move runs off main".
func (w *Window) moveRunsOffMain() error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(w.project != nil, "runs move in a loaded project")
	adopt := func(tree string) error { return runref.Adopt(tree, w.path) }
	if err := w.afterRunsChange(w.runStore.Change(adopt, runref.NetworkTimeout)); err != nil {
		return err
	}
	moving := w.runsOnMain()
	if _, err := os.Stat(filepath.Join(runs.Dir(w.path), "files")); err == nil {
		moving = append(moving, filepath.Join(runs.Dir(w.path), "files"))
	}
	for _, path := range moving {
		if err := w.app.trash(path); err != nil {
			return err
		}
	}
	marker := filepath.Join(runs.Dir(w.path), runs.OffMainFileName)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(marker, []byte(offMainText), 0o644); err != nil {
		return err
	}
	w.runsNote = MessageRunsMoved
	return w.reload()
}

// Scenario: Change the limits

// RunLimits are how many runs, and MB of them, the ref keeps: its limits file, or the
// defaults until one is written. Needed by "Change the limits".
func (w *Window) RunLimits() (runs.Limits, error) {
	assert.That(w != nil, "run limits run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if !w.keepsRunsOffMain() {
		return runs.Limits{}, ErrRunsOnMain
	}
	return runs.ReadLimits(w.runStore.Tree())
}

// SetRunLimits writes the ref's limits and shares them, so every device trims the same way.
// Lower limits trim the ref at once. Needed by "Change the limits".
func (w *Window) SetRunLimits(limits runs.Limits) error {
	assert.That(w != nil, "set run limits runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if !w.keepsRunsOffMain() {
		return ErrRunsOnMain
	}
	if limits.Runs < 1 || limits.Runs > runs.LimitMax || limits.Megabytes < 1 ||
		limits.Megabytes > runs.LimitMax {
		return fmt.Errorf("each limit is a whole number from 1 to %d", runs.LimitMax)
	}
	write := func(tree string) error { return runs.WriteLimits(tree, limits) }
	return w.afterRunsChange(w.runStore.Change(write, runref.NetworkTimeout))
}

// Feature: Mix shell, browser and hand steps
//
// capabilities/mix-shell-browser-and-hand-steps. One setup mixes shell lines, browser lines and
// steps by hand, and a setup card's "bash run" or "js run" block runs by itself. The browser
// functions (browser_open, in_browser, app_action) are bash in internal/kit/givens.sh; they
// call `givens browser` (cmd/givens/browser.go), which drives a Chrome of Givens's own through
// internal/browser, the wrapper on chromedp. So in a Drop in, this story's app code for the
// browser lines is GIVENS_BIN; the terminal scenarios ("Browser lines in a terminal", "Run
// cards in a terminal", "The givens command isn't installed") are the kit and
// cmd/givens/run.go (runBlocks). Code elsewhere: Drop in calls into this section from its own
// functions. startSession asks for trust when runBlocksIn finds a run block, which is "Cards
// that run code ask for trust"; precheck checks a setup with run blocks only; begin adds
// givensBin. addMarkdownCards marks run blocks with runLanguage and clears a card's offers when
// it runs; it and markMarkdownCard move on with runNextCard, and ChooseOnCard's "Open" calls
// openLink. cardLines shows runState; poll and Wait poll a running block; "Stop" (choose) and
// leaveSession call stopBlock. In big_helpers.go: runBlocksIn, since Share setup's setupTexts
// reads run blocks as script lines too; scratchDir, which Drop in's argument files share; and
// RequiredCommands, which adds Chrome through chromeRequired. runNextCard and openLink came in
// with this story and hold Drop in's part too: a card that waits, the checklist once every card
// is marked, and the default browser when the Givens browser isn't open. Session's block and
// markdownFailed fields are in its type, in Drop in's section, and SetupCard's Output and
// RunBlocks and CodeBlock's Run in internal/app/model's dropin.go.

// cardOutputLinesMax bounds what one card keeps of its blocks' output.
const cardOutputLinesMax = 1000

// blockRun is the run block running now, and those of its card still to run after it.
type blockRun struct {
	card    int   // the card's index in Session.cards
	waiting []int // the card's run blocks still to run, as indices into its Blocks
	process *setup.Process
	parsed  int // output lines already moved into the card
}

// Scenario: Open a page from setup
// Scenario: Shell, browser and hand steps in one script
// Scenario: An app action that returns a promise
// Scenario: Use what the JavaScript returns
// Scenario: The JavaScript throws
// Scenario: The page has no such app action
// Scenario: JavaScript that never answers
// Scenario: Carry on in the same browser
// Scenario: Reset state in the same browser
// Scenario: A terminal run and Drop in share the browser

// givensBin is the GIVENS_BIN variable a Drop in's setup runs with. The kit's browser
// functions run Givens's own givens, so a Drop in needn't find one on the PATH.
func (s *Session) givensBin() (string, error) {
	assert.That(s != nil, "givens bin runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	command, err := s.window.app.givensCommand()
	if err != nil {
		return "", err
	}
	return "GIVENS_BIN=" + command, nil
}

// givensCommand is the givens command: Config.Command, or else the running program, which is
// the givens binary under serve.
func (a *App) givensCommand() (string, error) {
	assert.That(a != nil, "givens command runs on an app")
	assert.That(len(a.config.Command) <= PathLengthMax, "the path fits PathLengthMax")
	if a.config.Command != "" {
		return a.config.Command, nil
	}
	// Ref: https://pkg.go.dev/os#Executable
	return os.Executable()
}

// Scenario: No Chrome installed

// ChromeHint is the install hint for a setup with browser lines.
const ChromeHint = "brew install --cask google-chrome"

// chromeRequired is Chrome as a command a setup needs, and true, when a line of the setup's
// texts uses the browser: a browser line needs Chrome, as a need line needs its command.
func chromeRequired(all string) (model.RequiredCommand, bool) {
	assert.That(len(all) <= helpers.FileSizeBytesMax*helpers.HelpersMax, "texts stay bounded")
	assert.That(all == "" || strings.HasSuffix(all, "\n"), "each text ends its line")
	if !helpers.UsesBrowser(all) {
		return model.RequiredCommand{}, false
	}
	_, found := browser.Chrome()
	return model.RequiredCommand{Command: "Chrome", Hint: ChromeHint, Installed: found}, true
}

// Scenario: A step by hand in the Givens browser

// openLink opens a by_hand --url link: in the Givens browser while this project's is open,
// so the step shares the session setup signed in to, or else in the default browser.
// See capabilities/mix-shell-browser-and-hand-steps.
func (w *Window) openLink(link string) error {
	assert.That(w != nil, "open link runs on a window")
	assert.That(link != "", "a card with a link is opened")
	if profile, err := browser.Profile(w.path); err == nil && browser.Running(profile) {
		return browser.Open(profile, link)
	}
	if w.app.opener == nil {
		return errors.New("nothing is set up to open links")
	}
	return w.app.opener.OpenURL(link)
}

// Scenario: A card that runs a shell command
// Scenario: A card that runs JavaScript

// A setup card whose code block's info string is "bash run" or "js run" runs by itself when
// Drop in reaches it: a bash block with the kit loaded, a js block in the Givens browser
// through in_browser. Each runs as a script of Givens's own, so it's polled, stopped and
// timed like setup. See capabilities/mix-shell-browser-and-hand-steps.

// runLanguage says how a run block runs: "bash", "js", or "" when it doesn't run.
func runLanguage(block markdown.Block) string {
	assert.That(len(block.Code) <= markdown.TextBytesMax, "a block stays bounded")
	assert.That(block.Line >= 0, "a block has a line")
	return setup.BlockLanguage(block.Language, block.Run, block.Closed)
}

// runNextCard moves on through the markdown cards: the first one not yet marked runs its
// blocks if it has any, or else setup waits on it. Once every card is marked, the checklist
// shows.
func (s *Session) runNextCard() error {
	assert.That(s.markdownFrom >= 0, "markdown cards were added")
	assert.That(s.block == nil, "one card runs at a time")
	for index := s.markdownFrom; index < len(s.cards); index++ {
		if s.cards[index].Mark != "" {
			continue
		}
		if waiting := s.cards[index].RunBlocks; len(waiting) > 0 {
			s.resume()
			return s.startBlock(index, waiting)
		}
		if s.status != SessionWaiting {
			s.status, s.waitingSince = SessionWaiting, s.window.app.config.Clock.Now()
		}
		return nil
	}
	s.resume()
	s.status, s.checklist = SessionReady, checklistOf(s.scenario, s.row)
	return nil
}

// startBlock runs the first of a card's waiting run blocks.
func (s *Session) startBlock(card int, waiting []int) error {
	assert.That(card >= s.markdownFrom && card < len(s.cards), "the card is a markdown card")
	assert.That(len(waiting) > 0, "a block waits to run")
	script, err := s.blockScript(card, waiting[0])
	if err != nil {
		return err
	}
	// GIVENS_SCRIPT_AS makes the kit load the capability's own helpers, as for its .sh.
	stands := strings.TrimSuffix(s.scenario.SetupCards, ".md") + ".sh"
	process, err := setup.Start(setup.Request{
		Script: script, Dir: filepath.Dir(s.window.path),
		Env: append(append(s.env[:0:0], s.env...), setup.ScriptAsEnv+"="+stands),
	})
	if err != nil {
		s.fail("Setup couldn't start: " + err.Error())
		return nil
	}
	s.block = &blockRun{card: card, waiting: waiting[1:], process: process}
	return nil
}

// blockScript writes the script one run block runs as, beside the session's argument files.
func (s *Session) blockScript(card int, block int) (string, error) {
	assert.That(card >= 0 && card < len(s.cards), "the card exists")
	assert.That(block >= 0 && block < len(s.cards[card].Blocks), "the block exists")
	dir, err := s.scratchDir()
	if err != nil {
		return "", err
	}
	code := s.cards[card].Blocks[block]
	language := setup.BlockLanguage(code.Language, code.Run, code.Copyable)
	return setup.WriteBlock(dir, "card-"+strconv.Itoa(card)+"-block-"+strconv.Itoa(block),
		filepath.Join(s.window.path, "_shared", "givens.sh"), language, code.Code)
}

// runState follows a card's heading when it runs by itself: " · runs by itself" until it
// runs, then " · running", " · ran" or " · failed".
func (s *Session) runState(card int) string {
	assert.That(card >= 0 && card < len(s.cards), "the card exists")
	assert.That(s.markdownFrom >= 0, "a markdown card is asked about")
	switch {
	case len(s.cards[card].RunBlocks) == 0:
		return ""
	case s.block != nil && s.block.card == card:
		return " · running"
	case s.cards[card].Mark == MarkDone:
		return " · ran"
	case s.cards[card].Mark != "":
		return " · failed"
	}
	return " · runs by itself"
}

// Scenario: A card's run block fails

// pollBlock moves the running block's output into its card, and moves on once it ends: to
// the card's next block, to the next card, or to "Setup stopped at step N of M (exit X)".
func (s *Session) pollBlock() {
	assert.That(s.block != nil, "a block is running")
	assert.That(s.block.process != nil, "the block has a process")
	run := s.block
	result, ended := run.process.Wait(time.Millisecond)
	card := &s.cards[run.card]
	lines := run.process.Output()
	for _, line := range lines[run.parsed:] {
		if len(card.Output) < cardOutputLinesMax {
			card.Output = append(card.Output, line)
		}
	}
	run.parsed = len(lines)
	if !ended {
		return
	}
	s.block = nil
	step, steps := run.card-s.markdownFrom+1, len(s.cards)-s.markdownFrom
	switch {
	case errors.Is(result.Err, setup.ErrStopped):
		s.status = SessionStopped
	case result.Err != nil:
		s.fail("Setup failed: " + result.Err.Error())
	case result.ExitCode != 0:
		card.Mark, s.markdownFailed = "failed", true
		s.fail(fmt.Sprintf("Setup stopped at step %d of %d (exit %d)", step, steps,
			result.ExitCode))
	case len(run.waiting) > 0:
		if err := s.startBlock(run.card, run.waiting); err != nil {
			s.fail("Setup couldn't start: " + err.Error())
		}
	default:
		card.Mark = MarkDone
		if err := s.runNextCard(); err != nil {
			s.fail("Setup couldn't start: " + err.Error())
		}
	}
}

// stopBlock stops the running block, if there's one: "Stop", and leaving the Drop in.
func (s *Session) stopBlock() error {
	assert.That(s != nil, "stop block runs on a session")
	assert.That(s.block == nil || s.block.process != nil, "a running block has a process")
	if s.block == nil {
		return nil
	}
	return s.block.process.Stop()
}

// Feature: Open a project
//
// capabilities/open-a-project. Opening a folder finds its capabilities folder, reads it into a
// window and watches it; nothing is copied, so files other tools write show up. The loading
// every story reads through is in big_helpers.go: findCapabilities, reload, refresh (which
// "Files written by other tools appear" runs on), findCapability and findScenario; so is
// writable, which every change asks of a branch's window. Code elsewhere: core glue shows
// MessageNoCapabilities and branchMessages (Snapshot, snapshot.go), and offers and runs
// "Create capabilities folder" and cuts a branch's offers with readOnly (allOffers and Choose,
// offers.go). createCapabilities writes with writeNew (big_helpers.go),
// which Write stories' NewScenario uses too. Open starts Keep runs off main's sync
// (openRunStore), and OpenBranch links a branch's copy to the ref the project keeps runs in
// (runs.LinkRefCopy). Window's branch fields are in window.go's types; internal/branch
// copies a branch out of git. The terminal scenarios are openFolder in cmd/givens/main.go and
// the TUI.

// CloseWindow closes this window. Closing the last project window leaves nothing to reopen
// at the next launch. A branch's window takes its copy with it. Needed by "Reopen a recent
// project", whose last quit had nothing open.
func (w *Window) CloseWindow() error {
	assert.That(w != nil, "close runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.session != nil {
		w.session.removeArguments()
	}
	w.app.windows = slices.DeleteFunc(w.app.windows, func(other *Window) bool {
		return other == w
	})
	if w.branch != "" {
		return w.removeBranchCopy()
	}
	return nil
}

// Scenario: Open a documented project
// Scenario: Open the project folder instead
// Scenario: Open a folder with no capabilities

// Open opens a capabilities folder, or a project folder holding one, in a window. Opening a
// project that's already open brings its window to the front instead.
func (a *App) Open(path string) (*Window, error) {
	assert.That(a != nil, "open runs on an app")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	folder, found, err := findCapabilities(path)
	if err != nil {
		return nil, err
	}
	for _, window := range a.windows {
		if window.kind == WindowProject && window.path == folder {
			a.toFront(window)
			return window, window.reload()
		}
	}
	if len(a.windows) >= WindowsMax {
		return nil, fmt.Errorf("%d windows are open, the most Givens keeps", WindowsMax)
	}
	a.windowsEver++
	window := &Window{app: a, id: a.windowsEver, kind: WindowProject, path: folder, found: found}
	if err := window.reload(); err != nil {
		return nil, err
	}
	window.openRunStore()
	a.windows = append([]*Window{window}, a.windows...)
	assert.That(a.windows[0] == window, "an opened window is in front")
	return window, nil
}

// toFront moves an open window to the front.
func (a *App) toFront(window *Window) {
	assert.That(window != nil, "a window is given")
	assert.That(len(a.windows) <= WindowsMax, "windows stay bounded")
	others := make([]*Window, 0, len(a.windows))
	for _, other := range a.windows {
		if other != window {
			others = append(others, other)
		}
	}
	a.windows = append([]*Window{window}, others...)
}

// Scenario: Create a capabilities folder

// A project window with no capabilities folder says so and offers to make one.
const (
	MessageNoCapabilities = "No capabilities here yet"
	OfferCreateFolder     = "Create capabilities folder"
	// gitattributes keeps both branches' run lines in a merge.
	// Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
	gitattributes = "runs/** merge=union\n"
)

// createCapabilities makes the window's project a capabilities folder: GIVENS.md, the kit,
// the "Add a story" starter with its setup scripts, and .gitattributes for runs/. It writes
// nothing over a file that's there. See "Create a capabilities folder".
func (w *Window) createCapabilities() error {
	assert.That(w != nil, "create runs on a window")
	assert.That(!w.found, "the window has no capabilities folder yet")
	root := filepath.Join(w.path, capabilitiesFolderName)
	if _, err := os.Stat(root); err == nil {
		return errors.New("there's already a capabilities folder here")
	}
	files := map[string]string{
		"GIVENS.md":         starter.Guide(),
		"_shared/givens.sh": kit.Script,
		".gitattributes":    gitattributes,
	}
	for relative, text := range files {
		if err := writeNew(filepath.Join(root, relative), text); err != nil {
			return err
		}
	}
	// Ref: https://pkg.go.dev/io/fs#WalkDir
	err := fs.WalkDir(starter.Files(), "add-a-story", func(path string, entry fs.DirEntry,
		err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		text, err := fs.ReadFile(starter.Files(), path)
		if err != nil {
			return err
		}
		return writeNew(filepath.Join(root, filepath.FromSlash(path)), string(text))
	})
	if err != nil {
		return err
	}
	w.path, w.found = root, true
	return w.reload()
}

// Scenario: Read capabilities from another branch
// Scenario: Open a branch that doesn't exist

// Another branch's capabilities, read without checking that branch out. internal/branch copies
// the folder out of git into Givens's data folder; the window that shows the copy is read
// only, since its setup scripts belong to code that isn't checked out.

// Messages about branches.
const (
	// MessageNoBranch starts the message for a branch the repository doesn't have.
	MessageNoBranch = "No branch called "
	// messageReadOnly starts the message across the top of a branch's window.
	messageReadOnly = "Read only: "
)

// readOnlyOffers are the only offers a branch's window keeps: none of them writes a file or
// runs a script.
func readOnlyOffers() []string { return []string{OfferCopy, OfferCopySteps, OfferCancel} }

// Branches lists the branches of the git repository holding this window's project: local ones
// first, then remote ones. Needed by "Read capabilities from another branch".
func (w *Window) Branches() ([]string, error) {
	assert.That(w != nil, "branches runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has branches")
	if w.project == nil {
		return []string{}, nil
	}
	return branch.List(w.origin())
}

// OpenBranch opens this project's capabilities folder as it is on name, read only, in a new
// window in front. A branch the repository doesn't have, or one without the folder, is said
// in this window instead, and no window opens. Needed by "Read capabilities from another
// branch" and "Open a branch that doesn't exist".
func (w *Window) OpenBranch(name string) (*Window, error) {
	assert.That(w != nil, "open branch runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has branches")
	w.branchProblem = ""
	key := slug.Of(name)
	if key == "" || len(name) > branch.NameLengthMax {
		w.branchProblem = MessageNoBranch + name
		return nil, nil
	}
	copied, err := branch.Extract(w.origin(), name, w.app.branchCopy(w.origin(), name))
	switch {
	case errors.Is(err, branch.ErrNoBranch):
		w.branchProblem = MessageNoBranch + name
		return nil, nil
	case errors.Is(err, branch.ErrNoCapabilities):
		w.branchProblem = fmt.Sprintf("%s has no capabilities folder on %s", w.origin(), name)
		return nil, nil
	case err != nil:
		return nil, err
	}
	// Runs aren't kept per branch: the branch's window reads the ref this project keeps.
	if err := runs.LinkRefCopy(copied, w.origin()); err != nil {
		return nil, err
	}
	return w.app.openBranchWindow(copied, w.origin(), name)
}

// openBranchWindow shows a branch's copied capabilities folder in a read-only window in front,
// or brings its window forward if it's already open.
func (a *App) openBranchWindow(folder string, origin string, name string) (*Window, error) {
	assert.That(filepath.IsAbs(folder), "the copy is at an absolute path")
	assert.That(name != "", "a branch has a name")
	for _, window := range a.windows {
		if window.kind == WindowProject && window.path == folder {
			a.toFront(window)
			return window, window.reload()
		}
	}
	if len(a.windows) >= WindowsMax {
		return nil, fmt.Errorf("%d windows are open, the most Givens keeps", WindowsMax)
	}
	a.windowsEver++
	window := &Window{
		app: a, id: a.windowsEver, kind: WindowProject, path: folder, found: true,
		branchOrigin: origin, branch: name,
	}
	if err := window.reload(); err != nil {
		return nil, err
	}
	a.windows = append([]*Window{window}, a.windows...)
	assert.That(a.windows[0] == window, "an opened window is in front")
	return window, nil
}

// branchesDirName is the folder under Givens's data folder that holds branch copies.
const branchesDirName = "branches"

// branchCopy is where a branch's capabilities are copied to:
// <app data>/branches/<hash of origin>/<branch slug>-<hash of name>. The name's own hash keeps
// "a/b" and "a-b", which slug the same, in different folders.
func (a *App) branchCopy(origin string, name string) string {
	assert.That(filepath.IsAbs(origin), "the origin is an absolute path")
	assert.That(slug.Of(name) != "", "a branch's name has letters or digits")
	return filepath.Join(a.config.AppDataDir, branchesDirName, shortHash(origin),
		slug.Of(name)+"-"+shortHash(name))
}

// removeBranchCopy deletes a closed branch window's copy, and its origin's folder once that's
// empty. The copy is never reopened: opening the branch again copies it afresh.
func (w *Window) removeBranchCopy() error {
	assert.That(w.branch != "", "only a branch's window has a copy")
	assert.That(w.branchOrigin != "", "a branch's window knows its origin")
	copied := w.app.branchCopy(w.branchOrigin, w.branch)
	if err := os.RemoveAll(copied); err != nil {
		return err
	}
	// Remove fails on a folder that still holds another branch's copy, which is meant.
	// Ref: https://pkg.go.dev/os#Remove
	_ = os.Remove(filepath.Dir(copied))
	return nil
}

// origin is the capabilities folder this window's project lives in: for a branch's window,
// the folder it was copied from.
func (w *Window) origin() string {
	assert.That(w != nil, "origin runs on a window")
	assert.That(w.branch == "" || w.branchOrigin != "", "a branch's window knows its origin")
	if w.branch != "" {
		return w.branchOrigin
	}
	return w.path
}

// branchMessages say a branch's window is read only, and why a branch didn't open.
func (w *Window) branchMessages() []string {
	assert.That(w != nil, "branch messages runs on a window")
	messages := make([]string, 0, 2)
	if w.branch != "" {
		messages = append(messages, messageReadOnly+w.branch+" isn't checked out, so its "+
			"setup scripts don't run here")
	}
	if w.branchProblem != "" {
		messages = append(messages, w.branchProblem)
	}
	assert.That(len(messages) <= 2, "branch messages stay bounded")
	return messages
}

// readOnly cuts offers down to readOnlyOffers in a branch's window.
func (w *Window) readOnly(offers []string) []string {
	assert.That(w != nil, "read only runs on a window")
	assert.That(len(offers) <= NameLengthMax, "offers stay bounded")
	if w.branch == "" {
		return offers
	}
	return slices.DeleteFunc(offers, func(offer string) bool {
		return !slices.Contains(readOnlyOffers(), offer)
	})
}

// shortHash names a folder after text, briefly: the first 12 hex digits of its SHA-256.
// Ref: https://pkg.go.dev/crypto/sha256#Sum256
func shortHash(text string) string {
	assert.That(text != "", "something is hashed")
	sum := sha256.Sum256([]byte(text))
	short := hex.EncodeToString(sum[:])[:12]
	assert.That(len(short) == 12, "the hash is 12 digits")
	return short
}

// Feature: Read a capability
//
// capabilities/read-a-capability. Selecting a capability shows its story, its scenarios and its
// about.md; selecting a scenario shows its steps, tags and setup, the runs recorded for it, and
// "Copy steps". Code elsewhere: core glue selects by name (Select and selectNow, selection.go),
// finds the scenario on show (shownScenario, selection.go, which the Drop in page and every
// scenario page offer read too), shows both (Snapshot, snapshot.go, which calls
// shownCapability and shownHistory, and says MessageNoSetup), and offers and runs "Copy steps"
// (scenarioPageOffers and chooseOnScenarioPage, offers.go). A capability and a scenario as a
// window shows them are capabilityOf and scenarioOf (project.go), with the Capability and
// Scenario types in window.go and Window's histories field there too. A scenario's status is
// statusOf and about.md's missing images become placeholders in withImagePlaceholders, both in
// big_helpers.go. "A scenario with no setup" is noSetup and its offers (offers.go). The TUI
// shows it all (internal/tui/browse.go and story.go, r for runs), and `givens history` is
// runHistory in cmd/givens/panels.go.

// Scenario: Read a story
// Scenario: A story image that isn't on disk

// shownCapability is the capability on show, or nil when none is selected or it's gone.
func (w *Window) shownCapability() *Capability {
	assert.That(w != nil, "shown capability runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	name := w.selectedCapability
	if w.selected == "" && w.project != nil && len(w.project.Capabilities) > 0 {
		// A project opens on its first capability's story.
		name = w.project.Capabilities[0].Slug
	}
	if name == "" {
		return nil
	}
	loaded, ok := w.findCapability(name)
	if !ok {
		return nil
	}
	shown := w.capabilityOf(loaded)
	return &shown
}

// Scenario: See a scenario's runs

// RunsShownMax bounds the runs a scenario's page lists, newest first.
const RunsShownMax = 20

// RunLine is one recorded run as a scenario's page lists it, so what's been tried shows
// before dropping in. Records.swift's RunRecord.
type RunLine struct {
	Result string `json:"result"` // runs.ResultPass or runs.ResultFail
	Text   string `json:"text"`   // "Passed today at 14:05 by Sam", " · card=expired" for a row
	Note   string `json:"note"`   // what was written with the run, or ""
	// Files are the paths of the files kept with the run, like a screenshot.
	Files []string `json:"files"`
	// NoteMarkdown is the whole note when it ran to more than one line, or "".
	NoteMarkdown string `json:"noteMarkdown"`
	// Folder is the run's own folder, which a note's image links are relative to.
	Folder string `json:"folder"`
}

// History lists a scenario's recorded runs, newest first, up to RunsShownMax; an outline's
// rows all together. Kept, like statuses, until a file changes; the day is part of the key,
// so "today" turns into "yesterday" on time.
func (w *Window) History(scenario project.Scenario) []RunLine {
	assert.That(w.project != nil, "history is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	now := w.app.config.Clock.Now()
	key := now.Format(time.DateOnly) + "|" + scenario.Capability + "/" + scenario.Name
	if kept, ok := w.histories[key]; ok {
		return kept
	}
	if w.histories == nil {
		w.histories = make(map[string][]RunLine, 16)
	}
	lines := []RunLine{}
	if recorded, err := runs.Read(w.project.Root); err == nil && scenario.Slug != "" {
		lines = historyOf(w.project.Root, recorded, scenario.Key(), now)
	}
	w.histories[key] = lines
	return lines
}

// historyOf picks out one scenario's runs, by its run key or a row's key under it, newest
// first.
func historyOf(capabilities string, recorded []runs.Run, key string, now time.Time) []RunLine {
	assert.That(strings.Count(key, "/") == 1, "a scenario key is capability/slug")
	assert.That(!now.IsZero(), "now is a time")
	mine := make([]runs.Run, 0, 8)
	for _, run := range recorded {
		if run.Key == key || strings.HasPrefix(run.Key, key+"/") {
			mine = append(mine, run)
		}
	}
	// Ref: https://pkg.go.dev/sort#SliceStable
	sort.SliceStable(mine, func(a, b int) bool { return mine[a].Time.After(mine[b].Time) })
	lines := make([]RunLine, 0, min(len(mine), RunsShownMax))
	for _, run := range mine[:min(len(mine), RunsShownMax)] {
		// A note.md that can't be read leaves the run with its one-line note.
		markdown, _ := runs.ReadNote(capabilities, run)
		lines = append(lines, RunLine{
			Result: run.Result, Text: runText(run, now), Note: run.Note,
			Files: runFiles(capabilities, run), NoteMarkdown: markdown,
			Folder: runs.FolderOf(capabilities, run),
		})
	}
	return lines
}

// runText says one run as a person would: "Failed yesterday at 09:12 by Sam · card=expired".
func runText(run runs.Run, now time.Time) string {
	assert.That(!run.Time.IsZero(), "a run has a time")
	assert.That(len(run.Values) <= runs.FieldsMax, "a row stays bounded")
	text := "Failed"
	if run.Result == runs.ResultPass {
		text = "Passed"
	}
	when := run.Time.In(now.Location())
	day := "on " + when.Format(time.DateOnly)
	switch when.Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		day = "today"
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		day = "yesterday"
	}
	text += " " + day + " at " + when.Format("15:04")
	if run.Author != "" {
		text += " by " + run.Author
	}
	values := make([]string, 0, len(run.Values))
	for _, pair := range run.Values {
		values = append(values, pair.Name+"="+pair.Value)
	}
	if len(values) > 0 {
		text += " · " + strings.Join(values, ", ")
	}
	return text
}

// shownHistory is the runs of the scenario on show, or none.
func (w *Window) shownHistory() []RunLine {
	assert.That(w != nil, "history runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.project == nil || w.selectedScenario == "" {
		return []RunLine{}
	}
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok {
		return []RunLine{}
	}
	return w.History(scenario)
}

// Scenario: Copy a scenario's steps

// OfferCopySteps copies the scenario on show as Gherkin text. Needed by "Copy a scenario's
// steps" in capabilities/read-a-capability.
const OfferCopySteps = "Copy steps"

// gherkinOf is a scenario as Gherkin text, for "Copy steps": its tags, its Scenario: line, then
// its steps indented. An outline, whose steps hold <placeholders>, is a Scenario Outline.
// Ref: https://cucumber.io/docs/gherkin/reference/
func gherkinOf(scenario Scenario) string {
	assert.That(scenario.Name != "", "a scenario has a name")
	assert.That(len(scenario.Steps) <= NameLengthMax, "steps stay bounded")
	outline := false
	for _, step := range scenario.Steps {
		outline = outline || (strings.Contains(step, "<") && strings.Contains(step, ">"))
	}
	lines := make([]string, 0, len(scenario.Steps)+2)
	if len(scenario.Tags) > 0 {
		lines = append(lines, strings.Join(scenario.Tags, " "))
	}
	keyword := "Scenario: "
	if outline {
		keyword = "Scenario Outline: "
	}
	lines = append(lines, keyword+scenario.Name)
	for _, step := range scenario.Steps {
		lines = append(lines, "  "+step)
	}
	return strings.Join(lines, "\n") + "\n"
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

// Feature: Share setup between scenarios
//
// capabilities/share-setup-between-scenarios. Helpers are shell functions in _shared folders: the
// kit loads them and warns when two shared files define one (internal/kit/givens.sh), and
// internal/helpers reads their `# @helper` comments, calls, need and from lines. Helpers, the
// Shared list itself, is in big_helpers.go, since Search's findHelpers and Views' Shared view read
// it too; so is RequiredCommands, which Drop in's precheck also calls; so are the bash -n limits
// and bashError, which Write stories' shellProblems shares; and so is FingerprintOf, the way in to
// the setup fingerprint that the four "Changing …" scenarios and "Updating the kit keeps scenarios
// verified" read, since statusFor (big_helpers.go) and Drop in's RecordRun call it. Code elsewhere:
// Drop in's page shows elsewhere's message (messages); core glue offers OfferEdit and runs
// editHelper (allOffers and Choose, offers.go), selects a helper (selectNow, selection.go), and
// shows a helper and a scenario's requirements (helperLabels and requirementLabels, snapshot.go).
// Helper and its kin are types in internal/app/model's helpers.go, and Window's helpers cache is in
// window.go. The Helpers tab is internal/tui/helpers.go; `givens helpers` and `givens helper` are
// cmd/givens/panels.go.

// sharedFolder is where helpers live: _shared at the root, or in a capability's folder.
const sharedFolder = "_shared"

// OfferEdit opens the selected helper in the editor.
const OfferEdit = "Edit"

// Scenario: Calling another capability's helper

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

// Scenario: Two helpers with the same name
// Scenario: Two shared files define the same helper

// listHelpers turns one folder's documented helpers into Shared list entries.
func (w *Window) listHelpers(found []helpers.Helper, capability string,
	shared []helpers.Helper) []model.Helper {
	assert.That(w.project != nil, "helpers are listed for a loaded project")
	assert.That(len(found) <= helpers.HelpersMax, "helpers stay bounded")
	listed := make([]model.Helper, 0, len(found))
	for _, helper := range found {
		if !helper.Documented() {
			continue
		}
		entry := model.Helper{
			Name: helper.Name, Signature: helper.Signature, Description: helper.Description,
			Capability: capability, DefinedIn: helper.File, Line: helper.Line,
			Callers: w.callersOf(helper), Code: w.helperCode(helper.File, helper.Line),
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

// helperCodeLinesMax bounds how much of a helper the Shared page shows.
const helperCodeLinesMax = 200

// helperCode is a helper's function as written: from its `name() {` line, counting from 1,
// to the first line that is only a closing brace. "" when the file can't be read.
func (w *Window) helperCode(file string, line int) string {
	assert.That(!filepath.IsAbs(file), "a helper's file is relative to the capabilities folder")
	assert.That(line >= 0, "lines count from 1")
	text, err := helpers.ReadScript(filepath.Join(w.path, filepath.FromSlash(file)))
	if err != nil || line < 1 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if line > len(lines) {
		return ""
	}
	end := min(len(lines), line-1+helperCodeLinesMax)
	for index := line - 1; index < end; index++ {
		if strings.TrimRight(lines[index], " \t") == "}" {
			end = index + 1
			break
		}
	}
	return strings.Join(lines[line-1:end], "\n")
}

// Scenario: Changing a helper unverifies the scenarios that call it
// Scenario: Changing a step unverifies the scenario
// Scenario: Changing a setup unverifies the scenarios built on it
// Scenario: Changing a nested helper unverifies its callers
// Scenario: Updating the kit keeps scenarios verified

// From internal/fingerprint, which works out a scenario's setup fingerprint: one short code per
// thing a run depends on, so Givens can say what changed since the last run.
//
// The parts, in order: the scenario's steps (its tags, steps and Examples), its own setup files
// (.sh, .md and teardown), each setup it builds on through `from`, and each helper those
// scripts call, nested calls included. The Feature description, about.md and the kit aren't
// parts, so updating givens.sh unverifies nothing. FingerprintOf, the way in, is in
// big_helpers.go, since statusFor and Drop in's RecordRun both call it.

// stepsText is the scenario's block as it bears on a run: tags, steps and Examples, or only
// the given row of them.
func stepsText(scenario project.Scenario, row []runs.Pair) string {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(row) <= runs.FieldsMax, "a row stays bounded")
	var text strings.Builder
	text.WriteString(strings.Join(scenario.Tags, " ") + "\n")
	for _, step := range append(append([]project.Step{}, scenario.Background...),
		scenario.Steps...) {
		text.WriteString(step.Keyword + " " + step.Text + "\n" + step.DocString + "\n")
		for _, cells := range step.Table {
			text.WriteString(strings.Join(cells, "|") + "\n")
		}
	}
	if row != nil {
		for _, value := range row {
			text.WriteString(value.Name + "=" + value.Value + "\n")
		}
		return text.String()
	}
	for _, block := range scenario.Examples {
		text.WriteString(block.Name + "\n" + strings.Join(block.Header, "|") + "\n")
		for _, cells := range block.Rows {
			text.WriteString(strings.Join(cells, "|") + "\n")
		}
	}
	return text.String()
}

// setupText is a scenario's setup files, one after another; a missing file adds nothing.
func setupText(scenario project.Scenario) (string, error) {
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	assert.That(scenario.Setup == "" || filepath.IsAbs(scenario.Setup), "paths are absolute")
	var text strings.Builder
	for _, path := range []string{scenario.Setup, scenario.SetupCards, scenario.Teardown} {
		text.WriteString("\x00")
		if path == "" {
			continue
		}
		content, err := helpers.ReadScript(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		text.WriteString(content)
	}
	return text.String(), nil
}

// fromChain lists the scenarios a scenario's setup builds on, nearest first, each once. A
// from line naming no scenario, and a loop, end the chain: Drop in reports both before running.
func fromChain(loaded *project.Project, scenario project.Scenario) ([]project.Scenario, error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(len(loaded.Capabilities) <= spec.CapabilitiesMax, "capabilities stay bounded")
	chain := make([]project.Scenario, 0, 4)
	seen := map[string]bool{scenario.Key(): true}
	queue := []project.Scenario{scenario}
	// bound: a scenario joins the queue only once, and the project has finitely many.
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.Setup)
		if err != nil {
			return nil, err
		}
		for _, from := range helpers.Froms(text) {
			built, err := loaded.Find(from.Key)
			if err != nil || seen[built.Key()] {
				continue
			}
			seen[built.Key()] = true
			chain = append(chain, built)
			queue = append(queue, built)
		}
	}
	return chain, nil
}

// calledHelpers fingerprints every helper the scenarios' setup scripts call, each seen from
// its own capability, nested calls included, sorted by name.
func calledHelpers(loaded *project.Project, scenarios []project.Scenario) ([]runs.Pair, error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(len(scenarios) > 0, "there's at least the scenario itself")
	shared, err := helpers.InFolder(loaded.Root, "", "")
	if err != nil {
		return nil, err
	}
	bodies := make(map[string]string, 16)
	for _, scenario := range scenarios {
		if scenario.Setup == "" {
			continue
		}
		own, err := helpers.InFolder(loaded.Root, scenario.Capability, scenario.Capability)
		if err != nil {
			return nil, err
		}
		visible := helpers.Visible(shared, own)
		text, err := helpers.ReadScript(scenario.Setup)
		if err != nil {
			return nil, err
		}
		for _, name := range helpers.Called(text, visible) {
			bodies[name] = filepath.ToSlash(visible[name].File) + "\n" + visible[name].Body
		}
	}
	return sortedParts(bodies), nil
}

// sortedParts hashes each body, by name, in name order.
func sortedParts(bodies map[string]string) []runs.Pair {
	assert.That(bodies != nil, "there's a map of bodies")
	assert.That(len(bodies) <= helpers.HelpersMax, "helpers stay bounded")
	names := make([]string, 0, len(bodies))
	for name := range bodies {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]runs.Pair, 0, len(names))
	for _, name := range names {
		parts = append(parts, runs.Pair{Name: name, Value: hash(bodies[name])})
	}
	return parts
}

// FingerprintChanged names the first part of now that differs from then, a run's fingerprint,
// as a person would say it: "steps", "setup", "checkout/pay-with-saved-card setup" or a
// helper's name. It's "" when nothing changed.
func FingerprintChanged(then []runs.Pair, now []runs.Pair) string {
	assert.That(len(then) <= runs.FieldsMax, "then stays bounded")
	assert.That(len(now) <= runs.FieldsMax, "now stays bounded")
	before := make(map[string]string, len(then))
	for _, part := range then {
		before[part.Name] = part.Value
	}
	for _, part := range now {
		if was, ok := before[part.Name]; !ok || was != part.Value {
			return partWords(part.Name)
		}
	}
	if len(then) != len(now) {
		return FingerprintPartSetup
	}
	return ""
}

// partWords says a part's name as a message would.
func partWords(name string) string {
	assert.That(name != "", "a part has a name")
	assert.That(!strings.ContainsAny(name, "\t\n"), "a part's name fits a run line field")
	if key, ok := strings.CutPrefix(name, fromPrefix); ok {
		return key + " setup"
	}
	return name
}

// Scenario: See what a scenario needs installed

// setupTexts is every script a scenario's setup runs: its own, the setups it builds on through
// from lines, and the bodies of the helpers each of those calls, nested calls included.
func (w *Window) setupTexts(scenario project.Scenario) ([]string, error) {
	assert.That(w.project != nil, "setup is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	shared, err := helpers.InFolder(w.path, "", "")
	if err != nil {
		return nil, err
	}
	texts := make([]string, 0, 8)
	queue, seen := []project.Scenario{scenario}, map[string]bool{scenario.Key(): true}
	// bound: a scenario joins the queue once, and the project has finitely many.
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		texts = append(texts, runBlocksIn(current.SetupCards)...)
		if current.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.Setup)
		if err != nil {
			return nil, err
		}
		own, err := helpers.InFolder(w.path, current.Capability, current.Capability)
		if err != nil {
			return nil, err
		}
		visible := helpers.Visible(shared, own)
		texts = append(texts, text)
		for _, name := range helpers.Called(text, visible) {
			texts = append(texts, visible[name].Body)
		}
		for _, from := range helpers.Froms(text) {
			if built, err := w.project.Find(from.Key); err == nil && !seen[built.Key()] {
				seen[built.Key()] = true
				queue = append(queue, built)
			}
		}
	}
	return texts, nil
}

// Scenario: Edit a helper

// findHelper finds a documented helper in the Shared list by name, or by
// "<file relative to capabilities>:<name>" when two share it.
func (w *Window) findHelper(name string) (model.Helper, bool) {
	assert.That(w != nil, "find helper runs on a window")
	assert.That(name != "", "a helper has a name")
	for _, helper := range w.Helpers() {
		if helper.Name == name || helper.DefinedIn+":"+helper.Name == name {
			return helper, true
		}
	}
	return model.Helper{}, false
}

// selectHelper selects a helper in the Shared list, if one has that name.
func (w *Window) selectHelper(name string) bool {
	assert.That(w != nil, "select helper runs on a window")
	assert.That(name != "", "a helper has a name")
	helper, ok := w.findHelper(name)
	if !ok {
		return false
	}
	w.selected, w.selectedCapability, w.selectedScenario = name, "", ""
	w.selectedHelper = helper.DefinedIn + ":" + helper.Name
	return true
}

// editHelper opens the selected helper's file in the editor, at its `name() {` line.
func (w *Window) editHelper() error {
	assert.That(w != nil, "edit helper runs on a window")
	assert.That(w.selectedHelper != "", "a helper is selected")
	helper, ok := w.findHelper(w.selectedHelper)
	if !ok {
		return fmt.Errorf("%s: %w", w.selectedHelper, ErrNotFound)
	}
	editor, err := w.Edit(helper.DefinedIn)
	if err != nil {
		return err
	}
	editor.line = helper.Line
	return nil
}

// EditingHelper is the helper file open in the editor, where it opened, the scripts that call
// its helpers and the syntax errors marked in it. Its Path is "" when no helper is open.
// Needed by "Edit a helper" and "Save a helper with a syntax error".
func (w *Window) EditingHelper() model.HelperEdit {
	assert.That(w != nil, "editing helper runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor == nil || filepath.Base(filepath.Dir(w.editor.path)) != sharedFolder {
		return model.HelperEdit{Callers: []string{}, Problems: []model.HelperProblem{}}
	}
	relative, err := filepath.Rel(w.path, w.editor.path)
	if err != nil {
		return model.HelperEdit{Callers: []string{}, Problems: []model.HelperProblem{}}
	}
	callers := make([]string, 0, 8)
	for _, helper := range w.Helpers() {
		if helper.DefinedIn == filepath.ToSlash(relative) &&
			(w.editor.line == 0 || helper.Line == w.editor.line) {
			callers = append(callers, helper.Callers...)
		}
	}
	return model.HelperEdit{
		Path: w.editor.path, Line: w.editor.line, Callers: callers,
		Problems: append([]model.HelperProblem{}, w.editor.helperProblems...),
	}
}

// Scenario: Add a helper

// NewHelper creates <capability>/_shared/<name>.sh holding a `# @helper` comment and an empty
// function called name, and opens it in the editor. capability is a capability's name, such as
// "Checkout", or "" for the capabilities folder's own _shared, which every capability's
// scripts can call. Needed by "Add a helper".
func (w *Window) NewHelper(capability string, name string) error {
	assert.That(w != nil, "new helper runs on a window")
	assert.That(len(capability) <= NameLengthMax, "the capability's name fits NameLengthMax")
	assert.That(!strings.ContainsAny(name, "/ \n"), "a helper's name is a shell function name")
	if err := w.writable(); err != nil {
		return err
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return fmt.Errorf("%q isn't a shell function name", name)
	}
	dir := w.path
	if capability != "" {
		owner, ok := w.findCapability(capability)
		if !ok {
			return fmt.Errorf("%s: %w", capability, ErrNotFound)
		}
		dir = owner.Dir
	}
	path := filepath.Join(dir, sharedFolder, name+".sh")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text := "# shellcheck shell=bash\n\n# @helper " + name + "\n" +
		"# What " + name + " sets up, for the scripts that call it.\n" +
		name + "() {\n  :\n}\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.openHelperFile(path, name)
}

// openHelperFile opens a helper file in the editor at name's definition.
func (w *Window) openHelperFile(path string, name string) error {
	assert.That(filepath.IsAbs(path), "the helper file is absolute")
	assert.That(name != "", "a helper has a name")
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return err
	}
	editor, err := w.Edit(relative)
	if err != nil {
		return err
	}
	for index, line := range strings.Split(editor.text, "\n") {
		if strings.HasPrefix(line, name+"()") {
			editor.line = index + 1
		}
	}
	return nil
}

// Scenario: Save a helper with a syntax error

// SaveHelper saves text as the helper file open in the editor, even when bash can't parse it,
// then marks the lines `bash -n` reports. Needed by "Save a helper with a syntax error".
// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin (-n reads only)
func (w *Window) SaveHelper(text string) error {
	assert.That(w != nil, "save helper runs on a window")
	assert.That(utf8.ValidString(text), "a helper file is text")
	if err := w.writable(); err != nil {
		return err
	}
	if w.editor == nil || filepath.Base(filepath.Dir(w.editor.path)) != sharedFolder {
		return errors.New("no helper file is open in the editor")
	}
	if len(text) > helpers.FileSizeBytesMax {
		return fmt.Errorf("a helper file is at most %d bytes", helpers.FileSizeBytesMax)
	}
	if err := os.WriteFile(w.editor.path, []byte(text), 0o644); err != nil {
		return err
	}
	w.editor.text, w.editor.opened = text, text
	problems, err := syntaxProblems(w.editor.path)
	if err != nil {
		return err
	}
	w.editor.helperProblems = problems
	return w.reload()
}

// syntaxProblems runs `bash -n` on a file and reads the lines it reports.
func syntaxProblems(path string) ([]model.HelperProblem, error) {
	assert.That(filepath.IsAbs(path), "the file is absolute")
	assert.That(syntaxCheckTimeout > 0, "the check has a limit")
	ctx, cancel := context.WithTimeout(context.Background(), syntaxCheckTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "bash", "-n", path).CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return nil, err
	}
	problems := make([]model.HelperProblem, 0, 2)
	for _, match := range bashError.FindAllStringSubmatch(string(output), helperProblemsMax) {
		line, convertErr := strconv.Atoi(match[1])
		if convertErr == nil {
			problems = append(problems, model.HelperProblem{Line: line, Message: match[2]})
		}
	}
	return problems, nil
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
	assert.That(len(lines) <= model.EditorTextBytesMax, "lines stay bounded")
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
// glue shows kitMessages under the list (Snapshot, snapshot.go), and adds kitOffers and runs
// updateKit on "Update kit" (allOffers and Choose, offers.go); reload and refresh
// (big_helpers.go) call restoreKit whenever the project is read. App's kits field is in app.go and
// Window's kitProblem in window.go; internal/kit reads a kit's version; "Compare" opens the
// kit in the editor (Edit, big_helpers.go); the TUI's "u" key is internal/tui/browse.go.

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

// Feature: Write stories and scenarios
//
// capabilities/write-stories-and-scenarios. Givens writes stories, scenarios and setup as plain
// files: a new story or scenario goes straight into its .feature file, and the editor holds one
// file's whole text, checks it, and writes back only what changed. Code elsewhere: Edit and
// readEditable, which open and read a file for the editor, are in big_helpers.go, since Gherkin
// files, Share setup, Update the kit and Drop in open files in the editor too, and Tags' flagTodo
// reads with readEditable; so is writeNew, which Open a project's createCapabilities writes with
// too, and so are trash and withoutTrashed, since Keep runs off main's moveRunsOffMain sends files
// to the Trash too and reload leaves out what's waiting to go (App's pendingTrash is in app.go).
// The suggestions count steps with uniqueSteps, and shellProblems shares the bash -n limits and
// bashError with Share setup (both big_helpers.go). Core glue offers OfferNewStory,
// OfferNewScenario (with the page offers) and editorOffers and runs chooseInEditor (allOffers and
// Choose, offers.go), types into the editor (Type, input.go), calls editShown on KeyEdit (Press,
// input.go), closes the editor before selecting (closeEditor in Select, selection.go), and shows it
// (Snapshot, snapshot.go). It also offers aboutOffers and runs editAbout (allOffers and Choose),
// and the scenario page offers and runs "Create script" and "Write setup steps" (scenarioPageOffers
// and chooseOnScenarioPage, offers.go), whose labels stay there; a new setup script starts with
// KitSourceLine (big_helpers.go), as Drop in's teardown does; createSetup opens its new file with
// editFile (big_helpers.go). Editor and StorySentenceLengthMax open this section; the drafts,
// marks, suggestions, preview lines and EditorTextBytesMax are types in internal/app/model's
// edit.go; EditorSnapshot is in snapshot.go. The terminal edits in the developer's editor: `givens
// new` and `givens edit` (cmd/givens/new.go and edit.go) and the TUI's e, s and n (internal/tui)
// hand the text over through internal/termedit; `givens rename` and `givens delete` are
// cmd/givens/manage.go.

// StorySentenceLengthMax bounds each line typed into the New story dialog. Tiger Style: put a
// limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const StorySentenceLengthMax = NameLengthMax

// Editor is the window's editor, holding one file: a .feature file, an about.md, a setup
// script or a helper. Its text is unsaved until Save.
type Editor struct {
	window   *Window
	path     string
	text     string // what the editor shows, unsaved changes included
	opened   string // the file's text when it was opened or last saved
	problems []string
	line     int // the line the editor opened at, counting from 1; 0 for the top

	helperProblems []model.HelperProblem // `bash -n` reports, for a helper file
	scenario       string                // the scenario the editor opened at, or ""
	typing         int                   // the line being typed, counting from 0; -1 when none
	cursor         int                   // where typing starts a new line, counting from 0
	marks          []model.EditorMark    // lines the parser couldn't read at the last save
	previewing     bool                  // "Preview" is showing beside the text
}

// Authoring offers.
const (
	OfferNewStory = "New story"
)

// Editor actions.
const (
	OfferPreview   = "Preview"
	OfferKeepMine  = "Keep mine"
	OfferUseTheirs = "Use theirs"
	OfferCompare   = "Compare"
	// OfferCloseEditor closes the editor; with unsaved changes it asks first.
	OfferCloseEditor = "Close"
	// OfferDiscardChanges answers that question: close, losing what wasn't saved.
	OfferDiscardChanges = "Discard changes"
	// OfferDeleteFile moves a setup file open in the editor to the Trash, after asking.
	OfferDeleteFile = "Delete file"
	// SuggestionsMax bounds the steps offered while typing.
	SuggestionsMax = 20
)

// MessageChangedOnDisk is shown instead of saving over another tool's changes.
const MessageChangedOnDisk = "This file changed on disk"

// Scenario: Write a new story

// NewStory creates a capability from the New story dialog: <slug>/<slug>.feature with
// "Feature: <Name>" and the story as its description, then lists it in the sidebar.
// Needed by "Write a new story" in capabilities/write-stories-and-scenarios.
func (w *Window) NewStory(draft model.StoryDraft) error {
	assert.That(w != nil, "new story runs on a window")
	assert.That(utf8.ValidString(draft.Name), "the story's name is text")
	assert.That(len(draft.Name) <= NameLengthMax, "the name fits NameLengthMax")
	assert.That(len(draft.SoThat) <= StorySentenceLengthMax, "each story line stays bounded")
	if err := w.writable(); err != nil {
		return err
	}
	name := slug.Of(draft.Name)
	if name == "" || w.project == nil {
		return errors.New("a story needs a name with letters or digits, in an open project")
	}
	for _, part := range []string{draft.Name, draft.AsA, draft.IWant, draft.SoThat} {
		if strings.TrimSpace(part) == "" || strings.Contains(part, "\n") {
			return errors.New("each line of the story is one line of text")
		}
	}
	dir := filepath.Join(w.path, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("there's already a %s folder", name)
	}
	text := "Feature: " + draft.Name + "\n  As a " + draft.AsA + "\n  I want " + draft.IWant +
		"\n  So that " + draft.SoThat + "\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".feature"), []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.Select(draft.Name)
}

// Scenario: Add a scenario to a capability
// Scenario: A new scenario starts flagged to do

// NewScenario adds a scenario to the selected capability from the New scenario dialog: a
// "Scenario:" block tagged @todo (and @error for an error state) and scripts/<slug>.sh sourcing
// the kit, then opens the scenario in the editor. Needed by "Add a scenario to a capability",
// "A new scenario starts flagged to do" and "Reuse an existing step" in
// capabilities/write-stories-and-scenarios.
func (w *Window) NewScenario(draft model.ScenarioDraft) error {
	assert.That(w != nil, "new scenario runs on a window")
	assert.That(utf8.ValidString(draft.Name), "the scenario's name is text")
	assert.That(len(draft.Name) <= NameLengthMax, "the name fits NameLengthMax")
	if err := w.writable(); err != nil {
		return err
	}
	capability, ok := w.findCapability(w.selectedCapability)
	if w.selectedCapability == "" || !ok {
		return errors.New("select a capability to add a scenario to")
	}
	key := slug.Of(draft.Name)
	if key == "" || strings.Contains(draft.Name, "\n") {
		return errors.New("a scenario needs a one-line name with letters or digits")
	}
	if _, exists := w.findIn(capability.Slug, draft.Name); exists {
		return fmt.Errorf("%s already has a scenario called %q", capability.Name, draft.Name)
	}
	text, err := readEditable(capability.FeaturePath)
	if err != nil {
		return err
	}
	// A new scenario isn't built yet, so it starts flagged @todo.
	tags := project.TagTodo
	if draft.Error {
		tags = project.TagError + " " + tags
	}
	block := "\n  " + tags + "\n  Scenario: " + draft.Name + "\n"
	updated := strings.TrimRight(text, "\n") + "\n" + block
	if err := os.WriteFile(capability.FeaturePath, []byte(updated), 0o644); err != nil {
		return err
	}
	script := filepath.Join(capability.Dir, "scripts", key+".sh")
	if err := writeNew(script, KitSourceLine+"\n"); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	_, err = w.Edit(capability.Slug + "/" + draft.Name)
	return err
}

// blockEnd is the line after a scenario's block: past its indented lines, from the line
// holding "Scenario:" (counting from 0).
func blockEnd(lines []string, start int) int {
	assert.That(start >= 0, "a block starts on a line")
	assert.That(len(lines) <= model.EditorTextBytesMax, "lines stay bounded")
	end := start + 1
	// bound: end only grows, up to len(lines).
	for end < len(lines) && strings.HasPrefix(lines[end], "    ") {
		end++
	}
	return min(end, len(lines))
}

// Scenario: Reuse an existing step

// stepKeywords start a step line.
func stepKeywords() []string {
	keywords := []string{"Given", "When", "Then", "And", "But", "*"}
	assert.That(len(keywords) == 6, "Gherkin has six step keywords")
	assert.That(keywords[5] == "*", "the bullet is a keyword too")
	return keywords
}

// typeText types into the editor: the first text starts a new line at the cursor, as a step
// in the scenario's block; more text adds to that line.
func (e *Editor) typeText(text string) error {
	assert.That(e != nil, "type runs on an editor")
	assert.That(!strings.Contains(text, "\n"), "typing adds to one line")
	lines := strings.Split(e.text, "\n")
	if e.typing < 0 || e.typing >= len(lines) {
		at := min(max(e.cursor, 0), len(lines))
		lines = append(lines[:at], append([]string{"    " + text}, lines[at:]...)...)
		e.typing = at
	} else {
		lines[e.typing] += text
	}
	joined := strings.Join(lines, "\n")
	if len(joined) > model.EditorTextBytesMax {
		return errors.New("the file is full")
	}
	e.text = joined
	return nil
}

// typedStep splits the line being typed into its keyword and the start of its text.
func (e *Editor) typedStep() (string, string, bool) {
	assert.That(e != nil, "typed step runs on an editor")
	assert.That(e.typing < len(strings.Split(e.text, "\n")) || e.typing < 0, "typing is a line")
	if e.typing < 0 {
		return "", "", false
	}
	line := strings.TrimSpace(strings.Split(e.text, "\n")[e.typing])
	keyword, partial, _ := strings.Cut(line, " ")
	return keyword, partial, partial != ""
}

// Suggestions lists the steps offered for the step being typed, drawn from the project's
// steps: those starting with what's typed, most used first, each with "used in N". Needed by
// "Reuse an existing step".
func (e *Editor) Suggestions() []model.StepSuggestion {
	assert.That(e != nil, "suggestions runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	_, partial, ok := e.typedStep()
	if !ok {
		return []model.StepSuggestion{}
	}
	return e.suggestionsFor(partial)
}

// SuggestionsFor lists the steps offered for a step line as the Mac editor holds it: "Given a
// card" offers steps starting "a card". A line that isn't a step, or a step already written
// out word for word, offers none.
func (e *Editor) SuggestionsFor(line string) []model.StepSuggestion {
	assert.That(e != nil, "suggestions runs on an editor")
	assert.That(!strings.Contains(line, "\n"), "suggestions are for one line")
	keyword, partial, _ := strings.Cut(strings.TrimSpace(line), " ")
	if !isStepKeyword(keyword) || strings.TrimSpace(partial) == "" {
		return []model.StepSuggestion{}
	}
	offered := e.suggestionsFor(partial)
	if len(offered) == 1 && offered[0].Text == partial {
		return []model.StepSuggestion{}
	}
	return offered
}

// suggestionsFor lists the project's steps starting with partial, most used first.
func (e *Editor) suggestionsFor(partial string) []model.StepSuggestion {
	assert.That(e.window != nil, "an editor belongs to a window")
	assert.That(partial != "", "suggestions start from what's typed")
	if e.window.project == nil {
		return []model.StepSuggestion{}
	}
	uses := make(map[string]int, 32)
	for _, capability := range e.window.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, step := range uniqueSteps(scenario) {
				if strings.HasPrefix(strings.ToLower(step.Text), strings.ToLower(partial)) {
					uses[step.Text]++
				}
			}
		}
	}
	texts := make([]string, 0, len(uses))
	for text := range uses {
		texts = append(texts, text)
	}
	sort.Slice(texts, func(a, b int) bool {
		return uses[texts[a]] > uses[texts[b]] ||
			(uses[texts[a]] == uses[texts[b]] && texts[a] < texts[b])
	})
	suggestions := make([]model.StepSuggestion, 0, min(len(texts), SuggestionsMax))
	for _, text := range texts[:min(len(texts), SuggestionsMax)] {
		suggestions = append(suggestions, model.StepSuggestion{
			Text: text, Label: fmt.Sprintf("used in %d", uses[text]),
		})
	}
	return suggestions
}

// ChooseSuggestion replaces the step being typed with an offered step, word for word.
func (e *Editor) ChooseSuggestion(text string) error {
	assert.That(e != nil, "choose suggestion runs on an editor")
	assert.That(text != "", "a suggestion has text")
	assert.That(!strings.Contains(text, "\n"), "a suggestion is one step")
	keyword, _, ok := e.typedStep()
	if !ok {
		return errors.New("no step is being typed")
	}
	for _, suggestion := range e.Suggestions() {
		if suggestion.Text != text {
			continue
		}
		lines := strings.Split(e.text, "\n")
		lines[e.typing] = "    " + keyword + " " + text
		e.text, e.typing = strings.Join(lines, "\n"), -1
		return nil
	}
	return fmt.Errorf("%q isn't offered", text)
}

// Scenario: Saving changes only what was changed
// Scenario: The file changes on disk while it's being edited

// Save writes the editor's text to its file. The text is the whole file as edited, so only
// the lines that were changed differ. When the file changed on disk since it was opened, it
// shows MessageChangedOnDisk instead and writes nothing. Needed by "Saving changes only what
// was changed" and "The file changes on disk while it's being edited".
func (e *Editor) Save() error {
	assert.That(e != nil, "save runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	onDisk, err := readEditable(e.path)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if onDisk != e.opened {
		e.problems = []string{MessageChangedOnDisk}
		return nil
	}
	if err := os.WriteFile(e.path, []byte(e.text), 0o644); err != nil {
		return err
	}
	e.opened, e.problems = e.text, []string{}
	e.markParseErrors()
	return e.window.reload()
}

// conflict is true while the file on disk differs from the one the editor opened: another
// tool saved it.
func (e *Editor) conflict() bool {
	assert.That(e != nil, "conflict runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	onDisk, err := os.ReadFile(e.path)
	return err == nil && string(onDisk) != e.opened
}

// ChangedOnDisk is true while another tool has saved the file since the editor opened it, so
// Save would write nothing.
func (e *Editor) ChangedOnDisk() bool {
	assert.That(e != nil, "changed on disk runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return e.conflict()
}

// editorOffers are what the editor offers: the ways out of a conflict, or "Preview".
func (e *Editor) editorOffers() []string {
	assert.That(e != nil, "editor offers run on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	if e.conflict() {
		return []string{OfferKeepMine, OfferUseTheirs, OfferCompare}
	}
	if e.previewable() {
		return []string{OfferPreview, OfferCloseEditor}
	}
	if e.setupFile() {
		return []string{OfferDeleteFile, OfferCloseEditor}
	}
	return []string{OfferCloseEditor}
}

// chooseInEditor takes one of the editor's offers.
func (e *Editor) chooseInEditor(action string) error {
	assert.That(e != nil, "choose runs on an editor")
	assert.That(action != "", "an action has a label")
	switch action {
	case OfferKeepMine:
		if err := os.WriteFile(e.path, []byte(e.text), 0o644); err != nil {
			return err
		}
		e.opened, e.problems = e.text, []string{}
		return e.window.reload()
	case OfferUseTheirs:
		theirs, err := readEditable(e.path)
		if err != nil {
			return err
		}
		e.text, e.opened, e.problems = theirs, theirs, []string{}
		return nil
	case OfferCompare:
		theirs, err := readEditable(e.path)
		if err != nil {
			return err
		}
		e.problems = []string{MessageChangedOnDisk, "Theirs:\n" + theirs}
		return nil
	case OfferPreview:
		// Preview toggles: chosen again, it hides.
		e.previewing = !e.previewing
		return nil
	case OfferCloseEditor:
		return e.window.closeEditor(func() error { return nil })
	case OfferDeleteFile:
		return e.window.deleteFile()
	}
	return ErrNotImplemented
}

// Scenario: A line that isn't a step

// Marks lists the problems marked in the editor's text, by line: what the parser couldn't
// read at the last save.
func (e *Editor) Marks() []model.EditorMark {
	assert.That(e != nil, "marks runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return append([]model.EditorMark{}, e.marks...)
}

// markParseErrors reads a saved .feature file and marks the line the parser stops at.
func (e *Editor) markParseErrors() {
	assert.That(e != nil, "mark runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	e.marks = []model.EditorMark{}
	if !strings.HasSuffix(e.path, ".feature") {
		return
	}
	if _, err := spec.ParseFeature(e.path); err != nil {
		line, message := spec.Explain(err)
		e.marks = append(e.marks, model.EditorMark{Line: int(line), Message: capitalized(message)})
	}
}

// capitalized starts a message with a capital letter, as a mark shows it.
func capitalized(message string) string {
	assert.That(len(message) <= NameLengthMax*4, "a message stays bounded")
	if message == "" {
		return message
	}
	return strings.ToUpper(message[:1]) + message[1:]
}

// Problems are what the parser can't read in the editor's text as it is now, unsaved: the
// marks a save would leave. Only a .feature file has them. The givens command checks them
// before it saves an edit made in $EDITOR. See capabilities/work-in-the-terminal.
func (e *Editor) Problems() []model.EditorMark {
	assert.That(e != nil, "problems runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	if strings.HasSuffix(e.path, ".sh") {
		return shellProblems(e.text)
	}
	if !strings.HasSuffix(e.path, ".feature") {
		return []model.EditorMark{}
	}
	if _, err := spec.ParseFeatureText(e.text); err != nil {
		line, message := spec.Explain(err)
		return []model.EditorMark{{Line: int(line), Message: capitalized(message)}}
	}
	return []model.EditorMark{}
}

// shellProblems are the lines `bash -n` reports in a setup script or helper file's text, so a
// script that doesn't parse isn't saved from the developer's editor, as a .feature file that
// doesn't parse isn't. A check that can't run reports nothing: the file is saved as typed.
// Ref: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html (-n)
func shellProblems(text string) []model.EditorMark {
	assert.That(utf8.ValidString(text), "a script is text")
	assert.That(syntaxCheckTimeout > 0, "the check has a limit")
	ctx, cancel := context.WithTimeout(context.Background(), syntaxCheckTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", "-n")
	command.Stdin = strings.NewReader(text)
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return []model.EditorMark{}
	}
	marks := make([]model.EditorMark, 0, 2)
	for _, match := range bashError.FindAllStringSubmatch(string(output), helperProblemsMax) {
		// bash quotes the line it stopped on after its message, as a line of its own.
		line, convertErr := strconv.Atoi(match[1])
		if convertErr == nil && !strings.HasPrefix(match[2], "`") {
			marks = append(marks, model.EditorMark{Line: line, Message: capitalized(match[2])})
		}
	}
	return marks
}

// Scenario: Rename a scenario
// Scenario: Warned before a rename
// Scenario: A renamed scenario starts at Never run
// Scenario: Scenarios built on it are unverified too

// Rename renames a scenario or a capability by its name. When the rename would start runs
// over, it asks first, offering "Rename" and "Cancel"; otherwise it renames at once:
// the Scenario: or Feature: line, the setup files or folder, and every from line that names it.
// Needed by the rename scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Rename(name string, newName string) error {
	assert.That(w != nil, "rename runs on a window")
	assert.That(name != "", "rename names what to rename")
	assert.That(newName != "", "rename names the new name")
	assert.That(len(newName) <= NameLengthMax, "the new name fits NameLengthMax")
	if err := w.writable(); err != nil {
		return err
	}
	if slug.Of(newName) == "" {
		return errors.New("a name needs letters or digits")
	}
	if capability, ok := w.findCapability(name); ok {
		return w.renameCapability(capability, newName)
	}
	scenario, ok := w.findScenario(name)
	if !ok {
		return fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	rename := func() error { return w.renameScenario(scenario, newName) }
	recorded, err := runs.Read(w.path)
	if err != nil {
		return err
	}
	for _, run := range recorded {
		if run.Key == scenario.Key() || strings.HasPrefix(run.Key, scenario.Key()+"/") {
			w.ask("Renaming starts "+scenario.Name+"'s runs over", "Rename", rename)
			return nil
		}
	}
	return rename()
}

// renameScenario rewrites the scenario's Scenario: line, then moves its setup files and
// updates from lines, unless another scenario still makes the old slug.
func (w *Window) renameScenario(scenario project.Scenario, newName string) error {
	assert.That(scenario.Line > 0, "a loaded scenario has a line")
	assert.That(slug.Of(newName) != "", "the new name makes a slug")
	capability, ok := w.findCapability(scenario.Capability)
	if !ok {
		return fmt.Errorf("%s: %w", scenario.Capability, ErrNotFound)
	}
	if err := renameLine(capability.FeaturePath, scenario.Line, newName); err != nil {
		return err
	}
	newSlug := slug.Of(newName)
	if slugClash(capability, scenario) == "" && newSlug != scenario.Slug {
		if err := moveSetup(capability.Dir, scenario.Slug, newSlug); err != nil {
			return err
		}
		from := capability.Slug + "/" + scenario.Slug
		if err := w.updateFromLines(from, capability.Slug+"/"+newSlug); err != nil {
			return err
		}
	}
	if w.selectedScenario == scenario.Capability+"/"+scenario.Name {
		w.selected, w.selectedScenario = newName, scenario.Capability+"/"+newName
	}
	return w.reload()
}

// renameLine replaces the name after the keyword on one line of a .feature file, keeping the
// rest of the file byte for byte.
func renameLine(path string, line int64, newName string) error {
	assert.That(line > 0, "lines count from 1")
	assert.That(!strings.Contains(newName, "\n"), "a name is one line")
	text, err := readEditable(path)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	if line > int64(len(lines)) {
		return fmt.Errorf("%s has no line %d", filepath.Base(path), line)
	}
	keyword, _, ok := strings.Cut(lines[line-1], ":")
	if !ok {
		return fmt.Errorf("%s line %d has no keyword", filepath.Base(path), line)
	}
	lines[line-1] = keyword + ": " + newName
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// moveSetup renames scripts/<from>.sh, .md and .teardown.sh to <to>, leaving any that would
// overwrite a file.
func moveSetup(dir string, from string, to string) error {
	assert.That(from != "" && to != "", "both slugs are given")
	assert.That(from != to, "a move changes the name")
	for _, suffix := range []string{".sh", ".md", ".teardown.sh"} {
		source := filepath.Join(dir, "scripts", from+suffix)
		target := filepath.Join(dir, "scripts", to+suffix)
		if _, err := os.Stat(source); err != nil {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
	}
	return nil
}

// updateFromLines changes every `from <old>` line in the project's setup scripts to name
// replacement, keeping any row values after it. Both are run keys. It reads the scripts on
// disk, so a script renamed a moment ago is found under its new name.
func (w *Window) updateFromLines(old string, replacement string) error {
	assert.That(strings.Count(old, "/") == 1, "old is a run key")
	assert.That(strings.Count(replacement, "/") == 1, "replacement is a run key")
	pattern := regexp.MustCompile(`(?m)^(\s*from\s+)` + regexp.QuoteMeta(old) + `(\s|$)`)
	// Glob returns every setup script, in name order. Ref: https://pkg.go.dev/path/filepath#Glob
	scripts, err := filepath.Glob(filepath.Join(w.path, "*", "scripts", "*.sh"))
	if err != nil {
		return err
	}
	for _, script := range scripts {
		text, err := readEditable(script)
		if err != nil {
			return err
		}
		updated := pattern.ReplaceAllString(text, "${1}"+replacement+"${2}")
		if updated == text {
			continue
		}
		if err := os.WriteFile(script, []byte(updated), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Scenario: Rename a capability
// Scenario: Renaming a capability updates from lines

// renameCapability renames a capability's Feature: line, its folder and .feature file, and
// every from line naming one of its scenarios. Its runs stay under the old name, so its
// scenarios start at Never run.
func (w *Window) renameCapability(capability project.Capability, newName string) error {
	assert.That(capability.Slug != "", "a capability has a slug")
	assert.That(slug.Of(newName) != "", "the new name makes a slug")
	newSlug := slug.Of(newName)
	if newSlug != capability.Slug {
		if _, err := os.Stat(filepath.Join(w.path, newSlug)); err == nil {
			return fmt.Errorf("there's already a %s folder", newSlug)
		}
	}
	if err := renameFeatureLine(capability.FeaturePath, newName); err != nil {
		return err
	}
	if newSlug == capability.Slug {
		return w.reload()
	}
	newDir := filepath.Join(w.path, newSlug)
	if err := os.Rename(capability.Dir, newDir); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(newDir, capability.Slug+".feature"),
		filepath.Join(newDir, newSlug+".feature")); err != nil {
		return err
	}
	for _, scenario := range capability.Scenarios {
		if err := w.updateFromLines(capability.Slug+"/"+scenario.Slug,
			newSlug+"/"+scenario.Slug); err != nil {
			return err
		}
	}
	if w.selectedCapability == capability.Slug {
		w.selected, w.selectedCapability = newName, newSlug
	}
	return w.reload()
}

// renameFeatureLine replaces the name on a .feature file's Feature: line.
func renameFeatureLine(path string, newName string) error {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	assert.That(!strings.Contains(newName, "\n"), "a name is one line")
	text, err := readEditable(path)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if keyword, _, ok := strings.Cut(line, "Feature:"); ok && strings.TrimSpace(keyword) == "" {
			lines[index] = keyword + "Feature: " + newName
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return fmt.Errorf("%s has no Feature: line", filepath.Base(path))
}

// Scenario: Delete a scenario
// Scenario: Delete a scenario another builds on
// Scenario: Delete a capability
// Scenario: Delete a capability another builds on

// Delete buttons.
const (
	OfferDelete       = "Delete"
	OfferDeleteAnyway = "Delete anyway"
)

// Delete asks to delete a scenario or a capability by its name, offering "Delete" and
// "Cancel", or "Delete anyway" when another setup builds on it. Choosing it removes the
// scenario's block and moves its setup files, or the capability's folder, to the Trash.
// Needed by the delete scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Delete(name string) error {
	assert.That(w != nil, "delete runs on a window")
	assert.That(name != "", "delete names what to delete")
	assert.That(len(name) <= NameLengthMax, "the name fits NameLengthMax")
	if err := w.writable(); err != nil {
		return err
	}
	if capability, ok := w.findCapability(name); ok {
		builder, err := w.builtOn(capability.Slug+"/", capability.Slug)
		if err != nil {
			return err
		}
		run := func() error { return w.deleteCapability(capability) }
		w.askDelete(builder, builder+" builds on a scenario in "+capability.Name,
			"Delete "+capability.Name+"?", run)
		return nil
	}
	scenario, ok := w.findScenario(name)
	if !ok {
		return fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	builder, err := w.builtOn(scenario.Key(), "")
	if err != nil {
		return err
	}
	run := func() error { return w.deleteScenario(scenario) }
	w.askDelete(builder, builder+" builds on this scenario", "Delete "+scenario.Name+"?", run)
	return nil
}

// askDelete asks before a delete: "Delete anyway" when something builds on what's deleted.
func (w *Window) askDelete(builder string, builtMessage string, message string,
	run func() error) {
	assert.That(message != "", "a delete asks something")
	assert.That(run != nil, "a delete has something to run")
	if builder != "" {
		w.ask(builtMessage, OfferDeleteAnyway, run)
		return
	}
	w.ask(message, OfferDelete, run)
}

// builtOn names a scenario whose setup builds, through a from line, on the run key prefix,
// leaving out scenarios in the capability skip; "" when none does.
func (w *Window) builtOn(prefix string, skip string) (string, error) {
	assert.That(w.project != nil, "deletes happen in a loaded project")
	assert.That(prefix != "", "a key or prefix is given")
	for _, capability := range w.project.Capabilities {
		if capability.Slug == skip {
			continue
		}
		for _, scenario := range capability.Scenarios {
			if scenario.Setup == "" {
				continue
			}
			text, err := helpers.ReadScript(scenario.Setup)
			if err != nil {
				return "", err
			}
			for _, from := range helpers.Froms(text) {
				if from.Key == prefix || (strings.HasSuffix(prefix, "/") &&
					strings.HasPrefix(from.Key, prefix)) {
					return scenario.Name, nil
				}
			}
		}
	}
	return "", nil
}

// deleteScenario removes a scenario's block, with its tags and the blank line before it, and
// moves its setup files to the Trash.
func (w *Window) deleteScenario(scenario project.Scenario) error {
	assert.That(scenario.Line > 0, "a loaded scenario has a line")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	capability, ok := w.findCapability(scenario.Capability)
	if !ok {
		return fmt.Errorf("%s: %w", scenario.Capability, ErrNotFound)
	}
	text, err := readEditable(capability.FeaturePath)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	start := int(scenario.Line) - 1
	end := blockEnd(lines, start)
	first := start
	tag := regexp.MustCompile(`^\s*@`)
	// bound: first only falls, to 0 at most.
	for first > 0 && tag.MatchString(lines[first-1]) {
		first--
	}
	if first > 0 && strings.TrimSpace(lines[first-1]) == "" {
		first--
	}
	kept := append(append([]string{}, lines[:first]...), lines[end:]...)
	if err := os.WriteFile(capability.FeaturePath, []byte(strings.Join(kept, "\n")),
		0o644); err != nil {
		return err
	}
	for _, path := range []string{scenario.Setup, scenario.SetupCards, scenario.Teardown} {
		if path == "" {
			continue
		}
		if err := w.app.trash(path); err != nil {
			return err
		}
	}
	w.selected, w.selectedScenario = "", ""
	return w.reload()
}

// deleteCapability moves a capability's folder to the Trash.
func (w *Window) deleteCapability(capability project.Capability) error {
	assert.That(filepath.IsAbs(capability.Dir), "a capability's folder is absolute")
	assert.That(strings.HasPrefix(capability.Dir, w.path), "it's in this project")
	if err := w.app.trash(capability.Dir); err != nil {
		return err
	}
	if w.selectedCapability == capability.Slug {
		w.selected, w.selectedCapability, w.selectedScenario = "", "", ""
	}
	return w.reload()
}

// Scenario: Preview a scenario while editing
// Scenario: Preview a story with an image
// Scenario: Preview a line that isn't a step

// previewable is true for what Preview can show: a scenario's block, or about.md.
func (e *Editor) previewable() bool {
	assert.That(e != nil, "previewable runs on an editor")
	assert.That(e.path != "", "an editor holds a file")
	return e.scenario != "" || filepath.Base(e.path) == "about.md"
}

// Preview is what "Preview" shows for the editor's text, unsaved changes included: a
// scenario as it reads in the library, keywords lined up, with any line that isn't a step
// marked with the parser's message; or about.md as the library renders it. Needed by the
// preview scenarios in capabilities/write-stories-and-scenarios.
func (e *Editor) Preview() []model.PreviewLine {
	assert.That(e != nil, "preview runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	if filepath.Base(e.path) == "about.md" {
		rendered := withImagePlaceholders(e.text, filepath.Dir(e.path))
		lines := make([]model.PreviewLine, 0, 16)
		for _, line := range strings.Split(rendered, "\n") {
			lines = append(lines, model.PreviewLine{Text: line})
		}
		return lines
	}
	if e.scenario == "" {
		return []model.PreviewLine{}
	}
	return previewScenario(strings.Split(e.text, "\n"), e.scenario)
}

// previewScenario shows one scenario's block from the editor's lines: its name, then its
// steps lined up. A line that isn't a step, a tag or a comment is marked.
func previewScenario(lines []string, name string) []model.PreviewLine {
	assert.That(name != "", "a scenario has a name")
	assert.That(len(lines) <= model.EditorTextBytesMax, "lines stay bounded")
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Scenario: "+name || trimmed == "Scenario Outline: "+name {
			start = index
		}
	}
	if start < 0 {
		return []model.PreviewLine{}
	}
	preview := []model.PreviewLine{{Text: strings.TrimSpace(lines[start])}}
	steps, marked := make([]string, 0, 8), make(map[int]string, 1)
	for _, line := range lines[start+1 : blockEnd(lines, start)] {
		trimmed := strings.TrimSpace(line)
		keyword, _, _ := strings.Cut(trimmed, " ")
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case isStepKeyword(keyword):
			steps = append(steps, trimmed)
		default:
			marked[len(steps)] = trimmed
			steps = append(steps, trimmed)
		}
	}
	plain := make([]string, 0, len(steps))
	for index, step := range steps {
		if _, bad := marked[index]; !bad {
			plain = append(plain, step)
		}
	}
	aligned := linedUp(plain)
	for index, step := range steps {
		if text, bad := marked[index]; bad {
			preview = append(preview, model.PreviewLine{Text: text, Mark: MessageNotAStep})
			continue
		}
		preview = append(preview, model.PreviewLine{Text: aligned[0]})
		aligned = aligned[1:]
		assert.That(step != "", "a step has text")
	}
	return preview
}

// MessageNotAStep marks a line among a scenario's steps that the parser can't read.
const MessageNotAStep = "Expected a step, a tag, a comment or a new scenario"

// isStepKeyword is true for a word that starts a step.
func isStepKeyword(word string) bool {
	assert.That(!strings.Contains(word, " "), "a keyword is one word")
	assert.That(len(word) <= NameLengthMax, "a word stays bounded")
	for _, keyword := range stepKeywords() {
		if word == keyword {
			return true
		}
	}
	return false
}

// Scenario: Press e to edit a scenario
// Scenario: Edit the story on show

// editShown opens the editor at the scenario on show, at its block, or else at the top of the
// .feature file of the story on show. With the editor, a Drop in or search already open, the
// key is typed there instead, so it does nothing here. Needed by "Press e to edit a scenario"
// and "Press e to edit a story".
func (w *Window) editShown() error {
	assert.That(w != nil, "edit shown runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor != nil || w.session != nil || w.search != nil || w.prompt != nil {
		return nil
	}
	if w.selectedScenario != "" {
		_, err := w.Edit(w.selectedScenario)
		return err
	}
	capability, ok := w.storyCapability()
	if !ok {
		return nil
	}
	_, err := w.Edit(filepath.Join(capability.Slug, filepath.Base(capability.FeaturePath)))
	return err
}

// Editor is the editor open in the window, or nil when nothing is being edited.
func (w *Window) Editor() *Editor {
	assert.That(w != nil, "editor runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.editor
}

// Editing is the path of the file open in the window's editor, or "".
func (w *Window) Editing() string {
	assert.That(w != nil, "editing runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor == nil {
		return ""
	}
	return w.editor.path
}

// Path is the absolute path of the file the editor holds.
func (e *Editor) Path() string {
	assert.That(e != nil, "path runs on an editor")
	assert.That(e.path == "" || filepath.IsAbs(e.path), "an editor's path is absolute")
	return e.path
}

// Scenario is the scenario the editor opened at, its block, or "" for a whole file.
func (e *Editor) Scenario() string {
	assert.That(e != nil, "scenario runs on an editor")
	assert.That(e.scenario == "" || e.line > 0, "a scenario's block has a line")
	return e.scenario
}

// Line is the line the editor opened at, counting from 1, or 0 for the top of the file.
func (e *Editor) Line() int {
	assert.That(e != nil, "line runs on an editor")
	assert.That(e.line >= 0, "a line isn't negative")
	return e.line
}

// Text is the editor's text, unsaved changes included.
func (e *Editor) Text() string {
	assert.That(e != nil, "text runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	return e.text
}

// SetText replaces the editor's text without saving, as typing would.
func (e *Editor) SetText(text string) error {
	assert.That(e != nil, "set text runs on an editor")
	assert.That(utf8.ValidString(text), "the editor holds text")
	assert.That(len(text) <= model.EditorTextBytesMax, "the text fits EditorTextBytesMax")
	e.text = text
	return nil
}

// dirty is true while the editor holds changes that aren't saved.
func (e *Editor) dirty() bool {
	assert.That(e != nil, "dirty runs on an editor")
	assert.That(len(e.text) <= model.EditorTextBytesMax, "the text stays bounded")
	return e.text != e.opened
}

// closeEditor closes the editor and then runs next, asking first when changes aren't saved.
func (w *Window) closeEditor(next func() error) error {
	assert.That(w.editor != nil, "there's an editor to close")
	assert.That(next != nil, "something runs after closing")
	if !w.editor.dirty() {
		w.editor = nil
		return next()
	}
	question := "Close " + filepath.Base(w.editor.path) + "? Your changes aren't saved"
	w.ask(question, OfferDiscardChanges, func() error {
		w.editor = nil
		return next()
	})
	return nil
}

// CloseEditor closes the editor without asking, whatever it holds unsaved: the givens
// command's own editing ends in the developer's editor, which saved or kept the text.
func (w *Window) CloseEditor() {
	assert.That(w != nil, "close editor runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.editor = nil
}

// snapshot is the editor as the window shows it.
func (e *Editor) snapshot() *EditorSnapshot {
	assert.That(e != nil, "snapshot runs on an editor")
	assert.That(e.problems != nil, "problems is a list")
	problems := append([]string{}, e.problems...)
	if e.conflict() && !slices.Contains(problems, MessageChangedOnDisk) {
		problems = append(problems, MessageChangedOnDisk)
	}
	for _, problem := range e.helperProblems {
		problems = append(problems, fmt.Sprintf("line %d: %s", problem.Line, problem.Message))
	}
	for _, mark := range e.marks {
		problems = append(problems, fmt.Sprintf("line %d: %s", mark.Line, mark.Message))
	}
	preview := make([]string, 0, 16)
	if e.previewing {
		for _, line := range e.Preview() {
			preview = append(preview, strings.TrimSpace(line.Text+" "+line.Mark))
		}
	}
	return &EditorSnapshot{
		Path: e.path, Text: e.text, Problems: problems, Preview: preview, Dirty: e.dirty(),
		Scenario: e.scenario,
	}
}

// Scenario: Edit a capability's about.md
// Scenario: Add an about.md

// A capability's about.md, edited from its story: "Edit about.md", or "Add about.md" when
// there's none yet.

// About.md actions, offered with the story of the capability on show.
const (
	OfferEditAbout = "Edit about.md"
	OfferAddAbout  = "Add about.md"
)

// storyCapability is the capability whose story is on show: the selected one, or the first
// when nothing is selected, as shownCapability picks it.
func (w *Window) storyCapability() (project.Capability, bool) {
	assert.That(w != nil, "story capability runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.project == nil {
		return project.Capability{}, false
	}
	name := w.selectedCapability
	if w.selected == "" && len(w.project.Capabilities) > 0 {
		name = w.project.Capabilities[0].Slug
	}
	if name == "" {
		return project.Capability{}, false
	}
	return w.findCapability(name)
}

// aboutOffers is "Edit about.md" or "Add about.md" while a story is on show and nothing is in
// the editor. Needed by "Edit a capability's about.md" and "Add an about.md".
func (w *Window) aboutOffers() []string {
	assert.That(w != nil, "about offers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor != nil || w.session != nil {
		return []string{}
	}
	capability, ok := w.storyCapability()
	if !ok {
		return []string{}
	}
	if capability.About == "" {
		return []string{OfferAddAbout}
	}
	return []string{OfferEditAbout}
}

// editAbout opens the story's about.md in the editor, first writing one that holds only the
// capability's name as a heading when there's none.
func (w *Window) editAbout() error {
	assert.That(w != nil, "edit about runs on a window")
	assert.That(w.editor == nil, "nothing is in the editor yet")
	capability, ok := w.storyCapability()
	if !ok {
		return fmt.Errorf("no capability is on show: %w", ErrNotFound)
	}
	path := filepath.Join(filepath.Dir(capability.FeaturePath), "about.md")
	// O_EXCL leaves an about.md another tool just wrote alone.
	// Ref: https://pkg.go.dev/os#O_EXCL
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		_, err = file.WriteString("# " + capability.Name + "\n\n")
		err = errors.Join(err, file.Close())
	}
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	_, err = w.Edit(filepath.Join(capability.Slug, "about.md"))
	return err
}

// Scenario: Write setup steps

// createSetup starts the selected scenario's setup: a script sourcing the kit, or setup steps
// done by hand as an empty numbered list. It opens the new file in the editor.
func (w *Window) createSetup(script bool) error {
	assert.That(w != nil, "create setup runs on a window")
	assert.That(w.selectedScenario != "", "a scenario is selected")
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok || scenario.Slug == "" {
		return fmt.Errorf("%s: %w", w.selectedScenario, ErrNotFound)
	}
	capability, _ := w.findCapability(scenario.Capability)
	path := filepath.Join(capability.Dir, "scripts", scenario.Slug+".md")
	text := "1.\n"
	if script {
		path = filepath.Join(capability.Dir, "scripts", scenario.Slug+".sh")
		text = KitSourceLine + "\n"
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.editFile(path)
}

// Scenario: Delete a setup file while editing it
// Scenario: Delete a setup script another builds on

// setupFile is true while the editor holds one of a scenario's setup files: its script, its
// setup steps or its teardown.
func (e *Editor) setupFile() bool {
	assert.That(e != nil, "setup file runs on an editor")
	assert.That(filepath.IsAbs(e.path), "the editor's path is absolute")
	if e.window.project == nil {
		return false
	}
	for _, capability := range e.window.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if e.path == scenario.Setup || e.path == scenario.SetupCards ||
				e.path == scenario.Teardown {
				return true
			}
		}
	}
	return false
}

// deleteFile asks, then moves the file open in the editor to the Trash and closes the editor,
// unsaved changes and all. A setup script another scenario runs with a from line asks "Delete
// anyway", as deleting the scenario does. Needed by "Delete a setup file while editing it"
// and "Delete a setup script another builds on".
func (w *Window) deleteFile() error {
	assert.That(w.editor != nil, "there's a file open to delete")
	assert.That(w.editor.setupFile(), "only a setup file is offered for deleting")
	path := w.editor.path
	builder := ""
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if scenario.Setup != path {
				continue
			}
			found, err := w.builtOn(scenario.Key(), "")
			if err != nil {
				return err
			}
			builder = found
		}
	}
	w.askDelete(builder, builder+" builds on this scenario", "Delete "+filepath.Base(path)+"?",
		func() error {
			if err := w.app.trash(path); err != nil {
				return err
			}
			w.editor = nil
			return w.reload()
		})
	return nil
}
