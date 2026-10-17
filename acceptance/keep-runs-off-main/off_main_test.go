// Package keeprunsoffmain runs capabilities/keep-runs-off-main. Steps shared with other
// capabilities are in acceptance/world.
package keeprunsoffmain

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/runref"
	"givens/internal/runs"
)

const capability = "keep-runs-off-main"

func TestScenarios(t *testing.T) {
	// git reads only the repositories the steps make, not the developer's own config.
	// Ref: https://git-scm.com/docs/git#Documentation/git.txt-GITCONFIGGLOBAL
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	world.RunSuite(t, capability, steps)
}

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// device is the other device a scenario clones the shop onto: its own app and window.
type device struct {
	app          *app.App
	window       *app.Window
	capabilities string
}

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	remote  string      // the bare remote the shop pushes to
	other   *device     // the other device, once cloned
	times   []time.Time // the runs a Given step recorded, oldest first
	dropped []time.Time // those a trim was expected to drop
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^the sample shop is a git repository with a remote$`, s.gitWithRemote),
		world.Def(`^the sample shop is a git repository without a remote$`, s.gitWithoutRemote),
		world.Def(`^"([^"]*)" has a passing run recorded in capabilities/runs$`,
			s.RecordPassingRun),
		world.Def(`^a developer chooses "Move runs off main" and confirms$`, s.moveRunsOffMain),
		world.Def(`^capabilities/runs holds the run of "([^"]*)"$`, s.workingTreeHolds),
		world.Def(`^the remote's refs/givens/runs holds the run of "([^"]*)"$`,
			func(name string) error { return s.remoteHolds(name) }),
		world.Def(`^the remote's refs/givens/runs holds the runs of "([^"]*)" and "([^"]*)"$`,
			func(first string, second string) error {
				return s.remoteHolds(first, second)
			}),
		world.Def(`^capabilities/runs holds only off-main\.md$`, s.onlyMarker),
		world.Def(`^the sample shop is a git repository with a remote, with a passing run of `+
			`"([^"]*)"$`, s.keepsOffMainWithRun),
		world.Def(`^the sample shop is a git repository with a remote, cloned on another `+
			`device$`, s.keepsOffMainCloned),
		world.Def(`^the sample shop is a git repository with a remote, with a limit of (\d+) `+
			`runs$`, func(count string) error { return s.withLimit(count) }),
		world.Def(`^a developer records a passing run of "([^"]*)"$`, s.RecordPassingRun),
		world.Def(`^git status shows nothing changed$`, s.nothingChanged),
		world.Def(`^a developer clones it on another device and opens it in Givens$`,
			s.openOther),
		world.Def(`^a passing run of "([^"]*)" is recorded on the other device$`,
			func(name string) error {
				return s.onOther(func() error { return s.RecordPassingRun(name) })
			}),
		world.Def(`^its remote can't be reached$`, s.remoteAway),
		world.Def(`^a passing run of "([^"]*)" was recorded while its remote couldn't be `+
			`reached$`, s.recordedAway),
		world.Def(`^the remote is back and a developer chooses "([^"]*)"$`, s.backAndChoose),
		world.Def(`^they don't see "([^"]*)"$`, s.notSee),
		world.Def(`^"([^"]*)" has (\d+) runs recorded, each with a screenshot$`,
			func(name string, count string) error { return s.withRuns(name, count) }),
		world.Def(`^the remote's refs/givens/runs holds the newest (\d+) runs of "([^"]*)"$`,
			s.remoteHoldsNewest),
		world.Def(`^its history holds none of the older runs' screenshots$`, s.historyDropped),
	}
}

// remoteHolds expects a run of each named scenario in the remote's ref.
func (s *state) remoteHolds(names ...string) error {
	held, err := s.remoteRuns()
	if err != nil {
		return err
	}
	for _, name := range names {
		key, err := s.runKey(name)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(held, func(run runs.Run) bool { return run.Key == key }) {
			return fmt.Errorf("the remote's %s holds no run of %s: %+v", runref.Ref, key, held)
		}
	}
	return nil
}

// workingTreeHolds expects a run of the scenario in capabilities/runs itself.
func (s *state) workingTreeHolds(name string) error {
	key, err := s.runKey(name)
	if err != nil {
		return err
	}
	held, err := runs.ReadFolder(s.Capabilities())
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(held, func(run runs.Run) bool { return run.Key == key }) {
		return fmt.Errorf("capabilities/runs holds no run of %s: %+v", key, held)
	}
	return nil
}

func (s *state) onlyMarker() error {
	entries, err := os.ReadDir(runs.Dir(s.Capabilities()))
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{runs.OffMainFileName}) {
		return fmt.Errorf("capabilities/runs holds %q", names)
	}
	return nil
}

func (s *state) withLimit(count string) error {
	limit, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	return s.keepsOffMainWithLimit(limit)
}

func (s *state) nothingChanged() error {
	status, err := git(s.Project, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("git status shows changes:\n%s", status)
	}
	return nil
}

// openOther clones the shop on another device and makes it the window the next steps look at.
func (s *state) openOther() error {
	if err := s.cloneOther(); err != nil {
		return err
	}
	s.App, s.Window, s.Session = s.other.app, s.other.window, nil
	return nil
}

func (s *state) remoteAway() error { return os.Rename(s.remote, s.remote+".away") }

func (s *state) recordedAway(name string) error {
	if err := s.remoteAway(); err != nil {
		return err
	}
	return s.RecordPassingRun(name)
}

func (s *state) backAndChoose(action string) error {
	if err := os.Rename(s.remote+".away", s.remote); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	return window.Choose(action)
}

func (s *state) notSee(text string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	return world.ExpectNot(window.Visible(), text, "the window")
}

func (s *state) withRuns(name string, count string) error {
	runCount, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	return s.recordedRuns(name, runCount)
}

// remoteHoldsNewest expects count runs of the scenario in the remote's ref, none of them the
// oldest that a Given step recorded.
func (s *state) remoteHoldsNewest(count string, name string) error {
	want, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	key, err := s.runKey(name)
	if err != nil {
		return err
	}
	held, err := s.remoteRuns()
	if err != nil {
		return err
	}
	mine := slices.DeleteFunc(held, func(run runs.Run) bool { return run.Key != key })
	if len(mine) != want {
		return fmt.Errorf("the remote holds %d runs of %s, not %d", len(mine), key, want)
	}
	// The run the When step recorded is newer than each of the Given step's, so those kept
	// are its newest want-1, and the rest were dropped.
	s.dropped = s.times[:max(len(s.times)-(want-1), 0)]
	for _, old := range s.dropped {
		if slices.ContainsFunc(mine, func(run runs.Run) bool { return run.Time.Equal(old) }) {
			return fmt.Errorf("the remote still holds the run at %s", old.Format(time.RFC3339))
		}
	}
	return nil
}

// historyDropped expects no commit of the remote's ref to hold a dropped run's files folder,
// which is named after the run's time.
func (s *state) historyDropped() error {
	if len(s.dropped) == 0 {
		return fmt.Errorf("no step said which runs were dropped")
	}
	objects, err := git(s.remote, "rev-list", "--objects", runref.Ref)
	if err != nil {
		return err
	}
	for _, old := range s.dropped {
		if folder := runs.FilesDir("/", "x/y", old); strings.Contains(objects,
			filepath.Base(folder)) {
			return fmt.Errorf("the ref's history still holds %s", filepath.Base(folder))
		}
	}
	return nil
}
