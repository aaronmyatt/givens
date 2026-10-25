package readacapability

import (
	"fmt"
	"strings"

	"givens/acceptance/world"
)

// historySteps cover the runs listed for a scenario, and "Copy steps".
func (s *state) historySteps() []world.Step {
	return []world.Step{
		world.Def(`^one run is listed for it, "([^"]*)"$`, s.listsOneRun),
		world.Def(`^the clipboard holds "([^"]*)" as Gherkin, its Scenario: line then its `+
			`steps$`, s.clipboardHoldsGherkin),
	}
}

func (s *state) listsOneRun(text string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	for _, shown := range s.App.Snapshot().Windows {
		if shown.ID != window.ID() {
			continue
		}
		if len(shown.Runs) != 1 || !strings.HasPrefix(shown.Runs[0].Text, text) {
			return fmt.Errorf("the scenario's runs listed are %+v", shown.Runs)
		}
		return nil
	}
	return fmt.Errorf("no snapshot for window %s", window.ID())
}

// clipboardHoldsGherkin expects the Scenario: line, then each step from checkout.feature as
// written, indented, with its tags first if it has any.
func (s *state) clipboardHoldsGherkin(name string) error {
	onDisk, err := s.checkoutScenario(name)
	if err != nil {
		return err
	}
	lines := make([]string, 0, len(onDisk.Steps)+2)
	if len(onDisk.Tags) > 0 {
		lines = append(lines, strings.Join(onDisk.Tags, " "))
	}
	lines = append(lines, "Scenario: "+name)
	for _, step := range onDisk.Steps {
		lines = append(lines, "  "+step)
	}
	want := strings.Join(lines, "\n") + "\n"
	got, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("the clipboard holds %q, not %q", got, want)
	}
	return nil
}
