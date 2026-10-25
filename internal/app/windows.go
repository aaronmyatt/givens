package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"

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
	recents, err := a.folders(recentsFileName)
	if err != nil {
		return []string{}
	}
	return recents
}

// recentsFileName holds Recents, newest first.
const recentsFileName = "recents.txt"

// rememberRecent puts a folder first in Recents, keeping RecentsMax.
func (a *App) rememberRecent(folder string) error {
	assert.That(a != nil, "remember recent runs on an app")
	assert.That(filepath.IsAbs(folder), "a recent folder is absolute")
	recents := slices.DeleteFunc(a.Recents(), func(other string) bool { return other == folder })
	recents = append([]string{folder}, recents...)
	return a.writeFolders(recentsFileName, recents[:min(len(recents), RecentsMax)])
}

// Launch starts Givens: it reopens the windows open at the last quit, or shows the welcome
// window. See capabilities/open-a-project.
func (a *App) Launch() error {
	assert.That(a != nil, "launch runs on an app")
	assert.That(a.config.AppDataDir != "", "the app knows where its data lives")
	// Branch windows aren't reopened after quitting, so every copy left from before is stale.
	if err := os.RemoveAll(filepath.Join(a.config.AppDataDir, branchesDirName)); err != nil {
		return err
	}
	open, err := a.folders(openWindowsFileName)
	if err != nil {
		return err
	}
	// Reopen back to front, so the window in front at the quit ends up in front again.
	for index := len(open) - 1; index >= 0; index-- {
		if _, err := os.Stat(open[index]); err != nil {
			continue
		}
		if _, err := a.Open(open[index]); err != nil {
			return err
		}
	}
	if len(a.windows) == 0 {
		a.showWelcome()
	}
	return nil
}

// Welcome window text and action.
const (
	OfferOpenFolder = "Open folder…"
	LabelRecent     = "Recent"
)

// showWelcome opens the welcome window, unless one is open.
func (a *App) showWelcome() {
	assert.That(a != nil, "show welcome runs on an app")
	assert.That(len(a.windows) < WindowsMax, "there's room for a window")
	for _, window := range a.windows {
		if window.kind == WindowWelcome {
			return
		}
	}
	a.windowsEver++
	a.windows = append([]*Window{{app: a, id: a.windowsEver, kind: WindowWelcome}}, a.windows...)
}

// closeWelcome closes the welcome window once a project opens.
func (a *App) closeWelcome() {
	assert.That(a != nil, "close welcome runs on an app")
	assert.That(len(a.windows) <= WindowsMax, "windows stay bounded")
	a.windows = slices.DeleteFunc(a.windows, func(window *Window) bool {
		return window.kind == WindowWelcome
	})
}

// OpenRecent opens one of Recents from the welcome window, like clicking it under "Recent".
// path is an entry of Recents. Needed by "Reopen a recent project".
func (w *Window) OpenRecent(path string) (*Window, error) {
	assert.That(w != nil, "open recent runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	assert.That(filepath.IsAbs(path), "a recent project has an absolute path")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	return w.app.Open(path)
}

// FolderPickerShown is true while the folder picker that "Open folder…" shows is up. The Swift
// UI draws the real NSOpenPanel for it. Needed by "Start with nothing open".
// Ref: https://developer.apple.com/documentation/appkit/nsopenpanel
func (w *Window) FolderPickerShown() bool {
	assert.That(w != nil, "folder picker runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.folderPicker
}

// FolderPicker shows the system's folder picker. The Swift UI's draws NSOpenPanel; under
// serve it's an effect the app carries out.
type FolderPicker interface {
	ShowFolderPicker() error
}

// UseFolderPicker sets what shows the folder picker for "Open folder…".
func (a *App) UseFolderPicker(picker FolderPicker) error {
	assert.That(a != nil, "use folder picker runs on an app")
	assert.That(picker != nil, "a picker is given")
	a.picker = picker
	return nil
}

// openFolder is "Open folder…": it shows the folder picker.
func (w *Window) openFolder() error {
	assert.That(w != nil, "open folder runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.folderPicker = true
	if w.app.picker == nil {
		return nil
	}
	return w.app.picker.ShowFolderPicker()
}

// CloseWindow closes this window. Closing the last project window leaves nothing to reopen
// at the next launch. A branch's window takes its copy with it. Needed by "Reopen a recent
// project", whose last quit had nothing open.
func (w *Window) CloseWindow() error {
	assert.That(w != nil, "close runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.session != nil {
		w.session.removeArguments()
	}
	w.app.windows = slices.DeleteFunc(w.app.windows, func(other *Window) bool {
		return other == w
	})
	if w.branch != "" {
		return w.removeBranchCopy()
	}
	return nil
}

// OpenDropped opens folders dropped on Givens's Dock icon, launching first if needed. Each
// folder opens like Open does. The Swift UI calls it from application(_:open:).
// Needed by "Drop a folder on Givens".
// Ref: https://developer.apple.com/documentation/appkit/nsapplicationdelegate/application(_:open:)
func (a *App) OpenDropped(paths []string) error {
	assert.That(a != nil, "open dropped runs on an app")
	assert.That(len(paths) > 0, "a drop holds at least one folder")
	assert.That(len(paths) <= DroppedPathsMax, "a drop fits DroppedPathsMax")
	if len(a.windows) == 0 {
		if err := a.Launch(); err != nil {
			return err
		}
	}
	for _, path := range paths {
		if _, err := a.Open(path); err != nil {
			return err
		}
	}
	return nil
}

// InstallCommand is "Install givens command" in the Givens menu: it links the givens command
// into binDir, a folder on the user's PATH, and returns what Givens says, naming where it
// went. The Swift UI passes the folder it picked; tests pass a temporary one.
// Needed by "Install the givens command".
// Ref: https://pkg.go.dev/os#Symlink
func (a *App) InstallCommand(binDir string) (string, error) {
	assert.That(a != nil, "install runs on an app")
	assert.That(filepath.IsAbs(binDir), "the install folder is an absolute path")
	assert.That(len(binDir) <= PathLengthMax, "the path fits PathLengthMax")
	command, err := a.givensCommand()
	if err != nil {
		return "", err
	}
	installed := filepath.Join(binDir, "givens")
	if err := os.Remove(installed); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Symlink(command, installed); err != nil {
		return "", err
	}
	return "Installed givens in " + binDir + ". Open a new terminal window to use it.", nil
}

// givensCommand is the givens command: Config.Command, or else the running program, which is
// the givens binary under serve.
func (a *App) givensCommand() (string, error) {
	assert.That(a != nil, "givens command runs on an app")
	assert.That(len(a.config.Command) <= PathLengthMax, "the path fits PathLengthMax")
	if a.config.Command != "" {
		return a.config.Command, nil
	}
	// Ref: https://pkg.go.dev/os#Executable
	return os.Executable()
}
