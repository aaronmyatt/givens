package acceptance

import (
	"bufio"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	messages "github.com/cucumber/messages/go/v34"

	"givens/internal/spec"
)

// bigFile tells Givens's stories in order, relative to this package, and bigHelpersFile holds
// the code two or more of them use. See their headers and tasks/one-big-file/notes.md.
const (
	bigFile        = "../internal/app/big.go"
	bigHelpersFile = "../internal/app/big_helpers.go"
)

// The comments that lay out the big file: a section per Feature, and inside it each Rule and,
// where it helps, a Scenario.
const (
	featureHeading  = "// Feature: "
	ruleHeading     = "// Rule: "
	scenarioHeading = "// Scenario: "
)

// section is one story's part of the big file.
type section struct {
	feature   string
	line      int
	rules     []string
	scenarios []string
}

// story is what a feature file says, for comparing with its section.
type story struct {
	path      string
	order     int // its place among the capabilities folders
	rules     []string
	scenarios []string
}

// TestBigFile keeps the big file's sections in step with the feature files, as a plain check
// that needs no agent. Each section names a Feature word for word, the sections follow the
// order of the capabilities folders, a section has every one of its Feature's Rules in order,
// a Scenario comment names one of the Feature's scenarios, and every Feature has a section.
func TestBigFile(t *testing.T) {
	stories, err := readStories()
	if err != nil {
		t.Fatal(err)
	}
	sections, err := readSections(bigFile)
	if err != nil {
		t.Fatal(err)
	}
	strays, err := readSections(bigHelpersFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, stray := range strays {
		t.Errorf("internal/app/big_helpers.go:%d: \"Feature: %s\" belongs in big.go; this file "+
			"holds only helpers", stray.line, stray.feature)
	}
	last := -1
	for _, part := range sections {
		where := fmt.Sprintf("internal/app/big.go:%d", part.line)
		told, ok := stories[part.feature]
		if !ok {
			t.Errorf("%s: no capability has \"Feature: %s\"", where, part.feature)
			continue
		}
		if told.order <= last {
			t.Errorf("%s: %q is out of order; sections follow the capabilities folders", where,
				part.feature)
		}
		last = told.order
		if !slices.Equal(part.rules, told.rules) {
			t.Errorf("%s: its Rule comments are %q; %s has the Rules %q, in that order", where,
				part.rules, told.path, told.rules)
		}
		for _, name := range part.scenarios {
			if !slices.Contains(told.scenarios, name) {
				t.Errorf("%s: \"Scenario: %s\" isn't in %s", where, name, told.path)
			}
		}
	}
	told := make(map[string]bool, len(sections))
	for _, part := range sections {
		told[part.feature] = true
	}
	// Every story is in now, so one with no section is a story left out, not one to come.
	// Ref: https://pkg.go.dev/maps#Keys
	for _, name := range slices.Sorted(maps.Keys(stories)) {
		if !told[name] {
			t.Errorf("%s: \"Feature: %s\" has no section in internal/app/big.go", stories[name].path,
				name)
		}
	}
	t.Logf("%d of %d stories are in the big file", len(sections), len(stories))
}

// readStories reads every capability's Feature, Rules and scenario names, keyed by its name.
func readStories() (map[string]story, error) {
	paths, err := filepath.Glob(filepath.Join(capabilitiesDir, "*", "*.feature"))
	if err != nil {
		return nil, err
	}
	stories := make(map[string]story, len(paths))
	for order, path := range paths {
		feature, err := spec.ParseFeature(path)
		if err != nil {
			return nil, err
		}
		told := story{path: strings.TrimPrefix(path, "../"), order: order, rules: rulesOf(feature)}
		for _, scenario := range spec.Scenarios(feature) {
			told.scenarios = append(told.scenarios, scenario.Name)
		}
		stories[feature.Name] = told
	}
	return stories, nil
}

func rulesOf(feature *messages.Feature) []string {
	rules := make([]string, 0, len(feature.Children))
	for _, child := range feature.Children {
		if child.Rule != nil {
			rules = append(rules, child.Rule.Name)
		}
	}
	return rules
}

// readSections reads a file's Feature, Rule and Scenario comments into sections.
func readSections(path string) ([]section, error) {
	opened, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = opened.Close() }()
	sections := make([]section, 0, 32)
	scanner := bufio.NewScanner(opened)
	// Ref: https://pkg.go.dev/bufio#Scanner
	for number := 1; scanner.Scan(); number++ { // bound: ends at the file's end
		line := scanner.Text()
		if name, ok := strings.CutPrefix(line, featureHeading); ok {
			sections = append(sections, section{feature: name, line: number})
			continue
		}
		if len(sections) == 0 {
			continue // the header, before the first story
		}
		current := &sections[len(sections)-1]
		if name, ok := strings.CutPrefix(line, ruleHeading); ok {
			current.rules = append(current.rules, name)
		}
		if name, ok := strings.CutPrefix(line, scenarioHeading); ok {
			current.scenarios = append(current.scenarios, name)
		}
	}
	return sections, scanner.Err()
}
