package workintheterminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"givens/acceptance/world"
	"givens/internal/tui"
)

// browseSteps are the list's: a capability's story, folding, and scrolling.
func browseSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^they open the "([^"]*)" story$`, func(name string) error {
			if !s.model.MoveTo(name) {
				return fmt.Errorf("the TUI doesn't list %q", name)
			}
			return s.press("enter")
		}),
		world.Def(`^they see its As a, I want and So that lines, and its about.md$`,
			func() error {
				return s.sees("As a returning shopper", "I want to pay with my saved card",
					"So that checkout takes seconds", "mock_payments --decline")
			}),
		world.Def(`^pressing Esc goes back to the list$`, func() error {
			if err := s.press("esc"); err != nil {
				return err
			}
			return s.sees("[Capabilities]", "Checkout (5)")
		}),
		world.Def(`^they press "([^"]*)"$`, func(key string) error { return s.press(key) }),
		world.Def(`^they move to "([^"]*)" and press "([^"]*)"$`, func(name string,
			key string) error {
			if !s.model.MoveTo(name) {
				return fmt.Errorf("the TUI doesn't list %q", name)
			}
			return s.press(key)
		}),
		world.Def(`^"([^"]*)" shows just above "([^"]*)"$`, s.justAbove),
		world.Def(`^a developer has the sample shop open in a terminal (\d+) lines tall$`,
			func(height int) error {
				if err := s.openTUI(); err != nil {
					return err
				}
				s.update(tea.WindowSizeMsg{Width: 120, Height: height})
				return nil
			}),
		world.Def(`^they press "z", then Down until the cursor is on the last scenario$`,
			s.unfoldAndGoToTheEnd),
		world.Def(`^"([^"]*)" is on show, with the cursor on it$`, func(name string) error {
			return s.shownLine("▸", name)
		}),
		world.Def(`^"([^"]*)" has scrolled off the top$`, func(heading string) error {
			if strings.Contains(s.model.Text(), heading) {
				return fmt.Errorf("%q is still on show:\n%s", heading, s.model.Text())
			}
			return nil
		}),
	}
}

// listedNot expects the first text drawn and the second not.
func (s *state) listedNot(shown string, hidden string) error {
	if err := s.sees(shown); err != nil {
		return err
	}
	if strings.Contains(s.model.Text(), hidden) {
		return fmt.Errorf("%q is still listed:\n%s", hidden, s.model.Text())
	}
	return nil
}

// unfoldAndGoToTheEnd unfolds every capability, then moves the cursor to the last row.
func (s *state) unfoldAndGoToTheEnd() error {
	if err := s.press("z"); err != nil {
		return err
	}
	// The sample shop has 12 rows unfolded; the cursor starts on the first. One press more
	// would wrap round to the top.
	for range 11 {
		if err := s.press("down"); err != nil {
			return err
		}
	}
	return nil
}

// justAbove expects a line holding first, then the next line holding second.
func (s *state) justAbove(first string, second string) error {
	shown := strings.Split(s.model.Text(), "\n")
	for index := range len(shown) - 1 {
		if strings.Contains(shown[index], first) && strings.Contains(shown[index+1], second) {
			return nil
		}
	}
	return fmt.Errorf("%q isn't just above %q:\n%s", first, second, s.model.Text())
}

// emptySteps are a project with no capabilities folder, opened in the TUI.
func emptySteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer's terminal is in a project with no capabilities folder$`,
			func() error {
				s.Cwd = filepath.Join(s.Dir, "new-project")
				return os.MkdirAll(s.Cwd, 0o755)
			}),
		// The built givens, with no terminal, prints the TUI's first screen through the
		// GIVENS_TEST_SCREEN seam; see cmd/givens/tui.go.
		world.Def(`^they run "givens" there$`, func() error {
			return s.Run("GIVENS_TEST_SCREEN=1 givens")
		}),
		world.Def(`^the TUI says "([^"]*)" and offers "([^"]*)"$`, func(message string,
			offer string) error {
			if s.ExitCode != 0 || !strings.Contains(s.Output, message) ||
				!strings.Contains(s.Output, offer) {
				return fmt.Errorf("givens exited %d and printed:\n%s", s.ExitCode, s.Output)
			}
			return nil
		}),
		world.Def(`^pressing "c" makes the capabilities folder, and the TUI lists "([^"]*)"$`,
			func(heading string) error {
				if err := s.StartApp(); err != nil {
					return err
				}
				window, err := s.App.Open(s.Cwd)
				if err != nil {
					return err
				}
				s.model = tui.New(window, runNow)
				if err := s.press("c"); err != nil {
					return err
				}
				guide := filepath.Join(s.Cwd, "capabilities", "GIVENS.md")
				if _, err := os.Stat(guide); err != nil {
					return err
				}
				return s.sees(heading)
			}),
	}
}

// tagPayments tags two checkout scenarios @payments, before Givens opens the shop.
func (s *state) tagPayments() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	text = strings.Replace(text, "  Scenario: Pay with saved card",
		"  @payments\n  Scenario: Pay with saved card", 1)
	text = strings.Replace(text, "@error @timeout:5m", "@error @timeout:5m @payments", 1)
	return s.WriteFile(checkoutFeature, text)
}

// tabTo presses Tab until the named tab is on show.
func (s *state) tabTo(tab string) error {
	for range len("CVTH") {
		if strings.Contains(s.model.Text(), "["+tab+"]") {
			return nil
		}
		if err := s.press("tab"); err != nil {
			return err
		}
	}
	return fmt.Errorf("Tab never shows %s:\n%s", tab, s.model.Text())
}
