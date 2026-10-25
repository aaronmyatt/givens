package keeprunsoffmain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

// git runs git in dir and returns its output, the final newline trimmed. A run line ends in
// tabs when it has no note, so nothing else is trimmed.
func git(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimRight(string(output), "\n"), nil
}

// gitAll runs each git command in dir, stopping at the first that fails.
func gitAll(dir string, commands ...[]string) error {
	for _, args := range commands {
		if _, err := git(dir, args...); err != nil {
			return err
		}
	}
	return nil
}

// gitWithRemote makes the sample shop a git repository, committed on main and pushed to a
// bare remote in the scenario's folder. A window already open is opened again, as a
// developer who just added a remote would, so Givens sees the repository.
func (s *state) gitWithRemote() error {
	if err := s.gitWithoutRemote(); err != nil {
		return err
	}
	s.remote = filepath.Join(s.Dir, "remote.git")
	if _, err := git(s.Dir, "init", "--quiet", "--bare", "--initial-branch=main",
		s.remote); err != nil {
		return err
	}
	if err := gitAll(s.Project,
		[]string{"remote", "add", "origin", s.remote},
		[]string{"push", "--quiet", "origin", "main"},
	); err != nil {
		return err
	}
	if s.Window == nil {
		return nil
	}
	if err := s.Window.CloseWindow(); err != nil {
		return err
	}
	s.Window, s.Session = nil, nil
	window, err := s.App.Open(s.Capabilities())
	s.Window = window
	if err != nil {
		return err
	}
	return world.WaitForRunsSync(window)
}

// remoteSilent makes the shop a git repository whose remote takes the connection and never
// answers: git's ext transport runs a command that only sleeps.
// Ref: https://git-scm.com/docs/git-remote-ext
// Ref: https://git-scm.com/docs/git-config#Documentation/git-config.txt-protocolallow
func (s *state) remoteSilent() error {
	if err := s.gitWithRemote(); err != nil {
		return err
	}
	return gitAll(s.Project,
		[]string{"config", "protocol.ext.allow", "always"},
		[]string{"remote", "set-url", "origin", "ext::sh -c sleep% 30"},
	)
}

// gitWithoutRemote makes the sample shop a git repository, committed on main.
func (s *state) gitWithoutRemote() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return gitAll(s.Project,
		[]string{"init", "--quiet", "--initial-branch=main"},
		[]string{"config", "user.name", "Sam"},
		[]string{"config", "user.email", "sam@example.com"},
		[]string{"add", "."},
		[]string{"commit", "--quiet", "-m", "Sample shop"},
	)
}

// moveRunsOffMain chooses "Move runs off main" and confirms it, from the project: a Drop in
// still open is left first, as the developer would close it.
func (s *state) moveRunsOffMain() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if window.Session() != nil {
		if err := window.Select("Checkout"); err != nil {
			return err
		}
		s.Session = nil
	}
	if err := window.Choose(app.OfferMoveRunsOffMain); err != nil {
		return err
	}
	return window.Choose(app.OfferMoveRunsOffMain)
}

// keepsOffMainWithRun opens the shop in a git repository with a remote, then records a
// passing run, which goes to the ref.
func (s *state) keepsOffMainWithRun(name string) error {
	if err := s.gitWithRemote(); err != nil {
		return err
	}
	return s.RecordPassingRun(name)
}

// keepsOffMainWithLimit opens the shop in a git repository with a remote, and sets the ref's
// run limit.
func (s *state) keepsOffMainWithLimit(count int) error {
	if err := s.gitWithRemote(); err != nil {
		return err
	}
	store, ok := app.RunStoreOf(s.Capabilities())
	if !ok {
		return fmt.Errorf("the sample shop isn't in a git repository")
	}
	limits := app.RunLimits{Runs: count, Megabytes: app.MegabytesKeptDefault}
	limit := func(tree string) error { return app.WriteRunLimits(tree, limits) }
	return store.Change(limit, app.RunsNetworkTimeout)
}

// keepsOffMainCloned opens the shop in a git repository with a remote, then clones it on
// another device.
func (s *state) keepsOffMainCloned() error {
	if err := s.gitWithRemote(); err != nil {
		return err
	}
	if err := s.OpenSampleShop(); err != nil {
		return err
	}
	return s.cloneOther()
}

// cloneOther clones the remote as another device, with an app of its own, and opens it.
func (s *state) cloneOther() error {
	folder := filepath.Join(s.Dir, "other-device")
	if _, err := git(s.Dir, "clone", "--quiet", s.remote, folder); err != nil {
		return err
	}
	appData := filepath.Join(s.Dir, "other-app-data")
	if err := os.MkdirAll(appData, 0o755); err != nil {
		return err
	}
	core, err := app.New(app.Config{
		AppDataDir: appData, Clipboard: s.Clipboard, Trash: s.Trash, Clock: s.Clock,
	})
	if err != nil {
		return err
	}
	capabilities := filepath.Join(folder, "capabilities")
	if err := core.MarkTrusted(capabilities); err != nil {
		return err
	}
	window, err := core.Open(capabilities)
	if err != nil {
		return err
	}
	if err := world.WaitForRunsSync(window); err != nil {
		return err
	}
	s.other = &device{app: core, window: window, capabilities: capabilities}
	return nil
}

// onOther runs a step against the other device's app and window.
func (s *state) onOther(step func() error) error {
	if s.other == nil {
		return fmt.Errorf("no other device was cloned")
	}
	app, window, session := s.App, s.Window, s.Session
	s.App, s.Window, s.Session = s.other.app, s.other.window, nil
	err := step()
	s.App, s.Window, s.Session = app, window, session
	return err
}

// recordedRuns writes count passing runs of a scenario into the ref, each with a screenshot,
// a minute apart and before the clock's now, and shares them.
func (s *state) recordedRuns(name string, count int) error {
	key, err := s.runKey(name)
	if err != nil {
		return err
	}
	store, ok := app.RunStoreOf(s.Capabilities())
	if !ok {
		return fmt.Errorf("the sample shop isn't in a git repository")
	}
	s.times = make([]time.Time, 0, count)
	for index := range count {
		at := s.Clock.Now().Add(-time.Duration(count-index) * time.Minute)
		s.times = append(s.times, at)
	}
	write := func(tree string) error {
		for _, at := range s.times {
			if err := writeShot(tree, key, at); err != nil {
				return err
			}
		}
		return nil
	}
	return store.Change(write, app.RunsNetworkTimeout)
}

// writeShot writes one passing run of key at at, with a screenshot, under root.
func writeShot(root string, key string, at time.Time) error {
	folder := app.AttachedFilesDir(root, key, at)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(folder, "shot.png"), []byte(at.String()),
		0o644); err != nil {
		return err
	}
	return app.AppendRun(root, app.Run{Time: at, Key: key, Result: app.RunResultPass})
}

// runKey is the run key of a sample shop scenario: <capability>/<slug>.
func (s *state) runKey(name string) (string, error) {
	folder, err := s.ScenarioCapability(name)
	if err != nil {
		return "", err
	}
	return folder + "/" + app.SlugOf(name), nil
}

// remoteRuns lists the runs the remote's refs/givens/runs holds.
func (s *state) remoteRuns() ([]app.Run, error) {
	names, err := git(s.remote, "ls-tree", "-r", "--name-only", app.RunsRef)
	if err != nil {
		return nil, err
	}
	found := make([]app.Run, 0, 8)
	for _, name := range strings.Split(names, "\n") {
		if !strings.HasPrefix(name, "runs/") || !strings.HasSuffix(name, ".log") ||
			strings.Count(name, "/") != 1 {
			continue
		}
		text, err := git(s.remote, "show", app.RunsRef+":"+name)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(text, "\n") {
			if run, err := app.ParseRun(line); err == nil {
				found = append(found, run)
			}
		}
	}
	return found, nil
}
