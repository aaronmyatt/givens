package dropintoascenario

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

const (
	payWithSavedCard   = "Pay with saved card"
	threeDSChallenge   = "3-D Secure challenge"
	terminalRunTimeout = 20 * time.Second // a kit that lets a from loop run forever fails here
)

// hasSetupPrinting gives the named scenario a setup that prints line, and gives 3-D Secure
// challenge a setup that prints its own line, for a step to put a from line in front of.
func (s *state) hasSetupPrinting(name string, line string) error {
	message, found := strings.CutPrefix(line, "✓ ")
	if !found {
		return fmt.Errorf("%q isn't a line the kit's ok prints", line)
	}
	s.fromSetupLine = line
	if err := s.writeSetup(name, script("ok "+quote(message))); err != nil {
		return err
	}
	if err := s.writeSetup(threeDSChallenge, script(`ok "challenge mocked"`)); err != nil {
		return err
	}
	return s.prepare()
}

// fromFollowedBySetup expects the from line, then the setup it names, then the rest of this
// scenario's own setup.
func (s *state) fromFollowedBySetup(line string) error {
	return s.inOrder(line, s.fromSetupLine, "✓ challenge mocked")
}

func (s *state) checklistOnlyOf(name string) error {
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return err
	}
	whens, thens, err := s.expectedChecks(name)
	if err != nil {
		return err
	}
	return expectKinds(session.Checklist(), map[string][]string{"when": whens, "then": thens})
}

// bothHaveTeardowns gives Pay with saved card and 3-D Secure challenge a teardown and a setup
// each printing one line named after its scenario.
func (s *state) bothHaveTeardowns() error {
	for _, name := range []string{payWithSavedCard, threeDSChallenge} {
		teardown := script(`ok "` + app.SlugOf(name) + ` torn down"`)
		if err := s.writeTeardown(name, teardown); err != nil {
			return err
		}
	}
	if err := s.writeSetup(payWithSavedCard, script(`ok "saved card added"`)); err != nil {
		return err
	}
	err := s.writeSetup(threeDSChallenge, scriptFrom(
		"from checkout/pay-with-saved-card", `ok "challenge mocked"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

// fromOrder finds each line in the Drop in's output and returns their indexes.
func (s *state) fromOrder(lines ...string) ([]int, []string, error) {
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return nil, nil, err
	}
	output := session.Output()
	found := make([]int, 0, len(lines))
	for _, line := range lines {
		at := indexOf(output, line, 0)
		if at < 0 {
			return nil, nil, fmt.Errorf("the output has no %q: %q", line, output)
		}
		found = append(found, at)
	}
	return found, output, nil
}

// inOrder fails unless the lines appear in the output in the order given.
func (s *state) inOrder(lines ...string) error {
	found, output, err := s.fromOrder(lines...)
	if err != nil {
		return err
	}
	for index := 1; index < len(found); index++ {
		if found[index] <= found[index-1] {
			return fmt.Errorf("%q comes before %q: %q", lines[index], lines[index-1], output)
		}
	}
	return nil
}

func (s *state) ownTeardownFirst() error {
	return s.inOrder("→ teardown", "✓ 3-d-secure-challenge torn down",
		"→ from checkout/pay-with-saved-card")
}

func (s *state) fromThenTeardownAndSetup(line string) error {
	return s.inOrder("✓ 3-d-secure-challenge torn down", line,
		"✓ pay-with-saved-card torn down", "✓ saved card added")
}

func (s *state) restOfSetup() error {
	return s.inOrder("✓ saved card added", "✓ challenge mocked")
}

// startsWith makes line the named scenario's setup, followed by a line that leaves a marker.
func (s *state) startsWith(name string, line string) error {
	marker := "ran-" + app.SlugOf(name)
	s.markers = append(s.markers, marker)
	if err := s.writeSetup(name, scriptFrom(line, touch(marker))); err != nil {
		return err
	}
	return s.prepare()
}

// sameLoop writes the loop from "Setups that build on each other in a loop", without Givens.
func (s *state) sameLoop() error {
	loop := map[string]string{
		payWithSavedCard: "from checkout/3-d-secure-challenge",
		threeDSChallenge: "from checkout/pay-with-saved-card",
	}
	for name, line := range loop {
		if err := s.writeSetup(name,
			scriptFrom(line, `ok "`+app.SlugOf(name)+` ran"`)); err != nil {
			return err
		}
	}
	return nil
}

// runFromRoot runs command with bash from the sample shop's root, within a time limit, so a
// loop the kit doesn't catch fails the step instead of hanging the suite.
// Ref: https://pkg.go.dev/os/exec#CommandContext
func (s *state) runFromRoot(command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), terminalRunTimeout)
	defer cancel()
	run := exec.CommandContext(ctx, "bash", "-c", command)
	run.Dir = s.Project
	run.Env = os.Environ()
	output, err := run.CombinedOutput()
	s.Output = string(output)
	if ctx.Err() != nil {
		return fmt.Errorf("%q was still running after %s:\n%s", command, terminalRunTimeout,
			output)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		s.ExitCode = exit.ExitCode()
		return nil
	}
	s.ExitCode = 0
	return err
}

func (s *state) kitPrints(text string) error {
	return world.Expect(strings.Split(s.Output, "\n"), text, "the script's output")
}

func (s *state) nonZeroExit() error {
	if s.ExitCode == 0 {
		return fmt.Errorf("the script exited 0:\n%s", s.Output)
	}
	return nil
}

func (s *state) outputShows(text string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	return world.Expect(session.Output(), text, "the setup's output")
}
