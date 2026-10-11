package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/runs"
	"givens/internal/setup"
)

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

// CheckItem is one line of a Drop in checklist.
type CheckItem struct {
	Text string `json:"text"` // the step as shown, placeholders filled in
	Kind string `json:"kind"` // "given" (done by hand), "when" or "then"
	Mark string `json:"mark"` // "", "done", "pass" or "fail"
}

// Session is one Drop in: a setup run and the checklist that follows.
type Session struct {
	window   *Window
	scenario project.Scenario
	row      []runs.Pair // the outline row's values, or nil
	examples string      // the row's Examples block's slug, or ""

	process  *setup.Process
	scripts  []string // scripts still to run after the current one: teardown, then setup
	teardown bool     // the script running now is the teardown

	skipTeardown bool   // "Run setup anyway" leaves a failing teardown out
	argumentsDir string // where GIVENS_ARG_<n> files are, or ""
	untrusted    bool   // the developer chose "Don't trust", so scripts are off

	// Cards: by_hand and ask cards the script printed, then the markdown setup's steps.
	cards          []SetupCard
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
	checklist   []CheckItem
	note        string
}

// Session is the Drop in on show, or nil.
func (w *Window) Session() *Session {
	assert.That(w != nil, "session runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.session
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
		if s.Status() != SessionRunning || s.process == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(pollInterval)
	}
	return s.Status()
}

// Checklist is the manual Givens, When steps and Then steps, in that order. It's empty until
// setup finishes.
func (s *Session) Checklist() []CheckItem {
	assert.That(s != nil, "checklist runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return append(make([]CheckItem, 0, len(s.checklist)), s.checklist...)
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

// fail ends the session with a message and no checklist.
func (s *Session) fail(message string) {
	assert.That(message != "", "a failure says what failed")
	assert.That(s.status != SessionReady, "a finished setup doesn't fail afterwards")
	s.status, s.message, s.checklist = SessionFailed, message, nil
}

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
