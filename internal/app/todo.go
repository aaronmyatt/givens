package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/project"
)

// Offers that flag a scenario to do, or take the flag off. See capabilities/tags.
const (
	OfferFlagTodo   = "Flag as to do"
	OfferRemoveTodo = "Remove to do flag"
)

// todoOffers is the flag offer a scenario's page shows. A @todo from its Rule or Feature line
// isn't offered for removal: taking it off would unflag every scenario under that line.
func todoOffers(loaded project.Scenario) []string {
	assert.That(loaded.Name != "", "a shown scenario has a name")
	assert.That(len(loaded.OwnTags) <= len(loaded.Tags), "own tags are among its tags")
	switch {
	case !loaded.Todo:
		return []string{OfferFlagTodo}
	case slices.Contains(loaded.OwnTags, project.TagTodo):
		return []string{OfferRemoveTodo}
	}
	return []string{}
}

// flagTodo adds @todo to a scenario's own tags, or takes it off, in its .feature file. Only
// the tag lines above its Scenario: line change, so git diff shows one line.
// Needed by "Flag a scenario to do" and "Remove a to do flag" in capabilities/tags.
func (w *Window) flagTodo(scenario project.Scenario, flagged bool) error {
	assert.That(w != nil, "flag runs on a window")
	assert.That(scenario.Line > 0, "a loaded scenario has a line")
	capability, ok := w.findCapability(scenario.Capability)
	if !ok {
		return fmt.Errorf("%s: %w", scenario.Capability, ErrNotFound)
	}
	text, err := readEditable(capability.FeaturePath)
	if err != nil {
		return err
	}
	lines, err := withTodo(strings.Split(text, "\n"), int(scenario.Line-1), flagged)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(capability.FeaturePath), err)
	}
	if err := os.WriteFile(capability.FeaturePath, []byte(strings.Join(lines, "\n")),
		0o644); err != nil {
		return err
	}
	return w.reload()
}

// withTodo flags or unflags the scenario whose Scenario: line is lines[at] (counting from 0).
// Gherkin tags sit on the lines just above the keyword they apply to.
// Ref: https://cucumber.io/docs/cucumber/api/#tags
func withTodo(lines []string, at int, flagged bool) ([]string, error) {
	assert.That(at >= 0, "lines count from 0")
	assert.That(len(lines) <= EditorTextBytesMax, "lines stay bounded")
	if at >= len(lines) || !strings.Contains(lines[at], "Scenario") {
		return nil, fmt.Errorf("line %d isn't a Scenario: line", at+1)
	}
	top := at
	// bound: top counts down to 0 at most.
	for top > 0 && strings.HasPrefix(strings.TrimSpace(lines[top-1]), "@") {
		top--
	}
	if flagged {
		if top == at {
			indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " \t"))]
			return slices.Insert(lines, at, indent+project.TagTodo), nil
		}
		lines[at-1] += " " + project.TagTodo
		return lines, nil
	}
	kept := make([]string, 0, len(lines))
	for index, line := range lines {
		fields := strings.Fields(line)
		if index < top || index >= at || !slices.Contains(fields, project.TagTodo) {
			kept = append(kept, line)
			continue
		}
		// A tag line left with no tags goes; one with others keeps them, in order.
		fields = slices.DeleteFunc(fields, func(field string) bool {
			return field == project.TagTodo
		})
		if len(fields) > 0 {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			kept = append(kept, indent+strings.Join(fields, " "))
		}
	}
	if slices.Equal(kept, lines) {
		return nil, fmt.Errorf("line %d's own tags have no %s", at+1, project.TagTodo)
	}
	assert.That(len(kept) <= len(lines), "unflagging adds no line")
	return kept, nil
}
