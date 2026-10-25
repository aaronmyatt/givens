package mixshellbrowserandhandsteps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// payInApp is the scenario whose setup is only pay-in-app.md.
const payInApp = "Pay in app"

// payInAppIntro and payInAppSteps are world.PayInAppSetup taken apart, so a step can give
// one of its steps a code block.
const payInAppIntro = "You'll need the TestFlight build of the shop app."

// fence opens and closes a code block. A const, as Go's raw strings can't hold a backtick.
const fence = "```"

// cardStepsOf are the steps about pay-in-app.md's cards and their run blocks.
func cardStepsOf(s *state) []world.Step {
	return []world.Step{
		world.Def(`^step (\d+) of pay-in-app\.md has a "(bash|js) run" block that calls ok `+
			`"([^"]*)"$`, func(step string, language string, message string) error {
			return s.runBlock(step, language, `ok "`+message+`"`)
		}),
		world.Def(`^step (\d+) of pay-in-app\.md has a "(bash|js) run" block with browser_open `+
			`"([^"]*)"$`, func(step string, language string, url string) error {
			return s.runBlock(step, language, `browser_open "`+s.ShopFor().URL(url)+`"`)
		}),
		world.Def(`^step (\d+) has a "(bash|js) run" block holding (.+)$`,
			func(step string, language string, code string) error {
				return s.runBlock(step, language, code)
			}),
		world.Def(`^step (\d+) of pay-in-app\.md has a "(bash|js) run" block that exits with `+
			`(\d+)$`, func(step string, language string, code string) error {
			return s.runBlock(step, language, "echo \"seeding shopper\"\nexit "+code)
		}),
		world.Def(`^step (\d+) of pay-in-app\.md has a "(bash|js) run" block$`,
			func(step string, language string) error {
				return s.runBlock(step, language, `ok "ran step `+step+`"`)
			}),
		world.Def(`^step (\d+) of pay-in-app\.md has a "bash run" block and step (\d+) has `+
			`none$`, func(step string, other string) error {
			if err := s.runBlock(step, "bash", `ok "seeded shopper"`); err != nil {
				return err
			}
			return s.noRunBlock(other)
		}),
		world.Def(`^step (\d+) has no run block$`, s.noRunBlock),
		world.Def(`^a developer has never run anything from the sample shop$`, func() error {
			s.Untrusted = true
			return nil
		}),
		world.Def(`^step (\d+) runs by itself and its card shows "([^"]*)"$`, s.cardShows),
		world.Def(`^step (\d+) is marked done when its block exits with 0$`, s.cardDone),
		world.Def(`^step (\d+) waits for "Done"$`, s.cardWaits),
		world.Def(`^step (\d+) runs in the Givens browser by itself$`, s.cardDone),
		world.Def(`^its card shows "([^"]*)"$`, func(text string) error {
			return s.cardShows(strconv.Itoa(len(s.cardSteps)-1), text)
		}),
		world.Def(`^step (\d+)'s card shows the block's output so far$`, func(step string) error {
			return s.cardShows(step, "seeding shopper")
		}),
		world.Def(`^they see "([^"]*)" before any card runs$`, s.trustFirst),
		world.Def(`^step (\d+) runs and prints its output$`, func(string) error {
			return s.printed("✓ seeded shopper")
		}),
		world.Def(`^step (\d+) is printed as plain text and waits for Enter, or f if it `+
			`failed$`, func(step string) error {
			if err := s.printed(step + ". Sign in as sam@example.com"); err != nil {
				return err
			}
			return s.printed("Press Enter when done, or type f if it failed")
		}),
	}
}

// runBlock gives pay-in-app.md's step a run block of language holding code, and makes it the
// scenario's only setup.
func (s *state) runBlock(step string, language string, code string) error {
	number, err := s.cardStep(step)
	if err != nil {
		return err
	}
	indented := "   " + strings.ReplaceAll(code, "\n", "\n   ")
	s.cardSteps[number-1] += "\n\n   " + fence + language + " run\n" + indented + "\n   " +
		fence
	return s.writeCards()
}

// noRunBlock expects a step of pay-in-app.md to hold no run block.
func (s *state) noRunBlock(step string) error {
	number, err := s.cardStep(step)
	if err != nil {
		return err
	}
	if strings.Contains(s.cardSteps[number-1], " run\n") {
		return fmt.Errorf("step %d has a run block", number)
	}
	return nil
}

// cardStep checks a step number of pay-in-app.md, writing the file first if no step has.
func (s *state) cardStep(step string) (int, error) {
	if s.cardSteps == nil {
		s.cardSteps = []string{
			"Install it on your phone: [Shop beta](https://testflight.apple.com/join/abc)",
			"Sign in as `sam@example.com`",
			"Add the test card `4242 4242 4242 4242` under Wallet",
		}
		if err := s.CopySampleShop(); err != nil {
			return 0, err
		}
		if err := s.EnsureScenario("checkout", payInApp); err != nil {
			return 0, err
		}
	}
	number, err := strconv.Atoi(step)
	if err != nil || number < 1 || number > len(s.cardSteps) {
		return 0, fmt.Errorf("pay-in-app.md has no step %s", step)
	}
	return number, nil
}

// writeCards writes pay-in-app.md from its steps, with no .sh beside it.
func (s *state) writeCards() error {
	scripts := filepath.Join("capabilities", "checkout", "scripts")
	err := os.Remove(filepath.Join(s.Project, scripts, "pay-in-app.sh"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := payInAppIntro + "\n\n"
	for index, step := range s.cardSteps {
		text += strconv.Itoa(index+1) + ". " + step + "\n"
	}
	return s.WriteFile(filepath.Join(scripts, "pay-in-app.md"), text)
}

// card is one of pay-in-app.md's cards, once setup has run as far as it goes.
func (s *state) card(step string) (app.SetupCard, error) {
	session, err := s.session()
	if err != nil {
		return app.SetupCard{}, err
	}
	session.Wait(world.SetupWait)
	number, err := s.cardStep(step)
	if err != nil {
		return app.SetupCard{}, err
	}
	cards := session.Cards()
	if len(cards) != len(s.cardSteps) {
		return app.SetupCard{}, fmt.Errorf("%d cards, not %d; setup is %s: %s %q", len(cards),
			len(s.cardSteps), session.Status(), s.visible(), session.Output())
	}
	return cards[number-1], nil
}

func (s *state) cardShows(step string, text string) error {
	card, err := s.card(step)
	if err != nil {
		return err
	}
	return world.Expect(card.Output, text, "step "+step+"'s card")
}

func (s *state) cardDone(step string) error {
	card, err := s.card(step)
	if err != nil {
		return err
	}
	if card.Mark != app.MarkDone {
		return fmt.Errorf("step %s's card is marked %q: %q", step, card.Mark, card.Output)
	}
	return nil
}

func (s *state) cardWaits(step string) error {
	card, err := s.card(step)
	if err != nil {
		return err
	}
	if card.Mark != "" || !slices.Contains(card.Offers, app.OfferDone) {
		return fmt.Errorf("step %s's card is marked %q and offers %q", step, card.Mark,
			card.Offers)
	}
	if status := s.Session.Status(); status != app.SessionWaiting {
		return fmt.Errorf("setup is %s, not waiting", status)
	}
	return nil
}

// trustFirst expects the trust question, and no card to have run yet.
func (s *state) trustFirst(text string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), text, "the window"); err != nil {
		return err
	}
	for _, card := range s.Session.Cards() {
		if card.Mark != "" || len(card.Output) > 0 {
			return fmt.Errorf("a card ran before trust: %+v", card)
		}
	}
	return nil
}

// printed expects text in the last command's output.
func (s *state) printed(text string) error {
	if !strings.Contains(s.Output, text) {
		return fmt.Errorf("the output doesn't hold %q:\n%s", text, s.Output)
	}
	return nil
}

// visible is what the front window shows, for a failing step's message.
func (s *state) visible() []string {
	window, err := s.Front()
	if err != nil {
		return []string{err.Error()}
	}
	return window.Visible()
}
