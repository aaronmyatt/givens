package app

import (
	"path/filepath"
	"strings"

	"givens/internal/assert"
)

// Preview is what "Preview" shows for the editor's text, unsaved changes included: a
// scenario as it reads in the library, keywords lined up, with any line that isn't a step
// marked with the parser's message; or about.md as the library renders it. Needed by the
// preview scenarios in capabilities/write-stories-and-scenarios.
func (e *Editor) Preview() []PreviewLine {
	assert.That(e != nil, "preview runs on an editor")
	assert.That(e.window != nil, "an editor belongs to a window")
	if filepath.Base(e.path) == "about.md" {
		rendered := withImagePlaceholders(e.text, filepath.Dir(e.path))
		lines := make([]PreviewLine, 0, 16)
		for _, line := range strings.Split(rendered, "\n") {
			lines = append(lines, PreviewLine{Text: line})
		}
		return lines
	}
	if e.scenario == "" {
		return []PreviewLine{}
	}
	return previewScenario(strings.Split(e.text, "\n"), e.scenario)
}

// previewScenario shows one scenario's block from the editor's lines: its name, then its
// steps lined up. A line that isn't a step, a tag or a comment is marked.
func previewScenario(lines []string, name string) []PreviewLine {
	assert.That(name != "", "a scenario has a name")
	assert.That(len(lines) <= EditorTextBytesMax, "lines stay bounded")
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Scenario: "+name || trimmed == "Scenario Outline: "+name {
			start = index
		}
	}
	if start < 0 {
		return []PreviewLine{}
	}
	preview := []PreviewLine{{Text: strings.TrimSpace(lines[start])}}
	steps, marked := make([]string, 0, 8), make(map[int]string, 1)
	for _, line := range lines[start+1 : blockEnd(lines, start)] {
		trimmed := strings.TrimSpace(line)
		keyword, _, _ := strings.Cut(trimmed, " ")
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case isStepKeyword(keyword):
			steps = append(steps, trimmed)
		default:
			marked[len(steps)] = trimmed
			steps = append(steps, trimmed)
		}
	}
	plain := make([]string, 0, len(steps))
	for index, step := range steps {
		if _, bad := marked[index]; !bad {
			plain = append(plain, step)
		}
	}
	aligned := linedUp(plain)
	for index, step := range steps {
		if text, bad := marked[index]; bad {
			preview = append(preview, PreviewLine{Text: text, Mark: MessageNotAStep})
			continue
		}
		preview = append(preview, PreviewLine{Text: aligned[0]})
		aligned = aligned[1:]
		assert.That(step != "", "a step has text")
	}
	return preview
}

// MessageNotAStep marks a line among a scenario's steps that the parser can't read.
const MessageNotAStep = "Expected a step, a tag, a comment or a new scenario"

// isStepKeyword is true for a word that starts a step.
func isStepKeyword(word string) bool {
	assert.That(!strings.Contains(word, " "), "a keyword is one word")
	assert.That(len(word) <= NameLengthMax, "a word stays bounded")
	for _, keyword := range stepKeywords() {
		if word == keyword {
			return true
		}
	}
	return false
}
