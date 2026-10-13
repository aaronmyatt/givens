package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// KeyCopySetup copies the selected search result's setup command.
const KeyCopySetup = "⇧⌘C"

// repoDepthMax bounds the walk up from a capabilities folder to its git repo's root.
const repoDepthMax = 64

// copySetup puts a scenario's setup on the clipboard: `bash <path>.sh`, the path from the git
// repo's root, or for a setup done only by hand, its markdown. See
// capabilities/copy-a-setup-script.
func (w *Window) copySetup(scenario project.Scenario) error {
	assert.That(w != nil, "copy runs on a window")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	clipboard := w.app.config.Clipboard
	if scenario.Setup == "" && scenario.SetupCards != "" {
		text, err := helpers.ReadScript(scenario.SetupCards)
		if err != nil {
			return err
		}
		return clipboard.WriteText(text)
	}
	if scenario.Setup == "" {
		return errors.New(scenario.Name + " has no setup to copy")
	}
	relative, err := filepath.Rel(repoRoot(w.path), scenario.Setup)
	if err != nil {
		return err
	}
	return clipboard.WriteText("bash " + filepath.ToSlash(relative))
}

// repoRoot is the root of the git repository holding capabilities, or, outside one, the
// folder holding capabilities. The kit moves there before running anything.
func repoRoot(capabilities string) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(repoDepthMax > 0, "the walk up has a limit")
	dir := filepath.Dir(capabilities)
	for depth := 0; depth < repoDepthMax; depth++ {
		// .git is a folder in a clone and a file in a worktree.
		// Ref: https://git-scm.com/docs/gitrepository-layout
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(capabilities)
}

// copyFromSearch copies the setup of the scenario first among the search's results.
func (w *Window) copyFromSearch() error {
	assert.That(w != nil, "copy from search runs on a window")
	assert.That(w.search != nil, "search is open")
	for _, group := range w.SearchResults().Groups {
		if group.Kind != SearchScenarios || len(group.Results) == 0 {
			continue
		}
		first := group.Results[0]
		found, ok := w.findScenario(first.Capability + "/" + first.Name)
		if !ok {
			return errors.New(first.Name + ": " + ErrNotFound.Error())
		}
		return w.copySetup(found)
	}
	return errors.New("no scenario is listed to copy")
}

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
