package dropintoascenario

import (
	"fmt"
	"slices"
	"strings"

	"givens/acceptance/world"
)

// backAndForth is what "goes When, Then, When, Then" adds to the end of a scenario: a second
// When and the Then that checks it.
func backAndForth() []string {
	return []string{`    When they choose "Try another card"`, `    Then the card form opens again`}
}

// sequenceSteps cover steps kept in the order they're written.
func sequenceSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^"([^"]*)" goes When, Then, When, Then$`, s.goesBackAndForth),
		world.Def(`^the checklist is in the order the steps are written$`, s.checklistInOrder),
		world.Def(`^the Drop in page shows a When part, a Then part, a When part and a Then `+
			`part$`, s.partsInOrder),
	}
}

// goesBackAndForth adds a second When and Then to the end of the named checkout scenario, and
// gives it a setup that finishes at once.
func (s *state) goesBackAndForth(name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	start := slices.Index(lines, "  Scenario: "+name)
	if start < 0 {
		return fmt.Errorf("checkout.feature has no scenario %q", name)
	}
	end := start + 1
	// bound: end only grows, and stops at the last line.
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		end++
	}
	lines = slices.Insert(lines, end, backAndForth()...)
	if err := s.WriteFile(checkoutFeature, strings.Join(lines, "\n")); err != nil {
		return err
	}
	s.scenario = name
	if err := s.writeSetup(name, quickSetup()); err != nil {
		return err
	}
	return s.prepare()
}

// checklistInOrder checks the checklist against Card declined's steps as written, the added
// When and Then last.
func (s *state) checklistInOrder() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	want := []string{
		"when: When they place the order",
		`then: Then they see "Your card was declined"`,
		"then: And the order stays pending",
		"when: " + strings.TrimSpace(backAndForth()[0]),
		"then: " + strings.TrimSpace(backAndForth()[1]),
	}
	got := make([]string, 0, len(want))
	for _, item := range session.Checklist() {
		got = append(got, item.Kind+": "+item.Text)
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("the checklist is\n%s\nnot\n%s", strings.Join(got, "\n"),
			strings.Join(want, "\n"))
	}
	return nil
}

func (s *state) partsInOrder() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	kinds := make([]string, 0, 4)
	for _, part := range session.Parts() {
		kinds = append(kinds, part.Kind)
	}
	if want := []string{"when", "then", "when", "then"}; !slices.Equal(kinds, want) {
		return fmt.Errorf("the Drop in page's parts are %q, not %q", kinds, want)
	}
	return nil
}
