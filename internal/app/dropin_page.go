package app

import (
	"path/filepath"

	"givens/internal/assert"
)

// The Drop in page: what it shows and offers, and what its buttons do.

// snapshot is the Drop in page as the window shows it.
func (s *Session) snapshot() *SessionSnapshot {
	assert.That(s != nil, "snapshot runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return &SessionSnapshot{
		Scenario: s.scenario.Name, Status: s.Status(), Output: s.Output(),
		Cards: make([]string, 0, 4), Checklist: s.Checklist(),
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
	switch s.Status() {
	case SessionRunning:
		if s.stillRunning() != "" {
			offers = append(offers, OfferKeepWaiting)
		}
		offers = append(offers, OfferStop)
	case SessionReady:
		offers = append(offers, OfferRecordRun)
	case SessionFailed, SessionStopped:
		if s.scenario.Setup != "" {
			offers = append(offers, OfferEditScript)
		}
		offers = append(offers, OfferTryAgain)
	}
	if len(s.hints) > 0 {
		offers = append(offers, OfferCopy)
	}
	return offers
}

// choose takes one of the Drop in page's offers.
func (s *Session) choose(action string) error {
	assert.That(s != nil, "choose runs on a session")
	assert.That(action != "", "an action has a label")
	window := s.window
	switch action {
	case OfferStop:
		if s.process == nil {
			return nil
		}
		s.scripts = nil
		return s.process.Stop()
	case OfferKeepWaiting:
		s.keepWaiting = true
		return nil
	case OfferTryAgain:
		_, err := window.DropIn(s.scenario.Capability + "/" + s.scenario.Name)
		return err
	case OfferEditScript:
		relative, err := filepath.Rel(window.path, s.scenario.Setup)
		if err != nil {
			return err
		}
		_, err = window.Edit(relative)
		return err
	case OfferRecordRun:
		return s.RecordRun(s.note)
	case OfferCopy:
		return window.app.config.Clipboard.WriteText(s.hints[0])
	}
	return ErrNotImplemented
}
