package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/markdown"
	"givens/internal/project"
)

// answerBytesMax bounds one line typed at a markdown step.
const answerBytesMax = 4096

// runSetup runs one scenario's setup in the terminal: its script, with its "your turn" lines,
// then any markdown steps, one at a time.
func runSetup(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 1 {
		fmt.Println(usage)
		return exitUsage
	}
	root, err := findCapabilities(".")
	if err != nil {
		complain(err)
		return exitProblems
	}
	loaded, err := project.Load(root)
	if err != nil {
		complain(err)
		return exitProblems
	}
	scenario, err := loaded.Find(args[0])
	if err != nil {
		fmt.Println("There's no scenario " + args[0])
		return exitProblems
	}
	if scenario.Setup != "" {
		if code := runScript(root, scenario.Setup); code != exitOK {
			return code
		}
	}
	if scenario.SetupCards != "" {
		return walkSteps(scenario.SetupCards, os.Stdin)
	}
	return exitOK
}

// runScript runs a setup script with bash from the project's folder, with the terminal as
// its stdin, stdout and stderr, and returns its exit code.
// Ref: https://pkg.go.dev/os/exec#Cmd
func runScript(root string, script string) int {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(filepath.IsAbs(script), "the script is absolute")
	command := exec.Command("bash", script)
	command.Dir = filepath.Dir(root)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exitProblems
	}
	if err != nil {
		complain(err)
		return exitProblems
	}
	return exitOK
}

// walkSteps prints a markdown setup's steps one at a time as plain text, each code block on
// its own lines, and waits for Enter after each, or f if it failed.
func walkSteps(path string, input io.Reader) int {
	assert.That(filepath.IsAbs(path), "the setup is absolute")
	assert.That(input != nil, "answers come from somewhere")
	text, err := helpers.ReadScript(path)
	if err != nil {
		complain(err)
		return exitProblems
	}
	setup, err := markdown.Parse(text)
	if err != nil {
		complain(err)
		return exitProblems
	}
	if setup.Intro != "" {
		fmt.Printf("%s\n\n", markdown.Plain(setup.Intro))
	}
	answers := bufio.NewReaderSize(input, answerBytesMax)
	for _, step := range setup.Steps {
		printStep(step)
		fmt.Print("Press Enter when done, or type f if it failed ")
		answer, err := answers.ReadString('\n')
		if err != nil && answer == "" {
			fmt.Printf("\n✗ Stopped at step %d of %d: nobody answered\n", step.Number,
				len(setup.Steps))
			return exitProblems
		}
		if strings.TrimSpace(answer) == "f" {
			fmt.Printf("✗ Stopped at step %d of %d\n", step.Number, len(setup.Steps))
			return exitProblems
		}
	}
	return exitOK
}

// printStep prints one step: its number and plain text, then each code block exactly as it
// should be pasted, set apart by blank lines.
func printStep(step markdown.Step) {
	assert.That(step.Number > 0, "steps count from 1")
	assert.That(len(step.Blocks) <= markdown.StepsMax, "blocks stay bounded")
	fmt.Printf("\n%d. %s\n", step.Number, markdown.Plain(step.Text))
	for _, block := range step.Blocks {
		fmt.Printf("\n%s\n\n", block.Code)
	}
}
