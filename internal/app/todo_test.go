package app

import (
	"slices"
	"strings"
	"testing"
)

// TestWithTodo covers the tag lines a flag edits: a new line, an existing line, a word taken
// off, a line taken out, and a @todo that isn't the scenario's own.
func TestWithTodo(t *testing.T) {
	cases := []struct {
		name    string
		before  string
		at      int
		flagged bool
		after   string
	}{
		{"flag with no tags", "  Scenario: A\n    Given x", 0, true,
			"  @todo\n  Scenario: A\n    Given x"},
		{"flag beside other tags", "  @error  @slow\n  Scenario: A", 1, true,
			"  @error  @slow @todo\n  Scenario: A"},
		{"unflag one of several", "  @error @todo @slow\n  Scenario: A", 1, false,
			"  @error @slow\n  Scenario: A"},
		{"unflag the only tag", "Feature: F\n\n  @todo\n  Scenario: A", 3, false,
			"Feature: F\n\n  Scenario: A"},
		{"other tag lines stay as written", "  @a  @b\n  @todo\n  Scenario: A", 2, false,
			"  @a  @b\n  Scenario: A"},
	}
	for _, each := range cases {
		got, err := withTodo(strings.Split(each.before, "\n"), each.at, each.flagged)
		if err != nil {
			t.Fatalf("%s: %v", each.name, err)
		}
		if want := strings.Split(each.after, "\n"); !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", each.name, got, want)
		}
	}
}

// TestWithTodoRefuses covers lines a flag mustn't touch: a @todo above the Feature line, and
// a line that isn't a scenario's.
func TestWithTodoRefuses(t *testing.T) {
	inherited := strings.Split("@todo\nFeature: F\n\n  Scenario: A", "\n")
	if _, err := withTodo(inherited, 3, false); err == nil {
		t.Error("unflagging a scenario whose @todo is on its Feature line didn't fail")
	}
	if _, err := withTodo([]string{"    Given x"}, 0, true); err == nil {
		t.Error("flagging a step line didn't fail")
	}
}
