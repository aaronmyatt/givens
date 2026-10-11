package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/runs"
	"givens/internal/slug"
)

// Rename renames a scenario or a capability by its name. When the rename would start runs
// over, it asks first, offering "Rename" and "Cancel"; otherwise it renames at once: the
// Scenario: or Feature: line, the setup files or folder, and every from line that names it.
// Needed by the rename scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Rename(name string, newName string) error {
	assert.That(w != nil, "rename runs on a window")
	assert.That(name != "", "rename names what to rename")
	assert.That(newName != "", "rename names the new name")
	assert.That(len(newName) <= NameLengthMax, "the new name fits NameLengthMax")
	if slug.Of(newName) == "" {
		return errors.New("a name needs letters or digits")
	}
	if _, ok := w.findCapability(name); ok {
		return ErrNotImplemented
	}
	scenario, ok := w.findScenario(name)
	if !ok {
		return fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	rename := func() error { return w.renameScenario(scenario, newName) }
	recorded, err := runs.Read(w.path)
	if err != nil {
		return err
	}
	for _, run := range recorded {
		if run.Key == scenario.Key() || strings.HasPrefix(run.Key, scenario.Key()+"/") {
			w.ask("Renaming starts "+scenario.Name+"'s runs over", "Rename", rename)
			return nil
		}
	}
	return rename()
}

// renameScenario rewrites the scenario's Scenario: line, then moves its setup files and
// updates from lines, unless another scenario still makes the old slug.
func (w *Window) renameScenario(scenario project.Scenario, newName string) error {
	assert.That(scenario.Line > 0, "a loaded scenario has a line")
	assert.That(slug.Of(newName) != "", "the new name makes a slug")
	capability, ok := w.findCapability(scenario.Capability)
	if !ok {
		return fmt.Errorf("%s: %w", scenario.Capability, ErrNotFound)
	}
	if err := renameLine(capability.FeaturePath, scenario.Line, newName); err != nil {
		return err
	}
	newSlug := slug.Of(newName)
	if slugClash(capability, scenario) == "" && newSlug != scenario.Slug {
		if err := moveSetup(capability.Dir, scenario.Slug, newSlug); err != nil {
			return err
		}
		from := capability.Slug + "/" + scenario.Slug
		if err := w.updateFromLines(from, capability.Slug+"/"+newSlug); err != nil {
			return err
		}
	}
	if w.selectedScenario == scenario.Capability+"/"+scenario.Name {
		w.selected, w.selectedScenario = newName, scenario.Capability+"/"+newName
	}
	return w.reload()
}

// renameLine replaces the name after the keyword on one line of a .feature file, keeping the
// rest of the file byte for byte.
func renameLine(path string, line int64, newName string) error {
	assert.That(line > 0, "lines count from 1")
	assert.That(!strings.Contains(newName, "\n"), "a name is one line")
	text, err := readEditable(path)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	if line > int64(len(lines)) {
		return fmt.Errorf("%s has no line %d", filepath.Base(path), line)
	}
	keyword, _, ok := strings.Cut(lines[line-1], ":")
	if !ok {
		return fmt.Errorf("%s line %d has no keyword", filepath.Base(path), line)
	}
	lines[line-1] = keyword + ": " + newName
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// moveSetup renames scripts/<from>.sh, .md and .teardown.sh to <to>, leaving any that would
// overwrite a file.
func moveSetup(dir string, from string, to string) error {
	assert.That(from != "" && to != "", "both slugs are given")
	assert.That(from != to, "a move changes the name")
	for _, suffix := range []string{".sh", ".md", ".teardown.sh"} {
		source := filepath.Join(dir, "scripts", from+suffix)
		target := filepath.Join(dir, "scripts", to+suffix)
		if _, err := os.Stat(source); err != nil {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
	}
	return nil
}

// updateFromLines changes every `from <old>` line in the project's setup scripts to name
// replacement, keeping any row values after it. Both are run keys.
func (w *Window) updateFromLines(old string, replacement string) error {
	assert.That(w.project != nil, "from lines live in a loaded project")
	assert.That(strings.Count(old, "/") == 1, "old is a run key")
	assert.That(strings.Count(replacement, "/") == 1, "replacement is a run key")
	pattern := regexp.MustCompile(`(?m)^(\s*from\s+)` + regexp.QuoteMeta(old) + `(\s|$)`)
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			if scenario.Setup == "" {
				continue
			}
			text, err := readEditable(scenario.Setup)
			if err != nil {
				return err
			}
			updated := pattern.ReplaceAllString(text, "${1}"+replacement+"${2}")
			if updated == text {
				continue
			}
			if err := os.WriteFile(scenario.Setup, []byte(updated), 0o755); err != nil {
				return err
			}
		}
	}
	return nil
}
