// Package project loads a capabilities folder into the capabilities, scenarios, tags and setup
// files Givens shows and runs. It reads; it never writes.
//
// The layout it reads is written down in capabilities/GIVENS.md. Parsing is internal/spec's, so
// what Givens loads is what spec.Check judges and what test runners accept.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/slug"
	"givens/internal/spec"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// HelperFilesMax bounds the .sh files in one _shared folder.
	HelperFilesMax = 1000
	// StillRunningDefault is when Givens shows "Still running" without a @timeout tag.
	StillRunningDefault = 2 * time.Minute
)

// Tags with a meaning of their own. Every other tag is a view.
const (
	TagError         = "@error"
	TagTimeoutPrefix = "@timeout:"
)

// Project is one loaded capabilities folder.
type Project struct {
	Root         string       // the capabilities folder, absolute
	Capabilities []Capability // sorted by folder name
	Helpers      []string     // _shared/*.sh at the root, sorted; the kit included
	Problems     []spec.Problem
}

// Capability is one folder holding <slug>/<slug>.feature.
type Capability struct {
	Name        string // the Feature's name, or the folder's when the file can't be read
	Slug        string // the folder's name
	Dir         string
	FeaturePath string
	Story       []string // the As a, I want and So that lines, in file order
	Description string   // the Feature's whole description, trimmed
	Tags        []string // tags on the Feature
	About       string   // about.md's path, or "" when there is none
	Helpers     []string // <slug>/_shared/*.sh, sorted
	Scenarios   []Scenario
	ParseError  string   // why the .feature file couldn't be read, or ""
	Ignored     []string // other .feature files in the folder, which aren't read; sorted
}

// Load reads root, a capabilities folder. An error means root couldn't be read; a broken
// capability is loaded with its ParseError set and its problems in Problems.
func Load(root string) (*Project, error) {
	assert.That(root != "", "root is a path")
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, err
	}
	if len(entries) > spec.CapabilitiesMax {
		return nil, fmt.Errorf("%s holds %d entries, more than %d",
			absolute, len(entries), spec.CapabilitiesMax)
	}
	project := &Project{Root: absolute}
	project.Capabilities = make([]Capability, 0, len(entries))
	// ReadDir returns entries sorted by name. Ref: https://pkg.go.dev/os#ReadDir
	for _, entry := range entries {
		if !isCapabilityFolder(absolute, entry) {
			continue
		}
		project.Capabilities = append(project.Capabilities,
			loadCapability(filepath.Join(absolute, entry.Name())))
	}
	if project.Helpers, err = helperFiles(absolute); err != nil {
		return nil, err
	}
	if project.Problems, err = spec.Check(absolute); err != nil {
		return nil, err
	}
	assert.That(len(project.Capabilities) <= len(entries), "each capability is an entry")
	return project, nil
}

// isCapabilityFolder is true for a visible folder holding <name>/<name>.feature.
func isCapabilityFolder(root string, entry os.DirEntry) bool {
	assert.That(root != "", "root is a path")
	assert.That(entry != nil, "the entry was read")
	name := entry.Name()
	// _shared, runs and hidden folders hold helpers and history, not capabilities.
	if !entry.IsDir() || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		return false
	}
	_, err := os.Stat(filepath.Join(root, name, name+".feature"))
	return err == nil
}

// loadCapability reads one capability folder. It never fails: what can't be read is reported
// in ParseError, so the folder still shows.
func loadCapability(dir string) Capability {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	name := filepath.Base(dir)
	capability := Capability{
		Name: name, Slug: name, Dir: dir,
		FeaturePath: filepath.Join(dir, name+".feature"),
	}
	if _, err := os.Stat(filepath.Join(dir, "about.md")); err == nil {
		capability.About = filepath.Join(dir, "about.md")
	}
	helpers, err := helperFiles(dir)
	if err != nil {
		capability.ParseError = err.Error()
		return capability
	}
	capability.Helpers = helpers
	capability.Ignored = otherFeatures(dir, capability.FeaturePath)
	feature, err := spec.ParseFeature(capability.FeaturePath)
	if err != nil {
		capability.ParseError = parseErrorOf(capability.FeaturePath, err)
		capability.Name = featureNameIn(capability.FeaturePath, name)
		return capability
	}
	capability.Name = feature.Name
	capability.Description = strings.TrimSpace(feature.Description)
	capability.Story = storyLines(feature.Description)
	capability.Tags = tagNames(feature.Tags)
	capability.Scenarios = scenariosOf(capability, feature)
	assert.That(capability.Slug == name, "a capability's slug is its folder's name")
	return capability
}

// otherFeatures lists the .feature files in dir besides the capability's own.
func otherFeatures(dir string, own string) []string {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	assert.That(filepath.Dir(own) == dir, "the capability's file is in its folder")
	// Glob only fails on a bad pattern, and this one is fixed.
	// Ref: https://pkg.go.dev/path/filepath#Glob
	paths, _ := filepath.Glob(filepath.Join(dir, "*.feature"))
	others := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != own {
			others = append(others, path)
		}
	}
	return others
}

// parseErrorOf says why a .feature file can't be read, naming the file and its line:
// "checkout.feature line 17: expected a step, a tag, a comment or a new scenario".
func parseErrorOf(path string, err error) string {
	assert.That(err != nil, "only an error is described")
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	line, message := spec.Explain(err)
	if line > 0 {
		return fmt.Sprintf("%s line %d: %s", filepath.Base(path), line, message)
	}
	return filepath.Base(path) + ": " + message
}

// featureNameIn reads the name on a file's "Feature:" line without parsing the rest, so a
// capability whose file doesn't parse keeps its name. fallback is used when there's none.
func featureNameIn(path string, fallback string) string {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	assert.That(fallback != "", "there's a name to fall back on")
	text, err := os.ReadFile(path)
	if err != nil || len(text) > spec.FeatureFileSizeBytesMax {
		return fallback
	}
	for _, line := range strings.Split(string(text), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "Feature:"); ok {
			if name = strings.TrimSpace(name); name != "" {
				return name
			}
		}
	}
	return fallback
}

// storyLines picks the As a, I want and So that lines out of a Feature's description.
func storyLines(description string) []string {
	assert.That(len(description) <= spec.FeatureFileSizeBytesMax, "a description fits its file")
	lines := strings.Split(description, "\n")
	story := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, start := range []string{"As a", "I want", "So that"} {
			if strings.HasPrefix(line, start) {
				story = append(story, line)
			}
		}
	}
	assert.That(len(story) <= len(lines), "each story line is a description line")
	return story
}

// helperFiles lists dir/_shared/*.sh, sorted. No _shared folder means no helpers.
func helperFiles(dir string) ([]string, error) {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	// Glob returns matches in lexical order and only fails on a bad pattern.
	// Ref: https://pkg.go.dev/path/filepath#Glob
	paths, err := filepath.Glob(filepath.Join(dir, "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	if len(paths) > HelperFilesMax {
		return nil, fmt.Errorf("%s/_shared holds %d helper files, more than %d",
			dir, len(paths), HelperFilesMax)
	}
	sort.Strings(paths)
	assert.That(len(paths) <= HelperFilesMax, "helper files stay bounded")
	return paths, nil
}

// ErrNoScenario is returned by Find when no scenario has the key.
var ErrNoScenario = errors.New("there's no scenario with that name")

// Find returns the scenario whose key is <capability slug>/<scenario slug>.
func (p *Project) Find(key string) (Scenario, error) {
	assert.That(p != nil, "find runs on a project")
	assert.That(key != "", "a key names a scenario")
	capabilitySlug, scenarioSlug, ok := strings.Cut(key, "/")
	if !ok {
		return Scenario{}, fmt.Errorf("%s: %w", key, ErrNoScenario)
	}
	for _, capability := range p.Capabilities {
		if capability.Slug != capabilitySlug {
			continue
		}
		for _, scenario := range capability.Scenarios {
			if scenario.Slug == scenarioSlug {
				return scenario, nil
			}
		}
	}
	assert.That(len(key) <= slug.NameLengthMax*2, "a key is two slugs")
	return Scenario{}, fmt.Errorf("%s: %w", key, ErrNoScenario)
}
