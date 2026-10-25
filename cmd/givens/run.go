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

	"givens/internal/app"
	"givens/internal/assert"
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
	loaded, err := app.LoadProject(root)
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
			return exitProblems
		}
	}
	if scenario.SetupCards != "" {
		return walkSteps(root, scenario.SetupCards, os.Stdin)
	}
	return exitOK
}

// runScript runs a setup script with bash from the project's folder, with the terminal as
// its stdin, stdout and stderr, and returns its exit code: 1 when it couldn't run.
// Ref: https://pkg.go.dev/os/exec#Cmd
func runScript(root string, script string, env ...string) int {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(filepath.IsAbs(script), "the script is absolute")
	command := exec.Command("bash", script)
	command.Dir = filepath.Dir(root)
	command.Env = append(os.Environ(), env...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		complain(err)
		return exitProblems
	}
	return exitOK
}

// walkSteps prints a markdown setup's steps one at a time as plain text, each code block on
// its own lines. A step with "bash run" or "js run" blocks runs them and moves on; any other
// waits for Enter, or f if it failed.
func walkSteps(root string, path string, input io.Reader) int {
	assert.That(filepath.IsAbs(path), "the setup is absolute")
	assert.That(input != nil, "answers come from somewhere")
	text, err := app.ReadScript(path)
	if err != nil {
		complain(err)
		return exitProblems
	}
	setup, err := app.ParseMarkdownSetup(text)
	if err != nil {
		complain(err)
		return exitProblems
	}
	if setup.Intro != "" {
		fmt.Printf("%s\n\n", app.PlainMarkdown(setup.Intro))
	}
	answers := bufio.NewReaderSize(input, answerBytesMax)
	for _, step := range setup.Steps {
		printStep(step)
		ran, code := runBlocks(root, path, step)
		if code != exitOK {
			fmt.Printf("✗ Stopped at step %d of %d (exit %d)\n", step.Number, len(setup.Steps),
				code)
			return exitProblems
		}
		if ran {
			continue
		}
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

// runBlocks runs a step's run blocks in order, as Drop in does, and says whether it had any
// and the exit code of the first that failed.
func runBlocks(root string, path string, step app.MarkdownStep) (bool, int) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(strings.HasSuffix(path, ".md"), "a markdown setup is read")
	ran := false
	for index, block := range step.Blocks {
		language := app.BlockLanguage(block.Language, block.Run, block.Closed)
		if language == "" {
			continue
		}
		ran = true
		dir, err := os.MkdirTemp("", "givens-run-")
		if err != nil {
			complain(err)
			return ran, exitProblems
		}
		script, err := app.WriteRunBlock(dir, fmt.Sprintf("step-%d-block-%d", step.Number,
			index), filepath.Join(root, "_shared", "givens.sh"), language, block.Code)
		code := exitProblems
		if err == nil {
			stands := strings.TrimSuffix(path, ".md") + ".sh"
			code = runScript(root, script, app.ScriptAsEnv+"="+stands)
		}
		_ = os.RemoveAll(dir)
		if code != exitOK {
			return ran, code
		}
	}
	return ran, exitOK
}

// printStep prints one step: its number and plain text, then each code block exactly as it
// should be pasted, set apart by blank lines.
func printStep(step app.MarkdownStep) {
	assert.That(step.Number > 0, "steps count from 1")
	assert.That(len(step.Blocks) <= app.MarkdownStepsMax, "blocks stay bounded")
	fmt.Printf("\n%d. %s\n", step.Number, app.PlainMarkdown(step.Text))
	for _, block := range step.Blocks {
		fmt.Printf("\n%s\n\n", block.Code)
	}
}
