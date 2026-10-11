package app

import (
	"path/filepath"

	"givens/internal/assert"
)

// Limits for Open a project.
const (
	// RecentsMax bounds the welcome window's "Recent" list.
	RecentsMax = 16
	// DroppedPathsMax bounds how many folders one drop on the Dock icon can open.
	DroppedPathsMax = WindowsMax
)

// Recents lists the projects opened before, newest first, as the capabilities folder paths
// they were opened with. It's read from AppDataDir, so it survives a quit and never lives in a
// project. Needed by "Reopen a recent project" in capabilities/open-a-project.
func (a *App) Recents() []string {
	assert.That(a != nil, "recents runs on an app")
	assert.That(a.config.AppDataDir != "", "recents live in the app's data folder")
	return nil
}

// OpenRecent opens one of Recents from the welcome window, like clicking it under "Recent".
// path is an entry of Recents. Needed by "Reopen a recent project".
func (w *Window) OpenRecent(path string) (*Window, error) {
	assert.That(w != nil, "open recent runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	assert.That(filepath.IsAbs(path), "a recent project has an absolute path")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	return nil, ErrNotImplemented
}

// FolderPickerShown is true while the folder picker that "Open folder…" shows is up. The Swift
// UI draws the real NSOpenPanel for it. Needed by "Start with nothing open".
// Ref: https://developer.apple.com/documentation/appkit/nsopenpanel
func (w *Window) FolderPickerShown() bool {
	assert.That(w != nil, "folder picker runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return false
}

// CloseWindow closes this window. Closing the last project window leaves nothing to reopen
// at the next launch. Needed by "Reopen a recent project", whose last quit had nothing open.
func (w *Window) CloseWindow() error {
	assert.That(w != nil, "close runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return ErrNotImplemented
}

// OpenDropped opens folders dropped on Givens's Dock icon, launching first if needed. Each
// folder opens like Open does. The Swift UI calls it from application(_:open:).
// Needed by "Drop a folder on Givens".
// Ref: https://developer.apple.com/documentation/appkit/nsapplicationdelegate/application(_:open:)
func (a *App) OpenDropped(paths []string) error {
	assert.That(a != nil, "open dropped runs on an app")
	assert.That(len(paths) > 0, "a drop holds at least one folder")
	assert.That(len(paths) <= DroppedPathsMax, "a drop fits DroppedPathsMax")
	return ErrNotImplemented
}

// InstallCommand is "Install givens command" in the Givens menu: it writes the givens command
// into binDir, a folder on the user's PATH, and returns what Givens says, naming where it
// went. The Swift UI passes the folder it picked; tests pass a temporary one.
// Needed by "Install the givens command".
func (a *App) InstallCommand(binDir string) (string, error) {
	assert.That(a != nil, "install runs on an app")
	assert.That(filepath.IsAbs(binDir), "the install folder is an absolute path")
	assert.That(len(binDir) <= PathLengthMax, "the path fits PathLengthMax")
	return "", ErrNotImplemented
}
