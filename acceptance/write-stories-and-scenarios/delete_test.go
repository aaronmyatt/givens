package writestoriesandscenarios

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"givens/acceptance/world"
)

func (s *state) deleteSteps() []world.Step {
	return []world.Step{
		world.Def(`^they delete "([^"]*)"$`, s.delete),
		world.Def(`^they confirm$`, s.confirmDelete),
		world.Def(`^its block is removed from checkout\.feature, and its setup script is moved `+
			`to the Trash$`, s.scenarioDeleted),
		world.Def(`^Checkout shows (\d+) scenarios$`, s.checkoutShows),
		world.Def(`^nothing is deleted unless they choose "([^"]*)"$`, s.deletedOnlyWhen),
		world.Def(`^the invites folder is moved to the Trash$`, s.invitesTrashed),
		world.Def(`^the sidebar lists "([^"]*)" and "([^"]*)"$`, s.sidebarLists),
		world.Def(`^a developer has chosen "([^"]*)" on "([^"]*)"$`, s.editFromPage),
		world.Def(`^([a-z0-9-]+\.(?:md|sh)) is moved to the Trash$`, s.setupFileTrashed),
		world.Def(`^the editor closes$`, s.editorClosed),
	}
}

// delete asks to delete a scenario or capability, remembering every file before it.
func (s *state) delete(name string) error {
	if err := s.arrangeSetups(); err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	s.deleting, s.deletingFolder, s.deletingScript = name, "", ""
	capabilitySlug, err := s.capabilityNamed(name)
	if err != nil {
		return err
	}
	if capabilitySlug != "" {
		s.deletingFolder = filepath.Join(s.Capabilities(), capabilitySlug)
	} else if err := s.rememberDeletedScenario(name); err != nil {
		return err
	}
	if s.before, err = snapshot(s.Capabilities(), everyFile); err != nil {
		return err
	}
	return window.Delete(name)
}

func (s *state) rememberDeletedScenario(name string) error {
	capabilitySlug, err := s.ScenarioCapability(name)
	if err != nil {
		return err
	}
	script, err := s.ScriptPath(name)
	if err != nil {
		return err
	}
	s.actedOn = capabilitySlug
	s.deletingScript = filepath.Join(s.Project, script)
	s.featureBefore, err = s.ReadFile(featurePath(capabilitySlug))
	return err
}

func (s *state) confirmDelete() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	return window.Choose("Delete")
}

// withoutBlock is text with the named scenario's block removed: its tags, its Scenario: line,
// its steps, and the blank line before it.
func withoutBlock(text string, name string) (string, error) {
	lines := strings.Split(text, "\n")
	start := slices.Index(trimmedLines(text), "Scenario: "+name)
	if start < 0 {
		return "", fmt.Errorf("there's no scenario %q to remove", name)
	}
	end := start + 1
	for end < len(lines) && strings.HasPrefix(lines[end], "    ") { // bound: len(lines)
		end++
	}
	first := start
	for first > 0 && strings.HasPrefix(strings.TrimSpace(lines[first-1]), "@") { // bound: start
		first--
	}
	if first > 0 && strings.TrimSpace(lines[first-1]) == "" {
		first--
	}
	return strings.Join(slices.Concat(lines[:first], lines[end:]), "\n"), nil
}

// trashed expects path gone and moved to the Trash. Paths are compared by folder once
// symlinks are resolved, since macOS reaches its temporary folder through a link.
func (s *state) trashed(path string) error {
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s is still there: %v", path, err)
	}
	wantDir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	for _, moved := range s.Trash.Moved {
		gotDir, err := filepath.EvalSymlinks(filepath.Dir(moved))
		if err == nil && gotDir == wantDir && filepath.Base(moved) == filepath.Base(path) {
			return nil
		}
	}
	return fmt.Errorf("%s wasn't moved to the Trash; the Trash got %q", path, s.Trash.Moved)
}

func (s *state) scenarioDeleted() error {
	want, err := withoutBlock(s.featureBefore, s.deleting)
	if err != nil {
		return err
	}
	got, err := s.ReadFile(featurePath(s.actedOn))
	if err != nil {
		return err
	}
	if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
		return fmt.Errorf("%s.feature is\n%s\nnot\n%s", s.actedOn, got, want)
	}
	return s.trashed(s.deletingScript)
}

func (s *state) checkoutShows(count string) error {
	want, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	_, names, err := s.scenarioSteps("checkout")
	if err != nil {
		return err
	}
	if len(names) != want {
		return fmt.Errorf("checkout.feature has %d scenarios, not %d", len(names), want)
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	for _, item := range window.Sidebar() {
		if item.Name == "Checkout" && item.Kind == "capability" && item.Count == want {
			return nil
		}
	}
	return fmt.Errorf("the sidebar doesn't show Checkout with %d scenarios: %v", want,
		window.Sidebar())
}

// deletedOnlyWhen expects nothing deleted yet, then chooses action and expects the delete.
func (s *state) deletedOnlyWhen(action string) error {
	now, err := snapshot(s.Capabilities(), everyFile)
	if err != nil {
		return err
	}
	if err := sameFiles(s.before, now); err != nil {
		return fmt.Errorf("something was deleted before %q: %w", action, err)
	}
	if len(s.Trash.Moved) > 0 {
		return fmt.Errorf("the Trash got %q before %q", s.Trash.Moved, action)
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	if offers := window.Offers(); !slices.Contains(offers, action) {
		return fmt.Errorf("the window doesn't offer %q; it offers %q", action, offers)
	}
	if err := window.Choose(action); err != nil {
		return err
	}
	if s.deletingFolder != "" {
		return s.trashed(s.deletingFolder)
	}
	return s.scenarioDeleted()
}

func (s *state) invitesTrashed() error {
	return s.trashed(filepath.Join(s.Capabilities(), "invites"))
}

func (s *state) sidebarLists(first string, second string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	capabilities := make([]string, 0, 4)
	for _, item := range window.Sidebar() {
		if item.Kind == "capability" {
			capabilities = append(capabilities, item.Name)
		}
	}
	slices.Sort(capabilities)
	want := []string{first, second}
	slices.Sort(want)
	if !slices.Equal(capabilities, want) {
		return fmt.Errorf("the sidebar lists the capabilities %q, not %q", capabilities, want)
	}
	return nil
}

// editFromPage selects a scenario and chooses an edit offer on its page, like "Edit setup".
func (s *state) editFromPage(action string, name string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Select(name); err != nil {
		return err
	}
	if err := window.Choose(action); err != nil {
		return err
	}
	if window.Editing() == "" {
		return fmt.Errorf("%q opened nothing in the editor", action)
	}
	return nil
}

func (s *state) setupFileTrashed(file string) error {
	return s.trashed(filepath.Join(s.Capabilities(), "checkout", "scripts", file))
}

func (s *state) editorClosed() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if window.Editor() != nil {
		return fmt.Errorf("the editor still holds %s", window.Editing())
	}
	return nil
}
