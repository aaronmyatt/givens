package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/app"
	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/spec"
)

// listed is one line of `givens list`, and one object of `givens list --json`.
type listed struct {
	Path   string `json:"path"`   // the run key: "checkout/card-declined"
	Kind   string `json:"kind"`   // "happy path" or "error state"
	Status string `json:"status"` // as the app shows it: "Never run", "Verified today by Sam"
}

// runList prints every scenario in the project, or those a tag expression matches.
func runList(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	asJSON, expression := false, ""
	for index := 0; index < len(args); index++ {
		switch {
		case args[index] == "--json":
			asJSON = true
		case args[index] == "--tags" && index+1 < len(args):
			expression = args[index+1]
			index++
		default:
			fmt.Println(usage)
			return exitUsage
		}
	}
	filter, err := app.ParseTagExpression(expression)
	if err != nil {
		complain(err)
		return exitUsage
	}
	items, err := listScenarios(filter)
	if err != nil {
		complain(err)
		return exitProblems
	}
	if asJSON {
		// Ref: https://pkg.go.dev/encoding/json#Encoder.SetIndent
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(items); err != nil {
			complain(err)
			return exitProblems
		}
		return exitOK
	}
	for _, item := range items {
		fmt.Printf("%-44s %-12s %s\n", item.Path, item.Kind, item.Status)
	}
	return exitOK
}

// listScenarios loads the project around the working folder and lists the scenarios filter
// matches, in capability then file order.
func listScenarios(filter app.TagExpression) ([]listed, error) {
	assert.That(spec.ScenariosPerFeatureMax > 0, "scenarios are bounded")
	root, err := findCapabilities(".")
	if err != nil {
		return nil, err
	}
	loaded, err := project.Load(root)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	items := make([]listed, 0, 16)
	for _, capability := range loaded.Capabilities {
		for _, scenario := range capability.Scenarios {
			if scenario.Slug == "" || !filter.Match(scenario.Tags) {
				continue
			}
			kind := "happy path"
			if scenario.Error {
				kind = "error state"
			}
			items = append(items, listed{
				Path: scenario.Key(), Kind: kind, Status: app.StatusOf(loaded, scenario, now),
			})
		}
	}
	assert.That(len(items) <= spec.CapabilitiesMax*spec.ScenariosPerFeatureMax, "bounded")
	return items, nil
}

// runCheck checks a capabilities folder against GIVENS.md and prints each problem as
// "checkout/checkout.feature line 17: …", then a count.
func runCheck(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	start := "."
	if len(args) > 0 {
		start = args[0]
	}
	root, err := findCapabilities(start)
	if err != nil {
		complain(err)
		return exitProblems
	}
	loaded, err := project.Load(root)
	if err != nil {
		complain(err)
		return exitProblems
	}
	count := 0
	for _, capability := range loaded.Capabilities {
		count += len(capability.Scenarios)
	}
	for _, problem := range loaded.Problems {
		fmt.Println(describe(root, problem))
	}
	fmt.Printf("%d scenarios, %d problems\n", count, len(loaded.Problems))
	if len(loaded.Problems) > 0 {
		return exitProblems
	}
	return exitOK
}

// describe says one problem as `givens check` prints it, the parser's own words made plain.
func describe(root string, problem spec.Problem) string {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(problem.Message != "", "a problem says what's wrong")
	path := problem.Path
	if relative, err := filepath.Rel(root, problem.Path); err == nil {
		path = filepath.ToSlash(relative)
	}
	line, message := problem.Line, problem.Message
	if line == 0 && strings.Contains(message, "expected:") {
		line, message = spec.Explain(errors.New(message))
	}
	return fmt.Sprintf("%s line %d: %s", path, line, message)
}
