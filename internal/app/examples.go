package app

import (
	"givens/internal/assert"
	"givens/internal/project"
)

// MessageNameExamples is the warning on an outline whose Examples blocks can't be told apart.
const MessageNameExamples = "Name each Examples block so runs can tell them apart"

// examplesWarning is the warning on an outline with more than one unnamed Examples block: runs
// are recorded against the block's name, so unnamed blocks would share them.
func examplesWarning(scenario project.Scenario) string {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Examples) <= project.HelperFilesMax, "blocks stay bounded")
	unnamed := 0
	for _, block := range scenario.Examples {
		if block.Slug == "" {
			unnamed++
		}
	}
	if unnamed > 1 {
		return MessageNameExamples
	}
	return ""
}
