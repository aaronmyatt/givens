package openaproject

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/acceptance/world"
)

// testScreenSeam has the built givens, with no terminal, print the TUI's first screen and exit
// 0, in place of usage: the steps' stand-in for a terminal. World.Run sets GIVENS_TEST_SEAMS=1
// as well, which it needs. See testScreenEnv in cmd/givens/tui.go.
const testScreenSeam = "GIVENS_TEST_SCREEN=1 "

// tuiTabs is the TUI's tab line, which only its project page draws.
const tuiTabs = "[Capabilities]"

// terminalSteps run the built givens command, as a developer would type it.
func terminalSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has no folder at ~/code/nope$`, s.noFolder),
		world.Def(`^they run "([^"]*)" in a terminal in the sample shop's folder$`, s.runInShop),
		world.Def(`^they run "([^"]*)" in a terminal$`, s.runInTerminal),
		world.Def(`^the TUI opens on the sample shop's capabilities folder$`, s.tuiOnShop),
		world.Def(`^its list holds "([^"]*)", "([^"]*)" and "([^"]*)", folded$`, s.tuiLists),
		world.Def(`^the command prints "([^"]*)"$`, s.commandPrints),
		world.Def(`^the TUI doesn't open$`, s.tuiDidntOpen),
	}
}

// noFolder checks ~/code/nope really is missing on this Mac, as the scenario needs.
func (s *state) noFolder() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(home, "code", "nope")); err == nil {
		return errors.New("~/code/nope exists on this Mac, so the scenario can't use it")
	}
	return nil
}

// runInShop runs command from the sample shop's folder, with the test seam for a terminal.
func (s *state) runInShop(command string) error {
	if s.Project == "" {
		return errors.New("no step copied the sample shop")
	}
	s.Cwd = s.Project
	return s.Run(testScreenSeam + command)
}

// runInTerminal runs command, with the test seam for a terminal.
func (s *state) runInTerminal(command string) error {
	return s.Run(testScreenSeam + command)
}

// tuiOnShop expects givens to have drawn the TUI's project page, its top line the sample
// shop's capabilities folder, as the window's title is.
func (s *state) tuiOnShop() error {
	if s.ExitCode != 0 {
		return fmt.Errorf("givens exited %d:\n%s", s.ExitCode, s.Output)
	}
	top, _, _ := strings.Cut(s.Output, "\n")
	if !strings.Contains(top, tuiTabs) {
		return fmt.Errorf("givens didn't draw the TUI's project page; it printed:\n%s", s.Output)
	}
	if !containsPath(top, s.Capabilities()) {
		return fmt.Errorf("the TUI's top line is %q, not the sample shop's %s",
			top, s.Capabilities())
	}
	return nil
}

// tuiLists expects each capability as a folded heading, "+ Checkout (5)", in this order.
func (s *state) tuiLists(first string, second string, third string) error {
	from := 0
	for _, name := range []string{first, second, third} {
		at := strings.Index(s.Output[from:], "+ "+name+" (")
		if at < 0 {
			return fmt.Errorf("the TUI doesn't list %q folded after its earlier capabilities; "+
				"it printed:\n%s", name, s.Output)
		}
		from += at + 1
	}
	return nil
}

func (s *state) commandPrints(text string) error {
	lines := strings.Split(s.Output, "\n")
	if err := world.Expect(lines, text, "the command's output"); err != nil {
		return err
	}
	if s.ExitCode == 0 {
		return fmt.Errorf("the command printed %q but exited 0", text)
	}
	return nil
}

// tuiDidntOpen expects no TUI page in the output: with the seam on, one would have been
// printed had givens gone on to open a project.
func (s *state) tuiDidntOpen() error {
	if strings.Contains(s.Output, tuiTabs) {
		return fmt.Errorf("givens opened the TUI:\n%s", s.Output)
	}
	return nil
}

// noBranchCopy expects the sample shop's window alone: no copy of a branch opened beside it.
func (s *state) noBranchCopy() error {
	return expectWindows(s.App, []string{s.Capabilities()})
}
