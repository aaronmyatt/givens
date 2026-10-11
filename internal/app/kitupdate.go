package app

import (
	"strings"

	"givens/internal/assert"
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
	return ErrNotImplemented
}
