package workintheterminal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// scriptSteps are setup scripts and helpers from the TUI: s on a scenario, and the Helpers
// tab.
func scriptSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^"([^"]*)" has no setup yet$`, s.noSetupYet),
		world.Def(`^checkout/scripts/empty-cart\.sh is written with the kit's source line, `+
			`and opens in their editor$`, func() error {
			if err := s.holds("capabilities/checkout/scripts/empty-cart.sh",
				app.KitSourceLine); err != nil {
				return err
			}
			if err := s.calledWith("", "empty-cart.sh"); err != nil {
				return fmt.Errorf("%w\nthe TUI shows:\n%s", err, s.model.Text())
			}
			return nil
		}),
		world.Def(`^each helper is listed with where it's defined, like "([^"]*)" with `+
			`"([^"]*)"$`, func(name string, file string) error { return s.shownLine(name, file) }),
		world.Def(`^pressing Enter on "seed_shopper" opens users\.sh in vim at its function$`,
			func() error {
				if !s.model.MoveTo("seed_shopper") {
					return fmt.Errorf("Helpers doesn't list seed_shopper:\n%s", s.model.Text())
				}
				if err := s.press("enter"); err != nil {
					return err
				}
				line, err := s.lineOf("capabilities/_shared/users.sh", "seed_shopper() {")
				if err != nil {
					return err
				}
				return s.calledWith("+"+strconv.Itoa(line)+" ", "users.sh")
			}),
		world.Def(`^a developer's editor names a new helper "([^"]*)" in _shared$`,
			func(name string) error {
				return s.FakeEditor(`if grep -q '^Name: $' "$1"; then sed -i '' ` +
					`'s/^Name: $/Name: ` + name + `/' "$1"; fi`)
			}),
		world.Def(`^they press Tab until "Helpers" is shown and press "n"$`, func() error {
			if err := s.tabTo("Helpers"); err != nil {
				return err
			}
			return s.press("n")
		}),
		world.Def(`^capabilities/_shared/([a-z_]+)\.sh defines ([a-z_]+)$`,
			func(file string, name string) error {
				return s.holds("capabilities/_shared/"+file+".sh", name+"() {")
			}),
		world.Def(`^"([^"]*)" is listed$`, func(name string) error { return s.sees(name) }),
	}
}

// noSetupYet takes a checkout scenario's setup files away.
func (s *state) noSetupYet(name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	scripts := filepath.Join(s.Capabilities(), "checkout", "scripts")
	for _, extension := range []string{".sh", ".md", ".teardown.sh"} {
		err := os.Remove(filepath.Join(scripts, app.SlugOf(name)+extension))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// lineOf is the line, counting from 1, of the first line of a sample shop file holding text.
func (s *state) lineOf(file string, text string) (int, error) {
	content, err := s.ReadFile(file)
	if err != nil {
		return 0, err
	}
	for index, line := range strings.Split(content, "\n") {
		if strings.Contains(line, text) {
			return index + 1, nil
		}
	}
	return 0, fmt.Errorf("%s doesn't hold %q", file, text)
}
