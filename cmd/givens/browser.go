package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/browser"
)

// The givens browser commands are what the kit's browser functions call: bash can't speak
// the DevTools Protocol's websocket. Results go to stdout, so X="$(in_browser …)" holds one;
// a problem goes to stderr as one plain line, with exit 1, and the kit's fail adds its ✗.
// See capabilities/mix-shell-browser-and-hand-steps.
const browserUsage = `usage: givens browser open <url>
       givens browser eval <javascript>
       givens browser action <name> [json]
       givens browser close`

// runBrowser runs one givens browser command against the project's browser: the one of
// GIVENS_ROOT, which the kit sets, or else of the capabilities folder found from here.
func runBrowser(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) < 1 || !browserArity(args[0], len(args)-1) {
		_, _ = fmt.Fprintln(os.Stderr, browserUsage)
		return exitUsage
	}
	profile, err := browserProfile()
	if err != nil {
		complain(err)
		return exitProblems
	}
	result := ""
	switch args[0] {
	case "open":
		err = browser.Open(profile, args[1])
	case "eval":
		result, err = browser.Eval(profile, args[1])
	case "action":
		argument := ""
		if len(args) == 3 {
			argument = args[2]
		}
		result, err = browser.Action(profile, args[1], argument)
	case "close":
		err = browser.Close(profile)
	}
	if errors.Is(err, browser.ErrNoChrome) {
		err = fmt.Errorf("%w: brew install --cask google-chrome, or set %s", err,
			browser.ChromeEnv)
	}
	if err != nil {
		complain(err)
		return exitProblems
	}
	if result != "" {
		fmt.Println(result)
	}
	return exitOK
}

// browserArity is true when command takes count arguments.
func browserArity(command string, count int) bool {
	assert.That(count >= 0, "a count isn't negative")
	assert.That(browserUsage != "", "there's usage to print")
	switch command {
	case "open", "eval":
		return count == 1
	case "action":
		return count == 1 || count == 2
	case "close":
		return count == 0
	}
	return false
}

// browserProfile is the Givens browser profile of the project: GIVENS_ROOT's, or the
// capabilities folder's found from the working folder.
func browserProfile() (string, error) {
	assert.That(folderDepthMax > 0, "the walk up has a limit")
	capabilities := os.Getenv("GIVENS_ROOT")
	if capabilities == "" {
		found, err := findCapabilities(".")
		if err != nil {
			return "", err
		}
		capabilities = found
	}
	absolute, err := filepath.Abs(capabilities)
	if err != nil {
		return "", err
	}
	assert.That(filepath.IsAbs(absolute), "the project is found at an absolute path")
	return browser.Profile(absolute)
}
