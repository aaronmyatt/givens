package workintheterminal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// outline is a Scenario Outline added to checkout.feature, with two rows and a setup.
const outline = `
  Scenario Outline: Card rejected by the bank
    Given a signed-in shopper whose card is <card>
    When they place the order
    Then they see "<message>"

    Examples: Rejected cards
      | card    | message                       |
      | expired | Your card has expired         |
      | stolen  | Your card was reported stolen |
`

func dropInSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^they move to "([^"]*)" and press Enter$`, s.dropInTUI),
		world.Def(`^setup's output scrolls by as it runs$`, func() error {
			s.settle()
			return s.sees("Drop in · Card declined", "✓ ")
		}),
		world.Def(`^once setup is done, the checklist shows "(When [^"]*)" and "(Then .*)"$`,
			func(when string, then string) error {
				if err := s.ready(); err != nil {
					return err
				}
				return s.sees(when, then)
			}),
		world.Def(`^they run "givens drop-in ([^"]*)" in a terminal$`, func(key string) error {
			if err := s.openTUI(); err != nil {
				return err
			}
			if err := s.model.StartDropIn(key); err != nil {
				return err
			}
			s.Session = s.model.Session()
			return nil
		}),
		world.Def(`^setup runs and the checklist shows$`, func() error {
			if err := s.ready(); err != nil {
				return err
			}
			return s.sees("[ ] When they place the order")
		}),
	}
}

// cardSteps are the steps about cards, asks and setup that runs long.
func cardSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the "([^"]*)" setup has a by_hand step for "([^"]*)" with the text `+
			`"([^"]*)"$`, func(name string, where string, text string) error {
			return s.SetupStartsWith(name, `by_hand "`+where+`" "`+text+`"`)
		}),
		world.Def(`^a developer drops in to "([^"]*)" in the TUI$`, s.dropInTUI),
		world.Def(`^they drop in to "([^"]*)" in the TUI$`, s.dropInTUI),
		world.Def(`^they see "([^"]*)" and "([^"]*)"$`, func(first string, second string) error {
			return s.sees(first, second)
		}),
		world.Def(`^pressing Enter marks it done, and setup carries on$`, func() error {
			if err := s.press("enter"); err != nil {
				return err
			}
			return s.ready()
		}),
		world.Def(`^the "([^"]*)" setup asks for ([A-Z_]+) with "([^"]*)"$`,
			func(name string, variable string, prompt string) error {
				if err := s.SetupStartsWith(name, `ok "code is $`+variable+`"`); err != nil {
					return err
				}
				return s.SetupStartsWith(name, `ask `+variable+` "`+prompt+`"`)
			}),
		world.Def(`^they type "([^"]*)" and press Enter$`, func(text string) error {
			return s.press(text, "enter")
		}),
		world.Def(`^setup carries on with ([A-Z_]+) set to "([^"]*)"$`,
			func(_ string, value string) error {
				if err := s.ready(); err != nil {
					return err
				}
				return s.sees("✓ code is " + value)
			}),
		world.Def(`^step 1 is shown as a card$`, func() error {
			return s.sees("Step 1", "Install it on your phone")
		}),
		world.Def(`^pressing "c" on a card copies its inline code or code block$`, s.copyCode),
		world.Def(`^pressing Enter on each card in turn shows the checklist$`, func() error {
			if err := s.press("enter", "enter"); err != nil {
				return err
			}
			return s.ready()
		}),
		world.Def(`^the "([^"]*)" setup takes longer than its limit of 1 second$`, s.slowSetup),
		world.Def(`^they can press "k" to keep waiting or "s" to stop$`, func() error {
			if err := s.sees("k Keep waiting", "s Stop"); err != nil {
				return err
			}
			if err := s.press("s"); err != nil {
				return err
			}
			if status := s.settle().Status(); status != app.SessionStopped {
				return fmt.Errorf("s left setup %s", status)
			}
			return nil
		}),
	}
}

// copyCode marks step 1 done, then copies step 2's inline code with c.
func (s *state) copyCode() error {
	if err := s.press("enter", "c"); err != nil {
		return err
	}
	copied, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if copied != "sam@example.com" {
		return fmt.Errorf("c copied %q:\n%s", copied, s.model.Text())
	}
	return nil
}

// slowSetup gives a scenario a limit of 1 second and a setup that sleeps, and moves the
// app's clock past the limit once setup starts.
func (s *state) slowSetup(name string) error {
	if err := s.SetupStartsWith(name, "sleep 20"); err != nil {
		return err
	}
	text, err := s.ReadFile("capabilities/checkout/checkout.feature")
	if err != nil {
		return err
	}
	if err := s.WriteFile("capabilities/checkout/checkout.feature",
		strings.Replace(text, "@timeout:5m", "@timeout:1s", 1)); err != nil {
		return err
	}
	s.slow = true
	return nil
}

func recordSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has dropped in to "([^"]*)" in the TUI and setup is done$`,
			func(name string) error {
				if err := s.dropInTUI(name); err != nil {
					return err
				}
				return s.ready()
			}),
		world.Def(`^they mark each step with "p" or Enter, write a note in their editor with `+
			`"n", and press "r"$`, s.markNoteRecord),
		world.Def(`^the run is recorded with the note$`, s.recordedWithNote),
		world.Def(`^they mark every step done or passed but the last, which they mark "f"$`,
			s.markAllButLastFailed),
		world.Def(`^the Drop in page says "([^"]*)"$`, func(text string) error {
			return s.sees(text)
		}),
		// The cursor stays on the last step once it's marked, so "p" re-marks that step.
		world.Def(`^marking that step "p" instead says "([^"]*)"$`, func(text string) error {
			if err := s.press("p"); err != nil {
				return err
			}
			return s.sees(text)
		}),
		world.Def(`^pressing "r" records the run and says "([^"]*)"$`, s.recordsAndSays),
		world.Def(`^they press "a" and give the path of a screenshot$`, s.attach),
		world.Def(`^they record the run$`, s.markAndRecord),
		world.Def(`^the screenshot is kept with the run$`, s.screenshotKept),
		world.Def(`^checkout\.feature has the outline "([^"]*)"$`, s.addOutline),
		world.Def(`^they press Enter on "([^"]*)"$`, func(name string) error {
			if !s.model.MoveTo(name) {
				return fmt.Errorf("the TUI doesn't list %q", name)
			}
			return s.press("enter")
		}),
		world.Def(`^they pick one of its Examples rows before setup runs$`, s.pickRow),
		world.Def(`^the "([^"]*)" setup fails with exit (\d+)$`, func(name string,
			code string) error {
			return s.SetupStartsWith(name, "exit "+code)
		}),
		world.Def(`^pressing "([a-z])" opens ([a-z0-9-]+\.sh) in their editor$`, s.editsScript),
	}
}

func (s *state) markNoteRecord() error {
	note := `printf 'Declined as expected\n' > "$1"`
	if err := s.FakeEditor(note); err != nil {
		return err
	}
	for range s.Session.Checklist() {
		if err := s.press("enter"); err != nil {
			return err
		}
	}
	return s.press("n", "r")
}

// latestRun is the newest run of Card declined.
func (s *state) latestRun() (app.Run, error) {
	recorded, err := app.ReadRuns(s.Capabilities())
	if err != nil {
		return app.Run{}, err
	}
	latest, ok := app.LatestRun(recorded, "checkout/card-declined", nil)
	if !ok {
		return app.Run{}, fmt.Errorf("no run was recorded:\n%s", s.model.Text())
	}
	return latest, nil
}

func (s *state) recordedWithNote() error {
	latest, err := s.latestRun()
	if err != nil {
		return err
	}
	if latest.Note != "Declined as expected" || latest.Result != app.RunResultPass {
		return fmt.Errorf("the run is %+v", latest)
	}
	return nil
}

func (s *state) attach() error {
	shot := filepath.Join(s.Dir, "shot.png")
	if err := os.WriteFile(shot, []byte("a screenshot"), 0o644); err != nil {
		return err
	}
	return s.press("a", shot, "enter")
}

func (s *state) markAndRecord() error {
	for range s.Session.Checklist() {
		if err := s.press("enter"); err != nil {
			return err
		}
	}
	return s.press("r")
}

// markAllButLastFailed presses Enter on every step but the last, which is done for a When
// step and passed for a Then step, then "f" on the last.
func (s *state) markAllButLastFailed() error {
	items := s.Session.Checklist()
	if len(items) == 0 {
		return errors.New("the checklist is empty")
	}
	for range items[:len(items)-1] {
		if err := s.press("enter"); err != nil {
			return err
		}
	}
	return s.press("f")
}

// recordsAndSays presses "r", then expects a run recorded and the page to say so.
func (s *state) recordsAndSays(text string) error {
	if err := s.press("r"); err != nil {
		return err
	}
	if _, err := s.latestRun(); err != nil {
		return err
	}
	return s.sees(text)
}

func (s *state) screenshotKept() error {
	latest, err := s.latestRun()
	if err != nil {
		return err
	}
	kept := filepath.Join(app.AttachedFilesFolder(s.Capabilities(), latest), "shot.png")
	if _, err := os.Stat(kept); err != nil {
		return fmt.Errorf("the screenshot isn't kept with the run: %w", err)
	}
	return nil
}

func (s *state) addOutline(name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile("capabilities/checkout/checkout.feature")
	if err != nil {
		return err
	}
	if err := s.WriteFile("capabilities/checkout/checkout.feature",
		strings.TrimRight(text, "\n")+"\n"+outline); err != nil {
		return err
	}
	return s.WriteFile("capabilities/checkout/scripts/"+strings.ToLower(
		strings.ReplaceAll(name, " ", "-"))+".sh",
		world.KitSource+"\nok \"card set to $GIVENS_EX_card\"\n")
}

// pickRow expects the rows to pick from, with no setup run yet, then drops in to the first.
func (s *state) pickRow() error {
	if err := s.sees("Pick a row", "card=expired", "card=stolen"); err != nil {
		return err
	}
	if s.model.Session() != nil {
		return errors.New("setup ran before a row was picked")
	}
	if err := s.press("enter"); err != nil {
		return err
	}
	s.Session = s.model.Session()
	if err := s.ready(); err != nil {
		return err
	}
	return s.sees("✓ card set to expired")
}

func (s *state) editsScript(key string, script string) error {
	if err := s.FakeEditor(); err != nil {
		return err
	}
	if err := s.press(key); err != nil {
		return err
	}
	calls, err := s.EditorCalls()
	if err != nil {
		return err
	}
	if len(calls) != 1 || !strings.HasSuffix(calls[0], script) {
		return fmt.Errorf("the editor was called %q:\n%s", calls, s.model.Text())
	}
	if !slices.Contains(strings.Split(s.model.Text(), "\n"), "No changes") {
		return fmt.Errorf("the TUI doesn't say what happened:\n%s", s.model.Text())
	}
	return nil
}
