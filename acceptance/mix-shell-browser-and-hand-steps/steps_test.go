// Package mixshellbrowserandhandsteps runs capabilities/mix-shell-browser-and-hand-steps
// against a fake shop page and a real, headless Chrome. Steps shared with other capabilities
// are in acceptance/world. The spec's http://localhost:3000 is swapped for the fake shop's
// own address wherever a step writes or reads it.
package mixshellbrowserandhandsteps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"givens/acceptance/shop"
	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/browser"
)

const capability = "mix-shell-browser-and-hand-steps"

func TestScenarios(t *testing.T) {
	if _, ok := browser.Chrome(); !ok {
		t.Fatal("these scenarios need Chrome: brew install --cask google-chrome")
	}
	world.RunSuite(t, capability, steps)
}

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	added     map[string]int  // lines the steps put in each setup, after the kit's line
	opened    map[string]bool // a setup has a browser_open line
	cardSteps []string        // pay-in-app.md's steps, as written after "N. "
	page      string          // the project's page before the When step
	opener    *urlOpener      // the fake default browser, once a step sets one
}

func steps(w *world.World) []world.Step {
	s := &state{World: w, added: make(map[string]int, 2), opened: make(map[string]bool, 2)}
	return append(append(scriptSteps(s), browserSteps(s)...), cardStepsOf(s)...)
}

// scriptSteps are the Given steps that write a setup script, line by line.
func scriptSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the "([^"]*)" setup has browser_open "([^"]*)"$`,
			func(name string, url string) error {
				return s.addLine(name, `browser_open "`+url+`"`)
			}),
		world.Def(`^the "([^"]*)" setup seeds a shopper with a shell command$`,
			func(name string) error { return s.addLine(name, `ok "seeded $(echo sam)"`) }),
		world.Def(`^then signs in with app_action signIn '([^']*)'$`, func(who string) error {
			return s.withPage("Card declined", `app_action signIn '`+who+`'`)
		}),
		world.Def(`^then has a by_hand step for "([^"]*)" with the text "([^"]*)"$`,
			func(where string, text string) error {
				return s.addLine("Card declined", `by_hand "`+where+`" "`+text+`"`)
			}),
		world.Def(`^the "([^"]*)" setup has app_action addToCart '([^']*)', then ok "([^"]*)"$`,
			func(name string, item string, message string) error {
				return s.withPage(name, `app_action addToCart '`+item+`'`, `ok "`+message+`"`)
			}),
		world.Def(`^the "([^"]*)" setup has ORDER="\$\(in_browser '([^']*)'\)"$`,
			func(name string, js string) error {
				return s.withPage(name, `ORDER="$(in_browser '`+js+`')"`,
					`ok "ORDER is $ORDER"`)
			}),
		world.Def(`^the "([^"]*)" setup has in_browser '([^']*)'$`,
			func(name string, js string) error {
				return s.withPage(name, `in_browser '`+js+`'`)
			}),
		world.Def(`^the "([^"]*)" setup has app_action refund '([^']*)'$`,
			func(name string, order string) error {
				return s.withPage(name, `app_action refund '`+order+`'`)
			}),
		world.Def(`^the "([^"]*)" setup has in_browser with a promise that never settles$`,
			func(name string) error {
				return s.withPage(name, `in_browser 'new Promise(() => {})'`)
			}),
		world.Def(`^the "([^"]*)" setup signs in as "([^"]*)" with app_action$`,
			func(name string, email string) error {
				return s.withPage(name, `app_action signIn '{"email":"`+email+`"}'`)
			}),
		world.Def(`^the "([^"]*)" setup opens the Givens browser$`,
			func(name string) error { return s.withPage(name) }),
		world.Def(`^then has by_hand --url "([^"]*)" for "([^"]*)"$`,
			func(url string, where string) error {
				return s.addLine("Card declined",
					`by_hand --url "`+url+`" "`+where+`" "Look at the admin page"`)
			}),
		world.Def(`^the "([^"]*)" setup has browser_open and app_action lines$`,
			func(name string) error {
				return s.withPage(name, `app_action signIn '{"email":"sam@example.com"}'`)
			}),
		world.Def(`^the shop's window\.app\.addToCart returns a promise that settles after 2 `+
			`seconds$`, func() error { return s.shopShows("addToCart") }),
		world.Def(`^the shop's page returns "ord_42" from window\.app\.lastOrderId\(\)$`,
			func() error { return s.shopShows("lastOrderId") }),
		world.Def(`^the shop's page has no window\.app\.refund$`,
			func() error { return s.shopLacks("refund") }),
		world.Def(`^Chrome isn't installed$`, func() error {
			return os.Setenv(browser.ChromeEnv, filepath.Join(s.Dir, "no-chrome-here"))
		}),
	}
}

// addLine puts line in a scenario's setup script, after the kit's line and the lines earlier
// steps put there, so lines run in the order the steps give them. The fixture's own lines
// stay after.
func (s *state) addLine(name string, line string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	path, err := s.ScriptPath(name)
	if err != nil {
		return err
	}
	text, err := s.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.Split(strings.TrimPrefix(text, world.KitSource+"\n"), "\n")
	at := min(s.added[name], len(lines))
	s.opened[name] = s.opened[name] || strings.HasPrefix(line, "browser_open ")
	lines = append(lines[:at], append([]string{s.ShopFor().URL(line)}, lines[at:]...)...)
	s.added[name]++
	return s.WriteFile(path, world.KitSource+"\n"+strings.Join(lines, "\n"))
}

// withPage opens the shop's page first, unless an earlier line did, then adds the lines: an
// app action or in_browser needs a page to run in.
func (s *state) withPage(name string, lines ...string) error {
	if !s.opened[name] {
		if err := s.addLine(name, `browser_open "`+world.ShopURL+`/"`); err != nil {
			return err
		}
	}
	for _, line := range lines {
		if err := s.addLine(name, line); err != nil {
			return err
		}
	}
	return nil
}

// shopShows expects the fake shop's page to set window.app.<name>.
func (s *state) shopShows(name string) error {
	if !shop.Has(name) {
		return fmt.Errorf("the fake shop's page has no window.app.%s", name)
	}
	s.ShopFor()
	return nil
}

// shopLacks expects the fake shop's page not to set window.app.<name>.
func (s *state) shopLacks(name string) error {
	if shop.Has(name) {
		return fmt.Errorf("the fake shop's page has window.app.%s", name)
	}
	s.ShopFor()
	return nil
}

// session is the Drop in under test.
func (s *state) session() (*app.Session, error) {
	if s.Session == nil {
		return nil, errors.New("no step dropped in")
	}
	return s.Session, nil
}

// outputShows expects a line of setup's output to hold text, the shop's address swapped in.
func (s *state) outputShows(text string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	return world.Expect(session.Output(), s.ShopFor().URL(text), "setup's output")
}

// noChecklist expects setup to have ended without a checklist.
func (s *state) noChecklist() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if items := session.Checklist(); len(items) > 0 {
		return fmt.Errorf("a checklist of %d items is shown", len(items))
	}
	if status := session.Status(); status == app.SessionReady {
		return errors.New("setup ended ready")
	}
	return nil
}
