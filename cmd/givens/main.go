// Command givens lists, runs and checks a project's scenarios from a terminal, and serves the
// macOS app. See capabilities/use-givens-from-the-terminal.
//
//	givens list [--json] [--tags <expression>]
//	givens run <capability>/<scenario>
//	givens check [folder]
//	givens serve --app-data <folder>
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/serve"
)

// Exit codes. Ref: https://pkg.go.dev/os#Exit
const (
	exitOK       = 0
	exitProblems = 1
	exitUsage    = 2
)

// folderDepthMax bounds the walk up from the working folder to the capabilities folder.
const folderDepthMax = 64

const usage = `usage: givens list [--json] [--tags <expression>]
       givens run <capability>/<scenario>
       givens check [folder]
       givens serve --app-data <folder>`

func main() {
	assert.That(len(os.Args) >= 1, "the OS passes the program name")
	code := run(os.Args[1:])
	assert.That(code >= exitOK && code <= exitUsage, "run returns a known exit code")
	os.Exit(code)
}

// run handles one command line and returns the exit code.
func run(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(usage != "", "there's usage to print")
	if len(args) == 0 {
		fmt.Println(usage)
		return exitUsage
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "check":
		return runCheck(args[1:])
	case "list":
		return runList(args[1:])
	case "run":
		return runSetup(args[1:])
	}
	fmt.Println(usage)
	return exitUsage
}

// complain prints an error to stderr.
func complain(err error) {
	assert.That(err != nil, "only an error is printed")
	assert.That(err.Error() != "", "an error says what's wrong")
	_, _ = fmt.Fprintln(os.Stderr, err)
}

// findCapabilities finds the capabilities folder from start: start itself when it's named
// capabilities, else the first `capabilities` folder in start or a folder above it.
func findCapabilities(start string) (string, error) {
	assert.That(start != "", "the search starts somewhere")
	assert.That(folderDepthMax > 0, "the walk up has a limit")
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	if filepath.Base(dir) == "capabilities" {
		return dir, nil
	}
	for depth := 0; depth < folderDepthMax; depth++ {
		candidate := filepath.Join(dir, "capabilities")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("no capabilities folder here or in any folder above")
}

// runServe answers the macOS app on stdin and stdout until stdin closes. GIVENS_TEST_SEAMS=1
// lets the acceptance suites call the "test." methods.
func runServe(args []string) int {
	assert.That(args != nil, "args is a list")
	if len(args) != 2 || args[0] != "--app-data" || args[1] == "" {
		_, _ = fmt.Fprintln(os.Stderr, "usage: givens serve --app-data <folder>")
		return exitUsage
	}
	if err := os.MkdirAll(args[1], 0o755); err != nil {
		complain(err)
		return exitProblems
	}
	server, err := serve.New(args[1], os.Getenv("GIVENS_TEST_SEAMS") == "1")
	if err != nil {
		complain(err)
		return exitProblems
	}
	if err := server.Serve(os.Stdin, os.Stdout); err != nil {
		complain(err)
		return exitProblems
	}
	assert.That(server != nil, "serve ran on a server")
	return exitOK
}
