// Package app is the core of Givens. The Swift UI (through `givens serve`), the CLI and the
// acceptance steps all drive it, so a scenario that passes against this package passes for the
// app too.
//
// Most of it is a stub for now: each unbuilt call returns ErrNotImplemented, and the acceptance
// suite fails on it. Building Givens means replacing these stubs, scenario by scenario, through
// the backlog in todo.txt. Every query a window offers ("what's visible", "what's offered") is
// what a person would read off the screen, so the Swift UI renders exactly this state.
package app

import (
	"errors"
	"fmt"
	"time"

	"givens/internal/assert"
)

// ErrNotImplemented is returned by every part of the core that isn't built yet.
var ErrNotImplemented = errors.New("not built yet: see todo.txt")

// ErrNotFound is returned for a capability or scenario name a window doesn't have.
var ErrNotFound = errors.New("there's nothing with that name")

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	PathLengthMax = 4096
	NameLengthMax = 1024
	WindowsMax    = 64
)

// Window kinds.
const (
	WindowWelcome = "welcome"
	WindowProject = "project"
)

// Clipboard is the system clipboard. The Swift UI passes the real one; tests pass a fake.
type Clipboard interface {
	WriteText(text string) error
	ReadText() (string, error)
}

// Trash is the Finder Trash, so a delete can be undone in Finder.
type Trash interface {
	Move(path string) error
}

// Clock tells the time, so "Verified today" is testable.
type Clock interface {
	Now() time.Time
}

// Config is everything the core needs from outside itself.
type Config struct {
	// AppDataDir holds Givens's own data: trusted folders, recent projects, open windows.
	// It is never inside a project.
	AppDataDir string
	Clipboard  Clipboard
	Trash      Trash
	Clock      Clock
}

// App is one running Givens. It isn't safe for concurrent use: `givens serve` and the steps
// each call it one command at a time.
type App struct {
	config      Config
	windows     []*Window // front first
	opener      URLOpener // opens links; nil until UseURLOpener
	windowsEver int       // windows opened so far; numbers the next one
}

// New returns an App that hasn't launched yet.
func New(config Config) (*App, error) {
	assert.That(config.AppDataDir != "", "the app has a data folder")
	assert.That(config.Clipboard != nil, "the app has a clipboard")
	assert.That(config.Trash != nil, "the app has a trash")
	assert.That(config.Clock != nil, "the app has a clock")
	return &App{config: config, windows: make([]*Window, 0, WindowsMax)}, nil
}

// Launch starts Givens: it reopens the windows open at the last quit, or shows the welcome
// window. See capabilities/open-a-project.
func (a *App) Launch() error {
	assert.That(a != nil, "launch runs on an app")
	assert.That(a.config.AppDataDir != "", "the app knows where its data lives")
	return ErrNotImplemented
}

// Open opens a capabilities folder, or a project folder holding one, in a window. Opening a
// project that's already open brings its window to the front instead.
func (a *App) Open(path string) (*Window, error) {
	assert.That(a != nil, "open runs on an app")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	folder, found, err := findCapabilities(path)
	if err != nil {
		return nil, err
	}
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
	window := &Window{app: a, id: a.windowsEver, kind: WindowProject, path: folder, found: found}
	if err := window.reload(); err != nil {
		return nil, err
	}
	a.windows = append([]*Window{window}, a.windows...)
	assert.That(a.windows[0] == window, "an opened window is in front")
	return window, nil
}

// toFront moves an open window to the front.
func (a *App) toFront(window *Window) {
	assert.That(window != nil, "a window is given")
	assert.That(len(a.windows) <= WindowsMax, "windows stay bounded")
	others := make([]*Window, 0, len(a.windows))
	for _, other := range a.windows {
		if other != window {
			others = append(others, other)
		}
	}
	a.windows = append([]*Window{window}, others...)
}

// Windows lists the open windows, front first.
func (a *App) Windows() []*Window {
	assert.That(a != nil, "windows runs on an app")
	assert.That(len(a.windows) <= WindowsMax, "windows stay bounded")
	return append([]*Window{}, a.windows...)
}

// Front returns the front window, or nil when none is open.
func (a *App) Front() *Window {
	assert.That(a != nil, "front runs on an app")
	assert.That(len(a.windows) <= WindowsMax, "windows stay bounded")
	if len(a.windows) == 0 {
		return nil
	}
	return a.windows[0]
}

// Quit closes every window, remembering which projects were open.
func (a *App) Quit() error {
	assert.That(a != nil, "quit runs on an app")
	assert.That(a.config.AppDataDir != "", "the app knows where to remember windows")
	open := make([]string, 0, len(a.windows))
	for _, window := range a.windows {
		if window.kind == WindowProject {
			open = append(open, window.path)
		}
	}
	if err := a.writeFolders(openWindowsFileName, open); err != nil {
		return err
	}
	for _, window := range a.windows {
		if window.session != nil {
			window.session.removeArguments()
		}
	}
	a.windows = a.windows[:0]
	return nil
}

// openWindowsFileName remembers which projects were open at the last quit, front first.
const openWindowsFileName = "open-windows.txt"
