package thekit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/acceptance/world"
)

// firstRunPrintsCommand checks the first run of the When mode script printed the command that
// reruns it with --when, ran its setup, and didn't run its When part.
func (s *state) firstRunPrintsCommand() error {
	first := lines(s.firstOutput)
	want := "When you run this: bash " + scriptPath + " --when"
	if err := world.Expect(first, want, "the first run's output"); err != nil {
		return err
	}
	if err := world.Expect(first, "✓ seeded shopper", "the first run's output"); err != nil {
		return err
	}
	if err := world.ExpectNot(first, "placed the order", "the first run's output"); err != nil {
		return err
	}
	if s.firstExit != 0 {
		return fmt.Errorf("expected the first run to exit 0, got %d", s.firstExit)
	}
	return nil
}

func (s *state) secondRunWhenOnly() error {
	second := lines(s.Output)
	if err := world.Expect(second, "✓ placed the order", "the second run's output"); err != nil {
		return err
	}
	for _, setupLine := range []string{"seeded shopper", "When you run this"} {
		if err := world.ExpectNot(second, setupLine, "the second run's output"); err != nil {
			return err
		}
	}
	return s.exitsZero()
}

// printsSnippet checks the heading is followed, after any blank lines, by the snippet.
func (s *state) printsSnippet(heading string) error {
	lines := lines(s.Output)
	at := indexContaining(lines, heading)
	if at < 0 {
		return fmt.Errorf("expected a line %q, got:\n%s", heading, s.Output)
	}
	for _, line := range lines[at+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.Contains(line, byHandSnippet) {
			return fmt.Errorf("expected %q after %q, got %q", byHandSnippet, heading, line)
		}
		return nil
	}
	return fmt.Errorf("expected %q after %q, got nothing", byHandSnippet, heading)
}

func (s *state) waitsWith(prompt string) error {
	if err := world.Expect(lines(s.Output), prompt, "the output"); err != nil {
		return err
	}
	if !s.waited {
		return fmt.Errorf("expected the script to wait for Enter after %q, but it didn't wait",
			prompt)
	}
	return nil
}

func (s *state) carriesOn() error {
	if !s.waited {
		return errors.New("expected the script to wait for Enter, but it didn't wait")
	}
	if err := world.Expect(lines(s.Output), byHandAfter, "the output"); err != nil {
		return err
	}
	return s.exitsZero()
}

// seesVariable reads the file the ask script wrote the variable's value to.
func (s *state) seesVariable(name string, want string) error {
	got, err := s.ReadFile(strings.ToLower(name) + ".txt")
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the script stopped before using %s; it printed:\n%s", name, s.Output)
	}
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("expected the script to see %s as %q, got %q", name, want, got)
	}
	return nil
}

// notCreated checks a file a later line of the script would have made doesn't exist.
func (s *state) notCreated(name string) error {
	_, err := os.Stat(filepath.Join(s.Project, name))
	if err == nil {
		return fmt.Errorf("expected the script to stop before it made %s, but it made it", name)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func lines(output string) []string { return strings.Split(output, "\n") }

// indexContaining is the index of the first line containing text, or -1.
func indexContaining(lines []string, text string) int {
	for index, line := range lines {
		if strings.Contains(line, text) {
			return index
		}
	}
	return -1
}
