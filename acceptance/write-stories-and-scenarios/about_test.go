package writestoriesandscenarios

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// aboutSteps cover opening what's on show to edit, and a capability's about.md, or starting
// one.
func (s *state) aboutSteps() []world.Step {
	return []world.Step{
		world.Def(`^([a-z0-9-]+/[a-z0-9-]+\.(?:md|feature)) opens to edit$`, s.aboutInEditor),
		world.Def(`^(?:they press e|they edit what's on show)$`, s.pressE),
		world.Def(`^([a-z0-9-]+/[a-z0-9-]+\.feature) opens to edit at the "([^"]*)" block$`,
			s.inEditorAtBlock),
		world.Def(`^it opens to edit$`, s.opensToEdit),
		world.Def(`^Invites has no about\.md$`, s.invitesHasNoAbout),
		world.Def(`^([a-z0-9-]+/about\.md) is created, starting with the line "([^"]*)"$`,
			s.aboutCreated),
	}
}

// aboutInEditor checks path, a file in the capabilities folder, is open to edit.
func (s *state) aboutInEditor(path string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	want := filepath.Join(s.Capabilities(), filepath.FromSlash(path))
	if editing := window.Editing(); editing != want {
		return fmt.Errorf("the editor holds %q, not %s", editing, want)
	}
	return nil
}

// invitesHasNoAbout opens the sample shop, whose Invites has no about.md, and checks so.
func (s *state) invitesHasNoAbout() error {
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	path := filepath.Join(s.Capabilities(), "invites", "about.md")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("invites/about.md is there: %v", err)
	}
	return nil
}

func (s *state) aboutCreated(path string, line string) error {
	text, err := s.ReadFile(filepath.Join("capabilities", filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	if first, _, _ := strings.Cut(text, "\n"); first != line {
		return fmt.Errorf("%s starts with %q, not %q", path, first, line)
	}
	return nil
}

// opensToEdit checks something is open to edit: what the TUI and givens edit hand to the
// developer's editor.
func (s *state) opensToEdit() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if window.Editing() == "" {
		return errors.New("nothing is open to edit")
	}
	return nil
}

// pressE opens what's on show to edit: the selected scenario at its block, or the story's
// .feature file.
func (s *state) pressE() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Press(app.KeyEdit)
}

// inEditorAtBlock checks path is open to edit at the named scenario's block.
func (s *state) inEditorAtBlock(path string, name string) error {
	if err := s.aboutInEditor(path); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if editing := window.Editor().Scenario(); editing != name {
		return fmt.Errorf("the editor is at %q, not %q", editing, name)
	}
	return nil
}
