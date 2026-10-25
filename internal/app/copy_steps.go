package app

import (
	"strings"

	"givens/internal/assert"
)

// OfferCopySteps copies the scenario on show as Gherkin text. Needed by "Copy a scenario's
// steps" in capabilities/read-a-capability.
const OfferCopySteps = "Copy steps"

// gherkinOf is a scenario as Gherkin text, for "Copy steps": its tags, its Scenario: line, then
// its steps indented. An outline, whose steps hold <placeholders>, is a Scenario Outline.
// Ref: https://cucumber.io/docs/gherkin/reference/
func gherkinOf(scenario Scenario) string {
	assert.That(scenario.Name != "", "a scenario has a name")
	assert.That(len(scenario.Steps) <= NameLengthMax, "steps stay bounded")
	outline := false
	for _, step := range scenario.Steps {
		outline = outline || (strings.Contains(step, "<") && strings.Contains(step, ">"))
	}
	lines := make([]string, 0, len(scenario.Steps)+2)
	if len(scenario.Tags) > 0 {
		lines = append(lines, strings.Join(scenario.Tags, " "))
	}
	keyword := "Scenario: "
	if outline {
		keyword = "Scenario Outline: "
	}
	lines = append(lines, keyword+scenario.Name)
	for _, step := range scenario.Steps {
		lines = append(lines, "  "+step)
	}
	return strings.Join(lines, "\n") + "\n"
}
