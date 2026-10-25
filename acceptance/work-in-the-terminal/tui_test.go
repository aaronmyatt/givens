// Package workintheterminal runs capabilities/work-in-the-terminal. The TUI's steps drive
// internal/tui's model with key messages and read what it draws, with no terminal; the
// developer's editor is acceptance/world's fake. Commands like givens edit run as the built
// givens, as a developer would type them. Steps shared with other capabilities are in
// acceptance/world.
package workintheterminal

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/tui"
)

const capability = "work-in-the-terminal"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	model   *tui.Model
	quit    bool     // the TUI quit
	outputs []string // the outputs of commands run one after another
	before  string   // a file's text before a When step
	slow    bool     // setup runs past its limit: dropping in moves the clock past it
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	all := append(append(tuiSteps(s), dropInSteps(s)...), recordSteps(s)...)
	all = append(append(all, cardSteps(s)...), editSteps(s)...)
	all = append(append(all, browseSteps(s)...), keySteps(s)...)
	all = append(append(all, scriptSteps(s)...), emptySteps(s)...)
	return append(append(all, commandSteps(s)...), panelSteps(s)...)
}

// runNow runs the developer's editor at once, as the TUI's Exec, since the steps have no
// terminal to hand over.
func runNow(command *exec.Cmd, done tea.ExecCallback) tea.Cmd {
	err := command.Run()
	return func() tea.Msg { return done(err) }
}

// openTUI opens the sample shop in the TUI, as bare givens does.
func (s *state) openTUI() error {
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	s.model = tui.New(s.Window, runNow)
	s.update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return nil
}

// update sends one message, then whatever its command gives, as a Program would, leaving
// out the ticks: the steps send their own.
func (s *state) update(message tea.Msg) {
	// bound: each command gives one message, and the commands this model returns end.
	for range 64 {
		_, cmd := s.model.Update(message)
		if cmd == nil {
			return
		}
		next := cmd()
		if _, quit := next.(tea.QuitMsg); quit {
			s.quit = true
			return
		}
		if next == nil || tui.IsTick(next) {
			return
		}
		message = next
	}
}

// press sends keys: "enter", "tab", "esc", "backspace", "space", the arrows, or one
// character.
func (s *state) press(keys ...string) error {
	if s.model == nil {
		return errors.New("the TUI isn't open")
	}
	named := map[string]rune{"enter": tea.KeyEnter, "tab": tea.KeyTab, "esc": tea.KeyEscape,
		"backspace": tea.KeyBackspace, "space": tea.KeySpace, "up": tea.KeyUp,
		"down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight}
	for _, key := range keys {
		if code, ok := named[key]; ok {
			s.update(tea.KeyPressMsg{Code: code})
			continue
		}
		for _, letter := range key {
			s.update(tea.KeyPressMsg{Code: letter, Text: string(letter)})
		}
	}
	return nil
}

// settle sends ticks until setup has finished or paused, or until world.SetupWait.
func (s *state) settle() *app.Session {
	deadline := time.Now().Add(world.SetupWait)
	// bound: the deadline ends it.
	for time.Now().Before(deadline) {
		s.update(tui.Tick())
		session := s.model.Session()
		if session == nil || session.Status() != app.SessionRunning {
			return session
		}
		time.Sleep(20 * time.Millisecond)
	}
	return s.model.Session()
}

// sees expects every text in what the TUI draws.
func (s *state) sees(texts ...string) error {
	if s.model == nil {
		return errors.New("the TUI isn't open")
	}
	shown := s.model.Text()
	for _, text := range texts {
		if !strings.Contains(shown, text) {
			return fmt.Errorf("the TUI doesn't show %q; it shows:\n%s", text, shown)
		}
	}
	return nil
}

// shownLine expects a line of the TUI to hold every text.
func (s *state) shownLine(texts ...string) error {
	for _, line := range strings.Split(s.model.Text(), "\n") {
		holds := true
		for _, text := range texts {
			holds = holds && strings.Contains(line, text)
		}
		if holds {
			return nil
		}
	}
	return fmt.Errorf("no line shows %q:\n%s", texts, s.model.Text())
}

// dropInTUI opens the TUI, moves to a scenario and presses Enter.
func (s *state) dropInTUI(name string) error {
	if s.model == nil {
		if err := s.openTUI(); err != nil {
			return err
		}
	}
	if !s.model.MoveTo(name) {
		return fmt.Errorf("the TUI doesn't list %q:\n%s", name, s.model.Text())
	}
	if err := s.press("enter"); err != nil {
		return err
	}
	s.Session = s.model.Session()
	if s.Session == nil {
		return fmt.Errorf("no Drop in opened:\n%s", s.model.Text())
	}
	if s.slow {
		s.Clock.Advance(2 * time.Second)
		s.update(tui.Tick())
		return nil
	}
	s.settle()
	return nil
}

// ready expects setup to be done, with a checklist.
func (s *state) ready() error {
	session := s.settle()
	if session == nil || session.Status() != app.SessionReady {
		return fmt.Errorf("setup isn't done:\n%s", s.model.Text())
	}
	return nil
}

func tuiSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer's terminal is in the sample shop's folder$`, s.CopySampleShop),
		world.Def(`^they run "givens" in a terminal$`, s.openTUI),
		world.Def(`^a developer has the sample shop open in the TUI$`, s.openTUI),
		world.Def(`^they see each capability with its scenario count, folded$`, func() error {
			return s.listedNot("+ Checkout (5)", "Card declined")
		}),
		world.Def(`^unfolding "Checkout" shows each scenario with its status, like `+
			`"([^"]*)" with "([^"]*)"$`, func(name string, status string) error {
			if err := s.press("space"); err != nil {
				return err
			}
			return s.shownLine(name, status)
		}),
		world.Def(`^they move to "([^"]*)"$`, func(name string) error {
			if !s.model.MoveTo(name) {
				return fmt.Errorf("the TUI doesn't list %q", name)
			}
			return nil
		}),
		world.Def(`^its steps show beside the list, with its setup script's path$`,
			func() error {
				return s.sees("Given a signed-in shopper with one item in the cart",
					"When they place the order", "Setup: checkout/scripts/card-declined.sh")
			}),
		world.Def(`^they press Tab until "Views" is shown and choose "([^"]*)"$`, s.chooseView),
		world.Def(`^the list holds the scenarios tagged @error$`, s.errorStates),
		world.Def(`^they press "/" and type "([^"]*)"$`, func(words string) error {
			return s.press("/", words)
		}),
		world.Def(`^"([^"]*)" is among the results$`, func(name string) error {
			return s.sees(name)
		}),
		world.Def(`^pressing Enter on it shows "([^"]*)"$`, func(name string) error {
			if err := s.press("enter"); err != nil {
				return err
			}
			return s.sees("── " + name)
		}),
	}
}

// chooseView presses Tab until Views is on show, then Enter on the view.
func (s *state) chooseView(view string) error {
	for range 3 {
		if strings.Contains(s.model.Text(), "[Views]") {
			break
		}
		if err := s.press("tab"); err != nil {
			return err
		}
	}
	if !s.model.MoveTo(view) {
		return fmt.Errorf("Views doesn't list %q:\n%s", view, s.model.Text())
	}
	return s.press("enter")
}

// errorStates expects every scenario tagged @error in the list.
func (s *state) errorStates() error {
	for _, name := range []string{"Card declined", "3-D Secure failed", "Empty cart",
		"Expired link", "Invite declined"} {
		if err := s.sees(name); err != nil {
			return err
		}
	}
	return nil
}
