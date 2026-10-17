package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/runref"
	"givens/internal/runs"
)

// In a git repository with a remote, a project keeps its runs off main by default: in the git
// ref refs/givens/runs, which Givens fetches and pushes itself; internal/runref does the git
// work. Without a remote, runs go in capabilities/runs. See capabilities/keep-runs-off-main.

// Offers and messages about runs kept off main.
const (
	OfferMoveRunsOffMain = "Move runs off main"
	OfferShareRuns       = "Share runs"
	// messageMoveOffMain asks before runs already in capabilities/runs are moved.
	messageMoveOffMain = "Move the runs in capabilities/runs to refs/givens/runs on " +
		runref.Remote + "? They go to the Trash here, and new runs already go to the ref."
	// MessageRunsMoved says what's left for the developer to do: commit the change.
	MessageRunsMoved = "Runs moved off main. Commit the change to capabilities/runs: the " +
		"old run files went to the Trash, and off-main.md says where runs are."
)

// offMainText is capabilities/runs/off-main.md: what anyone looking in runs/ reads.
const offMainText = `# Runs are kept off main

Givens keeps this project's runs in the git ref ` + "`" + runref.Ref + "`" + `, not on a
branch. It fetches and pushes the ref itself, so runs aren't in pull requests, and a plain
clone doesn't download them. See capabilities/keep-runs-off-main in the Givens repository.

To look by hand:

    git fetch origin ` + runref.Ref + `
    git ls-tree -r FETCH_HEAD
`

// openRunStore finds the window's runs store when its project is in a git repository with a
// remote, and syncs it, so another device's runs show up. With nothing here and nothing on
// the remote, the sync makes nothing.
func (w *Window) openRunStore() error {
	assert.That(w != nil, "open run store runs on a window")
	assert.That(w.branch == "", "a branch's window keeps no runs of its own")
	store, ok := runref.Open(w.path)
	if !ok || !w.found || !store.HasRemote() {
		return nil
	}
	w.runStore = &store
	return w.syncRuns(runref.OpenTimeout)
}

// keepsRunsOffMain is true when the window's runs go to the ref: in a git repository with a
// remote.
func (w *Window) keepsRunsOffMain() bool {
	assert.That(w != nil, "keeps runs off main runs on a window")
	assert.That(w.runStore == nil || w.branch == "", "only a working tree keeps runs")
	return w.runStore != nil
}

// runsOnMain lists the day logs still in capabilities/runs: runs recorded before the project
// had a remote, which "Move runs off main" moves.
func (w *Window) runsOnMain() []string {
	assert.That(w != nil, "runs on main runs on a window")
	assert.That(filepath.IsAbs(w.path) || w.path == "", "a project's path is absolute")
	logs, err := filepath.Glob(filepath.Join(runs.Dir(w.path), "*.log"))
	if err != nil {
		return []string{}
	}
	return logs
}

// SyncRuns shares runs kept off main, and fetches other devices' runs: the Mac app calls it
// every few minutes. A remote that can't be reached is said in the window, not returned.
func (w *Window) SyncRuns() error {
	assert.That(w != nil, "sync runs runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if !w.keepsRunsOffMain() {
		return nil
	}
	return w.syncRuns(runref.NetworkTimeout)
}

// syncRuns fetches, merges and pushes the ref, within timeout.
func (w *Window) syncRuns(timeout time.Duration) error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(timeout > 0, "a sync has a time limit")
	return w.afterRunsChange(w.runStore.Change(nil, timeout))
}

// afterRunsChange forgets statuses and histories, which the change may have moved on, and
// counts the runs still to share. A remote that can't be reached isn't an error: the runs are
// kept here, and the window says how many wait.
func (w *Window) afterRunsChange(changed error) error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(w.app != nil, "a window belongs to an app")
	w.statuses, w.histories = nil, nil
	unshared, err := w.runStore.Unshared()
	if err != nil {
		return errors.Join(changed, err)
	}
	w.runsUnshared = unshared
	if errors.Is(changed, runref.ErrUnreachable) {
		return nil
	}
	return changed
}

// runsOffers is "Move runs off main" while capabilities/runs still holds runs, and "Share
// runs" while runs wait to be shared; both only when runs are kept off main.
func (w *Window) runsOffers() []string {
	assert.That(w != nil, "runs offers run on a window")
	assert.That(w.runsUnshared >= 0, "a count isn't negative")
	offers := make([]string, 0, 2)
	if w.runStore == nil {
		return offers
	}
	if len(w.runsOnMain()) > 0 {
		offers = append(offers, OfferMoveRunsOffMain)
	}
	if w.runsUnshared > 0 {
		offers = append(offers, OfferShareRuns)
	}
	return offers
}

// runsMessages say how many runs wait to be shared, and what's left after moving them.
func (w *Window) runsMessages() []string {
	assert.That(w != nil, "runs messages run on a window")
	assert.That(w.runsUnshared >= 0, "a count isn't negative")
	messages := make([]string, 0, 2)
	if w.runsUnshared == 1 {
		messages = append(messages, "1 run not shared yet")
	} else if w.runsUnshared > 1 {
		messages = append(messages, fmt.Sprintf("%d runs not shared yet", w.runsUnshared))
	}
	if w.runsNote != "" {
		messages = append(messages, w.runsNote)
	}
	return messages
}

// askMoveRunsOffMain asks before the runs in capabilities/runs are moved.
func (w *Window) askMoveRunsOffMain() error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(len(w.runsOnMain()) > 0, "there are runs to move")
	w.ask(messageMoveOffMain, OfferMoveRunsOffMain, w.moveRunsOffMain)
	return nil
}

// moveRunsOffMain brings capabilities/runs into the ref and shares it, then sends the old run
// files to the Trash and writes off-main.md, for the developer to commit. Givens doesn't
// commit to their branch. Needed by "Move runs off main".
func (w *Window) moveRunsOffMain() error {
	assert.That(w.runStore != nil, "runs are kept in a store")
	assert.That(w.project != nil, "runs move in a loaded project")
	adopt := func(tree string) error { return runref.Adopt(tree, w.path) }
	if err := w.afterRunsChange(w.runStore.Change(adopt, runref.NetworkTimeout)); err != nil {
		return err
	}
	moving := w.runsOnMain()
	if _, err := os.Stat(filepath.Join(runs.Dir(w.path), "files")); err == nil {
		moving = append(moving, filepath.Join(runs.Dir(w.path), "files"))
	}
	for _, path := range moving {
		if err := w.app.trash(path); err != nil {
			return err
		}
	}
	marker := filepath.Join(runs.Dir(w.path), runs.OffMainFileName)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(marker, []byte(offMainText), 0o644); err != nil {
		return err
	}
	w.runsNote = MessageRunsMoved
	return w.reload()
}

// writeRun writes a run under root (the capabilities folder, or the ref's copy): its
// attached files first, so a line is never there without them, then its note, then its
// line. A second run of the scenario in the same second moves on a second, so it gets a
// files folder of its own.
func writeRun(root string, run runs.Run, attachments []string, note string) error {
	assert.That(filepath.IsAbs(root), "runs are written under an absolute folder")
	assert.That(run.Key != "", "a run has a key")
	at, err := runs.FreeTime(root, run.Key, run.Time)
	if err != nil {
		return err
	}
	run.Time = at
	if err := runs.CopyFiles(root, run, attachments); err != nil {
		return err
	}
	if strings.Contains(note, "\n") {
		run.Note = runs.Summary(note)
		if err := runs.WriteNote(root, run, note); err != nil {
			return err
		}
	}
	return runs.Append(root, run)
}
