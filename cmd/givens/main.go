// Command givens browses, drops in to, edits, lists, runs and checks a project's scenarios
// from a terminal. See capabilities/work-in-the-terminal and use-givens-from-the-terminal.
//
//	givens [folder]                 (in a terminal: the TUI)
//	givens drop-in <capability>/<scenario>
//	givens list [--json] [--tags <expression>]
//	givens run <capability>/<scenario>
//	givens check [folder]
//	givens browser open|eval|action|close …
//	givens edit <capability>/<scenario> | <file>
//	givens new story <name> | givens new scenario <capability>
//	givens rename <key> <new name> [--yes], givens delete <key> [--yes]
//	givens todo <key>, givens done <key>
//	givens skill
//	givens helpers | helper new|edit … | history <key> | search <words> | trust | sync
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/internal/assert"
)

// Exit codes. Ref: https://pkg.go.dev/os#Exit
const (
	exitOK       = 0
	exitProblems = 1
	exitUsage    = 2
)

// folderDepthMax bounds the walk up from the working folder to the capabilities folder.
const folderDepthMax = 64

const usage = `usage: givens [folder]         (in a terminal, the TUI)
       givens drop-in <capability>/<scenario>
       givens list [--json] [--tags <expression>]
       givens run <capability>/<scenario>
       givens check [folder]
       givens browser open|eval|action|close …
       givens edit <capability>/<scenario> | <file>
       givens new story <name> | givens new scenario <capability>
       givens rename <key> <new name> [--yes] | givens delete <key> [--yes]
       givens todo <key> | givens done <key>
       givens skill
       givens helpers | helper new|edit … | history <key> | search <words> | trust | sync`

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
		return runTUI("", ".")
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "list":
		return runList(args[1:])
	case "run":
		return runSetup(args[1:])
	case "browser":
		return runBrowser(args[1:])
	case "edit":
		return runEdit(args[1:])
	case "drop-in":
		return runDropIn(args[1:])
	case "new":
		return runNew(args[1:])
	case "rename":
		return runRename(args[1:])
	case "delete":
		return runDelete(args[1:])
	case "todo", "done":
		return runFlag(args[1:], args[0] == "todo")
	case "skill":
		return runSkill(args[1:])
	}
	if code, ok := runPanel(args); ok {
		return code
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

// openFolder opens the project in folder in the TUI, like `code .`: givens <folder>.
func openFolder(folder string) int {
	assert.That(folder != "", "a folder is named")
	assert.That(exitProblems != exitOK, "a problem exits apart")
	absolute, err := filepath.Abs(folder)
	if err != nil {
		complain(err)
		return exitProblems
	}
	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		fmt.Println("There's no folder at " + homeShort(absolute))
		return exitProblems
	}
	return runTUI("", absolute)
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

// runPanel runs one of the commands for the app's other panels, saying whether args named one.
func runPanel(args []string) (int, bool) {
	assert.That(len(args) > 0, "a command is named")
	assert.That(args[0] != "", "a command has a name")
	commands := map[string]func([]string) int{
		"helpers": runHelpers, "helper": runHelper, "history": runHistory,
		"search": runSearch, "trust": runTrust, "sync": runSync,
	}
	command, ok := commands[args[0]]
	if !ok {
		return 0, false
	}
	return command(args[1:]), true
}
