// Command givens opens a project's capabilities folder, and lists, runs and checks its
// scenarios from a terminal. See capabilities/use-givens-from-the-terminal.
//
// `givens serve` is the core the macOS app runs as a child process. Only `givens check` and
// `givens serve` work so far; the rest is driven by the backlog in todo.txt.
package main

import (
	"fmt"
	"os"

	"givens/internal/assert"
	"givens/internal/serve"
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
	if len(args) > 0 && args[0] == "serve" {
		return runServe(args[1:])
	}
	if len(args) == 0 || args[0] != "check" {
		fmt.Println("usage: givens check [capabilities folder]")
		fmt.Println("       givens serve --app-data <folder>")
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

// runServe answers the macOS app on stdin and stdout until stdin closes. GIVENS_TEST_SEAMS=1
// lets the acceptance suites call the "test." methods.
func runServe(args []string) int {
	assert.That(args != nil, "args is a list")
	if len(args) != 2 || args[0] != "--app-data" || args[1] == "" {
		_, _ = fmt.Fprintln(os.Stderr, "usage: givens serve --app-data <folder>")
		return exitUsage
	}
	if err := os.MkdirAll(args[1], 0o755); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitProblems
	}
	server, err := serve.New(args[1], os.Getenv("GIVENS_TEST_SEAMS") == "1")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitProblems
	}
	if err := server.Serve(os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitProblems
	}
	assert.That(server != nil, "serve ran on a server")
	return exitOK
}
