// Command givens lists, runs and checks a project's scenarios from a terminal, and serves the
// macOS app. See capabilities/use-givens-from-the-terminal.
//
//	givens <folder>
//	givens list [--json] [--tags <expression>]
//	givens run <capability>/<scenario>
//	givens check [folder]
//	givens serve --app-data <folder>
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

const usage = `usage: givens <folder>
       givens list [--json] [--tags <expression>]
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
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		return openFolder(args[0])
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

// openCommandEnv names a stand-in for `open`, so the acceptance suite can see what would open.
const openCommandEnv = "GIVENS_OPEN_COMMAND"

// openFolder opens a project in the Givens app, like `code .`: `open -a Givens <folder>`.
// Ref: https://ss64.com/mac/open.html
func openFolder(folder string) int {
	assert.That(folder != "", "a folder is named")
	assert.That(openCommandEnv != "", "the stand-in has a name")
	absolute, err := filepath.Abs(folder)
	if err != nil {
		complain(err)
		return exitProblems
	}
	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		fmt.Println("There's no folder at " + homeShort(absolute))
		return exitProblems
	}
	opener := os.Getenv(openCommandEnv)
	if opener == "" {
		opener = "open"
	}
	command := exec.Command(opener, "-a", "Givens", absolute)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		complain(err)
		return exitProblems
	}
	return exitOK
}

// homeShort writes a path under the home folder with ~, as a person would type it.
func homeShort(path string) string {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	assert.That(!strings.HasPrefix(path, home+"/"), "a path under home was shortened")
	return path
}
