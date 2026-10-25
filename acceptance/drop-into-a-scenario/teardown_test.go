package dropintoascenario

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

const teardownLine = "✓ cleared orders"

// droppedInMarkedDone drops in to a setup that counts its runs, and marks the step containing
// text done.
func (s *state) droppedInMarkedDone(name string, text string) error {
	err := s.writeSetup(name, script(
		`echo run >> `+shopData+`/setup-runs"`, `ok "seeded shopper"`))
	if err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	session, err := s.dropInTo(name, app.SessionReady)
	if err != nil {
		return err
	}
	for _, item := range session.Checklist() {
		if strings.Contains(item.Text, text) {
			return session.Mark(item.Text, "done")
		}
	}
	return fmt.Errorf("the checklist has no step %q: %+v", text, session.Checklist())
}

func (s *state) setupRanAgain() error {
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return err
	}
	runs, _, err := s.shopFile("setup-runs")
	if err != nil {
		return err
	}
	if count := strings.Count(runs, "run\n"); count != 2 {
		return fmt.Errorf("setup ran %d times, not twice", count)
	}
	return world.Expect(session.Output(), firstLine, "the output after Reset state")
}

func (s *state) checklistCleared() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	items := session.Checklist()
	if len(items) == 0 {
		return errors.New("there's no checklist after Reset state")
	}
	for _, item := range items {
		if item.Mark != "" {
			return fmt.Errorf("%q is still marked %q after Reset state", item.Text, item.Mark)
		}
	}
	return nil
}

// noTeardown removes the scenario's teardown and gives it an idempotent setup: it seeds the
// shopper only if the shop doesn't have them yet. It selects the scenario, so its actions,
// such as "Add teardown", are offered.
func (s *state) noTeardown(name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	path, err := s.teardownPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.Project, path)); err != nil && !os.IsNotExist(err) {
		return err
	}
	shoppers := shopData + `/shoppers.txt"`
	err = s.writeSetup(name, script(
		"touch "+shoppers,
		"grep -qx 'sam@example.com' "+shoppers+" || echo 'sam@example.com' >> "+shoppers,
		`ok "seeded shopper"`,
	))
	if err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	return s.Window.Select(name)
}

func (s *state) droppedInOnce(name string) error {
	_, err := s.dropInTo(name, app.SessionReady)
	return err
}

func (s *state) finishesWithoutErrors() error {
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return err
	}
	return world.ExpectNot(session.Output(), "✗", "the output")
}

func (s *state) oneShopper(email string) error {
	shoppers, _, err := s.shopFile("shoppers.txt")
	if err != nil {
		return err
	}
	if count := strings.Count(shoppers, email+"\n"); count != 1 {
		return fmt.Errorf("the shop has %d shoppers %q, not one", count, email)
	}
	return nil
}

// hasTeardown gives the scenario a teardown that prints one line, and a setup that fails
// while shop-data/fail-setup exists, so a step can reach "Try again".
func (s *state) hasTeardown(name string, file string) error {
	s.scenario = name
	if err := s.writeTeardown(name, script(`ok "cleared orders"`)); err != nil {
		return err
	}
	path, err := s.teardownPath(name)
	if err != nil {
		return err
	}
	if filepath.Base(path) != file {
		return fmt.Errorf("%q's teardown is %s, not %s", name, filepath.Base(path), file)
	}
	err = s.writeSetup(name, script(`ok "seeded shopper"`,
		`if [ -f `+shopData+`/fail-setup" ]; then fail "card service is down"; fi`))
	if err != nil {
		return err
	}
	return s.prepare()
}

// everyWayIn drops in, chooses "Reset state", then makes setup fail once to reach "Try again",
// keeping the output after each.
func (s *state) everyWayIn() error {
	session, err := s.dropInTo(s.scenario, app.SessionReady)
	if err != nil {
		return err
	}
	s.outputs["drop in"] = session.Output()
	if err := s.chooseAndKeep("Reset state", app.SessionReady); err != nil {
		return err
	}
	failMarker := filepath.Join(shopDataFolder, "fail-setup")
	if err := s.WriteFile(failMarker, ""); err != nil {
		return err
	}
	if err := s.chooseAndKeep("Reset state", app.SessionFailed); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.Project, failMarker)); err != nil {
		return err
	}
	return s.chooseAndKeep("Try again", app.SessionReady)
}

// chooseAndKeep takes an action, waits for setup to end in want, and keeps its output.
func (s *state) chooseAndKeep(action string, want string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Choose(action); err != nil {
		return err
	}
	session, err := s.settleTo(want)
	if err != nil {
		return fmt.Errorf("after %q: %w", action, err)
	}
	if want == app.SessionReady {
		s.outputs[action] = session.Output()
	}
	return nil
}

func (s *state) teardownFirst() error {
	if len(s.outputs) != 3 {
		return fmt.Errorf("only %d ways in were tried", len(s.outputs))
	}
	for way, output := range s.outputs {
		at := indexOf(output, "→ teardown", 0)
		if at < 0 || at+1 >= len(output) || !strings.Contains(output[at+1], teardownLine) {
			return fmt.Errorf("after %s, \"→ teardown\" isn't followed by %q: %q", way,
				teardownLine, output)
		}
	}
	return nil
}

func (s *state) setupAfterTeardown() error {
	for way, output := range s.outputs {
		teardown := indexOf(output, teardownLine, 0)
		if teardown < 0 || indexOf(output, firstLine, teardown) < 0 {
			return fmt.Errorf("after %s, setup didn't run after the teardown: %q", way, output)
		}
	}
	return nil
}

func (s *state) teardownCreated() error {
	text, err := s.ReadFile("capabilities/checkout/scripts/card-declined.teardown.sh")
	if err != nil {
		return err
	}
	if first, _, _ := strings.Cut(text, "\n"); first != world.KitSource {
		return fmt.Errorf("the teardown starts with %q, not %q", first, world.KitSource)
	}
	return nil
}

// teardownFails gives Card declined a teardown that exits 2, and a setup that leaves a marker.
func (s *state) teardownFails() error {
	if err := s.writeTeardown("Card declined", script("exit 2")); err != nil {
		return err
	}
	err := s.writeSetup("Card declined", script(touch("setup-ran"), `ok "seeded shopper"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) setupDidNotRun() error {
	session, err := s.settleTo(app.SessionFailed)
	if err != nil {
		return err
	}
	if _, ran, err := s.shopFile("setup-ran"); err != nil || ran {
		return fmt.Errorf("setup ran after the teardown failed (%v)", err)
	}
	return world.ExpectNot(session.Output(), firstLine, "the output")
}
