// Package openaproject runs capabilities/open-a-project. Steps shared with other
// capabilities are in acceptance/world.
//
// Opening a project is internal/app's App.Open, which the givens command calls, so these steps
// drive that core; what a person sees is read from internal/tui's model drawn over the window.
// The terminal scenarios run the built givens command, with its test seam standing in for a
// terminal (see testScreenEnv in cmd/givens).
package openaproject

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/tui"
)

const capability = "open-a-project"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	bare     string      // a project folder with no capabilities folder
	before   []string    // the bare project's files before the step under test
	opened   *app.Window // the window the last Open returned
	gotFront *app.Window // the front window before the step under test
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	all := append(projectSteps(s), terminalSteps(s)...)
	return append(all, branchSteps(s)...)
}

// projectSteps open one project and read what its window shows.
func projectSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has the sample shop project, with (.+)$`, s.shopWithFeatures),
		world.Def(`^a developer has the sample shop project open$`, s.OpenSampleShop),
		world.Def(`^they open its capabilities folder$`, s.openCapabilities),
		world.Def(`^Givens lists "([^"]*)", "([^"]*)" and "([^"]*)", from each Feature: line$`,
			s.sidebarListsThree),
		world.Def(`^each capability shows how many scenarios it has$`, s.countsShown),
		world.Def(`^the ([A-Z][a-z]+) story is shown beside the list$`, s.storyShown),
		world.Def(`^they open the project's root folder$`, s.openRoot),
		world.Def(`^Givens finds the capabilities folder inside it$`, s.foundCapabilities),
		world.Def(`^the top line of the TUI shows the path to that capabilities folder$`,
			s.titleIsPath),
		world.Def(`^a developer has a project with no capabilities folder$`, s.bareProject),
		world.Def(`^Givens opens it$`, s.openBare),
		world.Def(`^nothing is written to the project$`, s.nothingWritten),
		world.Def(`^a developer has opened a project with no capabilities folder$`,
			s.bareProjectOpen),
		world.Def(`^the project gets a capabilities folder containing GIVENS\.md and `+
			`_shared/givens\.sh$`, s.gotCapabilitiesFolder),
		world.Def(`^it contains the starter capability "([^"]*)", with add-a-story\.feature, `+
			`its about\.md and setup scripts$`, s.gotStarter),
		world.Def(`^it contains \.gitattributes marking runs/ as merge=union$`, s.gotAttributes),
		world.Def(`^Givens lists "([^"]*)"$`, s.sidebarListsOne),
		world.Def(`^another tool adds a scenario to checkout\.feature$`, s.otherToolAdds),
		world.Def(`^"([^"]*)" appears in the ([A-Za-z ]+) scenarios within two seconds$`,
			s.appearsWithin),
		world.Def(`^"([^"]*)" stays selected$`, s.staysSelected),
	}
}

// shopWithFeatures copies the sample shop and checks it holds the named .feature files, so
// the scenario's starting state is what its step says.
func (s *state) shopWithFeatures(list string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	names := strings.Split(strings.ReplaceAll(list, " and ", ", "), ", ")
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(s.Capabilities(), name)); err != nil {
			return fmt.Errorf("the sample shop has no capabilities/%s: %w", name, err)
		}
	}
	return nil
}

// open opens path in this scenario's app and keeps the window it returns.
func (s *state) open(path string) error {
	if err := s.StartApp(); err != nil {
		return err
	}
	window, err := s.App.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	if window == nil {
		return fmt.Errorf("opening %s returned no window", path)
	}
	s.Window, s.opened = window, window
	return nil
}

func (s *state) openCapabilities() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.open(s.Capabilities())
}

// capabilityNames lists the capabilities the front window lists, in order: the TUI's
// Capabilities tab, and what givens list groups by.
func (s *state) capabilityNames() ([]string, error) {
	window, err := s.Front()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 8)
	for _, item := range window.Sidebar() {
		if item.Kind == "capability" {
			names = append(names, item.Name)
		}
	}
	return names, nil
}

func (s *state) sidebarLists(want []string) error {
	got, err := s.capabilityNames()
	if err != nil {
		return err
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("Givens lists capabilities %q, not %q", got, want)
	}
	return nil
}

func (s *state) sidebarListsThree(first string, second string, third string) error {
	return s.sidebarLists([]string{first, second, third})
}

func (s *state) sidebarListsOne(name string) error {
	return s.sidebarLists([]string{name})
}

// countsShown compares each capability's count with its .feature file, and expects the TUI's
// list to show it as "Checkout (5)".
func (s *state) countsShown() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	screen := screenOf(window)
	shown := 0
	for _, item := range window.Sidebar() {
		if item.Kind != "capability" {
			continue
		}
		path, err := featureNamed(s.Capabilities(), item.Name)
		if err != nil {
			return err
		}
		scenarios, err := app.ScenariosIn(path)
		if err != nil {
			return err
		}
		if item.Count != len(scenarios) {
			return fmt.Errorf("%q shows %d scenarios; its .feature file has %d",
				item.Name, item.Count, len(scenarios))
		}
		heading := fmt.Sprintf("%s (%d)", item.Name, item.Count)
		if !strings.Contains(screen, heading) {
			return fmt.Errorf("the TUI doesn't show %q; it shows:\n%s", heading, screen)
		}
		shown++
	}
	if shown == 0 {
		return errors.New("Givens lists no capabilities")
	}
	return nil
}

// storyShown expects the As a, I want and So that lines of the named capability in the
// window, and in the TUI beside the list, where they show while the cursor is on it.
func (s *state) storyShown(name string) error {
	path, err := featureNamed(s.Capabilities(), name)
	if err != nil {
		return err
	}
	story, err := storyOf(path)
	if err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	screen := screenOf(window)
	for _, line := range story {
		if err := world.Expect(window.Visible(), line, "the window"); err != nil {
			return err
		}
		if !strings.Contains(screen, line) {
			return fmt.Errorf("the TUI doesn't show %q; it shows:\n%s", line, screen)
		}
	}
	return nil
}

// screenOf is what the TUI draws first over window, as plain text: what bare givens shows.
func screenOf(window *app.Window) string {
	return tui.New(window, nil).Text()
}

// featureNamed finds the .feature file under capabilities whose Feature: line is name.
func featureNamed(capabilities string, name string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(capabilities, "*", "*.feature"))
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		text, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if slices.Contains(strings.Split(string(text), "\n"), "Feature: "+name) {
			return path, nil
		}
	}
	return "", fmt.Errorf("no .feature file in %s has the line %q", capabilities, "Feature: "+name)
}

// storyOf reads the As a, I want and So that lines of a .feature file.
func storyOf(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	story := make([]string, 0, 3)
	// Scanner reads line by line. Ref: https://pkg.go.dev/bufio#Scanner
	scanner := bufio.NewScanner(file)
	for scanner.Scan() { // bound: the file's lines
		line := strings.TrimSpace(scanner.Text())
		for _, start := range []string{"As a", "I want", "So that"} {
			if strings.HasPrefix(line, start) {
				story = append(story, line)
			}
		}
	}
	if len(story) != 3 {
		return nil, fmt.Errorf("%s has story lines %q, not three", path, story)
	}
	return story, scanner.Err()
}
