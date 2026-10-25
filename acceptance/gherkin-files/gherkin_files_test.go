// Package gherkinfiles runs capabilities/gherkin-files. Steps shared with other capabilities
// are in acceptance/world.
package gherkinfiles

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/slug"
)

const (
	capability   = "gherkin-files"
	checkoutPath = "capabilities/checkout/checkout.feature"
)

func TestScenarios(t *testing.T) {
	// npx runs only the sample shop's own cucumber-js: an empty cache and "no" to installs stop
	// it fetching the unrelated registry package named cucumber-js.
	// Ref: https://docs.npmjs.com/cli/commands/npm-exec#yes
	t.Setenv("npm_config_yes", "false")
	t.Setenv("npm_config_cache", t.TempDir())
	world.RunSuite(t, capability, steps)
}

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	names    []string // the scenarios a Given step arranged, in the order it named them
	listed   string   // the capability a Then step last found listed, for "selecting it"
	comment  string   // the comment line a Given step added
	editFrom string   // the step line an edit replaced
	editTo   string   // what the edit replaced it with
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^they run "([^"]*)" from its root folder$`, s.runFromRoot),
		world.Def(`^cucumber-js finds all (\d+) scenarios$`, s.cucumberFinds),
		world.Def(`^it reports no parse errors$`, s.noParseErrors),
		world.Def(`^checkout\.feature puts "([^"]*)" and "([^"]*)" under Rule: (.+)$`,
			s.underRule),
		world.Def(`^those two scenarios are listed under "([^"]*)"$`, s.listedUnderRule),
		world.Def(`^their run keys stay ([a-z0-9/-]+) and ([a-z0-9/-]+)$`, s.runKeysStay),
		world.Def(`^checkout\.feature has the comment "([^"]*)" above "([^"]*)"$`,
			s.commentAbove),
		world.Def(`^a developer edits a step of "([^"]*)" with givens edit and saves$`,
			s.editStep),
		world.Def(`^the comment is still there$`, s.commentStillThere),
		world.Def(`^git diff shows only the step that changed$`, s.diffShowsOnlyStep),
		world.Def(`^the sample shop has ([a-z0-9/._-]+\.ts)$`, s.stepsFile),
		world.Def(`^a folder capabilities/steps/ with no \.feature file$`, s.stepsFolder),
		world.Def(`^Checkout loads as normal, with no warning about the steps file$`,
			s.checkoutLoadsWithoutWarning),
		world.Def(`^"([^"]*)" isn't listed as a capability$`, s.notACapability),
		world.Def(`^"([^"]*)" and "([^"]*)" load as normal$`, s.loadAsNormal),
		world.Def(`^"([^"]*)" is listed with a warning$`, s.listedWithWarning),
		world.Def(`^selecting it shows "([^"]*)"$`, s.selectingItShows),
		world.Def(`^checkout\.feature has scenarios named "([^"]*)" and "([^"]*)"$`,
			s.scenariosNamed),
		world.Def(`^both are listed with the warning "([^"]*)"$`, s.bothWarned),
		world.Def(`^"([^"]*)" isn't offered on either until one is renamed$`,
			s.notOfferedUntilRenamed),
		world.Def(`^the checkout folder holds checkout\.feature and ([a-z-]+\.feature)$`,
			s.secondFeature),
		world.Def(`^Checkout loads from checkout\.feature$`, s.loadsFromCheckoutFeature),
		world.Def(`^it shows the warning "([^"]*)"$`, s.showsWarning),
	}
}

// runFromRoot runs a test runner's command from the sample shop's root folder, with a deadline
// so a runner that hangs fails the step instead of the whole suite.
func (s *state) runFromRoot(command string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	s.Cwd = s.Project
	return runBounded(s.World, command, runnerDeadline)
}

// cucumberFinds reads the summary line cucumber-js prints, like "9 scenarios (9 undefined)".
// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/cli.md
func (s *state) cucumberFinds(want int) error {
	got, err := scenarioCount(s.Output)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("cucumber-js found %d scenarios, not %d:\n%s", got, want, s.Output)
	}
	return nil
}

// noParseErrors fails on cucumber-js's "Parse error in ..." report.
func (s *state) noParseErrors() error {
	if strings.Contains(strings.ToLower(s.Output), "parse error") {
		return fmt.Errorf("cucumber-js reported a parse error:\n%s", s.Output)
	}
	if _, err := scenarioCount(s.Output); err != nil {
		return err
	}
	return nil
}

// underRule moves two checkout scenarios into a Rule: block at the end of checkout.feature,
// adding the second when the sample shop doesn't have it.
func (s *state) underRule(first string, second string, rule string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.EnsureScenario("checkout", second); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutPath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	moved := make([]string, 0, 16)
	for _, name := range []string{first, second} {
		start, end, err := scenarioBlock(lines, name)
		if err != nil {
			return err
		}
		for _, line := range lines[start:end] {
			moved = append(moved, strings.TrimRight("  "+line, " "))
		}
		moved = append(moved, "")
		lines = slices.Delete(lines, start, end)
		if start > 0 && start < len(lines) && lines[start-1] == "" && lines[start] == "" {
			lines = slices.Delete(lines, start, start+1)
		}
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" { // bound: lines only shrinks
		lines = lines[:len(lines)-1]
	}
	lines = append(append(lines, "", "  Rule: "+rule, ""), moved...)
	s.names = []string{first, second}
	return s.writeCheckout(lines)
}

func (s *state) listedUnderRule(rule string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	groups, err := window.RuleGroups("Checkout")
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.Name != rule {
			continue
		}
		if !slices.Equal(group.Scenarios, s.names) {
			return fmt.Errorf("Rule %q lists %q, not %q", rule, group.Scenarios, s.names)
		}
		return nil
	}
	return fmt.Errorf("Checkout has no Rule %q; its rules are %+v", rule, groups)
}

// runKeysStay works out each arranged scenario's run key from what the window shows: the
// capability's slug, a slash, and the scenario's slug.
func (s *state) runKeysStay(firstKey string, secondKey string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	checkout, err := window.Capability("Checkout")
	if err != nil {
		return err
	}
	for index, want := range []string{firstKey, secondKey} {
		scenario, err := findScenario(checkout, s.names[index])
		if err != nil {
			return err
		}
		if got := checkout.Slug + "/" + scenario.Slug; got != want {
			return fmt.Errorf("%q has the run key %q, not %q", scenario.Name, got, want)
		}
	}
	return nil
}

// commentAbove puts a comment line above a checkout scenario and its tags, then commits the
// shop to git so a later step can diff an edit against it.
func (s *state) commentAbove(comment string, name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutPath)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	start, _, err := scenarioBlock(lines, name)
	if err != nil {
		return err
	}
	lines = slices.Insert(lines, start, "  "+comment)
	s.comment = comment
	if err := s.writeCheckout(lines); err != nil {
		return err
	}
	return commitShop(s.World)
}

func (s *state) commentStillThere() error {
	text, err := s.ReadFile(checkoutPath)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	start, _, err := scenarioBlock(lines, "Card declined")
	if err != nil {
		return err
	}
	if start == 0 || lines[start-1] != "  "+s.comment {
		return fmt.Errorf("the line above \"Card declined\" and its tags is %q, not %q",
			lineBefore(lines, start), "  "+s.comment)
	}
	return nil
}

// diffShowsOnlyStep reads git diff with no context lines: an edit that rewrote the file shows
// more than the one removed and one added line. Ref: https://git-scm.com/docs/git-diff
func (s *state) diffShowsOnlyStep() error {
	if s.editFrom == "" {
		return fmt.Errorf("no step edited a step in this scenario")
	}
	s.Cwd = s.Project
	if err := s.Run("git diff --no-color --unified=0 -- " + checkoutPath); err != nil {
		return err
	}
	if s.ExitCode != 0 {
		return fmt.Errorf("git diff exited %d:\n%s", s.ExitCode, s.Output)
	}
	got := changedLines(s.Output)
	want := []string{"-" + s.editFrom, "+" + s.editTo}
	if !slices.Equal(got, want) {
		return fmt.Errorf("git diff changed %q, not just %q:\n%s", got, want, s.Output)
	}
	return nil
}

func (s *state) stepsFile(rel string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.WriteFile(filepath.Join("capabilities", rel), stepsSource)
}

func (s *state) stepsFolder() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.WriteFile("capabilities/steps/shared.steps.ts", stepsSource)
}

// checkoutLoadsWithoutWarning expects every scenario in checkout.feature, and no warning on
// the capability, its line in the list or anywhere in the window that names the steps file.
func (s *state) checkoutLoadsWithoutWarning() error {
	if err := s.loadsNormally("Checkout"); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	return world.ExpectNot(window.Visible(), "steps.ts", "the window")
}

func (s *state) notACapability(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	names := capabilityNames(window.Sidebar())
	if len(names) == 0 {
		return fmt.Errorf("no capabilities are listed")
	}
	if slices.ContainsFunc(names, func(got string) bool { return strings.EqualFold(got, name) }) {
		return fmt.Errorf("%q is listed as a capability: %q", name, names)
	}
	return nil
}

func (s *state) loadAsNormal(first string, second string) error {
	if err := s.loadsNormally(first); err != nil {
		return err
	}
	return s.loadsNormally(second)
}

func (s *state) listedWithWarning(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	item, err := listedCapability(window.Sidebar(), name)
	if err != nil {
		return err
	}
	if item.Warning == "" {
		return fmt.Errorf("%q is listed with no warning", name)
	}
	s.listed = name
	return nil
}

func (s *state) selectingItShows(text string) error {
	if s.listed == "" {
		return fmt.Errorf("no earlier step found a listed item to select")
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := window.Select(s.listed); err != nil {
		return err
	}
	return world.Expect(window.Visible(), text, "the window")
}

func (s *state) scenariosNamed(first string, second string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	for _, name := range []string{first, second} {
		if err := s.EnsureScenario("checkout", name); err != nil {
			return err
		}
	}
	s.names = []string{first, second}
	return nil
}

func (s *state) bothWarned(warning string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	for _, name := range s.names {
		scenario, err := window.Scenario(name)
		if err != nil {
			return err
		}
		if scenario.Warning != warning {
			return fmt.Errorf("%q has the warning %q, not %q", name, scenario.Warning, warning)
		}
	}
	return nil
}

// loadsNormally expects a capability listed with no warning, showing exactly the
// scenarios its .feature file holds.
func (s *state) loadsNormally(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	item, err := listedCapability(window.Sidebar(), name)
	if err != nil {
		return err
	}
	shown, err := window.Capability(name)
	if err != nil {
		return err
	}
	if item.Warning != "" || shown.Warning != "" {
		return fmt.Errorf("%q has a warning: %q / %q", name, item.Warning, shown.Warning)
	}
	want, err := s.namesInFeature(slug.Of(name), slug.Of(name)+".feature")
	if err != nil {
		return err
	}
	if got := scenarioNames(shown); !slices.Equal(got, want) {
		return fmt.Errorf("%q shows the scenarios %q, not %q", name, got, want)
	}
	return nil
}

// namesInFeature lists the scenario names in a sample shop .feature file, read with the same
// parser Givens uses.
func (s *state) namesInFeature(folder string, file string) ([]string, error) {
	scenarios, err := app.ScenariosIn(filepath.Join(s.Capabilities(), folder, file))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, scenario.Name)
	}
	return names, nil
}

// writeCheckout writes checkout.feature and checks it still parses, so a broken arrangement
// fails here and not as a confusing app failure later.
func (s *state) writeCheckout(lines []string) error {
	if err := s.WriteFile(checkoutPath, strings.Join(lines, "\n")+"\n"); err != nil {
		return err
	}
	_, err := app.ScenariosIn(filepath.Join(s.Project, checkoutPath))
	if err != nil {
		return fmt.Errorf("the arranged checkout.feature doesn't parse: %w", err)
	}
	return nil
}

func findScenario(shown app.Capability, name string) (app.Scenario, error) {
	for _, scenario := range shown.Scenarios {
		if scenario.Name == name {
			return scenario, nil
		}
	}
	return app.Scenario{}, fmt.Errorf("%q doesn't show %q; it shows %q",
		shown.Name, name, scenarioNames(shown))
}

func scenarioNames(shown app.Capability) []string {
	names := make([]string, 0, len(shown.Scenarios))
	for _, scenario := range shown.Scenarios {
		names = append(names, scenario.Name)
	}
	return names
}

// editStep opens the scenario to edit, adds words to its last step, and saves, as givens edit
// does with what the developer's editor gives back.
func (s *state) editStep(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	editor, err := window.Edit(name)
	if err != nil {
		return err
	}
	lines := strings.Split(editor.Text(), "\n")
	_, end, err := scenarioBlock(lines, name)
	if err != nil {
		return fmt.Errorf("the editor's text: %w", err)
	}
	s.editFrom = lines[end-1]
	s.editTo = s.editFrom + " for an hour"
	lines[end-1] = s.editTo
	if err := editor.SetText(strings.Join(lines, "\n")); err != nil {
		return err
	}
	return editor.Save()
}

// notOfferedUntilRenamed expects neither action offered on either scenario, renames the second
// in Givens, and then expects both actions offered on both.
func (s *state) notOfferedUntilRenamed(action string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	for _, name := range s.names {
		if err := window.Select(name); err != nil {
			return err
		}
		if slices.Contains(window.Offers(), action) {
			return fmt.Errorf("%q is offered on %q while its slug clashes", action, name)
		}
	}
	renamed := s.names[1] + " again"
	if err := window.Rename(s.names[1], renamed); err != nil {
		return err
	}
	for _, name := range []string{s.names[0], renamed} {
		if err := window.Select(name); err != nil {
			return err
		}
		if err := world.Expect(window.Offers(), action, name+"'s actions"); err != nil {
			return fmt.Errorf("after the rename: %w", err)
		}
	}
	return nil
}

func (s *state) secondFeature(file string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.WriteFile(filepath.Join("capabilities", "checkout", file), refundsFeature)
}

// loadsFromCheckoutFeature expects Checkout to show checkout.feature's scenarios and none of
// refunds.feature's.
func (s *state) loadsFromCheckoutFeature() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	shown, err := window.Capability("Checkout")
	if err != nil {
		return err
	}
	want, err := s.namesInFeature("checkout", "checkout.feature")
	if err != nil {
		return err
	}
	if got := scenarioNames(shown); !slices.Equal(got, want) {
		return fmt.Errorf("Checkout shows the scenarios %q, not %q", got, want)
	}
	s.listed = "Checkout"
	return nil
}

func (s *state) showsWarning(warning string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	shown, err := window.Capability(s.listed)
	if err != nil {
		return err
	}
	if shown.Warning != warning {
		return fmt.Errorf("%q has the warning %q, not %q", s.listed, shown.Warning, warning)
	}
	return nil
}
