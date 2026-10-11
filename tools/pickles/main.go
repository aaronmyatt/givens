// Command pickles prints the compiled scenarios ("pickles") of one .feature file as a JSON
// array, so the Swift acceptance suite in macos/ runs exactly the scenarios godog runs: same
// parser, same Background, Rule and Scenario Outline expansion.
//
//	go run ./tools/pickles capabilities/open-a-project/open-a-project.feature
//
// The JSON is the Cucumber Pickle message, unchanged.
// Ref: https://github.com/cucumber/messages/blob/main/messages.md#pickle
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	gherkin "github.com/cucumber/gherkin/go/v42"
	messages "github.com/cucumber/messages/go/v34"

	"givens/internal/assert"
	"givens/internal/spec"
)

// Exit codes. Ref: https://pkg.go.dev/os#Exit
const (
	exitOK       = 0
	exitProblems = 1
	exitUsage    = 2
)

// featureLengthMax bounds one .feature file, in bytes.
const featureLengthMax = 1 << 20

func main() {
	assert.That(len(os.Args) >= 1, "the OS passes the program name")
	code := run(os.Args[1:])
	assert.That(code >= exitOK && code <= exitUsage, "run returns a known exit code")
	os.Exit(code)
}

// run prints the pickles of the one .feature file named in args.
func run(args []string) int {
	assert.That(args != nil, "args is a list")
	if len(args) != 1 || !strings.HasSuffix(args[0], ".feature") {
		fmt.Println("usage: pickles <file.feature>")
		return exitUsage
	}
	pickles, err := compile(args[0])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitProblems
	}
	// MarshalIndent keeps the output readable when a person runs the tool by hand.
	// Ref: https://pkg.go.dev/encoding/json#MarshalIndent
	output, err := json.MarshalIndent(pickles, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitProblems
	}
	assert.That(len(output) > 0, "the JSON is never empty")
	fmt.Println(string(output))
	return exitOK
}

// compile parses path and compiles its scenarios into pickles.
// Ref: https://pkg.go.dev/github.com/cucumber/gherkin/go/v42#Pickles
func compile(path string) ([]*messages.Pickle, error) {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(source) > featureLengthMax {
		return nil, fmt.Errorf("%s is over %d bytes", path, featureLengthMax)
	}
	ids := &messages.Incrementing{}
	document, err := gherkin.ParseGherkinDocument(strings.NewReader(string(source)), ids.NewId)
	if err != nil {
		return nil, err
	}
	pickles := gherkin.Pickles(*document, path, ids.NewId)
	assert.That(len(pickles) <= spec.ScenariosPerFeatureMax*64, "pickles stay bounded")
	return pickles, nil
}
