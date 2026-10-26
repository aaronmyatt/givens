// Package usegivensfromtheterminal runs capabilities/use-givens-from-the-terminal. Steps shared
// with other capabilities are in acceptance/world.
package usegivensfromtheterminal

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
)

const capability = "use-givens-from-the-terminal"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	walked    string   // the output of a setup walked through with Enter, once per scenario
	codeBlock []string // the code block a Given step added to a markdown setup
}

// listed is one scenario in `givens list --json`. encoding/json matches keys without regard
// to case, so "path" and "Path" both fill Path. Ref: https://pkg.go.dev/encoding/json#Unmarshal
type listed struct {
	Path   string
	Kind   string
	Status string
}

// listPath is a run key at the start of a `givens list` line, like "checkout/card-declined".
const listPath = `^[\p{L}\p{N}-]+/[\p{L}\p{N}-]+$`

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^a developer's terminal is in the sample shop's folder$`, s.terminalInShop),
		world.Def(`^a developer's terminal is anywhere inside the sample shop's folder$`,
			s.terminalInsideShop),
		world.Def(
			`^each scenario is printed as its path, like "([^"]*)", with its kind and status$`,
			s.eachScenarioPrinted),
		world.Def(`^"givens list --json" prints the same as JSON$`, s.sameAsJSON),
		world.Def(`^the setup runs, with its "your turn" lines$`, s.setupRunsWithYourTurn),
		world.Def(`^every file in the sample shop is valid$`, s.everyFileValid),
		world.Def(`^it exits with (\d+)$`, s.exitsWith),
		world.Def(`^it prints the same scenarios the Error states view lists$`,
			s.sameAsErrorStates),
		world.Def(`^scripts/pay-in-app\.md has a fenced code block indented under step (\d+)$`,
			s.codeBlockUnderStep),
		world.Def(`^it prints each step as plain text$`, s.stepsAsPlainText),
		world.Def(`^it waits for Enter after each one, or f if it failed$`, s.waitsForEnter),
		world.Def(`^each code block is printed on its own lines, with no fences and no indent$`,
			s.codeBlockOnOwnLines),
		world.Def(`^it can be selected and pasted as it is$`, s.codeBlockPastable),
		world.Def(`^the sample shop has no \.claude/skills/givens/SKILL\.md$`, s.noSkill),
		world.Def(`^\.claude/skills/givens/SKILL\.md is written, a skill named "([^"]*)"$`,
			s.skillWritten),
		world.Def(`^the sample shop's CLAUDE\.md has a paragraph of its own$`, s.claudeHasOwn),
		world.Def(`^CLAUDE\.md has a "([^"]*)" paragraph after its own$`, s.claudeHasParagraph),
		world.Def(`^running "([^"]*)" again changes nothing$`, s.againChangesNothing),
	}
}

func (s *state) terminalInShop() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	s.Cwd = s.Project
	return nil
}

// terminalInsideShop starts the terminal two folders below the shop's root, where givens has
// to find the capabilities folder by looking up.
func (s *state) terminalInsideShop() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	s.Cwd = filepath.Join(s.Project, "src", "components")
	return os.MkdirAll(s.Cwd, 0o755)
}

// eachScenarioPrinted expects one line per scenario in the shop's .feature files: its path
// first, then its kind ("happy path" or "error state") and its status ("Never run": the copy
// has no runs).
func (s *state) eachScenarioPrinted(example string) error {
	if s.ExitCode != 0 {
		return fmt.Errorf("givens list exited %d:\n%s", s.ExitCode, s.Output)
	}
	want, err := s.shopScenarios()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(want, func(item listed) bool { return item.Path == example }) {
		return fmt.Errorf("the sample shop has no scenario %s to print", example)
	}
	lines := pathLines(s.Output)
	if len(lines) != len(want) {
		return fmt.Errorf("givens list printed %d scenarios, not %d:\n%s",
			len(lines), len(want), s.Output)
	}
	for _, item := range want {
		line, ok := lines[item.Path]
		if !ok {
			return fmt.Errorf("givens list didn't print %s:\n%s", item.Path, s.Output)
		}
		if !strings.Contains(line, item.Kind) || !strings.Contains(line, item.Status) {
			return fmt.Errorf("givens list printed %q for %s, without %q and %q",
				line, item.Path, item.Kind, item.Status)
		}
	}
	return nil
}

// sameAsJSON runs `givens list --json` and expects the scenarios the plain list printed, each
// with the kind and status printed on its plain line.
func (s *state) sameAsJSON() error {
	plain := pathLines(s.Output)
	if err := s.Run("givens list --json"); err != nil {
		return err
	}
	if s.ExitCode != 0 {
		return fmt.Errorf("givens list --json exited %d:\n%s", s.ExitCode, s.Output)
	}
	var items []listed
	if err := json.Unmarshal([]byte(s.Output), &items); err != nil {
		return fmt.Errorf("givens list --json didn't print a JSON list: %w\n%s", err, s.Output)
	}
	if len(items) != len(plain) || len(items) == 0 {
		return fmt.Errorf("the JSON has %d scenarios and the plain list %d", len(items),
			len(plain))
	}
	for _, item := range items {
		line, ok := plain[item.Path]
		if !ok || item.Kind == "" || item.Status == "" {
			return fmt.Errorf("the JSON's %+v isn't a scenario the plain list printed", item)
		}
		if !strings.Contains(line, item.Kind) || !strings.Contains(line, item.Status) {
			return fmt.Errorf("the JSON says %+v; the plain list says %q", item, line)
		}
	}
	return nil
}

// setupRunsWithYourTurn expects card-declined.sh to finish, printing the "Your turn" heading
// and the line each first_you, when_you and then_check call prints (see the kit's about.md).
func (s *state) setupRunsWithYourTurn() error {
	if s.ExitCode != 0 {
		return fmt.Errorf("the setup exited %d:\n%s", s.ExitCode, s.Output)
	}
	script, err := s.ReadFile("capabilities/checkout/scripts/card-declined.sh")
	if err != nil {
		return err
	}
	want := yourTurnLines(script)
	if len(want) == 0 {
		return fmt.Errorf("card-declined.sh calls no first_you, when_you or then_check")
	}
	lines := strings.Split(s.Output, "\n")
	for _, line := range append([]string{"Your turn"}, want...) {
		if err := world.Expect(lines, line, "the setup's output"); err != nil {
			return err
		}
	}
	return nil
}

// everyFileValid checks the copied shop with the same checker givens check uses, so the
// scenario starts from a shop with nothing to report.
func (s *state) everyFileValid() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	problems, err := app.CheckCapabilities(s.Capabilities())
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("the sample shop isn't valid to start with: %v", problems)
	}
	s.Cwd = s.Project
	return nil
}

func (s *state) exitsWith(code int) error {
	if s.ExitCode != code {
		return fmt.Errorf("it exited %d, not %d:\n%s", s.ExitCode, code, s.Output)
	}
	return nil
}

// sameAsErrorStates compares the paths givens list printed with the Error states view.
func (s *state) sameAsErrorStates() error {
	if s.ExitCode != 0 {
		return fmt.Errorf("givens list exited %d:\n%s", s.ExitCode, s.Output)
	}
	printed := slices.Sorted(maps.Keys(pathLines(s.Output)))
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	if err := s.Window.Select("Error states"); err != nil {
		return err
	}
	view := make([]string, 0, len(printed))
	for _, scenario := range s.Window.ListedScenarios() {
		view = append(view, app.SlugOf(scenario.Capability)+"/"+scenario.Slug)
	}
	slices.Sort(view)
	if len(view) == 0 || !slices.Equal(printed, view) {
		return fmt.Errorf("givens list printed %q; the Error states view lists %q",
			printed, view)
	}
	return nil
}

// shopScenarios lists every scenario in the copied shop with the kind and status givens list
// should print, read with the parser Givens uses.
func (s *state) shopScenarios() ([]listed, error) {
	features, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "*.feature"))
	if err != nil {
		return nil, err
	}
	items := make([]listed, 0, app.ScenariosPerFeatureMax)
	for _, feature := range features {
		scenarios, err := app.ScenariosIn(feature)
		if err != nil {
			return nil, err
		}
		folder := filepath.Base(filepath.Dir(feature))
		for _, scenario := range scenarios {
			kind := "happy path"
			for _, tag := range scenario.Tags {
				if tag.Name == "@error" {
					kind = "error state"
				}
			}
			path := folder + "/" + app.SlugOf(scenario.Name)
			items = append(items, listed{Path: path, Kind: kind, Status: "Never run"})
		}
	}
	return items, nil
}

// pathLines maps each path givens list printed to its whole line.
func pathLines(output string) map[string]string {
	pattern := regexp.MustCompile(listPath)
	lines := make(map[string]string, 16)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && pattern.MatchString(fields[0]) {
			lines[fields[0]] = line
		}
	}
	return lines
}

// yourTurnLines works out what a script's first_you, when_you and then_check calls print, per
// the kit's about.md: "First, …", "When you …" and "Then check that …".
func yourTurnLines(script string) []string {
	call := regexp.MustCompile(`(?m)^\s*(first_you|when_you|then_check) (?:'([^']*)'|"([^"]*)")`)
	prefixes := map[string]string{
		"first_you": "First, ", "when_you": "When you ", "then_check": "Then check that ",
	}
	lines := make([]string, 0, 8)
	for _, match := range call.FindAllStringSubmatch(script, -1) {
		lines = append(lines, prefixes[match[1]]+match[2]+match[3])
	}
	return lines
}
