package workintheterminal

import (
	"fmt"
	"strconv"
	"strings"

	"givens/acceptance/world"
)

// keySteps are the list's keys on every tab: folding, moving, and the keys on a scenario.
func keySteps(s *state) []world.Step {
	return append(append(foldSteps(s), moveSteps(s)...), scenarioKeySteps(s)...)
}

// foldSteps are Space, l and h, and z, on a heading.
func foldSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^"Pay with saved card" and "Card declined" are tagged @payments$`,
			s.tagPayments),
		world.Def(`^they press Tab until "([^"]*)" is shown$`, s.tabTo),
		world.Def(`^"([^"]*)" is folded, with "([^"]*)" not listed$`, func(heading string,
			name string) error {
			return s.listedNot("+ "+heading, name)
		}),
		world.Def(`^Space on "([^"]*)" lists "([^"]*)", and Space again folds it away$`,
			func(heading string, name string) error {
				return s.unfoldsThenFolds(heading, name, "space", "space")
			}),
		world.Def(`^"l" on "([^"]*)" lists "([^"]*)", and "h" folds it away$`,
			func(heading string, name string) error {
				return s.unfoldsThenFolds(heading, name, "l", "h")
			}),
		world.Def(`^"z" unfolds every heading, and "z" again folds them all$`, func() error {
			if err := s.press("z"); err != nil {
				return err
			}
			if err := s.everyHeading("−"); err != nil {
				return err
			}
			if err := s.press("z"); err != nil {
				return err
			}
			return s.everyHeading("+")
		}),
	}
}

// moveSteps are l and h between levels, j and k wrapping round, and g and G.
func moveSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^they press Tab until "([^"]*)" is shown and press "l" on "([^"]*)"$`,
			s.openHeading),
		world.Def(`^pressing "l" again puts the cursor on "([^"]*)"$`, func(name string) error {
			return s.pressThenOn("l", name)
		}),
		world.Def(`^pressing "j" puts the cursor on "([^"]*)", with "([^"]*)" still listed$`,
			func(name string, still string) error {
				if err := s.pressThenOn("j", name); err != nil {
					return err
				}
				return s.sees(still)
			}),
		world.Def(`^pressing "h" puts the cursor back on "([^"]*)"$`, func(top string) error {
			return s.pressThenOn("h", top)
		}),
		world.Def(`^pressing "k" wraps the cursor round to the last row, and "j" back to `+
			`"([^"]*)"$`, func(top string) error { return s.toLastAndBack("k", "j", top) }),
		world.Def(`^pressing "G" puts the cursor on the last row, and "g" back on "([^"]*)"$`,
			func(top string) error { return s.toLastAndBack("G", "g", top) }),
	}
}

// scenarioKeySteps are e, s, r and l on a scenario listed under a heading.
func scenarioKeySteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer's editor is vim, and changes nothing$`, func() error {
			return s.FakeEditorNamed("vim")
		}),
		world.Def(`^they press Tab until "([^"]*)" is shown, open "([^"]*)" and move to `+
			`"([^"]*)"$`, s.moveUnder),
		world.Def(`^pressing "e" opens checkout\.feature in vim at the line of "Scenario: `+
			`Card declined"$`, func() error {
			line, err := s.lineOf(checkoutFeature, "Scenario: Card declined")
			if err != nil {
				return err
			}
			return s.pressThenEdits("e", "+"+strconv.Itoa(line)+" ", "checkout.feature")
		}),
		world.Def(`^pressing "s" opens card-declined\.sh in vim$`, func() error {
			return s.pressThenEdits("s", "", "card-declined.sh")
		}),
		world.Def(`^pressing "r" shows its runs: "([^"]*)"$`, func(runs string) error {
			if err := s.press("r"); err != nil {
				return err
			}
			for _, line := range strings.Split(s.model.Text(), "\n") {
				if strings.HasSuffix(line, "│ "+runs) {
					return nil
				}
			}
			return fmt.Errorf("the runs don't show %q:\n%s", runs, s.model.Text())
		}),
		world.Def(`^pressing "l" drops in to "([^"]*)"$`, func(name string) error {
			if err := s.press("l"); err != nil {
				return err
			}
			if s.model.Session() == nil {
				return fmt.Errorf("no Drop in opened:\n%s", s.model.Text())
			}
			return s.sees("Drop in · " + name)
		}),
	}
}

// openHeading shows a tab, moves to a heading on it, and unfolds it with l.
func (s *state) openHeading(tab string, heading string) error {
	if err := s.tabTo(tab); err != nil {
		return err
	}
	if !s.model.MoveTo(heading) {
		return fmt.Errorf("%s doesn't list %q:\n%s", tab, heading, s.model.Text())
	}
	return s.press("l")
}

// unfoldsThenFolds moves to a heading, unfolds it with one key and folds it with another.
func (s *state) unfoldsThenFolds(heading string, name string, unfold string,
	fold string) error {
	if !s.model.MoveTo(heading) {
		return fmt.Errorf("the TUI doesn't list %q:\n%s", heading, s.model.Text())
	}
	if err := s.press(unfold); err != nil {
		return err
	}
	if err := s.sees("− "+heading, name); err != nil {
		return err
	}
	if err := s.press(fold); err != nil {
		return err
	}
	return s.listedNot("+ "+heading, name)
}

// pressThenOn presses a key, then expects the cursor on the row holding name.
func (s *state) pressThenOn(key string, name string) error {
	if err := s.press(key); err != nil {
		return err
	}
	return s.shownLine("▸", name)
}

// toLastAndBack presses a key that should put the cursor on the last row, then one that
// should put it back on top.
func (s *state) toLastAndBack(toLast string, back string, top string) error {
	if err := s.press(toLast); err != nil {
		return err
	}
	rows := s.listRows()
	if len(rows) == 0 || !strings.HasPrefix(rows[len(rows)-1], "▸") {
		return fmt.Errorf("%q didn't put the cursor on the last row:\n%s", toLast,
			s.model.Text())
	}
	return s.pressThenOn(back, top)
}

// listRows are the list's rows as drawn: the left of each line beside the details.
func (s *state) listRows() []string {
	rows := make([]string, 0, 16)
	for _, line := range strings.Split(s.model.Text(), "\n") {
		left, _, beside := strings.Cut(line, "│")
		if left = strings.TrimRight(left, " "); beside && left != "" {
			rows = append(rows, left)
		}
	}
	return rows
}

// everyHeading expects each heading on the tab drawn with sign: − unfolded, + folded.
func (s *state) everyHeading(sign string) error {
	headings := 0
	for _, row := range s.listRows() {
		text := strings.TrimPrefix(strings.TrimPrefix(row, "▸ "), "  ")
		if !strings.HasPrefix(text, "+ ") && !strings.HasPrefix(text, "− ") {
			continue
		}
		headings++
		if !strings.HasPrefix(text, sign+" ") {
			return fmt.Errorf("%q isn't drawn with %s:\n%s", text, sign, s.model.Text())
		}
	}
	if headings == 0 {
		return fmt.Errorf("no headings are listed:\n%s", s.model.Text())
	}
	return nil
}

// moveUnder unfolds a heading on a tab, then presses j until the cursor is on name.
func (s *state) moveUnder(tab string, heading string, name string) error {
	if err := s.openHeading(tab, heading); err != nil {
		return err
	}
	// bound: the sample shop's longest list under one heading is 9 rows.
	for range 9 {
		if err := s.press("j"); err != nil {
			return err
		}
		if s.shownLine("▸", name) == nil {
			return nil
		}
	}
	return fmt.Errorf("j never reached %q under %q:\n%s", name, heading, s.model.Text())
}

// pressThenEdits presses a key, then expects the developer's editor's latest call to start
// with first and end with last.
func (s *state) pressThenEdits(key string, first string, last string) error {
	if err := s.press(key); err != nil {
		return err
	}
	calls, err := s.EditorCalls()
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return fmt.Errorf("%q didn't open the editor:\n%s", key, s.model.Text())
	}
	latest := calls[len(calls)-1]
	if !strings.HasPrefix(latest, first) || !strings.HasSuffix(latest, last) {
		return fmt.Errorf("%q opened the editor with %q:\n%s", key, latest, s.model.Text())
	}
	return nil
}
