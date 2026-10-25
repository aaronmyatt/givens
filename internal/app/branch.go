package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"givens/internal/assert"
	"givens/internal/branch"
	"givens/internal/runs"
	"givens/internal/slug"
)

// Another branch's capabilities, read without checking that branch out. internal/branch copies
// the folder out of git into Givens's data folder; the window that shows the copy is read
// only, since its setup scripts belong to code that isn't checked out.

// Messages about branches.
const (
	// MessageNoBranch starts the message for a branch the repository doesn't have.
	MessageNoBranch = "No branch called "
	// messageReadOnly starts the message across the top of a branch's window.
	messageReadOnly = "Read only: "
)

// readOnlyOffers are the only offers a branch's window keeps: none of them writes a file or
// runs a script.
func readOnlyOffers() []string { return []string{OfferCopy, OfferCopySteps, OfferCancel} }

// Branches lists the branches of the git repository holding this window's project: local ones
// first, then remote ones. Needed by "Read capabilities from another branch".
func (w *Window) Branches() ([]string, error) {
	assert.That(w != nil, "branches runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has branches")
	if w.project == nil {
		return []string{}, nil
	}
	return branch.List(w.origin())
}

// OpenBranch opens this project's capabilities folder as it is on name, read only, in a new
// window in front. A branch the repository doesn't have, or one without the folder, is said
// in this window instead, and no window opens. Needed by "Read capabilities from another
// branch" and "Open a branch that doesn't exist".
func (w *Window) OpenBranch(name string) (*Window, error) {
	assert.That(w != nil, "open branch runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has branches")
	w.branchProblem = ""
	key := slug.Of(name)
	if key == "" || len(name) > branch.NameLengthMax {
		w.branchProblem = MessageNoBranch + name
		return nil, nil
	}
	copied, err := branch.Extract(w.origin(), name, w.app.branchCopy(w.origin(), name))
	switch {
	case errors.Is(err, branch.ErrNoBranch):
		w.branchProblem = MessageNoBranch + name
		return nil, nil
	case errors.Is(err, branch.ErrNoCapabilities):
		w.branchProblem = fmt.Sprintf("%s has no capabilities folder on %s", w.origin(), name)
		return nil, nil
	case err != nil:
		return nil, err
	}
	// Runs aren't kept per branch: the branch's window reads the ref this project keeps.
	if err := runs.LinkRefCopy(copied, w.origin()); err != nil {
		return nil, err
	}
	return w.app.openBranchWindow(copied, w.origin(), name)
}

// openBranchWindow shows a branch's copied capabilities folder in a read-only window in front,
// or brings its window forward if it's already open. It isn't added to Recents: the copy is
// throwaway.
func (a *App) openBranchWindow(folder string, origin string, name string) (*Window, error) {
	assert.That(filepath.IsAbs(folder), "the copy is at an absolute path")
	assert.That(name != "", "a branch has a name")
	for _, window := range a.windows {
		if window.kind == WindowProject && window.path == folder {
			a.toFront(window)
			return window, window.reload()
		}
	}
	if len(a.windows) >= WindowsMax {
		return nil, fmt.Errorf("%d windows are open, the most Givens keeps", WindowsMax)
	}
	a.windowsEver++
	window := &Window{
		app: a, id: a.windowsEver, kind: WindowProject, path: folder, found: true,
		branchOrigin: origin, branch: name,
	}
	if err := window.reload(); err != nil {
		return nil, err
	}
	a.closeWelcome()
	a.windows = append([]*Window{window}, a.windows...)
	assert.That(a.windows[0] == window, "an opened window is in front")
	return window, nil
}

// branchesDirName is the folder under Givens's data folder that holds branch copies.
const branchesDirName = "branches"

// branchCopy is where a branch's capabilities are copied to:
// <app data>/branches/<hash of origin>/<branch slug>-<hash of name>. The name's own hash keeps
// "a/b" and "a-b", which slug the same, in different folders.
func (a *App) branchCopy(origin string, name string) string {
	assert.That(filepath.IsAbs(origin), "the origin is an absolute path")
	assert.That(slug.Of(name) != "", "a branch's name has letters or digits")
	return filepath.Join(a.config.AppDataDir, branchesDirName, shortHash(origin),
		slug.Of(name)+"-"+shortHash(name))
}

// removeBranchCopy deletes a closed branch window's copy, and its origin's folder once that's
// empty. The copy is never reopened: opening the branch again copies it afresh.
func (w *Window) removeBranchCopy() error {
	assert.That(w.branch != "", "only a branch's window has a copy")
	assert.That(w.branchOrigin != "", "a branch's window knows its origin")
	copied := w.app.branchCopy(w.branchOrigin, w.branch)
	if err := os.RemoveAll(copied); err != nil {
		return err
	}
	// Remove fails on a folder that still holds another branch's copy, which is meant.
	// Ref: https://pkg.go.dev/os#Remove
	_ = os.Remove(filepath.Dir(copied))
	return nil
}

// writable refuses a change in a branch's window. The UI doesn't offer one there; this stops
// a Needs call or a CLI one that asks anyway.
func (w *Window) writable() error {
	assert.That(w != nil, "writable runs on a window")
	assert.That(w.branch == "" || w.branchOrigin != "", "a branch's window knows its origin")
	if w.branch != "" {
		return fmt.Errorf("%s @ %s: %w", filepath.Base(w.branchOrigin), w.branch, ErrReadOnly)
	}
	return nil
}

// origin is the capabilities folder this window's project lives in: for a branch's window,
// the folder it was copied from.
func (w *Window) origin() string {
	assert.That(w != nil, "origin runs on a window")
	assert.That(w.branch == "" || w.branchOrigin != "", "a branch's window knows its origin")
	if w.branch != "" {
		return w.branchOrigin
	}
	return w.path
}

// branchMessages say a branch's window is read only, and why a branch didn't open.
func (w *Window) branchMessages() []string {
	assert.That(w != nil, "branch messages runs on a window")
	messages := make([]string, 0, 2)
	if w.branch != "" {
		messages = append(messages, messageReadOnly+w.branch+" isn't checked out, so its "+
			"setup scripts don't run here")
	}
	if w.branchProblem != "" {
		messages = append(messages, w.branchProblem)
	}
	assert.That(len(messages) <= 2, "branch messages stay bounded")
	return messages
}

// readOnly cuts offers down to readOnlyOffers in a branch's window.
func (w *Window) readOnly(offers []string) []string {
	assert.That(w != nil, "read only runs on a window")
	assert.That(len(offers) <= NameLengthMax, "offers stay bounded")
	if w.branch == "" {
		return offers
	}
	return slices.DeleteFunc(offers, func(offer string) bool {
		return !slices.Contains(readOnlyOffers(), offer)
	})
}

// shortHash names a folder after text, briefly: the first 12 hex digits of its SHA-256.
// Ref: https://pkg.go.dev/crypto/sha256#Sum256
func shortHash(text string) string {
	assert.That(text != "", "something is hashed")
	sum := sha256.Sum256([]byte(text))
	short := hex.EncodeToString(sum[:])[:12]
	assert.That(len(short) == 12, "the hash is 12 digits")
	return short
}
