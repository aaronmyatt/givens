package trustaproject

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// choseInEarlierApp answers the trust prompt in an earlier run of Givens on this scenario's
// app data, then quits it, so only what Givens stored outside the project carries over.
func (s *state) choseInEarlierApp(choice string) error {
	earlier, err := app.New(app.Config{
		AppDataDir: s.AppData, Clipboard: s.Clipboard, Trash: s.Trash, Clock: s.Clock,
	})
	if err != nil {
		return err
	}
	window, err := earlier.Open(s.Capabilities())
	if err != nil {
		return fmt.Errorf("an earlier Givens opening the sample shop: %w", err)
	}
	if err := s.chooseOnPrompt(window, choice); err != nil {
		return err
	}
	if err := earlier.Quit(); err != nil {
		return err
	}
	return s.forgetRun()
}

// forgetRun removes the marker an earlier run left, so later steps see only new runs.
func (s *state) forgetRun() error {
	if err := os.Remove(s.marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *state) choseBefore(choice string) error {
	if err := s.arrangeShop(); err != nil {
		return err
	}
	return s.choseInEarlierApp(choice)
}

// choseNow answers the prompt in this scenario's own app, which stays open.
func (s *state) choseNow(choice string) error {
	if err := s.arrangeShop(); err != nil {
		return err
	}
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	if err := s.chooseOnPrompt(s.Window, choice); err != nil {
		return err
	}
	if s.ran() {
		return fmt.Errorf("choosing %q ran the setup", choice)
	}
	return nil
}

func (s *state) startsWithoutAsking() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := world.ExpectNot(window.Visible(), trustPrompt, "the window"); err != nil {
		return err
	}
	if err := world.ExpectNot(window.Offers(), "Don't trust", "the offered actions"); err != nil {
		return err
	}
	return s.startedRunning()
}

// dropInWithoutRunning expects the Drop in page to show Card declined's steps and its setup
// script's text, while the script stays unrun.
func (s *state) dropInWithoutRunning() error {
	if s.Session == nil {
		return errors.New("dropping in opened no Drop in page")
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	scenarios, err := app.ScenariosIn(filepath.Join(s.Capabilities(), "checkout",
		"checkout.feature"))
	if err != nil {
		return err
	}
	for _, scenario := range scenarios {
		if scenario.Name != scenarioName {
			continue
		}
		for _, step := range scenario.Steps {
			if err := world.Expect(window.Visible(), step.Text, "the Drop in page"); err != nil {
				return err
			}
		}
	}
	if err := world.Expect(window.Visible(), s.script, "the Drop in page"); err != nil {
		return err
	}
	return s.stayedQuiet()
}

// seeAndCopyWorks expects the message, then copies the setup command with scripts off.
func (s *state) seeAndCopyWorks(text string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), text, "the window"); err != nil {
		return err
	}
	if err := window.Choose("Copy"); err != nil {
		return err
	}
	copied, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if !strings.Contains(copied, "card-declined") {
		return fmt.Errorf("\"Copy\" put %q on the clipboard, not Card declined's setup", copied)
	}
	return s.stayedQuiet()
}

// trustedAtShop moves the copied shop to <scenario>/home/code/shop, standing in for
// ~/code/shop, and trusts it in an earlier run of Givens.
func (s *state) trustedAtShop() error {
	if err := s.arrangeShop(); err != nil {
		return err
	}
	shop := filepath.Join(s.Dir, "home", "code", "shop")
	if err := os.MkdirAll(filepath.Dir(shop), 0o755); err != nil {
		return err
	}
	if err := os.Rename(s.Project, shop); err != nil {
		return err
	}
	s.Project = shop
	return s.choseInEarlierApp("Trust")
}

// openClone copies the trusted shop to shop-2, as a fresh clone would be, and drops in there.
func (s *state) openClone() error {
	clone := filepath.Join(filepath.Dir(s.Project), "shop-2")
	// CopyFS copies a whole tree. Ref: https://pkg.go.dev/os#CopyFS
	if err := os.CopyFS(clone, os.DirFS(s.Project)); err != nil {
		return err
	}
	s.Project = clone
	if err := s.StartApp(); err != nil {
		return err
	}
	window, err := s.App.Open(s.Capabilities())
	if err != nil {
		return fmt.Errorf("opening the clone: %w", err)
	}
	s.Window = window
	return s.DropIn(scenarioName)
}

func (s *state) askedAgain(text string) error {
	if err := s.seeWithPath(text); err != nil {
		return err
	}
	return s.stayedQuiet()
}
