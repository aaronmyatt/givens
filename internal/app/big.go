// The big file: Givens's behaviour, told in the order of its stories.
//
// Each story in capabilities/ that's moved here has a section, in the order of the
// capabilities folders. A section starts with a "// Feature: <name>" comment, the Feature's
// name word for word, and runs to the next one. Inside it, each of the Feature's Rules has a
// "// Rule: <name>" comment, in the Feature's order, and a "// Scenario: <name>" comment marks
// the code a scenario starts in where that helps a reader; several can share one piece of code.
// Helpers that every story uses sit below all the sections, after "// Helpers".
//
// TestBigFile in acceptance/ checks those comments against the feature files, and
// scripts/entry.sh finds the section for a Feature, Rule or Scenario name. The workflow is
// tasks/one-big-file/essay.md; the pilot is tasks/one-big-file/notes.md. The rest of
// internal/app isn't here yet.
//
// Code outside this file that a story depends on, and why it's elsewhere:
//   - internal/kit/givens.sh: bash, run in the developer's shell by every setup script.

package app

import (
	"errors"
	"os"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// Feature: Copy a setup script
//
// capabilities/copy-a-setup-script. A scenario's setup goes on the clipboard as a command to
// paste anywhere in the project. "Run from a subfolder" is the kit's: it moves to the repo's
// root before running anything (the cd after the helpers load, in internal/kit/givens.sh).

// Scenario: Copy
// Scenario: Capabilities folder isn't at the root
// Scenario: Copy steps done by hand

// copySetup puts a scenario's setup on the clipboard: `bash <path>.sh`, the path from the git
// repo's root, or for a setup done only by hand, its markdown.
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

// Scenario: Copy from search

// KeyCopySetup copies the selected search result's setup command.
const KeyCopySetup = "⇧⌘C"

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

// Helpers
//
// Below every story, so a section reads as what the story does, and the how comes after.

// repoDepthMax bounds the walk up from a capabilities folder to its git repo's root.
const repoDepthMax = 64

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
