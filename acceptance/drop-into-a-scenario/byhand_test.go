package dropintoascenario

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

const (
	afterCard   = "✓ rest of setup ran"
	byHandWhere = "DigitalOcean console"
	byHandText  = "shopctl flags set new_checkout on"
	inviteCode  = "Paste the invite code shown on the phone"
)

func (s *state) endsWithFirstYou(text string) error {
	err := s.writeSetup("Card declined", script(`ok "seeded shopper"`, "first_you "+quote(text)))
	if err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) checklistStartsWith(text string) error {
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return err
	}
	items := session.Checklist()
	if len(items) == 0 || items[0].Text != text || items[0].Kind != "given" {
		return fmt.Errorf("the checklist doesn't start with the Given %q: %+v", text, items)
	}
	return nil
}

func (s *state) whensAfter() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	whens, thens, err := s.expectedChecks("Card declined")
	if err != nil {
		return err
	}
	items := session.Checklist()
	if len(items) < 1+len(whens) || items[1].Kind != "when" {
		return fmt.Errorf("the When steps don't follow the first Given: %+v", items)
	}
	return expectKinds(items, map[string][]string{"when": whens, "then": thens})
}

// onlyThensPassFail expects a Given or When marked "pass" or "fail" to be refused, while
// "done" and a Then's "pass" are taken.
func (s *state) onlyThensPassFail() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	items := session.Checklist()
	for _, kind := range []string{"given", "when"} {
		item, err := firstOfKind(items, kind)
		if err != nil {
			return err
		}
		if session.Mark(item.Text, "pass") == nil || session.Mark(item.Text, "fail") == nil {
			return fmt.Errorf("the %s step %q took a pass or fail mark", kind, item.Text)
		}
		if err := session.Mark(item.Text, "done"); err != nil {
			return err
		}
	}
	then, err := firstOfKind(items, "then")
	if err != nil {
		return err
	}
	return session.Mark(then.Text, "pass")
}

// byHandScript is a setup that stops at a by_hand step, then writes a line once it's done.
func byHandScript(where string, args ...string) string {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, args...)
	quoted = append(quoted, quote(where), quote(byHandText))
	return script(`ok "seeded shopper"`, "by_hand "+strings.Join(quoted, " "),
		`ok "rest of setup ran"`)
}

func (s *state) byHandSnippet(name string, where string, snippet string) error {
	s.scenario, s.snippet = name, snippet
	if snippet != byHandText {
		return fmt.Errorf("the by_hand snippet is %q, not %q", snippet, byHandText)
	}
	if err := s.writeSetup(name, byHandScript(where)); err != nil {
		return err
	}
	return s.prepare()
}

// byHandCard waits for setup to pause, and finds the card for a by_hand step done at where.
func (s *state) byHandCard(where string) (*app.Session, app.SetupCard, error) {
	session, err := s.settleTo(app.SessionWaiting)
	if err != nil {
		return nil, app.SetupCard{}, err
	}
	cards := session.Cards()
	for index, card := range cards {
		if card.Kind == app.CardByHand && card.Where == where {
			s.cardIndex = index
			return session, card, nil
		}
	}
	return nil, app.SetupCard{}, fmt.Errorf("no by_hand card for %q: %+v", where, cards)
}

func (s *state) pausedAtCard(where string) error {
	session, card, err := s.byHandCard(where)
	if err != nil {
		return err
	}
	if !strings.Contains(card.Text, s.snippet) {
		return fmt.Errorf("the %q card shows %q, not the snippet %q", where, card.Text, s.snippet)
	}
	return world.ExpectNot(session.Output(), afterCard, "the output while paused")
}

func (s *state) copySnippet() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if err := session.ChooseOnCard(s.cardIndex, "Copy"); err != nil {
		return err
	}
	copied, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if strings.TrimRight(copied, "\n") != s.snippet {
		return fmt.Errorf("Copy put %q on the clipboard, not %q", copied, s.snippet)
	}
	return nil
}

func (s *state) doneCarriesOn() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if err := session.ChooseOnCard(s.cardIndex, "Done"); err != nil {
		return err
	}
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return err
	}
	return world.Expect(session.Output(), afterCard, "the output after Done")
}

// pausedAtByHand drops in to a setup that stops at a by_hand step in the DigitalOcean console.
func (s *state) pausedAtByHand(name string) error {
	s.snippet = byHandText
	if err := s.writeSetup(name, byHandScript(byHandWhere)); err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	if err := s.DropIn(name); err != nil {
		return err
	}
	_, _, err := s.byHandCard(byHandWhere)
	return err
}

func (s *state) setupStopsWith(text string) error {
	session, err := s.settleTo(app.SessionFailed)
	if err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), text, "the window"); err != nil {
		return err
	}
	return world.ExpectNot(session.Output(), afterCard, "the output")
}

// byHandLink gives the scenario a by_hand step with --url or --qr, and a fake default browser.
func (s *state) byHandLink(name string, flag string, link string, where string) error {
	s.scenario, s.link, s.snippet = name, link, byHandText
	err := s.writeSetup(name, byHandScript(where, "--"+flag, quote(link)))
	if err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	return s.App.UseURLOpener(s.opener)
}

// linkCard is the paused setup's by_hand card that holds the step's link.
func (s *state) linkCard() (*app.Session, app.SetupCard, error) {
	session, err := s.settleTo(app.SessionWaiting)
	if err != nil {
		return nil, app.SetupCard{}, err
	}
	for index, card := range session.Cards() {
		if card.Kind == app.CardByHand && card.Link == s.link {
			s.cardIndex = index
			return session, card, nil
		}
	}
	return nil, app.SetupCard{}, fmt.Errorf("no card links to %q: %+v", s.link, session.Cards())
}

func (s *state) dropInToPause() error {
	if err := s.DropIn(s.scenario); err != nil {
		return err
	}
	_, _, err := s.linkCard()
	return err
}

func (s *state) cardOffers(action string) error {
	_, card, err := s.linkCard()
	if err != nil {
		return err
	}
	return world.Expect(card.Offers, action, "the card's actions")
}

func (s *state) opensInBrowser() error {
	session, _, err := s.linkCard()
	if err != nil {
		return err
	}
	if err := session.ChooseOnCard(s.cardIndex, "Open"); err != nil {
		return err
	}
	if !slices.Equal(s.opener.urls, []string{s.link}) {
		return fmt.Errorf("the default browser was asked to open %q, not %q", s.opener.urls,
			s.link)
	}
	return nil
}

func (s *state) showsQRCode() error {
	_, card, err := s.linkCard()
	if err != nil {
		return err
	}
	if !card.QRCode {
		return fmt.Errorf("the card doesn't show %q as a QR code: %+v", s.link, card)
	}
	return nil
}

func (s *state) linkAsText() error {
	_, card, err := s.linkCard()
	if err != nil {
		return err
	}
	return world.Expect([]string{card.Text}, s.link, "the card's text")
}

// setupAsks gives the scenario a setup that asks for a value, then saves it in shop-data.
func (s *state) setupAsks(name string, variable string, prompt string) error {
	s.scenario = name
	s.markers = append(s.markers, "answer")
	err := s.writeSetup(name, script("ask "+variable+" "+quote(prompt),
		`printf '%s' "$`+variable+`" > `+shopData+`/answer"`, `ok "rest of setup ran"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

// askCard waits for setup to pause at an ask card, and finds it.
func (s *state) askCard() (*app.Session, error) {
	session, err := s.settleTo(app.SessionWaiting)
	if err != nil {
		return nil, err
	}
	for index, card := range session.Cards() {
		if card.Kind == app.CardAsk {
			s.cardIndex = index
			return session, nil
		}
	}
	return nil, fmt.Errorf("setup paused with no ask card: %+v", session.Cards())
}

func (s *state) typeAndChoose(value string, action string) error {
	if err := s.DropIn(s.scenario); err != nil {
		return err
	}
	session, err := s.askCard()
	if err != nil {
		return err
	}
	if err := s.Window.Type(value); err != nil {
		return err
	}
	return session.ChooseOnCard(s.cardIndex, action)
}

func (s *state) restRunsWith(variable string, value string) error {
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return err
	}
	answer, _, err := s.shopFile("answer")
	if err != nil {
		return err
	}
	if answer != value {
		return fmt.Errorf("setup ran on with %s=%q, not %q", variable, answer, value)
	}
	return nil
}

func (s *state) pausedAtAsk() error {
	if err := s.setupAsks("Accept invite", "INVITE_CODE", inviteCode); err != nil {
		return err
	}
	if err := s.DropIn("Accept invite"); err != nil {
		return err
	}
	_, err := s.askCard()
	return err
}

func (s *state) continueEmpty() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	return session.ChooseOnCard(s.cardIndex, "Continue")
}

func (s *state) staysPaused() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if status := session.Status(); status != app.SessionWaiting {
		return fmt.Errorf("setup is %q, not still paused", status)
	}
	if _, answered, err := s.shopFile("answer"); err != nil || answered {
		return fmt.Errorf("setup carried on past the ask card (%v)", err)
	}
	return world.ExpectNot(session.Output(), afterCard, "the output")
}

// fromScenarioByHand gives Pay with saved card a setup with "your turn" lines, which `from`
// hides, then a by_hand step in a bank portal.
func (s *state) fromScenarioByHand() error {
	err := s.writeSetup(payWithSavedCard, script(
		`first_you 'open the shop app'`, `when_you 'tap "Pay"'`,
		`then_check 'you see "Order placed"'`, `ok "saved card added"`,
		`by_hand 'Bank portal' 'Approve the test card'`, `ok "card approved"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) fromCardShown() error {
	_, _, err := s.byHandCard("Bank portal")
	return err
}

func (s *state) yourTurnHidden() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	for _, hidden := range []string{"Your turn", "open the shop app", `tap "Pay"`, "Order placed"} {
		if err := world.ExpectNot(session.Output(), hidden, "the output"); err != nil {
			return err
		}
	}
	return world.Expect(session.Output(), "✓ saved card added", "the output")
}

// minutesPassThenDone moves the clock on while setup waits at a card, watches for "Still
// running", then chooses "Done".
func (s *state) minutesPassThenDone(minutes string) error {
	count, err := strconv.Atoi(minutes)
	if err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	s.Clock.Advance(time.Duration(count) * time.Minute)
	s.sawSlow = !staysFalse(func() bool { return includes(window.Visible(), stillRunning) })
	session, err := s.session()
	if err != nil {
		return err
	}
	if err := session.ChooseOnCard(s.cardIndex, "Done"); err != nil {
		return err
	}
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return errors.Join(errors.New("setup didn't finish after Done"), err)
	}
	return nil
}
