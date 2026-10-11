// Package sharesetupbetweenscenarios runs capabilities/share-setup-between-scenarios: helpers
// in _shared folders, the sidebar's Shared list, and setup fingerprints. Steps shared with other
// capabilities are in acceptance/world.
package sharesetupbetweenscenarios

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"givens/acceptance/world"
)

const capability = "share-setup-between-scenarios"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	pair   []string // the scenarios a "both" step names
	helper string   // the helper a step selected, edited or changed
	hint   string   // the install hint on the need line of a helper a scenario calls
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return append(append(givenSteps(s), whenSteps(s)...), thenSteps(s)...)
}

func givenSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the "([^"]*)" setup calls ([a-z_]+), which only Checkout's helpers define$`,
			s.callsCheckoutHelper),
		world.Def(`^Checkout's helpers define a seed_shopper of their own$`, s.checkoutOverride),
		world.Def(`^two files in _shared define seed_shopper$`, s.sharedDuplicate),
		world.Def(`^"([^"]*)" and "([^"]*)" both have (?:a passing run|passing runs) recorded$`,
			s.bothPassed),
		world.Def(`^([a-z_]+) calls ([a-z_]+)$`, s.helperCallsHelper),
		world.Def(`^"([^"]*)" calls ([a-z_]+), which runs (\S+) with ([a-z]+)$`,
			s.callsHelperRunning),
		world.Def(`^a developer is editing ([a-z_]+) in Givens$`, s.editingHelper),
	}
}

func whenSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^([a-z_]+) changes on disk$`, s.helperChanges),
		world.Def(`^"([^"]*)" is changed to "([^"]*)"$`, s.stepChanges),
		world.Def(`^the "([^"]*)" setup script changes$`, s.setupChanges),
		world.Def(`^the sample shop's kit is updated from version 1 to 2$`, s.kitUpdated),
		world.Def(`^they select ([a-z_]+) under Shared and choose "([^"]*)"$`, s.chooseOnHelper),
		world.Def(`^they choose "New helper" in (\S+) and name it "([a-z_]+)"$`, s.newHelper),
		world.Def(`^they save it with its closing brace missing$`, s.saveWithoutBrace),
	}
}

func thenSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^setup fails with "([^"]*)"$`, s.setupFailsWith),
		world.Def(`^Givens points out that ([a-z_]+) is defined in (\S+)$`, s.pointsOut),
		world.Def(`^Checkout's seed_shopper runs in place of the shared one$`, s.overrideRan),
		world.Def(`^no warning is shown$`, s.noWarning),
		world.Def(`^the Shared list marks it "([^"]*)"$`, s.marksOverride),
		world.Def(`^the output warns "([^"]*)"$`, s.outputWarns),
		world.Def(`^both definitions are flagged in the Shared list$`, s.bothFlagged),
		world.Def(`^"([^"]*)" still shows as verified$`, s.stillVerified),
		world.Def(`^both show "([^"]*)"$`, s.bothShow),
		world.Def(`^if ([a-z]+) isn't installed, it's marked "([^"]*)" with its install hint$`,
			s.markedIfMissing),
		world.Def(`^(\S+) opens in the editor at ([a-z_]+)$`, s.opensAt),
		world.Def(`^the (\d+) scripts that call it are listed beside the editor$`,
			s.callersListed),
		world.Def(`^(\S+) is created with a # @helper comment and an empty ([a-z_]+) function$`,
			s.helperCreated),
		world.Def(`^([a-z_]+) is listed under Shared as one of (\S+)'s helpers$`,
			s.listedUnder),
		world.Def(`^the line bash reports is marked with its syntax error$`, s.syntaxMarked),
	}
}

// callsCheckoutHelper makes the scenario's setup call helper, after checking only Checkout's
// _shared folder defines it.
func (s *state) callsCheckoutHelper(scenario string, helper string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	definers, err := s.definersOf(helper)
	if err != nil {
		return err
	}
	if len(definers) != 1 || !strings.HasPrefix(definers[0], "checkout/_shared/") {
		return fmt.Errorf("expected only Checkout's helpers to define %s, got %q", helper, definers)
	}
	return s.SetupStartsWith(scenario, helper)
}

// checkoutShopper is Checkout's own seed_shopper; its ok line tells it apart from the shared one.
const checkoutShopper = `# shellcheck shell=bash

# @helper seed_shopper [email]
# Seeds a shopper with Checkout's own test card.
seed_shopper() {
  ok "seeded Checkout's own shopper"
}
`

func (s *state) checkoutOverride() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.WriteFile("capabilities/checkout/_shared/shopper.sh", checkoutShopper)
}

func (s *state) sharedDuplicate() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	definers, err := s.definersOf("seed_shopper")
	if err != nil {
		return err
	}
	if len(definers) != 1 || definers[0] != "_shared/users.sh" {
		return fmt.Errorf("expected only _shared/users.sh to define seed_shopper, got %q",
			definers)
	}
	people := strings.Replace(checkoutShopper, "Checkout's own", "people.sh's", 2)
	return s.WriteFile("capabilities/_shared/people.sh", people)
}

func (s *state) bothPassed(first string, second string) error {
	s.pair = []string{first, second}
	for _, scenario := range s.pair {
		if err := s.RecordPassingRun(scenario); err != nil {
			return err
		}
	}
	return nil
}

// helperCallsHelper checks the sample shop as written: caller's body calls callee.
func (s *state) helperCallsHelper(caller string, callee string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	body, _, err := s.helperBody(caller)
	if err != nil {
		return err
	}
	if !callsWord(body, callee) {
		return fmt.Errorf("expected %s to call %s; its body is:\n%s", caller, callee, body)
	}
	return nil
}

// callsHelperRunning checks the sample shop as written: the scenario's script calls helper,
// whose body needs command and runs file with it. It keeps the need line's install hint.
func (s *state) callsHelperRunning(scenario, helper, file, command string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	script, err := s.ScriptPath(scenario)
	if err != nil {
		return err
	}
	text, err := s.ReadFile(script)
	if err != nil {
		return err
	}
	if !callsWord(text, helper) {
		return fmt.Errorf("expected %s to call %s; it holds:\n%s", script, helper, text)
	}
	body, _, err := s.helperBody(helper)
	if err != nil {
		return err
	}
	if !strings.Contains(body, command+` "$GIVENS_ROOT/`+file+`"`) {
		return fmt.Errorf("expected %s to run %s with %s; its body is:\n%s",
			helper, file, command, body)
	}
	need := regexp.MustCompile(`need ` + command + ` "([^"]*)"`).FindStringSubmatch(body)
	if need == nil {
		return fmt.Errorf("expected %s to have a need %s line; its body is:\n%s",
			helper, command, body)
	}
	s.hint = need[1]
	_, err = os.Stat(filepath.Join(s.Capabilities(), file))
	return err
}

func (s *state) editingHelper(helper string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.chooseOnHelper(helper, "Edit")
}

func (s *state) chooseOnHelper(helper string, action string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	s.helper = helper
	if err := window.Select(helper); err != nil {
		return err
	}
	return window.Choose(action)
}

// helperChanges adds a line to the start of the helper's body, in the one file defining it.
func (s *state) helperChanges(helper string) error {
	_, path, err := s.helperBody(helper)
	if err != nil {
		return err
	}
	text, err := s.ReadFile(path)
	if err != nil {
		return err
	}
	header := helper + "() {\n"
	if strings.Count(text, header) != 1 {
		return fmt.Errorf("expected one %q in %s", header, path)
	}
	changed := strings.Replace(text, header, header+"  export SHOP_SEED_REVISION=2\n", 1)
	return s.WriteFile(path, changed)
}

func (s *state) stepChanges(from string, to string) error {
	path := filepath.Join("capabilities", "checkout", "checkout.feature")
	text, err := s.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Count(text, from) != 1 {
		return fmt.Errorf("expected %q once in %s", from, path)
	}
	return s.WriteFile(path, strings.Replace(text, from, to, 1))
}

func (s *state) setupChanges(scenario string) error {
	script, err := s.ScriptPath(scenario)
	if err != nil {
		return err
	}
	text, err := s.ReadFile(script)
	if err != nil {
		return err
	}
	return s.WriteFile(script, text+"ok \"set the cart's currency to GBP\"\n")
}

// kitUpdated rewrites the kit's version line, `# givens kit <n>`, and adds a function, as an
// update from version 1 to 2 would.
func (s *state) kitUpdated() error {
	path := filepath.Join("capabilities", "_shared", "givens.sh")
	text, err := s.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("the sample shop has no kit: internal/kit/givens.sh isn't built yet")
	}
	if err != nil {
		return err
	}
	version := regexp.MustCompile(`(?m)^# givens kit 1$`)
	if !version.MatchString(text) {
		return fmt.Errorf("expected the kit to start at version 1 (`# givens kit 1`), got:\n%s",
			firstLine(text))
	}
	updated := version.ReplaceAllString(text, "# givens kit 2") +
		"\n_givens_version_2() { :; }\n"
	return s.WriteFile(path, updated)
}

func (s *state) newHelper(capabilityName string, helper string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	s.helper = helper
	return window.NewHelper(capabilityName, helper)
}

// saveWithoutBrace saves the open helper's file with the helper's closing brace removed.
func (s *state) saveWithoutBrace() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	path := window.EditingHelper().Path
	if path == "" {
		return errors.New("no helper is open in the editor")
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	edited, err := withoutClosingBrace(string(text), s.helper)
	if err != nil {
		return err
	}
	s.EditedPath, s.EditedText = path, edited
	return window.SaveHelper(edited)
}
