// Package readacapability runs capabilities/read-a-capability. Steps shared with other
// capabilities are in acceptance/world.
package readacapability

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
)

const capability = "read-a-capability"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	scenario string // the scenario a step named, for steps that say "its"
	// The about.md paragraphs written around a missing image, for the placeholder's place.
	textBeforeImage, textAfterImage string
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return append(s.historySteps(),
		world.Def(`^they see "([^"]*)", "([^"]*)" and "([^"]*)"$`, s.seeStory),
		world.Def(`^after the scenarios, about\.md is shown with the checkout flow diagram$`,
			s.aboutAfterScenarios),
		world.Def(`^its (\d+) scenarios are listed, each marked as a happy path or an error `+
			`state$`, s.scenariosListed),
		world.Def(`^they select the "([^"]*)" scenario$`, s.selectScenario),
		world.Def(`^its steps are shown (?:with their keywords lined up|as normal)$`,
			s.stepsLinedUp),
		world.Def(`^its tags, (\S+) and (\S+), are shown above its steps$`, s.tagsAboveSteps),
		world.Def(`^its setup script is shown with its steps$`, s.setupShown),
		world.Def(`^the "([^"]*)" scenario has no setup yet$`, s.noSetup),
		world.Def(`^in place of a setup they see "([^"]*)"$`, s.noSetupShown),
		world.Def(`^Checkout's about\.md shows an image that isn't on disk$`, s.aboutImageMissing),
		world.Def(`^the rest of the story is shown as normal$`, s.restOfStoryShown),
		world.Def(`^where the image would be there's a placeholder naming "([^"]*)"$`,
			s.placeholderShown),
	)
}

// diskScenario is a scenario as checkout.feature defines it, read without Givens.
type diskScenario struct {
	Name  string
	Error bool
	Tags  []string
	Steps []string // keyword and text: "Given a signed-in shopper with one item in the cart"
}

// checkoutScenarios parses checkout.feature with the Gherkin parser, through app.ScenariosIn.
func (s *state) checkoutScenarios() ([]diskScenario, error) {
	feature := filepath.Join(s.Capabilities(), "checkout", "checkout.feature")
	scenarios, err := app.ScenariosIn(feature)
	if err != nil {
		return nil, err
	}
	found := make([]diskScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		each := diskScenario{Name: scenario.Name}
		for _, tag := range scenario.Tags {
			each.Tags = append(each.Tags, tag.Name)
			each.Error = each.Error || tag.Name == "@error"
		}
		for _, step := range scenario.Steps {
			// The parser keeps the space after a keyword: "Given ". Ref:
			// https://pkg.go.dev/github.com/cucumber/messages/go/v34#Step
			each.Steps = append(each.Steps, step.Keyword+step.Text)
		}
		found = append(found, each)
	}
	return found, nil
}

func (s *state) checkoutScenario(name string) (diskScenario, error) {
	scenarios, err := s.checkoutScenarios()
	if err != nil {
		return diskScenario{}, err
	}
	for _, scenario := range scenarios {
		if scenario.Name == name {
			return scenario, nil
		}
	}
	return diskScenario{}, fmt.Errorf("checkout.feature has no scenario %q", name)
}

// linedUp is how a scenario's steps read when it's selected: keywords right-aligned to the
// widest one, so every step's text starts in the same column.
func linedUp(steps []string) []string {
	width := 0
	for _, step := range steps {
		width = max(width, len(strings.Fields(step)[0]))
	}
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		keyword, text, _ := strings.Cut(step, " ")
		lines = append(lines, strings.Repeat(" ", width-len(keyword))+keyword+" "+text)
	}
	return lines
}

// visibleLines is everything the window shows, one line per entry, so a block of text shown
// as one piece can still be searched line by line.
func visibleLines(window *app.Window) []string {
	visible := window.Visible()
	lines := make([]string, 0, len(visible))
	for _, entry := range visible {
		lines = append(lines, strings.Split(entry, "\n")...)
	}
	return lines
}

// indexContaining returns the first line at or after from that contains text, or -1.
func indexContaining(lines []string, text string, from int) int {
	for index := from; index < len(lines); index++ {
		if strings.Contains(lines[index], text) {
			return index
		}
	}
	return -1
}

func (s *state) selectedCapability() (*app.Window, app.Capability, error) {
	window, err := s.Front()
	if err != nil {
		return nil, app.Capability{}, err
	}
	name := window.Selected()
	if name == "" {
		return nil, app.Capability{}, errors.New("nothing is selected")
	}
	capability, err := window.Capability(name)
	return window, capability, err
}

func (s *state) seeStory(asA string, iWant string, soThat string) error {
	window, capability, err := s.selectedCapability()
	if err != nil {
		return err
	}
	want := []string{asA, iWant, soThat}
	if !slices.Equal(capability.Story, want) {
		return fmt.Errorf("%s's story is %q, not %q", capability.Name, capability.Story, want)
	}
	for _, line := range want {
		if err := world.Expect(visibleLines(window), line, "the window"); err != nil {
			return err
		}
	}
	return nil
}

// selectScenario selects a scenario, giving it a setup script first when the sample shop has
// none, so there's a setup to show with its steps.
func (s *state) selectScenario(name string) error {
	s.scenario = name
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.ensureSetup(name); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Select(name)
}

func (s *state) ensureSetup(name string) error {
	scripts := filepath.Join("capabilities", "checkout", "scripts")
	for _, extension := range []string{".sh", ".md"} {
		_, err := os.Stat(filepath.Join(s.Project, scripts, app.SlugOf(name)+extension))
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	setup := world.KitSource + "\nseed_shopper\nseed_cart 1\nmock_payments --decline\n"
	return s.WriteFile(filepath.Join(scripts, app.SlugOf(name)+".sh"), setup)
}

func (s *state) shownScenario() (*app.Window, app.Scenario, diskScenario, error) {
	window, err := s.Front()
	if err != nil {
		return nil, app.Scenario{}, diskScenario{}, err
	}
	name := s.scenario
	if name == "" {
		name = window.Selected()
	}
	onDisk, err := s.checkoutScenario(name)
	if err != nil {
		return nil, app.Scenario{}, diskScenario{}, err
	}
	shown, err := window.Scenario(name)
	return window, shown, onDisk, err
}

func (s *state) scenariosListed(count string) error {
	want, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	_, capability, err := s.selectedCapability()
	if err != nil {
		return err
	}
	onDisk, err := s.checkoutScenarios()
	if err != nil {
		return err
	}
	if len(onDisk) != want || len(capability.Scenarios) != want {
		return fmt.Errorf("%s lists %d scenarios and its file has %d, not %d",
			capability.Name, len(capability.Scenarios), len(onDisk), want)
	}
	kinds := map[bool]int{}
	for index, scenario := range onDisk {
		shown := capability.Scenarios[index]
		if shown.Name != scenario.Name || shown.Error != scenario.Error {
			return fmt.Errorf("scenario %d is %q (error state: %t), not %q (error state: %t)",
				index+1, shown.Name, shown.Error, scenario.Name, scenario.Error)
		}
		kinds[scenario.Error]++
	}
	if kinds[true] == 0 || kinds[false] == 0 {
		return fmt.Errorf("checkout.feature should hold both kinds of scenario: %v", kinds)
	}
	return nil
}
