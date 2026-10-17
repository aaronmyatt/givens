package app

import (
	"fmt"
	"path/filepath"
	"slices"

	"givens/internal/assert"
)

// The Drop in page: what it shows and offers, and what its buttons do.

// snapshot is the Drop in page as the window shows it.
func (s *Session) snapshot() *SessionSnapshot {
	assert.That(s != nil, "snapshot runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return &SessionSnapshot{
		Scenario: s.scenario.Name, Status: s.Status(), Output: s.Output(),
		Cards: s.cardLines(), Checklist: s.Checklist(), Parts: s.Parts(),
		Attachments: s.Attachments(),
	}
}

// ChecklistPart is one numbered part of the Drop in page: a run of checklist items of one kind.
// Records.swift's ChecklistPart.
type ChecklistPart struct {
	Kind  string      `json:"kind"` // "given" (done by hand), "when" or "then"
	Items []CheckItem `json:"items"`
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
		parts = append(parts, ChecklistPart{Kind: item.Kind, Items: []CheckItem{item}})
	}
	assert.That(len(parts) <= len(checklist), "each part holds at least one item")
	return parts
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
		if s.process == nil {
			return nil
		}
		s.scripts = nil
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
		if err := window.app.MarkTrusted(window.path); err != nil {
			return err
		}
		return s.restart(false)
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

// editFile opens a file inside the capabilities folder in the editor.
func (w *Window) editFile(path string) error {
	assert.That(w != nil, "edit file runs on a window")
	assert.That(path == "" || filepath.IsAbs(path), "the path is absolute")
	if path == "" {
		return ErrNotFound
	}
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return err
	}
	_, err = w.Edit(relative)
	return err
}

// cardLines are the cards as the Drop in page shows them: the markdown setup's intro first,
// then each card's heading, text, links and warning.
func (s *Session) cardLines() []string {
	assert.That(s != nil, "card lines run on a session")
	assert.That(len(s.cards) <= SetupCardsMax, "cards stay bounded")
	lines := make([]string, 0, 4*len(s.cards)+1)
	if s.intro != "" {
		lines = append(lines, s.intro)
	}
	for index, card := range s.cards {
		switch card.Kind {
		case CardByHand:
			lines = append(lines, "By hand, in "+card.Where)
		case CardMarkdown:
			lines = append(lines, fmt.Sprintf("Step %d", index-s.markdownFrom+1))
		}
		lines = append(append(lines, card.Text), card.Images...)
		if card.Warning != "" {
			lines = append(lines, card.Warning)
		}
	}
	return lines
}
