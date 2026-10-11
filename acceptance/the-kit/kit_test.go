// Package thekit runs capabilities/the-kit. The kit is capabilities/_shared/givens.sh, built
// from internal/kit/givens.sh; these steps need no app. Each Given writes a small setup script
// into the sample shop, each When runs it with bash, and each Then reads its output. Steps
// shared with other capabilities are in acceptance/world.
package thekit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"givens/acceptance/world"
)

const capability = "the-kit"

// scriptPath is where the scenarios' setup script goes: a real scenario's script, so the kit
// loads the shared and Checkout helpers around it as it would for any setup.
const scriptPath = "capabilities/checkout/scripts/card-declined.sh"

// The by_hand step the by_hand scenarios share.
const (
	byHandWhere   = "DigitalOcean console"
	byHandSnippet = "Open the droplet's console and run: systemctl restart shop"
	byHandAfter   = "✓ carried on after the step by hand"
)

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	top         []string // lines just after the kit's source line
	body        []string // the rest of the script
	waited      bool     // the script was running and waiting when the first answer was typed
	firstOutput string   // When mode: the first run's output
	firstExit   int      // When mode: the first run's exit code
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return append(append(givenSteps(s), whenSteps(s)...), thenSteps(s)...)
}

func givenSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a setup script's second command fails$`, s.secondCommandFails),
		world.Def(`^a setup script calls first_you, then when_you twice, then then_check$`,
			s.yourTurnScript),
		world.Def(`^the same script as kit-your-turn$`, s.yourTurnScript),
		world.Def(`^a setup script calls (ok|warn|fail) "([^"]*)"$`, s.callsOne),
		world.Def(`^a setup script calls warn "([^"]*)", then ok "([^"]*)"$`, s.warnThenOk),
		world.Def(`^a setup script has need ([a-z]+) "([^"]*)"$`, s.needs),
		world.Def(`^([a-z]+) isn't installed$`, s.notInstalled),
		world.Def(`^a setup script ends with when_you "run this: \$\(when_command\)"$`,
			s.endsWithWhenCommand),
		world.Def(`^it has if when_mode; then …; exit 0; fi near the top$`, s.whenModeAtTop),
		world.Def(`^a setup script has a by_hand step for "([^"]*)" with a snippet$`,
			s.byHandFor),
		world.Def(`^a setup script has a by_hand step$`, s.byHand),
		world.Def(`^a setup script is waiting at a by_hand step in a terminal$`, s.byHand),
		world.Def(`^a setup script has ask ([A-Z_]+) "([^"]*)"$`, s.asks),
	}
}

func whenSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer runs the script in a terminal$`, s.runInTerminal),
		world.Def(`^a developer runs it in a terminal$`, s.runInTerminalPressingEnter),
		world.Def(`^a developer runs it in a terminal and types "([^"]*)"$`, s.runTyping),
		world.Def(`^the developer types f and presses Enter$`, s.typeF),
		world.Def(`^it runs$`, s.runPlain),
		world.Def(`^it runs with GIVENS_QUIET=1$`, s.runQuiet),
		world.Def(`^it runs with no terminal and not from Givens, as in CI$`, s.runPlain),
		world.Def(`^its output is sent to a file$`, s.runToFile),
		world.Def(`^a developer runs it, then runs the command it prints$`, s.runTwice),
	}
}

func thenSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^setup stops at that command$`, s.stopsAtCommand),
		world.Def(`^the lines after it don't run$`, s.linesAfterDontRun),
		world.Def(`^the script exits with a non-zero code(?: instead of waiting)?$`,
			s.exitsNonZero),
		world.Def(`^the script exits with 0$`, s.exitsZero),
		world.Def(`^"Your turn" is printed once, just before the first_you line$`,
			s.yourTurnOnce),
		world.Def(`^no "Your turn" heading or first_you, when_you or then_check lines are printed$`,
			s.noYourTurnLines),
		world.Def(`^ok, warn and fail lines are still printed$`, s.statusLinesPrinted),
		world.Def(`^the file holds "([^"]*)" with no colour codes$`, s.fileHoldsPlain),
		world.Def(`^nothing after the need line runs$`, s.nothingAfterNeed),
		world.Def(`^both lines are printed$`, s.bothLinesPrinted),
		world.Def(`^the first run prints "When you run this: bash capabilities/…/<slug>\.sh `+
			`--when"$`, s.firstRunPrintsCommand),
		world.Def(`^the second run does only the When part, without rerunning setup, then `+
			`exits$`, s.secondRunWhenOnly),
		world.Def(`^it prints "([^"]*)" followed by the snippet$`, s.printsSnippet),
		world.Def(`^it waits with "([^"]*)"$`, s.waitsWith),
		world.Def(`^pressing Enter carries on with the rest of setup$`, s.carriesOn),
		world.Def(`^the rest of the script sees ([A-Z_]+) as "([^"]*)"$`, s.seesVariable),
	}
}

// writeScript writes the kit's source line, then top, then body, to scriptPath in a fresh copy
// of the sample shop. It fails while the kit isn't built, since every scenario here needs it.
func (s *state) writeScript() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	kit := filepath.Join(s.Capabilities(), "_shared", "givens.sh")
	if _, err := os.Stat(kit); errors.Is(err, os.ErrNotExist) {
		return errors.New("the sample shop has no kit: internal/kit/givens.sh isn't built yet")
	} else if err != nil {
		return err
	}
	lines := append(append([]string{world.KitSource}, s.top...), s.body...)
	return s.WriteFile(scriptPath, strings.Join(lines, "\n")+"\n")
}

func (s *state) setBody(lines ...string) error {
	s.body = lines
	return s.writeScript()
}

func (s *state) secondCommandFails() error {
	return s.setBody(
		`ok "seeded shopper"`,
		`ls missing-folder`,
		`ok "seeded the catalogue"`,
		`touch ran-after-failure`,
	)
}

// yourTurnScript is the script kit-your-turn and kit-quiet share: status lines around the
// "your turn" lines, ending in a fail so quiet mode can show fail lines still print.
func (s *state) yourTurnScript() error {
	return s.setBody(
		`ok "seeded shopper"`,
		`warn "Mailpit isn't running"`,
		`first_you 'select "Checkout"'`,
		`when_you 'click "Place order"'`,
		`when_you 'click "Confirm"'`,
		`then_check 'you see "Saved"'`,
		`fail "No database found"`,
	)
}

func (s *state) callsOne(function string, message string) error {
	return s.setBody(function+" "+shellQuote(message), `touch ran-after-`+function)
}

func (s *state) warnThenOk(warning string, message string) error {
	return s.setBody("warn "+shellQuote(warning), "ok "+shellQuote(message))
}

func (s *state) needs(command string, hint string) error {
	return s.setBody(
		"need "+command+" "+shellQuote(hint),
		`ok "set up the shop"`,
		`touch ran-after-need`,
	)
}

// notInstalled checks the command really is missing here, so the scenario tests the kit and
// not this machine.
// Ref: https://pkg.go.dev/os/exec#LookPath
func (s *state) notInstalled(command string) error {
	if path, err := exec.LookPath(command); err == nil {
		return fmt.Errorf("%s is installed on this machine at %s; the scenario needs it missing",
			command, path)
	}
	return nil
}

func (s *state) endsWithWhenCommand() error {
	return s.setBody(`ok "seeded shopper"`, `when_you "run this: $(when_command)"`)
}

func (s *state) whenModeAtTop() error {
	s.top = []string{`if when_mode; then`, `  ok "placed the order"`, `  exit 0`, `fi`}
	return s.writeScript()
}

func (s *state) byHandFor(where string) error {
	return s.setBody(
		`ok "seeded shopper"`,
		"by_hand "+shellQuote(where)+" <<'EOF'",
		byHandSnippet,
		"EOF",
		`ok `+shellQuote(strings.TrimPrefix(byHandAfter, "✓ ")),
	)
}

func (s *state) byHand() error { return s.byHandFor(byHandWhere) }

// asks writes the answer to a file, so the check sees exactly what the variable holds.
func (s *state) asks(name string, prompt string) error {
	return s.setBody(
		"ask "+name+" "+shellQuote(prompt),
		`printf '%s' "$`+name+`" > `+strings.ToLower(name)+`.txt`,
	)
}
