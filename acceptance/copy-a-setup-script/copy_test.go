// Package copyasetupscript runs capabilities/copy-a-setup-script. Steps shared with other
// capabilities are in acceptance/world.
package copyasetupscript

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"givens/acceptance/world"
)

const capability = "copy-a-setup-script"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	capabilitiesDir string // where the sample shop's capabilities folder is, if moved
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^running it from the (?:project's|repo's) root folder sets up the scenario$`,
			s.runClipboardFromRoot),
		world.Def(`^they press ⌘K, type "([^"]*)" and press ⇧⌘C$`, s.copyFromSearch),
		world.Def(`^"([^"]*)" is still selected$`, s.stillSelected),
		world.Def(`^the sample shop's capabilities folder is at docs/capabilities in its git repo$`,
			s.capabilitiesInDocs),
		world.Def(`^a developer chooses "([^"]*)" on "([^"]*)"$`, s.chooseOn),
		world.Def(`^a developer's terminal is in the sample shop's src folder$`, s.terminalInSrc),
		world.Def(`^setup runs the same as it would from the root folder$`, s.sameAsFromRoot),
		world.Def(`^the clipboard holds the markdown of ([a-z0-9-]+\.md)$`,
			s.clipboardHoldsMarkdown),
	}
}

// runClipboardFromRoot pastes the clipboard into a shell at the project's root, as a developer
// would, and expects setup to succeed.
func (s *state) runClipboardFromRoot() error {
	command, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if command == "" {
		return errors.New("the clipboard is empty")
	}
	s.Cwd = s.Project
	if err := s.Run(command); err != nil {
		return err
	}
	if s.ExitCode != 0 {
		return fmt.Errorf("%q exited %d:\n%s", command, s.ExitCode, s.Output)
	}
	return nil
}

func (s *state) copyFromSearch(query string) error {
	if err := s.Search(query); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Press("⇧⌘C")
}

func (s *state) stillSelected(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if got := window.Selected(); got != name {
		return fmt.Errorf("%q is selected, not %q", got, name)
	}
	return nil
}

// capabilitiesInDocs moves the sample shop's capabilities folder to docs/capabilities and makes
// the shop a git repo, so the repo root and the capabilities folder's parent differ.
func (s *state) capabilitiesInDocs() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	docs := filepath.Join(s.Project, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		return err
	}
	s.capabilitiesDir = filepath.Join(docs, "capabilities")
	if err := os.Rename(s.Capabilities(), s.capabilitiesDir); err != nil {
		return err
	}
	// git init makes the shop a repository. Ref: https://git-scm.com/docs/git-init
	init := exec.Command("git", "init", "--quiet")
	init.Dir = s.Project
	if output, err := init.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %v\n%s", err, output)
	}
	return nil
}

// chooseOn selects a scenario, then takes an action on it, opening the shop first if needed.
func (s *state) chooseOn(action string, scenario string) error {
	if s.Window == nil {
		if err := s.StartApp(); err != nil {
			return err
		}
		dir := s.capabilitiesDir
		if dir == "" {
			dir = s.Capabilities()
		}
		window, err := s.App.Open(dir)
		if err != nil {
			return err
		}
		s.Window = window
	}
	if err := s.Window.Select(scenario); err != nil {
		return err
	}
	return s.Window.Choose(action)
}

func (s *state) terminalInSrc() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	s.Cwd = filepath.Join(s.Project, "src")
	return os.MkdirAll(s.Cwd, 0o755)
}

// sameAsFromRoot runs the last command again from the root and compares the two runs.
func (s *state) sameAsFromRoot() error {
	fromSubfolder, exitCode := s.Output, s.ExitCode
	if exitCode != 0 {
		return fmt.Errorf("setup exited %d from the subfolder:\n%s", exitCode, fromSubfolder)
	}
	s.Cwd = s.Project
	if err := s.Run("bash capabilities/checkout/scripts/card-declined.sh"); err != nil {
		return err
	}
	if s.ExitCode != 0 || s.Output != fromSubfolder {
		return fmt.Errorf("from the root it exited %d with:\n%s\nfrom src:\n%s",
			s.ExitCode, s.Output, fromSubfolder)
	}
	return nil
}

func (s *state) clipboardHoldsMarkdown(name string) error {
	want, err := s.ReadFile(filepath.Join("capabilities", "checkout", "scripts", name))
	if err != nil {
		return err
	}
	got, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("the clipboard holds %q, not the markdown of %s", got, name)
	}
	return nil
}
