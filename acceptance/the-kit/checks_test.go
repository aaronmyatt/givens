package thekit

import (
	"fmt"
	"strings"

	"givens/acceptance/world"
)

const outputFile = "output.txt"

// runCommand runs the setup script from the sample shop's root folder, as a developer would.
func runCommand() string { return "bash " + scriptPath }

func (s *state) runInTerminal() error {
	return s.run(runRequest{command: runCommand(), terminal: true})
}

// runInTerminalPressingEnter presses Enter if the script waits; a script that doesn't wait
// never reads it.
func (s *state) runInTerminalPressingEnter() error {
	return s.run(runRequest{command: runCommand(), terminal: true, answers: []string{""}})
}

func (s *state) runTyping(text string) error {
	return s.run(runRequest{command: runCommand(), terminal: true, answers: []string{text}})
}

func (s *state) typeF() error {
	return s.run(runRequest{command: runCommand(), terminal: true, answers: []string{"f"}})
}

// runPlain runs with no terminal and no GIVENS_ variables: stdin is /dev/null and there's no
// /dev/tty, as in CI.
func (s *state) runPlain() error { return s.run(runRequest{command: runCommand()}) }

func (s *state) runQuiet() error {
	return s.run(runRequest{command: runCommand(), env: []string{"GIVENS_QUIET=1"}})
}

// runToFile runs in a terminal with only the output going to a file, so a kit that checks the
// wrong stream for a terminal colours the file.
func (s *state) runToFile() error {
	return s.run(runRequest{
		command: runCommand() + " > " + outputFile + " 2>&1", terminal: true,
	})
}

// runTwice runs the script, then runs the command its "When you run this:" line prints.
func (s *state) runTwice() error {
	if err := s.run(runRequest{command: runCommand()}); err != nil {
		return err
	}
	s.firstOutput, s.firstExit = s.Output, s.ExitCode
	const prefix = "When you run this: "
	command := ""
	for _, line := range strings.Split(s.firstOutput, "\n") {
		if _, after, found := strings.Cut(line, prefix); found {
			command = strings.TrimSpace(after)
		}
	}
	if !strings.HasPrefix(command, "bash ") {
		return fmt.Errorf("the first run printed no %q line with a bash command; it printed:\n%s",
			prefix, s.firstOutput)
	}
	return s.run(runRequest{command: command})
}

func (s *state) stopsAtCommand() error {
	lines := strings.Split(s.Output, "\n")
	seeded := indexContaining(lines, "✓ seeded shopper")
	failed := indexContaining(lines, "missing-folder")
	if seeded < 0 || failed < seeded {
		return fmt.Errorf("expected \"✓ seeded shopper\" then ls's error about missing-folder; "+
			"got:\n%s", s.Output)
	}
	return nil
}

func (s *state) linesAfterDontRun() error {
	if err := world.ExpectNot(lines(s.Output), "seeded the catalogue", "the output"); err != nil {
		return err
	}
	return s.notCreated("ran-after-failure")
}

func (s *state) exitsNonZero() error {
	if s.ExitCode == 0 {
		return fmt.Errorf("expected a non-zero exit code, got 0; the output was:\n%s", s.Output)
	}
	return nil
}

func (s *state) exitsZero() error {
	if s.ExitCode != 0 {
		return fmt.Errorf("expected exit code 0, got %d; the output was:\n%s",
			s.ExitCode, s.Output)
	}
	return nil
}

// yourTurnOnce checks there's one "Your turn" line, and only blank lines between it and the
// first_you line.
func (s *state) yourTurnOnce() error {
	lines := lines(s.Output)
	headings := 0
	for _, line := range lines {
		if strings.Contains(line, "Your turn") {
			headings++
		}
	}
	if headings != 1 {
		return fmt.Errorf("expected \"Your turn\" once, got %d times in:\n%s", headings, s.Output)
	}
	heading := indexContaining(lines, "Your turn")
	first := indexContaining(lines, `First, select "Checkout"`)
	if first < heading {
		return fmt.Errorf("expected \"Your turn\" before `First, select \"Checkout\"`, got:\n%s",
			s.Output)
	}
	for _, between := range lines[heading+1 : first] {
		if strings.TrimSpace(between) != "" {
			return fmt.Errorf("expected \"Your turn\" just before the first_you line, but %q "+
				"is between them", between)
		}
	}
	return nil
}

func (s *state) noYourTurnLines() error {
	unwanted := []string{"Your turn", "First, ", "When you ", "Then check that "}
	for _, text := range unwanted {
		if err := world.ExpectNot(lines(s.Output), text, "quiet output"); err != nil {
			return err
		}
	}
	return nil
}

func (s *state) statusLinesPrinted() error {
	wanted := []string{"✓ seeded shopper", "! Mailpit isn't running", "✗ No database found"}
	for _, text := range wanted {
		if err := world.Expect(lines(s.Output), text, "quiet output"); err != nil {
			return err
		}
	}
	return nil
}

// fileHoldsPlain reads the file as bash wrote it, so any colour code would still be in it.
func (s *state) fileHoldsPlain(want string) error {
	written, err := s.ReadFile(outputFile)
	if err != nil {
		return err
	}
	if strings.Contains(written, "\x1b") {
		return fmt.Errorf("expected no colour codes in the file, got %q", written)
	}
	for _, line := range strings.Split(written, "\n") {
		if line == want {
			return nil
		}
	}
	return fmt.Errorf("expected a line %q in the file, got %q", want, written)
}

func (s *state) nothingAfterNeed() error {
	if err := world.ExpectNot(lines(s.Output), "set up the shop", "the output"); err != nil {
		return err
	}
	return s.notCreated("ran-after-need")
}

func (s *state) bothLinesPrinted() error {
	lines := lines(s.Output)
	warned := indexContaining(lines, "! Mailpit isn't running")
	seeded := indexContaining(lines, "✓ seeded shopper")
	if warned < 0 || seeded < warned {
		return fmt.Errorf("expected \"! Mailpit isn't running\" then \"✓ seeded shopper\", "+
			"got:\n%s", s.Output)
	}
	return nil
}
