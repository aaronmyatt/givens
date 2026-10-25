package app

import (
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
	"givens/internal/spec"
)

// fromProblem finds what's wrong with the setups a scenario builds on: a from line naming no
// scenario, an outline named without a row, or setups that build on each other in a loop.
func (w *Window) fromProblem(scenario project.Scenario) (string, error) {
	assert.That(w.project != nil, "from lines are read in a loaded project")
	assert.That(scenario.Setup != "", "the scenario has a setup script")
	type visit struct {
		scenario project.Scenario
		path     []string // slugs from the scenario dropped in to
	}
	stack := []visit{{scenario: scenario, path: []string{scenario.Slug}}}
	visits := 0
	// A loop is reported the first time a slug repeats on a path.
	// bound: visits stop at the most scenarios a project can hold.
	for len(stack) > 0 && visits <= spec.CapabilitiesMax*spec.ScenariosPerFeatureMax {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visits++
		if current.scenario.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.scenario.Setup)
		if err != nil {
			return "", err
		}
		for _, from := range helpers.Froms(text) {
			built, err := w.project.Find(from.Key)
			if err != nil {
				return "from " + from.Key + ": there's no scenario with that name", nil
			}
			if problem := outlineWithoutRow(built, from.Values); problem != "" {
				return problem, nil
			}
			path := append(append([]string{}, current.path...), built.Slug)
			if slices.Contains(current.path, built.Slug) {
				return "These setups build on each other in a loop: " +
					strings.Join(path, " → "), nil
			}
			stack = append(stack, visit{scenario: built, path: path})
		}
	}
	return "", nil
}

// outlineWithoutRow says an outline was named without a row to build on, or "".
func outlineWithoutRow(built project.Scenario, values []string) string {
	assert.That(built.Slug != "", "the scenario has a slug")
	assert.That(len(values) <= helpers.HelpersMax, "values stay bounded")
	if !built.Outline || len(values) > 0 {
		return ""
	}
	example := ""
	for _, block := range built.Examples {
		if len(block.Header) > 0 && len(block.Rows) > 0 && example == "" {
			example = ", like " + block.Header[0] + "=" + block.Rows[0][0]
		}
	}
	return built.Slug + " is an outline: pick a row" + example
}
