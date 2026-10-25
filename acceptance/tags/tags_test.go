// Package tags runs capabilities/tags. Steps shared with other capabilities are in
// acceptance/world.
package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
)

const capability = "tags"

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
	names   []string // the scenarios a Given step tagged, in the order it named them
	subject string   // the scenario a step called "it"
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	defined := []world.Step{
		world.Def(`^"([^"]*)" is tagged (@\S+) in checkout\.feature$`, s.taggedInCheckout),
		world.Def(`^a developer selects the "([^"]*)" view$`, s.selectView),
		world.Def(`^(?:they search|a developer searches) for "([^"]*)"$`, s.Search),
		world.Def(`^"([^"]*)" is listed, marked as an error state$`, s.listedAsError),
		world.Def(`^"([^"]*)" and "([^"]*)" are both tagged (@\S+)$`, s.bothTagged),
		world.Def(`^a developer selects "([^"]*)" on the Tags tab$`, s.selectTag),
		world.Def(`^only those two scenarios are listed$`, s.onlyThoseListed),
		world.Def(`^the Feature: (.+) line is tagged (@\S+)$`, s.featureTagged),
		world.Def(`^every (.+) scenario is listed$`, s.everyScenarioListed),
		world.Def(`^cucumber-js finds the same scenarios the Error states view lists$`,
			s.cucumberMatchesErrorStates),
		world.Def(`^only those two scenarios are listed under Scenarios$`, s.onlyThoseFound),
		world.Def(`^a scenario that gets (@\S+) from its Feature line is listed too$`,
			s.featureTagFound),
		world.Def(`^"([^"]*)" is tagged (@\S+) and "([^"]*)" is tagged (@\S+) and (@\S+)$`,
			s.taggedThree),
		world.Def(`^"([^"]*)" is listed$`, s.found),
		world.Def(`^"([^"]*)" isn't$`, s.notFound),
		world.Def(`^only "([^"]*)" is listed, with "([^"]*)" marked as the match$`,
			s.onlyFoundMarked),
		world.Def(`^no results are listed until the expression is complete$`,
			s.noResultsUntilComplete),
	}
	return append(defined, s.todoSteps()...)
}

func (s *state) taggedInCheckout(name string, tag string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	folder, err := s.ScenarioCapability(name)
	if err != nil {
		return err
	}
	if folder != "checkout" {
		return fmt.Errorf("%q is in %s, not checkout", name, folder)
	}
	s.subject = name
	return s.addTag(folder, "Scenario: "+name, tag)
}

func (s *state) selectView(view string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Select(view)
}

// listedAsError expects the view to list exactly the shop's @error scenarios, Feature tags
// included, with the named one among them marked as an error state.
func (s *state) listedAsError(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	listed := window.ListedScenarios()
	want, err := s.taggedScenarios("@error")
	if err != nil {
		return err
	}
	if got := sortedNames(listed); !slices.Equal(got, want) {
		return fmt.Errorf("the view lists %q, not the @error scenarios %q", got, want)
	}
	for _, scenario := range listed {
		if scenario.Name == name && scenario.Error {
			return nil
		}
	}
	return fmt.Errorf("%q isn't marked as an error state in %+v", name, listed)
}

func (s *state) bothTagged(first string, second string, tag string) error {
	for _, name := range []string{first, second} {
		if err := s.tagScenario(name, tag); err != nil {
			return err
		}
	}
	s.names = []string{first, second}
	return nil
}

// selectTag expects the tag among the views the Tags tab lists, then selects it.
func (s *state) selectTag(tag string) error {
	if err := s.underTags(tag); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Select(tag)
}

func (s *state) onlyThoseListed() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return sameNames(sortedNames(window.ListedScenarios()), s.names, "the tag view")
}

// featureTagged puts a tag on a capability's Feature: line, so every scenario in it has it.
func (s *state) featureTagged(feature string, tag string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.addTag(app.SlugOf(feature), "Feature: "+feature, tag)
}

func (s *state) everyScenarioListed(feature string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	want, err := s.namesIn(app.SlugOf(feature))
	if err != nil {
		return err
	}
	return sameNames(sortedNames(window.ListedScenarios()), want, "the tag view")
}

// cucumberMatchesErrorStates compares cucumber-js's dry run with the Error states view: the
// scenario count from its summary, and the names it reports for each scenario it couldn't run.
// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/filtering.md#tags
func (s *state) cucumberMatchesErrorStates() error {
	ran, err := cucumberScenarios(s.Output)
	if err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := window.Select("Error states"); err != nil {
		return err
	}
	view := sortedNames(window.ListedScenarios())
	if len(view) == 0 {
		return fmt.Errorf("the Error states view lists nothing")
	}
	if ran.count != len(view) {
		return fmt.Errorf("cucumber-js found %d scenarios; the Error states view lists %q",
			ran.count, view)
	}
	if len(ran.names) > 0 && !slices.Equal(ran.names, view) {
		return fmt.Errorf("cucumber-js found %q; the Error states view lists %q", ran.names, view)
	}
	return nil
}

func (s *state) onlyThoseFound() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return sameNames(foundScenarios(window), s.names, "the search's Scenarios")
}

// featureTagFound tags another capability's Feature: line, opens the shop afresh so Givens
// reads it, searches for the tag again, and expects that capability's scenarios among the
// results alongside the ones tagged one by one.
func (s *state) featureTagFound(tag string) error {
	const feature = "Sign in"
	if err := s.addTag(app.SlugOf(feature), "Feature: "+feature, tag); err != nil {
		return err
	}
	s.App, s.Window = nil, nil
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	if err := s.Search(tag); err != nil {
		return err
	}
	fromFeature, err := s.namesIn(app.SlugOf(feature))
	if err != nil {
		return err
	}
	want := append(append([]string{}, s.names...), fromFeature...)
	return sameNames(foundScenarios(s.Window), want, "the search's Scenarios")
}

func (s *state) taggedThree(first string, firstTag string, second string, secondTag string,
	thirdTag string,
) error {
	taggings := [][2]string{{first, firstTag}, {second, secondTag}, {second, thirdTag}}
	for _, tagging := range taggings {
		if err := s.tagScenario(tagging[0], tagging[1]); err != nil {
			return err
		}
	}
	return nil
}

func (s *state) found(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if got := foundScenarios(window); !slices.Contains(got, name) {
		return fmt.Errorf("the search's Scenarios are %q, without %q", got, name)
	}
	return nil
}

func (s *state) notFound(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	got := foundScenarios(window)
	if len(got) == 0 {
		return fmt.Errorf("the search lists no scenarios at all")
	}
	if slices.Contains(got, name) {
		return fmt.Errorf("the search lists %q among %q", name, got)
	}
	return nil
}

// onlyFoundMarked expects one scenario under Scenarios, with the word among its matches.
func (s *state) onlyFoundMarked(name string, word string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	results := group(window.SearchResults(), app.SearchScenarios)
	if len(results) != 1 || results[0].Name != name {
		return fmt.Errorf("the search's Scenarios are %+v, not just %q", results, name)
	}
	if !slices.Contains(results[0].Matches, word) {
		return fmt.Errorf("%q marks %q, not %q", name, results[0].Matches, word)
	}
	return nil
}

// noResultsUntilComplete expects no results while the expression is broken, then completes
// it and expects results and no error.
func (s *state) noResultsUntilComplete() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if groups := window.SearchResults().Groups; len(groups) > 0 {
		return fmt.Errorf("results are listed for a broken expression: %+v", groups)
	}
	if err := window.Type(" not @slow"); err != nil {
		return err
	}
	if len(foundScenarios(window)) == 0 {
		return fmt.Errorf("the completed expression %q lists no scenarios",
			window.SearchResults().Query)
	}
	return world.ExpectNot(window.Visible(), "Not a tag expression", "the window")
}

// cucumberRun is what a cucumber-js dry run reported.
type cucumberRun struct {
	count int
	names []string // sorted; empty when its formatter printed none
}

// cucumberScenarios reads the summary line ("4 scenarios (4 undefined)") and each reported
// scenario ("1) Scenario: Card declined # capabilities/checkout/checkout.feature:12").
// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/formatters.md
func cucumberScenarios(output string) (cucumberRun, error) {
	if strings.Contains(output, "command not found") && strings.Contains(output, "npx") {
		return cucumberRun{}, fmt.Errorf("npx isn't on PATH; install Node.js:\n%s", output)
	}
	if strings.Contains(output, "npx canceled due to missing packages") {
		return cucumberRun{}, fmt.Errorf("the sample shop has no cucumber-js of its own "+
			"(its package.json needs @cucumber/cucumber, installed):\n%s", output)
	}
	summary := regexp.MustCompile(`(?m)^(\d+) scenarios? \(`).FindStringSubmatch(output)
	if summary == nil {
		return cucumberRun{}, fmt.Errorf("cucumber-js printed no scenario count:\n%s", output)
	}
	count, err := strconv.Atoi(summary[1])
	if err != nil {
		return cucumberRun{}, err
	}
	reported := regexp.MustCompile(`(?m)^\d+\) Scenario: (.+?) # \S+$`)
	names := make([]string, 0, count)
	for _, match := range reported.FindAllStringSubmatch(output, -1) {
		names = append(names, match[1])
	}
	slices.Sort(names)
	return cucumberRun{count: count, names: names}, nil
}

// taggedScenarios lists, sorted, the names of the shop's scenarios that have tag, on the
// scenario or on its Feature, read with the parser Givens uses.
func (s *state) taggedScenarios(tag string) ([]string, error) {
	features, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "*.feature"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 16)
	for _, feature := range features {
		text, err := os.ReadFile(feature)
		if err != nil {
			return nil, err
		}
		featureTagged := hasTagAbove(strings.Split(string(text), "\n"), "Feature:", tag)
		scenarios, err := app.ScenariosIn(feature)
		if err != nil {
			return nil, err
		}
		for _, scenario := range scenarios {
			tagged := featureTagged
			for _, scenarioTag := range scenario.Tags {
				tagged = tagged || scenarioTag.Name == tag
			}
			if tagged {
				names = append(names, scenario.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}
