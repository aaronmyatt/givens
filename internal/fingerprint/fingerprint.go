// Package fingerprint works out a scenario's setup fingerprint: one short code per thing a run
// depends on, so Givens can say what changed since the last run.
//
// The parts, in order: the scenario's steps (its tags, steps and Examples), its own setup files
// (.sh, .md and teardown), each setup it builds on through `from`, and each helper those
// scripts call, nested calls included. The Feature description, about.md and the kit aren't
// parts, so updating givens.sh unverifies nothing.
// See capabilities/share-setup-between-scenarios.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
	"givens/internal/runs"
	"givens/internal/spec"
)

// Part names that aren't helper names.
const (
	PartSteps  = "steps"
	PartSetup  = "setup"
	fromPrefix = "from:"
)

// Of works out the fingerprint of scenario, one of loaded's, for the row whose values are row
// (nil for a scenario with no Examples).
func Of(loaded *project.Project, scenario project.Scenario, row []runs.Pair) ([]runs.Pair,
	error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	parts := []runs.Pair{{Name: PartSteps, Value: hash(stepsText(scenario, row))}}
	own, err := setupText(scenario)
	if err != nil {
		return nil, err
	}
	parts = append(parts, runs.Pair{Name: PartSetup, Value: hash(own)})
	chain, err := fromChain(loaded, scenario)
	if err != nil {
		return nil, err
	}
	for _, built := range chain {
		text, err := setupText(built)
		if err != nil {
			return nil, err
		}
		parts = append(parts, runs.Pair{Name: fromPrefix + built.Key(), Value: hash(text)})
	}
	called, err := calledHelpers(loaded, append([]project.Scenario{scenario}, chain...))
	if err != nil {
		return nil, err
	}
	return append(parts, called...), nil
}

// hash is a short code for text: the first 8 hex digits of its SHA-256.
// Ref: https://pkg.go.dev/crypto/sha256
func hash(text string) string {
	assert.That(len(text) < 1<<30, "hashed text stays bounded")
	sum := sha256.Sum256([]byte(text))
	code := hex.EncodeToString(sum[:4])
	assert.That(len(code) == 8, "a code is 8 hex digits")
	return code
}

// stepsText is the scenario's block as it bears on a run: tags, steps and Examples, or only
// the given row of them.
func stepsText(scenario project.Scenario, row []runs.Pair) string {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(row) <= runs.FieldsMax, "a row stays bounded")
	var text strings.Builder
	text.WriteString(strings.Join(scenario.Tags, " ") + "\n")
	for _, step := range append(append([]project.Step{}, scenario.Background...),
		scenario.Steps...) {
		text.WriteString(step.Keyword + " " + step.Text + "\n" + step.DocString + "\n")
		for _, cells := range step.Table {
			text.WriteString(strings.Join(cells, "|") + "\n")
		}
	}
	if row != nil {
		for _, value := range row {
			text.WriteString(value.Name + "=" + value.Value + "\n")
		}
		return text.String()
	}
	for _, block := range scenario.Examples {
		text.WriteString(block.Name + "\n" + strings.Join(block.Header, "|") + "\n")
		for _, cells := range block.Rows {
			text.WriteString(strings.Join(cells, "|") + "\n")
		}
	}
	return text.String()
}

// setupText is a scenario's setup files, one after another; a missing file adds nothing.
func setupText(scenario project.Scenario) (string, error) {
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	assert.That(scenario.Setup == "" || filepath.IsAbs(scenario.Setup), "paths are absolute")
	var text strings.Builder
	for _, path := range []string{scenario.Setup, scenario.SetupCards, scenario.Teardown} {
		text.WriteString("\x00")
		if path == "" {
			continue
		}
		content, err := helpers.ReadScript(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		text.WriteString(content)
	}
	return text.String(), nil
}

// fromChain lists the scenarios a scenario's setup builds on, nearest first, each once. A
// from line naming no scenario, and a loop, end the chain: Drop in reports both before running.
func fromChain(loaded *project.Project, scenario project.Scenario) ([]project.Scenario, error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(len(loaded.Capabilities) <= spec.CapabilitiesMax, "capabilities stay bounded")
	chain := make([]project.Scenario, 0, 4)
	seen := map[string]bool{scenario.Key(): true}
	queue := []project.Scenario{scenario}
	// bound: a scenario joins the queue only once, and the project has finitely many.
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.Setup)
		if err != nil {
			return nil, err
		}
		for _, from := range helpers.Froms(text) {
			built, err := loaded.Find(from.Key)
			if err != nil || seen[built.Key()] {
				continue
			}
			seen[built.Key()] = true
			chain = append(chain, built)
			queue = append(queue, built)
		}
	}
	return chain, nil
}

// calledHelpers fingerprints every helper the scenarios' setup scripts call, each seen from
// its own capability, nested calls included, sorted by name.
func calledHelpers(loaded *project.Project, scenarios []project.Scenario) ([]runs.Pair, error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(len(scenarios) > 0, "there's at least the scenario itself")
	shared, err := helpers.InFolder(loaded.Root, "", "")
	if err != nil {
		return nil, err
	}
	bodies := make(map[string]string, 16)
	for _, scenario := range scenarios {
		if scenario.Setup == "" {
			continue
		}
		own, err := helpers.InFolder(loaded.Root, scenario.Capability, scenario.Capability)
		if err != nil {
			return nil, err
		}
		visible := helpers.Visible(shared, own)
		text, err := helpers.ReadScript(scenario.Setup)
		if err != nil {
			return nil, err
		}
		for _, name := range helpers.Called(text, visible) {
			bodies[name] = filepath.ToSlash(visible[name].File) + "\n" + visible[name].Body
		}
	}
	return sortedParts(bodies), nil
}

// sortedParts hashes each body, by name, in name order.
func sortedParts(bodies map[string]string) []runs.Pair {
	assert.That(bodies != nil, "there's a map of bodies")
	assert.That(len(bodies) <= helpers.HelpersMax, "helpers stay bounded")
	names := make([]string, 0, len(bodies))
	for name := range bodies {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]runs.Pair, 0, len(names))
	for _, name := range names {
		parts = append(parts, runs.Pair{Name: name, Value: hash(bodies[name])})
	}
	return parts
}

// Changed names the first part of now that differs from then, a run's fingerprint, as a
// person would say it: "steps", "setup", "checkout/pay-with-saved-card setup" or a helper's
// name. It's "" when nothing changed.
func Changed(then []runs.Pair, now []runs.Pair) string {
	assert.That(len(then) <= runs.FieldsMax, "then stays bounded")
	assert.That(len(now) <= runs.FieldsMax, "now stays bounded")
	before := make(map[string]string, len(then))
	for _, part := range then {
		before[part.Name] = part.Value
	}
	for _, part := range now {
		if was, ok := before[part.Name]; !ok || was != part.Value {
			return partWords(part.Name)
		}
	}
	if len(then) != len(now) {
		return PartSetup
	}
	return ""
}

// partWords says a part's name as a message would.
func partWords(name string) string {
	assert.That(name != "", "a part has a name")
	assert.That(!strings.ContainsAny(name, "\t\n"), "a part's name fits a run line field")
	if key, ok := strings.CutPrefix(name, fromPrefix); ok {
		return key + " setup"
	}
	return name
}
