// Package spec reads a capabilities folder and reports every convention it breaks.
//
// The conventions are written down in capabilities/GIVENS.md. Feature files are parsed with the
// official Cucumber Gherkin parser, so Givens accepts exactly what test runners accept.
// Ref: https://github.com/cucumber/gherkin
package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	gherkin "github.com/cucumber/gherkin/go/v42"
	messages "github.com/cucumber/messages/go/v34"

	"givens/internal/assert"
	"givens/internal/slug"
)

// Limits. Tiger Style: put a limit on everything, so a runaway folder fails loudly.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	CapabilitiesMax         = 1000
	ScenariosPerFeatureMax  = 1000
	FeatureFileSizeBytesMax = 4 << 20
)

// timeoutTag matches @timeout:90s and @timeout:5m.
var timeoutTag = regexp.MustCompile(`^@timeout:[0-9]+[sm]$`) //tiger:allow-global compiled once

// Problem is one broken convention, with where it is.
type Problem struct {
	Path    string
	Line    int64
	Message string
}

func (p Problem) String() string {
	assert.That(p.Path != "", "a problem names its file")
	assert.That(p.Message != "", "a problem says what's wrong")
	return fmt.Sprintf("%s:%d: %s", p.Path, p.Line, p.Message)
}

// Check reads every capability folder directly under root and returns the problems found.
// An error means root itself couldn't be read; broken files are problems, not errors.
func Check(root string) ([]Problem, error) {
	assert.That(root != "", "root is a path")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	if len(entries) > CapabilitiesMax {
		return nil, fmt.Errorf("%s holds %d entries, more than %d",
			root, len(entries), CapabilitiesMax)
	}
	assert.That(len(entries) <= CapabilitiesMax, "entries are bounded from here on")

	problems := make([]Problem, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		// _shared, runs and hidden folders hold helpers and history, not capabilities.
		hidden := strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".")
		if !entry.IsDir() || hidden || name == "runs" {
			continue
		}
		found, err := checkCapability(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

// checkCapability checks one folder: one .feature file named after it, valid and conventional.
// A folder with no .feature file isn't a capability (it may hold a runner's step definitions).
func checkCapability(dir string) ([]Problem, error) {
	assert.That(dir != "", "dir is a path")
	features, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return nil, err
	}
	want := filepath.Join(dir, filepath.Base(dir)+".feature")
	problems := make([]Problem, 0, len(features))
	for _, path := range features {
		assert.That(strings.HasSuffix(path, ".feature"), "the glob only returns .feature files")
		if path != want {
			message := "isn't read: a capability has one .feature file, named after its folder"
			problems = append(problems, Problem{Path: path, Line: 1, Message: message})
			continue
		}
		found, err := CheckFeature(path)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

// CheckFeature parses one .feature file and checks its story, scenario names and tags.
func CheckFeature(path string) ([]Problem, error) {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(source) > FeatureFileSizeBytesMax {
		return []Problem{{Path: path, Line: 1, Message: "file is larger than 4 MiB"}}, nil
	}
	// ParseGherkinDocument returns parse errors with their line numbers in the message.
	// Ref: https://pkg.go.dev/github.com/cucumber/gherkin/go/v42#ParseGherkinDocument
	ids := &messages.Incrementing{}
	document, err := gherkin.ParseGherkinDocument(strings.NewReader(string(source)), ids.NewId)
	if err != nil {
		return []Problem{{Path: path, Line: 0, Message: err.Error()}}, nil
	}
	if document.Feature == nil {
		return []Problem{{Path: path, Line: 1, Message: "has no Feature"}}, nil
	}
	problems := checkStory(path, document.Feature)
	problems = append(problems, checkScenarios(path, document.Feature)...)
	assert.That(document.Feature.Name != "" || len(problems) > 0, "a nameless Feature is reported")
	return problems, nil
}

// ScenariosIn parses one .feature file and lists its scenarios, Rules included.
func ScenariosIn(path string) ([]*messages.Scenario, error) {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	feature, err := ParseFeature(path)
	if err != nil {
		return nil, err
	}
	scenarios := Scenarios(feature)
	assert.That(len(scenarios) <= ScenariosPerFeatureMax, "scenarios per feature stay bounded")
	return scenarios, nil
}

// ParseFeature parses one .feature file and returns its Feature. A file too large, not valid
// Gherkin, or with no Feature is an error.
// Ref: https://pkg.go.dev/github.com/cucumber/gherkin/go/v42#ParseGherkinDocument
func ParseFeature(path string) (*messages.Feature, error) {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(source) > FeatureFileSizeBytesMax {
		return nil, fmt.Errorf("%s is larger than 4 MiB", path)
	}
	feature, err := ParseFeatureText(string(source))
	if errors.Is(err, ErrNoFeature) {
		return nil, fmt.Errorf("%s has no Feature", path)
	}
	return feature, err
}

// ErrNoFeature is a .feature file's text with no Feature: line.
var ErrNoFeature = errors.New("has no Feature")

// ParseFeatureText parses a .feature file's text, as ParseFeature does a file: an editor's
// text, before it's saved.
func ParseFeatureText(source string) (*messages.Feature, error) {
	assert.That(utf8.ValidString(source), "a feature is text")
	assert.That(len(source) <= FeatureFileSizeBytesMax*2, "the text stays bounded")
	if len(source) > FeatureFileSizeBytesMax {
		return nil, errors.New("is larger than 4 MiB")
	}
	ids := &messages.Incrementing{}
	document, err := gherkin.ParseGherkinDocument(strings.NewReader(source), ids.NewId)
	if err != nil {
		return nil, err
	}
	if document.Feature == nil {
		return nil, ErrNoFeature
	}
	assert.That(document.Feature.Location != nil, "a parsed Feature has a location")
	return document.Feature, nil
}

// checkStory requires the As a, I want and So that lines in the Feature's description.
func checkStory(path string, feature *messages.Feature) []Problem {
	assert.That(feature != nil, "feature is parsed")
	assert.That(feature.Location != nil, "feature has a location")
	problems := make([]Problem, 0, 4)
	if feature.Name == "" {
		problems = append(problems, Problem{path, feature.Location.Line, "Feature has no name"})
	}
	for _, part := range []string{"As a", "I want", "So that"} {
		if !strings.Contains(feature.Description, part) {
			message := fmt.Sprintf("story has no %q line in the Feature description", part)
			problems = append(problems, Problem{path, feature.Location.Line, message})
		}
	}
	return problems
}

// checkScenarios requires unique slugs across the file, Rules included, and well-formed tags.
func checkScenarios(path string, feature *messages.Feature) []Problem {
	assert.That(feature != nil, "feature is parsed")
	scenarios := Scenarios(feature)
	assert.That(len(scenarios) <= ScenariosPerFeatureMax, "scenarios per feature stay bounded")

	problems := make([]Problem, 0, len(scenarios))
	seen := make(map[string]string, len(scenarios))
	for _, scenario := range scenarios {
		line := scenario.Location.Line
		key := slug.Of(scenario.Name)
		if key == "" {
			message := "scenario name has no letters or digits"
			problems = append(problems, Problem{path, line, message})
		} else if other, ok := seen[key]; ok {
			message := fmt.Sprintf("%q and %q both make the slug %s", other, scenario.Name, key)
			problems = append(problems, Problem{Path: path, Line: line, Message: message})
		}
		seen[key] = scenario.Name
		for _, tag := range scenario.Tags {
			if strings.HasPrefix(tag.Name, "@timeout") && !timeoutTag.MatchString(tag.Name) {
				message := "@timeout must look like @timeout:90s or @timeout:5m"
				problems = append(problems, Problem{path, tag.Location.Line, message})
			}
		}
	}
	return problems
}

// Scenarios lists a Feature's scenarios in file order, including those inside Rule blocks.
func Scenarios(feature *messages.Feature) []*messages.Scenario {
	assert.That(feature != nil, "feature is parsed")
	scenarios := make([]*messages.Scenario, 0, len(feature.Children))
	for _, child := range feature.Children {
		assert.That(child != nil, "the parser never returns a nil child")
		if child.Scenario != nil {
			scenarios = append(scenarios, child.Scenario)
		}
		if child.Rule == nil {
			continue
		}
		for _, ruleChild := range child.Rule.Children {
			if ruleChild.Scenario != nil {
				scenarios = append(scenarios, ruleChild.Scenario)
			}
		}
	}
	return scenarios
}
