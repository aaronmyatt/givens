package project

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	messages "github.com/cucumber/messages/go/v34"

	"givens/internal/assert"
	"givens/internal/slug"
	"givens/internal/spec"
)

// Scenario is one scenario with everything Givens needs to show it and set it up.
type Scenario struct {
	Name       string
	Slug       string
	Capability string // the capability's slug
	Line       int64
	Outline    bool
	Tags       []string // the scenario's tags, then its Rule's and Feature's, without repeats
	Error      bool     // tagged @error
	// StillRunning is when to show "Still running": StillRunningDefault, or the @timeout tag's.
	// It is zero when a @timeout tag can't be read.
	StillRunning time.Duration
	Background   []Step // the Feature's Background, then its Rule's
	Steps        []Step
	Examples     []Examples
	Setup        string // scripts/<slug>.sh, or "" when there is none
	SetupCards   string // scripts/<slug>.md, or "" when there is none
	Teardown     string // scripts/<slug>.teardown.sh, or "" when there is none
}

// Step is one Gherkin step.
type Step struct {
	Keyword   string // "Given", "When", "Then", "And", "But" or "*", as written
	Text      string
	Line      int64
	DocString string     // the step's doc string, or ""
	Table     [][]string // the step's data table, header row first, or nil
}

// Examples is one Examples block of a Scenario Outline.
type Examples struct {
	Name   string
	Slug   string // "" for an unnamed block
	Header []string
	Rows   [][]string
}

// Key is <capability slug>/<scenario slug>, the name runs are recorded against.
func (s Scenario) Key() string {
	assert.That(s.Capability != "", "a scenario belongs to a capability")
	assert.That(s.Slug != "", "a scenario has a slug")
	return s.Capability + "/" + s.Slug
}

// scenariosOf lists a Feature's scenarios in file order, Rules included, each with the
// Background steps and tags it inherits.
func scenariosOf(capability Capability, feature *messages.Feature) []Scenario {
	assert.That(capability.Dir != "", "the capability has a folder")
	assert.That(feature != nil, "the feature is parsed")
	scenarios := make([]Scenario, 0, len(feature.Children))
	featureSteps := make([]Step, 0, 4)
	for _, child := range feature.Children {
		if child.Background != nil {
			featureSteps = stepsOf(child.Background.Steps)
		}
		if child.Scenario != nil {
			scenarios = append(scenarios,
				scenarioOf(capability, child.Scenario, featureSteps, feature.Tags))
		}
		if child.Rule == nil {
			continue
		}
		ruleSteps := featureSteps
		inherited := append(append([]*messages.Tag{}, child.Rule.Tags...), feature.Tags...)
		for _, ruleChild := range child.Rule.Children {
			if ruleChild.Background != nil {
				ruleSteps = append(append([]Step{}, featureSteps...),
					stepsOf(ruleChild.Background.Steps)...)
			}
			if ruleChild.Scenario != nil {
				scenarios = append(scenarios,
					scenarioOf(capability, ruleChild.Scenario, ruleSteps, inherited))
			}
		}
	}
	assert.That(len(scenarios) <= spec.ScenariosPerFeatureMax, "scenarios stay bounded")
	return scenarios
}

// scenarioOf builds one Scenario and finds its setup files.
func scenarioOf(
	capability Capability, scenario *messages.Scenario, background []Step,
	inherited []*messages.Tag,
) Scenario {
	assert.That(scenario != nil, "the scenario is parsed")
	assert.That(scenario.Location != nil, "a parsed scenario has a location")
	key := slug.Of(scenario.Name)
	tags := tagNames(append(append([]*messages.Tag{}, scenario.Tags...), inherited...))
	built := Scenario{
		Name: scenario.Name, Slug: key, Capability: capability.Slug,
		Line:       scenario.Location.Line,
		Outline:    len(scenario.Examples) > 0,
		Tags:       tags,
		Background: background,
		Steps:      stepsOf(scenario.Steps),
		Examples:   examplesOf(scenario.Examples),
	}
	built.Error, built.StillRunning = readTags(tags)
	if key != "" {
		scripts := filepath.Join(capability.Dir, "scripts")
		built.Setup = existing(filepath.Join(scripts, key+".sh"))
		built.SetupCards = existing(filepath.Join(scripts, key+".md"))
		built.Teardown = existing(filepath.Join(scripts, key+".teardown.sh"))
	}
	return built
}

// readTags reads @error and @timeout out of a scenario's tags.
func readTags(tags []string) (bool, time.Duration) {
	assert.That(len(tags) <= spec.ScenariosPerFeatureMax, "tags stay bounded")
	isError := false
	stillRunning := StillRunningDefault
	for _, tag := range tags {
		if tag == TagError {
			isError = true
		}
		if strings.HasPrefix(tag, "@timeout") {
			stillRunning = timeoutOf(tag)
		}
	}
	assert.That(stillRunning >= 0, "a timeout is never negative")
	return isError, stillRunning
}

// timeoutOf reads @timeout:90s or @timeout:5m, and gives zero for anything else.
func timeoutOf(tag string) time.Duration {
	assert.That(strings.HasPrefix(tag, "@timeout"), "tag is a timeout tag")
	value, ok := strings.CutPrefix(tag, TagTimeoutPrefix)
	if !ok || (!strings.HasSuffix(value, "s") && !strings.HasSuffix(value, "m")) {
		return 0
	}
	// ParseDuration also reads "1.5s" and "1m30s"; GIVENS.md allows whole seconds or minutes,
	// so those count as unreadable. Ref: https://pkg.go.dev/time#ParseDuration
	duration, err := time.ParseDuration(value)
	whole := strings.Trim(value[:len(value)-1], "0123456789") == ""
	if err != nil || duration <= 0 || !whole {
		return 0
	}
	assert.That(duration > 0, "a readable timeout is positive")
	return duration
}

// stepsOf copies parsed steps into Steps.
func stepsOf(parsed []*messages.Step) []Step {
	assert.That(len(parsed) <= spec.ScenariosPerFeatureMax, "steps stay bounded")
	steps := make([]Step, 0, len(parsed))
	for _, step := range parsed {
		built := Step{
			Keyword: strings.TrimSpace(step.Keyword), Text: step.Text,
			Line: step.Location.Line,
		}
		if step.DocString != nil {
			built.DocString = step.DocString.Content
		}
		if step.DataTable != nil {
			built.Table = rowsOf(step.DataTable.Rows)
		}
		steps = append(steps, built)
	}
	assert.That(len(steps) == len(parsed), "every step is copied")
	return steps
}

// examplesOf copies a Scenario Outline's Examples blocks.
func examplesOf(parsed []*messages.Examples) []Examples {
	assert.That(len(parsed) <= spec.ScenariosPerFeatureMax, "examples stay bounded")
	blocks := make([]Examples, 0, len(parsed))
	for _, block := range parsed {
		built := Examples{Name: block.Name, Slug: slug.Of(block.Name)}
		if block.TableHeader != nil {
			built.Header = rowsOf([]*messages.TableRow{block.TableHeader})[0]
		}
		built.Rows = rowsOf(block.TableBody)
		blocks = append(blocks, built)
	}
	assert.That(len(blocks) == len(parsed), "every block is copied")
	return blocks
}

// rowsOf copies table rows into cell values.
func rowsOf(parsed []*messages.TableRow) [][]string {
	assert.That(len(parsed) <= spec.ScenariosPerFeatureMax, "rows stay bounded")
	rows := make([][]string, 0, len(parsed))
	for _, row := range parsed {
		cells := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, cell.Value)
		}
		rows = append(rows, cells)
	}
	assert.That(len(rows) == len(parsed), "every row is copied")
	return rows
}

// tagNames lists tag names in order, without repeats.
func tagNames(tags []*messages.Tag) []string {
	assert.That(len(tags) <= spec.ScenariosPerFeatureMax, "tags stay bounded")
	names := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if !seen[tag.Name] {
			names = append(names, tag.Name)
		}
		seen[tag.Name] = true
	}
	assert.That(len(names) <= len(tags), "no tag is added")
	return names
}

// existing returns path when a file is there, and "" when not.
func existing(path string) string {
	assert.That(filepath.IsAbs(path), "path is absolute")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	assert.That(!info.IsDir(), "only a file is returned")
	return path
}
