// Package views runs capabilities/views. Steps shared with other capabilities are in
// acceptance/world.
package views

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	gherkin "github.com/cucumber/gherkin/go/v42"
	messages "github.com/cucumber/messages/go/v34"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/spec"
)

const capability = "views"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	named []string // scenarios a step named, so "the other N scenarios" can leave them out
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^they select the "([^"]*)" view$`, s.selectView),
		world.Def(`^only error states are listed, from every capability$`, s.onlyErrorStates),
		world.Def(`^each one shows which capability it belongs to$`, s.eachShowsCapability),
		world.Def(`^"([^"]*)" has a failing run recorded$`, s.recordFailingRun),
		world.Def(`^"([^"]*)" isn't listed$`, s.notListed),
		world.Def(`^"([^"]*)" is listed with "([^"]*)"$`, s.listedWith),
		world.Def(`^the other (\d+) scenarios are listed with "([^"]*)"$`, s.othersListedWith),
		world.Def(`^every documented helper is listed with its signature and description$`,
			s.helpersWithSignatures),
		world.Def(`^each one shows the file it's defined in and how many scripts call it$`,
			s.helpersWithFilesAndCallers),
		world.Def(`^a developer has a project open in Givens whose scenarios are all happy paths$`,
			s.openAllHappyPaths),
	}
}

// diskScenario is a scenario as the sample shop's files define it, read without Givens, so
// the steps compare Givens's lists against the files themselves.
type diskScenario struct {
	Name       string
	Capability string // the Feature's name
	Error      bool   // tagged @error, on the scenario or its Feature
}

// projectScenarios parses every capability's .feature file with the Gherkin parser.
// Ref: https://pkg.go.dev/github.com/cucumber/gherkin/go/v42#ParseGherkinDocument
func (s *state) projectScenarios() ([]diskScenario, error) {
	features, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "*.feature"))
	if err != nil {
		return nil, err
	}
	found := make([]diskScenario, 0, 16)
	for _, feature := range features {
		source, err := os.ReadFile(feature)
		if err != nil {
			return nil, err
		}
		ids := &messages.Incrementing{}
		document, err := gherkin.ParseGherkinDocument(strings.NewReader(string(source)), ids.NewId)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", feature, err)
		}
		featureError := hasErrorTag(document.Feature.Tags)
		for _, scenario := range spec.Scenarios(document.Feature) {
			found = append(found, diskScenario{
				Name: scenario.Name, Capability: document.Feature.Name,
				Error: featureError || hasErrorTag(scenario.Tags),
			})
		}
	}
	if len(found) == 0 {
		return nil, errors.New("the sample shop has no scenarios")
	}
	return found, nil
}

func hasErrorTag(tags []*messages.Tag) bool {
	for _, tag := range tags {
		if tag.Name == "@error" {
			return true
		}
	}
	return false
}

func (s *state) listed() (map[string]app.Scenario, error) {
	window, err := s.Front()
	if err != nil {
		return nil, err
	}
	scenarios := window.ListedScenarios()
	byName := make(map[string]app.Scenario, len(scenarios))
	for _, scenario := range scenarios {
		byName[scenario.Name] = scenario
	}
	if len(byName) != len(scenarios) {
		return nil, fmt.Errorf("the view lists a scenario twice: %v", scenarios)
	}
	return byName, nil
}

func (s *state) selectView(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Select(name)
}

// onlyErrorStates compares the listed scenarios with every @error scenario on disk, which
// spans more than one capability in the sample shop.
func (s *state) onlyErrorStates() error {
	onDisk, err := s.projectScenarios()
	if err != nil {
		return err
	}
	listed, err := s.listed()
	if err != nil {
		return err
	}
	capabilities := make(map[string]bool, 4)
	wanted := 0
	for _, scenario := range onDisk {
		_, isListed := listed[scenario.Name]
		if scenario.Error && !isListed {
			return fmt.Errorf("error state %q (%s) isn't listed",
				scenario.Name, scenario.Capability)
		}
		if !scenario.Error && isListed {
			return fmt.Errorf("happy path %q is listed among error states", scenario.Name)
		}
		if scenario.Error {
			capabilities[scenario.Capability] = true
			wanted++
		}
	}
	if len(listed) != wanted {
		return fmt.Errorf("%d scenarios are listed, not the %d error states", len(listed), wanted)
	}
	if len(capabilities) < 2 {
		return fmt.Errorf("the sample shop's error states come from %d capability",
			len(capabilities))
	}
	return nil
}

func (s *state) eachShowsCapability() error {
	onDisk, err := s.projectScenarios()
	if err != nil {
		return err
	}
	listed, err := s.listed()
	if err != nil {
		return err
	}
	if len(listed) == 0 {
		return errors.New("nothing is listed")
	}
	for _, scenario := range onDisk {
		got, isListed := listed[scenario.Name]
		if isListed && got.Capability != scenario.Capability {
			return fmt.Errorf("%q shows capability %q, not %q",
				scenario.Name, got.Capability, scenario.Capability)
		}
	}
	return nil
}

// recordFailingRun drops in to a scenario, marks its Givens and Whens done and its Thens
// failed, and records the run, as a developer would when the app misbehaves.
func (s *state) recordFailingRun(name string) error {
	s.named = append(s.named, name)
	if s.Window == nil {
		if err := s.OpenSampleShop(); err != nil {
			return err
		}
	}
	if err := s.DropIn(name); err != nil {
		return err
	}
	if status := s.Session.Status(); status != app.SessionReady {
		return fmt.Errorf("drop in to %q ended %q, not ready", name, status)
	}
	for _, item := range s.Session.Checklist() {
		mark := "done"
		if item.Kind == "then" {
			mark = "fail"
		}
		if err := s.Session.Mark(item.Text, mark); err != nil {
			return err
		}
	}
	return s.Session.RecordRun("")
}

func (s *state) notListed(name string) error {
	s.named = append(s.named, name)
	listed, err := s.listed()
	if err != nil {
		return err
	}
	if len(listed) == 0 {
		return errors.New("nothing is listed, so the view wasn't worked out")
	}
	if _, isListed := listed[name]; isListed {
		return fmt.Errorf("%q is listed, and shouldn't be", name)
	}
	return nil
}

func (s *state) listedWith(name string, status string) error {
	s.named = append(s.named, name)
	listed, err := s.listed()
	if err != nil {
		return err
	}
	got, isListed := listed[name]
	if !isListed {
		return fmt.Errorf("%q isn't listed", name)
	}
	if got.Status != status {
		return fmt.Errorf("%q is listed with %q, not %q", name, got.Status, status)
	}
	return nil
}

// othersListedWith checks every scenario on disk that no earlier step named.
func (s *state) othersListedWith(count string, status string) error {
	want, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	onDisk, err := s.projectScenarios()
	if err != nil {
		return err
	}
	listed, err := s.listed()
	if err != nil {
		return err
	}
	// listedWith adds each name it checks to s.named, so keep the names earlier steps gave.
	named := slices.Clone(s.named)
	others := 0
	for _, scenario := range onDisk {
		if contains(named, scenario.Name) {
			continue
		}
		others++
		if err := s.listedWith(scenario.Name, status); err != nil {
			return err
		}
	}
	if others != want {
		return fmt.Errorf("the sample shop has %d other scenarios, not %d", others, want)
	}
	namedListed := 0
	for name := range listed {
		if contains(named, name) {
			namedListed++
		}
	}
	if len(listed) != others+namedListed {
		return fmt.Errorf("the view lists %d scenarios, not %d others and %d named",
			len(listed), others, namedListed)
	}
	return nil
}

func contains(names []string, name string) bool {
	for _, each := range names {
		if each == name {
			return true
		}
	}
	return false
}
