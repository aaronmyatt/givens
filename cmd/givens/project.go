package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/app"
	"givens/internal/assert"
)

// The givens command's own commands for a person at a terminal (edit, new, rename, delete,
// the TUI…) all drive internal/app, so they share every rule. They open the project with
// openProject.
// See capabilities/work-in-the-terminal.

// appDataEnv moves Givens's own data, as the acceptance suites do. By default it's where the
// archived Mac app kept it, so a folder trusted there stays trusted.
const appDataEnv = "GIVENS_APP_DATA"

// effectTimeout bounds pbcopy, pbpaste, open and the Finder's Trash.
const effectTimeout = 10 * time.Second

// openProject opens the project the working folder is in: its capabilities folder, found as
// givens list finds it, in a window of an app with no UI.
func openProject() (*app.App, *app.Window, error) {
	assert.That(appDataEnv != "", "app data can be moved")
	assert.That(effectTimeout > 0, "effects have a limit")
	return openProjectIn(".")
}

// openProjectIn opens the project whose capabilities folder is start, in start or above it.
func openProjectIn(start string) (*app.App, *app.Window, error) {
	assert.That(start != "", "the search starts somewhere")
	assert.That(appDataEnv != "", "app data can be moved")
	core, err := newTerminalApp()
	if err != nil {
		return nil, nil, err
	}
	capabilities, err := findCapabilities(start)
	if err != nil {
		return nil, nil, err
	}
	window, err := core.Open(capabilities)
	if err != nil {
		return nil, nil, err
	}
	return core, window, nil
}

// newTerminalApp is an app over Givens's data folder, with the Mac's clipboard and Trash.
func newTerminalApp() (*app.App, error) {
	assert.That(appDataEnv != "", "app data can be moved")
	assert.That(effectTimeout > 0, "effects have a limit")
	dir := os.Getenv(appDataEnv)
	if dir == "" {
		// On macOS, the user config dir is ~/Library/Application Support, where the app
		// keeps "Givens". Ref: https://pkg.go.dev/os#UserConfigDir
		config, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(config, "Givens")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	effects := macEffects{}
	if os.Getenv("GIVENS_TEST_SEAMS") == "1" && os.Getenv(testEffectsEnv) != "" {
		effects.dir = os.Getenv(testEffectsEnv)
	}
	core, err := app.New(app.Config{
		AppDataDir: dir, Clipboard: effects, Trash: effects, Clock: realClock{},
		NoSyncOnOpen: true,
	})
	if err != nil {
		return nil, err
	}
	if err := core.UseURLOpener(effects); err != nil {
		return nil, err
	}
	return core, nil
}

// realClock is the wall clock.
type realClock struct{}

// Now is the time now.
func (realClock) Now() time.Time {
	now := time.Now()
	assert.That(!now.IsZero(), "the clock tells a time")
	assert.That(now.Location() != nil, "the time has a zone")
	return now
}

// testEffectsEnv names a folder that, with GIVENS_TEST_SEAMS=1, stands in for the
// clipboard (its clipboard file), the Trash (its Trash folder) and the browser (its opened
// file), so the acceptance suites never touch the developer's own. Both must be set, so it
// can't be on by accident.
const testEffectsEnv = "GIVENS_TEST_EFFECTS"

// macEffects does what the Mac app's Platform does, from a terminal: the clipboard through
// pbcopy and pbpaste, the Trash through the Finder, links through open. With dir set, the
// test seam, they go to files in dir instead.
type macEffects struct {
	dir string
}

// WriteText puts text on the clipboard. Ref: https://ss64.com/mac/pbcopy.html
func (m macEffects) WriteText(text string) error {
	assert.That(len(text) < 1<<24, "the clipboard takes a bounded text")
	assert.That(effectTimeout > 0, "pbcopy has a limit")
	if m.dir != "" {
		return os.WriteFile(filepath.Join(m.dir, "clipboard"), []byte(text), 0o644)
	}
	_, err := runEffect(text, "pbcopy")
	return err
}

// ReadText reads the clipboard.
func (m macEffects) ReadText() (string, error) {
	assert.That(effectTimeout > 0, "pbpaste has a limit")
	assert.That(appDataEnv != "", "the effects belong to the givens command")
	if m.dir != "" {
		text, err := os.ReadFile(filepath.Join(m.dir, "clipboard"))
		return string(text), err
	}
	return runEffect("", "pbpaste")
}

// Move sends path to the Trash through the Finder, so it can be put back from there.
// Ref: https://ss64.com/mac/osascript.html
func (m macEffects) Move(path string) error {
	assert.That(filepath.IsAbs(path), "only an absolute path is trashed")
	assert.That(!strings.Contains(path, "\n"), "a path is one line")
	if m.dir != "" {
		trash := filepath.Join(m.dir, "Trash")
		if err := os.MkdirAll(trash, 0o755); err != nil {
			return err
		}
		return os.Rename(path, filepath.Join(trash, filepath.Base(path)))
	}
	quoted := strings.ReplaceAll(strings.ReplaceAll(path, `\`, `\\`), `"`, `\"`)
	_, err := runEffect("", "osascript", "-e",
		`tell application "Finder" to delete POSIX file "`+quoted+`"`)
	return err
}

// OpenURL opens a link in the default browser. Ref: https://ss64.com/mac/open.html
func (m macEffects) OpenURL(url string) error {
	assert.That(url != "", "a link is opened")
	assert.That(!strings.HasPrefix(url, "-"), "a link isn't an option")
	if m.dir != "" {
		opened, err := os.OpenFile(filepath.Join(m.dir, "opened"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = opened.WriteString(url + "\n")
		return errors.Join(err, opened.Close())
	}
	_, err := runEffect("", "open", url)
	return err
}

// runEffect runs one of macOS's commands with stdin, within effectTimeout.
func runEffect(stdin string, name string, args ...string) (string, error) {
	assert.That(name != "", "a command is run")
	assert.That(len(args) < 8, "an effect takes few arguments")
	command := exec.Command(name, args...)
	command.Stdin = strings.NewReader(stdin)
	command.WaitDelay = time.Second
	var out, problem bytes.Buffer
	command.Stdout, command.Stderr = &out, &problem
	if err := command.Start(); err != nil {
		return "", err
	}
	timer := time.AfterFunc(effectTimeout, func() { _ = command.Process.Kill() })
	defer timer.Stop()
	if err := command.Wait(); err != nil {
		return "", errors.Join(err, errors.New(strings.TrimSpace(problem.String())))
	}
	return out.String(), nil
}
