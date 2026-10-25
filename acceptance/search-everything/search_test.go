// Package searcheverything runs capabilities/search-everything. Steps shared with other
// capabilities are in acceptance/world.
package searcheverything

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
)

const capability = "search-everything"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	query    string           // what a Given step searched for
	selected app.SearchResult // the result a Then step last found, for "selecting it"
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^they search for "([^"]*)"$`, s.Search),
		world.Def(`^the results are grouped under (.+) and ([A-Z][a-z]+)$`, s.groupedUnder),
		world.Def(`^"([^"]*)" and "([^"]*)" are listed under ([A-Z][a-z]+)$`, s.bothListedUnder),
		world.Def(`^every match is marked in its result$`, s.everyMatchMarked),
		world.Def(`^"([^"]*)" is listed under ([A-Z][a-z]+), matching "([^"]*)"$`,
			s.listedMatching),
		world.Def(`^"([^"]*)" is listed under ([A-Z][a-z]+)$`, s.listedUnder),
		world.Def(`^the step "(.+)" is listed with "(\d+) uses?"$`, s.stepWithUses),
		world.Def(`^selecting it lists "([^"]*)" and "([^"]*)"$`, s.selectingItLists),
		world.Def(`^a developer has searched for "([^"]*)"$`, s.hasSearched),
		world.Def(`^they narrow the results to "([^"]*)"$`, s.narrowTo),
		world.Def(`^only (stories|scenarios|steps|scripts|helpers) are listed$`, s.onlyKind),
		world.Def(`^scripts with "([^"]*)" in their name come before scripts that only mention it$`,
			s.namedScriptsFirst),
		world.Def(`^([a-z_]+) is listed under Helpers with its description$`, s.helperListed),
		world.Def(`^selecting it shows its signature and the (\d+) scripts that call it$`,
			s.helperDetail),
		world.Def(`^they see "Nothing matches "([^"]*)""$`, s.nothingMatches),
	}
}

// groupedUnder expects exactly these kinds of results, in this order.
func (s *state) groupedUnder(kinds string, last string) error {
	want := append(strings.Split(kinds, ", "), last)
	window, err := s.Front()
	if err != nil {
		return err
	}
	got := make([]string, 0, len(want))
	for _, group := range window.SearchResults().Groups {
		if len(group.Results) > 0 {
			got = append(got, group.Kind)
		}
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("the results are grouped under %q, not %q", got, want)
	}
	return nil
}

func (s *state) bothListedUnder(first string, second string, kind string) error {
	if err := s.listedUnder(first, kind); err != nil {
		return err
	}
	return s.listedUnder(second, kind)
}

// everyMatchMarked expects each result to mark every place the query appears in its title and
// second line, and nothing else: its Matches, which `givens search --json` prints.
func (s *state) everyMatchMarked() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	results := window.SearchResults()
	if results.Query == "" || len(results.Groups) == 0 {
		return fmt.Errorf("no search results to check: %+v", results)
	}
	query := strings.ToLower(results.Query)
	for _, group := range results.Groups {
		for _, result := range group.Results {
			shown := strings.ToLower(result.Name + "\n" + result.Detail)
			want := strings.Count(shown, query)
			if want == 0 || len(result.Matches) != want {
				return fmt.Errorf("%s %q shows %q %d times and marks %q",
					group.Kind, result.Name, results.Query, want, result.Matches)
			}
			for _, match := range result.Matches {
				if !strings.EqualFold(match, results.Query) {
					return fmt.Errorf("%q marks %q, not %q", result.Name, match, query)
				}
			}
		}
	}
	return nil
}

func (s *state) listedMatching(name string, kind string, line string) error {
	if err := s.listedUnder(name, kind); err != nil {
		return err
	}
	if s.selected.Detail != line {
		return fmt.Errorf("%q under %s matches %q, not %q", name, kind, s.selected.Detail, line)
	}
	return nil
}

// listedUnder finds a result by its title under a kind, and keeps it for "selecting it".
func (s *state) listedUnder(name string, kind string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	results := group(window.SearchResults(), kind)
	for _, result := range results {
		if result.Name == name {
			s.selected = result
			return nil
		}
	}
	return fmt.Errorf("%q isn't listed under %s; it lists %q", name, kind, titles(results))
}

// stepWithUses expects the step once under Steps, however many scenarios use it.
func (s *state) stepWithUses(step string, uses int) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	results := group(window.SearchResults(), app.SearchSteps)
	found := 0
	for _, result := range results {
		if result.Name == step {
			found++
			s.selected = result
		}
	}
	if found != 1 {
		return fmt.Errorf("Steps lists %q %d times: %q", step, found, titles(results))
	}
	if s.selected.Uses != uses {
		return fmt.Errorf("%q is listed with %d uses, not %d", step, s.selected.Uses, uses)
	}
	return nil
}

// selectingItLists selects the result found earlier and expects exactly these scenarios.
func (s *state) selectingItLists(first string, second string) error {
	if s.selected.Name == "" {
		return fmt.Errorf("no earlier step found a result to select")
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := window.Select(s.selected.Name); err != nil {
		return err
	}
	got := make([]string, 0, 2)
	for _, scenario := range window.ListedScenarios() {
		got = append(got, scenario.Name)
	}
	want := []string{first, second}
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
		return fmt.Errorf("selecting %q lists %q, not %q", s.selected.Name, got, want)
	}
	return nil
}

func (s *state) hasSearched(query string) error {
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	s.query = query
	return s.Search(query)
}

// narrowTo keeps one kind of result, as `givens search --kind` does.
func (s *state) narrowTo(kind string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.FilterSearch(kind)
}

// onlyKind expects one non-empty group, the filtered kind, and the filter shown as chosen.
func (s *state) onlyKind(kind string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	results := window.SearchResults()
	heading := strings.ToUpper(kind[:1]) + kind[1:]
	if results.Filter != heading {
		return fmt.Errorf("the chosen filter is %q, not %q", results.Filter, heading)
	}
	if len(results.Groups) != 1 || results.Groups[0].Kind != heading {
		return fmt.Errorf("the results are %+v, not only %s", results.Groups, kind)
	}
	if len(results.Groups[0].Results) == 0 {
		return fmt.Errorf("no %s are listed", kind)
	}
	return nil
}

// namedScriptsFirst expects every script named for the query before any script that only
// mentions it, and at least one of each, so the order is really tested.
func (s *state) namedScriptsFirst(word string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	scripts := titles(group(window.SearchResults(), app.SearchScripts))
	named := func(name string) bool {
		return strings.Contains(strings.ToLower(name), strings.ToLower(word))
	}
	lastNamed, firstMention := -1, len(scripts)
	for index, name := range scripts {
		if named(name) {
			lastNamed = index
		} else if firstMention == len(scripts) {
			firstMention = index
		}
	}
	if lastNamed < 0 || firstMention == len(scripts) {
		return fmt.Errorf("the scripts %q don't include both kinds to order", scripts)
	}
	if lastNamed > firstMention {
		return fmt.Errorf("the scripts are in the order %q", scripts)
	}
	return nil
}

func (s *state) nothingMatches(query string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	message := `Nothing matches "` + query + `"`
	if err := world.Expect(window.Visible(), message, "the window"); err != nil {
		return err
	}
	if groups := window.SearchResults().Groups; len(groups) > 0 {
		return fmt.Errorf("%q is shown beside results: %+v", message, groups)
	}
	return nil
}

// group is the results of one kind, or nil when the search found none of that kind.
func group(results app.SearchResults, kind string) []app.SearchResult {
	for _, found := range results.Groups {
		if found.Kind == kind {
			return found.Results
		}
	}
	return nil
}

func titles(results []app.SearchResult) []string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Name)
	}
	return names
}
