// Package world is what every capability's acceptance suite shares: a fresh copy of the sample
// shop per scenario, an app wired to fake clipboard, trash and clock, the steps used by more than
// one capability, and the suite runner.
//
// Each capability has its own suite in acceptance/<capability>/, over its own .feature file.
// Scenarios tagged @wip are skipped by default; run them (and watch them fail) with
//
//	GIVENS_TAGS=@wip go test ./acceptance/...
//
// Ref: https://github.com/cucumber/godog#readme
package world

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/app"
	"givens/internal/assert"
)

// Limits.
const (
	repoDepthMax  = 32
	OutputLineMax = 1 << 16
	// SetupWait bounds how long a step waits for a Drop in's setup.
	SetupWait = 10 * time.Second
)

// World is one scenario's state. Capability suites keep their own state in a struct that holds
// a *World.
type World struct {
	Root    string // the repository root
	Dir     string // this scenario's temporary folder
	Project string // the sample shop's copy, inside Dir; "" until a step copies it
	AppData string // Givens's own data folder, inside Dir
	Bin     string // folder holding the built givens command
	Cwd     string // where commands run; "" means the sample shop's folder

	App       *app.App
	Window    *app.Window
	Session   *app.Session
	Clipboard *Clipboard
	Trash     *Trash
	Clock     *Clock

	Err      error  // the last action's error, for steps that expect one
	Output   string // the last command's output, stdout and stderr together
	ExitCode int    // the last command's exit code

	// Untrusted keeps OpenSampleShop from trusting the shop. Scenarios about trust set it;
	// every other scenario starts trusted, as if the developer chose "Trust" earlier.
	Untrusted bool

	EditedPath string // the file the scenario last edited, for "the file is saved anyway"
	EditedText string // what that edit saved
}

// Capabilities is the sample shop's capabilities folder.
func (w *World) Capabilities() string {
	assert.That(w != nil, "capabilities runs on a world")
	assert.That(w.Project != "", "a step copied the sample shop first")
	return filepath.Join(w.Project, "capabilities")
}

// CopySampleShop copies testdata/sample-shop into this scenario's folder, with the kit from
// internal/kit/givens.sh when it exists.
func (w *World) CopySampleShop() error {
	assert.That(w != nil, "copy runs on a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	if w.Project != "" {
		return nil
	}
	project := filepath.Join(w.Dir, "sample-shop")
	// CopyFS copies a whole tree. Ref: https://pkg.go.dev/os#CopyFS
	fixture := os.DirFS(filepath.Join(w.Root, "testdata", "sample-shop"))
	if err := os.CopyFS(project, fixture); err != nil {
		return err
	}
	w.Project = project
	kit, err := os.ReadFile(filepath.Join(w.Root, "internal", "kit", "givens.sh"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return w.WriteFile("capabilities/_shared/givens.sh", string(kit))
}

// StartApp builds the app on fakes. It doesn't launch it.
func (w *World) StartApp() error {
	assert.That(w != nil, "start runs on a world")
	assert.That(w.AppData != "", "the scenario has an app data folder")
	if w.App != nil {
		return nil
	}
	core, err := app.New(app.Config{
		AppDataDir: w.AppData, Clipboard: w.Clipboard, Trash: w.Trash, Clock: w.Clock,
	})
	w.App = core
	return err
}

// OpenSampleShop copies the sample shop, trusts it unless Untrusted, and opens its
// capabilities folder in the app.
func (w *World) OpenSampleShop() error {
	assert.That(w != nil, "open runs on a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	if err := w.CopySampleShop(); err != nil {
		return err
	}
	if err := w.StartApp(); err != nil {
		return err
	}
	if !w.Untrusted {
		if err := w.App.MarkTrusted(w.Capabilities()); err != nil {
			return fmt.Errorf("trusting the sample shop: %w", err)
		}
	}
	window, err := w.App.Open(w.Capabilities())
	if err != nil {
		return fmt.Errorf("opening the sample shop: %w", err)
	}
	w.Window = window
	return nil
}

// Front returns the window under test, failing the step when none is open.
func (w *World) Front() (*app.Window, error) {
	assert.That(w != nil, "front runs on a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	if w.Window != nil {
		return w.Window, nil
	}
	// A step that acts on Givens with nothing open means the sample shop, already copied.
	if w.App == nil && w.Project != "" {
		if err := w.OpenSampleShop(); err != nil {
			return nil, err
		}
		return w.Window, nil
	}
	if w.App != nil && w.App.Front() != nil {
		w.Window = w.App.Front()
		return w.Window, nil
	}
	return nil, errors.New("no Givens window is open")
}

// WriteFile writes content to rel, a path inside the sample shop, making folders as needed.
func (w *World) WriteFile(rel string, content string) error {
	assert.That(w != nil, "write runs on a world")
	assert.That(!filepath.IsAbs(rel), "rel is relative to the sample shop")
	path := filepath.Join(w.Project, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadFile reads rel, a path inside the sample shop.
func (w *World) ReadFile(rel string) (string, error) {
	assert.That(w != nil, "read runs on a world")
	assert.That(!filepath.IsAbs(rel), "rel is relative to the sample shop")
	content, err := os.ReadFile(filepath.Join(w.Project, rel))
	return string(content), err
}

// Expect returns nil when one of got contains want, and an error naming what was seen if not.
func Expect(got []string, want string, what string) error {
	assert.That(want != "", "an expectation names text")
	assert.That(what != "", "an expectation says where it looked")
	for _, line := range got {
		if strings.Contains(line, want) {
			return nil
		}
	}
	return fmt.Errorf("%s doesn't include %q; it has %q", what, want, got)
}

// ExpectNot returns nil when none of got contains unwanted.
func ExpectNot(got []string, unwanted string, what string) error {
	assert.That(unwanted != "", "an expectation names text")
	assert.That(what != "", "an expectation says where it looked")
	for _, line := range got {
		if strings.Contains(line, unwanted) {
			return fmt.Errorf("%s includes %q, and shouldn't: %q", what, unwanted, line)
		}
	}
	return nil
}

// RepoRoot finds the folder holding go.mod, walking up from the working directory.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	assert.That(err != nil || dir != "", "the working directory has a path")
	if err != nil {
		return "", err
	}
	for depth := 0; depth < repoDepthMax; depth++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	assert.That(repoDepthMax > 0, "the search had a bound")
	return "", errors.New("no go.mod above the working directory")
}
