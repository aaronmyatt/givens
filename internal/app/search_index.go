package app

import (
	"path/filepath"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
)

// searchIndex holds what ⌘K looks through, read and lower-cased once: every distinct step
// with its uses, and every setup file's lines. It's built on the first search after the
// project's files change, so typing a query reads nothing from disk.
type searchIndex struct {
	steps   []indexedStep
	scripts []indexedScript
}

// indexedStep is one step text, as first written, with how many scenarios use it.
type indexedStep struct {
	name       string // "Given a signed-in shopper"
	lower      string // the text alone, lower case, which is what's searched
	capability string // the first capability that uses it
	uses       int
}

// indexedScript is one setup, teardown or setup steps file.
type indexedScript struct {
	capability string
	name       string // relative to the capabilities folder, like "checkout/scripts/x.sh"
	lowerName  string
	lines      []string // trimmed
	lowerLines []string
}

// searchMemo is the last query's results, so a snapshot, its messages and the grouped
// results asked for straight after all share one search.
type searchMemo struct {
	query   string
	filter  string
	results SearchResults
}

// searchIndexOf builds the index for the window's project.
func (w *Window) searchIndexOf() *searchIndex {
	assert.That(w.project != nil, "search runs in a loaded project")
	assert.That(filepath.IsAbs(w.path), "the capabilities folder is absolute")
	if w.index != nil {
		return w.index
	}
	index := &searchIndex{steps: make([]indexedStep, 0, 64), scripts: make([]indexedScript, 0, 32)}
	at := make(map[string]int, 64)
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, step := range uniqueSteps(scenario) {
				if seen, ok := at[step.Text]; ok {
					index.steps[seen].uses++
					continue
				}
				at[step.Text] = len(index.steps)
				index.steps = append(index.steps, indexedStep{
					name: step.Keyword + " " + step.Text, lower: strings.ToLower(step.Text),
					capability: capability.Name, uses: 1,
				})
			}
			for _, path := range []string{scenario.Setup, scenario.SetupCards,
				scenario.Teardown} {
				if script, ok := w.indexScript(capability.Name, path); ok {
					index.scripts = append(index.scripts, script)
				}
			}
		}
	}
	w.index = index
	return index
}

// indexScript reads one setup file into the index, or reports there's none to read.
func (w *Window) indexScript(capability string, path string) (indexedScript, bool) {
	assert.That(capability != "", "a script belongs to a capability")
	assert.That(path == "" || filepath.IsAbs(path), "a script's path is absolute")
	if path == "" {
		return indexedScript{}, false
	}
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return indexedScript{}, false
	}
	text, err := helpers.ReadScript(path)
	if err != nil {
		return indexedScript{}, false
	}
	name := filepath.ToSlash(relative)
	lines := strings.Split(text, "\n")
	script := indexedScript{
		capability: capability, name: name, lowerName: strings.ToLower(name),
		lines: make([]string, 0, len(lines)), lowerLines: make([]string, 0, len(lines)),
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		script.lines = append(script.lines, trimmed)
		script.lowerLines = append(script.lowerLines, strings.ToLower(trimmed))
	}
	return script, true
}
