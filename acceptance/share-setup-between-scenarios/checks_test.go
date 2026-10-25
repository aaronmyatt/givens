package sharesetupbetweenscenarios

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/app/model"
)

// bashDurationMax bounds the bash runs these checks make to read a helper file.
const bashDurationMax = 10 * time.Second

func (s *state) setupFailsWith(message string) error {
	if s.Session == nil {
		return errors.New("no Drop in has started")
	}
	if status := s.Session.Status(); status != app.SessionFailed {
		return fmt.Errorf("expected setup to fail, but the Drop in is %q", status)
	}
	return world.Expect(s.Session.Output(), message, "the setup's output")
}

// pointsOut looks for one visible line naming both the helper and the file defining it.
func (s *state) pointsOut(helper string, file string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	for _, line := range window.Visible() {
		if strings.Contains(line, helper) && strings.Contains(line, file) {
			return nil
		}
	}
	return fmt.Errorf("expected a visible line naming %s and %s, got %q",
		helper, file, window.Visible())
}

func (s *state) overrideRan() error {
	if s.Session == nil {
		return errors.New("no Drop in has started")
	}
	if status := s.Session.Status(); status != app.SessionReady {
		return fmt.Errorf("expected setup to finish, but the Drop in is %q", status)
	}
	output := s.Session.Output()
	if err := world.Expect(output, "✓ seeded Checkout's own shopper", "the output"); err != nil {
		return err
	}
	return world.ExpectNot(output, "with a saved card", "the output")
}

func (s *state) noWarning() error {
	if s.Session == nil {
		return errors.New("no Drop in has started")
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	for _, line := range s.Session.Output() {
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			return fmt.Errorf("expected no warning in the output, got %q", line)
		}
	}
	return world.ExpectNot(window.Visible(), "defined in both", "the window")
}

func (s *state) marksOverride(mark string) error {
	helpers, err := s.helpersNamed("seed_shopper")
	if err != nil {
		return err
	}
	for _, helper := range helpers {
		if helper.DefinedIn == "checkout/_shared/shopper.sh" && helper.Overrides == mark {
			return nil
		}
	}
	return fmt.Errorf("expected checkout/_shared/shopper.sh's seed_shopper marked %q, got %+v",
		mark, helpers)
}

func (s *state) outputWarns(warning string) error {
	if s.Session == nil {
		return errors.New("no Drop in has started")
	}
	return world.Expect(s.Session.Output(), warning, "the setup's output")
}

func (s *state) bothFlagged() error {
	helpers, err := s.helpersNamed("seed_shopper")
	if err != nil {
		return err
	}
	flagged := make([]string, 0, len(helpers))
	for _, helper := range helpers {
		if helper.Warning != "" {
			flagged = append(flagged, helper.DefinedIn)
		}
	}
	slices.Sort(flagged)
	want := []string{"_shared/people.sh", "_shared/users.sh"}
	if !slices.Equal(flagged, want) {
		return fmt.Errorf("expected %q flagged in the Helpers list, got %q", want, flagged)
	}
	return nil
}

func (s *state) stillVerified(name string) error {
	scenario, err := s.scenario(name)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(scenario.Status, "Verified") {
		return fmt.Errorf("expected %q to show as verified, got %q", name, scenario.Status)
	}
	return nil
}

func (s *state) bothShow(text string) error {
	if len(s.pair) != 2 {
		return errors.New("no step named the two scenarios")
	}
	for _, name := range s.pair {
		scenario, err := s.scenario(name)
		if err != nil {
			return err
		}
		got := []string{scenario.Status, scenario.Warning}
		if err := world.Expect(got, text, name+"'s status"); err != nil {
			return err
		}
	}
	return nil
}

// markedIfMissing checks what Givens says about command against this machine: marked "Not
// installed" with its hint when it's missing, and not marked when it's on PATH.
// Ref: https://pkg.go.dev/os/exec#LookPath
func (s *state) markedIfMissing(command string, mark string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	required, err := window.RequiredCommands(window.Selected())
	if err != nil {
		return err
	}
	at := slices.IndexFunc(required, func(r model.RequiredCommand) bool {
		return r.Command == command
	})
	if at < 0 {
		return fmt.Errorf("expected %s among the commands required, got %+v", command, required)
	}
	_, lookErr := exec.LookPath(command)
	installed := lookErr == nil
	if got := required[at]; got.Installed != installed || got.Hint != s.hint {
		return fmt.Errorf("expected %s installed=%t with hint %q, got %+v",
			command, installed, s.hint, got)
	}
	if installed {
		return world.ExpectNot(window.Visible(), mark, "the window")
	}
	if err := world.Expect(window.Visible(), mark, "the window"); err != nil {
		return err
	}
	return world.Expect(window.Visible(), s.hint, "the window")
}

func (s *state) opensAt(file string, helper string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	want := filepath.Join(s.Capabilities(), file)
	edit := window.EditingHelper()
	if edit.Path != want || window.Editing() != want {
		return fmt.Errorf("expected %s in the editor, got %q (helper %q)",
			want, window.Editing(), edit.Path)
	}
	line, err := s.definitionLine(file, helper)
	if err != nil {
		return err
	}
	if edit.Line != line {
		return fmt.Errorf("expected the editor at line %d, %s, got line %d",
			line, helper, edit.Line)
	}
	return nil
}

// callersListed compares the callers Givens lists with the helper being edited with the
// sample shop's scripts that call it.
func (s *state) callersListed(count string) error {
	want, err := s.callersOf(s.helper)
	if err != nil {
		return err
	}
	if strconv.Itoa(len(want)) != count {
		return fmt.Errorf("the sample shop has %d scripts calling %s, not %s: %q",
			len(want), s.helper, count, want)
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	got := slices.Clone(window.EditingHelper().Callers)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return fmt.Errorf("expected %q listed with the helper, got %q", want, got)
	}
	return nil
}

// helperCreated checks the new file has a `# @helper` comment and a function that bash parses
// and that does nothing when called.
func (s *state) helperCreated(file string, helper string) error {
	text, err := s.ReadFile(filepath.Join("capabilities", file))
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`(?m)^# @helper\b`).MatchString(text) {
		return fmt.Errorf("expected a # @helper comment in %s, got:\n%s", file, text)
	}
	if _, err := s.definitionLine(file, helper); err != nil {
		return err
	}
	path := filepath.Join(s.Capabilities(), file)
	output, err := bash(`source "$1" && `+helper, path)
	if err != nil || output != "" {
		return fmt.Errorf("expected %s to load and do nothing, got %v and %q", helper, err, output)
	}
	return nil
}

func (s *state) listedUnder(helper string, capabilityName string) error {
	helpers, err := s.helpersNamed(helper)
	if err != nil {
		return err
	}
	want := filepath.Join(strings.ToLower(capabilityName), "_shared", helper+".sh")
	for _, listed := range helpers {
		if listed.Capability == capabilityName && listed.DefinedIn == want {
			return nil
		}
	}
	return fmt.Errorf("expected %s listed as %s's, in %s; got %+v",
		helper, capabilityName, want, helpers)
}

// syntaxMarked runs `bash -n` on the edited text itself, so the expected line is bash's own.
// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin
func (s *state) syntaxMarked() error {
	copied := filepath.Join(s.Dir, "edited-helper.sh")
	if err := os.WriteFile(copied, []byte(s.EditedText), 0o644); err != nil {
		return err
	}
	report, err := bash(`bash -n "$1"`, copied)
	found := regexp.MustCompile(`line (\d+): (syntax error.*)`).FindStringSubmatch(report)
	if err == nil || found == nil {
		return fmt.Errorf("expected bash to report a syntax error in the edit, got %q", report)
	}
	line, err := strconv.Atoi(found[1])
	if err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	problems := window.EditingHelper().Problems
	for _, problem := range problems {
		if problem.Line == line && strings.Contains(problem.Message, "syntax error") {
			return nil
		}
	}
	return fmt.Errorf("expected line %d marked %q, got %+v", line, found[2], problems)
}

// bash runs script with bash -c and one argument, bounded by bashDurationMax.
// Ref: https://pkg.go.dev/os/exec#CommandContext
func bash(script string, argument string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bashDurationMax)
	defer cancel()
	output, err := exec.CommandContext(ctx, "bash", "-c", script, "bash", argument).
		CombinedOutput()
	return string(output), err
}
