package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"givens/internal/app"
	"givens/internal/assert"
	"givens/internal/tui"
)

// The TUI: bare `givens` in a terminal shows the project; `givens drop-in <key>` drops in.
// Without a terminal, there's no screen to draw, so it says what to use instead and exits 2.
// See capabilities/work-in-the-terminal.

// isTerminal is true when f is a terminal, not a pipe or a file.
// Ref: https://pkg.go.dev/io/fs#ModeCharDevice
func isTerminal(f *os.File) bool {
	assert.That(f != nil, "a file is looked at")
	info, err := f.Stat()
	if err != nil {
		return false
	}
	assert.That(info != nil, "a stat has a mode")
	return info.Mode()&os.ModeCharDevice != 0
}

// runTUI shows the project, or drops in to key when it's given.
func runTUI(key string, folder string) int {
	assert.That(key == "" || len(key) < 1<<10, "a key is short")
	assert.That(exitUsage != exitProblems, "usage and problems exit apart")
	if !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
		if key == "" && os.Getenv("GIVENS_TEST_SEAMS") == "1" && os.Getenv(testScreenEnv) == "1" {
			return printScreen(folder)
		}
		if key == "" {
			fmt.Println(usage)
		} else {
			fmt.Printf("givens drop-in needs a terminal; use givens run %s\n", key)
		}
		return exitUsage
	}
	assert.That(folder != "", "the TUI opens a folder")
	window, err := openForTUI(folder)
	if err != nil {
		complain(err)
		return exitProblems
	}
	model := tui.New(window, nil)
	if key != "" {
		if err := model.StartDropIn(key); err != nil {
			complain(err)
			return exitProblems
		}
	}
	// Ref: https://pkg.go.dev/charm.land/bubbletea/v2#NewProgram
	if _, err := tea.NewProgram(model).Run(); err != nil {
		complain(err)
		return exitProblems
	}
	return exitOK
}

// testScreenEnv, set to 1 with GIVENS_TEST_SEAMS=1, has bare givens or givens <folder> with
// no terminal print the TUI's first screen as plain text and exit 0, in place of usage. It's
// the acceptance suites' stand-in for a terminal: they see what `givens <folder>` opens
// through the same openProjectIn and tui.New. Both must be set, so it can't be on by accident.
const testScreenEnv = "GIVENS_TEST_SCREEN"

// printScreen prints the TUI's first screen over the project in folder, as the test seam.
func printScreen(folder string) int {
	assert.That(folder != "", "the TUI opens a folder")
	assert.That(os.Getenv(testScreenEnv) == "1", "only the test seam prints a screen")
	window, err := openForTUI(folder)
	if err != nil {
		complain(err)
		return exitProblems
	}
	fmt.Print(tui.New(window, nil).Text())
	return exitOK
}

// runDropIn is givens drop-in <capability>/<scenario>.
func runDropIn(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 1 || args[0] == "" {
		fmt.Println("usage: givens drop-in <capability>/<scenario>")
		return exitUsage
	}
	return runTUI(args[0], ".")
}

// openForTUI opens the project in folder, or above it, as the other commands do. With no
// capabilities folder anywhere, it opens folder itself, where the core says "No capabilities
// here yet" and offers to make one, so the TUI can.
func openForTUI(folder string) (*app.Window, error) {
	assert.That(folder != "", "the TUI opens a folder")
	assert.That(appDataEnv != "", "app data can be moved")
	if _, window, err := openProjectIn(folder); err == nil {
		return window, nil
	} else if _, findErr := findCapabilities(folder); findErr == nil {
		return nil, err
	}
	core, err := newTerminalApp()
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return nil, err
	}
	return core.Open(absolute)
}
