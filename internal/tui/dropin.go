package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
)

// The Drop in page: setup's output as it runs, then its cards, then the scenario's Given
// steps over the checklist, so the whole scenario reads there. Each of the page's offers has
// a key; the keys shown are the offers on show. e edits the scenario, at its line; s edits its
// setup (the script, or the steps done by hand), as Edit script and Edit setup do once setup
// failed.

// offerKeys are the keys the Drop in page's offers answer to, in the order they're listed.
// One key can mean two offers that are never on show at once, like t for Try again and for
// Trust.
func offerKeys() [][2]string {
	keys := [][2]string{
		{"enter", app.OfferDone}, {"enter", app.OfferContinue}, {"f", app.OfferFailed},
		{"o", app.OfferOpen}, {"c", app.OfferCopy}, {"k", app.OfferKeepWaiting},
		{"s", app.OfferStop}, {"r", app.OfferRecordRun}, {"R", app.OfferResetState},
		{"t", app.OfferTryAgain}, {"s", app.OfferEditScript}, {"s", app.OfferEditSetup},
		{"E", app.OfferEditTeardown}, {"u", app.OfferRunSetupAnyway},
		{"A", app.OfferAddTeardown}, {"t", app.OfferTrust}, {"d", app.OfferDontTrust},
		{"T", app.OfferTrustThisProject}, {"S", app.OfferShareRuns},
		{"y", app.OfferLeave}, {"n", app.OfferCancel},
	}
	assert.That(len(keys) > 0, "offers have keys")
	assert.That(keys[0][0] == "enter", "Enter answers a card first")
	return keys
}

// dropInState is the Drop in page's own state.
type dropInState struct {
	scroll int    // output lines scrolled back from the end
	item   int    // the checklist item the cursor is on
	input  *input // the line being typed, or nil; see checklist.go
}

// startDropIn drops in to a scenario by its name or run key, and shows its page.
func (m *Model) startDropIn(scenario string) error {
	assert.That(m != nil, "drop in runs on a model")
	assert.That(scenario != "", "a scenario is named")
	session, err := m.window.DropIn(scenario)
	if err != nil {
		return err
	}
	m.session, m.mode, m.dropIn = session, modeDropIn, dropInState{}
	return nil
}

// StartDropIn drops in to a scenario by its run key, as `givens drop-in` does, or says why
// it couldn't.
func (m *Model) StartDropIn(key string) error {
	assert.That(m != nil, "drop in runs on a model")
	assert.That(key != "", "a scenario is named")
	return m.startDropIn(key)
}

// pollDropIn looks at setup, once a tick: the Session moves on when its script ends. Once
// the Drop in has been left, the page goes back to the project.
func (m *Model) pollDropIn() {
	assert.That(m.mode == modeDropIn, "a Drop in is on show")
	assert.That(m.session != nil, "a Drop in has a session")
	if m.window.Session() != m.session {
		m.session, m.mode = m.window.Session(), modeBrowse
		if m.session != nil {
			m.mode = modeDropIn
		}
		return
	}
	m.session.Status()
}

// dropInKey takes a key on the Drop in page.
func (m *Model) dropInKey(name string, text string) tea.Cmd {
	assert.That(m.mode == modeDropIn, "a Drop in is on show")
	assert.That(m.session != nil, "a Drop in has a session")
	offers := m.window.Offers()
	// A line being typed takes every key, q and the arrows included.
	if m.dropIn.input != nil {
		return m.inputKey(name, text)
	}
	switch {
	case name == "esc" || name == "q":
		return m.leaveDropIn()
	case name == "up" && m.dropIn.scroll < len(m.session.Output()):
		m.dropIn.scroll++
		return nil
	case name == "down" && m.dropIn.scroll > 0:
		m.dropIn.scroll--
		return nil
	}
	if handled, cmd := m.checklistKey(name, text, offers); handled {
		return cmd
	}
	// A markdown card has a Copy button per block in the app; c copies its first.
	if card := m.session.PendingCard(); name == "c" && card >= 0 &&
		!slices.Contains(offers, app.OfferCopy) {
		if err := m.session.CopyCard(card); err != nil {
			m.note = "✗ " + err.Error()
		} else {
			m.note = "Copied"
		}
		return nil
	}
	for _, pair := range offerKeys() {
		if pair[0] == name && slices.Contains(offers, pair[1]) {
			return m.chooseOffer(pair[1])
		}
	}
	switch name {
	case "e":
		return m.editPath(m.session.ScenarioKey())
	case "s":
		return m.editSessionSetup()
	}
	return nil
}

// editSessionSetup opens the Drop in's setup script, or its steps done by hand, in the
// developer's editor.
func (m *Model) editSessionSetup() tea.Cmd {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(m.mode == modeDropIn, "a Drop in is on show")
	for _, offer := range []string{app.OfferEditScript, app.OfferEditSetup} {
		if path := m.sessionFile(offer); path != "" {
			return m.editPath(path)
		}
	}
	m.note = "This scenario has no setup to edit"
	return nil
}

// chooseOffer takes one of the page's offers. An edit opens the developer's editor; the
// rest are the app's own.
func (m *Model) chooseOffer(offer string) tea.Cmd {
	assert.That(offer != "", "an offer has a label")
	assert.That(m.session != nil, "a Drop in has a session")
	if path := m.sessionFile(offer); path != "" {
		return m.editPath(path)
	}
	if err := m.window.Choose(offer); err != nil {
		m.note = "✗ " + err.Error()
	}
	m.pollDropIn()
	return nil
}

// sessionFile is the file an Edit offer opens, relative to the capabilities folder, or "".
func (m *Model) sessionFile(offer string) string {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(offer != "", "an offer has a label")
	return m.relative(m.session.FileOf(offer))
}

// relative is path relative to the capabilities folder, or "" for "".
func (m *Model) relative(path string) string {
	assert.That(m.window != nil, "the model has a window")
	assert.That(path == "" || strings.HasPrefix(path, "/"), "a path is absolute")
	return strings.TrimPrefix(strings.TrimPrefix(path, m.window.Path()), "/")
}

// leaveDropIn leaves the page, as selecting the scenario in the app does: asking first when
// marks aren't recorded.
func (m *Model) leaveDropIn() tea.Cmd {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(m.window != nil, "the model has a window")
	if question, _ := m.window.Question(); question != "" {
		if err := m.window.Choose(app.OfferCancel); err != nil {
			m.note = "✗ " + err.Error()
		}
		return nil
	}
	if err := m.window.Select(m.session.ScenarioKey()); err != nil {
		m.note = "✗ " + err.Error()
	}
	m.pollDropIn()
	return nil
}

// dropInView draws the page: the scenario, where setup stands, its output, the cards, the
// checklist, the messages, and the keys.
func (m *Model) dropInView() string {
	assert.That(m.session != nil, "a Drop in has a session")
	assert.That(m.mode == modeDropIn, "a Drop in is on show")
	snapshot := m.window.Snapshot().Session
	if snapshot == nil {
		return "Drop in closed\n"
	}
	status := snapshot.Status
	if status == app.SessionRunning {
		frames := spinner.Dot.Frames
		status = frames[m.ticks%len(frames)] + " setting up…"
	}
	page := fmt.Sprintf("Drop in · %s · %s\n\n", snapshot.Scenario, status)
	output := snapshot.Output
	end := len(output) - min(m.dropIn.scroll, len(output))
	page += lines(output[max(0, end-outputLinesShown):end])
	if len(snapshot.Cards) > 0 {
		page += "\n" + lines(snapshot.Cards)
	}
	page += m.checklistView(snapshot)
	page += "\n" + lines(m.messages())
	return page + m.keysLine(m.window.Offers()) + "\n"
}

// keysLine lists the keys that work now, as "enter Done · f Failed · q Leave".
func (m *Model) keysLine(offers []string) string {
	assert.That(m != nil, "keys run on a model")
	assert.That(len(offers) < 64, "offers stay bounded")
	parts := make([]string, 0, len(offers)+1)
	for _, pair := range offerKeys() {
		if slices.Contains(offers, pair[1]) {
			parts = append(parts, pair[0]+" "+pair[1])
		}
	}
	parts = append(parts, m.checklistKeys(offers)...)
	if m.dropIn.input == nil {
		parts = append(parts, "e edit scenario")
		// s is Stop while setup runs, and Edit script or Edit setup once it fails.
		taken := slices.ContainsFunc(offers, func(offer string) bool {
			return offer == app.OfferStop || offer == app.OfferEditScript ||
				offer == app.OfferEditSetup
		})
		if !taken {
			parts = append(parts, "s edit setup")
		}
	}
	return strings.Join(append(parts, "q Leave"), " · ")
}
