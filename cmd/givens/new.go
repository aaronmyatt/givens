package main

import (
	"errors"
	"fmt"
	"strings"

	"givens/internal/app"
	"givens/internal/assert"
	"givens/internal/termedit"
)

// The templates givens new opens in the developer's editor: the New story and New scenario
// dialogs, as forms. Lines starting with # are left out.
const (
	storyTemplate = `# A new story. Fill in each line, then save and quit. An empty line stops it.
Name: %s
As a: 
I want: 
So that: 
`
	scenarioTemplate = `# A new scenario in %s. Name it, then save and quit. It starts @todo.
Name: 
Error state: no
`
)

// runNew writes a new story or scenario from a template filled in in the developer's editor,
// through the app's NewStory and NewScenario. See capabilities/work-in-the-terminal.
func runNew(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 2 || (args[0] != "story" && args[0] != "scenario") {
		fmt.Println("usage: givens new story <name> | givens new scenario <capability>")
		return exitUsage
	}
	_, window, err := openProject()
	if err != nil {
		complain(err)
		return exitProblems
	}
	if args[0] == "story" {
		err = newStory(window, args[1])
	} else {
		err = newScenario(window, args[1])
	}
	if err != nil {
		complain(err)
		return exitProblems
	}
	return exitOK
}

// newStory fills in the story template, named after name, and writes the capability.
func newStory(window *app.Window, name string) error {
	assert.That(window != nil, "a story is written in a window")
	assert.That(name != "", "a story has a name")
	title := strings.ToUpper(name[:1]) + strings.ReplaceAll(name[1:], "-", " ")
	filled, err := termedit.Template("story.txt", fmt.Sprintf(storyTemplate, title))
	if err != nil {
		return err
	}
	fields := templateFields(filled)
	draft := app.StoryDraft{
		Name: fields["Name"], AsA: fields["As a"], IWant: fields["I want"],
		SoThat: fields["So that"],
	}
	if draft.Name == "" {
		return errors.New("no story was written: the template's Name was empty")
	}
	if err := window.NewStory(draft); err != nil {
		return err
	}
	fmt.Println("Created the story " + draft.Name)
	return nil
}

// newScenario fills in the scenario template and adds it to capability.
func newScenario(window *app.Window, capability string) error {
	assert.That(window != nil, "a scenario is written in a window")
	assert.That(capability != "", "a scenario goes in a capability")
	if err := window.Select(capability); err != nil {
		return err
	}
	filled, err := termedit.Template("scenario.txt", fmt.Sprintf(scenarioTemplate, capability))
	if err != nil {
		return err
	}
	fields := templateFields(filled)
	draft := app.ScenarioDraft{
		Name: fields["Name"], Error: strings.EqualFold(fields["Error state"], "yes"),
	}
	if draft.Name == "" {
		return errors.New("no scenario was written: the template's Name was empty")
	}
	if err := window.NewScenario(draft); err != nil {
		return err
	}
	fmt.Println("Created the scenario " + draft.Name + ", flagged @todo")
	return nil
}

// templateFields reads "Name: value" lines.
func templateFields(text string) map[string]string {
	assert.That(len(text) <= app.EditorTextBytesMax, "a template stays bounded")
	fields := make(map[string]string, 4)
	for _, line := range strings.Split(text, "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok {
			fields[strings.TrimSpace(name)] = strings.TrimSpace(value)
		}
	}
	assert.That(len(fields) <= strings.Count(text, ":"), "each field comes from a line")
	return fields
}
