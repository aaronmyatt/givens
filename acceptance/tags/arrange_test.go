package tags

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"givens/internal/app"
)

// tagScenario adds a tag to a scenario in whichever sample shop capability holds it.
func (s *state) tagScenario(name string, tag string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	folder, err := s.ScenarioCapability(name)
	if err != nil {
		return err
	}
	return s.addTag(folder, "Scenario: "+name, tag)
}

// addTag adds tag to the tag line above header ("Feature: Checkout", "Scenario: Card
// declined"), or puts a tag line there, at header's indentation. Gherkin tags sit on the
// lines before the keyword they apply to. Ref: https://cucumber.io/docs/cucumber/api/#tags
func (s *state) addTag(folder string, header string, tag string) error {
	rel := filepath.Join("capabilities", folder, folder+".feature")
	text, err := s.ReadFile(rel)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	at := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimSpace(line) == header
	})
	if at < 0 {
		return fmt.Errorf("%s has no line %q", rel, header)
	}
	above := ""
	if at > 0 {
		above = strings.TrimSpace(lines[at-1])
	}
	switch {
	case slices.Contains(strings.Fields(above), tag):
		return nil
	case strings.HasPrefix(above, "@"):
		lines[at-1] += " " + tag
	default:
		indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " "))]
		lines = slices.Insert(lines, at, indent+tag)
	}
	if err := s.WriteFile(rel, strings.Join(lines, "\n")); err != nil {
		return err
	}
	if _, err := app.ScenariosIn(filepath.Join(s.Project, rel)); err != nil {
		return fmt.Errorf("the tagged %s doesn't parse: %w", rel, err)
	}
	return nil
}

// hasTagAbove is true when the tag lines just above the line starting with prefix hold tag.
func hasTagAbove(lines []string, prefix string, tag string) bool {
	at := slices.IndexFunc(lines, func(line string) bool {
		return strings.HasPrefix(strings.TrimSpace(line), prefix)
	})
	for above := at - 1; above >= 0; above-- {
		line := strings.TrimSpace(lines[above])
		if !strings.HasPrefix(line, "@") {
			return false
		}
		if slices.Contains(strings.Fields(line), tag) {
			return true
		}
	}
	return false
}

// namesIn lists, sorted, the scenario names in a sample shop capability.
func (s *state) namesIn(folder string) ([]string, error) {
	scenarios, err := app.ScenariosIn(filepath.Join(s.Capabilities(), folder, folder+".feature"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, scenario.Name)
	}
	slices.Sort(names)
	return names, nil
}

func sortedNames(scenarios []app.Scenario) []string {
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, scenario.Name)
	}
	slices.Sort(names)
	return names
}

// sameNames compares two lists of names in any order.
func sameNames(got []string, want []string, what string) error {
	got, want = slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))
	if len(want) == 0 || !slices.Equal(got, want) {
		return fmt.Errorf("%s lists %q, not %q", what, got, want)
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

// foundScenarios names the scenarios under Scenarios in the search results.
func foundScenarios(window *app.Window) []string {
	results := group(window.SearchResults(), app.SearchScenarios)
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Name)
	}
	return names
}
