package openaproject

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// welcomeSteps launch Givens: the welcome window, recents, the Dock and the Givens menu.
func welcomeSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has never opened a project in Givens$`, s.neverOpened),
		world.Def(`^they open Givens$`, s.launch),
		world.Def(`^they see a welcome window with "([^"]*)"$`, s.welcomeWith),
		world.Def(`^choosing "([^"]*)" shows a folder picker$`, s.showsPicker),
		world.Def(`^a developer has opened the sample shop project in Givens before$`,
			s.openedBefore),
		world.Def(`^no project was open when they last quit Givens$`, s.quitWithNothingOpen),
		world.Def(`^the welcome window lists the sample shop under "([^"]*)"$`, s.listsRecent),
		world.Def(`^choosing it opens the project$`, s.chooseRecent),
		world.Def(`^a developer has Givens in their Dock$`, s.inDock),
		world.Def(`^they drag the sample shop's folder onto it$`, s.dropOnDock),
		world.Def(`^the sample shop project opens in Givens$`, s.shopOpens),
		world.Def(`^a developer has Givens open$`, s.givensOpen),
		world.Def(`^they choose "([^"]*)" from the Givens menu$`, s.chooseFromMenu),
		world.Def(`^givens works in new terminal windows$`, s.worksInNewTerminals),
		world.Def(`^Givens says where it installed it$`, s.saysWhere),
	}
}

// freshApp is another run of Givens on this scenario's app data folder and fakes: one
// run earlier, or the one launched after a quit.
func (s *state) freshApp() (*app.App, error) {
	return app.New(app.Config{
		AppDataDir: s.AppData, Clipboard: s.Clipboard, Trash: s.Trash, Clock: s.Clock,
	})
}

func (s *state) neverOpened() error {
	entries, err := os.ReadDir(s.AppData)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("the app data folder isn't empty: %d entries", len(entries))
	}
	return s.StartApp()
}

func (s *state) launch() error {
	if err := s.StartApp(); err != nil {
		return err
	}
	return s.App.Launch()
}

// welcome returns the front window, failing unless it's the only window and the welcome one.
func (s *state) welcome() (*app.Window, error) {
	front := s.App.Front()
	if front == nil {
		return nil, errors.New("no window is open")
	}
	if front.Kind() != app.WindowWelcome {
		return nil, fmt.Errorf("the front window is a %q window, not the welcome window",
			front.Kind())
	}
	if count := len(s.App.Windows()); count != 1 {
		return nil, fmt.Errorf("%d windows are open, not just the welcome window", count)
	}
	s.Window = front
	return front, nil
}

func (s *state) welcomeWith(action string) error {
	window, err := s.welcome()
	if err != nil {
		return err
	}
	return world.Expect(window.Offers(), action, "the welcome window's actions")
}

func (s *state) showsPicker(action string) error {
	window, err := s.welcome()
	if err != nil {
		return err
	}
	if err := window.Choose(action); err != nil {
		return err
	}
	if !window.FolderPickerShown() {
		return fmt.Errorf("choosing %q showed no folder picker", action)
	}
	return nil
}

// openedBefore runs an earlier Givens that opens the sample shop and quits with it open.
func (s *state) openedBefore() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	earlier, err := s.freshApp()
	if err != nil {
		return err
	}
	if _, err := earlier.Open(s.Capabilities()); err != nil {
		return fmt.Errorf("an earlier Givens opening the sample shop: %w", err)
	}
	return earlier.Quit()
}

// quitWithNothingOpen runs Givens once more, closes what it reopened, and quits, so the next
// launch has no windows to restore.
func (s *state) quitWithNothingOpen() error {
	earlier, err := s.freshApp()
	if err != nil {
		return err
	}
	if err := earlier.Launch(); err != nil {
		return err
	}
	for _, window := range earlier.Windows() {
		if window.Kind() != app.WindowProject {
			continue
		}
		if err := window.CloseWindow(); err != nil {
			return err
		}
	}
	return earlier.Quit()
}

func (s *state) listsRecent(heading string) error {
	window, err := s.welcome()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), heading, "the welcome window"); err != nil {
		return err
	}
	recents := s.App.Recents()
	for _, recent := range recents {
		if samePath(recent, s.Capabilities()) || samePath(recent, s.Project) {
			s.recent = recent
			return nil
		}
	}
	return fmt.Errorf("Recents is %q, without the sample shop at %s", recents, s.Project)
}

func (s *state) chooseRecent() error {
	window, err := s.welcome()
	if err != nil {
		return err
	}
	if s.recent == "" {
		return errors.New("no step found the sample shop under Recent")
	}
	opened, err := window.OpenRecent(s.recent)
	if err != nil {
		return err
	}
	if opened == nil || opened.Kind() != app.WindowProject {
		return errors.New("choosing the recent project opened no project window")
	}
	if !samePath(opened.Path(), s.Capabilities()) {
		return fmt.Errorf("the window shows %q, not %q", opened.Path(), s.Capabilities())
	}
	return nil
}

// inDock has the sample shop on disk and Givens built but not launched: a drop on the Dock
// icon launches it.
func (s *state) inDock() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.StartApp()
}

func (s *state) dropOnDock() error {
	return s.App.OpenDropped([]string{s.Project})
}

// shopOpens checks the project opened: through the stand-in for `open` when the givens
// command was run, otherwise in this scenario's app.
func (s *state) shopOpens() error {
	if s.openLog != "" {
		return s.openedFromTerminal()
	}
	if err := expectWindows(s.App, []string{s.Capabilities()}); err != nil {
		return err
	}
	front := s.App.Front()
	if front == nil || !samePath(front.Path(), s.Capabilities()) {
		return errors.New("the sample shop's window isn't in front")
	}
	return nil
}

func (s *state) givensOpen() error {
	return s.launch()
}

// chooseFromMenu installs the command into a folder of this scenario, standing in for the
// PATH folder the Swift UI picks.
func (s *state) chooseFromMenu(item string) error {
	if item != "Install givens command" {
		return fmt.Errorf("the Givens menu has no step for %q", item)
	}
	s.install = filepath.Join(s.Dir, "path-bin")
	if err := os.MkdirAll(s.install, 0o755); err != nil {
		return err
	}
	message, err := s.App.InstallCommand(s.install)
	s.message = message
	return err
}

// worksInNewTerminals starts a fresh shell whose PATH holds the install folder, as a new
// terminal window would, and expects it to find the installed givens there.
func (s *state) worksInNewTerminals() error {
	installed := filepath.Join(s.install, "givens")
	info, err := os.Stat(installed)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s isn't executable", installed)
	}
	// command -v prints the path a command name resolves to.
	// Ref: https://pubs.opengroup.org/onlinepubs/9799919799/utilities/command.html
	shell := exec.Command("bash", "-c", "command -v givens")
	shell.Env = []string{"PATH=" + s.install + ":/usr/bin:/bin", "HOME=" + s.Dir}
	output, err := shell.CombinedOutput()
	if err != nil {
		return fmt.Errorf("a new shell can't find givens: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != installed {
		return fmt.Errorf("a new shell finds givens at %q, not %q", got, installed)
	}
	return nil
}

func (s *state) saysWhere() error {
	if !containsPath(s.message, s.install) {
		return fmt.Errorf("Givens said %q, which doesn't name %s", s.message, s.install)
	}
	return nil
}
