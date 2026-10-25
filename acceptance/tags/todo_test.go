package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	messages "github.com/cucumber/messages/go/v34"

	"givens/acceptance/world"
	"givens/internal/spec"
)

// todoSteps are the steps of the to do flag's scenarios: "A scenario flagged to do", "Flag a
// scenario to do" and "Remove a to do flag".
func (s *state) todoSteps() []world.Step {
	return []world.Step{
		world.Def(`^"([^"]*)" is listed on the Tags tab$`, s.underTags),
		world.Def(`^"([^"]*)" is shown flagged to do$`, s.flagged),
		world.Def(`^"([^"]*)" is tagged (@\S+) in checkout\.feature, and nothing else changes$`,
			func(name string, tag string) error { return s.onlyFlagChanged(name, tag, true) }),
		world.Def(`^"([^"]*)" isn't tagged (@\S+) in checkout\.feature, and nothing else `+
			`changes$`,
			func(name string, tag string) error { return s.onlyFlagChanged(name, tag, false) }),
		world.Def(`^it's listed when they select "([^"]*)" on the Tags tab$`, s.listedUnderTag),
		world.Def(`^it isn't shown flagged to do$`, s.notFlagged),
	}
}

// underTags expects name among the window's tag views: what the TUI's Tags tab lists.
func (s *state) underTags(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	tags := make([]string, 0, 8)
	for _, item := range window.Sidebar() {
		if item.Kind == "tag" {
			tags = append(tags, item.Name)
		}
	}
	if !slices.Contains(tags, name) {
		return fmt.Errorf("the Tags tab lists %q, not %q", tags, name)
	}
	return nil
}

// onlyFlagChanged expects the scenario's own tags to have tag, or not, and checkout.feature
// to be the sample shop's as checked in apart from that tag: no other line and no other tag.
func (s *state) onlyFlagChanged(name string, tag string, want bool) error {
	s.subject = name
	path := filepath.Join(s.Capabilities(), "checkout", "checkout.feature")
	original := filepath.Join(s.Root, "testdata", "sample-shop", "capabilities", "checkout",
		"checkout.feature")
	now, err := ownTags(path, name)
	if err != nil {
		return err
	}
	before, err := ownTags(original, name)
	if err != nil {
		return err
	}
	if slices.Contains(now, tag) != want {
		return fmt.Errorf("%q's own tags are %q", name, now)
	}
	without := func(tags []string) []string {
		return slices.DeleteFunc(slices.Clone(tags), func(each string) bool { return each == tag })
	}
	if !slices.Equal(without(now), without(before)) {
		return fmt.Errorf("%q's other tags became %q, from %q", name, now, before)
	}
	nowLines, err := linesWithoutTagsAbove(path, "Scenario: "+name)
	if err != nil {
		return err
	}
	beforeLines, err := linesWithoutTagsAbove(original, "Scenario: "+name)
	if err != nil {
		return err
	}
	if !slices.Equal(nowLines, beforeLines) {
		return fmt.Errorf("checkout.feature changed beyond %q's tag lines:\n%s", name,
			strings.Join(nowLines, "\n"))
	}
	return nil
}

func (s *state) listedUnderTag(tag string) error {
	if err := s.selectTag(tag); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if names := sortedNames(window.ListedScenarios()); !slices.Contains(names, s.subject) {
		return fmt.Errorf("%q lists %q, not %q", tag, names, s.subject)
	}
	return nil
}

// flagged expects the scenario flagged to do where the window shows it, as the TUI's list
// flags it ⚑.
func (s *state) flagged(name string) error {
	s.subject = name
	flagged, err := s.ShownFlagged(name)
	if err != nil {
		return err
	}
	if !flagged {
		return fmt.Errorf("%q is shown without its to do flag", name)
	}
	return nil
}

func (s *state) notFlagged() error {
	flagged, err := s.ShownFlagged(s.subject)
	if err != nil {
		return err
	}
	if flagged {
		return fmt.Errorf("%q is still shown flagged to do", s.subject)
	}
	return nil
}

// ownTags lists the tags on the lines above a scenario's Scenario: line, as the parser reads
// them. Ref: https://github.com/cucumber/messages
func ownTags(path string, name string) ([]string, error) {
	scenarios, err := spec.ScenariosIn(path)
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(scenarios, func(each *messages.Scenario) bool {
		return each.Name == name
	})
	if index < 0 {
		return nil, fmt.Errorf("%s has no scenario %q", filepath.Base(path), name)
	}
	tags := make([]string, 0, len(scenarios[index].Tags))
	for _, tag := range scenarios[index].Tags {
		tags = append(tags, tag.Name)
	}
	return tags, nil
}

// linesWithoutTagsAbove is a file's lines, less the tag lines just above header.
func linesWithoutTagsAbove(path string, header string) ([]string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(text), "\n")
	at := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimSpace(line) == header
	})
	if at < 0 {
		return nil, fmt.Errorf("%s has no line %q", filepath.Base(path), header)
	}
	top := at
	// bound: top counts down to 0 at most.
	for top > 0 && strings.HasPrefix(strings.TrimSpace(lines[top-1]), "@") {
		top--
	}
	return slices.Delete(lines, top, at), nil
}
