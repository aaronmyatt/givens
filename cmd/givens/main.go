// Command givens opens a project's capabilities folder, and lists, runs and checks its
// scenarios from a terminal. See capabilities/use-givens-from-the-terminal.
//
// Only `givens check` works so far; the rest is driven by the backlog in todo.txt.
package main

import (
	"fmt"
	"os"

	"givens/internal/assert"
	"givens/internal/spec"
)

// Exit codes. Ref: https://pkg.go.dev/os#Exit
const (
	exitOK       = 0
	exitProblems = 1
	exitUsage    = 2
)

func main() {
	assert.That(len(os.Args) >= 1, "the OS passes the program name")
	code := run(os.Args[1:])
	assert.That(code >= exitOK && code <= exitUsage, "run returns a known exit code")
	os.Exit(code)
}

// run handles one command line and returns the exit code.
func run(args []string) int {
	assert.That(args != nil, "args is a list")
	if len(args) == 0 || args[0] != "check" {
		fmt.Println("usage: givens check [capabilities folder]")
		return exitUsage
	}
	root := "capabilities"
	if len(args) > 1 {
		root = args[1]
	}
	problems, err := spec.Check(root)
	if err != nil {
		fmt.Println(err)
		return exitProblems
	}
	for _, problem := range problems {
		fmt.Println(problem)
	}
	fmt.Printf("%d problems\n", len(problems))
	problemsMax := spec.CapabilitiesMax * spec.ScenariosPerFeatureMax
	assert.That(len(problems) <= problemsMax, "problems stay bounded")
	if len(problems) > 0 {
		return exitProblems
	}
	return exitOK
}
