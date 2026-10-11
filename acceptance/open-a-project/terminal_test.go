package openaproject

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// openCommandEnv names the program the givens command runs in place of macOS's `open`, so a
// test sees what it asked for without launching the real app. Unset, givens runs `open`.
// Ref: https://ss64.com/mac/open.html
const openCommandEnv = "GIVENS_OPEN_COMMAND"

// terminalSteps run the givens command the way a developer would after installing it.
func terminalSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has installed the givens command$`, s.installedCommand),
		world.Def(`^they run "([^"]*)" in the sample shop's folder$`, s.runInShop),
		world.Def(`^the command prints "([^"]*)"$`, s.commandPrints),
		world.Def(`^Givens doesn't open a window$`, s.noWindowOpened),
	}
}

// installedCommand puts a givens on PATH that runs the built command with `open` swapped
// for a recorder, and checks ~/code/nope really is missing on this Mac.
func (s *state) installedCommand() error {
	if strings.ContainsAny(s.Dir+s.Bin, "'\n") {
		return fmt.Errorf("the scenario's folders %q and %q need quoting", s.Dir, s.Bin)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(home, "code", "nope")); err == nil {
		return errors.New("~/code/nope exists on this Mac, so the scenario can't use it")
	}
	s.openLog = filepath.Join(s.Dir, "open-arguments.txt")
	recorder := filepath.Join(s.Dir, "open-recorder.sh")
	record := "#!/usr/bin/env bash\nprintf '%s\\n' \"$@\" >> '" + s.openLog + "'\n"
	if err := os.WriteFile(recorder, []byte(record), 0o755); err != nil {
		return err
	}
	installed := filepath.Join(s.Dir, "installed-bin")
	if err := os.MkdirAll(installed, 0o755); err != nil {
		return err
	}
	wrapper := "#!/usr/bin/env bash\nexport " + openCommandEnv + "='" + recorder + "'\n" +
		"exec '" + filepath.Join(s.Bin, "givens") + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(installed, "givens"), []byte(wrapper), 0o755); err != nil {
		return err
	}
	s.Bin = installed
	return nil
}

func (s *state) runInShop(command string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	s.Cwd = s.Project
	return s.Run(command)
}

// openRequests reads what the givens command passed to `open`, one argument per line.
func (s *state) openRequests() ([]string, error) {
	text, err := os.ReadFile(s.openLog)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return strings.Fields(string(text)), err
}

// openedFromTerminal expects `open -a Givens <folder>` with the sample shop's folder, made
// absolute, since `open` may run from another folder.
func (s *state) openedFromTerminal() error {
	if s.ExitCode != 0 {
		return fmt.Errorf("givens exited %d:\n%s", s.ExitCode, s.Output)
	}
	arguments, err := s.openRequests()
	if err != nil {
		return err
	}
	if len(arguments) == 0 {
		return fmt.Errorf("givens never ran %s, its stand-in for `open -a Givens`; "+
			"it printed:\n%s", openCommandEnv, s.Output)
	}
	if !slices.Equal(arguments[:min(2, len(arguments))], []string{"-a", "Givens"}) {
		return fmt.Errorf("givens ran open with %q, not -a Givens <folder>", arguments)
	}
	folder := arguments[len(arguments)-1]
	if !filepath.IsAbs(folder) {
		return fmt.Errorf("givens asked to open %q, not an absolute path", folder)
	}
	if !samePath(folder, s.Project) && !samePath(folder, s.Capabilities()) {
		return fmt.Errorf("givens asked to open %s, not the sample shop at %s", folder, s.Project)
	}
	return nil
}

func (s *state) commandPrints(text string) error {
	lines := strings.Split(s.Output, "\n")
	if err := world.Expect(lines, text, "the command's output"); err != nil {
		return err
	}
	if s.ExitCode == 0 {
		return fmt.Errorf("the command printed %q but exited 0", text)
	}
	return nil
}

func (s *state) noWindowOpened() error {
	arguments, err := s.openRequests()
	if err != nil {
		return err
	}
	if len(arguments) != 0 {
		return fmt.Errorf("givens asked Givens to open %q", arguments)
	}
	return nil
}

// windowSteps open several projects and quit with them open.
func windowSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^they open another project's capabilities folder$`, s.openOther),
		world.Def(`^it opens in a second window$`, s.secondWindow),
		world.Def(`^each window's title shows its own project's path$`, s.titlesShowPaths),
		world.Def(`^they open the sample shop's capabilities folder again$`, s.openAgain),
		world.Def(`^its existing window comes to the front$`, s.existingInFront),
		world.Def(`^no second window opens$`, s.noSecondWindow),
		world.Def(`^a developer has two projects open in Givens$`, s.twoOpen),
		world.Def(`^they quit Givens and open it again$`, s.quitAndRelaunch),
		world.Def(`^both projects reopen in their own windows$`, s.bothReopen),
	}
}

// openOther copies the sample shop to a second folder and opens that copy's capabilities.
func (s *state) openOther() error {
	project := filepath.Join(s.Dir, "other-shop")
	// CopyFS copies a whole tree. Ref: https://pkg.go.dev/os#CopyFS
	fixture := os.DirFS(filepath.Join(s.Root, "testdata", "sample-shop"))
	if err := os.CopyFS(project, fixture); err != nil {
		return err
	}
	s.other = filepath.Join(project, "capabilities")
	s.gotFront = s.Window
	return s.open(s.other)
}

func (s *state) secondWindow() error {
	if s.opened == s.gotFront {
		return errors.New("the other project opened in the sample shop's window")
	}
	return expectWindows(s.App, []string{s.Capabilities(), s.other})
}

func (s *state) titlesShowPaths() error {
	for _, window := range s.App.Windows() {
		if !containsPath(window.Title(), window.Path()) {
			return fmt.Errorf("a window showing %s has the title %q", window.Path(), window.Title())
		}
	}
	return expectWindows(s.App, []string{s.Capabilities(), s.other})
}

func (s *state) openAgain() error {
	s.gotFront = s.Window
	return s.open(s.Capabilities())
}

func (s *state) existingInFront() error {
	if s.opened != s.gotFront {
		return errors.New("opening the sample shop again returned a new window")
	}
	if s.App.Front() != s.gotFront {
		return errors.New("the sample shop's window isn't in front")
	}
	return nil
}

func (s *state) noSecondWindow() error {
	return expectWindows(s.App, []string{s.Capabilities()})
}

func (s *state) twoOpen() error {
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	return s.openOther()
}

// quitAndRelaunch quits this scenario's app and starts a new one on the same app data.
func (s *state) quitAndRelaunch() error {
	if err := s.App.Quit(); err != nil {
		return err
	}
	relaunched, err := s.freshApp()
	if err != nil {
		return err
	}
	s.App, s.Window = relaunched, nil
	return s.App.Launch()
}

func (s *state) bothReopen() error {
	for _, window := range s.App.Windows() {
		if window.Kind() != app.WindowProject {
			return fmt.Errorf("a %q window opened alongside the projects", window.Kind())
		}
	}
	return expectWindows(s.App, []string{s.Capabilities(), s.other})
}
