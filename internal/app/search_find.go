package app

import (
	"sort"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// findStories finds capabilities whose name or a story line holds words.
func (w *Window) findStories(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	found := make([]SearchResult, 0, 4)
	for _, capability := range w.project.Capabilities {
		for _, line := range capability.Story {
			if !strings.Contains(strings.ToLower(line), words) {
				continue
			}
			found = append(found, SearchResult{
				Name: capability.Name, Detail: line, Capability: capability.Name,
				Matches: matchesIn(words, capability.Name, line),
			})
			break
		}
	}
	return limit(found)
}

// findScenarios finds scenarios matching the tag expression, whose names hold the words.
func (w *Window) findScenarios(read query) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(read.problem == "", "the query parsed")
	found := make([]SearchResult, 0, 8)
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if read.tagged && !read.expression.Match(scenario.Tags) {
				continue
			}
			if !strings.Contains(strings.ToLower(scenario.Name), read.words) {
				continue
			}
			found = append(found, SearchResult{
				Name: scenario.Name, Detail: capability.Name, Capability: capability.Name,
				Matches: matchesIn(read.words, scenario.Name, capability.Name),
			})
		}
	}
	return limit(found)
}

// findSteps finds steps holding the words, each step text once, with how many scenarios use
// it word for word.
func (w *Window) findSteps(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	found := make([]SearchResult, 0, 8)
	for _, step := range w.searchIndexOf().steps {
		if strings.Contains(step.lower, words) {
			found = append(found, SearchResult{
				Name: step.name, Capability: step.capability, Uses: step.uses,
				Matches: matchesIn(words, step.name, ""),
			})
		}
	}
	return limit(found)
}

// uniqueSteps is a scenario's steps, each text once, so a step it repeats counts as one use.
func uniqueSteps(scenario project.Scenario) []project.Step {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= helpers.HelpersMax, "steps stay bounded")
	steps := make([]project.Step, 0, len(scenario.Steps))
	seen := make(map[string]bool, len(scenario.Steps))
	for _, step := range scenario.Steps {
		if !seen[step.Text] {
			seen[step.Text] = true
			steps = append(steps, step)
		}
	}
	return steps
}

// findScripts finds setup files whose name or text holds the words: those named for them
// first, then those that only mention them, each with its first matching line.
func (w *Window) findScripts(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	named, mentioned := make([]SearchResult, 0, 8), make([]SearchResult, 0, 8)
	for _, script := range w.searchIndexOf().scripts {
		detail := ""
		for index, line := range script.lowerLines {
			if strings.Contains(line, words) {
				detail = script.lines[index]
				break
			}
		}
		isNamed := strings.Contains(script.lowerName, words)
		if detail == "" && !isNamed {
			continue
		}
		result := SearchResult{
			Name: script.name, Detail: detail, Capability: script.capability,
			Matches: matchesIn(words, script.name, detail),
		}
		if isNamed {
			named = append(named, result)
		} else {
			mentioned = append(mentioned, result)
		}
	}
	return limit(append(named, mentioned...))
}

// findHelpers finds documented helpers whose name or description holds the words.
func (w *Window) findHelpers(words string) []SearchResult {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(words != "", "there are words to find")
	found := make([]SearchResult, 0, 4)
	for _, helper := range w.Helpers() {
		shown := strings.ToLower(helper.Name + "\n" + helper.Description)
		if !strings.Contains(shown, words) {
			continue
		}
		found = append(found, SearchResult{
			Name: helper.Name, Detail: helper.Description, Capability: helper.Capability,
			Matches: matchesIn(words, helper.Name, helper.Description),
		})
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].Name < found[b].Name })
	return limit(found)
}

// limit keeps the first SearchResultsMax results.
func limit(found []SearchResult) []SearchResult {
	assert.That(found != nil, "results are a list")
	assert.That(SearchResultsMax > 0, "results have a limit")
	if len(found) > SearchResultsMax {
		return found[:SearchResultsMax]
	}
	return found
}
