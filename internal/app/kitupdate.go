package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/internal/assert"
	"givens/internal/kit"
)

// Limits for Update the kit.
const (
	// KitReleasesMax bounds how many kit versions Givens remembers.
	KitReleasesMax = 256
	// KitLengthMax bounds one kit's text, in bytes.
	KitLengthMax = 1 << 20
)

// KitVersionPrefix starts the kit's first line: "# givens kit 2".
const KitVersionPrefix = "# givens kit "

// KitRelease is one version of the kit, as Givens shipped it. Givens keeps every release it
// has shipped so it can tell an unedited old kit from one edited by hand.
type KitRelease struct {
	Version int    `json:"version"`
	Text    string `json:"text"` // the whole givens.sh, starting with its version line
}

// ShipKits sets the kit releases this app knows, oldest first; the last is the version it
// ships. Without a call the app uses the releases built into it. Acceptance steps call it so
// a scenario controls "Givens ships version N". Needed by every scenario in
// capabilities/update-the-kit.
func (a *App) ShipKits(releases []KitRelease) error {
	assert.That(a != nil, "ship kits runs on an app")
	assert.That(len(releases) > 0, "Givens ships at least one kit")
	assert.That(len(releases) <= KitReleasesMax, "releases fit KitReleasesMax")
	for _, release := range releases {
		assert.That(release.Version > 0, "kit versions start at 1")
		versioned := strings.HasPrefix(release.Text, KitVersionPrefix)
		assert.That(versioned, "a kit starts with its version line")
		assert.That(len(release.Text) <= KitLengthMax, "a kit fits KitLengthMax")
	}
	a.kits = append([]KitRelease{}, releases...)
	return nil
}

// shippedKits are the releases this app knows, oldest first: ShipKits's, or the one built in.
func (a *App) shippedKits() []KitRelease {
	assert.That(a != nil, "shipped kits runs on an app")
	assert.That(len(a.kits) <= KitReleasesMax, "releases stay bounded")
	if len(a.kits) > 0 {
		return a.kits
	}
	return []KitRelease{{Version: kit.ShippedVersion(), Text: kit.Script}}
}

// Kit messages and actions.
const (
	OfferUpdateKit     = "Update kit"
	OfferReplaceAnyway = "Replace anyway"
	MessageKitEdited   = "givens.sh has changes of its own"
	MessageNewerKit    = "This project uses a newer kit. Update Givens to edit its scripts."
	kitFile            = "_shared/givens.sh"
)

// kitState reads the project's kit: its text, its version (0 when unreadable), and the
// release Givens ships last.
func (w *Window) kitState() (string, int, KitRelease) {
	assert.That(w != nil, "kit state runs on a window")
	assert.That(w.found, "the window shows a capabilities folder")
	shipped := w.app.shippedKits()
	latest := shipped[len(shipped)-1]
	text, err := os.ReadFile(filepath.Join(w.path, filepath.FromSlash(kitFile)))
	if err != nil {
		return "", 0, latest
	}
	version, _ := kit.Version(string(text))
	return string(text), version, latest
}

// kitMessages say where the project's kit stands against the one Givens ships.
func (w *Window) kitMessages() []string {
	assert.That(w != nil, "kit messages run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if !w.found || w.kind != WindowProject {
		return []string{}
	}
	if w.kitProblem != "" {
		return []string{w.kitProblem}
	}
	_, version, latest := w.kitState()
	switch {
	case version == 0:
		return []string{}
	case version < latest.Version:
		return []string{fmt.Sprintf("Kit update available (%d → %d)", version, latest.Version)}
	case version > latest.Version:
		return []string{MessageNewerKit}
	}
	return []string{}
}

// updateKit replaces the project's kit with the latest Givens ships, when it's an unedited
// copy of a release; otherwise it asks first, offering "Compare" and "Replace anyway".
func (w *Window) updateKit() error {
	assert.That(w != nil, "update kit runs on a window")
	assert.That(w.found, "the window shows a capabilities folder")
	text, version, latest := w.kitState()
	path := filepath.Join(w.path, filepath.FromSlash(kitFile))
	replace := func() error {
		if err := os.WriteFile(path, []byte(latest.Text), 0o644); err != nil {
			return err
		}
		return w.reload()
	}
	for _, release := range w.app.shippedKits() {
		if release.Version == version && release.Text == text {
			return replace()
		}
	}
	w.askEither(MessageKitEdited, OfferReplaceAnyway, replace, OfferCompare, func() error {
		_, err := w.Edit(kitFile)
		return err
	})
	return nil
}

// restoreKit writes the kit Givens ships when the capabilities folder has none, since every
// setup script's first line sources it. It runs whenever the project is read from disk: on
// opening, and after any change to its files. A write that fails shows across the top.
// Needed by "A missing kit is put back".
func (w *Window) restoreKit() {
	assert.That(w.found, "the window shows a capabilities folder")
	assert.That(filepath.IsAbs(w.path), "the capabilities folder is absolute")
	path := filepath.Join(w.path, filepath.FromSlash(kitFile))
	w.kitProblem = ""
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return
	}
	shipped := w.app.shippedKits()
	latest := shipped[len(shipped)-1]
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.kitProblem = "Couldn't put back " + kitFile + ": " + err.Error()
		return
	}
	if err := os.WriteFile(path, []byte(latest.Text), 0o644); err != nil {
		w.kitProblem = "Couldn't put back " + kitFile + ": " + err.Error()
	}
}
