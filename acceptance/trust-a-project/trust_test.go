// Package trustaproject runs capabilities/trust-a-project. Steps shared with other
// capabilities are in acceptance/world.
//
// The Card declined setup touches a marker file outside the project, so a step can tell
// whether any of the project's script ran.
package trustaproject

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

const capability = "trust-a-project"

// The scenario every step drops in to, and the prompt Givens asks before running scripts.
const (
	scenarioName = "Card declined"
	trustPrompt  = "Run scripts from this project?"
)

// Polling: a fixed number of looks, a fixed pause apart.
const (
	pollInterval      = 100 * time.Millisecond
	quietAttempts     = 5                                   // how long "nothing ran" is watched
	setupPollAttempts = int(world.SetupWait / pollInterval) // how long a run may take to start
)

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	marker string // the file Card declined's setup touches when it runs
	script string // the line in the setup that touches it
}

func steps(w *world.World) []world.Step {
	// Trust is what these scenarios are about, so the shop starts untrusted.
	w.Untrusted = true
	s := &state{World: w}
	return []world.Step{
		world.Def(`^a developer has never run anything from the sample shop$`, s.neverRun),
		world.Def(`^they see "([^"]*)" with the project's path$`, s.seeWithPath),
		world.Def(`^nothing runs until they choose$`, s.nothingRunsUntilChoice),
		world.Def(`^a developer chose "([^"]*)" for the sample shop before$`, s.choseBefore),
		world.Def(`^setup starts without asking$`, s.startsWithoutAsking),
		world.Def(`^a developer chose "([^"]*)" for the sample shop$`, s.choseNow),
		world.Def(`^the Drop in screen opens with the scenario's steps and setup script, `+
			`but nothing in the script runs$`, s.dropInWithoutRunning),
		world.Def(`^they see "([^"]*)", and "Copy" still works$`, s.seeAndCopyWorks),
		world.Def(`^a developer trusted the sample shop at ~/code/shop$`, s.trustedAtShop),
		world.Def(`^they open a fresh clone of it at ~/code/shop-2 and drop in$`, s.openClone),
		world.Def(`^they're asked "([^"]*)" again$`, s.askedAgain),
	}
}

// arrangeShop copies the sample shop and makes Card declined's setup touch the marker.
func (s *state) arrangeShop() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	s.marker = filepath.Join(s.Dir, "card-declined-ran")
	if strings.Contains(s.marker, "'") {
		return fmt.Errorf("the marker path %q needs quoting", s.marker)
	}
	s.script = "touch '" + s.marker + "'"
	return s.SetupStartsWith(scenarioName, s.script)
}

// neverRun is a fresh copy of the shop and an app data folder with nothing in it.
func (s *state) neverRun() error {
	entries, err := os.ReadDir(s.AppData)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("the app data folder isn't empty: %d entries", len(entries))
	}
	return s.arrangeShop()
}

func (s *state) ran() bool {
	_, err := os.Stat(s.marker)
	return err == nil
}

// stayedQuiet watches the marker for a short, fixed time and fails if the setup ran.
func (s *state) stayedQuiet() error {
	for attempt := 0; attempt < quietAttempts; attempt++ {
		if s.ran() {
			return errors.New("the setup script ran")
		}
		time.Sleep(pollInterval)
	}
	return nil
}

// startedRunning waits up to world.SetupWait for the setup to touch the marker.
func (s *state) startedRunning() error {
	for attempt := 0; attempt < setupPollAttempts; attempt++ {
		if s.ran() {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("the setup didn't run within %s", world.SetupWait)
}

func (s *state) seeWithPath(text string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), text, "the window"); err != nil {
		return err
	}
	return expectPath(window.Visible(), s.Project)
}

// expectPath expects a line showing path, as given or with its links resolved: macOS's
// temporary folder /var is a link to /private/var.
// Ref: https://pkg.go.dev/path/filepath#EvalSymlinks
func expectPath(lines []string, path string) error {
	forms := []string{path}
	if target, err := filepath.EvalSymlinks(path); err == nil {
		forms = append(forms, target)
	}
	for _, form := range forms {
		if world.Expect(lines, form, "the window") == nil {
			return nil
		}
	}
	return fmt.Errorf("the window doesn't show the project's path %s; it has %q", path, lines)
}

// nothingRunsUntilChoice checks the setup waits on the question, then that choosing "Trust"
// lets it run, so the wait was for the choice.
func (s *state) nothingRunsUntilChoice() error {
	if err := s.stayedQuiet(); err != nil {
		return fmt.Errorf("before a choice was made, %w", err)
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := window.Choose("Trust"); err != nil {
		return err
	}
	return s.startedRunning()
}

// chooseOnPrompt drops in to Card declined in window, answers the prompt with choice, and
// waits for any setup that choice started.
func (s *state) chooseOnPrompt(window *app.Window, choice string) error {
	if _, err := window.DropIn(scenarioName); err != nil {
		return err
	}
	if err := window.Choose(choice); err != nil {
		return err
	}
	if session := window.Session(); session != nil {
		session.Wait(world.SetupWait)
	}
	return nil
}
