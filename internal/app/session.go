package app

import (
	"time"

	"givens/internal/assert"
)

// Session states.
const (
	SessionRunning = "running" // setup is running
	SessionWaiting = "waiting" // setup is paused at a card: by_hand, ask, or a markdown step
	SessionReady   = "ready"   // setup finished; the checklist is shown
	SessionFailed  = "failed"  // setup or a card failed; no checklist
	SessionStopped = "stopped" // the developer chose "Stop"
)

// CheckItem is one line of a Drop in checklist.
type CheckItem struct {
	Text string // the step as shown, placeholders filled in: "When they place the order"
	Kind string // "given" (done by hand), "when" or "then"
	Mark string // "", "done", "pass" or "fail"
}

// Session is one Drop in: a setup run and the checklist that follows.
type Session struct {
	window   *Window
	scenario string
}

// Status is one of the Session* states.
func (s *Session) Status() string {
	assert.That(s != nil, "status runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return ""
}

// Output is the setup's output so far, one line per entry.
func (s *Session) Output() []string {
	assert.That(s != nil, "output runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return nil
}

// Wait waits up to limit for setup to finish or pause, then returns Status.
func (s *Session) Wait(limit time.Duration) string {
	assert.That(s != nil, "wait runs on a session")
	assert.That(limit > 0, "a wait has a limit")
	return ""
}

// Checklist is the manual Givens, When steps and Then steps, in that order.
func (s *Session) Checklist() []CheckItem {
	assert.That(s != nil, "checklist runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return nil
}

// Mark marks a checklist item: "done" for Givens and Whens, "pass" or "fail" for Thens.
func (s *Session) Mark(item string, mark string) error {
	assert.That(s != nil, "mark runs on a session")
	assert.That(mark == "done" || mark == "pass" || mark == "fail", "a mark is done, pass or fail")
	return ErrNotImplemented
}

// RecordRun records the run with note, which may be "".
func (s *Session) RecordRun(note string) error {
	assert.That(s != nil, "record run runs on a session")
	assert.That(len(note) <= NameLengthMax, "a note fits NameLengthMax")
	return ErrNotImplemented
}
