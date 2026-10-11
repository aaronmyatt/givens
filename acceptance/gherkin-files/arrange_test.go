package gherkinfiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

// runnerDeadline bounds a test runner's run. npx may download cucumber-js the first time.
const runnerDeadline = 5 * time.Minute

// stepsSource is a cucumber-js step definitions file, which Givens must leave alone.
// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/support_files/step_definitions.md
const stepsSource = `import { Given, When, Then } from "@cucumber/cucumber";

Given("a signed-in shopper with one item in the cart", async function () {});
When("they place the order", async function () {});
Then("they see {string}", async function (text: string) {});
`

// refundsFeature is a valid second .feature file for the checkout folder.
const refundsFeature = `Feature: Refunds
  As a shopper who changed their mind
  I want my money back
  So that I'm not out of pocket

  Scenario: Full refund
    Given a shopper with a paid order
    When they ask for a refund
    Then they see "Refund sent"
`

// summaryLine matches cucumber-js's summary, like "9 scenarios (9 undefined)".
// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/formatters.md
const summaryLine = `(?m)^(\d+) scenarios? \(`

// runBounded runs command like world.Run, but kills it at the deadline. A runner that isn't
// installed fails with a message naming it rather than an unexplained exit code.
// Ref: https://pkg.go.dev/os/exec#CommandContext
func runBounded(w *world.World, command string, deadline time.Duration) error {
	if strings.HasPrefix(command, "npx ") {
		if _, err := exec.LookPath("npx"); err != nil {
			return fmt.Errorf("npx isn't on PATH, so cucumber-js can't run; install Node.js: %w",
				err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	run := exec.CommandContext(ctx, "bash", "-c", command)
	run.Dir = w.Cwd
	run.Env = append(os.Environ(),
		"PATH="+w.Bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := run.CombinedOutput()
	w.Output = string(output)
	w.ExitCode = run.ProcessState.ExitCode()
	if ctx.Err() != nil {
		return fmt.Errorf("%q didn't finish within %s:\n%s", command, deadline, w.Output)
	}
	if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
		return nil
	}
	return err
}

// scenarioCount reads the number of scenarios from cucumber-js's summary line.
func scenarioCount(output string) (int, error) {
	match := regexp.MustCompile(summaryLine).FindStringSubmatch(output)
	if strings.Contains(output, "npx canceled due to missing packages") {
		return 0, fmt.Errorf("the sample shop has no cucumber-js of its own (its package.json "+
			"needs @cucumber/cucumber, installed):\n%s", output)
	}
	if match == nil {
		return 0, fmt.Errorf("cucumber-js printed no scenario count:\n%s", output)
	}
	return strconv.Atoi(match[1])
}

// scenarioBlock finds a scenario's lines in a feature file: from its first tag line to its
// last non-blank line. The next line indented two spaces (a tag, scenario, Rule or comment)
// starts the next block.
func scenarioBlock(lines []string, name string) (int, int, error) {
	start := indexOfSuffix(lines, "Scenario: "+name)
	if start < 0 {
		return 0, 0, fmt.Errorf("checkout.feature has no scenario %q", name)
	}
	end := start + 1
	for end < len(lines) && !startsBlock(lines[end]) { // bound: len(lines)
		end++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" { // bound: end only falls
		end--
	}
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "@") { // bound: falls
		start--
	}
	return start, end, nil
}

// indexOfSuffix finds the line that is want after its indentation, or -1.
func indexOfSuffix(lines []string, want string) int {
	for index, line := range lines {
		if strings.TrimSpace(line) == want {
			return index
		}
	}
	return -1
}

// startsBlock is true for a line indented exactly two spaces.
func startsBlock(line string) bool {
	return len(line) > 2 && strings.HasPrefix(line, "  ") && line[2] != ' '
}

func lineBefore(lines []string, index int) string {
	if index == 0 {
		return ""
	}
	return lines[index-1]
}

// commitShop makes the sample shop a git repository with everything committed, so git diff
// shows only what later steps change. Ref: https://git-scm.com/docs/git-commit
func commitShop(w *world.World) error {
	w.Cwd = w.Project
	command := "git init --quiet && git add -A && " +
		"git -c user.name=Givens -c user.email=givens@example.com commit --quiet -m shop"
	if err := w.Run(command); err != nil {
		return err
	}
	if w.ExitCode != 0 {
		return fmt.Errorf("committing the sample shop exited %d:\n%s", w.ExitCode, w.Output)
	}
	return nil
}

// changedLines keeps a unified diff's removed and added lines, without the file headers.
// Ref: https://www.gnu.org/software/diffutils/manual/html_node/Unified-Format.html
func changedLines(diff string) []string {
	changed := make([]string, 0, 4)
	for _, line := range strings.Split(diff, "\n") {
		header := strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ")
		if !header && (strings.HasPrefix(line, "-") || strings.HasPrefix(line, "+")) {
			changed = append(changed, line)
		}
	}
	return changed
}

// capabilityNames lists the sidebar's capabilities by name.
func capabilityNames(items []app.SidebarItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind == "capability" {
			names = append(names, item.Name)
		}
	}
	return names
}

// sidebarCapability finds a capability's line in the sidebar.
func sidebarCapability(items []app.SidebarItem, name string) (app.SidebarItem, error) {
	for _, item := range items {
		if item.Kind == "capability" && item.Name == name {
			return item, nil
		}
	}
	return app.SidebarItem{}, fmt.Errorf("the sidebar has no capability %q; it lists %q",
		name, capabilityNames(items))
}
