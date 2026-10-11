package world

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"givens/internal/assert"
	"givens/internal/spec"
)

// Step is one step definition: a regular expression over the step's text (without its
// keyword) and a function taking the captured groups as strings and returning an error.
// Ref: https://pkg.go.dev/github.com/cucumber/godog#ScenarioContext.Step
type Step struct {
	Pattern string
	Func    any
}

// Steps builds a capability's step definitions for one scenario's world. Suites call it with a
// real world; CheckStepsDefined calls it with a world whose steps are never run.
type Steps func(w *World) []Step

// FeaturePath is a capability's .feature file in Givens's own spec.
func FeaturePath(root string, capability string) string {
	assert.That(root != "", "root is the repository")
	assert.That(capability != "", "capability is a slug")
	return filepath.Join(root, "capabilities", capability, capability+".feature")
}

// RunSuite runs one capability's scenarios strictly: an undefined, pending or failing step
// fails the test. GIVENS_TAGS picks scenarios by tag expression; the default, "~@wip", skips
// work in progress. GIVENS_TAGS=@wip runs only the scenarios still to build.
func RunSuite(t *testing.T, capability string, steps Steps) {
	assert.That(t != nil, "suites run inside a test")
	assert.That(steps != nil, "a suite has steps")
	root, err := RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	bin := buildGivens(t, root)
	tags := os.Getenv("GIVENS_TAGS")
	if tags == "" {
		tags = "~@wip"
	}
	suite := godog.TestSuite{
		Name: capability,
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			w := &World{Root: root, Bin: bin}
			ctx.Before(w.setUp)
			ctx.After(w.tearDown)
			for _, step := range append(Common(w), steps(w)...) {
				ctx.Step(step.Pattern, step.Func)
			}
		},
		Options: &godog.Options{
			Format: "pretty", Paths: []string{FeaturePath(root, capability)},
			Tags: tags, Strict: true, TestingT: t,
		},
	}
	if status := suite.Run(); status != 0 {
		t.Fatalf("godog exited with status %d", status)
	}
}

// CheckStepsDefined fails the test for every step in the capability's .feature file that no
// definition matches, or that more than one matches. It runs no steps.
func CheckStepsDefined(t *testing.T, capability string, steps Steps) {
	assert.That(t != nil, "the check runs inside a test")
	assert.That(steps != nil, "the check has steps")
	root, err := RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	definitions := append(Common(&World{}), steps(&World{})...)
	patterns := make([]*regexp.Regexp, 0, len(definitions))
	for _, definition := range definitions {
		patterns = append(patterns, regexp.MustCompile(definition.Pattern))
	}
	scenarios, err := spec.ScenariosIn(FeaturePath(root, capability))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range scenarios {
		for _, step := range scenario.Steps {
			if matched := matching(patterns, step.Text); len(matched) != 1 {
				t.Errorf("%q, step %q: %d definitions match %q", scenario.Name, step.Text,
					len(matched), matched)
			}
		}
	}
}

// matching lists the patterns that match text.
func matching(patterns []*regexp.Regexp, text string) []string {
	assert.That(patterns != nil, "there are patterns")
	assert.That(!strings.Contains(text, "\n"), "a step is one line")
	matched := make([]string, 0, 2)
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			matched = append(matched, pattern.String())
		}
	}
	return matched
}

// buildGivens builds cmd/givens once per test binary, for steps that run the command.
func buildGivens(t *testing.T, root string) string {
	assert.That(t != nil, "the build runs inside a test")
	assert.That(root != "", "root is the repository")
	bin := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "givens"), "./cmd/givens")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building givens: %v\n%s", err, output)
	}
	return bin
}

// setUp gives each scenario a fresh folder, app data folder and fakes.
func (w *World) setUp(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	assert.That(w != nil, "set up runs on a world")
	assert.That(w.Root != "", "the world knows the repository")
	dir, err := os.MkdirTemp("", "givens-scenario-")
	if err != nil {
		return ctx, err
	}
	w.Dir = dir
	w.AppData = filepath.Join(dir, "app-data")
	w.Clipboard = &Clipboard{}
	w.Trash = &Trash{Dir: filepath.Join(dir, "Trash")}
	w.Clock = NewClock()
	return ctx, os.MkdirAll(w.AppData, 0o755)
}

// tearDown removes the scenario's folder.
func (w *World) tearDown(
	ctx context.Context, _ *godog.Scenario, err error,
) (context.Context, error) {
	assert.That(w != nil, "tear down runs on a world")
	assert.That(strings.Contains(w.Dir, "givens-scenario-"), "only a scenario folder is removed")
	if removeErr := os.RemoveAll(w.Dir); removeErr != nil {
		return ctx, fmt.Errorf("removing %s: %w", w.Dir, removeErr)
	}
	// godog already reports the scenario's own error; returning it again would repeat it.
	assert.That(err == nil || w.Dir != "", "a failed scenario still had a folder")
	return ctx, nil
}

// Run runs command with bash in Cwd (or the sample shop's folder), with the built givens on
// PATH, and keeps its output and exit code. A non-zero exit isn't an error: steps check
// ExitCode.
func (w *World) Run(command string) error {
	assert.That(w != nil, "run runs on a world")
	assert.That(command != "", "a command is given")
	dir := w.Cwd
	if dir == "" {
		dir = w.Project
	}
	if dir == "" {
		dir = w.Dir
	}
	run := exec.Command("bash", "-c", command)
	run.Dir = dir
	run.Env = append(os.Environ(), "PATH="+w.Bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := run.CombinedOutput()
	w.Output = string(output)
	w.ExitCode = run.ProcessState.ExitCode()
	if _, exited := err.(*exec.ExitError); exited {
		return nil
	}
	return err
}
