package workintheterminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// checkoutFeature is the sample shop's checkout.feature, relative to the shop.
const checkoutFeature = "capabilities/checkout/checkout.feature"

// The edits the fake editor makes, as bash snippets on "$1", the file it's given.
const (
	cancelEdit = `sed -i '' 's/the order stays pending/the order is cancelled/' "$1"`
	// breakEdit puts a line that isn't a step on line 17, after Card declined's last step.
	breakEdit = `perl -pi -e 's/^(    And the order stays pending)$/$1\n    Check the order ` +
		`is pending/' "$1"`
)

func editSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer's editor is vim$`, func() error {
			return s.FakeEditorNamed("vim", cancelEdit)
		}),
		world.Def(`^vim opens checkout\.feature's text at the line of "Scenario: Card `+
			`declined"$`, func() error { return s.calledWith("+12 ", "checkout.feature") }),
		world.Def(`^once they save a change and quit, checkout\.feature holds it$`, func() error {
			return s.holds(checkoutFeature, "the order is cancelled")
		}),
		world.Def(`^a developer's editor adds a line to the file it opens$`, func() error {
			return s.FakeEditor(`echo 'ok "and a line more"' >> "$1"`)
		}),
		world.Def(`^their editor opens the script's text, in a file ending in \.sh$`,
			func() error { return s.calledWith("", "card-declined.sh") }),
		world.Def(`^once they save a change and quit, the script holds it$`, func() error {
			return s.holds("capabilities/checkout/scripts/card-declined.sh",
				`ok "and a line more"`)
		}),
		world.Def(`^a developer is editing "Card declined" with "givens edit"$`, s.remember),
		world.Def(`^they add the line "Check the order is pending" among its steps and quit$`,
			func() error {
				top := `head -1 "$1" > ` + filepath.Join(s.EditorDir(), "second-top")
				if err := s.FakeEditor(breakEdit, top); err != nil {
					return err
				}
				return s.Run("givens edit checkout/card-declined")
			}),
		world.Def(`^a developer's editor adds the line "([^"]*)" to the end of the file it `+
			`opens$`, s.appendsLine),
		world.Def(`^card-declined\.sh is left as it was until they fix it or quit without `+
			`saving$`, func() error {
			return s.unchanged("capabilities/checkout/scripts/card-declined.sh")
		}),
		world.Def(`^their editor opens again with "([^"]*)" at the top$`, s.reopenedWith),
		world.Def(`^checkout\.feature is left as it was until they fix it or quit without `+
			`saving$`, s.leftAsItWas),
		world.Def(`^a developer runs "givens edit checkout/card-declined" and quits without `+
			`saving$`, func() error {
			if err := s.remember(); err != nil {
				return err
			}
			if err := s.FakeEditor(); err != nil {
				return err
			}
			return s.Run("givens edit checkout/card-declined")
		}),
		world.Def(`^nothing is written$`, s.leftAsItWas),
		world.Def(`^another tool saves a change to checkout\.feature$`, func() error {
			other := `printf '\n' >> ` + filepath.Join(s.Project, checkoutFeature)
			return s.FakeEditor(cancelEdit + "; " + other)
		}),
		world.Def(`^they save and quit$`, func() error {
			return s.Run("givens edit checkout/card-declined")
		}),
		world.Def(`^their edit is kept in a file whose path it prints$`, s.editKept),
	}
}

// calledWith expects the editor to have been called once, its arguments starting with first
// and ending with last.
func (s *state) calledWith(first string, last string) error {
	calls, err := s.EditorCalls()
	if err != nil {
		return err
	}
	if len(calls) == 0 || !strings.HasPrefix(calls[0], first) ||
		!strings.HasSuffix(calls[0], last) {
		return fmt.Errorf("the editor was called %q; givens printed:\n%s", calls, s.Output)
	}
	return nil
}

// holds expects a file of the sample shop to hold text.
func (s *state) holds(file string, text string) error {
	content, err := s.ReadFile(file)
	if err != nil {
		return err
	}
	if !strings.Contains(content, text) {
		return fmt.Errorf("%s doesn't hold %q; givens printed:\n%s", file, text, s.Output)
	}
	return nil
}

// remember keeps checkout.feature's text, to see later that nothing was written.
func (s *state) remember() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutFeature)
	s.before = text
	return err
}

func (s *state) leftAsItWas() error {
	now, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	if now != s.before {
		return fmt.Errorf("checkout.feature changed; givens printed:\n%s", s.Output)
	}
	// An edit left with problems is kept in a temporary file, which the step tidies away.
	if _, kept, found := strings.Cut(strings.TrimSpace(s.Output), "Your edit is in "); found {
		return os.Remove(kept)
	}
	return nil
}

func (s *state) reopenedWith(top string) error {
	calls, err := s.EditorCalls()
	if err != nil {
		return err
	}
	if len(calls) != 2 {
		return fmt.Errorf("the editor opened %d times; givens printed:\n%s", len(calls),
			s.Output)
	}
	shown, err := os.ReadFile(filepath.Join(s.EditorDir(), "second-top"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(shown)) != top {
		return fmt.Errorf("the reopened file starts %q", shown)
	}
	return nil
}

// editKept expects the printed path to hold the developer's edit.
func (s *state) editKept() error {
	_, path, found := strings.Cut(strings.TrimSpace(s.Output), "Your edit is in ")
	if !found {
		return fmt.Errorf("no path was printed:\n%s", s.Output)
	}
	kept, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(kept), "the order is cancelled") {
		return fmt.Errorf("%s doesn't hold the edit", path)
	}
	return os.Remove(path)
}

func commandSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer runs "givens new story ([^"]*)" and fills in the template$`,
			func(name string) error {
				fill := `perl -pi -e 's/^As a: $/As a: shopper/; s/^I want: $/I want: to pay ` +
					`with a gift card/; s/^So that: $/So that: I can spend my gift/' "$1"`
				if err := s.FakeEditor(fill); err != nil {
					return err
				}
				return s.Run("givens new story " + name)
			}),
		world.Def(`^capabilities/gift-cards/gift-cards\.feature holds the story they wrote$`,
			func() error {
				return s.holds("capabilities/gift-cards/gift-cards.feature",
					"Feature: Gift cards\n  As a shopper\n  I want to pay with a gift card\n"+
						"  So that I can spend my gift")
			}),
		world.Def(`^a developer runs "givens new scenario ([^"]*)" and names it "([^"]*)"$`,
			func(capability string, name string) error {
				fill := `perl -pi -e 's/^Name: $/Name: ` + name + `/' "$1"`
				if err := s.FakeEditor(fill); err != nil {
					return err
				}
				return s.Run("givens new scenario " + capability)
			}),
		world.Def(`^checkout\.feature gets "([^"]*)" tagged @todo$`, func(line string) error {
			return s.holds(checkoutFeature, "  @todo\n  "+line)
		}),
		world.Def(`^checkout/scripts/([a-z0-9-]+\.sh) is created$`, func(script string) error {
			_, err := s.ReadFile("capabilities/checkout/scripts/" + script)
			return err
		}),
		world.Def(`^a developer runs "(givens rename [^ ]+ ".*")"$`, s.Run),
		world.Def(`^checkout\.feature has "([^"]*)"$`, func(line string) error {
			return s.holds(checkoutFeature, line)
		}),
		world.Def(`^its setup script is now checkout/scripts/([a-z0-9-]+\.sh)$`,
			func(script string) error {
				old := "capabilities/checkout/scripts/card-declined.sh"
				if _, err := s.ReadFile(old); err == nil {
					return errors.New("card-declined.sh is still there")
				}
				_, err := s.ReadFile("capabilities/checkout/scripts/" + script)
				return err
			}),
		world.Def(`^nothing is deleted unless they run it again with --yes$`, s.deletedOnYes),
		world.Def(`^"Card declined" is tagged @todo$`, func() error {
			return s.holds(checkoutFeature, "@todo\n  Scenario: Card declined")
		}),
		world.Def(`^the TUI flags "([^"]*)" with ⚑ in the list$`, func(name string) error {
			if err := s.openTUI(); err != nil {
				return err
			}
			if !s.model.MoveTo(name) {
				return fmt.Errorf("the TUI doesn't list %q", name)
			}
			return s.shownLine("⚑ "+name, "Never run")
		}),
		world.Def(`^"givens done checkout/card-declined" takes the tag off$`, func() error {
			if err := s.Run("givens done checkout/card-declined"); err != nil {
				return err
			}
			text, err := s.ReadFile(checkoutFeature)
			if err == nil && strings.Contains(text, "@todo\n  Scenario: Card declined") {
				return fmt.Errorf("the tag is still there; givens printed:\n%s", s.Output)
			}
			return err
		}),
	}
}

// panelSteps are the steps about helpers, history, search, trust, sync and no terminal.
func panelSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer runs "givens helpers", "givens history ([^"]*)" and "givens `+
			`search ([^"]*)"$`, s.panels),
		world.Def(`^the first lists seed_shopper with where it's defined and how many scripts `+
			`call it$`, func() error {
			return s.outputHas(0, `seed_shopper\s+_shared/users\.sh:\d+\s+called by \d+ scripts`)
		}),
		world.Def(`^the second lists the passing run, newest first$`, func() error {
			return s.outputHas(1, `^Passed`)
		}),
		world.Def(`^the third lists "([^"]*)"$`, func(name string) error {
			return s.outputHas(2, regexp.QuoteMeta(name))
		}),
		world.Def(`^a developer has never run anything from the sample shop$`, func() error {
			s.Untrusted = true
			return nil
		}),
		world.Def(`^they see "([^"]*)" with "t" to trust and "d" not to$`, func(text string) error {
			return s.sees(text, "t Trust", "d Don't trust")
		}),
		world.Def(`^"givens trust" trusts it from the command line$`, s.trusts),
		world.Def(`^the sample shop is a git repository with a remote, with a run recorded `+
			`while it couldn't be reached$`, s.unsharedRun),
		world.Def(`^the remote's refs/givens/runs holds the run$`, s.remoteHoldsRun),
		world.Def(`^a developer runs "([^"]*)" with its output piped$`, func(command string) error {
			return s.Run("set -o pipefail; " + command + " | cat")
		}),
		world.Def(`^it exits with (\d+)$`, func(code string) error {
			if fmt.Sprint(s.ExitCode) != code {
				return fmt.Errorf("it exited %d, not %s:\n%s", s.ExitCode, code, s.Output)
			}
			return nil
		}),
	}
}

func (s *state) deletedOnYes() error {
	script := "capabilities/checkout/scripts/pay-with-saved-card.sh"
	if err := s.holds(checkoutFeature, "Scenario: Pay with saved card"); err != nil {
		return err
	}
	if _, err := s.ReadFile(script); err != nil {
		return errors.New("the script went without --yes")
	}
	if err := s.Run("givens delete checkout/pay-with-saved-card --yes"); err != nil {
		return err
	}
	if text, _ := s.ReadFile(checkoutFeature); strings.Contains(text, "Pay with saved card") {
		return fmt.Errorf("--yes didn't delete it:\n%s", s.Output)
	}
	trashed := filepath.Join(s.Dir, "effects", "Trash", "pay-with-saved-card.sh")
	if _, err := os.Stat(trashed); err != nil {
		return fmt.Errorf("the script isn't in the Trash: %w", err)
	}
	return nil
}

// panels runs the three commands, keeping each one's output.
func (s *state) panels(key string, words string) error {
	for _, command := range []string{"givens helpers", "givens history " + key,
		"givens search " + words} {
		if err := s.Run(command); err != nil {
			return err
		}
		if s.ExitCode != 0 {
			return fmt.Errorf("%s exited %d:\n%s", command, s.ExitCode, s.Output)
		}
		s.outputs = append(s.outputs, s.Output)
	}
	return nil
}

// outputHas expects the nth command's output to have a line matching pattern.
func (s *state) outputHas(index int, pattern string) error {
	if index >= len(s.outputs) {
		return fmt.Errorf("only %d commands ran", len(s.outputs))
	}
	if !regexp.MustCompile(`(?m)` + pattern).MatchString(s.outputs[index]) {
		return fmt.Errorf("no line matches %q:\n%s", pattern, s.outputs[index])
	}
	return nil
}

func (s *state) trusts() error {
	if err := s.Run("givens trust"); err != nil {
		return err
	}
	trusted, err := os.ReadFile(filepath.Join(s.AppData, "trusted.txt"))
	if err != nil {
		return err
	}
	resolved, _ := filepath.EvalSymlinks(s.Capabilities())
	if !strings.Contains(string(trusted), s.Capabilities()) &&
		!strings.Contains(string(trusted), resolved) {
		return fmt.Errorf("trusted.txt holds %q; givens printed:\n%s", trusted, s.Output)
	}
	return nil
}

// git runs git in dir with the scenario's own config.
func git(dir string, args ...string) error {
	command := exec.Command("git", append([]string{"-c", "user.name=Sam", "-c",
		"user.email=sam@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return nil
}

// unsharedRun makes the shop a repository with a bare remote, and records a run while the
// remote is moved away, then moves it back.
func (s *state) unsharedRun() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	remote := filepath.Join(s.Dir, "remote.git")
	for _, args := range [][]string{{"init", "--quiet", "--initial-branch=main"},
		{"add", "."}, {"commit", "--quiet", "-m", "Sample shop"},
		{"init", "--quiet", "--bare", remote}, {"remote", "add", "origin", remote}} {
		if err := git(s.Project, args...); err != nil {
			return err
		}
	}
	if err := os.Rename(remote, remote+".away"); err != nil {
		return err
	}
	if err := s.RecordPassingRun("Card declined"); err != nil {
		return err
	}
	return os.Rename(remote+".away", remote)
}

func (s *state) remoteHoldsRun() error {
	command := exec.Command("git", "ls-tree", "-r", "--name-only", app.RunsRef)
	command.Dir = filepath.Join(s.Dir, "remote.git")
	names, err := command.Output()
	if err != nil {
		return fmt.Errorf("the remote has no %s: %v; givens printed:\n%s", app.RunsRef, err,
			s.Output)
	}
	if !strings.Contains(string(names), "runs/") {
		return fmt.Errorf("the remote's ref holds %q", names)
	}
	return nil
}

// appendsLine makes the developer's editor add line to the end of the file once, and keep
// the first line of each file it opens, so a step can read what it reopened with.
func (s *state) appendsLine(line string) error {
	top := `head -1 "$1" > ` + filepath.Join(s.EditorDir(), "second-top")
	add := `grep -qxF '` + line + `' "$1" || echo '` + line + `' >> "$1"`
	return s.FakeEditor(add, top)
}

// unchanged expects a file of the sample shop to be as the sample shop has it.
func (s *state) unchanged(file string) error {
	shown, err := s.ReadFile(file)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(filepath.Join(s.Root, "testdata", "sample-shop", file))
	if err != nil {
		return err
	}
	if shown != string(original) {
		return fmt.Errorf("%s changed:\n%s", file, shown)
	}
	return nil
}
